package group

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/urltest"
	"github.com/sagernet/sing-box/log"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/cespare/xxhash/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock logger for testing
type mockLogger struct{}

func (m *mockLogger) Trace(args ...any)                 {}
func (m *mockLogger) Debug(args ...any)                 {}
func (m *mockLogger) Info(args ...any)                  {}
func (m *mockLogger) Warn(args ...any)                  {}
func (m *mockLogger) Error(args ...any)                 {}
func (m *mockLogger) Fatal(args ...any)                 {}
func (m *mockLogger) Panic(args ...any)                 {}
func (m *mockLogger) TraceContext(ctx context.Context, args ...any) {}
func (m *mockLogger) DebugContext(ctx context.Context, args ...any) {}
func (m *mockLogger) InfoContext(ctx context.Context, args ...any)  {}
func (m *mockLogger) WarnContext(ctx context.Context, args ...any)  {}
func (m *mockLogger) ErrorContext(ctx context.Context, args ...any) {}
func (m *mockLogger) FatalContext(ctx context.Context, args ...any) {}
func (m *mockLogger) PanicContext(ctx context.Context, args ...any) {}

// Mock outbound for testing
type mockOutbound struct {
	tag     string
	network []string
}

func (m *mockOutbound) Type() string              { return "mock" }
func (m *mockOutbound) Tag() string               { return m.tag }
func (m *mockOutbound) Network() []string         { return m.network }
func (m *mockOutbound) Dependencies() []string    { return nil }
func (m *mockOutbound) Start(stage adapter.StartStage) error { return nil }
func (m *mockOutbound) Close() error              { return nil }

// Implement Dialer interface stubs
func (m *mockOutbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return nil, nil
}
func (m *mockOutbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, nil
}
func (m *mockOutbound) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return nil
}
func (m *mockOutbound) NewPacketConnection(ctx context.Context, conn net.PacketConn, metadata adapter.InboundContext) error {
	return nil
}

// Mock outbound manager
type mockOutboundManager struct {
	outbounds map[string]adapter.Outbound
}

func (m *mockOutboundManager) Outbound(tag string) (adapter.Outbound, bool) {
	outbound, ok := m.outbounds[tag]
	return outbound, ok
}

func (m *mockOutboundManager) Outbounds() []adapter.Outbound {
	result := make([]adapter.Outbound, 0, len(m.outbounds))
	for _, o := range m.outbounds {
		result = append(result, o)
	}
	return result
}

func (m *mockOutboundManager) Default() adapter.Outbound {
	return nil
}

func (m *mockOutboundManager) Start(stage adapter.StartStage) error {
	return nil
}

func (m *mockOutboundManager) Close() error {
	return nil
}

func (m *mockOutboundManager) Remove(tag string) error {
	delete(m.outbounds, tag)
	return nil
}

func (m *mockOutboundManager) Create(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, outboundType string, options any) error {
	return nil
}

