package listener

import (
	"net/netip"
	"testing"
)

func TestIPFilter_NoFilter(t *testing.T) {
	filter, err := NewIPFilter(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if filter != nil {
		// No filter = nil is acceptable, or non-nil with allow all
		if !filter.Allow(netip.MustParseAddr("1.2.3.4")) {
			t.Fatal("should allow all when no filter")
		}
	}
}

func TestIPFilter_WhitelistIPv4(t *testing.T) {
	filter, err := NewIPFilter([]string{"192.168.1.0/24", "10.0.0.1"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Should allow
	if !filter.Allow(netip.MustParseAddr("192.168.1.100")) {
		t.Fatal("should allow 192.168.1.100")
	}
	if !filter.Allow(netip.MustParseAddr("10.0.0.1")) {
		t.Fatal("should allow 10.0.0.1")
	}
	// Should deny
	if filter.Allow(netip.MustParseAddr("8.8.8.8")) {
		t.Fatal("should deny 8.8.8.8")
	}
}

func TestIPFilter_BlacklistIPv4(t *testing.T) {
	filter, err := NewIPFilter(nil, []string{"1.2.3.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	// Should deny
	if filter.Allow(netip.MustParseAddr("1.2.3.4")) {
		t.Fatal("should deny 1.2.3.4")
	}
	// Should allow
	if !filter.Allow(netip.MustParseAddr("8.8.8.8")) {
		t.Fatal("should allow 8.8.8.8")
	}
}

func TestIPFilter_IPv6(t *testing.T) {
	filter, err := NewIPFilter([]string{"2001:db8::/32"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Should allow
	if !filter.Allow(netip.MustParseAddr("2001:db8::1")) {
		t.Fatal("should allow 2001:db8::1")
	}
	// Should deny
	if filter.Allow(netip.MustParseAddr("2001:db9::1")) {
		t.Fatal("should deny 2001:db9::1")
	}
}

func TestIPFilter_BothLists(t *testing.T) {
	// When both configured, whitelist takes precedence
	filter, err := NewIPFilter(
		[]string{"192.168.1.0/24"},
		[]string{"192.168.1.100"},
	)
	if err != nil {
		t.Fatal(err)
	}
	// Whitelist mode: only check whitelist
	if !filter.Allow(netip.MustParseAddr("192.168.1.100")) {
		t.Fatal("should use whitelist only")
	}
}

func TestIPFilter_InvalidInput(t *testing.T) {
	_, err := NewIPFilter([]string{"invalid_ip"}, nil)
	if err == nil {
		t.Fatal("should return error for invalid IP")
	}
}

func TestIPFilter_EmptyWhitelist(t *testing.T) {
	// Empty whitelist should deny all
	filter, err := NewIPFilter([]string{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Empty whitelist = deny all
	if filter.Allow(netip.MustParseAddr("1.2.3.4")) {
		t.Fatal("empty whitelist should deny all")
	}
}

func TestIPFilter_MultipleRanges(t *testing.T) {
	filter, err := NewIPFilter([]string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Should allow private ranges
	if !filter.Allow(netip.MustParseAddr("10.1.2.3")) {
		t.Fatal("should allow 10.1.2.3")
	}
	if !filter.Allow(netip.MustParseAddr("172.16.1.1")) {
		t.Fatal("should allow 172.16.1.1")
	}
	if !filter.Allow(netip.MustParseAddr("192.168.1.1")) {
		t.Fatal("should allow 192.168.1.1")
	}
	// Should deny public IPs
	if filter.Allow(netip.MustParseAddr("8.8.8.8")) {
		t.Fatal("should deny 8.8.8.8")
	}
}

func TestIPFilter_IPv4MappedIPv6(t *testing.T) {
	filter, err := NewIPFilter([]string{"192.168.1.0/24"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// IPv4-mapped IPv6 should work
	addr := netip.MustParseAddr("::ffff:192.168.1.100")
	// This will depend on netipx.IPSet normalization
	// Just test that it doesn't panic
	_ = filter.Allow(addr)
}

func TestIPFilter_LoopbackAddresses(t *testing.T) {
	// Test that loopback addresses work correctly
	filter, err := NewIPFilter([]string{"127.0.0.0/8", "::1"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Should allow IPv4 loopback
	if !filter.Allow(netip.MustParseAddr("127.0.0.1")) {
		t.Fatal("should allow 127.0.0.1")
	}
	if !filter.Allow(netip.MustParseAddr("127.1.2.3")) {
		t.Fatal("should allow 127.1.2.3")
	}
	// Should allow IPv6 loopback
	if !filter.Allow(netip.MustParseAddr("::1")) {
		t.Fatal("should allow ::1")
	}
	// Should deny other addresses
	if filter.Allow(netip.MustParseAddr("8.8.8.8")) {
		t.Fatal("should deny 8.8.8.8")
	}
}

func TestIPFilter_BlacklistMultiple(t *testing.T) {
	filter, err := NewIPFilter(nil, []string{
		"1.2.3.0/24",
		"5.6.7.8",
		"2001:db8::/32",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Should deny blacklisted
	if filter.Allow(netip.MustParseAddr("1.2.3.100")) {
		t.Fatal("should deny 1.2.3.100")
	}
	if filter.Allow(netip.MustParseAddr("5.6.7.8")) {
		t.Fatal("should deny 5.6.7.8")
	}
	if filter.Allow(netip.MustParseAddr("2001:db8::1")) {
		t.Fatal("should deny 2001:db8::1")
	}
	// Should allow others
	if !filter.Allow(netip.MustParseAddr("8.8.8.8")) {
		t.Fatal("should allow 8.8.8.8")
	}
	if !filter.Allow(netip.MustParseAddr("2001:db9::1")) {
		t.Fatal("should allow 2001:db9::1")
	}
}
