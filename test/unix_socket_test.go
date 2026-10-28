//go:build linux

package main

import (
	"context"
	"net/netip"
	"path/filepath"
	"testing"

	"github.com/sagernet/sing-box"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/json/badoption"
	N "github.com/sagernet/sing/common/network"

	"github.com/stretchr/testify/require"
)

func unixSocketMixedInbound() option.Inbound {
	return option.Inbound{
		Type: C.TypeMixed,
		Tag:  "mixed-in",
		Options: &option.HTTPMixedInboundOptions{
			ListenOptions: option.ListenOptions{
				Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
				ListenPort: clientPort,
			},
		},
	}
}

func unixSocketRouteTo(outbound string) *option.RouteOptions {
	return &option.RouteOptions{
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
							Outbound: outbound,
						},
					},
				},
			},
		},
	}
}

// TestUnixSocketSOCKS chains a socks outbound to a socks inbound over a Unix
// socket. UDP goes through UDP over TCP, so it also crosses the socket.
func TestUnixSocketSOCKS(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "socks.sock")
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			unixSocketMixedInbound(),
			{
				Type: C.TypeSOCKS,
				Options: &option.SocksInboundOptions{
					ListenOptions: option.ListenOptions{
						ListenUnix: socketPath,
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
			},
			{
				Type: C.TypeSOCKS,
				Tag:  "socks-out",
				Options: &option.SOCKSOutboundOptions{
					ServerUnixOptions: option.ServerUnixOptions{
						ServerUnix: socketPath,
					},
					UDPOverTCP: &option.UDPOverTCPOptions{
						Enabled: true,
					},
				},
			},
		},
		Route: unixSocketRouteTo("socks-out"),
	})
	testSuit(t, clientPort, testPort)
}

// TestUnixSocketVLESSWebSocketTLS runs VLESS over WebSocket and TLS on a Unix
// socket. The server name cannot be resolved, which shows that server is only
// used as the logical address (TLS server name and Host) and never dialed.
func TestUnixSocketVLESSWebSocketTLS(t *testing.T) {
	const serverName = "unix.invalid"
	socketPath := filepath.Join(t.TempDir(), "vless.sock")
	_, certPem, keyPem := createSelfSignedCertificate(t, serverName)
	user := "b831381d-6324-4d53-ad4f-8cda48b30811"
	transport := &option.V2RayTransportOptions{
		Type: C.V2RayTransportTypeWebsocket,
		WebsocketOptions: option.V2RayWebsocketOptions{
			Path: "/ws",
		},
	}
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			unixSocketMixedInbound(),
			{
				Type: C.TypeVLESS,
				Options: &option.VLESSInboundOptions{
					ListenOptions: option.ListenOptions{
						ListenUnix: socketPath,
					},
					Users: []option.VLESSUser{{UUID: user}},
					InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
						TLS: &option.InboundTLSOptions{
							Enabled:         true,
							ServerName:      serverName,
							CertificatePath: certPem,
							KeyPath:         keyPem,
						},
					},
					Transport: transport,
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
			},
			{
				Type: C.TypeVLESS,
				Tag:  "vless-out",
				Options: &option.VLESSOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     serverName,
						ServerPort: 443,
					},
					ServerUnixOptions: option.ServerUnixOptions{
						ServerUnix: socketPath,
					},
					UUID: user,
					OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
						TLS: &option.OutboundTLSOptions{
							Enabled:         true,
							CertificatePath: certPem,
						},
					},
					Transport: transport,
				},
			},
		},
		Route: unixSocketRouteTo("vless-out"),
	})
	testSuit(t, clientPort, testPort)
}

func TestUnixSocketInvalidOptions(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "invalid.sock")
	for _, testCase := range []struct {
		name    string
		options option.Options
		atStart bool
	}{
		{
			name: "listen_unix with listen_port",
			options: option.Options{
				Inbounds: []option.Inbound{{
					Type: C.TypeSOCKS,
					Options: &option.SocksInboundOptions{
						ListenOptions: option.ListenOptions{ListenUnix: socketPath, ListenPort: clientPort},
					},
				}},
			},
			atStart: true,
		},
		{
			name: "listen_unix on a udp only inbound",
			options: option.Options{
				Inbounds: []option.Inbound{{
					Type: C.TypeDirect,
					Options: &option.DirectInboundOptions{
						ListenOptions: option.ListenOptions{ListenUnix: socketPath},
						Network:       option.NetworkList(N.NetworkUDP),
					},
				}},
			},
			atStart: true,
		},
		{
			name: "server_unix with detour",
			options: option.Options{
				Outbounds: []option.Outbound{
					{
						Type: C.TypeSOCKS,
						Options: &option.SOCKSOutboundOptions{
							DialerOptions:     option.DialerOptions{Detour: "direct"},
							ServerUnixOptions: option.ServerUnixOptions{ServerUnix: socketPath},
						},
					},
					{Type: C.TypeDirect, Tag: "direct"},
				},
			},
		},
		{
			name: "server_unix with tcp_fast_open",
			options: option.Options{
				Outbounds: []option.Outbound{{
					Type: C.TypeSOCKS,
					Options: &option.SOCKSOutboundOptions{
						DialerOptions: option.DialerOptions{
							AbstractDialerOptions: option.AbstractDialerOptions{TCPFastOpen: true},
						},
						ServerUnixOptions: option.ServerUnixOptions{ServerUnix: socketPath},
					},
				}},
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(globalCtx)
			defer cancel()
			instance, err := box.New(box.Options{Context: ctx, Options: testCase.options})
			if !testCase.atStart {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			defer instance.Close()
			require.Error(t, instance.Start())
		})
	}
}
