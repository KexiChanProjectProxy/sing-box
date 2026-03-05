package dialer

import (
	"context"
	"hash/fnv"
	"net"
	"net/netip"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/listener"
	C "github.com/sagernet/sing-box/constant"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ ParallelInterfaceDialer = (*perUserIPv6Dialer)(nil)

type perUserIPv6Dialer struct {
	inner  *DefaultDialer
	prefix netip.Prefix
}

func NewPerUserIPv6Dialer(inner *DefaultDialer, prefix netip.Prefix) ParallelInterfaceDialer {
	return &perUserIPv6Dialer{
		inner:  inner,
		prefix: prefix,
	}
}

// computeIPv6Address generates a deterministic IPv6 address from user and source IP
func (d *perUserIPv6Dialer) computeIPv6Address(ctx context.Context) netip.Addr {
	metadata := adapter.ContextFrom(ctx)
	if metadata == nil || metadata.User == "" {
		// No user authenticated, return zero address (fallback to default)
		return netip.Addr{}
	}

	// Create hash input from user and source IP
	hasher := fnv.New128a()
	hasher.Write([]byte(metadata.User))
	if metadata.Source.Addr.IsValid() {
		hasher.Write(metadata.Source.Addr.AsSlice())
	}
	hashBytes := hasher.Sum(nil) // 16 bytes

	// Get prefix bytes
	prefixAddr := d.prefix.Addr().As16()
	prefixBits := d.prefix.Bits()

	// Compute the IPv6 address by combining prefix and hash
	var result [16]byte
	copy(result[:], prefixAddr[:])

	// Write hash bytes into the host portion
	// Calculate how many bytes are in the prefix
	prefixBytes := prefixBits / 8
	remainingBits := prefixBits % 8

	if remainingBits == 0 {
		// Byte-aligned prefix, just copy hash into host portion
		copy(result[prefixBytes:], hashBytes[:16-prefixBytes])
	} else {
		// Not byte-aligned, need to preserve prefix bits in the boundary byte
		mask := byte(0xFF >> remainingBits)
		result[prefixBytes] = (result[prefixBytes] &^ mask) | (hashBytes[0] & mask)
		copy(result[prefixBytes+1:], hashBytes[1:16-prefixBytes])
	}

	return netip.AddrFrom16(result)
}

func (d *perUserIPv6Dialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	// Only apply per-user binding for IPv6 destinations
	if destination.IsIPv6() {
		addr := d.computeIPv6Address(ctx)
		if addr.IsValid() {
			// Create a per-connection dialer with the computed local address
			var dialer net.Dialer
			if N.NetworkName(network) == N.NetworkTCP {
				dialer = d.inner.TCPDialer6()
				dialer.LocalAddr = &net.TCPAddr{IP: addr.AsSlice()}
			} else {
				dialer = d.inner.UDPDialer6()
				dialer.LocalAddr = &net.UDPAddr{IP: addr.AsSlice()}
			}

			// Dial using the per-connection dialer
			return d.inner.trackConn(listener.ListenNetworkNamespace[net.Conn](d.inner.NetNs(), func() (net.Conn, error) {
				return dialer.DialContext(ctx, network, destination.String())
			}))
		}
	}

	// Fallback to inner dialer for IPv4 or when no user info
	return d.inner.DialContext(ctx, network, destination)
}

func (d *perUserIPv6Dialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	// Only apply per-user binding for IPv6 destinations
	if destination.IsIPv6() {
		addr := d.computeIPv6Address(ctx)
		if addr.IsValid() {
			// Create a per-connection listener config with the computed local address
			listenerConfig := d.inner.UDPListenerConfig()
			localAddr := M.SocksaddrFrom(addr, 0).String()

			return d.inner.trackPacketConn(listener.ListenNetworkNamespace[net.PacketConn](d.inner.NetNs(), func() (net.PacketConn, error) {
				return listenerConfig.ListenPacket(ctx, N.NetworkUDP, localAddr)
			}))
		}
	}

	// Fallback to inner dialer
	return d.inner.ListenPacket(ctx, destination)
}

func (d *perUserIPv6Dialer) DialParallelInterface(ctx context.Context, network string, destination M.Socksaddr, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.Conn, error) {
	// Only apply per-user binding for IPv6 destinations
	if destination.IsIPv6() {
		addr := d.computeIPv6Address(ctx)
		if addr.IsValid() {
			// Note: DialParallelInterface expects DefaultDialer which manages its own LocalAddr
			// For simplicity, we delegate to the inner dialer and let it handle parallel dialing
			// The per-user IPv6 feature may not fully integrate with parallel interface dialing
			// as it requires modifying the dialer's LocalAddr which conflicts with interface binding
		}
	}

	// Delegate to inner dialer
	return d.inner.DialParallelInterface(ctx, network, destination, strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
}

func (d *perUserIPv6Dialer) ListenSerialInterfacePacket(ctx context.Context, destination M.Socksaddr, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.PacketConn, error) {
	// Only apply per-user binding for IPv6 destinations
	if destination.IsIPv6() {
		addr := d.computeIPv6Address(ctx)
		if addr.IsValid() {
			// Similar to DialParallelInterface, interface-specific packet listening
			// may conflict with per-user address binding
		}
	}

	// Delegate to inner dialer
	return d.inner.ListenSerialInterfacePacket(ctx, destination, strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
}

func (d *perUserIPv6Dialer) Upstream() any {
	return d.inner
}
