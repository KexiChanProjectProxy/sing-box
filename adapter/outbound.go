package adapter

import (
	"context"
	"net/netip"
	"time"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-tun"
	N "github.com/sagernet/sing/common/network"
)

// PreferDomainConfig holds the resolved prefer_domain configuration for an outbound.
type PreferDomainConfig struct {
	Enabled   bool
	MarkValue uint32
	MarkMask  uint32
}

// PreferDomainOverrider is implemented by outbounds that support the prefer_domain option.
type PreferDomainOverrider interface {
	PreferDomainConfig() *PreferDomainConfig
}

// Note: for proxy protocols, outbound creates early connections by default.

type Outbound interface {
	Type() string
	Tag() string
	Network() []string
	Dependencies() []string
	N.Dialer
}

type OutboundWithPreferredRoutes interface {
	Outbound
	PreferredDomain(domain string) bool
	PreferredAddress(address netip.Addr) bool
}

type DirectRouteOutbound interface {
	Outbound
	NewDirectRouteConnection(metadata InboundContext, routeContext tun.DirectRouteContext, timeout time.Duration) (tun.DirectRouteDestination, error)
}

type OutboundRegistry interface {
	option.OutboundOptionsRegistry
	CreateOutbound(ctx context.Context, router Router, logger log.ContextLogger, tag string, outboundType string, options any) (Outbound, error)
}

type OutboundManager interface {
	Lifecycle
	Outbounds() []Outbound
	Outbound(tag string) (Outbound, bool)
	Default() Outbound
	Remove(tag string) error
	Create(ctx context.Context, router Router, logger log.ContextLogger, tag string, outboundType string, options any) error
}