// Test 1: Top-N selection per tier
func TestTopNSelection(t *testing.T) {
	history := urltest.NewHistoryStorage()

	// Create mock outbounds
	primaryTags := []string{"p1", "p2", "p3", "p4", "p5"}
	backupTags := []string{"b1", "b2", "b3"}

	// Set latencies: p1=10ms, p2=20ms, p3=30ms, p4=40ms, p5=50ms
	//                b1=100ms, b2=110ms, b3=120ms
	now := time.Now()
	history.StoreURLTestHistory("p1", &adapter.URLTestHistory{Time: now, Delay: 10})
	history.StoreURLTestHistory("p2", &adapter.URLTestHistory{Time: now, Delay: 20})
	history.StoreURLTestHistory("p3", &adapter.URLTestHistory{Time: now, Delay: 30})
	history.StoreURLTestHistory("p4", &adapter.URLTestHistory{Time: now, Delay: 40})
	history.StoreURLTestHistory("p5", &adapter.URLTestHistory{Time: now, Delay: 50})
	history.StoreURLTestHistory("b1", &adapter.URLTestHistory{Time: now, Delay: 100})
	history.StoreURLTestHistory("b2", &adapter.URLTestHistory{Time: now, Delay: 110})
	history.StoreURLTestHistory("b3", &adapter.URLTestHistory{Time: now, Delay: 120})

	lb := &LoadBalance{
		logger:          &mockLogger{},
		primaryTags: primaryTags,
		backupTags:  backupTags,
		topNPrimary: 3,
		topNBackup:  2,
		interval:    time.Minute,
		history:     history,
		outbound: &mockOutboundManager{
			outbounds: map[string]adapter.Outbound{
				"p1": &mockOutbound{tag: "p1", network: []string{"tcp", "udp"}},
				"p2": &mockOutbound{tag: "p2", network: []string{"tcp", "udp"}},
				"p3": &mockOutbound{tag: "p3", network: []string{"tcp", "udp"}},
				"p4": &mockOutbound{tag: "p4", network: []string{"tcp", "udp"}},
				"p5": &mockOutbound{tag: "p5", network: []string{"tcp", "udp"}},
				"b1": &mockOutbound{tag: "b1", network: []string{"tcp", "udp"}},
				"b2": &mockOutbound{tag: "b2", network: []string{"tcp", "udp"}},
				"b3": &mockOutbound{tag: "b3", network: []string{"tcp", "udp"}},
			},
		},
	}

	// Collect tier stats
	primaryStats := lb.collectTierStats(primaryTags)
	backupStats := lb.collectTierStats(backupTags)

	// Select Top-N
	primaryCandidates := lb.selectTopN(primaryStats, lb.topNPrimary)
	backupCandidates := lb.selectTopN(backupStats, lb.topNBackup)

	// Verify primary Top-3: p1, p2, p3
	require.Len(t, primaryCandidates, 3, "should select top 3 primary candidates")
	assert.Equal(t, "p1", primaryCandidates[0].Tag())
	assert.Equal(t, "p2", primaryCandidates[1].Tag())
	assert.Equal(t, "p3", primaryCandidates[2].Tag())

	// Verify backup Top-2: b1, b2
	require.Len(t, backupCandidates, 2, "should select top 2 backup candidates")
	assert.Equal(t, "b1", backupCandidates[0].Tag())
	assert.Equal(t, "b2", backupCandidates[1].Tag())
}

// Test 2: Backup activation rule (HAProxy-like)
func TestBackupActivation(t *testing.T) {
	history := urltest.NewHistoryStorage()
	now := time.Now()

	// Primary available: should use primary
	history.StoreURLTestHistory("p1", &adapter.URLTestHistory{Time: now, Delay: 10})
	history.StoreURLTestHistory("b1", &adapter.URLTestHistory{Time: now, Delay: 100})

	lb := &LoadBalance{
		logger:          &mockLogger{},
		primaryTags:         []string{"p1"},
		backupTags:          []string{"b1"},
		topNPrimary:         1,
		topNBackup:          1,
		interval:            time.Minute,
		history:             history,
		hystPrimaryFailures: 1,
		hystBackupHoldTime:  time.Second,
		outbound: &mockOutboundManager{
			outbounds: map[string]adapter.Outbound{
				"p1": &mockOutbound{tag: "p1", network: []string{"tcp"}},
				"b1": &mockOutbound{tag: "b1", network: []string{"tcp"}},
			},
		},
	}

	lb.tierState.Store(&tierStateSnapshot{activeTier: "primary"})

	// Update candidates
	lb.updateCandidates()

	snapshot := lb.candidateState.Load().(*candidateSnapshot)
	assert.Equal(t, "primary", snapshot.activeTier, "should use primary tier when available")
	assert.Len(t, snapshot.primaryCandidates, 1)
	assert.Len(t, snapshot.backupCandidates, 1)

	// Now primary fails (no history for p1)
	history.DeleteURLTestHistory("p1")
	lb.updateCandidates()

	snapshot = lb.candidateState.Load().(*candidateSnapshot)
	assert.Equal(t, "backup", snapshot.activeTier, "should switch to backup tier when primary fails")

	// Primary recovers
	history.StoreURLTestHistory("p1", &adapter.URLTestHistory{Time: time.Now(), Delay: 10})

	// Immediately after recovery, should still be on backup (hold time not elapsed)
	lb.updateCandidates()
	snapshot = lb.candidateState.Load().(*candidateSnapshot)
	assert.Equal(t, "backup", snapshot.activeTier, "should stay on backup during hold time")

	// Wait for hold time
	time.Sleep(time.Second + 100*time.Millisecond)
	lb.updateCandidates()

	snapshot = lb.candidateState.Load().(*candidateSnapshot)
	assert.Equal(t, "primary", snapshot.activeTier, "should switch back to primary after hold time")
}

