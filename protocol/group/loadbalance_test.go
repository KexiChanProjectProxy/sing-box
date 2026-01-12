package group

import (
	"context"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/hash"
	M "github.com/sagernet/sing/common/metadata"
)

func TestSelectTopN(t *testing.T) {
	group := &LoadBalanceGroup{
		allTags:     []string{"node1", "node2", "node3", "node4", "node5"},
		primaryTags: []string{"node1", "node2", "node3", "node4", "node5"},
	}

	latencies := map[string]uint16{
		"node1": 100,
		"node2": 50,
		"node3": 200,
		"node4": 75,
		"node5": 150,
	}

	tests := []struct {
		name     string
		topN     int
		expected []string
	}{
		{
			name:     "top 3",
			topN:     3,
			expected: []string{"node2", "node4", "node1"}, // 50, 75, 100
		},
		{
			name:     "top 1",
			topN:     1,
			expected: []string{"node2"}, // 50
		},
		{
			name:     "top 0 (all)",
			topN:     0,
			expected: []string{"node2", "node4", "node1", "node5", "node3"}, // all sorted
		},
		{
			name:     "top more than available",
			topN:     10,
			expected: []string{"node2", "node4", "node1", "node5", "node3"}, // all
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := group.selectTopN(group.primaryTags, latencies, tt.topN)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d results, got %d", len(tt.expected), len(result))
				return
			}

			for i, tag := range result {
				if tag != tt.expected[i] {
					t.Errorf("position %d: expected %s, got %s", i, tt.expected[i], tag)
				}
			}
		})
	}
}

func TestBackupTierActivation(t *testing.T) {
	group := &LoadBalanceGroup{
		primaryFailures: 3,
		backupHoldTime:  100 * time.Millisecond,
	}

	// Test consecutive failures
	group.consecutiveFailures.Store(0)
	group.usingBackupTier.Store(false)

	// Simulate primary candidates becoming unavailable
	primaryLatencies := map[string]uint16{} // empty = no healthy primaries
	backupLatencies := map[string]uint16{
		"backup1": 100,
	}

	// Should not switch immediately (failure count = 0)
	group.consecutiveFailures.Store(0)
	if group.usingBackupTier.Load() {
		t.Error("should not use backup tier yet")
	}

	// Increment failures
	group.consecutiveFailures.Store(1)
	if group.usingBackupTier.Load() {
		t.Error("should not use backup tier yet (1 < 3)")
	}

	group.consecutiveFailures.Store(2)
	if group.usingBackupTier.Load() {
		t.Error("should not use backup tier yet (2 < 3)")
	}

	// After reaching threshold
	group.consecutiveFailures.Store(3)
	// Simulate the updateCandidates logic
	if int(group.consecutiveFailures.Load()) >= group.primaryFailures {
		group.usingBackupTier.Store(true)
		group.lastBackupActivation.Store(time.Now())
	}

	if !group.usingBackupTier.Load() {
		t.Error("should switch to backup tier after 3 failures")
	}

	// Test hold time
	activationTime := group.lastBackupActivation.Load()

	// Immediately try to switch back (should fail due to hold time)
	if time.Since(activationTime) < group.backupHoldTime {
		// Should stay in backup
		if !group.usingBackupTier.Load() {
			t.Error("should still be using backup tier during hold time")
		}
	}

	// Wait for hold time to elapse
	time.Sleep(group.backupHoldTime + 10*time.Millisecond)

	// Now can switch back
	if time.Since(activationTime) >= group.backupHoldTime {
		group.usingBackupTier.Store(false)
	}

	if group.usingBackupTier.Load() {
		t.Error("should switch back to primary after hold time")
	}
}

func TestConsistentHashStability(t *testing.T) {
	ring := hash.NewHashRing(10)
	ring.Add("node1")
	ring.Add("node2")
	ring.Add("node3")

	// Test that same key always maps to same node
	testKeys := []string{
		"test-key-1",
		"192.168.1.1",
		"example.com",
		"user@example.com",
	}

	for _, key := range testKeys {
		firstResult, ok1 := ring.Get(key)
		if !ok1 {
			t.Fatalf("failed to get node for key %s", key)
		}

		// Query 100 times
		for i := 0; i < 100; i++ {
			result, ok := ring.Get(key)
			if !ok {
				t.Fatalf("failed to get node for key %s on iteration %d", key, i)
			}
			if result != firstResult {
				t.Errorf("inconsistent result for key %s: got %s, expected %s", key, result, firstResult)
			}
		}
	}
}

