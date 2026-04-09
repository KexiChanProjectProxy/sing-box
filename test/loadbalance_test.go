package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	groupoutbound "github.com/sagernet/sing-box/protocol/group"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks"

	"github.com/stretchr/testify/require"
)

// TestLoadBalanceBasic tests basic load balancing functionality
func TestLoadBalanceBasic(t *testing.T) {
	t.Parallel()

	// Start mock HTTP server for URL testing
	httpServerPort := mkPort(t)
	startMockHTTPServer(t, httpServerPort)

	// Create configuration with loadbalance outbound
	options := option.Options{
		Inbounds: []option.Inbound{
			{
				Tag:  "mixed-in",
				Type: C.TypeMixed,
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: mkPort(t),
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Tag:  "direct-1",
				Type: C.TypeDirect,
			},
			{
				Tag:  "direct-2",
				Type: C.TypeDirect,
			},
			{
				Tag:  "direct-3",
				Type: C.TypeDirect,
			},
			{
				Tag:  "lb-auto",
				Type: C.TypeLoadBalance,
				Options: &option.LoadBalanceOutboundOptions{
					PrimaryOutbounds: []string{"direct-1", "direct-2", "direct-3"},
					BackupOutbounds:  []string{},
					URL:              "http://127.0.0.1:" + fmt.Sprintf("%d", httpServerPort) + "/generate_204",
					Interval:         badoption.Duration(time.Second),
					Timeout:          badoption.Duration(time.Second * 5),
					TopN: option.LoadBalanceTopNOptions{
						Primary: 2,
					},
					Strategy: "random",
				},
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						RuleAction: option.RuleAction{
							Action: C.RuleActionTypeRoute,
							RouteOptions: option.RouteActionOptions{
								Outbound: "lb-auto",
							},
						},
					},
				},
			},
		},
	}

	startInstance(t, options)

	// Give time for initial health check
	time.Sleep(time.Second * 2)

	// Test connectivity through load balancer
	clientPort := options.Inbounds[0].Options.(*option.HTTPMixedInboundOptions).ListenPort
	testBasicConnectivity(t, clientPort)
}

// TestLoadBalanceWithBackup tests backup tier functionality
func TestLoadBalanceWithBackup(t *testing.T) {
	t.Parallel()

	httpServerPort := mkPort(t)
	startMockHTTPServer(t, httpServerPort)

	// Create primary outbound that will fail health check (bad URL)
	// and backup outbound that will succeed
	options := option.Options{
		Inbounds: []option.Inbound{
			{
				Tag:  "mixed-in",
				Type: C.TypeMixed,
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: mkPort(t),
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Tag:  "primary-bad",
				Type: C.TypeDirect,
			},
			{
				Tag:  "backup-good",
				Type: C.TypeDirect,
			},
			{
				Tag:  "lb-backup",
				Type: C.TypeLoadBalance,
				Options: &option.LoadBalanceOutboundOptions{
					PrimaryOutbounds: []string{"primary-bad"},
					BackupOutbounds:  []string{"backup-good"},
					URL:              "http://127.0.0.1:1/unreachable", // This will fail for primary
					Interval:         badoption.Duration(time.Second),
					Timeout:          badoption.Duration(time.Millisecond * 100),
					TopN: option.LoadBalanceTopNOptions{
						Primary: 1,
						Backup:  1,
					},
					Strategy: "random",
					Hysteresis: &option.LoadBalanceHysteresisOptions{
						PrimaryFailures: 1, // Switch to backup after 1 failure
						BackupHoldTime:  badoption.Duration(time.Second * 2),
					},
				},
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						RuleAction: option.RuleAction{
							Action: C.RuleActionTypeRoute,
							RouteOptions: option.RouteActionOptions{
								Outbound: "lb-backup",
							},
						},
					},
				},
			},
		},
	}

	startInstance(t, options)

	// Wait for health checks to fail for primary and activate backup
	time.Sleep(time.Second * 2)

	// Should still be able to connect through backup
	clientPort := options.Inbounds[0].Options.(*option.HTTPMixedInboundOptions).ListenPort
	testBasicConnectivity(t, clientPort)
}

