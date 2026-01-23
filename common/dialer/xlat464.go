package dialer

import (
	"context"
	"net"
	"net/netip"
	"time"

	C "github.com/sagernet/sing-box/constant"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ ParallelInterfaceDialer = (*xlat464Dialer)(nil)

type xlat464Dialer struct {
	dialer N.Dialer
	prefix netip.Prefix
}

func NewXLAT464Dialer(dialer N.Dialer, prefix netip.Prefix) ParallelInterfaceDialer {
	parallelDialer := dialer.(ParallelInterfaceDialer)
	return &xlat464Dialer{
		dialer: parallelDialer,
		prefix: prefix,
	}
}

func (d *xlat464Dialer) translateDestination(destination M.Socksaddr) M.Socksaddr {
	// Debug: Check what we received
	// log.Printf("XLAT464: Received destination: %+v, IsIP=%v, IsFqdn=%v, Addr=%v",
	//            destination, destination.IsIP(), destination.IsFqdn(), destination.Addr)

	// Only translate IPv4 addresses
	// Check if we have an IPv4 address (either as Addr or if Fqdn is empty)
	if destination.Addr.IsValid() && destination.Addr.Is4() {
		// Translate IPv4 to IPv6
		translatedAddr := translateIPv4ToIPv6(destination.Addr, d.prefix)
		result := M.SocksaddrFrom(translatedAddr, destination.Port)
		// Debug: Show translation result
		// log.Printf("XLAT464: Translated %v to %v", destination.Addr, translatedAddr)
		return result
	}

	// Pass through domain names, IPv6, or invalid addresses unchanged
	return destination
}

func (d *xlat464Dialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return d.dialer.DialContext(ctx, network, d.translateDestination(destination))
}

func (d *xlat464Dialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return d.dialer.ListenPacket(ctx, d.translateDestination(destination))
}

func (d *xlat464Dialer) DialParallelInterface(ctx context.Context, network string, destination M.Socksaddr, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.Conn, error) {
	parallelDialer := d.dialer.(ParallelInterfaceDialer)
	return parallelDialer.DialParallelInterface(ctx, network, d.translateDestination(destination), strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
}

func (d *xlat464Dialer) ListenSerialInterfacePacket(ctx context.Context, destination M.Socksaddr, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.PacketConn, error) {
	parallelDialer := d.dialer.(ParallelInterfaceDialer)
	return parallelDialer.ListenSerialInterfacePacket(ctx, d.translateDestination(destination), strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
}

func (d *xlat464Dialer) Upstream() any {
	return d.dialer
}

func translateIPv4ToIPv6(ipv4 netip.Addr, prefix netip.Prefix) netip.Addr {
	// Extract IPv4 as 4 bytes
	ipv4Bytes := ipv4.As4()

	// Get prefix bytes (first 96 bits / 12 bytes)
	prefixBytes := prefix.Addr().As16()

	// Embed IPv4 in last 32 bits (bytes 12-15)
	result := prefixBytes
	copy(result[12:16], ipv4Bytes[:])

	return netip.AddrFrom16(result)
}
