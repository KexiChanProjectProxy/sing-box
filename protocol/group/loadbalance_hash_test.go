package group

import (
	"testing"

	"github.com/sagernet/sing-box/adapter"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/cespare/xxhash/v2"
	"github.com/stretchr/testify/assert"
)

// TestHashKeySrcIpRuleset tests SRC_IP + RULESET hash mode
func TestHashKeySrcIpRuleset(t *testing.T) {
	lb := &LoadBalance{
		hashKeyParts: []string{"src_ip", "matched_ruleset"},
	}

	// Test 1: Same src_ip + same ruleset → same hash
	metadata1 := &adapter.InboundContext{
		Source:         M.ParseSocksaddr("192.168.1.100:12345"),
		MatchedRuleSet: "geosite-google",
	}

	metadata2 := &adapter.InboundContext{
		Source:         M.ParseSocksaddr("192.168.1.100:54321"), // Different port
		MatchedRuleSet: "geosite-google",                        // Same ruleset
	}

	key1 := lb.buildHashKey(metadata1)
	key2 := lb.buildHashKey(metadata2)

	assert.NotEmpty(t, key1)
	assert.Equal(t, key1, key2, "Same src_ip + same ruleset should produce same hash key")

	hash1 := xxhash.Sum64String(key1)
	hash2 := xxhash.Sum64String(key2)
	assert.Equal(t, hash1, hash2, "Hash values should match")

	// Test 2: Same src_ip + different ruleset → different hash
	metadata3 := &adapter.InboundContext{
		Source:         M.ParseSocksaddr("192.168.1.100:12345"),
		MatchedRuleSet: "geosite-netflix", // Different ruleset
	}

	key3 := lb.buildHashKey(metadata3)
	assert.NotEmpty(t, key3)
	assert.NotEqual(t, key1, key3, "Same src_ip + different ruleset should produce different hash key")

	hash3 := xxhash.Sum64String(key3)
	assert.NotEqual(t, hash1, hash3, "Hash values should differ")

	// Test 3: No ruleset matched → uses placeholder
	metadata4 := &adapter.InboundContext{
		Source:         M.ParseSocksaddr("192.168.1.100:12345"),
		MatchedRuleSet: "", // No ruleset matched
	}

	key4 := lb.buildHashKey(metadata4)
	assert.NotEmpty(t, key4)
	assert.Contains(t, key4, "-", "Should use placeholder when no ruleset matched")

	// Verify the key format
	expectedKey1 := "192.168.1.100|geosite-google"
	assert.Equal(t, expectedKey1, key1, "Hash key format should be correct")

	expectedKey4 := "192.168.1.100|-"
	assert.Equal(t, expectedKey4, key4, "Hash key with placeholder should be correct")
}

// TestHashKeySrcIpTopDomain tests SRC_IP + TOP_DOMAIN hash mode
func TestHashKeySrcIpTopDomain(t *testing.T) {
	lb := &LoadBalance{
		hashKeyParts: []string{"src_ip", "etld_plus_one"},
	}

	// Test 1: Same src_ip + same eTLD+1 → same hash
	metadata1 := &adapter.InboundContext{
		Source:      M.ParseSocksaddr("192.168.1.100:12345"),
		Destination: M.ParseSocksaddrHostPort("a.b.example.com", 443),
	}

	metadata2 := &adapter.InboundContext{
		Source:      M.ParseSocksaddr("192.168.1.100:54321"), // Different port
		Destination: M.ParseSocksaddrHostPort("c.d.example.com", 80), // Different subdomain, same eTLD+1
	}

	key1 := lb.buildHashKey(metadata1)
	key2 := lb.buildHashKey(metadata2)

	assert.NotEmpty(t, key1)
	assert.NotEmpty(t, key2)

	// Both should extract example.com as eTLD+1
	assert.Equal(t, key1, key2, "Same src_ip + same eTLD+1 should produce same hash key")

	hash1 := xxhash.Sum64String(key1)
	hash2 := xxhash.Sum64String(key2)
	assert.Equal(t, hash1, hash2, "Hash values should match")

	// Verify the extracted eTLD+1
	expectedKey := "192.168.1.100|example.com"
	assert.Equal(t, expectedKey, key1, "Should extract example.com as eTLD+1")

	// Test 2: Same src_ip + different eTLD+1 → different hash
	metadata3 := &adapter.InboundContext{
		Source:      M.ParseSocksaddr("192.168.1.100:12345"),
		Destination: M.ParseSocksaddrHostPort("sub.google.com", 443), // Different eTLD+1
	}

	key3 := lb.buildHashKey(metadata3)
	assert.NotEmpty(t, key3)
	assert.NotEqual(t, key1, key3, "Same src_ip + different eTLD+1 should produce different hash key")

	expectedKey3 := "192.168.1.100|google.com"
	assert.Equal(t, expectedKey3, key3, "Should extract google.com as eTLD+1")

	hash3 := xxhash.Sum64String(key3)
	assert.NotEqual(t, hash1, hash3, "Hash values should differ")
}