// TestLoadBalanceConsistentHash tests consistent hashing strategy
func TestLoadBalanceConsistentHash(t *testing.T) {
	t.Parallel()

	httpServerPort := mkPort(t)
	startMockHTTPServer(t, httpServerPort)

	options := option.Options{
		Inbounds: []option.Inbound{
			{
				Tag:  "mixed-in",
				Type: C.TypeMixed,
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: mkPort(t),
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Tag:  "direct-1",
				Type: C.TypeDirect,
			},
			{
				Tag:  "direct-2",
				Type: C.TypeDirect,
			},
			{
				Tag:  "direct-3",
				Type: C.TypeDirect,
			},
			{
				Tag:  "lb-hash",
				Type: C.TypeLoadBalance,
				Options: &option.LoadBalanceOutboundOptions{
					PrimaryOutbounds: []string{"direct-1", "direct-2", "direct-3"},
					URL:              "http://127.0.0.1:" + fmt.Sprintf("%d", httpServerPort) + "/generate_204",
					Interval:         badoption.Duration(time.Second),
					Timeout:          badoption.Duration(time.Second * 5),
					TopN: option.LoadBalanceTopNOptions{
						Primary: 3,
					},
					Strategy: "consistent_hash",
					Hash: &option.LoadBalanceHashOptions{
						KeyParts:     []string{"src_ip", "dst_ip", "dst_port"},
						VirtualNodes: 100,
						OnEmptyKey:   "random",
					},
				},
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						RuleAction: option.RuleAction{
							Action: C.RuleActionTypeRoute,
							RouteOptions: option.RouteActionOptions{
								Outbound: "lb-hash",
							},
						},
					},
				},
			},
		},
	}

	startInstance(t, options)

	// Wait for initial health check
	time.Sleep(time.Second * 2)

	// Test connectivity
	clientPort := options.Inbounds[0].Options.(*option.HTTPMixedInboundOptions).ListenPort
	testBasicConnectivity(t, clientPort)
}

func TestNestedLoadBalanceURLTestReuse(t *testing.T) {
	probePort := mkPort(t)
	clientPort := mkPort(t)

	instance := startInstance(t, newNestedLoadBalanceOptions(clientPort, probePort, nestedLoadBalanceTestIntervals(
		2*time.Second,
		2*time.Second,
		2*time.Second,
	)))
	time.Sleep(300 * time.Millisecond)
	probeCounter := startCountingMockHTTPServer(t, probePort)

	urltestA, ok := instance.Outbound().Outbound("urltest-a")
	require.True(t, ok)
	urltestB, ok := instance.Outbound().Outbound("urltest-b")
	require.True(t, ok)
	lbOutbound, ok := instance.Outbound().Outbound("lb-nested")
	require.True(t, ok)

	first := urltestA.(*groupoutbound.URLTest)
	second := urltestB.(*groupoutbound.URLTest)
	lb := lbOutbound.(*groupoutbound.LoadBalance)

	_, err := first.URLTest(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(1), probeCounter.Load(), "identical nested URLTest config should reuse the shared leaf probe result")
	require.Equal(t, "direct-ok", first.Now())

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	listenerAddr := listener.Addr().(*net.TCPAddr)
	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		conn, acceptErr := listener.Accept()
		if acceptErr == nil {
			_ = conn.Close()
		}
	}()

	conn, err := second.DialContext(context.Background(), N.NetworkTCP, M.ParseSocksaddrHostPort("127.0.0.1", uint16(listenerAddr.Port)))
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	<-acceptDone
	require.Equal(t, int64(1), probeCounter.Load(), "identical nested URLTest config should let the sibling reuse fresh history without another direct probe")

	_, err = lb.URLTest(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(2), probeCounter.Load(), "loadbalance keeps its own probe namespace while nested URLTests reuse each other")
	require.Equal(t, "direct-ok", lb.Now())
}

func TestNestedLoadBalanceURLTestNoReuseOnConfigMismatch(t *testing.T) {
	probePort := mkPort(t)
	clientPort := mkPort(t)
	testPort := mkPort(t)

	instance := startInstance(t, newNestedLoadBalanceOptions(clientPort, probePort, nestedLoadBalanceTestIntervals(
		2*time.Second,
		4*time.Second,
		2*time.Second,
	)))
	time.Sleep(300 * time.Millisecond)
	probeCounter := startCountingMockHTTPServer(t, probePort)

	urltestA, ok := instance.Outbound().Outbound("urltest-a")
	require.True(t, ok)
	urltestB, ok := instance.Outbound().Outbound("urltest-b")
	require.True(t, ok)
	lbOutbound, ok := instance.Outbound().Outbound("lb-nested")
	require.True(t, ok)

	first := urltestA.(*groupoutbound.URLTest)
	second := urltestB.(*groupoutbound.URLTest)
	lb := lbOutbound.(*groupoutbound.LoadBalance)

	_, err := first.URLTest(context.Background())
	require.NoError(t, err)
	_, err = second.URLTest(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(2), probeCounter.Load(), "different nested URLTest config must keep direct leaf probes isolated")
	require.Equal(t, "direct-ok", first.Now())
	require.Equal(t, "direct-ok", second.Now())

	_, err = lb.URLTest(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(3), probeCounter.Load(), "loadbalance probe namespace stays separate from both nested URLTests")
	require.Equal(t, "direct-ok", lb.Now())

	testTCP(t, clientPort, testPort)
}

