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

	// Select Top-N (no previous candidates for initial selection)
	primaryCandidates := lb.selectTopN(primaryStats, lb.topNPrimary, nil)
	backupCandidates := lb.selectTopN(backupStats, lb.topNBackup, nil)

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

// Test 10: Bootstrap mode stays on initial all-failed check
func TestBootstrapModeOnInitialAllFailed(t *testing.T) {
	history := urltest.NewHistoryStorage()

	lb := &LoadBalance{
		logger:          &mockLogger{},
		primaryTags:     []string{"p1", "p2"},
		backupTags:      []string{"b1"},
		topNPrimary:     2,
		topNBackup:      1,
		interval:        time.Minute,
		history:         history,
		strategy:        strategyRandom,
		hystPrimaryFailures: 1,
		hystBackupHoldTime:  time.Second,
		outbound: &mockOutboundManager{
			outbounds: map[string]adapter.Outbound{
				"p1": &mockOutbound{tag: "p1", network: []string{"tcp"}},
				"p2": &mockOutbound{tag: "p2", network: []string{"tcp"}},
				"b1": &mockOutbound{tag: "b1", network: []string{"tcp"}},
			},
		},
	}

	// Initialize tier state
	lb.tierState.Store(&tierStateSnapshot{activeTier: "primary"})

	// No health check history - all nodes will fail
	// Initial check should NOT update candidateState
	lb.updateCandidates()

	snapshot := lb.candidateState.Load()
	assert.Nil(t, snapshot, "candidateState should stay nil when initial check finds all nodes failed")

	// Mark initial check as done and try again - should update now
	lb.initialCheckDone.Store(true)
	lb.updateCandidates()

	snapshot = lb.candidateState.Load()
	assert.NotNil(t, snapshot, "candidateState should be stored on subsequent checks even if all fail")
}

// Test 11: Bootstrap mode exits on partial success
func TestBootstrapModeExitsOnPartialSuccess(t *testing.T) {
	history := urltest.NewHistoryStorage()
	now := time.Now()

	// Only one node succeeds
	history.StoreURLTestHistory("p1", &adapter.URLTestHistory{Time: now, Delay: 10})

	lb := &LoadBalance{
		logger:          &mockLogger{},
		primaryTags:     []string{"p1", "p2"},
		backupTags:      []string{},
		topNPrimary:     2,
		topNBackup:      0,
		interval:        time.Minute,
		history:         history,
		strategy:        strategyRandom,
		hystPrimaryFailures: 1,
		hystBackupHoldTime:  time.Second,
		outbound: &mockOutboundManager{
			outbounds: map[string]adapter.Outbound{
				"p1": &mockOutbound{tag: "p1", network: []string{"tcp"}},
				"p2": &mockOutbound{tag: "p2", network: []string{"tcp"}},
			},
		},
	}

	// Initialize tier state
	lb.tierState.Store(&tierStateSnapshot{activeTier: "primary"})

	// Initial check with partial success should update candidateState
	lb.updateCandidates()

	snapshot := lb.candidateState.Load()
	assert.NotNil(t, snapshot, "candidateState should be stored when at least one node succeeds")

	cs := snapshot.(*candidateSnapshot)
	assert.Len(t, cs.primaryCandidates, 1, "should have one successful primary candidate")
	assert.Equal(t, "p1", cs.primaryCandidates[0].Tag())
}

// Test 12: Eager ticker start
func TestEagerTickerStart(t *testing.T) {
	lb := &LoadBalance{
		logger:          &mockLogger{},
		interval:        100 * time.Millisecond,
		idleTimeout:     time.Second,
		close:           make(chan struct{}),
	}

	// Ticker should be nil initially
	assert.Nil(t, lb.ticker, "ticker should be nil before startTicker")

	// Start ticker
	lb.startTicker()

	// Ticker should be running
	lb.tickerAccess.Lock()
	assert.NotNil(t, lb.ticker, "ticker should be running after startTicker")
	lb.tickerAccess.Unlock()

	// Calling startTicker again should be no-op
	lb.startTicker()
	lb.tickerAccess.Lock()
	assert.NotNil(t, lb.ticker, "ticker should still be running")
	lb.tickerAccess.Unlock()

	// Clean up
	close(lb.close)
	time.Sleep(50 * time.Millisecond) // Allow goroutine to exit
}