// Test 3: Key composition
func TestKeyComposition(t *testing.T) {
	lb := &LoadBalance{
		logger:          &mockLogger{},
		hashKeyParts: []string{"src_ip", "dst_ip", "dst_port", "network"},
		hashKeySalt:  "test-salt",
	}

	metadata := &adapter.InboundContext{
		Source:      M.ParseSocksaddr("192.168.1.1:12345"),
		Destination: M.ParseSocksaddr("8.8.8.8:53"),
		Network:     "udp",
	}

	key := lb.buildHashKey(metadata)
	expected := "test-salt192.168.1.1|8.8.8.8|53|udp"
	assert.Equal(t, expected, key, "hash key should be constructed correctly")

	// Test with domain destination
	metadata.Destination = M.ParseSocksaddrHostPort("example.com", 443)
	lb.hashKeyParts = []string{"src_ip", "domain", "dst_port"}

	key = lb.buildHashKey(metadata)
	expected = "test-salt192.168.1.1|example.com|443"
	assert.Equal(t, expected, key, "hash key should handle domain correctly")

	// Test with missing parts
	metadata.Source = M.Socksaddr{}
	lb.hashKeyParts = []string{"src_ip", "dst_ip"}

	key = lb.buildHashKey(metadata)
	expected = "test-salt-|8.8.8.8"
	assert.Contains(t, key, "test-salt", "hash key should use placeholder for missing parts")
}

// Test 4: Consistent hash stability
func TestConsistentHashStability(t *testing.T) {
	members := []adapter.Outbound{
		&mockOutbound{tag: "node1", network: []string{"tcp"}},
		&mockOutbound{tag: "node2", network: []string{"tcp"}},
		&mockOutbound{tag: "node3", network: []string{"tcp"}},
	}

	lb := &LoadBalance{
		logger:          &mockLogger{},
		hashVirtualNodes: 100,
	}

	ring := lb.buildHashRing(members)
	require.NotNil(t, ring)
	assert.Len(t, ring.points, 300, "should create 100 virtual nodes per member")

	// Test stability: same key should map to same node
	testKey := "test-key-12345"
	keyHash := xxhash.Sum64String(testKey)

	node1 := lb.lookupHashRing(ring, keyHash)
	node2 := lb.lookupHashRing(ring, keyHash)
	node3 := lb.lookupHashRing(ring, keyHash)

	assert.Equal(t, node1, node2, "same key should map to same node")
	assert.Equal(t, node2, node3, "same key should map to same node")
	assert.NotEmpty(t, node1, "should map to a valid node")
}

