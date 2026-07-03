package firefoxvpn

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"testing"
	"time"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	protocolfirefoxvpn "github.com/sagernet/sing-box/protocol/firefoxvpn"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks"
	"github.com/sagernet/sing/service"
	"github.com/stretchr/testify/require"
)

func TestFirefoxVPNApiDetourSeparateFromDataDetour(t *testing.T) {
	// Given
	echo := newEchoBackend(t)
	proxyPassToken := newProxyPassToken(t, time.Now().Add(time.Hour))
	fxa := newFakeFxAServer(t)
	guardian := newFakeGuardianServer(t, proxyPassToken)
	h2Proxy := newFakeH2ProxyServer(t)
	h2Proxy.ExpectedProxyPassToken = proxyPassToken
	apiProxy := newObservableSOCKSProxy(t, "api-upstream")
	apiProxy.SetHostAlias(fakeFxAHost, "127.0.0.1")
	apiProxy.SetHostAlias(fakeGuardianHost, "127.0.0.1")
	dataProxy := newObservableSOCKSProxy(t, "data-upstream")
	dataProxy.SetHostAlias(fakeProxyHost, "127.0.0.1")
	mixedPort := reserveTCPPort(t)
	patchControlPlaneClientFactory(t, fxa.EndpointURL()+"/v1", guardian.EndpointURL())

	options := mixedFirefoxVPNOptions(mixedPort,
		directOutbound("direct"),
		socksDetourOutbound("api-upstream", apiProxy.Port()),
		socksDetourOutbound("data-upstream", dataProxy.Port()),
		firefoxVPNOutbound("firefox-vpn", newFirefoxVPNOutboundOptions("data-upstream", "api-upstream", fakeProxyHost, h2Proxy.Port, h2Proxy.ServerName)),
	)

	startInstance(t, options)
	client := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", mixedPort), socks.Version5, "", "")

	// When
	conn, err := client.DialContext(t.Context(), N.NetworkTCP, M.ParseSocksaddr(echo.Address))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	payload := []byte("detour split verification")
	_, err = conn.Write(payload)
	require.NoError(t, err)
	response := make([]byte, len(payload))
	_, err = io.ReadFull(conn, response)
	require.NoError(t, err)

	// Then
	require.Equal(t, payload, response)
	require.Equal(t,
		[]string{
			net.JoinHostPort(fakeFxAHost, netPortString(fxa.Port)),
			net.JoinHostPort(fakeGuardianHost, netPortString(guardian.Port)),
		},
		apiProxy.Destinations(),
	)
	require.Equal(t, []string{h2Proxy.EndpointAddress()}, dataProxy.Destinations())
	require.Equal(t, []string{echo.Address}, h2Proxy.ConnectDestinations())
}

func TestFirefoxVPNRestartRequiresLogin(t *testing.T) {
	// Given
	echo := newEchoBackend(t)
	proxyPassToken := newProxyPassToken(t, time.Now().Add(time.Hour))
	fxa := newFakeFxAServer(t)
	guardian := newFakeGuardianServer(t, proxyPassToken)
	h2Proxy := newFakeH2ProxyServer(t)
	h2Proxy.ExpectedProxyPassToken = proxyPassToken
	patchControlPlaneClientFactory(t, fxa.BaseURL+"/v1", guardian.BaseURL)
	mixedPort := reserveTCPPort(t)
	options := firefoxVPNHarnessOptions(mixedPort, newFirefoxVPNOutboundOptions("data-upstream", "api-upstream", "127.0.0.1", h2Proxy.Port, h2Proxy.ServerName))

	// When
	firstInstance := startInstance(t, options)
	roundTripThroughFirefoxVPN(t, mixedPort, echo.Address, "first login")
	require.NoError(t, firstInstance.Close())

	secondInstance := startInstance(t, options)
	t.Cleanup(func() { _ = secondInstance.Close() })
	roundTripThroughFirefoxVPN(t, mixedPort, echo.Address, "second login")

	// Then
	tokens := fxa.SessionTokens()
	require.Equal(t, 2, fxa.LoginCalls())
	require.Len(t, tokens, 2)
	require.NotEqual(t, tokens[0], tokens[1])
	require.Equal(t, []string{echo.Address, echo.Address}, h2Proxy.ConnectDestinations())
}

func TestFirefoxVPNNoPersistence(t *testing.T) {
	// Given
	echo := newEchoBackend(t)
	proxyPassToken := newProxyPassToken(t, time.Now().Add(time.Hour))
	fxa := newFakeFxAServer(t)
	guardian := newFakeGuardianServer(t, proxyPassToken)
	h2Proxy := newFakeH2ProxyServer(t)
	h2Proxy.ExpectedProxyPassToken = proxyPassToken
	patchControlPlaneClientFactory(t, fxa.BaseURL+"/v1", guardian.BaseURL)
	tempDir := t.TempDir()
	oldWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWD)) })
	mixedPort := reserveTCPPort(t)

	// When
	instance := startInstance(t, firefoxVPNHarnessOptions(mixedPort, newFirefoxVPNOutboundOptions("data-upstream", "api-upstream", "127.0.0.1", h2Proxy.Port, h2Proxy.ServerName)))
	roundTripThroughFirefoxVPN(t, mixedPort, echo.Address, "memory only")
	require.NoError(t, instance.Close())

	// Then
	secrets := []string{
		"correct horse battery staple",
		proxyPassToken,
		"fxa-access-token",
		"fxa-refresh-token",
	}
	secrets = append(secrets, fxa.SessionTokens()...)
	assertNoSecretsOnDisk(t, tempDir, secrets)
}