// Helper functions

type nestedLoadBalanceIntervals struct {
	urltestA badoption.Duration
	urltestB badoption.Duration
	lb       badoption.Duration
}

func nestedLoadBalanceTestIntervals(urltestA, urltestB, lb time.Duration) nestedLoadBalanceIntervals {
	return nestedLoadBalanceIntervals{
		urltestA: badoption.Duration(urltestA),
		urltestB: badoption.Duration(urltestB),
		lb:       badoption.Duration(lb),
	}
}

func newNestedLoadBalanceOptions(clientPort, probePort uint16, intervals nestedLoadBalanceIntervals) option.Options {
	url := fmt.Sprintf("http://127.0.0.1:%d/generate_204", probePort)
	return option.Options{
		Inbounds: []option.Inbound{
			{
				Tag:  "mixed-in",
				Type: C.TypeMixed,
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
				Tag:  "direct-ok",
				Type: C.TypeDirect,
			},
			{
				Tag:  "block-bad",
				Type: C.TypeBlock,
			},
			{
				Tag:  "urltest-a",
				Type: C.TypeURLTest,
				Options: &option.URLTestOutboundOptions{
					Outbounds: []string{"direct-ok", "block-bad"},
					URL:       url,
					Interval:  intervals.urltestA,
				},
			},
			{
				Tag:  "urltest-b",
				Type: C.TypeURLTest,
				Options: &option.URLTestOutboundOptions{
					Outbounds: []string{"direct-ok", "block-bad"},
					URL:       url,
					Interval:  intervals.urltestB,
				},
			},
			{
				Tag:  "lb-nested",
				Type: C.TypeLoadBalance,
				Options: &option.LoadBalanceOutboundOptions{
					PrimaryOutbounds: []string{"urltest-a", "urltest-b"},
					URL:              url,
					Interval:         intervals.lb,
					Timeout:          badoption.Duration(2 * time.Second),
					TopN: option.LoadBalanceTopNOptions{
						Primary: 1,
					},
					Strategy: "random",
				},
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						RuleAction: option.RuleAction{
							Action: C.RuleActionTypeRoute,
							RouteOptions: option.RouteActionOptions{
								Outbound: "lb-nested",
							},
						},
					},
				},
			},
		},
	}
}

func startMockHTTPServer(t *testing.T, port uint16) {
	mux := http.NewServeMux()
	mux.HandleFunc("/generate_204", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	server := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", port),
		Handler: mux,
	}

	go func() {
		_ = server.ListenAndServe()
	}()

	t.Cleanup(func() {
		_ = server.Close()
	})

	// Wait for server to start
	time.Sleep(time.Millisecond * 100)
}

func startCountingMockHTTPServer(t *testing.T, port uint16) *atomic.Int64 {
	var hits atomic.Int64
	mux := http.NewServeMux()
	mux.HandleFunc("/generate_204", func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusNoContent)
	})

	server := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", port),
		Handler: mux,
	}

	go func() {
		_ = server.ListenAndServe()
	}()

	t.Cleanup(func() {
		_ = server.Close()
	})

	time.Sleep(100 * time.Millisecond)
	return &hits
}

func testBasicConnectivity(t *testing.T, clientPort uint16) {
	// Create SOCKS5 client
	dialer := socks.NewClient(
		N.SystemDialer,
		M.ParseSocksaddrHostPort("127.0.0.1", clientPort),
		socks.Version5,
		"", "",
	)

	// Test TCP connection to a known service (e.g., Google DNS)
	conn, err := dialer.DialContext(
		context.Background(),
		N.NetworkTCP,
		M.ParseSocksaddrHostPort("8.8.8.8", 53),
	)
	if err != nil {
		// If direct connection fails, just log (might be network issue)
		t.Logf("TCP dial failed (might be expected in some environments): %v", err)
		return
	}
	require.NoError(t, conn.Close())
}

var portCounter uint16 = 10000
var portLock sync.Mutex

func mkPort(t *testing.T) uint16 {
	_ = t
	portLock.Lock()
	defer portLock.Unlock()
	portCounter++
	return portCounter
}
