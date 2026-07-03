package firefoxvpn

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func RegisterOutbound(registry *outbound.Registry) {
	outbound.Register[option.FirefoxVPNOutboundOptions](registry, C.TypeFirefoxVPN, NewOutbound)
}

type Outbound struct {
	outbound.Adapter
	dependencies []string
}

func NewOutbound(_ context.Context, _ adapter.Router, _ log.StructuredLogger, tag string, options option.FirefoxVPNOutboundOptions) (adapter.Outbound, error) {
	return &Outbound{
		Adapter:      newFirefoxAdapter(tag, options),
		dependencies: newFirefoxDependencies(options),
	}, nil
}

func newFirefoxAdapter(tag string, options option.FirefoxVPNOutboundOptions) outbound.Adapter {
	return outbound.NewAdapter(C.TypeFirefoxVPN, tag, []string{N.NetworkTCP}, newFirefoxDependencies(options))
}

func newFirefoxDependencies(options option.FirefoxVPNOutboundOptions) []string {
	dependencies := make([]string, 0, 2)
	appendDependency := func(tag string) {
		if tag == "" {
			return
		}
		for _, dependency := range dependencies {
			if dependency == tag {
				return
			}
		}
		dependencies = append(dependencies, tag)
	}
	appendDependency(options.DialerOptions.Detour)
	appendDependency(options.APIDetour)
	return dependencies
}

func (o *Outbound) Dependencies() []string {
	return o.dependencies
}

func (o *Outbound) Start() error {
	return nil
}

func (o *Outbound) DialContext(_ context.Context, _ string, _ M.Socksaddr) (net.Conn, error) {
	return nil, E.New("not implemented")
}

func (o *Outbound) ListenPacket(_ context.Context, _ M.Socksaddr) (net.PacketConn, error) {
	return nil, E.New("not implemented")
}