// TestHashKeyTopDomainExtraction tests various domain extraction scenarios
func TestHashKeyTopDomainExtraction(t *testing.T) {
	lb := &LoadBalance{
		hashKeyParts: []string{"etld_plus_one"},
	}

	testCases := []struct {
		name           string
		domain         string
		expectedETLD   string
		description    string
	}{
		{
			name:         "simple domain",
			domain:       "example.com",
			expectedETLD: "example.com",
			description:  "Simple domain should return itself",
		},
		{
			name:         "subdomain",
			domain:       "a.b.example.com",
			expectedETLD: "example.com",
			description:  "Subdomain should extract eTLD+1",
		},
		{
			name:         "co.uk domain",
			domain:       "example.co.uk",
			expectedETLD: "example.co.uk",
			description:  "UK domain should preserve co.uk suffix",
		},
		{
			name:         "co.uk subdomain",
			domain:       "a.b.example.co.uk",
			expectedETLD: "example.co.uk",
			description:  "UK subdomain should extract eTLD+1",
		},
		{
			name:         "domain with port",
			domain:       "example.com:443",
			expectedETLD: "example.com",
			description:  "Port should be stripped",
		},
		{
			name:         "uppercase domain",
			domain:       "EXAMPLE.COM",
			expectedETLD: "example.com",
			description:  "Domain should be normalized to lowercase",
		},
		{
			name:         "domain with trailing dot",
			domain:       "example.com.",
			expectedETLD: "example.com",
			description:  "Trailing dot should be stripped",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metadata := &adapter.InboundContext{
				Domain: tc.domain,
			}

			key := lb.buildHashKey(metadata)
			assert.Equal(t, tc.expectedETLD, key, tc.description)
		})
	}
}

// TestHashKeyIPAddress tests that IP addresses are handled correctly
func TestHashKeyIPAddress(t *testing.T) {
	lb := &LoadBalance{
		hashKeyParts: []string{"etld_plus_one"},
	}

	// IP addresses should return "-" placeholder
	metadata := &adapter.InboundContext{
		Destination: M.ParseSocksaddr("8.8.8.8:53"),
	}

	key := lb.buildHashKey(metadata)
	assert.Equal(t, "-", key, "IP addresses should use placeholder for eTLD+1")
}

// TestHashKeyBackwardCompatibility tests that existing configurations still work
func TestHashKeyBackwardCompatibility(t *testing.T) {
	lb := &LoadBalance{
		hashKeyParts: []string{"src_ip", "dst_ip", "dst_port"},
		hashKeySalt:  "my-salt",
	}

	metadata := &adapter.InboundContext{
		Source:      M.ParseSocksaddr("192.168.1.100:12345"),
		Destination: M.ParseSocksaddr("8.8.8.8:53"),
	}

	key := lb.buildHashKey(metadata)

	// Verify the traditional hash key format still works
	expectedKey := "my-salt192.168.1.100|8.8.8.8|53"
	assert.Equal(t, expectedKey, key, "Traditional hash key format should be preserved")

	// Verify hash is deterministic
	key2 := lb.buildHashKey(metadata)
	assert.Equal(t, key, key2, "Hash key should be deterministic")
}

// TestHashKeyEmptyParts tests handling of empty/missing metadata
func TestHashKeyEmptyParts(t *testing.T) {
	lb := &LoadBalance{
		hashKeyParts: []string{"src_ip", "matched_ruleset", "etld_plus_one"},
	}

	// Metadata with some fields missing
	metadata := &adapter.InboundContext{
		Source:         M.ParseSocksaddr("192.168.1.100:12345"),
		MatchedRuleSet: "", // No ruleset matched
		Destination:    M.ParseSocksaddr("8.8.8.8:53"), // IP, not domain
	}

	key := lb.buildHashKey(metadata)

	// Should use placeholders for missing values
	expectedKey := "192.168.1.100|-|-"
	assert.Equal(t, expectedKey, key, "Should use placeholders for missing values")

	// Verify hash can still be computed
	hash := xxhash.Sum64String(key)
	assert.NotZero(t, hash, "Hash should be computable even with placeholders")
}

