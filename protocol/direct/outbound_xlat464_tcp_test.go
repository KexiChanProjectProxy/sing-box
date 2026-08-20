package direct

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func TestXLAT464TCP(t *testing.T) {
	for _, destination := range []M.Socksaddr{
		M.SocksaddrFrom(netip.MustParseAddr("192.0.2.1"), 443),
		M.SocksaddrFrom(netip.MustParseAddr("::ffff:192.0.2.1"), 443),
	} {
		t.Run(destination.String(), func(t *testing.T) {
			// Given
			base := &xlat464OutboundCaptureDialer{}
			outbound := newXLAT464TCPTestOutbound(t, base, true)

			// When
			_, err := outbound.DialContext(context.Background(), N.NetworkTCP, destination)

			// Then
			if !errors.Is(err, errXLAT464OutboundTestDial) {
				t.Fatalf("DialContext error: got %v, want %v", err, errXLAT464OutboundTestDial)
			}
			xlat464OutboundExpectDestinations(t, base, []M.Socksaddr{M.SocksaddrFrom(netip.MustParseAddr("64:ff9b::c000:201"), 443)})
		})
	}
}

func TestXLAT464PreResolved(t *testing.T) {
	strategy := C.NetworkStrategy(42)
	interfaceTypes := []C.InterfaceType{1}
	fallbackInterfaceTypes := []C.InterfaceType{2}
	for _, test := range []struct {
		name string
		dial func(*Outbound) error
		want xlat464OutboundDialCall
	}{
		{
			name: "parallel",
			dial: func(outbound *Outbound) error {
				_, err := outbound.DialParallel(context.Background(), N.NetworkTCP, M.ParseSocksaddrHostPort("example.com", 443), []netip.Addr{
					netip.MustParseAddr("192.0.2.1"),
					netip.MustParseAddr("2001:db8::1"),
				})
				return err
			},
			want: xlat464OutboundDialCall{fallbackDelay: 23 * time.Millisecond},
		},
		{
			name: "parallel network",
			dial: func(outbound *Outbound) error {
				_, err := outbound.DialParallelNetwork(context.Background(), N.NetworkTCP, M.ParseSocksaddrHostPort("example.com", 443), []netip.Addr{
					netip.MustParseAddr("192.0.2.1"),
					netip.MustParseAddr("2001:db8::1"),
				}, &strategy, interfaceTypes, fallbackInterfaceTypes, 37*time.Millisecond)
				return err
			},
			want: xlat464OutboundDialCall{
				strategy:               &strategy,
				interfaceTypes:         interfaceTypes,
				fallbackInterfaceTypes: fallbackInterfaceTypes,
				fallbackDelay:          37 * time.Millisecond,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			base := &xlat464OutboundCaptureDialer{}
			outbound := newXLAT464TCPTestOutbound(t, base, true)

			// When
			err := test.dial(outbound)

			// Then
			if !errors.Is(err, errXLAT464OutboundTestDial) {
				t.Fatalf("dial error: got %v, want %v", err, errXLAT464OutboundTestDial)
			}
			calls := base.capturedCalls()
			if len(calls) != 1 {
				t.Fatalf("captured calls: got %v, want one", calls)
			}
			want := test.want
			want.destination = M.SocksaddrFrom(netip.MustParseAddr("64:ff9b::c000:201"), 443)
			if !reflect.DeepEqual(calls[0], want) {
				t.Fatalf("captured call: got %#v, want %#v", calls[0], want)
			}
		})
	}
}

func TestXLAT464DropsAAAA(t *testing.T) {
	// Given
	base := &xlat464OutboundCaptureDialer{}
	outbound := newXLAT464TCPTestOutbound(t, base, true)

	// When
	_, err := outbound.DialParallel(context.Background(), N.NetworkTCP, M.ParseSocksaddrHostPort("example.com", 443), []netip.Addr{netip.MustParseAddr("2001:db8::1")})

	// Then
	if err == nil {
		t.Fatal("expected IPv6-only address list rejection")
	}
	xlat464OutboundExpectDestinations(t, base, nil)
}

func TestXLAT464RejectsIPv6Native(t *testing.T) {
	// Given
	base := &xlat464OutboundCaptureDialer{}
	outbound := newXLAT464TCPTestOutbound(t, base, true)
	destination := M.SocksaddrFrom(netip.MustParseAddr("2001:db8::1"), 443)

	// When
	_, err := outbound.DialContext(context.Background(), N.NetworkTCP, destination)

	// Then
	if err == nil {
		t.Fatal("expected native IPv6 destination rejection")
	}
	xlat464OutboundExpectDestinations(t, base, nil)
}

func TestXLAT464DisabledTCP(t *testing.T) {
	// Given
	base := &xlat464OutboundCaptureDialer{}
	outbound := newXLAT464TCPTestOutbound(t, base, false)
	addresses := []netip.Addr{netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("2001:db8::1")}

	// When
	_, err := outbound.DialParallel(context.Background(), N.NetworkTCP, M.ParseSocksaddrHostPort("example.com", 443), addresses)

	// Then
	if !errors.Is(err, errXLAT464OutboundTestDial) {
		t.Fatalf("DialParallel error: got %v, want %v", err, errXLAT464OutboundTestDial)
	}
	xlat464OutboundExpectDestinations(t, base, []M.Socksaddr{
		M.SocksaddrFrom(netip.MustParseAddr("192.0.2.1"), 443),
		M.SocksaddrFrom(netip.MustParseAddr("2001:db8::1"), 443),
	})
}