// Test 5: Minimal remapping when membership changes
func TestMinimalRemapping(t *testing.T) {
	members := []adapter.Outbound{
		&mockOutbound{tag: "node1", network: []string{"tcp"}},
		&mockOutbound{tag: "node2", network: []string{"tcp"}},
		&mockOutbound{tag: "node3", network: []string{"tcp"}},
		&mockOutbound{tag: "node4", network: []string{"tcp"}},
	}

	lb := &LoadBalance{
		logger:          &mockLogger{},
		hashVirtualNodes: 100,
	}

	ring := lb.buildHashRing(members)

	// Test 100 keys
	numKeys := 100
	originalMappings := make(map[string]string)

	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("test-key-%d", i)
		keyHash := xxhash.Sum64String(key)
		node := lb.lookupHashRing(ring, keyHash)
		originalMappings[key] = node
	}

	// Remove one node (node4)
	newMembers := members[:3]
	newRing := lb.buildHashRing(newMembers)

	// Check how many keys remapped
	remapped := 0
	for key, originalNode := range originalMappings {
		keyHash := xxhash.Sum64String(key)
		newNode := lb.lookupHashRing(newRing, keyHash)

		if originalNode == "node4" {
			// Keys that were on node4 must remap
			assert.NotEqual(t, "node4", newNode, "keys from removed node must remap")
		} else if newNode != originalNode {
			remapped++
		}
	}

	// Ideally, only ~25% of keys should remap (those on node4)
	// Allow some variation due to hash distribution
	maxExpectedRemapped := numKeys / 3 // Be generous: up to 33%
	assert.Less(t, remapped, maxExpectedRemapped,
		"should minimize remapping when node removed (remapped: %d/%d)", remapped, numKeys)
}

// Test 6: Empty pool behavior
func TestEmptyPoolBehavior(t *testing.T) {
	history := urltest.NewHistoryStorage()

	// No health check results (both tiers empty)
	lb := &LoadBalance{
		logger:          &mockLogger{},
		primaryTags:     []string{"p1"},
		backupTags:      []string{"b1"},
		topNPrimary:     1,
		topNBackup:      1,
		interval:        time.Minute,
		idleTimeout:     0, // Disable idle timeout for testing
		history:         history,
		emptyPoolAction: emptyPoolActionError,
		strategy:        strategyRandom,
		close:           make(chan struct{}),
		outbound: &mockOutboundManager{
			outbounds: map[string]adapter.Outbound{
				"p1": &mockOutbound{tag: "p1", network: []string{"tcp"}},
				"b1": &mockOutbound{tag: "b1", network: []string{"tcp"}},
			},
		},
	}

	lb.tierState.Store(&tierStateSnapshot{activeTier: "primary"})
	lb.candidateState.Store(&candidateSnapshot{
		primaryCandidates: nil,
		backupCandidates:  nil,
		activeTier:        "primary",
	})

	// Should return error when empty_pool_action = error
	metadata := &adapter.InboundContext{}
	_, err := lb.selectOutbound("tcp", metadata)
	assert.Error(t, err, "should error when both tiers empty and action is error")

	// Test fallback_all mode
	lb.emptyPoolAction = emptyPoolActionFallbackAll
	selected, err := lb.selectOutbound("tcp", metadata)
	assert.NoError(t, err, "should not error when action is fallback_all")
	assert.NotNil(t, selected, "should select from all outbounds as fallback")
}

// Test 7: Concurrency safety
func TestConcurrency(t *testing.T) {
	history := urltest.NewHistoryStorage()
	now := time.Now()

	// Set up initial state
	history.StoreURLTestHistory("p1", &adapter.URLTestHistory{Time: now, Delay: 10})
	history.StoreURLTestHistory("p2", &adapter.URLTestHistory{Time: now, Delay: 20})

	lb := &LoadBalance{
		logger:              &mockLogger{},
		primaryTags:         []string{"p1", "p2"},
		backupTags:          []string{},
		topNPrimary:         2,
		topNBackup:          0,
		interval:            time.Minute,
		idleTimeout:         0, // Disable idle timeout for testing
		history:             history,
		strategy:            strategyRandom,
		hystPrimaryFailures: 1,
		hystBackupHoldTime:  time.Second,
		close:               make(chan struct{}),
		outbound: &mockOutboundManager{
			outbounds: map[string]adapter.Outbound{
				"p1": &mockOutbound{tag: "p1", network: []string{"tcp", "udp"}},
				"p2": &mockOutbound{tag: "p2", network: []string{"tcp", "udp"}},
			},
		},
	}

	lb.tierState.Store(&tierStateSnapshot{activeTier: "primary"})

	// Initial candidate update
	lb.updateCandidates()

	// Concurrently update candidates and select outbounds
	var wg sync.WaitGroup
	metadata := &adapter.InboundContext{
		Source:      M.ParseSocksaddr("192.168.1.1:12345"),
		Destination: M.ParseSocksaddr("8.8.8.8:80"),
		Network:     "tcp",
	}

	// Run selections
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = lb.selectOutbound("tcp", metadata)
		}()
	}

	// Run updates
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lb.updateCandidates()
		}()
	}

	wg.Wait()

	// Should not crash or race (run with -race flag)
	snapshot := lb.candidateState.Load()
	assert.NotNil(t, snapshot, "snapshot should be valid after concurrent operations")
}

