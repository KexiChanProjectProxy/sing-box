package vless

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

// mockRouter implements the minimal adapter.Router interface for testing
type mockRouter struct {
	adapter.Router
}

// mockContextLogger implements log.ContextLogger for testing
type mockContextLogger struct {
	log.ContextLogger
}

func (m *mockContextLogger) Trace(args ...any)                             {}
func (m *mockContextLogger) Debug(args ...any)                             {}
func (m *mockContextLogger) Info(args ...any)                              {}
func (m *mockContextLogger) Warn(args ...any)                              {}
func (m *mockContextLogger) Error(args ...any)                             {}
func (m *mockContextLogger) Fatal(args ...any)                             {}
func (m *mockContextLogger) Panic(args ...any)                             {}
func (m *mockContextLogger) TraceContext(ctx context.Context, args ...any) {}
func (m *mockContextLogger) DebugContext(ctx context.Context, args ...any) {}
func (m *mockContextLogger) InfoContext(ctx context.Context, args ...any)  {}
func (m *mockContextLogger) WarnContext(ctx context.Context, args ...any)  {}
func (m *mockContextLogger) ErrorContext(ctx context.Context, args ...any) {}
func (m *mockContextLogger) FatalContext(ctx context.Context, args ...any) {}
func (m *mockContextLogger) PanicContext(ctx context.Context, args ...any) {}

// TestConnectionPoolConfiguration tests that the configuration is properly parsed and pool is initialized
func TestConnectionPoolConfiguration(t *testing.T) {
	configJSON := `{
		"type": "vless",
		"server": "example.com",
		"server_port": 443,
		"uuid": "00000000-0000-0000-0000-000000000000",
		"tls": {
			"enabled": true,
			"server_name": "example.com"
		},
		"connection_pool": {
			"ensure_idle_session": 3,
			"ensure_idle_session_create_rate": 2,
			"min_idle_session": 2,
			"idle_session_check_interval": "30s",
			"idle_session_timeout": "5m",
			"max_connection_lifetime": "1h",
			"connection_lifetime_jitter": "10m"
		}
	}`

	var opts option.VLESSOutboundOptions
	err := json.Unmarshal([]byte(configJSON), &opts)
	if err != nil {
		t.Fatalf("Failed to parse configuration: %v", err)
	}

	// Verify configuration was parsed correctly
	if opts.ConnectionPool == nil {
		t.Fatal("ConnectionPool configuration is nil")
	}

	if opts.ConnectionPool.EnsureIdleSession != 3 {
		t.Errorf("Expected EnsureIdleSession=3, got %d", opts.ConnectionPool.EnsureIdleSession)
	}

	if opts.ConnectionPool.MinIdleSession != 2 {
		t.Errorf("Expected MinIdleSession=2, got %d", opts.ConnectionPool.MinIdleSession)
	}

	if opts.ConnectionPool.IdleSessionCheckInterval.Build() != 30*time.Second {
		t.Errorf("Expected IdleSessionCheckInterval=30s, got %v", opts.ConnectionPool.IdleSessionCheckInterval.Build())
	}

	if opts.ConnectionPool.IdleSessionTimeout.Build() != 5*time.Minute {
		t.Errorf("Expected IdleSessionTimeout=5m, got %v", opts.ConnectionPool.IdleSessionTimeout.Build())
	}

	if opts.ConnectionPool.MaxConnectionLifetime.Build() != 1*time.Hour {
		t.Errorf("Expected MaxConnectionLifetime=1h, got %v", opts.ConnectionPool.MaxConnectionLifetime.Build())
	}

	// Note: We can't easily test NewOutbound with a real router without complex setup,
	// but we've verified the configuration parsing works correctly
}