// TestHashKeyConsistency tests that same metadata produces same hash
func TestHashKeyConsistency(t *testing.T) {
	lb := &LoadBalance{
		hashKeyParts: []string{"src_ip", "matched_ruleset", "etld_plus_one"},
	}

	metadata := &adapter.InboundContext{
		Source:         M.ParseSocksaddr("192.168.1.100:12345"),
		MatchedRuleSet: "geosite-google",
		Destination:    M.ParseSocksaddrHostPort("www.google.com", 443),
	}

	// Generate hash multiple times
	key1 := lb.buildHashKey(metadata)
	key2 := lb.buildHashKey(metadata)
	key3 := lb.buildHashKey(metadata)

	assert.Equal(t, key1, key2, "Hash key should be consistent across calls")
	assert.Equal(t, key2, key3, "Hash key should be consistent across calls")

	hash1 := xxhash.Sum64String(key1)
	hash2 := xxhash.Sum64String(key2)
	hash3 := xxhash.Sum64String(key3)

	assert.Equal(t, hash1, hash2, "Hash values should be consistent")
	assert.Equal(t, hash2, hash3, "Hash values should be consistent")
}

// TestHashKeyDomainFromSniff tests using domain from sniffer
func TestHashKeyDomainFromSniff(t *testing.T) {
	lb := &LoadBalance{
		hashKeyParts: []string{"etld_plus_one"},
	}

	// Test with domain from sniffer (Domain field) instead of Destination
	metadata := &adapter.InboundContext{
		Destination: M.ParseSocksaddr("1.2.3.4:443"), // IP address
		Domain:      "mail.google.com",                // Domain from SNI sniffing
	}

	key := lb.buildHashKey(metadata)

	// Should extract eTLD+1 from sniffed domain
	expectedKey := "google.com"
	assert.Equal(t, expectedKey, key, "Should use sniffed domain for eTLD+1 extraction")
}

// TestHashKeyCombinations tests various combinations of key parts
func TestHashKeyCombinations(t *testing.T) {
	testCases := []struct {
		name         string
		keyParts     []string
		metadata     *adapter.InboundContext
		expectedKey  string
	}{
		{
			name:     "src_ip + matched_ruleset",
			keyParts: []string{"src_ip", "matched_ruleset"},
			metadata: &adapter.InboundContext{
				Source:         M.ParseSocksaddr("10.0.0.1:1234"),
				MatchedRuleSet: "geosite-category-ads",
			},
			expectedKey: "10.0.0.1|geosite-category-ads",
		},
		{
			name:     "src_ip + etld_plus_one",
			keyParts: []string{"src_ip", "etld_plus_one"},
			metadata: &adapter.InboundContext{
				Source:      M.ParseSocksaddr("10.0.0.1:1234"),
				Destination: M.ParseSocksaddrHostPort("cdn.example.com", 443),
			},
			expectedKey: "10.0.0.1|example.com",
		},
		{
			name:     "matched_ruleset + etld_plus_one",
			keyParts: []string{"matched_ruleset", "etld_plus_one"},
			metadata: &adapter.InboundContext{
				MatchedRuleSet: "geosite-google",
				Destination:    M.ParseSocksaddrHostPort("www.google.co.uk", 443),
			},
			expectedKey: "geosite-google|google.co.uk",
		},
		{
			name:     "all three parts",
			keyParts: []string{"src_ip", "matched_ruleset", "etld_plus_one"},
			metadata: &adapter.InboundContext{
				Source:         M.ParseSocksaddr("172.16.0.1:5678"),
				MatchedRuleSet: "geosite-netflix",
				Destination:    M.ParseSocksaddrHostPort("api.netflix.com", 443),
			},
			expectedKey: "172.16.0.1|geosite-netflix|netflix.com",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lb := &LoadBalance{
				hashKeyParts: tc.keyParts,
			}

			key := lb.buildHashKey(tc.metadata)
			assert.Equal(t, tc.expectedKey, key)

			// Verify hash is computable
			hash := xxhash.Sum64String(key)
			assert.NotZero(t, hash)
		})
	}
}