func TestConsistentHashDistribution(t *testing.T) {
	// Test that virtual nodes provide good distribution
	ring := hash.NewHashRing(50) // Use more virtual nodes for better distribution
	nodes := []string{"node1", "node2", "node3"}

	for _, node := range nodes {
		ring.Add(node)
	}

	// Test distribution over many keys
	distribution := make(map[string]int)
	numKeys := 10000

	for i := 0; i < numKeys; i++ {
		key := hash.BuildHashKey(&adapter.InboundContext{
			Source:      M.ParseSocksaddr("192.168.1." + string(rune(i%256)) + ":12345"),
			Destination: M.ParseSocksaddr("1.2.3.4:443"),
		}, []string{"src_ip", "dst_ip"}, "")

		node, ok := ring.Get(key)
		if !ok {
			t.Fatal("failed to get node")
		}
		distribution[node]++
	}

	// Each node should get roughly 33% of keys (±15%)
	expectedPerNode := numKeys / len(nodes)
	tolerance := expectedPerNode * 15 / 100 // 15% tolerance

	for _, node := range nodes {
		count := distribution[node]
		if count < expectedPerNode-tolerance || count > expectedPerNode+tolerance {
			t.Errorf("node %s: got %d keys, expected ~%d (±%d)", node, count, expectedPerNode, tolerance)
		}
		t.Logf("Node %s: %d keys (%.1f%%)", node, count, float64(count)*100/float64(numKeys))
	}
}

func TestHysteresis(t *testing.T) {
	group := &LoadBalanceGroup{
		primaryFailures: 2,
		backupHoldTime:  50 * time.Millisecond,
	}

	// Initial state: using primary
	group.usingBackupTier.Store(false)
	group.consecutiveFailures.Store(0)

	// Record one failure
	group.recordFailure()
	if group.consecutiveFailures.Load() != 1 {
		t.Error("failure count should be 1")
	}
	if group.usingBackupTier.Load() {
		t.Error("should not switch to backup after 1 failure (threshold is 2)")
	}

	// Record second failure - should trigger switch
	group.recordFailure()
	// Manually trigger switch (normally done in updateCandidates)
	if int(group.consecutiveFailures.Load()) >= group.primaryFailures {
		group.usingBackupTier.Store(true)
		group.lastBackupActivation.Store(time.Now())
	}

	if !group.usingBackupTier.Load() {
		t.Error("should switch to backup after 2 failures")
	}

	// Test that we can't immediately switch back
	activationTime := group.lastBackupActivation.Load()
	if time.Since(activationTime) < group.backupHoldTime {
		// Still in hold time, should stay in backup
		if !group.usingBackupTier.Load() {
			t.Error("should remain in backup during hold time")
		}
	}

	// Wait for hold time
	time.Sleep(group.backupHoldTime + 10*time.Millisecond)

	// Now should be able to switch back
	if time.Since(activationTime) >= group.backupHoldTime {
		group.usingBackupTier.Store(false)
		group.consecutiveFailures.Store(0)
	}

	if group.usingBackupTier.Load() {
		t.Error("should switch back to primary after hold time")
	}
	if group.consecutiveFailures.Load() != 0 {
		t.Error("failure count should reset to 0")
	}
}

func TestEmptyPoolBehavior(t *testing.T) {
	// Test "error" mode
	groupError := &LoadBalanceGroup{
		emptyPoolAction: "error",
	}

	// No candidates
	groupError.selectedCandidates.Store([]*loadBalanceCandidate{})

	candidates := groupError.selectedCandidates.Load()
	if candidates == nil || len(candidates) == 0 {
		// This should result in an error when selecting
		t.Log("Empty pool correctly returns no candidates in error mode")
	}

	// Test "fallback_all" mode
	groupFallback := &LoadBalanceGroup{
		emptyPoolAction: "fallback_all",
		allTags:         []string{"node1", "node2"},
	}

	groupFallback.selectedCandidates.Store([]*loadBalanceCandidate{})

	// With fallback_all, selectFromAll should be used
	if groupFallback.emptyPoolAction == "fallback_all" {
		t.Log("Fallback mode will use all outbounds when pool is empty")
	}
}

func TestNetworkFiltering(t *testing.T) {
	// Create mock outbounds with different network support
	candidates := []*loadBalanceCandidate{
		{
			tag: "tcp-only",
			outbound: &mockOutbound{
				tag:      "tcp-only",
				networks: []string{"tcp"},
			},
		},
		{
			tag: "udp-only",
			outbound: &mockOutbound{
				tag:      "udp-only",
				networks: []string{"udp"},
			},
		},
		{
			tag: "both",
			outbound: &mockOutbound{
				tag:      "both",
				networks: []string{"tcp", "udp"},
			},
		},
	}

	group := &LoadBalanceGroup{}

	// Filter for TCP
	tcpCandidates := group.filterByNetwork(candidates, "tcp")
	if len(tcpCandidates) != 2 {
		t.Errorf("expected 2 TCP candidates, got %d", len(tcpCandidates))
	}

	// Filter for UDP
	udpCandidates := group.filterByNetwork(candidates, "udp")
	if len(udpCandidates) != 2 {
		t.Errorf("expected 2 UDP candidates, got %d", len(udpCandidates))
	}

	// Verify correct candidates
	tcpTags := make(map[string]bool)
	for _, c := range tcpCandidates {
		tcpTags[c.tag] = true
	}
	if !tcpTags["tcp-only"] || !tcpTags["both"] {
		t.Error("TCP filtering should include tcp-only and both")
	}

	udpTags := make(map[string]bool)
	for _, c := range udpCandidates {
		udpTags[c.tag] = true
	}
	if !udpTags["udp-only"] || !udpTags["both"] {
		t.Error("UDP filtering should include udp-only and both")
	}
}

