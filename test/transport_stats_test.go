package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/http3"
	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tcpinfo"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/json/badoption"

	"github.com/stretchr/testify/require"
)

// transportStatsSorter reads every transport keyword in both directions, so
// that the group enables collection on its members in both.
var transportStatsSorter = map[string]float64{
	"latency":             1,
	"client_rtt":          1,
	"server_rtt":          1,
	"client_loss_rate_1m": 10,
	"server_loss_rate_1m": 10,
}

// loadBalanceOver builds a group that routes the mixed inbound through member.
func loadBalanceOver(member string) option.Outbound {
	return option.Outbound{
		Type: C.TypeLoadBalance,
		Tag:  "lb",
		Options: &option.LoadBalanceOutboundOptions{
			PrimaryOutbounds: []string{member},
			URL:              "http://127.0.0.1:1/",
			Interval:         badoption.Duration(15 * time.Second),
			Sorter:           transportStatsSorter,
			EmptyPoolAction:  "random",
		},
	}
}

func mixedInboundTo(outbound string) (option.Inbound, *option.RouteOptions) {
	return option.Inbound{
			Type: C.TypeMixed,
			Tag:  "mixed-in",
			Options: &option.HTTPMixedInboundOptions{
				ListenOptions: option.ListenOptions{
					Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
					ListenPort: clientPort,
				},
			},
		}, &option.RouteOptions{
			Rules: []option.Rule{{
				Type: C.RuleTypeDefault,
				DefaultOptions: option.DefaultRule{
					RawDefaultRule: option.RawDefaultRule{Inbound: []string{"mixed-in"}},
					RuleAction: option.RuleAction{
						Action:       C.RuleActionTypeRoute,
						RouteOptions: option.RouteActionOptions{Outbound: outbound},
					},
				},
			}},
		}
}

func statsMember(t *testing.T, instance *box.Box, tag string) adapter.OutboundWithTransportStats {
	t.Helper()
	detour, loaded := instance.Outbound().Outbound(tag)
	require.True(t, loaded)
	member, isStatsMember := detour.(adapter.OutboundWithTransportStats)
	require.True(t, isStatsMember, "%s must report transport statistics", tag)
	return member
}

// waitForStats keeps traffic flowing through the proxy until both directions
// have an RTT and a loss rate, which needs two sampling rounds for the counters
// to have a baseline and an increment.
func waitForStats(t *testing.T, member adapter.OutboundWithTransportStats, directions ...adapter.TransportStatsDirection) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for {
		testTCP(t, clientPort, testPort)
		ready := true
		for _, direction := range directions {
			reader := member.TransportStats(direction)
			if reader == nil {
				ready = false
				break
			}
			if _, ok := reader.RTT(time.Minute); !ok {
				ready = false
				break
			}
			if _, ok := reader.LossRate(time.Minute); !ok {
				ready = false
				break
			}
		}
		if ready {
			break
		}
		if time.Now().After(deadline) {
			for _, direction := range directions {
				reader := member.TransportStats(direction)
				if reader == nil {
					t.Fatalf("%s direction never enabled", direction)
				}
				_, rttOK := reader.RTT(time.Minute)
				_, lossOK := reader.LossRate(time.Minute)
				t.Logf("%s: rtt=%v loss=%v", direction, rttOK, lossOK)
			}
			t.Fatal("transport statistics did not arrive")
		}
		time.Sleep(time.Second)
	}
	for _, direction := range directions {
		reader := member.TransportStats(direction)
		rtt, _ := reader.RTT(time.Minute)
		require.Positive(t, rtt, "%s rtt", direction)
		require.Less(t, rtt, 1000.0, "%s rtt on loopback is milliseconds at most", direction)
		loss, _ := reader.LossRate(time.Minute)
		require.GreaterOrEqual(t, loss, 0.0)
		require.LessOrEqual(t, loss, 100.0)
		rate, ok := reader.DeliveryRate(time.Minute)
		if ok {
			require.Positive(t, rate, "%s delivery rate", direction)
		}
	}
}

func TestHysteria2TransportStats(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	mixedIn, route := mixedInboundTo("lb")
	instance := startInstance(t, option.Options{
		Inbounds: []option.Inbound{mixedIn, {
			Type: C.TypeHysteria2,
			Options: &option.Hysteria2InboundOptions{
				ListenOptions: option.ListenOptions{
					Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
					ListenPort: serverPort,
				},
				UpMbps:   100,
				DownMbps: 100,
				Users:    []option.Hysteria2User{{Password: "password"}},
				InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
					TLS: &option.InboundTLSOptions{
						Enabled:         true,
						ServerName:      "example.org",
						CertificatePath: certPem,
						KeyPath:         keyPem,
					},
				},
			},
		}},
		Outbounds: []option.Outbound{
			{Type: C.TypeDirect},
			{
				Type: C.TypeHysteria2,
				Tag:  "hy2-out",
				Options: &option.Hysteria2OutboundOptions{
					ServerOptions: option.ServerOptions{Server: "127.0.0.1", ServerPort: serverPort},
					UpMbps:        100,
					DownMbps:      100,
					Password:      "password",
					OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
						TLS: &option.OutboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
						},
					},
				},
			},
			loadBalanceOver("hy2-out"),
		},
		Route: route,
	})
	waitForStats(t, statsMember(t, instance, "hy2-out"), adapter.TransportStatsClient, adapter.TransportStatsServer)
}

