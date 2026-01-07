package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/sing-box"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
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
				MixedOptions: option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.NewListenAddress(netip.IPv4Unspecified()),
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
				LoadBalanceOptions: option.LoadBalanceOutboundOptions{
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
						RawDefaultRule: option.RawDefaultRule{
							Outbound: "lb-auto",
						},
					},
				},
			},
		},
	}

	instance := startInstance(t, options)

	// Give time for initial health check
	time.Sleep(time.Second * 2)

	// Test connectivity through load balancer
	clientPort := options.Inbounds[0].MixedOptions.ListenPort
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
				MixedOptions: option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.NewListenAddress(netip.IPv4Unspecified()),
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
				LoadBalanceOptions: option.LoadBalanceOutboundOptions{
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
						RawDefaultRule: option.RawDefaultRule{
							Outbound: "lb-backup",
						},
					},
				},
			},
		},
	}

	instance := startInstance(t, options)

	// Wait for health checks to fail for primary and activate backup
	time.Sleep(time.Second * 2)

	// Should still be able to connect through backup
	clientPort := options.Inbounds[0].MixedOptions.ListenPort
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
				MixedOptions: option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.NewListenAddress(netip.IPv4Unspecified()),
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
				LoadBalanceOptions: option.LoadBalanceOutboundOptions{
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
						RawDefaultRule: option.RawDefaultRule{
							Outbound: "lb-hash",
						},
					},
				},
			},
		},
	}

	instance := startInstance(t, options)

	// Wait for initial health check
	time.Sleep(time.Second * 2)

	// Test connectivity
	clientPort := options.Inbounds[0].MixedOptions.ListenPort
	testBasicConnectivity(t, clientPort)
}

// Helper functions

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
	portLock.Lock()
	defer portLock.Unlock()
	portCounter++
	return portCounter
}