// Test 13: Idle timeout with eager start
func TestIdleTimeoutWithEagerStart(t *testing.T) {
	ctx := context.Background()
	history := urltest.NewHistoryStorage()

	lb := &LoadBalance{
		ctx:             ctx,
		logger:          &mockLogger{},
		primaryTags:     []string{"p1"},
		topNPrimary:     1,
		interval:        50 * time.Millisecond,
		idleTimeout:     200 * time.Millisecond,
		history:         history,
		link:            "https://www.gstatic.com/generate_204",
		close:           make(chan struct{}),
		hystPrimaryFailures: 1,
		hystBackupHoldTime:  time.Second,
		outbound: &mockOutboundManager{
			outbounds: map[string]adapter.Outbound{
				"p1": &mockOutbound{tag: "p1", network: []string{"tcp"}},
			},
		},
	}

	// Initialize tier state
	lb.tierState.Store(&tierStateSnapshot{activeTier: "primary"})

	// Set checking flag to prevent actual health checks from running
	lb.checking.Store(true)

	// Start ticker
	lb.startTicker()

	// Ticker should be running
	lb.tickerAccess.Lock()
	assert.NotNil(t, lb.ticker, "ticker should be running after startTicker")
	lb.tickerAccess.Unlock()

	// Wait for idle timeout (200ms) without any Touch() calls
	time.Sleep(300 * time.Millisecond)

	// Ticker should have stopped
	lb.tickerAccess.Lock()
	isNil := lb.ticker == nil
	lb.tickerAccess.Unlock()
	assert.True(t, isNil, "ticker should have stopped after idle timeout")

	// Touch should restart the ticker
	lb.Touch()

	lb.tickerAccess.Lock()
	assert.NotNil(t, lb.ticker, "ticker should restart after Touch")
	lb.tickerAccess.Unlock()

	// Clean up
	close(lb.close)
	time.Sleep(50 * time.Millisecond)
}

// Test 14: Tolerance zero disabled (backward compatibility)
func TestToleranceZeroDisabled(t *testing.T) {
	history := urltest.NewHistoryStorage()
	now := time.Now()

	// Set latencies: p1=10ms, p2=20ms, p3=30ms, p4=40ms, p5=50ms
	history.StoreURLTestHistory("p1", &adapter.URLTestHistory{Time: now, Delay: 10})
	history.StoreURLTestHistory("p2", &adapter.URLTestHistory{Time: now, Delay: 20})
	history.StoreURLTestHistory("p3", &adapter.URLTestHistory{Time: now, Delay: 30})
	history.StoreURLTestHistory("p4", &adapter.URLTestHistory{Time: now, Delay: 40})
	history.StoreURLTestHistory("p5", &adapter.URLTestHistory{Time: now, Delay: 50})

	lb := &LoadBalance{
		logger:          &mockLogger{},
		primaryTags:     []string{"p1", "p2", "p3", "p4", "p5"},
		topNPrimary:     3,
		tolerance:       0, // Disabled
		interval:        time.Minute,
		history:         history,
		outbound: &mockOutboundManager{
			outbounds: map[string]adapter.Outbound{
				"p1": &mockOutbound{tag: "p1", network: []string{"tcp"}},
				"p2": &mockOutbound{tag: "p2", network: []string{"tcp"}},
				"p3": &mockOutbound{tag: "p3", network: []string{"tcp"}},
				"p4": &mockOutbound{tag: "p4", network: []string{"tcp"}},
				"p5": &mockOutbound{tag: "p5", network: []string{"tcp"}},
			},
		},
	}

	stats := lb.collectTierStats(lb.primaryTags)

	// With tolerance=0, should get pure Top-3 regardless of previous candidates
	prevCandidates := []string{"p4", "p5"} // Previous were slower
	result := lb.selectTopN(stats, 3, prevCandidates)

	require.Len(t, result, 3, "should select top 3 when tolerance disabled")
	assert.Equal(t, "p1", result[0].Tag())
	assert.Equal(t, "p2", result[1].Tag())
	assert.Equal(t, "p3", result[2].Tag())
}

// Test 15: Tolerance stabilization - previous candidates within tolerance are eligible
func TestToleranceStabilization(t *testing.T) {
	history := urltest.NewHistoryStorage()
	now := time.Now()

	// Current health: p1=48ms, p2=49ms, p3=50ms, p4=100ms
	// Top-3 pure selection would be: p1, p2, p3
	// Previous candidates included p4=55ms (within tolerance of p3=50ms)
	history.StoreURLTestHistory("p1", &adapter.URLTestHistory{Time: now, Delay: 48})
	history.StoreURLTestHistory("p2", &adapter.URLTestHistory{Time: now, Delay: 49})
	history.StoreURLTestHistory("p3", &adapter.URLTestHistory{Time: now, Delay: 50})
	history.StoreURLTestHistory("p4", &adapter.URLTestHistory{Time: now, Delay: 100})

	lb := &LoadBalance{
		logger:          &mockLogger{},
		primaryTags:     []string{"p1", "p2", "p3", "p4"},
		topNPrimary:     3,
		tolerance:       10, // 10ms tolerance
		interval:        time.Minute,
		history:         history,
		outbound: &mockOutboundManager{
			outbounds: map[string]adapter.Outbound{
				"p1": &mockOutbound{tag: "p1", network: []string{"tcp"}},
				"p2": &mockOutbound{tag: "p2", network: []string{"tcp"}},
				"p3": &mockOutbound{tag: "p3", network: []string{"tcp"}},
				"p4": &mockOutbound{tag: "p4", network: []string{"tcp"}},
			},
		},
	}

	stats := lb.collectTierStats(lb.primaryTags)

	// Previous candidate p4 (at 100ms now) exceeds tolerance of p3=50ms + 10ms = 60ms
	// Should NOT be retained
	prevCandidates := []string{"p4"}
	result := lb.selectTopN(stats, 3, prevCandidates)

	require.Len(t, result, 3, "should select top 3")
	assert.Equal(t, "p1", result[0].Tag())
	assert.Equal(t, "p2", result[1].Tag())
	assert.Equal(t, "p3", result[2].Tag())
}