func TestAnyTLSTransportStats(t *testing.T) {
	if !tcpinfo.Supported {
		t.Skip("TCP_INFO is only read on Linux")
	}
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	mixedIn, route := mixedInboundTo("lb")
	instance := startInstance(t, option.Options{
		Inbounds: []option.Inbound{mixedIn, {
			Type: C.TypeAnyTLS,
			Options: &option.AnyTLSInboundOptions{
				ListenOptions: option.ListenOptions{
					Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
					ListenPort: serverPort,
				},
				Users: []option.AnyTLSUser{{Name: "sekai", Password: "password"}},
				InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
					TLS: &option.InboundTLSOptions{
						Enabled:         true,
						ServerName:      "example.org",
						CertificatePath: certPem,
						KeyPath:         keyPem,
					},
				},
			},
		}},
		Outbounds: []option.Outbound{
			{Type: C.TypeDirect},
			{
				Type: C.TypeAnyTLS,
				Tag:  "anytls-out",
				Options: &option.AnyTLSOutboundOptions{
					ServerOptions:  option.ServerOptions{Server: "127.0.0.1", ServerPort: serverPort},
					Password:       "password",
					MinIdleSession: 2,
					OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
						TLS: &option.OutboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
						},
					},
				},
			},
			loadBalanceOver("anytls-out"),
		},
		Route: route,
	})
	waitForStats(t, statsMember(t, instance, "anytls-out"), adapter.TransportStatsClient, adapter.TransportStatsServer)
}

// A client that asks for statistics against a server that does not report
// them — here, an anytls member that never enabled the server direction, and a
// plain proxy — must carry traffic exactly as before.
func TestTransportStatsDisabledTrafficUnchanged(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	mixedIn, route := mixedInboundTo("hy2-out")
	instance := startInstance(t, option.Options{
		Inbounds: []option.Inbound{mixedIn, {
			Type: C.TypeHysteria2,
			Options: &option.Hysteria2InboundOptions{
				ListenOptions: option.ListenOptions{
					Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
					ListenPort: serverPort,
				},
				Users: []option.Hysteria2User{{Password: "password"}},
				InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
					TLS: &option.InboundTLSOptions{
						Enabled:         true,
						ServerName:      "example.org",
						CertificatePath: certPem,
						KeyPath:         keyPem,
					},
				},
			},
		}},
		Outbounds: []option.Outbound{
			{Type: C.TypeDirect},
			{
				Type: C.TypeHysteria2,
				Tag:  "hy2-out",
				Options: &option.Hysteria2OutboundOptions{
					ServerOptions: option.ServerOptions{Server: "127.0.0.1", ServerPort: serverPort},
					Password:      "password",
					OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
						TLS: &option.OutboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
						},
					},
				},
			},
		},
		Route: route,
	})
	testSuit(t, clientPort, testPort)
	member := statsMember(t, instance, "hy2-out")
	require.Nil(t, member.TransportStats(adapter.TransportStatsClient), "no group asked for statistics")
	require.Nil(t, member.TransportStats(adapter.TransportStatsServer))
}

// The statistics endpoint must not be visible to anyone but an authenticated
// client that negotiated it: an unauthenticated request for it reaches the
// masquerade site like any other path.
func TestHysteria2StatsEndpointHiddenBehindMasquerade(t *testing.T) {
	observations := make(chan string, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observations <- r.URL.Path
		_, _ = w.Write([]byte("masquerade"))
	}))
	t.Cleanup(upstream.Close)

	caPem, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{{
			Type: C.TypeHysteria2,
			Options: &option.Hysteria2InboundOptions{
				ListenOptions: option.ListenOptions{
					Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
					ListenPort: serverPort,
				},
				Users: []option.Hysteria2User{{Password: "password"}},
				InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
					TLS: &option.InboundTLSOptions{
						Enabled:         true,
						ServerName:      "example.org",
						CertificatePath: certPem,
						KeyPath:         keyPem,
					},
				},
				Masquerade: &option.Hysteria2Masquerade{
					Type:         C.Hysterai2MasqueradeTypeProxy,
					ProxyOptions: option.Hysteria2MasqueradeProxy{URL: upstream.URL},
				},
			},
		}},
	})

	caBytes, err := os.ReadFile(caPem)
	require.NoError(t, err)
	rootCAs := x509.NewCertPool()
	require.True(t, rootCAs.AppendCertsFromPEM(caBytes))
	packetConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	t.Cleanup(func() { _ = packetConn.Close() })
	serverAddress := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: int(serverPort)}
	transport := &http3.Transport{
		TLSClientConfig: &tls.Config{RootCAs: rootCAs, ServerName: "example.org"},
		Dial: func(ctx context.Context, _ string, tlsConfig *tls.Config, quicConfig *quic.Config) (*quic.Conn, error) {
			return quic.DialEarly(ctx, packetConn, serverAddress, tlsConfig, quicConfig)
		},
	}
	t.Cleanup(func() { _ = transport.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://hysteria/stats", nil)
	require.NoError(t, err)
	response, err := (&http.Client{Transport: transport}).Do(request)
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	_ = response.Body.Close()
	require.Equal(t, "masquerade", string(body))
	select {
	case path := <-observations:
		require.Equal(t, "/stats", path)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