func TestXLAT464DomainAOnly(t *testing.T) {
	// Given
	base := &xlat464OutboundCaptureDialer{}
	xlat464Dialer, err := newXLAT464Dialer(base, xlat464TestOptions("64:ff9b::/96"))
	if err != nil {
		t.Fatal(err)
	}
	dnsRouter := &xlat464OutboundTestDNSRouter{addresses: []netip.Addr{netip.MustParseAddr("192.0.2.1")}}
	resolver := dialer.NewResolveDialer(xlat464OutboundTestContext(dnsRouter), xlat464Dialer, true, "", adapter.DNSQueryOptions{Strategy: C.DomainStrategyIPv4Only}, 23*time.Millisecond)
	parallelResolver, isParallelResolver := resolver.(dialer.ParallelInterfaceDialer)
	if !isParallelResolver {
		t.Fatal("expected parallel resolve dialer")
	}
	outbound := &Outbound{logger: log.NewNOPFactory().Logger(), dialer: parallelResolver}

	// When
	_, err = outbound.DialContext(context.Background(), N.NetworkTCP, M.ParseSocksaddrHostPort("example.com", 443))

	// Then
	if !errors.Is(err, errXLAT464OutboundTestDial) {
		t.Fatalf("DialContext error: got %v, want %v", err, errXLAT464OutboundTestDial)
	}
	xlat464OutboundExpectDestinations(t, base, []M.Socksaddr{M.SocksaddrFrom(netip.MustParseAddr("64:ff9b::c000:201"), 443)})
}

func newXLAT464TCPTestOutbound(t *testing.T, base *xlat464OutboundCaptureDialer, enabled bool) *Outbound {
	t.Helper()
	var outboundDialer dialer.ParallelInterfaceDialer = base
	var mapper *xlat464AddressMapper
	if enabled {
		addressMapper, err := newXLAT464AddressMapper(xlat464TestOptions("64:ff9b::/96"))
		if err != nil {
			t.Fatal(err)
		}
		mapper = &addressMapper
		xlat464Dialer, err := newXLAT464Dialer(base, xlat464TestOptions("64:ff9b::/96"))
		if err != nil {
			t.Fatal(err)
		}
		outboundDialer = xlat464OutboundTestParallelDialer{xlat464Dialer}
	}
	return &Outbound{logger: log.NewNOPFactory().Logger(), dialer: outboundDialer, xlat464: mapper, fallbackDelay: 23 * time.Millisecond}
}

func xlat464OutboundExpectDestinations(t *testing.T, dialer *xlat464OutboundCaptureDialer, want []M.Socksaddr) {
	t.Helper()
	calls := dialer.capturedCalls()
	got := make([]M.Socksaddr, len(calls))
	for index, call := range calls {
		got[index] = call.destination
	}
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("captured destinations: got %v, want %v", got, want)
	}
}

var errXLAT464OutboundTestDial = errors.New("xlat464 outbound test dial")

type xlat464OutboundDialCall struct {
	destination            M.Socksaddr
	strategy               *C.NetworkStrategy
	interfaceTypes         []C.InterfaceType
	fallbackInterfaceTypes []C.InterfaceType
	fallbackDelay          time.Duration
}

type xlat464OutboundCaptureDialer struct {
	mutex sync.Mutex
	calls []xlat464OutboundDialCall
}

func (d *xlat464OutboundCaptureDialer) DialContext(_ context.Context, _ string, destination M.Socksaddr) (net.Conn, error) {
	d.record(destination, nil, nil, nil, 0)
	return nil, errXLAT464OutboundTestDial
}

func (d *xlat464OutboundCaptureDialer) ListenPacket(_ context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	d.record(destination, nil, nil, nil, 0)
	return nil, errXLAT464OutboundTestDial
}

func (d *xlat464OutboundCaptureDialer) DialParallelInterface(_ context.Context, _ string, destination M.Socksaddr, strategy *C.NetworkStrategy, interfaceTypes []C.InterfaceType, fallbackInterfaceTypes []C.InterfaceType, fallbackDelay time.Duration) (net.Conn, error) {
	d.record(destination, strategy, interfaceTypes, fallbackInterfaceTypes, fallbackDelay)
	return nil, errXLAT464OutboundTestDial
}

func (d *xlat464OutboundCaptureDialer) ListenSerialInterfacePacket(_ context.Context, destination M.Socksaddr, strategy *C.NetworkStrategy, interfaceTypes []C.InterfaceType, fallbackInterfaceTypes []C.InterfaceType, fallbackDelay time.Duration) (net.PacketConn, error) {
	d.record(destination, strategy, interfaceTypes, fallbackInterfaceTypes, fallbackDelay)
	return nil, errXLAT464OutboundTestDial
}

func (d *xlat464OutboundCaptureDialer) record(destination M.Socksaddr, strategy *C.NetworkStrategy, interfaceTypes []C.InterfaceType, fallbackInterfaceTypes []C.InterfaceType, fallbackDelay time.Duration) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.calls = append(d.calls, xlat464OutboundDialCall{destination, strategy, interfaceTypes, fallbackInterfaceTypes, fallbackDelay})
}

func (d *xlat464OutboundCaptureDialer) capturedCalls() []xlat464OutboundDialCall {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	return append([]xlat464OutboundDialCall(nil), d.calls...)
}

type xlat464OutboundTestParallelDialer struct {
	dialer.ParallelInterfaceDialer
}

var _ dialer.ParallelInterfaceDialer = (*xlat464OutboundCaptureDialer)(nil)