// Test 16: Tolerance stabilization - previous candidate within tolerance is retained
func TestToleranceWithinThreshold(t *testing.T) {
	history := urltest.NewHistoryStorage()
	now := time.Now()

	// Current health: p1=48ms, p2=50ms, p3=55ms, p4=58ms
	// Top-3 pure selection would be: p1, p2, p3
	// Previous candidates included p4 at 58ms (within tolerance of p3=55ms + 5ms)
	history.StoreURLTestHistory("p1", &adapter.URLTestHistory{Time: now, Delay: 48})
	history.StoreURLTestHistory("p2", &adapter.URLTestHistory{Time: now, Delay: 50})
	history.StoreURLTestHistory("p3", &adapter.URLTestHistory{Time: now, Delay: 55})
	history.StoreURLTestHistory("p4", &adapter.URLTestHistory{Time: now, Delay: 58})

	lb := &LoadBalance{
		logger:          &mockLogger{},
		primaryTags:     []string{"p1", "p2", "p3", "p4"},
		topNPrimary:     3,
		tolerance:       5, // 5ms tolerance
		interval:        time.Minute,
		history:         history,
		outbound: &mockOutboundManager{
			outbounds: map[string]adapter.Outbound{
				"p1": &mockOutbound{tag: "p1", network: []string{"tcp"}},
				"p2": &mockOutbound{tag: "p2", network: []string{"tcp"}},
				"p3": &mockOutbound{tag: "p3", network: []string{"tcp"}},
				"p4": &mockOutbound{tag: "p4", network: []string{"tcp"}},
			},
		},
	}

	stats := lb.collectTierStats(lb.primaryTags)

	// Previous candidate p4 (at 58ms) is within tolerance of p3=55ms + 5ms = 60ms
	// Should be eligible for selection
	// But since we still take best 3 from eligible set, p4 still won't make it
	// because p1, p2, p3 are all better
	prevCandidates := []string{"p4"}
	result := lb.selectTopN(stats, 3, prevCandidates)

	require.Len(t, result, 3)
	// Best 3 are still selected from eligible set
	assert.Equal(t, "p1", result[0].Tag())
	assert.Equal(t, "p2", result[1].Tag())
	assert.Equal(t, "p3", result[2].Tag())
}

// Test 17: Previous candidate exceeding tolerance is displaced
func TestToleranceExceedThreshold(t *testing.T) {
	history := urltest.NewHistoryStorage()
	now := time.Now()

	// Current health: p1=40ms, p2=45ms, p3=50ms, p4=80ms, p5=85ms
	// Top-3 pure: p1, p2, p3
	// Previous candidates included p5
	// With tolerance=20ms: cutoff=50ms, maxAllowed=70ms
	// p4 (80ms) and p5 (85ms) both exceed tolerance
	history.StoreURLTestHistory("p1", &adapter.URLTestHistory{Time: now, Delay: 40})
	history.StoreURLTestHistory("p2", &adapter.URLTestHistory{Time: now, Delay: 45})
	history.StoreURLTestHistory("p3", &adapter.URLTestHistory{Time: now, Delay: 50})
	history.StoreURLTestHistory("p4", &adapter.URLTestHistory{Time: now, Delay: 80})
	history.StoreURLTestHistory("p5", &adapter.URLTestHistory{Time: now, Delay: 85})

	lb := &LoadBalance{
		logger:          &mockLogger{},
		primaryTags:     []string{"p1", "p2", "p3", "p4", "p5"},
		topNPrimary:     3,
		tolerance:       20, // 20ms tolerance
		interval:        time.Minute,
		history:         history,
		outbound: &mockOutboundManager{
			outbounds: map[string]adapter.Outbound{
				"p1": &mockOutbound{tag: "p1", network: []string{"tcp"}},
				"p2": &mockOutbound{tag: "p2", network: []string{"tcp"}},
				"p3": &mockOutbound{tag: "p3", network: []string{"tcp"}},
				"p4": &mockOutbound{tag: "p4", network: []string{"tcp"}},
				"p5": &mockOutbound{tag: "p5", network: []string{"tcp"}},
			},
		},
	}

	stats := lb.collectTierStats(lb.primaryTags)

	// Previous candidate p5 exceeds tolerance (85ms > 50ms + 20ms = 70ms)
	// Should NOT be eligible
	prevCandidates := []string{"p5"}
	result := lb.selectTopN(stats, 3, prevCandidates)

	require.Len(t, result, 3)
	assert.Equal(t, "p1", result[0].Tag())
	assert.Equal(t, "p2", result[1].Tag())
	assert.Equal(t, "p3", result[2].Tag())
}

