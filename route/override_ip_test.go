package route

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/metadata"
	"github.com/stretchr/testify/require"
)

type mockOverrideIPOutbound struct {
	tag        string
	overrideIP *option.OverrideIPOptions
}

func (m *mockOverrideIPOutbound) Type() string      { return "mock" }
func (m *mockOverrideIPOutbound) Tag() string       { return m.tag }
func (m *mockOverrideIPOutbound) Network() []string { return nil }
func (m *mockOverrideIPOutbound) Dependencies() []string {
	return nil
}
func (m *mockOverrideIPOutbound) OverrideIP() *option.OverrideIPOptions { return m.overrideIP }
func (m *mockOverrideIPOutbound) DialContext(ctx context.Context, network string, destination metadata.Socksaddr) (net.Conn, error) {
	return nil, nil
}
func (m *mockOverrideIPOutbound) ListenPacket(ctx context.Context, destination metadata.Socksaddr) (net.PacketConn, error) {
	return nil, nil
}

type mockLookuper struct {
	domain  string
	options adapter.DNSQueryOptions
	addrs   []netip.Addr
	err     error
	called  bool
}

func (m *mockLookuper) Lookup(ctx context.Context, domain string, options adapter.DNSQueryOptions) ([]netip.Addr, error) {
	m.called = true
	m.domain = domain
	m.options = options
	return m.addrs, m.err
}

type mockTransportFinder struct {
	wanted string
	loaded bool
}

func (m *mockTransportFinder) Transport(tag string) (adapter.DNSTransport, bool) {
	m.wanted = tag
	return nil, m.loaded
}

func ipv4OnlyOptions() *option.OverrideIPOptions {
	return &option.OverrideIPOptions{Strategy: option.DomainStrategy(C.DomainStrategyIPv4Only)}
}

