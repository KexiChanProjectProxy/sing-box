package direct

import (
	"context"
	"net"
	"net/netip"
	"time"

	B "github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

type xlat464AddressMapper struct {
	prefix      netip.Prefix
	prefixBytes [16]byte
}

func newXLAT464AddressMapper(options option.Xlat464Options) (xlat464AddressMapper, error) {
	if options.Prefix == nil {
		return xlat464AddressMapper{}, E.New("xlat464: prefix is required")
	}
	prefix := netip.Prefix(*options.Prefix)
	if !prefix.IsValid() || prefix.Addr().Is4In6() || !prefix.Addr().Is6() || prefix.Bits() != 96 {
		return xlat464AddressMapper{}, E.New("xlat464: prefix must be an IPv6 /96")
	}
	prefix = prefix.Masked()
	return xlat464AddressMapper{prefix: prefix, prefixBytes: prefix.Addr().As16()}, nil
}

func (m xlat464AddressMapper) synthesize(address netip.Addr) netip.Addr {
	if address.Is4In6() {
		address = address.Unmap()
	}
	if !address.Is4() {
		return address
	}
	var mapped [16]byte
	copy(mapped[:12], m.prefixBytes[:12])
	ipv4 := address.As4()
	copy(mapped[12:], ipv4[:])
	return netip.AddrFrom16(mapped)
}

func (m xlat464AddressMapper) reverse(address netip.Addr) netip.Addr {
	if address.Is4In6() || !m.prefix.Contains(address) {
		return address
	}
	bytes := address.As16()
	var ipv4Mapped [16]byte
	ipv4Mapped[10] = 0xff
	ipv4Mapped[11] = 0xff
	copy(ipv4Mapped[12:], bytes[12:])
	return netip.AddrFrom16(ipv4Mapped).Unmap()
}

func (m xlat464AddressMapper) synthesizeIPv4Addresses(addresses []netip.Addr) []netip.Addr {
	mappedAddresses := make([]netip.Addr, 0, len(addresses))
	for _, address := range addresses {
		if address.Is4() || address.Is4In6() {
			mappedAddresses = append(mappedAddresses, m.synthesize(address))
		}
	}
	return mappedAddresses
}

type xlat464Dialer struct {
	dialer dialer.ParallelInterfaceDialer
	mapper xlat464AddressMapper
}

func newXLAT464Dialer(base dialer.ParallelInterfaceDialer, options option.Xlat464Options) (*xlat464Dialer, error) {
	mapper, err := newXLAT464AddressMapper(options)
	if err != nil {
		return nil, err
	}
	return &xlat464Dialer{dialer: base, mapper: mapper}, nil
}

func (d *xlat464Dialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return d.dialer.DialContext(ctx, network, d.mapDestination(destination))
}

func (d *xlat464Dialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	conn, err := d.dialer.ListenPacket(ctx, d.mapDestination(destination))
	if err != nil {
		return nil, err
	}
	return d.wrapPacketConn(conn), nil
}

func (d *xlat464Dialer) DialParallelInterface(ctx context.Context, network string, destination M.Socksaddr, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.Conn, error) {
	return d.dialer.DialParallelInterface(ctx, network, d.mapDestination(destination), strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
}

func (d *xlat464Dialer) ListenSerialInterfacePacket(ctx context.Context, destination M.Socksaddr, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.PacketConn, error) {
	conn, err := d.dialer.ListenSerialInterfacePacket(ctx, d.mapDestination(destination), strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
	if err != nil {
		return nil, err
	}
	return d.wrapPacketConn(conn), nil
}

func (d *xlat464Dialer) DialParallelNetwork(ctx context.Context, network string, destination M.Socksaddr, destinationAddresses []netip.Addr, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.Conn, error) {
	mappedAddresses, err := d.mapDestinationAddresses(destination, destinationAddresses)
	if err != nil {
		return nil, err
	}
	return dialer.DialParallelNetwork(ctx, d.dialer, network, d.mapDestination(destination), mappedAddresses, true, strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
}

func (d *xlat464Dialer) ListenSerialNetworkPacket(ctx context.Context, destination M.Socksaddr, destinationAddresses []netip.Addr, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.PacketConn, netip.Addr, error) {
	mappedAddresses, err := d.mapDestinationAddresses(destination, destinationAddresses)
	if err != nil {
		return nil, netip.Addr{}, err
	}
	conn, mappedDestination, err := dialer.ListenSerialNetworkPacket(ctx, d.dialer, d.mapDestination(destination), mappedAddresses, strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
	if err != nil {
		return nil, netip.Addr{}, err
	}
	return d.wrapPacketConn(conn), d.mapper.reverse(mappedDestination), nil
}

func (d *xlat464Dialer) mapDestination(destination M.Socksaddr) M.Socksaddr {
	if destination.IsIP() {
		destination.Addr = d.mapper.synthesize(destination.Addr)
	}
	return destination
}

func (d *xlat464Dialer) mapDestinationAddresses(destination M.Socksaddr, destinationAddresses []netip.Addr) ([]netip.Addr, error) {
	mappedAddresses := d.mapper.synthesizeIPv4Addresses(destinationAddresses)
	if len(destinationAddresses) > 0 && len(mappedAddresses) == 0 {
		return nil, E.New("xlat464: no IPv4 destination addresses")
	}
	if len(mappedAddresses) == 0 && !destination.IsIP() {
		return nil, E.New("xlat464: no destination addresses")
	}
	return mappedAddresses, nil
}

func (d *xlat464Dialer) wrapPacketConn(conn net.PacketConn) net.PacketConn {
	packetConn, isNetPacketConn := conn.(N.NetPacketConn)
	if !isNetPacketConn {
		packetConn = bufio.NewPacketConn(conn)
	}
	return &xlat464PacketConn{NetPacketConn: packetConn, mapper: d.mapper}
}

type xlat464PacketConn struct {
	N.NetPacketConn
	mapper xlat464AddressMapper
}

func (c *xlat464PacketConn) ReadFrom(p []byte) (int, net.Addr, error) {
	n, address, err := c.NetPacketConn.ReadFrom(p)
	return n, c.reverseNetAddr(address), err
}

func (c *xlat464PacketConn) WriteTo(p []byte, address net.Addr) (int, error) {
	return c.NetPacketConn.WriteTo(p, c.synthesizeNetAddr(address))
}

func (c *xlat464PacketConn) ReadPacket(buffer *B.Buffer) (M.Socksaddr, error) {
	destination, err := c.NetPacketConn.ReadPacket(buffer)
	return c.reverseDestination(destination), err
}

func (c *xlat464PacketConn) WritePacket(buffer *B.Buffer, destination M.Socksaddr) error {
	return c.NetPacketConn.WritePacket(buffer, c.synthesizeDestination(destination))
}

func (c *xlat464PacketConn) synthesizeDestination(destination M.Socksaddr) M.Socksaddr {
	if destination.IsIP() {
		destination.Addr = c.mapper.synthesize(destination.Addr)
	}
	return destination
}

func (c *xlat464PacketConn) reverseDestination(destination M.Socksaddr) M.Socksaddr {
	if destination.IsIP() {
		destination.Addr = c.mapper.reverse(destination.Addr)
	}
	return destination
}

func (c *xlat464PacketConn) synthesizeNetAddr(address net.Addr) net.Addr {
	udpAddress, isUDPAddress := address.(*net.UDPAddr)
	if !isUDPAddress || udpAddress == nil {
		return address
	}
	addrPort := udpAddress.AddrPort()
	mappedAddress := c.mapper.synthesize(addrPort.Addr())
	if mappedAddress == addrPort.Addr() {
		return address
	}
	return net.UDPAddrFromAddrPort(netip.AddrPortFrom(mappedAddress, addrPort.Port()))
}

func (c *xlat464PacketConn) reverseNetAddr(address net.Addr) net.Addr {
	udpAddress, isUDPAddress := address.(*net.UDPAddr)
	if !isUDPAddress || udpAddress == nil {
		return address
	}
	addrPort := udpAddress.AddrPort()
	mappedAddress := c.mapper.reverse(addrPort.Addr())
	if mappedAddress == addrPort.Addr() {
		return address
	}
	return net.UDPAddrFromAddrPort(netip.AddrPortFrom(mappedAddress, addrPort.Port()))
}

var (
	_ dialer.ParallelInterfaceDialer = (*xlat464Dialer)(nil)
	_ dialer.ParallelNetworkDialer   = (*xlat464Dialer)(nil)
	_ N.NetPacketConn                = (*xlat464PacketConn)(nil)
)
