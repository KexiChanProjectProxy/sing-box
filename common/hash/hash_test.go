package hash

import (
	"fmt"
	"sync"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	M "github.com/sagernet/sing/common/metadata"
)

func TestHashRing_BasicOperations(t *testing.T) {
	ring := NewHashRing(10)

	// Test empty ring
	if ring.Size() != 0 {
		t.Errorf("New ring should be empty, got size %d", ring.Size())
	}

	_, ok := ring.Get("test-key")
	if ok {
		t.Error("Get on empty ring should return false")
	}

	// Add nodes
	ring.Add("node1")
	ring.Add("node2")
	ring.Add("node3")

	if ring.Size() != 3 {
		t.Errorf("Ring size should be 3, got %d", ring.Size())
	}

	// Test that we can get a node
	node, ok := ring.Get("test-key")
	if !ok {
		t.Error("Get should return true with nodes in ring")
	}
	if node == "" {
		t.Error("Get should return a non-empty node name")
	}

	// Remove a node
	ring.Remove("node2")
	if ring.Size() != 2 {
		t.Errorf("Ring size should be 2 after removal, got %d", ring.Size())
	}

	// Test duplicate add (should be ignored)
	ring.Add("node1")
	if ring.Size() != 2 {
		t.Errorf("Duplicate add should be ignored, got size %d", ring.Size())
	}
}

func TestHashRing_ConsistentHashing(t *testing.T) {
	ring := NewHashRing(10)
	ring.Add("node1")
	ring.Add("node2")
	ring.Add("node3")

	// Test consistency: same key should always map to same node
	key := "test-key-123"
	node1, _ := ring.Get(key)

	for i := 0; i < 100; i++ {
		node, ok := ring.Get(key)
		if !ok {
			t.Fatal("Get should return true")
		}
		if node != node1 {
			t.Errorf("Same key should map to same node, got %s and %s", node1, node)
		}
	}
}

func TestHashRing_Distribution(t *testing.T) {
	ring := NewHashRing(100) // Use more virtual nodes for better distribution
	nodes := []string{"node1", "node2", "node3", "node4", "node5"}

	for _, node := range nodes {
		ring.Add(node)
	}

	// Test distribution across many keys
	distribution := make(map[string]int)
	numKeys := 10000

	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key-%d", i)
		node, ok := ring.Get(key)
		if !ok {
			t.Fatal("Get should return true")
		}
		distribution[node]++
	}

	// Each node should get roughly 20% of keys (±10%)
	expectedPerNode := numKeys / len(nodes)
	tolerance := expectedPerNode / 2 // 50% tolerance

	for _, node := range nodes {
		count := distribution[node]
		if count < expectedPerNode-tolerance || count > expectedPerNode+tolerance {
			t.Errorf("Node %s got %d keys, expected around %d (±%d)",
				node, count, expectedPerNode, tolerance)
		}
		t.Logf("Node %s: %d keys (%.1f%%)", node, count, float64(count)*100/float64(numKeys))
	}
}

func TestHashRing_MinimalRemapping(t *testing.T) {
	ring := NewHashRing(50)
	nodes := []string{"node1", "node2", "node3", "node4"}

	for _, node := range nodes {
		ring.Add(node)
	}

	// Record initial mappings
	numKeys := 1000
	initialMapping := make(map[string]string)

	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key-%d", i)
		node, _ := ring.Get(key)
		initialMapping[key] = node
	}

	// Remove one node (25% of nodes)
	ring.Remove("node3")

	// Check how many keys got remapped
	remapped := 0
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key-%d", i)
		newNode, _ := ring.Get(key)
		if initialMapping[key] != newNode {
			remapped++
		}
	}

	// With 25% of nodes removed, we expect around 25% remapping (±10%)
	expectedRemapped := numKeys / 4
	tolerance := numKeys / 10

	if remapped < expectedRemapped-tolerance || remapped > expectedRemapped+tolerance {
		t.Errorf("Remapped %d keys, expected around %d (±%d) when removing 25%% of nodes",
			remapped, expectedRemapped, tolerance)
	}

	t.Logf("Remapped %d/%d keys (%.1f%%) when removing 1/4 nodes",
		remapped, numKeys, float64(remapped)*100/float64(numKeys))
}

