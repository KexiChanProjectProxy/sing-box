package firefoxvpn

import (
	"context"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	box "github.com/sagernet/sing-box"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	protocolfirefoxvpn "github.com/sagernet/sing-box/protocol/firefoxvpn"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/debug"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

var globalCtx = include.Context(context.Background())

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestFirefoxVPNHappyPath(t *testing.T) {
	t.Parallel()

	// Given
	echo := newEchoBackend(t)
	proxyPassToken := newProxyPassToken(t, time.Now().Add(time.Hour))
	fxa := newFakeFxAServer(t)
	guardian := newFakeGuardianServer(t, proxyPassToken)
	h2Proxy := newFakeH2ProxyServer(t)
	h2Proxy.ExpectedProxyPassToken = proxyPassToken
	mixedPort := reserveTCPPort(t)
	patchControlPlaneClientFactory(t, fxa.BaseURL+"/v1", guardian.BaseURL)

	options := option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-in",
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.MustParseAddr("127.0.0.1"))),
						ListenPort: mixedPort,
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			directOutbound("direct"),
			directOutbound("api-upstream"),
			directOutbound("data-upstream"),
			{
				Type: C.TypeFirefoxVPN,
				Tag:  "firefox-vpn",
				Options: &option.FirefoxVPNOutboundOptions{
					DialerOptions: option.DialerOptions{Detour: "data-upstream"},
					ServerOptions: option.ServerOptions{Server: "127.0.0.1", ServerPort: h2Proxy.Port},
					APIDetour:     "api-upstream",
					Email:         "user@example.com",
					Password:      "correct horse battery staple",
					OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
						TLS: &option.OutboundTLSOptions{Enabled: true, Insecure: true, ServerName: h2Proxy.ServerName},
					},
				},
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{inboundRouteRule("mixed-in", "firefox-vpn")},
			Final: "direct",
		},
	}

	startInstance(t, options)

	client := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", mixedPort), socks.Version5, "", "")

	// When
	conn, err := client.DialContext(t.Context(), N.NetworkTCP, M.ParseSocksaddr(echo.Address))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	payload := []byte("firefox-vpn happy path")
	_, err = conn.Write(payload)
	require.NoError(t, err)
	response := make([]byte, len(payload))
	_, err = io.ReadFull(conn, response)
	require.NoError(t, err)

	// Then
	require.Equal(t, payload, response)
}

func directOutbound(tag string) option.Outbound {
	return option.Outbound{
		Type: C.TypeDirect,
		Tag:  tag,
		Options: &option.DirectOutboundOptions{
			DialerOptions: option.DialerOptions{ConnectTimeout: badoption.Duration(5 * time.Second)},
		},
	}
}

func inboundRouteRule(inboundTag string, outboundTag string) option.Rule {
	return option.Rule{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultRule{
			RawDefaultRule: option.RawDefaultRule{Inbound: []string{inboundTag}},
			RuleAction: option.RuleAction{
				Action:       C.RuleActionTypeRoute,
				RouteOptions: option.RouteActionOptions{Outbound: outboundTag},
			},
		},
	}
}

func patchControlPlaneClientFactory(t *testing.T, fxaBaseURL string, guardianBaseURL string) {
	t.Helper()
	old := protocolfirefoxvpn.TestNewControlPlaneClientOverride
	protocolfirefoxvpn.TestNewControlPlaneClientOverride = func(ctx context.Context, apiDetour string) (*protocolfirefoxvpn.ControlPlaneClient, error) {
		return protocolfirefoxvpn.NewControlPlaneClientWithEndpoints(ctx, apiDetour, protocolfirefoxvpn.NewControlPlaneEndpoints(fxaBaseURL, guardianBaseURL))
	}
	t.Cleanup(func() { protocolfirefoxvpn.TestNewControlPlaneClientOverride = old })
}

func reserveTCPPort(t *testing.T) uint16 {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	return uint16(listener.Addr().(*net.TCPAddr).Port)
}

func startInstance(t *testing.T, options option.Options) *box.Box {
	t.Helper()
	if debug.Enabled {
		options.Log = &option.LogOptions{Level: "trace"}
	} else {
		options.Log = &option.LogOptions{Level: "warning"}
	}
	ctx, cancel := context.WithCancel(globalCtx)
	var instance *box.Box
	var err error
	for retry := 0; retry < 3; retry++ {
		instance, err = box.New(box.Options{Context: ctx, Options: options})
		require.NoError(t, err)
		err = instance.Start()
		if err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = instance.Close()
		cancel()
	})
	return instance
}
