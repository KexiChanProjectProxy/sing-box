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
	allowIPv6   bool
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
	return xlat464AddressMapper{prefix: prefix, prefixBytes: prefix.Addr().As16(), allowIPv6: options.AllowIPv6}, nil
}

func (m xlat464AddressMapper) mapAddress(address netip.Addr) (netip.Addr, error) {
	if address.Is4() || address.Is4In6() {
		return m.synthesize(address), nil
	}
	if !address.Is6() || m.prefix.Contains(address) || m.allowIPv6 {
		return address, nil
	}
	return netip.Addr{}, E.New("xlat464: native IPv6 destination outside the configured prefix is disabled")
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

func (m xlat464AddressMapper) mapAddresses(addresses []netip.Addr) []netip.Addr {
	mappedAddresses := make([]netip.Addr, 0, len(addresses))
	for _, address := range addresses {
		if address.Is4() || address.Is4In6() {
			mappedAddresses = append(mappedAddresses, m.synthesize(address))
		} else if address.Is6() && (m.prefix.Contains(address) || m.allowIPv6) {
			mappedAddresses = append(mappedAddresses, address)
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
	mappedDestination, err := d.mapDestination(destination)
	if err != nil {
		return nil, err
	}
	return d.dialer.DialContext(ctx, network, mappedDestination)
}

func (d *xlat464Dialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	mappedDestination, err := d.mapDestination(destination)
	if err != nil {
		return nil, err
	}
	conn, err := d.dialer.ListenPacket(ctx, mappedDestination)
	if err != nil {
		return nil, err
	}
	return d.wrapPacketConn(conn), nil
}

func (d *xlat464Dialer) DialParallelInterface(ctx context.Context, network string, destination M.Socksaddr, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.Conn, error) {
	mappedDestination, err := d.mapDestination(destination)
	if err != nil {
		return nil, err
	}
	return d.dialer.DialParallelInterface(ctx, network, mappedDestination, strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
}

func (d *xlat464Dialer) ListenSerialInterfacePacket(ctx context.Context, destination M.Socksaddr, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.PacketConn, error) {
	mappedDestination, err := d.mapDestination(destination)
	if err != nil {
		return nil, err
	}
	conn, err := d.dialer.ListenSerialInterfacePacket(ctx, mappedDestination, strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
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
	mappedDestination, err := d.mapDestination(destination)
	if err != nil {
		return nil, err
	}
	return dialer.DialParallelNetwork(ctx, d.dialer, network, mappedDestination, mappedAddresses, true, strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
}

func (d *xlat464Dialer) ListenSerialNetworkPacket(ctx context.Context, destination M.Socksaddr, destinationAddresses []netip.Addr, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.PacketConn, netip.Addr, error) {
	mappedAddresses, err := d.mapDestinationAddresses(destination, destinationAddresses)
	if err != nil {
		return nil, netip.Addr{}, err
	}
	mappedDestination, err := d.mapDestination(destination)
	if err != nil {
		return nil, netip.Addr{}, err
	}
	conn, mappedDestinationAddress, err := dialer.ListenSerialNetworkPacket(ctx, d.dialer, mappedDestination, mappedAddresses, strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
	if err != nil {
		return nil, netip.Addr{}, err
	}
	return d.wrapPacketConn(conn), d.mapper.reverse(mappedDestinationAddress), nil
}

func (d *xlat464Dialer) mapDestination(destination M.Socksaddr) (M.Socksaddr, error) {
	if destination.IsIP() {
		mappedAddress, err := d.mapper.mapAddress(destination.Addr)
		if err != nil {
			return M.Socksaddr{}, err
		}
		destination.Addr = mappedAddress
	}
	return destination, nil
}

func (d *xlat464Dialer) mapDestinationAddresses(destination M.Socksaddr, destinationAddresses []netip.Addr) ([]netip.Addr, error) {
	mappedAddresses := d.mapper.mapAddresses(destinationAddresses)
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
	mappedAddress, err := c.synthesizeNetAddr(address)
	if err != nil {
		return 0, err
	}
	return c.NetPacketConn.WriteTo(p, mappedAddress)
}

func (c *xlat464PacketConn) ReadPacket(buffer *B.Buffer) (M.Socksaddr, error) {
	destination, err := c.NetPacketConn.ReadPacket(buffer)
	return c.reverseDestination(destination), err
}

func (c *xlat464PacketConn) WritePacket(buffer *B.Buffer, destination M.Socksaddr) error {
	mappedDestination, err := c.synthesizeDestination(destination)
	if err != nil {
		return err
	}
	return c.NetPacketConn.WritePacket(buffer, mappedDestination)
}

func (c *xlat464PacketConn) synthesizeDestination(destination M.Socksaddr) (M.Socksaddr, error) {
	if destination.IsIP() {
		mappedAddress, err := c.mapper.mapAddress(destination.Addr)
		if err != nil {
			return M.Socksaddr{}, err
		}
		destination.Addr = mappedAddress
	}
	return destination, nil
}

func (c *xlat464PacketConn) reverseDestination(destination M.Socksaddr) M.Socksaddr {
	if destination.IsIP() {
		destination.Addr = c.mapper.reverse(destination.Addr)
	}
	return destination
}

func (c *xlat464PacketConn) synthesizeNetAddr(address net.Addr) (net.Addr, error) {
	udpAddress, isUDPAddress := address.(*net.UDPAddr)
	if !isUDPAddress || udpAddress == nil {
		return address, nil
	}
	addrPort := udpAddress.AddrPort()
	mappedAddress, err := c.mapper.mapAddress(addrPort.Addr())
	if err != nil {
		return nil, err
	}
	if mappedAddress == addrPort.Addr() {
		return address, nil
	}
	return net.UDPAddrFromAddrPort(netip.AddrPortFrom(mappedAddress, addrPort.Port())), nil
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