func TestApplyOverrideIP(t *testing.T) {
	t.Parallel()

	original := metadata.SocksaddrFrom(netip.MustParseAddr("1.2.3.4"), 443)
	v4 := netip.MustParseAddr("9.9.9.9")
	v6 := netip.MustParseAddr("2001:db8::1")

	t.Run("rewrites sniffed domain to resolved IP", func(t *testing.T) {
		lookuper := &mockLookuper{addrs: []netip.Addr{v4, v6}}
		metadata := adapter.InboundContext{
			Protocol:    C.ProtocolTLS,
			Domain:      "example.com",
			Destination: original,
		}
		outbound := &mockOverrideIPOutbound{overrideIP: ipv4OnlyOptions()}

		ctx, err := ApplyOverrideIP(context.Background(), &metadata, outbound, lookuper, nil)
		require.NoError(t, err)
		require.True(t, lookuper.called)
		require.Equal(t, "example.com", lookuper.domain)
		require.Equal(t, C.DomainStrategyIPv4Only, lookuper.options.Strategy)
		require.Nil(t, lookuper.options.Transport)
		require.Equal(t, v4, metadata.Destination.Addr)
		require.Equal(t, uint16(443), metadata.Destination.Port)
		require.Empty(t, metadata.Destination.Fqdn)
		require.Equal(t, []netip.Addr{v4, v6}, metadata.DestinationAddresses)
		require.True(t, adapter.OverrideIPAppliedFromContext(ctx))
	})

	t.Run("skips when not sniffed", func(t *testing.T) {
		lookuper := &mockLookuper{addrs: []netip.Addr{v4}}
		metadata := adapter.InboundContext{
			Domain:      "example.com",
			Destination: original,
		}
		outbound := &mockOverrideIPOutbound{overrideIP: ipv4OnlyOptions()}

		_, err := ApplyOverrideIP(context.Background(), &metadata, outbound, lookuper, nil)
		require.NoError(t, err)
		require.False(t, lookuper.called)
		require.Equal(t, original, metadata.Destination)
		require.Empty(t, metadata.DestinationAddresses)
	})

	t.Run("skips unsupported protocol", func(t *testing.T) {
		lookuper := &mockLookuper{addrs: []netip.Addr{v4}}
		metadata := adapter.InboundContext{
			Protocol:    C.ProtocolDNS,
			Domain:      "example.com",
			Destination: original,
		}
		outbound := &mockOverrideIPOutbound{overrideIP: ipv4OnlyOptions()}

		_, err := ApplyOverrideIP(context.Background(), &metadata, outbound, lookuper, nil)
		require.NoError(t, err)
		require.False(t, lookuper.called)
		require.Equal(t, original, metadata.Destination)
	})

	t.Run("uses IP literal without lookup", func(t *testing.T) {
		lookuper := &mockLookuper{addrs: []netip.Addr{v4}}
		literal := netip.MustParseAddr("8.8.8.8")
		metadata := adapter.InboundContext{
			Protocol:    C.ProtocolHTTP,
			Domain:      literal.String(),
			Destination: original,
		}
		outbound := &mockOverrideIPOutbound{overrideIP: ipv4OnlyOptions()}

		_, err := ApplyOverrideIP(context.Background(), &metadata, outbound, lookuper, nil)
		require.NoError(t, err)
		require.False(t, lookuper.called)
		require.Equal(t, literal, metadata.Destination.Addr)
		require.Equal(t, []netip.Addr{literal}, metadata.DestinationAddresses)
	})

	t.Run("skips IP literal that does not match strategy", func(t *testing.T) {
		lookuper := &mockLookuper{addrs: []netip.Addr{v4}}
		metadata := adapter.InboundContext{
			Protocol:    C.ProtocolTLS,
			Domain:      v6.String(),
			Destination: original,
		}
		outbound := &mockOverrideIPOutbound{overrideIP: ipv4OnlyOptions()}

		_, err := ApplyOverrideIP(context.Background(), &metadata, outbound, lookuper, nil)
		require.NoError(t, err)
		require.False(t, lookuper.called)
		require.Equal(t, original, metadata.Destination)
		require.Empty(t, metadata.DestinationAddresses)
	})

	t.Run("lookup error fails the connection", func(t *testing.T) {
		lookuper := &mockLookuper{err: errors.New("nxdomain")}
		metadata := adapter.InboundContext{
			Protocol:    C.ProtocolQUIC,
			Domain:      "missing.example",
			Destination: original,
		}
		outbound := &mockOverrideIPOutbound{overrideIP: ipv4OnlyOptions()}

		_, err := ApplyOverrideIP(context.Background(), &metadata, outbound, lookuper, nil)
		require.ErrorContains(t, err, "nxdomain")
		require.Equal(t, original, metadata.Destination)
	})

	t.Run("empty lookup result fails", func(t *testing.T) {
		lookuper := &mockLookuper{}
		metadata := adapter.InboundContext{
			Protocol:    C.ProtocolTLS,
			Domain:      "empty.example",
			Destination: original,
		}
		outbound := &mockOverrideIPOutbound{overrideIP: ipv4OnlyOptions()}

		_, err := ApplyOverrideIP(context.Background(), &metadata, outbound, lookuper, nil)
		require.ErrorContains(t, err, "empty override_ip result")
	})

	t.Run("unknown DNS server fails", func(t *testing.T) {
		lookuper := &mockLookuper{addrs: []netip.Addr{v4}}
		finder := &mockTransportFinder{loaded: false}
		metadata := adapter.InboundContext{
			Protocol:    C.ProtocolTLS,
			Domain:      "example.com",
			Destination: original,
		}
		outbound := &mockOverrideIPOutbound{overrideIP: &option.OverrideIPOptions{
			Server:   "missing",
			Strategy: option.DomainStrategy(C.DomainStrategyPreferIPv4),
		}}

		_, err := ApplyOverrideIP(context.Background(), &metadata, outbound, lookuper, finder)
		require.ErrorContains(t, err, "DNS server not found")
		require.Equal(t, "missing", finder.wanted)
		require.False(t, lookuper.called)
	})

	t.Run("inherited group options apply to child without own setting", func(t *testing.T) {
		lookuper := &mockLookuper{addrs: []netip.Addr{v4}}
		metadata := adapter.InboundContext{
			Protocol:    C.ProtocolTLS,
			Domain:      "example.com",
			Destination: original,
		}
		child := &mockOverrideIPOutbound{tag: "child"}
		ctx := adapter.ContextWithOverrideIP(context.Background(), ipv4OnlyOptions())

		_, err := ApplyOverrideIP(ctx, &metadata, child, lookuper, nil)
		require.NoError(t, err)
		require.True(t, lookuper.called)
		require.Equal(t, v4, metadata.Destination.Addr)
	})

	t.Run("is idempotent", func(t *testing.T) {
		lookuper := &mockLookuper{addrs: []netip.Addr{v4}}
		metadata := adapter.InboundContext{
			Protocol:    C.ProtocolTLS,
			Domain:      "example.com",
			Destination: original,
		}
		outbound := &mockOverrideIPOutbound{overrideIP: ipv4OnlyOptions()}

		ctx, err := ApplyOverrideIP(context.Background(), &metadata, outbound, lookuper, nil)
		require.NoError(t, err)
		require.True(t, lookuper.called)

		lookuper.called = false
		_, err = ApplyOverrideIP(ctx, &metadata, outbound, lookuper, nil)
		require.NoError(t, err)
		require.False(t, lookuper.called)
	})
}