func TestHashRing_ConcurrentAccess(t *testing.T) {
	ring := NewHashRing(10)
	ring.Add("node1")
	ring.Add("node2")
	ring.Add("node3")

	// Test concurrent reads
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", id)
			for j := 0; j < 100; j++ {
				_, ok := ring.Get(key)
				if !ok {
					t.Error("Concurrent Get failed")
				}
			}
		}(i)
	}
	wg.Wait()

	// Test concurrent writes (adds/removes)
	ring2 := NewHashRing(10)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			nodeName := fmt.Sprintf("node-%d", id)
			ring2.Add(nodeName)
		}(i)
	}
	wg.Wait()

	if ring2.Size() != 50 {
		t.Errorf("Expected 50 nodes after concurrent adds, got %d", ring2.Size())
	}
}

func TestBuildHashKey_BasicParts(t *testing.T) {
	metadata := &adapter.InboundContext{
		Source:      M.ParseSocksaddr("192.168.1.100:12345"),
		Destination: M.ParseSocksaddr("1.2.3.4:443"),
		Network:     "tcp",
		Domain:      "example.com",
		Inbound:     "mixed-in",
	}

	tests := []struct {
		name     string
		keyParts []string
		expected string
	}{
		{
			name:     "src_ip only",
			keyParts: []string{"src_ip"},
			expected: "192.168.1.100",
		},
		{
			name:     "dst_ip only",
			keyParts: []string{"dst_ip"},
			expected: "1.2.3.4",
		},
		{
			name:     "dst_port only",
			keyParts: []string{"dst_port"},
			expected: "443",
		},
		{
			name:     "network only",
			keyParts: []string{"network"},
			expected: "tcp",
		},
		{
			name:     "domain only",
			keyParts: []string{"domain"},
			expected: "example.com",
		},
		{
			name:     "inbound_tag only",
			keyParts: []string{"inbound_tag"},
			expected: "mixed-in",
		},
		{
			name:     "composite: src_ip + dst_port",
			keyParts: []string{"src_ip", "dst_port"},
			expected: "192.168.1.100|443",
		},
		{
			name:     "composite: src_ip + dst_ip + network",
			keyParts: []string{"src_ip", "dst_ip", "network"},
			expected: "192.168.1.100|1.2.3.4|tcp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildHashKey(metadata, tt.keyParts, "")
			if result != tt.expected {
				t.Errorf("BuildHashKey() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestBuildHashKey_ETLDPlusOne(t *testing.T) {
	metadata := &adapter.InboundContext{
		Domain: "www.example.com",
	}

	key := BuildHashKey(metadata, []string{"etld_plus_one"}, "")
	expected := "example.com"

	if key != expected {
		t.Errorf("BuildHashKey with etld_plus_one = %q, want %q", key, expected)
	}

	// Test with multi-part TLD
	metadata.Domain = "www.example.co.uk"
	key = BuildHashKey(metadata, []string{"etld_plus_one"}, "")
	expected = "example.co.uk"

	if key != expected {
		t.Errorf("BuildHashKey with etld_plus_one (co.uk) = %q, want %q", key, expected)
	}
}

func TestBuildHashKey_MatchedRuleset(t *testing.T) {
	metadata := &adapter.InboundContext{
		Domain:         "example.com",
		MatchedRuleSet: "streaming",
	}

	key := BuildHashKey(metadata, []string{"matched_ruleset"}, "")
	expected := "streaming"

	if key != expected {
		t.Errorf("BuildHashKey with matched_ruleset = %q, want %q", key, expected)
	}
}

func TestBuildHashKey_SmartFallback(t *testing.T) {
	// Test with matched ruleset
	metadata := &adapter.InboundContext{
		Domain:         "www.netflix.com",
		MatchedRuleSet: "streaming",
	}

	key := BuildHashKey(metadata, []string{"matched_ruleset_or_etld"}, "")
	expected := "streaming"

	if key != expected {
		t.Errorf("BuildHashKey with ruleset should use ruleset, got %q, want %q", key, expected)
	}

	// Test without matched ruleset (fallback to eTLD+1)
	metadata.MatchedRuleSet = ""
	key = BuildHashKey(metadata, []string{"matched_ruleset_or_etld"}, "")
	expected = "netflix.com"

	if key != expected {
		t.Errorf("BuildHashKey without ruleset should use eTLD+1, got %q, want %q", key, expected)
	}
}

func TestBuildHashKey_WithSalt(t *testing.T) {
	metadata := &adapter.InboundContext{
		Source: M.ParseSocksaddr("192.168.1.100:12345"),
	}

	key1 := BuildHashKey(metadata, []string{"src_ip", "salt"}, "my-secret-salt")
	key2 := BuildHashKey(metadata, []string{"src_ip", "salt"}, "different-salt")

	if key1 == key2 {
		t.Error("Different salts should produce different keys")
	}

	expected := "192.168.1.100|my-secret-salt"
	if key1 != expected {
		t.Errorf("BuildHashKey with salt = %q, want %q", key1, expected)
	}
}

func TestBuildHashKey_EmptyValues(t *testing.T) {
	metadata := &adapter.InboundContext{
		// All fields empty
	}

	tests := []struct {
		name     string
		keyParts []string
		expected string
	}{
		{
			name:     "empty src_ip",
			keyParts: []string{"src_ip"},
			expected: "-",
		},
		{
			name:     "empty domain",
			keyParts: []string{"domain"},
			expected: "-",
		},
		{
			name:     "empty matched_ruleset",
			keyParts: []string{"matched_ruleset"},
			expected: "-",
		},
		{
			name:     "empty composite",
			keyParts: []string{"src_ip", "domain", "network"},
			expected: "-|-|-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildHashKey(metadata, tt.keyParts, "")
			if result != tt.expected {
				t.Errorf("BuildHashKey() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestBuildHashKey_Consistency(t *testing.T) {
	metadata := &adapter.InboundContext{
		Source:      M.ParseSocksaddr("192.168.1.100:12345"),
		Destination: M.ParseSocksaddr("1.2.3.4:443"),
		Network:     "tcp",
		Domain:      "www.example.com",
	}

	keyParts := []string{"src_ip", "dst_ip", "domain"}

	// Build key multiple times
	key1 := BuildHashKey(metadata, keyParts, "")
	key2 := BuildHashKey(metadata, keyParts, "")
	key3 := BuildHashKey(metadata, keyParts, "")

	if key1 != key2 || key2 != key3 {
		t.Errorf("BuildHashKey should be consistent, got %q, %q, %q", key1, key2, key3)
	}
}

func TestBuildHashKey_EmptyParts(t *testing.T) {
	metadata := &adapter.InboundContext{
		Domain: "example.com",
	}

	// Empty key parts should return empty string
	key := BuildHashKey(metadata, []string{}, "")
	if key != "" {
		t.Errorf("BuildHashKey with empty parts should return empty string, got %q", key)
	}
}

func TestHashKey_Function(t *testing.T) {
	// Test that hash function is deterministic
	key := "test-key"
	hash1 := hashKey(key)
	hash2 := hashKey(key)

	if hash1 != hash2 {
		t.Error("hashKey should be deterministic")
	}

	// Test that different keys produce different hashes (usually)
	hash3 := hashKey("different-key")
	if hash1 == hash3 {
		t.Error("Different keys should produce different hashes (collision)")
	}
}

func TestBuildHashKey_AllParts(t *testing.T) {
	// Test all 10 key parts together
	metadata := &adapter.InboundContext{
		Source:         M.ParseSocksaddr("192.168.1.100:12345"),
		Destination:    M.ParseSocksaddr("1.2.3.4:443"),
		Network:        "tcp",
		Domain:         "www.example.co.uk",
		Inbound:        "mixed-in",
		MatchedRuleSet: "streaming",
	}

	keyParts := []string{
		"src_ip",
		"dst_ip",
		"dst_port",
		"network",
		"domain",
		"inbound_tag",
		"matched_ruleset",
		"etld_plus_one",
		"matched_ruleset_or_etld",
		"salt",
	}

	key := BuildHashKey(metadata, keyParts, "my-salt")

	// Should contain all parts
	expected := "192.168.1.100|1.2.3.4|443|tcp|www.example.co.uk|mixed-in|streaming|example.co.uk|streaming|my-salt"

	if key != expected {
		t.Errorf("BuildHashKey with all parts:\ngot  %q\nwant %q", key, expected)
	}
}
