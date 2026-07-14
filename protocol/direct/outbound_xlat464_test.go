package direct

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"

	"github.com/miekg/dns"
)

func TestXLAT464Resolver(t *testing.T) {
	// Given
	options := xlat464OutboundTestOptions(C.DomainStrategyIPv6Only)
	dnsRouter := &xlat464OutboundTestDNSRouter{addresses: []netip.Addr{netip.MustParseAddr("192.0.2.1")}}

	// When
	rawOutbound, err := NewOutbound(xlat464OutboundTestContext(dnsRouter), nil, log.NewNOPFactory().Logger(), "direct", options)

	// Then
	if err != nil {
		t.Fatal(err)
	}
	outbound := rawOutbound.(*Outbound)
	resolver, isResolver := outbound.dialer.(dialer.ParallelInterfaceResolveDialer)
	if !isResolver {
		t.Fatal("expected resolve dialer")
	}
	if got := resolver.QueryOptions().Strategy; got != C.DomainStrategyIPv4Only {
		t.Fatalf("DNS strategy: got %v, want %v", got, C.DomainStrategyIPv4Only)
	}
	upstream, isUpstream := outbound.dialer.(interface{ Upstream() any })
	if !isUpstream {
		t.Fatal("expected resolve dialer upstream")
	}
	if _, isXLAT464 := upstream.Upstream().(*xlat464Dialer); !isXLAT464 {
		t.Fatalf("resolver upstream: got %T, want *xlat464Dialer", upstream.Upstream())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _ = outbound.dialer.DialContext(ctx, N.NetworkTCP, M.ParseSocksaddrHostPort("example.com", 443))
	if got := dnsRouter.options.Strategy; got != C.DomainStrategyIPv4Only {
		t.Fatalf("observed DNS strategy: got %v, want %v", got, C.DomainStrategyIPv4Only)
	}
}

func TestXLAT464Disabled(t *testing.T) {
	// Given
	options := xlat464OutboundTestOptions(C.DomainStrategyPreferIPv4)
	options.Xlat464 = nil
	dnsRouter := &xlat464OutboundTestDNSRouter{addresses: []netip.Addr{netip.MustParseAddr("192.0.2.1")}}

	// When
	rawOutbound, err := NewOutbound(xlat464OutboundTestContext(dnsRouter), nil, log.NewNOPFactory().Logger(), "direct", options)

	// Then
	if err != nil {
		t.Fatal(err)
	}
	outbound := rawOutbound.(*Outbound)
	resolver := outbound.dialer.(dialer.ParallelInterfaceResolveDialer)
	if got := resolver.QueryOptions().Strategy; got != C.DomainStrategyPreferIPv4 {
		t.Fatalf("DNS strategy: got %v, want %v", got, C.DomainStrategyPreferIPv4)
	}
	upstream := outbound.dialer.(interface{ Upstream() any })
	if _, isXLAT464 := upstream.Upstream().(*xlat464Dialer); isXLAT464 {
		t.Fatal("unexpected xlat464 wrapper")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _ = outbound.dialer.DialContext(ctx, N.NetworkTCP, M.ParseSocksaddrHostPort("example.com", 443))
	if got := dnsRouter.options.Strategy; got != C.DomainStrategyPreferIPv4 {
		t.Fatalf("observed DNS strategy: got %v, want %v", got, C.DomainStrategyPreferIPv4)
	}
}

func TestXLAT464InvalidPrefixReturnsError(t *testing.T) {
	// Given
	options := xlat464OutboundTestOptions(C.DomainStrategyAsIS)
	invalidPrefix := badoption.Prefix(netip.MustParsePrefix("2001:db8::/64"))
	options.Xlat464 = &option.Xlat464Options{Prefix: &invalidPrefix}

	// When
	_, err := NewOutbound(xlat464OutboundTestContext(nil), nil, log.NewNOPFactory().Logger(), "direct", options)

	// Then
	if err == nil {
		t.Fatal("expected invalid XLAT464 prefix error")
	}
}

func xlat464OutboundTestOptions(strategy C.DomainStrategy) option.DirectOutboundOptions {
	prefix := badoption.Prefix(netip.MustParsePrefix("64:ff9b::/96"))
	return option.DirectOutboundOptions{
		DialerOptions: option.DialerOptions{
			DomainResolver: &option.DomainResolveOptions{
				Server:   "resolver",
				Strategy: option.DomainStrategy(strategy),
			},
		},
		Xlat464: &option.Xlat464Options{Prefix: &prefix},
	}
}

func xlat464OutboundTestContext(dnsRouter adapter.DNSRouter) context.Context {
	ctx := service.ContextWith[adapter.DNSTransportManager](context.Background(), &xlat464OutboundTestDNSTransportManager{})
	if dnsRouter != nil {
		ctx = service.ContextWith[adapter.DNSRouter](ctx, dnsRouter)
	}
	return ctx
}

type xlat464OutboundTestDNSTransportManager struct{}

func (*xlat464OutboundTestDNSTransportManager) Start(adapter.StartStage) error { return nil }
func (*xlat464OutboundTestDNSTransportManager) Close() error                   { return nil }
func (*xlat464OutboundTestDNSTransportManager) Transports() []adapter.DNSTransport {
	return nil
}
func (*xlat464OutboundTestDNSTransportManager) Transport(string) (adapter.DNSTransport, bool) {
	return nil, true
}
func (*xlat464OutboundTestDNSTransportManager) Default() adapter.DNSTransport { return nil }
func (*xlat464OutboundTestDNSTransportManager) FakeIP() adapter.FakeIPTransport {
	return nil
}
func (*xlat464OutboundTestDNSTransportManager) Remove(string) error { return nil }
func (*xlat464OutboundTestDNSTransportManager) Create(context.Context, log.StructuredLogger, string, string, any) error {
	return nil
}

type xlat464OutboundTestDNSRouter struct {
	addresses []netip.Addr
	options   adapter.DNSQueryOptions
}

func (*xlat464OutboundTestDNSRouter) Start(adapter.StartStage) error { return nil }
func (*xlat464OutboundTestDNSRouter) Close() error                   { return nil }
func (*xlat464OutboundTestDNSRouter) Exchange(context.Context, *dns.Msg, adapter.DNSQueryOptions) (*dns.Msg, error) {
	return nil, errors.New("unused DNS exchange")
}
func (r *xlat464OutboundTestDNSRouter) Lookup(_ context.Context, _ string, options adapter.DNSQueryOptions) ([]netip.Addr, error) {
	r.options = options
	return r.addresses, nil
}
func (*xlat464OutboundTestDNSRouter) ClearCache() {}
func (*xlat464OutboundTestDNSRouter) LookupReverseMapping(netip.Addr) (string, bool) {
	return "", false
}
func (*xlat464OutboundTestDNSRouter) ResetNetwork() {}
