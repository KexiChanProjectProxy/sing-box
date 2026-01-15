package dialer

import (
	"net/netip"
	"testing"

	M "github.com/sagernet/sing/common/metadata"
)

func TestTranslateIPv4ToIPv6(t *testing.T) {
	tests := []struct {
		name     string
		ipv4     string
		prefix   string
		expected string
	}{
		{
			name:     "Standard translation",
			ipv4:     "104.16.184.241",
			prefix:   "2a0c:b641:69c:0:0:4::/96",
			expected: "2a0c:b641:69c:0:0:4:6810:b8f1",
		},
		{
			name:     "Loopback address",
			ipv4:     "127.0.0.1",
			prefix:   "64:ff9b::/96",
			expected: "64:ff9b::7f00:1",
		},
		{
			name:     "Private address 10.0.0.1",
			ipv4:     "10.0.0.1",
			prefix:   "64:ff9b::/96",
			expected: "64:ff9b::a00:1",
		},
		{
			name:     "Private address 192.168.1.1",
			ipv4:     "192.168.1.1",
			prefix:   "64:ff9b::/96",
			expected: "64:ff9b::c0a8:101",
		},
		{
			name:     "Broadcast address",
			ipv4:     "255.255.255.255",
			prefix:   "64:ff9b::/96",
			expected: "64:ff9b::ffff:ffff",
		},
		{
			name:     "Zero address",
			ipv4:     "0.0.0.0",
			prefix:   "64:ff9b::/96",
			expected: "64:ff9b::",
		},
		{
			name:     "Google DNS",
			ipv4:     "8.8.8.8",
			prefix:   "64:ff9b::/96",
			expected: "64:ff9b::808:808",
		},
		{
			name:     "Cloudflare DNS",
			ipv4:     "1.1.1.1",
			prefix:   "64:ff9b::/96",
			expected: "64:ff9b::101:101",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ipv4 := netip.MustParseAddr(tt.ipv4)
			prefix := netip.MustParsePrefix(tt.prefix)
			expected := netip.MustParseAddr(tt.expected)

			result := translateIPv4ToIPv6(ipv4, prefix)

			if result != expected {
				t.Errorf("Translation failed for %s:\n  got:      %v\n  expected: %v", tt.ipv4, result, expected)
			}

			// Verify the result is IPv6
			if !result.Is6() {
				t.Errorf("Result is not IPv6: %v", result)
			}

			// Verify the last 32 bits match the IPv4 address
			ipv4Bytes := ipv4.As4()
			resultBytes := result.As16()
			for i := 0; i < 4; i++ {
				if resultBytes[12+i] != ipv4Bytes[i] {
					t.Errorf("IPv4 embedding mismatch at byte %d: got %d, expected %d", i, resultBytes[12+i], ipv4Bytes[i])
				}
			}
		})
	}
}

func TestTranslateIPv4ToIPv6_PrefixPreservation(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
	}{
		{
			name:   "Well-known prefix",
			prefix: "64:ff9b::/96",
		},
		{
			name:   "Custom prefix 1",
			prefix: "2001:db8::/96",
		},
		{
			name:   "Custom prefix 2",
			prefix: "2a0c:b641:69c:4:0:4::/96",
		},
	}

	ipv4 := netip.MustParseAddr("192.0.2.1")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix := netip.MustParsePrefix(tt.prefix)
			result := translateIPv4ToIPv6(ipv4, prefix)

			// Verify prefix is preserved (first 96 bits)
			prefixAddr := prefix.Addr()
			prefixBytes := prefixAddr.As16()
			resultBytes := result.As16()

			for i := 0; i < 12; i++ { // First 96 bits = 12 bytes
				if resultBytes[i] != prefixBytes[i] {
					t.Errorf("Prefix not preserved at byte %d: got %d, expected %d", i, resultBytes[i], prefixBytes[i])
				}
			}
		})
	}
}

func TestXLAT464Dialer_TranslateDestination(t *testing.T) {
	prefix := netip.MustParsePrefix("64:ff9b::/96")
	dialer := &xlat464Dialer{
		prefix: prefix,
	}

	tests := []struct {
		name             string
		inputAddr        string
		inputPort        uint16
		expectedAddr     string
		shouldTranslate  bool
	}{
		{
			name:            "IPv4 address should be translated",
			inputAddr:       "8.8.8.8",
			inputPort:       53,
			expectedAddr:    "64:ff9b::808:808",
			shouldTranslate: true,
		},
		{
			name:            "IPv6 address should pass through",
			inputAddr:       "2001:4860:4860::8888",
			inputPort:       53,
			expectedAddr:    "2001:4860:4860::8888",
			shouldTranslate: false,
		},
		{
			name:            "Another IPv4 translation",
			inputAddr:       "1.1.1.1",
			inputPort:       443,
			expectedAddr:    "64:ff9b::101:101",
			shouldTranslate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputSocksaddr := M.ParseSocksaddrHostPort(tt.inputAddr, tt.inputPort)
			result := dialer.translateDestination(inputSocksaddr)

			expectedAddr := netip.MustParseAddr(tt.expectedAddr)
			if result.Addr != expectedAddr {
				t.Errorf("Address mismatch:\n  got:      %v\n  expected: %v", result.Addr, expectedAddr)
			}

			if result.Port != tt.inputPort {
				t.Errorf("Port mismatch: got %d, expected %d", result.Port, tt.inputPort)
			}

			// Verify translation occurred (or didn't) as expected
			wasTranslated := result.Addr != inputSocksaddr.Addr
			if wasTranslated != tt.shouldTranslate {
				if tt.shouldTranslate {
					t.Error("Expected translation to occur but it didn't")
				} else {
					t.Error("Expected no translation but translation occurred")
				}
			}
		})
	}
}

func TestXLAT464Dialer_DomainPassThrough(t *testing.T) {
	prefix := netip.MustParsePrefix("64:ff9b::/96")
	dialer := &xlat464Dialer{
		prefix: prefix,
	}

	// Domain names should not be translated (only IP addresses)
	destination := M.ParseSocksaddrHostPortStr("example.com", "80")
	result := dialer.translateDestination(destination)

	if destination != result {
		t.Errorf("Domain destination was modified:\n  original: %v\n  result:   %v", destination, result)
	}
}

func BenchmarkTranslateIPv4ToIPv6(b *testing.B) {
	ipv4 := netip.MustParseAddr("8.8.8.8")
	prefix := netip.MustParsePrefix("64:ff9b::/96")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = translateIPv4ToIPv6(ipv4, prefix)
	}
}
