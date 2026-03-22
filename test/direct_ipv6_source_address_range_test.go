package main

import (
	"context"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	boxpkg "github.com/sagernet/sing-box"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks"

	"github.com/stretchr/testify/require"
)

func TestDirectIPv6SourceAddressRangeOutboundDirect(t *testing.T) {
	requireIPv6Loopback(t)
	logPath := filepath.Join(t.TempDir(), "sing-box.log")

	startInstance(t, option.Options{
		Log: &option.LogOptions{
			Output:    logPath,
			Timestamp: false,
		},
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-in",
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: clientPort,
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
				Options: &option.DirectOutboundOptions{
					DialerOptions: option.DialerOptions{
						IPv6SourceAddressRange: mustIPv6Prefix("2001:db8::/64"),
						IPv6SourceAddressMode:  option.IPv6SourceAddressModeRandom,
					},
				},
			},
		},
	})

	testSuitIPv6(t, clientPort, testPort)

	require.Eventually(t, func() bool {
		content, err := os.ReadFile(logPath)
		if err != nil {
			return false
		}
		logText := string(content)
		return strings.Contains(logText, "ipv6_source_address_range fallback") &&
			strings.Contains(logText, "reason=bind_failed")
	}, 5*time.Second, 100*time.Millisecond)
}

func TestDirectIPv6SourceAddressRangeRouteActionDirect(t *testing.T) {
	requireIPv6Loopback(t)

	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-in",
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: clientPort,
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						RawDefaultRule: option.RawDefaultRule{
							Inbound: []string{"mixed-in"},
						},
						RuleAction: option.RuleAction{
							Action: C.RuleActionTypeDirect,
							DirectOptions: option.DirectActionOptions{
								IPv6SourceAddressRange: mustIPv6Prefix("2001:db8:1::/64"),
								IPv6SourceAddressMode:  option.IPv6SourceAddressMode("hash_5tuple"),
							},
						},
					},
				},
			},
		},
	})

	testSuitIPv6(t, clientPort, testPort)
}

func TestDirectIPv6SourceAddressRangeRouteActionDirectInvalidMode(t *testing.T) {
	ctx, cancel := context.WithCancel(globalCtx)
	t.Cleanup(cancel)

	_, err := boxpkg.New(boxpkg.Options{
		Context: ctx,
		Options: option.Options{
			Inbounds: []option.Inbound{
				{
					Type: C.TypeMixed,
					Tag:  "mixed-in",
					Options: &option.HTTPMixedInboundOptions{
						ListenOptions: option.ListenOptions{
							Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
							ListenPort: clientPort,
						},
					},
				},
			},
			Outbounds: []option.Outbound{{Type: C.TypeDirect}},
			Route: &option.RouteOptions{
				Rules: []option.Rule{
					{
						Type: C.RuleTypeDefault,
						DefaultOptions: option.DefaultRule{
							RawDefaultRule: option.RawDefaultRule{Inbound: []string{"mixed-in"}},
							RuleAction: option.RuleAction{
								Action: C.RuleActionTypeDirect,
								DirectOptions: option.DirectActionOptions{
									IPv6SourceAddressRange: mustIPv6Prefix("2001:db8:1::/64"),
									IPv6SourceAddressMode:  option.IPv6SourceAddressMode("invalid_mode"),
								},
							},
						},
					},
				},
			},
		},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "`ipv6_source_address_mode` must be one of: random, hash_5tuple")
}

func testSuitIPv6(t *testing.T, clientPort uint16, testPort uint16) {
	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", clientPort), socks.Version5, "", "")
	dialTCP := func() (net.Conn, error) {
		return dialer.DialContext(context.Background(), "tcp", M.ParseSocksaddrHostPort("::1", testPort))
	}
	dialUDP := func() (net.PacketConn, error) {
		return dialer.ListenPacket(context.Background(), M.ParseSocksaddrHostPort("::1", testPort))
	}
	require.NoError(t, testPingPongWithConn(t, testPort, dialTCP))
	require.NoError(t, testPingPongWithPacketConn(t, testPort, dialUDP))
	require.NoError(t, testLargeDataWithConn(t, testPort, dialTCP))
	require.NoError(t, testLargeDataWithPacketConn(t, testPort, dialUDP))
}

func requireIPv6Loopback(t *testing.T) {
	t.Helper()
	tcpListener, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback is unavailable: %v", err)
	}
	tcpListener.Close()
	packetConn, err := net.ListenPacket("udp6", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback UDP is unavailable: %v", err)
	}
	packetConn.Close()
}

func mustIPv6Prefix(prefix string) *badoption.Prefix {
	v := badoption.Prefix(netip.MustParsePrefix(prefix))
	return &v
}
