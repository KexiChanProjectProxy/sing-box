package option

import (
	"encoding/json"
	"testing"
)

func TestHysteria2Realm_UpstreamFields(t *testing.T) {
	type testCase struct {
		name            string
		json            string
		wantServerURL   string
		wantToken       string
		wantRealmID     string
		wantErr         bool
		wantErrContains string
	}

	tests := []testCase{
		{
			name:          "minimal realm with required fields",
			json:          `{"server_url": "hy2://example.com:8443", "realm_id": "test-realm"}`,
			wantServerURL: "hy2://example.com:8443",
			wantRealmID:   "test-realm",
			wantErr:       false,
		},
		{
			name:          "realm with token",
			json:          `{"server_url": "hy2://example.com:8443", "token": "secret-token", "realm_id": "test-realm"}`,
			wantServerURL: "hy2://example.com:8443",
			wantToken:     "secret-token",
			wantRealmID:   "test-realm",
			wantErr:       false,
		},
		{
			name:          "realm with STUN servers array",
			json:          `{"server_url": "hy2://example.com:8443", "realm_id": "test-realm", "stun_servers": ["stun.example.com:3478", "stun2.example.com:3478"]}`,
			wantServerURL: "hy2://example.com:8443",
			wantRealmID:   "test-realm",
			wantErr:       false,
		},
		{
			name:          "realm with single STUN server as string",
			json:          `{"server_url": "hy2://example.com:8443", "realm_id": "test-realm", "stun_servers": "stun.example.com:3478"}`,
			wantServerURL: "hy2://example.com:8443",
			wantRealmID:   "test-realm",
			wantErr:       false,
		},
		{
			name:            "realm missing server_url",
			json:            `{"realm_id": "test-realm"}`,
			wantErr:         true,
			wantErrContains: "server_url",
		},
		{
			name:            "realm missing realm_id",
			json:            `{"server_url": "hy2://example.com:8443"}`,
			wantErr:         true,
			wantErrContains: "realm_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var realm Hysteria2InboundRealm
			err := json.Unmarshal([]byte(tt.json), &realm)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unmarshal error = %v, want nil", err)
			}
			if realm.ServerURL != tt.wantServerURL {
				t.Errorf("ServerURL = %q, want %q", realm.ServerURL, tt.wantServerURL)
			}
			if realm.Token != tt.wantToken {
				t.Errorf("Token = %q, want %q", realm.Token, tt.wantToken)
			}
			if realm.RealmID != tt.wantRealmID {
				t.Errorf("RealmID = %q, want %q", realm.RealmID, tt.wantRealmID)
			}
		})
	}
}

func TestHysteria2Realm_KexiListenPorts(t *testing.T) {
	type testCase struct {
		name        string
		json        string
		wantPorts   []uint16
		wantErr     bool
		wantErrText string
	}

	tests := []testCase{
		{
			name:      "valid single port",
			json:      `{"server_url": "hy2://example.com:8443", "realm_id": "test", "listen_ports": [8080]}`,
			wantPorts: []uint16{8080},
			wantErr:   false,
		},
		{
			name:      "valid port range",
			json:      `{"server_url": "hy2://example.com:8443", "realm_id": "test", "listen_ports": [8080, 8081, 8082]}`,
			wantPorts: []uint16{8080, 8081, 8082},
			wantErr:   false,
		},
		{
			name:      "valid port range as string",
			json:      `{"server_url": "hy2://example.com:8443", "realm_id": "test", "listen_ports": "8080-8089"}`,
			wantPorts: []uint16{8080, 8081, 8082, 8083, 8084, 8085, 8086, 8087, 8088, 8089},
			wantErr:   false,
		},
		{
			name:        "invalid port range start greater than end",
			json:        `{"server_url": "hy2://example.com:8443", "realm_id": "test", "listen_ports": "9000-1000"}`,
			wantErr:     true,
			wantErrText: "port",
		},
		{
			name:        "invalid port number too high",
			json:        `{"server_url": "hy2://example.com:8443", "realm_id": "test", "listen_ports": [70000]}`,
			wantErr:     true,
			wantErrText: "port",
		},
		{
			name:        "invalid port number zero",
			json:        `{"server_url": "hy2://example.com:8443", "realm_id": "test", "listen_ports": [0]}`,
			wantErr:     true,
			wantErrText: "port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var realm Hysteria2InboundRealm
			err := json.Unmarshal([]byte(tt.json), &realm)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unmarshal error = %v, want nil", err)
			}
			if len(realm.ListenPorts) != len(tt.wantPorts) {
				t.Errorf("ListenPorts len = %d, want %d", len(realm.ListenPorts), len(tt.wantPorts))
				return
			}
			for i := range realm.ListenPorts {
				if realm.ListenPorts[i] != tt.wantPorts[i] {
					t.Errorf("ListenPorts[%d] = %d, want %d", i, realm.ListenPorts[i], tt.wantPorts[i])
				}
			}
		})
	}
}