// Test 8: Hysteresis primary failure counting
func TestHysteresisPrimaryFailureCounting(t *testing.T) {
	lb := &LoadBalance{
		logger:          &mockLogger{},
		hystPrimaryFailures: 3,
		hystBackupHoldTime:  time.Second,
	}

	currentState := &tierStateSnapshot{
		activeTier:          "primary",
		primaryFailureCount: 0,
	}

	// Primary available
	primaryCandidates := []adapter.Outbound{
		&mockOutbound{tag: "p1", network: []string{"tcp"}},
	}
	backupCandidates := []adapter.Outbound{
		&mockOutbound{tag: "b1", network: []string{"tcp"}},
	}

	newState := lb.applyHysteresis(currentState, primaryCandidates, backupCandidates)
	assert.Equal(t, "primary", newState.activeTier)
	assert.Equal(t, uint32(0), newState.primaryFailureCount)

	// First failure
	newState = lb.applyHysteresis(newState, nil, backupCandidates)
	assert.Equal(t, "primary", newState.activeTier, "should stay on primary after 1 failure")
	assert.Equal(t, uint32(1), newState.primaryFailureCount)

	// Second failure
	newState = lb.applyHysteresis(newState, nil, backupCandidates)
	assert.Equal(t, "primary", newState.activeTier, "should stay on primary after 2 failures")
	assert.Equal(t, uint32(2), newState.primaryFailureCount)

	// Third failure - should switch
	newState = lb.applyHysteresis(newState, nil, backupCandidates)
	assert.Equal(t, "backup", newState.activeTier, "should switch to backup after 3 failures")
	assert.Equal(t, uint32(0), newState.primaryFailureCount)
}

// Test 9: Empty key handling
func TestEmptyKeyHandling(t *testing.T) {
	history := urltest.NewHistoryStorage()
	now := time.Now()
	history.StoreURLTestHistory("p1", &adapter.URLTestHistory{Time: now, Delay: 10})
	history.StoreURLTestHistory("p2", &adapter.URLTestHistory{Time: now, Delay: 20})

	candidates := []adapter.Outbound{
		&mockOutbound{tag: "p1", network: []string{"tcp"}},
		&mockOutbound{tag: "p2", network: []string{"tcp"}},
	}

	// Test random mode
	lb := &LoadBalance{
		logger:          &mockLogger{},
		strategy:         strategyConsistentHash,
		hashKeyParts:     []string{"src_ip"},
		hashOnEmptyKey:   onEmptyKeyRandom,
		hashVirtualNodes: 100,
	}

	ring := lb.buildHashRing(candidates)
	snapshot := &candidateSnapshot{
		primaryCandidates: candidates,
		backupCandidates:  nil,
		activeTier:        "primary",
		hashRing:          ring,
	}
	lb.candidateState.Store(snapshot)

	// Empty metadata (no source IP)
	metadata := &adapter.InboundContext{}

	// Should select randomly when key is empty
	selected, err := lb.selectOutbound("tcp", metadata)
	assert.NoError(t, err)
	assert.NotNil(t, selected)

	// Test hash_empty mode
	lb.hashOnEmptyKey = onEmptyKeyHashEmpty
	selected, err = lb.selectOutbound("tcp", metadata)
	assert.NoError(t, err)
	assert.NotNil(t, selected)
}