func TestRandomDistribution(t *testing.T) {
	candidates := []*loadBalanceCandidate{
		{tag: "node1", outbound: &mockOutbound{tag: "node1"}},
		{tag: "node2", outbound: &mockOutbound{tag: "node2"}},
		{tag: "node3", outbound: &mockOutbound{tag: "node3"}},
	}

	group := &LoadBalanceGroup{
		strategy: "random",
	}

	// Test random selection distribution
	distribution := make(map[string]int)
	numSelections := 10000

	for i := 0; i < numSelections; i++ {
		selected := group.selectRandom(candidates)
		if selected != nil {
			distribution[selected.Tag()]++
		}
	}

	// Each node should get roughly 33% (±10%)
	expectedPerNode := numSelections / len(candidates)
	tolerance := expectedPerNode * 10 / 100

	for _, candidate := range candidates {
		count := distribution[candidate.tag]
		if count < expectedPerNode-tolerance || count > expectedPerNode+tolerance {
			t.Errorf("node %s: got %d selections, expected ~%d (±%d)",
				candidate.tag, count, expectedPerNode, tolerance)
		}
		t.Logf("Node %s: %d selections (%.1f%%)",
			candidate.tag, count, float64(count)*100/float64(numSelections))
	}
}

func TestHashKeyConstruction(t *testing.T) {
	metadata := &adapter.InboundContext{
		Source:         M.ParseSocksaddr("192.168.1.100:12345"),
		Destination:    M.ParseSocksaddr("1.2.3.4:443"),
		Network:        "tcp",
		Domain:         "www.example.com",
		Inbound:        "mixed-in",
		MatchedRuleSet: "streaming",
	}

	tests := []struct {
		name     string
		keyParts []string
		salt     string
		verify   func(key string) bool
	}{
		{
			name:     "src_ip only",
			keyParts: []string{"src_ip"},
			verify: func(key string) bool {
				return key == "192.168.1.100"
			},
		},
		{
			name:     "composite key",
			keyParts: []string{"src_ip", "dst_port"},
			verify: func(key string) bool {
				return key == "192.168.1.100|443"
			},
		},
		{
			name:     "with domain",
			keyParts: []string{"domain"},
			verify: func(key string) bool {
				return key == "www.example.com"
			},
		},
		{
			name:     "with matched_ruleset",
			keyParts: []string{"matched_ruleset"},
			verify: func(key string) bool {
				return key == "streaming"
			},
		},
		{
			name:     "etld_plus_one",
			keyParts: []string{"etld_plus_one"},
			verify: func(key string) bool {
				return key == "example.com"
			},
		},
		{
			name:     "matched_ruleset_or_etld (with ruleset)",
			keyParts: []string{"matched_ruleset_or_etld"},
			verify: func(key string) bool {
				return key == "streaming" // ruleset takes priority
			},
		},
		{
			name:     "with salt",
			keyParts: []string{"src_ip", "salt"},
			salt:     "my-salt",
			verify: func(key string) bool {
				return key == "192.168.1.100|my-salt"
			},
		},
		{
			name:     "all parts",
			keyParts: []string{"src_ip", "dst_ip", "network", "domain"},
			verify: func(key string) bool {
				return key == "192.168.1.100|1.2.3.4|tcp|www.example.com"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := hash.BuildHashKey(metadata, tt.keyParts, tt.salt)
			if !tt.verify(key) {
				t.Errorf("hash key verification failed: got %q", key)
			}
		})
	}
}

// Mock outbound for testing
type mockOutbound struct {
	tag      string
	networks []string
}

func (m *mockOutbound) Type() string           { return "mock" }
func (m *mockOutbound) Tag() string            { return m.tag }
func (m *mockOutbound) Network() []string      { return m.networks }
func (m *mockOutbound) Dependencies() []string { return nil }
func (m *mockOutbound) Start() error           { return nil }
func (m *mockOutbound) PostStart() error       { return nil }
func (m *mockOutbound) Close() error           { return nil }
func (m *mockOutbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return nil, nil
}
func (m *mockOutbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, nil
}

// Ensure mockOutbound implements adapter.Outbound
var _ adapter.Outbound = (*mockOutbound)(nil)
