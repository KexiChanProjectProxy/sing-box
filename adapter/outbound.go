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

type preferDomainContextKey struct{}

func ContextWithPreferDomain(ctx context.Context, effective bool) context.Context {
	return context.WithValue(ctx, (*preferDomainContextKey)(nil), effective)
}

func PreferDomainFromContext(ctx context.Context) bool {
	value := ctx.Value((*preferDomainContextKey)(nil))
	if value == nil {
		return false
	}
	return value.(bool)
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

type OutboundWithPreferDomain interface {
	Outbound
	PreferDomain() bool
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