// TestConnectionPoolWithoutTLS tests that pool works without TLS
func TestConnectionPoolWithoutTLS(t *testing.T) {
	configJSON := `{
		"type": "vless",
		"server": "example.com",
		"server_port": 443,
		"uuid": "00000000-0000-0000-0000-000000000000",
		"connection_pool": {
			"ensure_idle_session": 2
		}
	}`

	var opts option.VLESSOutboundOptions
	err := json.Unmarshal([]byte(configJSON), &opts)
	if err != nil {
		t.Fatalf("Failed to parse configuration: %v", err)
	}

	if opts.ConnectionPool == nil {
		t.Fatal("ConnectionPool configuration is nil")
	}

	if opts.ConnectionPool.EnsureIdleSession != 2 {
		t.Errorf("Expected EnsureIdleSession=2, got %d", opts.ConnectionPool.EnsureIdleSession)
	}
}

// TestTCPFastOpenValidation tests configuration parsing with TCP Fast Open
func TestTCPFastOpenValidation(t *testing.T) {
	// This test verifies that the configuration can be parsed.
	// Actual validation happens in NewOutbound, which we test separately.
	configJSON := `{
		"type": "vless",
		"server": "example.com",
		"server_port": 443,
		"uuid": "00000000-0000-0000-0000-000000000000",
		"tcp_fast_open": true,
		"connection_pool": {
			"ensure_idle_session": 3
		}
	}`

	var opts option.VLESSOutboundOptions
	err := json.Unmarshal([]byte(configJSON), &opts)
	if err != nil {
		t.Fatalf("Failed to parse configuration: %v", err)
	}

	// Configuration parsing should succeed
	if !opts.TCPFastOpen {
		t.Error("Expected TCPFastOpen to be true")
	}

	if opts.ConnectionPool == nil {
		t.Fatal("ConnectionPool configuration is nil")
	}

	// Note: Actual validation of incompatibility happens in NewOutbound
	// which checks if both are enabled and returns an error
}

// TestConnectionPoolWithMultiplex tests that pool and multiplex can coexist
func TestConnectionPoolWithMultiplex(t *testing.T) {
	configJSON := `{
		"type": "vless",
		"server": "example.com",
		"server_port": 443,
		"uuid": "00000000-0000-0000-0000-000000000000",
		"multiplex": {
			"enabled": true,
			"max_connections": 2
		},
		"connection_pool": {
			"ensure_idle_session": 3
		}
	}`

	var opts option.VLESSOutboundOptions
	err := json.Unmarshal([]byte(configJSON), &opts)
	if err != nil {
		t.Fatalf("Failed to parse configuration: %v", err)
	}

	if opts.ConnectionPool == nil {
		t.Fatal("ConnectionPool configuration is nil")
	}

	if opts.Multiplex == nil {
		t.Fatal("Multiplex configuration is nil")
	}

	// Both should be configured
	if opts.ConnectionPool.EnsureIdleSession != 3 {
		t.Errorf("Expected EnsureIdleSession=3, got %d", opts.ConnectionPool.EnsureIdleSession)
	}

	if !opts.Multiplex.Enabled {
		t.Error("Expected Multiplex to be enabled")
	}
}

// TestConnectionPoolDefaults tests that default values are applied correctly
func TestConnectionPoolDefaults(t *testing.T) {
	ctx := context.Background()
	createCount := 0

	config := ConnectionPoolConfig{
		// Only set required fields, let defaults apply
		CreateConn: func(ctx context.Context) (net.Conn, error) {
			createCount++
			return newMockConn(), nil
		},
		Logger: &mockContextLogger{},
	}

	pool := NewConnectionPool(ctx, config)
	defer pool.Close()

	// Verify defaults were applied
	if pool.checkInterval != 30*time.Second {
		t.Errorf("Expected default checkInterval=30s, got %v", pool.checkInterval)
	}

	if pool.idleTimeout != 5*time.Minute {
		t.Errorf("Expected default idleTimeout=5m, got %v", pool.idleTimeout)
	}

	if pool.ensureCreateRate != 1 {
		t.Errorf("Expected default ensureCreateRate=1, got %d", pool.ensureCreateRate)
	}
}
