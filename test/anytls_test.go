package main

import (
	"context"
	stdtls "crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/sagernet/sing-box"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/json/badoption"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"

	anytls "github.com/anytls/sing-anytls"
	"github.com/stretchr/testify/require"
)

func TestAnyTLSSelf(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
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
			{
				Type: C.TypeAnyTLS,
				Options: &option.AnyTLSInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					Users: []option.AnyTLSUser{
						{
							Name:     "sekai",
							Password: "password",
						},
					},
					InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
						TLS: &option.InboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
							KeyPath:         keyPem,
						},
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
			},
			{
				Type: C.TypeAnyTLS,
				Tag:  "anytls-out",
				Options: &option.AnyTLSOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					Password:                    "password",
					IdleSessionCheckInterval:    badoption.Duration(3 * time.Second),
					IdleSessionTimeout:          badoption.Duration(120 * time.Second),
					MinIdleSession:              48,
					EnsureIdleSession:           24,
					MinIdleSessionForAge:        8,
					EnsureIdleSessionCreateRate: 16,
					Heartbeat:                   badoption.Duration(17 * time.Second),
					MaxConnectionLifetime:       badoption.Duration(3 * time.Minute),
					ConnectionLifetimeJitter:    badoption.Duration(30 * time.Second),
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
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						RawDefaultRule: option.RawDefaultRule{
							Inbound: []string{"mixed-in"},
						},
						RuleAction: option.RuleAction{
							Action: C.RuleActionTypeRoute,

							RouteOptions: option.RouteActionOptions{
								Outbound: "anytls-out",
							},
						},
					},
				},
			},
		},
	})
	testSuit(t, clientPort, testPort)
}

func TestAnyTLSOutboundRequiresTLS(t *testing.T) {
	_, _, _ = createSelfSignedCertificate(t, "example.org")
	instance, err := box.New(box.Options{
		Context: globalCtx,
		Options: option.Options{
			Log: &option.LogOptions{
				Level: "warning",
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
					Type: C.TypeAnyTLS,
					Tag:  "anytls-out",
					Options: &option.AnyTLSOutboundOptions{
						ServerOptions: option.ServerOptions{
							Server:     "127.0.0.1",
							ServerPort: serverPort,
						},
						Password: "password",
					},
				},
			},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "TLS required")
	if instance != nil {
		instance.Close()
	}
}

func anyTLSFallbackInbound(fallbackPort uint16, tlsOptions *option.InboundTLSOptions) option.Inbound {
	inboundOptions := &option.AnyTLSInboundOptions{
		ListenOptions: option.ListenOptions{
			Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
			ListenPort: serverPort,
		},
		Users: []option.AnyTLSUser{{
			Name:     "sekai",
			Password: "password",
		}},
		InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
			TLS: tlsOptions,
		},
	}
	if fallbackPort != 0 {
		inboundOptions.Fallback = &option.ServerOptions{
			Server:     "127.0.0.1",
			ServerPort: fallbackPort,
		}
	}
	return option.Inbound{
		Type:    C.TypeAnyTLS,
		Options: inboundOptions,
	}
}

func newAnyTLSFallbackBackend(t *testing.T) (uint16, <-chan string) {
	requests := make(chan string, 1)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case requests <- r.URL.Path:
		default:
		}
		_, _ = w.Write([]byte("anytls fallback response"))
	}))
	t.Cleanup(backend.Close)
	return uint16(backend.Listener.Addr().(*net.TCPAddr).Port), requests
}

func requireAnyTLSFallbackResponse(t *testing.T, client *http.Client, url string, requests <-chan string) {
	t.Helper()
	response, err := client.Get(url)
	require.NoError(t, err)
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t, "anytls fallback response", string(body))
	select {
	case path := <-requests:
		require.Equal(t, "/probe", path)
	case <-time.After(5 * time.Second):
		t.Fatal("fallback backend did not receive the request")
	}
	client.CloseIdleConnections()
}

func TestAnyTLSFallbackPlaintext(t *testing.T) {
	backendPort, requests := newAnyTLSFallbackBackend(t)
	startInstance(t, option.Options{
		Inbounds:  []option.Inbound{anyTLSFallbackInbound(backendPort, nil)},
		Outbounds: []option.Outbound{{Type: C.TypeDirect}},
	})
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, network, "127.0.0.1:"+F.ToString(serverPort))
			},
		},
	}
	requireAnyTLSFallbackResponse(t, client, "http://example.org/probe", requests)
}

func TestAnyTLSFallbackTLS(t *testing.T) {
	backendPort, requests := newAnyTLSFallbackBackend(t)
	caPem, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{anyTLSFallbackInbound(backendPort, &option.InboundTLSOptions{
			Enabled:         true,
			ServerName:      "example.org",
			CertificatePath: certPem,
			KeyPath:         keyPem,
		})},
		Outbounds: []option.Outbound{{Type: C.TypeDirect}},
	})
	caBytes, err := os.ReadFile(caPem)
	require.NoError(t, err)
	rootCAs := x509.NewCertPool()
	require.True(t, rootCAs.AppendCertsFromPEM(caBytes))
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &stdtls.Config{
				RootCAs:    rootCAs,
				ServerName: "example.org",
			},
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, network, "127.0.0.1:"+F.ToString(serverPort))
			},
		},
	}
	requireAnyTLSFallbackResponse(t, client, "https://example.org/probe", requests)
}

func TestAnyTLSPlaintextSelf(t *testing.T) {
	startInstance(t, option.Options{
		Inbounds:  []option.Inbound{anyTLSFallbackInbound(0, nil)},
		Outbounds: []option.Outbound{{Type: C.TypeDirect}},
	})
	newClient := func(password string) *anytls.Client {
		client, err := anytls.NewClient(globalCtx, anytls.ClientConfig{
			Password: password,
			DialOut: func(ctx context.Context) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "tcp", "127.0.0.1:"+F.ToString(serverPort))
			},
			Logger: logger.NOP(),
		})
		require.NoError(t, err)
		t.Cleanup(func() { client.Close() })
		return client
	}
	client := newClient("password")
	dialTCP := func() (net.Conn, error) {
		return client.CreateProxy(globalCtx, M.ParseSocksaddrHostPort("127.0.0.1", testPort))
	}
	require.NoError(t, testPingPongWithConn(t, testPort, dialTCP))
	require.NoError(t, testLargeDataWithConn(t, testPort, dialTCP))

	wrongClient := newClient("wrong-password")
	require.Error(t, testPingPongWithConn(t, testPort, func() (net.Conn, error) {
		return wrongClient.CreateProxy(globalCtx, M.ParseSocksaddrHostPort("127.0.0.1", testPort))
	}))
}
