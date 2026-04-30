package vless

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sagernet/sing-box/option"
)

func TestLegacyConnectionPoolRejected(t *testing.T) {
	configJSON := `{
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
	decoder := json.NewDecoder(strings.NewReader(configJSON))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&opts)
	if err == nil {
		t.Fatal("Expected error when parsing config with connection_pool, but got nil")
	}
	if !strings.Contains(err.Error(), "connection_pool") {
		t.Fatalf("Expected error to mention 'connection_pool', got: %v", err)
	}
}

func TestLegacyConnectionPoolRejectedMinimal(t *testing.T) {
	configJSON := `{
		"server": "example.com",
		"server_port": 443,
		"uuid": "00000000-0000-0000-0000-000000000000",
		"connection_pool": {
			"ensure_idle_session": 3
		}
	}`

	var opts option.VLESSOutboundOptions
	decoder := json.NewDecoder(strings.NewReader(configJSON))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&opts)
	if err == nil {
		t.Fatal("Expected error when parsing config with connection_pool, but got nil")
	}
	if !strings.Contains(err.Error(), "connection_pool") {
		t.Fatalf("Expected error to mention 'connection_pool', got: %v", err)
	}
}

func TestValidVLESSOutboundOptionsWithoutPool(t *testing.T) {
	configJSON := `{
		"server": "example.com",
		"server_port": 443,
		"uuid": "00000000-0000-0000-0000-000000000000",
		"tls": {
			"enabled": true,
			"server_name": "example.com"
		},
		"tcp_fast_open": true,
		"multiplex": {
			"enabled": true,
			"max_connections": 4
		}
	}`

	var opts option.VLESSOutboundOptions
	err := json.Unmarshal([]byte(configJSON), &opts)
	if err != nil {
		t.Fatalf("Failed to parse VLESS config: %v", err)
	}

	if opts.UUID != "00000000-0000-0000-0000-000000000000" {
		t.Errorf("Expected UUID=00000000-0000-0000-0000-000000000000, got %s", opts.UUID)
	}

	if opts.Server != "example.com" {
		t.Errorf("Expected Server=example.com, got %s", opts.Server)
	}

	if opts.ServerPort != 443 {
		t.Errorf("Expected ServerPort=443, got %d", opts.ServerPort)
	}

	if !opts.TLS.Enabled {
		t.Error("Expected TLS.Enabled=true")
	}

	if opts.TLS.ServerName != "example.com" {
		t.Errorf("Expected TLS.ServerName=example.com, got %s", opts.TLS.ServerName)
	}

	if !opts.TCPFastOpen {
		t.Error("Expected TCPFastOpen=true")
	}

	if opts.Multiplex == nil {
		t.Fatal("Expected Multiplex to be non-nil")
	}

	if !opts.Multiplex.Enabled {
		t.Error("Expected Multiplex.Enabled=true")
	}

	if opts.Multiplex.MaxConnections != 4 {
		t.Errorf("Expected Multiplex.MaxConnections=4, got %d", opts.Multiplex.MaxConnections)
	}
}

func TestMultiplexWithoutPoolStillWorks(t *testing.T) {
	configJSON := `{
		"server": "example.com",
		"server_port": 443,
		"uuid": "00000000-0000-0000-0000-000000000000",
		"multiplex": {
			"enabled": true,
			"max_connections": 2,
			"min_streams": 4
		}
	}`

	var opts option.VLESSOutboundOptions
	err := json.Unmarshal([]byte(configJSON), &opts)
	if err != nil {
		t.Fatalf("Failed to parse VLESS config with multiplex: %v", err)
	}

	if opts.Multiplex == nil {
		t.Fatal("Expected Multiplex to be non-nil")
	}

	if !opts.Multiplex.Enabled {
		t.Error("Expected Multiplex.Enabled=true")
	}

	if opts.Multiplex.MaxConnections != 2 {
		t.Errorf("Expected MaxConnections=2, got %d", opts.Multiplex.MaxConnections)
	}

	if opts.Multiplex.MinStreams != 4 {
		t.Errorf("Expected MinStreams=4, got %d", opts.Multiplex.MinStreams)
	}
}