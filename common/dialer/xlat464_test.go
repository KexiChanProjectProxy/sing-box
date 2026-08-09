package dialer

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/service"

	"github.com/miekg/dns"
)

func TestXLAT464Domain(t *testing.T) {
	// Given
	dnsRouter := &xlat464TestDNSRouter{addresses: []netip.Addr{netip.MustParseAddr("192.0.2.1")}}
	ctx := service.ContextWith[adapter.DNSTransportManager](context.Background(), &xlat464TestDNSTransportManager{})
	ctx = service.ContextWith[adapter.DNSRouter](ctx, dnsRouter)
	socketDialer := &xlat464TestSocketDialer{}
	wrappedDialer, err := NewWithOptions(Options{
		Context: ctx,
		Options: option.DialerOptions{
			AbstractDialerOptions: option.AbstractDialerOptions{
				TCPFastOpen: true,
				DomainResolver: &option.DomainResolveOptions{
					Server:   "resolver",
					Strategy: option.DomainStrategy(C.DomainStrategyIPv6Only),
				},
			},
		},
		RemoteIsDomain:              true,
		ForceDomainStrategyIPv4Only: true,
		DialerWrapper: func(ParallelInterfaceDialer) ParallelInterfaceDialer {
			return &xlat464TestMapper{dialer: socketDialer}
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// When
	_, _ = wrappedDialer.DialContext(context.Background(), "tcp", M.ParseSocksaddrHostPort("example.com", 443))

	// Then
	if dnsRouter.options.Strategy != C.DomainStrategyIPv4Only {
		t.Fatalf("DNS strategy: got %v, want %v", dnsRouter.options.Strategy, C.DomainStrategyIPv4Only)
	}
	want := M.SocksaddrFrom(netip.MustParseAddr("64:ff9b::c000:201"), 443)
	if socketDialer.destination != want {
		t.Fatalf("socket destination: got %v, want %v", socketDialer.destination, want)
	}
}

type xlat464TestSocketDialer struct {
	destination M.Socksaddr
}

type xlat464TestMapper struct {
	dialer ParallelInterfaceDialer
}

func (d *xlat464TestMapper) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return d.dialer.DialContext(ctx, network, xlat464TestMapDestination(destination))
}

func (d *xlat464TestMapper) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return d.dialer.ListenPacket(ctx, xlat464TestMapDestination(destination))
}

func (d *xlat464TestMapper) DialParallelInterface(ctx context.Context, network string, destination M.Socksaddr, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.Conn, error) {
	return d.dialer.DialParallelInterface(ctx, network, xlat464TestMapDestination(destination), strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
}

func (d *xlat464TestMapper) ListenSerialInterfacePacket(ctx context.Context, destination M.Socksaddr, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.PacketConn, error) {
	return d.dialer.ListenSerialInterfacePacket(ctx, xlat464TestMapDestination(destination), strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
}

func xlat464TestMapDestination(destination M.Socksaddr) M.Socksaddr {
	if destination.Addr == netip.MustParseAddr("192.0.2.1") {
		destination.Addr = netip.MustParseAddr("64:ff9b::c000:201")
	}
	return destination
}

func (d *xlat464TestSocketDialer) DialContext(_ context.Context, _ string, destination M.Socksaddr) (net.Conn, error) {
	d.destination = destination
	return nil, errors.New("socket boundary reached")
}

func (d *xlat464TestSocketDialer) ListenPacket(context.Context, M.Socksaddr) (net.PacketConn, error) {
	return nil, errors.New("unused packet listener")
}

func (d *xlat464TestSocketDialer) DialParallelInterface(ctx context.Context, network string, destination M.Socksaddr, _ *C.NetworkStrategy, _ []C.InterfaceType, _ []C.InterfaceType, _ time.Duration) (net.Conn, error) {
	return d.DialContext(ctx, network, destination)
}

func (d *xlat464TestSocketDialer) ListenSerialInterfacePacket(ctx context.Context, destination M.Socksaddr, _ *C.NetworkStrategy, _ []C.InterfaceType, _ []C.InterfaceType, _ time.Duration) (net.PacketConn, error) {
	return d.ListenPacket(ctx, destination)
}

type xlat464TestDNSRouter struct {
	addresses []netip.Addr
	options   adapter.DNSQueryOptions
}

func (*xlat464TestDNSRouter) Start(adapter.StartStage) error { return nil }
func (*xlat464TestDNSRouter) Close() error                   { return nil }
func (*xlat464TestDNSRouter) Exchange(context.Context, *dns.Msg, adapter.DNSQueryOptions) (*dns.Msg, error) {
	return nil, errors.New("unused DNS exchange")
}
func (r *xlat464TestDNSRouter) ExchangeAsync(ctx context.Context, message *dns.Msg, options adapter.DNSQueryOptions, callback func(*dns.Msg, error)) {
	response, err := r.Exchange(ctx, message, options)
	callback(response, err)
}
func (r *xlat464TestDNSRouter) Lookup(_ context.Context, _ string, options adapter.DNSQueryOptions) ([]netip.Addr, error) {
	r.options = options
	return r.addresses, nil
}
func (*xlat464TestDNSRouter) ClearCache()                                    {}
func (*xlat464TestDNSRouter) LookupReverseMapping(netip.Addr) (string, bool) { return "", false }
func (*xlat464TestDNSRouter) ResetNetwork()                                  {}

type xlat464TestDNSTransportManager struct{}

func (*xlat464TestDNSTransportManager) Start(adapter.StartStage) error { return nil }
func (*xlat464TestDNSTransportManager) Close() error                   { return nil }
func (*xlat464TestDNSTransportManager) Transports() []adapter.DNSTransport {
	return nil
}
func (*xlat464TestDNSTransportManager) Transport(string) (adapter.DNSTransport, bool) {
	return nil, true
}
func (*xlat464TestDNSTransportManager) Default() adapter.DNSTransport { return nil }
func (*xlat464TestDNSTransportManager) FakeIP() adapter.FakeIPTransport {
	return nil
}
func (*xlat464TestDNSTransportManager) Remove(string) error { return nil }
func (*xlat464TestDNSTransportManager) Create(context.Context, log.StructuredLogger, string, string, any) error {
	return nil
}
