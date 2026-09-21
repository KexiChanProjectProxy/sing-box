package direct

import (
	"context"
	"net"
	"net/netip"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/stretchr/testify/require"
)

func sourceBindOutboundOptions(sourceBind option.SourceBindOptions) option.DirectOutboundOptions {
	return option.DirectOutboundOptions{
		DialerOptions: option.DialerOptions{AbstractDialerOptions: option.AbstractDialerOptions{
			DomainResolver: &option.DomainResolveOptions{Server: "resolver"},
		}},
		SourceBind: &sourceBind,
	}
}

// TestOutboundSourceBindEndToEnd dials a loopback listener through the real
// direct outbound and checks the local address the server observes.
func TestOutboundSourceBindEndToEnd(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	accepted := make(chan netip.Addr, 16)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			accepted <- M.SocksaddrFromNet(conn.RemoteAddr()).Addr
			conn.Close()
		}
	}()

	rawOutbound, err := NewOutbound(xlat464OutboundTestContext(nil), nil, log.NewNOPFactory().Logger(), "direct", sourceBindOutboundOptions(option.SourceBindOptions{
		Inet4Addresses: prefixes("127.0.10.0/24"),
		Rules: []option.SourceBindRule{{
			SourceIPCIDR:   prefixes("10.9.0.0/16"),
			Inet4Addresses: prefixes("127.0.20.7"),
		}},
	}))
	require.NoError(t, err)
	outbound := rawOutbound.(*Outbound)
	require.False(t, outbound.IsEmpty())
	_, isDefaultDialer := common.Cast[*dialer.DefaultDialer](outbound.dialer)
	require.True(t, isDefaultDialer, "ICMP relies on reaching the DefaultDialer through the wrapper")

	destination := M.SocksaddrFromNet(listener.Addr())
	dialFrom := func(source string) netip.Addr {
		ctx := context.Background()
		if source != "" {
			ctx = adapter.WithContext(ctx, &adapter.InboundContext{Source: M.ParseSocksaddrHostPort(source, 50000)})
		}
		conn, err := outbound.DialContext(ctx, N.NetworkTCP, destination)
		require.NoError(t, err)
		conn.Close()
		return <-accepted
	}

	pooled := dialFrom("192.168.5.5")
	require.True(t, netip.MustParsePrefix("127.0.10.0/24").Contains(pooled), pooled)
	require.Equal(t, pooled, dialFrom("192.168.5.5"))
	require.Equal(t, netip.MustParseAddr("127.0.20.7"), dialFrom("10.9.1.1"))
	require.Equal(t, netip.MustParseAddr("127.0.0.1"), dialFrom(""))
}

func TestOutboundSourceBindWithXLAT464(t *testing.T) {
	t.Parallel()

	options := xlat464OutboundTestOptions(C.DomainStrategyAsIS)
	options.SourceBind = &option.SourceBindOptions{Inet6Addresses: prefixes("2001:db8::/64")}
	rawOutbound, err := NewOutbound(xlat464OutboundTestContext(nil), nil, log.NewNOPFactory().Logger(), "direct", options)
	require.NoError(t, err)
	upstream := rawOutbound.(*Outbound).dialer.(interface{ Upstream() any }).Upstream()
	xlat, isXLAT := upstream.(*xlat464Dialer)
	require.True(t, isXLAT, "got %T", upstream)
	_, isSourceBind := xlat.dialer.(*sourceBindDialer)
	require.True(t, isSourceBind, "got %T", xlat.dialer)
}

func TestOutboundSourceBindRejectsNetworkStrategy(t *testing.T) {
	t.Parallel()

	options := sourceBindOutboundOptions(option.SourceBindOptions{Inet4Addresses: prefixes("192.0.2.1")})
	options.NetworkType = badoption.Listable[option.InterfaceType]{option.InterfaceType(C.InterfaceTypeWIFI)}
	_, err := NewOutbound(xlat464OutboundTestContext(nil), nil, log.NewNOPFactory().Logger(), "direct", options)
	require.ErrorContains(t, err, "`source_bind` is conflict with")
}

func TestOutboundNonLocalBindIsNotEmpty(t *testing.T) {
	t.Parallel()

	options := option.DirectOutboundOptions{NonLocalBind: true}
	options.DomainResolver = &option.DomainResolveOptions{Server: "resolver"}
	rawOutbound, err := NewOutbound(xlat464OutboundTestContext(nil), nil, log.NewNOPFactory().Logger(), "direct", options)
	require.NoError(t, err)
	require.False(t, rawOutbound.(*Outbound).IsEmpty())
}
