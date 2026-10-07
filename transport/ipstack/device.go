// Package ipstack provides a gVisor user-space IP stack that exchanges raw IP
// packets with a tunnel and serves outbound TCP and UDP connections.
package ipstack

import (
	"context"
	"net/netip"
	"time"

	"github.com/sagernet/sing-box/log"
	tun "github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/control"
	N "github.com/sagernet/sing/common/network"
)

const DefaultMTU = 1280

// PacketWriter receives packets leaving the stack. It takes ownership of the
// buffers and must release them.
type PacketWriter func(packetBuffers []*buf.Buffer) error

type Device interface {
	N.Dialer
	Start() error
	UpdateAddresses(addresses []netip.Prefix) error
	SetMTU(mtu uint32)
	MTU() uint32
	Addresses() (inet4Address netip.Addr, inet6Address netip.Addr)
	// WriteInboundBuffers delivers packets received from the tunnel. The caller
	// keeps ownership of the buffers.
	WriteInboundBuffers(packetBuffers []*buf.Buffer) error
	SetPacketWriter(writer PacketWriter)
	Close() error
}

type DeviceOptions struct {
	Context context.Context
	Logger  log.StructuredLogger
	// Handler receives connections initiated from the tunnel side. It is
	// optional; outbound-only users leave it nil.
	Handler         tun.Handler
	UDPTimeout      time.Duration
	UDPMapping      tun.NATMapping
	UDPFiltering    tun.NATFiltering
	UDPNATMax       uint32
	InterfaceFinder control.InterfaceFinder
	MTU             uint32
	Addresses       []netip.Prefix
	// PacketHeadroom is reserved in front of every packet passed to the
	// PacketWriter so tunnels can prepend framing without copying.
	PacketHeadroom int
}

func firstAddresses(addresses []netip.Prefix) (netip.Addr, netip.Addr) {
	var inet4Address netip.Addr
	var inet6Address netip.Addr
	for _, prefix := range addresses {
		if prefix.Addr().Is4() && !inet4Address.IsValid() {
			inet4Address = prefix.Addr()
		} else if prefix.Addr().Is6() && !inet6Address.IsValid() {
			inet6Address = prefix.Addr()
		}
	}
	return inet4Address, inet6Address
}
