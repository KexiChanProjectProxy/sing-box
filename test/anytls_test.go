package main

import (
	"net/netip"
	"testing"
	"time"

	"github.com/sagernet/sing-box"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/json/badoption"

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