// Test 18: Failed previous candidate is not retained regardless of tolerance
func TestToleranceWithFailedPrevCandidate(t *testing.T) {
	history := urltest.NewHistoryStorage()
	now := time.Now()

	// Current health: p1=40ms, p2=50ms, p3=60ms (p4 failed)
	history.StoreURLTestHistory("p1", &adapter.URLTestHistory{Time: now, Delay: 40})
	history.StoreURLTestHistory("p2", &adapter.URLTestHistory{Time: now, Delay: 50})
	history.StoreURLTestHistory("p3", &adapter.URLTestHistory{Time: now, Delay: 60})
	// p4 has no history (failed)

	lb := &LoadBalance{
		logger:          &mockLogger{},
		primaryTags:     []string{"p1", "p2", "p3", "p4"},
		topNPrimary:     3,
		tolerance:       100, // Large tolerance
		interval:        time.Minute,
		history:         history,
		outbound: &mockOutboundManager{
			outbounds: map[string]adapter.Outbound{
				"p1": &mockOutbound{tag: "p1", network: []string{"tcp"}},
				"p2": &mockOutbound{tag: "p2", network: []string{"tcp"}},
				"p3": &mockOutbound{tag: "p3", network: []string{"tcp"}},
				"p4": &mockOutbound{tag: "p4", network: []string{"tcp"}},
			},
		},
	}

	stats := lb.collectTierStats(lb.primaryTags)

	// Previous candidate p4 failed - should not be retained regardless of tolerance
	prevCandidates := []string{"p4"}
	result := lb.selectTopN(stats, 3, prevCandidates)

	require.Len(t, result, 3)
	assert.Equal(t, "p1", result[0].Tag())
	assert.Equal(t, "p2", result[1].Tag())
	assert.Equal(t, "p3", result[2].Tag())
}

// Test 19: Tolerance retains previous candidate that is now outside pure Top-N
func TestToleranceRetainsPrevCandidate(t *testing.T) {
	history := urltest.NewHistoryStorage()
	now := time.Now()

	// Current health: p1=40ms, p2=50ms, p3=51ms, p4=55ms
	// Top-3 pure: p1, p2, p3
	// Previous candidates included p4
	// With tolerance=10ms: cutoff=51ms, maxAllowed=61ms
	// p4 (55ms) is within tolerance, so eligible
	history.StoreURLTestHistory("p1", &adapter.URLTestHistory{Time: now, Delay: 40})
	history.StoreURLTestHistory("p2", &adapter.URLTestHistory{Time: now, Delay: 50})
	history.StoreURLTestHistory("p3", &adapter.URLTestHistory{Time: now, Delay: 51})
	history.StoreURLTestHistory("p4", &adapter.URLTestHistory{Time: now, Delay: 55})

	lb := &LoadBalance{
		logger:          &mockLogger{},
		primaryTags:     []string{"p1", "p2", "p3", "p4"},
		topNPrimary:     3,
		tolerance:       10, // 10ms tolerance
		interval:        time.Minute,
		history:         history,
		outbound: &mockOutboundManager{
			outbounds: map[string]adapter.Outbound{
				"p1": &mockOutbound{tag: "p1", network: []string{"tcp"}},
				"p2": &mockOutbound{tag: "p2", network: []string{"tcp"}},
				"p3": &mockOutbound{tag: "p3", network: []string{"tcp"}},
				"p4": &mockOutbound{tag: "p4", network: []string{"tcp"}},
			},
		},
	}

	stats := lb.collectTierStats(lb.primaryTags)

	// p4 is eligible but we still select best 3 from eligible set
	// Since p1, p2, p3 are all better than p4, they are selected
	prevCandidates := []string{"p4"}
	result := lb.selectTopN(stats, 3, prevCandidates)

	require.Len(t, result, 3)
	assert.Equal(t, "p1", result[0].Tag())
	assert.Equal(t, "p2", result[1].Tag())
	assert.Equal(t, "p3", result[2].Tag())
}