func TestFirefoxVPNEqualDetoursDedupDependencies(t *testing.T) {
	// Given
	ctx := service.ContextWith[adapter.OutboundManager](context.Background(), &stubOutboundManager{})
	rawOutbound, err := protocolfirefoxvpn.NewOutbound(ctx, nil, log.NewNOPFactory().Logger(), "firefox-vpn", option.FirefoxVPNOutboundOptions{
		DialerOptions:               option.DialerOptions{Detour: "shared-upstream"},
		ServerOptions:               option.ServerOptions{Server: "vpn.example.test", ServerPort: 443},
		APIDetour:                   "shared-upstream",
		Email:                       "user@example.com",
		Password:                    "correct horse battery staple",
		OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: &option.OutboundTLSOptions{Enabled: true, Insecure: true}},
	})
	require.NoError(t, err)

	options := option.Options{
		Outbounds: []option.Outbound{
			directOutbound("shared-upstream"),
			{
				Type: C.TypeFirefoxVPN,
				Tag:  "firefox-vpn",
				Options: &option.FirefoxVPNOutboundOptions{
					DialerOptions:               option.DialerOptions{Detour: "shared-upstream"},
					ServerOptions:               option.ServerOptions{Server: "vpn.example.test", ServerPort: 443},
					APIDetour:                   "shared-upstream",
					Email:                       "user@example.com",
					Password:                    "correct horse battery staple",
					OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: &option.OutboundTLSOptions{Enabled: true, Insecure: true}},
				},
			},
		},
	}

	// When
	instance, err := box.New(box.Options{Context: globalCtx, Options: options})
	require.NoError(t, err)
	t.Cleanup(func() { _ = instance.Close() })
	ob, ok := instance.Outbound().Outbound("firefox-vpn")

	// Then
	require.Equal(t, []string{"shared-upstream"}, rawOutbound.Dependencies())
	require.True(t, ok)
	require.Equal(t, []string{"shared-upstream"}, ob.Dependencies())
}

func TestFirefoxVPNBrokenDetourFailsControlPlane(t *testing.T) {
	// Given
	proxyPassToken := newProxyPassToken(t, time.Now().Add(time.Hour))
	fxa := newFakeFxAServer(t)
	guardian := newFakeGuardianServer(t, proxyPassToken)
	h2Proxy := newFakeH2ProxyServer(t)
	h2Proxy.ExpectedProxyPassToken = proxyPassToken
	brokenAPIProxy := newObservableSOCKSProxy(t, "api-upstream")
	brokenAPIProxy.FailDialsWith(errors.New("api detour blocked"))
	patchControlPlaneClientFactory(t, fxa.EndpointURL()+"/v1", guardian.EndpointURL())

	instance, err := box.New(box.Options{Context: globalCtx, Options: option.Options{
		Outbounds: []option.Outbound{
			directOutbound("direct"),
			socksDetourOutbound("api-upstream", brokenAPIProxy.Port()),
			directOutbound("data-upstream"),
			firefoxVPNOutbound("firefox-vpn", newFirefoxVPNOutboundOptions("data-upstream", "api-upstream", "127.0.0.1", h2Proxy.Port, h2Proxy.ServerName)),
		},
	}})
	require.NoError(t, err)
	t.Cleanup(func() { _ = instance.Close() })

	// When
	err = instance.Start()

	// Then
	require.Error(t, err)
	require.ErrorContains(t, err, "login to Firefox Accounts")
	require.ErrorContains(t, err, "socks5: request rejected")
	require.Len(t, brokenAPIProxy.Destinations(), 6)
	for _, destination := range brokenAPIProxy.Destinations() {
		require.Equal(t, net.JoinHostPort(fakeFxAHost, netPortString(fxa.Port)), destination)
	}
}

func TestFirefoxVPNBrokenDetourFailsTunnel(t *testing.T) {
	// Given
	echo := newEchoBackend(t)
	proxyPassToken := newProxyPassToken(t, time.Now().Add(time.Hour))
	fxa := newFakeFxAServer(t)
	guardian := newFakeGuardianServer(t, proxyPassToken)
	h2Proxy := newFakeH2ProxyServer(t)
	h2Proxy.ExpectedProxyPassToken = proxyPassToken
	brokenDataProxy := newObservableSOCKSProxy(t, "data-upstream")
	brokenDataProxy.FailDialsWith(errors.New("data detour blocked"))
	patchControlPlaneClientFactory(t, fxa.BaseURL+"/v1", guardian.BaseURL)
	mixedPort := reserveTCPPort(t)
	startInstance(t, mixedFirefoxVPNOptions(mixedPort,
		directOutbound("direct"),
		directOutbound("api-upstream"),
		socksDetourOutbound("data-upstream", brokenDataProxy.Port()),
		firefoxVPNOutbound("firefox-vpn", newFirefoxVPNOutboundOptions("data-upstream", "api-upstream", fakeProxyHost, h2Proxy.Port, h2Proxy.ServerName)),
	))
	client := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", mixedPort), socks.Version5, "", "")

	// When
	conn, err := client.DialContext(t.Context(), N.NetworkTCP, M.ParseSocksaddr(echo.Address))

	// Then
	if conn != nil {
		_ = conn.Close()
	}
	require.Error(t, err)
	require.ErrorContains(t, err, "socks5: request rejected")
	require.Equal(t, []string{h2Proxy.EndpointAddress()}, brokenDataProxy.Destinations())
	require.Empty(t, h2Proxy.ConnectDestinations())
}
