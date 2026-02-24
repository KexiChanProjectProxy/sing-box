package dialer

import (
	"context"
	"net/netip"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	M "github.com/sagernet/sing/common/metadata"
)

func TestPerUserIPv6Hash(t *testing.T) {
	// Test with /48 prefix
	prefix := netip.MustParsePrefix("2001:db8:abcd::/48")
	dialer := &perUserIPv6Dialer{
		prefix: prefix,
	}

	// Test 1: Same user + source IP = same IPv6
	ctx1 := adapter.WithContext(context.Background(), &adapter.InboundContext{
		User:   "alice",
		Source: M.SocksaddrFrom(netip.MustParseAddr("192.168.1.1"), 0),
	})
	addr1 := dialer.computeIPv6Address(ctx1)

	ctx2 := adapter.WithContext(context.Background(), &adapter.InboundContext{
		User:   "alice",
		Source: M.SocksaddrFrom(netip.MustParseAddr("192.168.1.1"), 0),
	})
	addr2 := dialer.computeIPv6Address(ctx2)

	if addr1 != addr2 {
		t.Errorf("Same user+source should produce same IPv6: %s != %s", addr1, addr2)
	}

	// Test 2: Different user = different IPv6
	ctx3 := adapter.WithContext(context.Background(), &adapter.InboundContext{
		User:   "bob",
		Source: M.SocksaddrFrom(netip.MustParseAddr("192.168.1.1"), 0),
	})
	addr3 := dialer.computeIPv6Address(ctx3)

	if addr1 == addr3 {
		t.Errorf("Different users should produce different IPv6: %s == %s", addr1, addr3)
	}

	// Test 3: Different source IP = different IPv6
	ctx4 := adapter.WithContext(context.Background(), &adapter.InboundContext{
		User:   "alice",
		Source: M.SocksaddrFrom(netip.MustParseAddr("192.168.1.2"), 0),
	})
	addr4 := dialer.computeIPv6Address(ctx4)

	if addr1 == addr4 {
		t.Errorf("Different source IPs should produce different IPv6: %s == %s", addr1, addr4)
	}

	// Test 4: Verify prefix is preserved
	if !prefix.Contains(addr1) {
		t.Errorf("Generated address %s is not in prefix %s", addr1, prefix)
	}
	if !prefix.Contains(addr3) {
		t.Errorf("Generated address %s is not in prefix %s", addr3, prefix)
	}
	if !prefix.Contains(addr4) {
		t.Errorf("Generated address %s is not in prefix %s", addr4, prefix)
	}

	// Test 5: No user = zero address
	ctx5 := adapter.WithContext(context.Background(), &adapter.InboundContext{
		Source: M.SocksaddrFrom(netip.MustParseAddr("192.168.1.1"), 0),
	})
	addr5 := dialer.computeIPv6Address(ctx5)

	if addr5.IsValid() {
		t.Errorf("No user should return invalid address, got: %s", addr5)
	}

	// Test 6: Nil context = zero address
	addr6 := dialer.computeIPv6Address(context.Background())
	if addr6.IsValid() {
		t.Errorf("Nil InboundContext should return invalid address, got: %s", addr6)
	}
}

func TestPerUserIPv6DifferentPrefixes(t *testing.T) {
	testCases := []struct {
		prefix string
		bits   int
	}{
		{"2001:db8::/32", 32},
		{"2001:db8:abcd::/48", 48},
		{"2001:db8:abcd:1234::/64", 64},
		{"2001:db8:abcd:1234:5678::/80", 80},
		{"2001:db8:abcd:1234:5678:90ab::/96", 96},
	}

	for _, tc := range testCases {
		t.Run(tc.prefix, func(t *testing.T) {
			prefix := netip.MustParsePrefix(tc.prefix)
			dialer := &perUserIPv6Dialer{
				prefix: prefix,
			}

			ctx := adapter.WithContext(context.Background(), &adapter.InboundContext{
				User:   "testuser",
				Source: M.SocksaddrFrom(netip.MustParseAddr("10.0.0.1"), 0),
			})

			addr := dialer.computeIPv6Address(ctx)

			if !addr.IsValid() {
				t.Errorf("Failed to generate address for prefix %s", tc.prefix)
			}

			if !prefix.Contains(addr) {
				t.Errorf("Generated address %s is not in prefix %s", addr, prefix)
			}

			// Verify prefix bits are preserved
			addrBytes := addr.As16()
			prefixBytes := prefix.Addr().As16()
			fullBytes := tc.bits / 8
			for i := 0; i < fullBytes; i++ {
				if addrBytes[i] != prefixBytes[i] {
					t.Errorf("Prefix byte %d mismatch: got %x, want %x", i, addrBytes[i], prefixBytes[i])
				}
			}

			// Check remaining bits if not byte-aligned
			if tc.bits%8 != 0 {
				remainingBits := tc.bits % 8
				mask := byte(0xFF << (8 - remainingBits))
				if (addrBytes[fullBytes] & mask) != (prefixBytes[fullBytes] & mask) {
					t.Errorf("Prefix boundary byte %d mismatch: got %x, want %x (mask %x)",
						fullBytes, addrBytes[fullBytes]&mask, prefixBytes[fullBytes]&mask, mask)
				}
			}
		})
	}
}

func TestPerUserIPv6Uniqueness(t *testing.T) {
	prefix := netip.MustParsePrefix("2001:db8::/48")
	dialer := &perUserIPv6Dialer{
		prefix: prefix,
	}

	// Generate many addresses and check for collisions
	seen := make(map[netip.Addr]bool)
	users := []string{"alice", "bob", "charlie", "david", "eve"}
	sources := []string{"192.168.1.1", "192.168.1.2", "10.0.0.1", "172.16.0.1", "8.8.8.8"}

	for _, user := range users {
		for _, source := range sources {
			ctx := adapter.WithContext(context.Background(), &adapter.InboundContext{
				User:   user,
				Source: M.SocksaddrFrom(netip.MustParseAddr(source), 0),
			})
			addr := dialer.computeIPv6Address(ctx)

			if !addr.IsValid() {
				t.Errorf("Failed to generate address for user=%s, source=%s", user, source)
				continue
			}

			if seen[addr] {
				t.Errorf("Hash collision detected: user=%s, source=%s produced duplicate address %s",
					user, source, addr)
			}
			seen[addr] = true
		}
	}

	expectedCount := len(users) * len(sources)
	if len(seen) != expectedCount {
		t.Errorf("Expected %d unique addresses, got %d", expectedCount, len(seen))
	}
}
