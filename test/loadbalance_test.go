package main

import (
	"net/netip"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/json/badoption"
)

func TestLoadBalancePrimary(t *testing.T) {
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
				Tag:  "direct-out",
			},
			{
				Type: C.TypeLoadBalance,
				Tag:  "lb-out",
				Options: &option.LoadBalanceOutboundOptions{
					PrimaryOutbounds: []string{"direct-out"},
					URL:             "http://127.0.0.1:1",
					Interval:        badoption.Duration(300),
					Timeout:         badoption.Duration(100),
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
								Outbound: "lb-out",
							},
						},
					},
				},
			},
		},
	})
	testTCP(t, clientPort, testPort)
}

func TestLoadBalanceBackupFallback(t *testing.T) {
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
				Tag:  "primary-out",
			},
			{
				Type: C.TypeDirect,
				Tag:  "backup-out",
			},
			{
				Type: C.TypeLoadBalance,
				Tag:  "lb-out",
				Options: &option.LoadBalanceOutboundOptions{
					PrimaryOutbounds: []string{"primary-out"},
					BackupOutbounds:  []string{"backup-out"},
					URL:             "http://127.0.0.1:1",
					Interval:        badoption.Duration(300),
					Timeout:         badoption.Duration(100),
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
								Outbound: "lb-out",
							},
						},
					},
				},
			},
		},
	})
	testTCP(t, clientPort, testPort)
}

func TestLoadBalanceEmptyPoolError(t *testing.T) {
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
				Tag:  "out-1",
			},
			{
				Type: C.TypeDirect,
				Tag:  "out-2",
			},
			{
				Type: C.TypeLoadBalance,
				Tag:  "lb-out",
				Options: &option.LoadBalanceOutboundOptions{
					PrimaryOutbounds: []string{"out-1", "out-2"},
					URL:             "http://127.0.0.1:1",
					Interval:        badoption.Duration(300),
					Timeout:         badoption.Duration(100),
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
								Outbound: "lb-out",
							},
						},
					},
				},
			},
		},
	})
	testTCP(t, clientPort, testPort)
}

func TestLoadBalanceConsistentHash(t *testing.T) {
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
				Tag:  "out-1",
			},
			{
				Type: C.TypeDirect,
				Tag:  "out-2",
			},
			{
				Type: C.TypeLoadBalance,
				Tag:  "lb-out",
				Options: &option.LoadBalanceOutboundOptions{
					PrimaryOutbounds: []string{"out-1", "out-2"},
					Strategy:        "consistent_hash",
					Hash: &option.LoadBalanceHashOptions{
						KeyParts:     []string{"src_ip"},
						VirtualNodes: 50,
					},
					URL:      "http://127.0.0.1:1",
					Interval: badoption.Duration(300),
					Timeout:  badoption.Duration(100),
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
								Outbound: "lb-out",
							},
						},
					},
				},
			},
		},
	})
	testTCP(t, clientPort, testPort)
}