func TestHysteria2Realm_KexiPreferIPVersion(t *testing.T) {
	type testCase struct {
		name      string
		json      string
		wantValue string
		wantErr   bool
	}

	tests := []testCase{
		{
			name:      "valid prefer_ipv4",
			json:      `{"server_url": "hy2://example.com:8443", "realm_id": "test", "prefer_ip_version": "prefer_ipv4"}`,
			wantValue: "prefer_ipv4",
			wantErr:   false,
		},
		{
			name:      "valid prefer_ipv6",
			json:      `{"server_url": "hy2://example.com:8443", "realm_id": "test", "prefer_ip_version": "prefer_ipv6"}`,
			wantValue: "prefer_ipv6",
			wantErr:   false,
		},
		{
			name:      "valid ipv4_only",
			json:      `{"server_url": "hy2://example.com:8443", "realm_id": "test", "prefer_ip_version": "ipv4_only"}`,
			wantValue: "ipv4_only",
			wantErr:   false,
		},
		{
			name:      "valid ipv6_only",
			json:      `{"server_url": "hy2://example.com:8443", "realm_id": "test", "prefer_ip_version": "ipv6_only"}`,
			wantValue: "ipv6_only",
			wantErr:   false,
		},
		{
			name:    "invalid prefer value",
			json:    `{"server_url": "hy2://example.com:8443", "realm_id": "test", "prefer_ip_version": "prefer_ip"}`,
			wantErr: true,
		},
		{
			name:    "invalid numeric value",
			json:    `{"server_url": "hy2://example.com:8443", "realm_id": "test", "prefer_ip_version": 4}`,
			wantErr: true,
		},
		{
			name:    "invalid empty string",
			json:    `{"server_url": "hy2://example.com:8443", "realm_id": "test", "prefer_ip_version": ""}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var realm Hysteria2InboundRealm
			err := json.Unmarshal([]byte(tt.json), &realm)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unmarshal error = %v, want nil", err)
			}
			if realm.PreferIPVersion != tt.wantValue {
				t.Errorf("PreferIPVersion = %q, want %q", realm.PreferIPVersion, tt.wantValue)
			}
		})
	}
}

func TestHysteria2Realm_KexiFallbackTimeout(t *testing.T) {
	type testCase struct {
		name      string
		json      string
		wantValue string
		wantErr   bool
	}

	tests := []testCase{
		{
			name:      "valid duration string",
			json:      `{"server_url": "hy2://example.com:8443", "realm_id": "test", "fallback_timeout": "5s"}`,
			wantValue: "5s",
			wantErr:   false,
		},
		{
			name:      "valid zero duration",
			json:      `{"server_url": "hy2://example.com:8443", "realm_id": "test", "fallback_timeout": "0s"}`,
			wantValue: "0s",
			wantErr:   false,
		},
		{
			name:      "valid duration in milliseconds",
			json:      `{"server_url": "hy2://example.com:8443", "realm_id": "test", "fallback_timeout": "500ms"}`,
			wantValue: "500ms",
			wantErr:   false,
		},
		{
			name:      "valid numeric seconds",
			json:      `{"server_url": "hy2://example.com:8443", "realm_id": "test", "fallback_timeout": 10}`,
			wantValue: "10s",
			wantErr:   false,
		},
		{
			name:    "invalid negative duration",
			json:    `{"server_url": "hy2://example.com:8443", "realm_id": "test", "fallback_timeout": "-1s"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var realm Hysteria2InboundRealm
			err := json.Unmarshal([]byte(tt.json), &realm)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unmarshal error = %v, want nil", err)
			}
			if realm.FallbackTimeout != tt.wantValue {
				t.Errorf("FallbackTimeout = %q, want %q", realm.FallbackTimeout, tt.wantValue)
			}
		})
	}
}

func TestHysteria2Realm_ListenPortsAndPreferIPCombined(t *testing.T) {
	type testCase struct {
		name           string
		json           string
		wantPorts      []uint16
		wantPreferIP   string
		wantFallback   string
		wantErr        bool
	}

	tests := []testCase{
		{
			name:         "valid combined port range with prefer_ipv4",
			json:         `{"server_url": "hy2://example.com:8443", "realm_id": "test", "listen_ports": "8000-8002", "prefer_ip_version": "prefer_ipv4"}`,
			wantPorts:    []uint16{8000, 8001, 8002},
			wantPreferIP: "prefer_ipv4",
			wantErr:      false,
		},
		{
			name:         "valid combined with fallback_timeout",
			json:         `{"server_url": "hy2://example.com:8443", "realm_id": "test", "listen_ports": [8080, 8081], "prefer_ip_version": "prefer_ipv6", "fallback_timeout": "3s"}`,
			wantPorts:    []uint16{8080, 8081},
			wantPreferIP: "prefer_ipv6",
			wantFallback: "3s",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var realm Hysteria2InboundRealm
			err := json.Unmarshal([]byte(tt.json), &realm)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unmarshal error = %v, want nil", err)
			}
			if len(realm.ListenPorts) != len(tt.wantPorts) {
				t.Errorf("ListenPorts len = %d, want %d", len(realm.ListenPorts), len(tt.wantPorts))
			}
			for i := range realm.ListenPorts {
				if realm.ListenPorts[i] != tt.wantPorts[i] {
					t.Errorf("ListenPorts[%d] = %d, want %d", i, realm.ListenPorts[i], tt.wantPorts[i])
				}
			}
			if realm.PreferIPVersion != tt.wantPreferIP {
				t.Errorf("PreferIPVersion = %q, want %q", realm.PreferIPVersion, tt.wantPreferIP)
			}
			if realm.FallbackTimeout != tt.wantFallback {
				t.Errorf("FallbackTimeout = %q, want %q", realm.FallbackTimeout, tt.wantFallback)
			}
		})
	}
}


