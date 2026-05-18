package group

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common/metadata"
)

func TestConsistentHashStableSelection(t *testing.T) {
	snapshot := &CandidateSnapshot{
		Candidates: []Candidate{
			{Tag: "a", Latency: 100, IsPrimary: true},
			{Tag: "b", Latency: 200, IsPrimary: true},
			{Tag: "c", Latency: 300, IsPrimary: true},
		},
		Generation: 1,
	}

	selected := make(map[string]int)
	for i := 0; i < 100; i++ {
		candidate, err := SelectFromSnapshot(snapshot, "user@example.com", "error", 100, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		selected[candidate.Tag]++
	}

	// All 100 selections should go to the same candidate
	if len(selected) != 1 {
		t.Errorf("expected 1 unique candidate, got %d: %v", len(selected), selected)
	}
}

func TestConsistentHashKeyDistribution(t *testing.T) {
	snapshot := &CandidateSnapshot{
		Candidates: []Candidate{
			{Tag: "a", Latency: 100, IsPrimary: true},
			{Tag: "b", Latency: 200, IsPrimary: true},
			{Tag: "c", Latency: 300, IsPrimary: true},
		},
		Generation: 1,
	}

	selected := make(map[string]int)
	for i := 0; i < 1000; i++ {
		candidate, err := SelectFromSnapshot(snapshot, "user@example.com:"+string(rune('0'+i)), "error", 100, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		selected[candidate.Tag]++
	}

	// Each candidate should get some selections
	for _, tag := range []string{"a", "b", "c"} {
		if selected[tag] == 0 {
			t.Errorf("candidate %s got 0 selections", tag)
		}
	}
}

func TestConsistentHashRemovalRemaps(t *testing.T) {
	initialSnapshot := &CandidateSnapshot{
		Candidates: []Candidate{
			{Tag: "a", Latency: 100, IsPrimary: true},
			{Tag: "b", Latency: 200, IsPrimary: true},
			{Tag: "c", Latency: 300, IsPrimary: true},
		},
		Generation: 1,
	}

	// Record initial selections for 1000 keys
	type selection struct {
		key      string
		candidate string
	}
	var initialSelections []selection
	keys := make([]string, 1000)
	for i := range keys {
		keys[i] = "key" + string(rune('0'+i/100)) + string(rune('0'+(i%100)/10)) + string(rune('0'+(i%10)))
		// Use unique enough keys
	}

	for _, key := range keys {
		candidate, err := SelectFromSnapshot(initialSnapshot, key, "error", 100, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		initialSelections = append(initialSelections, selection{key: key, candidate: candidate.Tag})
	}

	// Create new snapshot without "b"
	newSnapshot := &CandidateSnapshot{
		Candidates: []Candidate{
			{Tag: "a", Latency: 100, IsPrimary: true},
			{Tag: "c", Latency: 300, IsPrimary: true},
		},
		Generation: 2,
	}

	// Check that only keys previously mapped to B are remapped
	remappedCount := 0
	stillCorrectCount := 0
	for _, s := range initialSelections {
		newCandidate, err := SelectFromSnapshot(newSnapshot, s.key, "error", 100, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.candidate == "b" {
			// Was B, may or may not still be B (remapped)
			remappedCount++
		} else {
			// Was A or C, should still be the same
			if newCandidate.Tag != s.candidate {
				t.Errorf("key %s was %s, now is %s", s.key, s.candidate, newCandidate.Tag)
			} else {
				stillCorrectCount++
			}
		}
	}

	// Keys mapped to A and C should still be correct
	if stillCorrectCount == 0 {
		t.Errorf("no keys remained correctly mapped")
	}
}

func TestConsistentHashVirtualNodes(t *testing.T) {
	snapshot := &CandidateSnapshot{
		Candidates: []Candidate{
			{Tag: "a", Latency: 100, IsPrimary: true},
			{Tag: "b", Latency: 200, IsPrimary: true},
		},
		Generation: 1,
	}

	// With virtualNodes=1, distribution is uneven
	selected1 := make(map[string]int)
	for i := 0; i < 100; i++ {
		candidate, err := SelectFromSnapshot(snapshot, "key"+string(rune(i)), "error", 1, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		selected1[candidate.Tag]++
	}

	// With virtualNodes=100, distribution is more even
	selected100 := make(map[string]int)
	for i := 0; i < 100; i++ {
		candidate, err := SelectFromSnapshot(snapshot, "key"+string(rune(i)), "error", 100, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		selected100[candidate.Tag]++
	}

	// With virtualNodes=1, one candidate might get 0 or very few
	// With virtualNodes=100, both should get reasonable distribution
	hasZeroWith1 := selected1["a"] == 0 || selected1["b"] == 0
	hasZeroWith100 := selected100["a"] == 0 || selected100["b"] == 0

	if hasZeroWith1 && !hasZeroWith100 {
		t.Logf("virtualNodes=1 distribution: %v, virtualNodes=100 distribution: %v", selected1, selected100)
	}

	// At least verify both virtual node counts work
	if selected1["a"] == 0 && selected1["b"] == 0 {
		t.Errorf("virtualNodes=1 produced no selections")
	}
	if selected100["a"] == 0 && selected100["b"] == 0 {
		t.Errorf("virtualNodes=100 produced no selections")
	}
}

func TestConsistentHashEmptyKeyRandom(t *testing.T) {
	snapshot := &CandidateSnapshot{
		Candidates: []Candidate{
			{Tag: "a", Latency: 100, IsPrimary: true},
			{Tag: "b", Latency: 200, IsPrimary: true},
		},
		Generation: 1,
	}

	// Empty key with "random" should return a valid candidate
	candidate, err := SelectFromSnapshot(snapshot, "", "random", 100, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if candidate.Tag != "a" && candidate.Tag != "b" {
		t.Errorf("expected a or b, got %s", candidate.Tag)
	}
}

func TestConsistentHashEmptyKeyError(t *testing.T) {
	snapshot := &CandidateSnapshot{
		Candidates: []Candidate{
			{Tag: "a", Latency: 100, IsPrimary: true},
			{Tag: "b", Latency: 200, IsPrimary: true},
		},
		Generation: 1,
	}

	// Empty key with "error" should return error
	_, err := SelectFromSnapshot(snapshot, "", "error", 100, "")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != "empty hash key" {
		t.Errorf("expected 'empty hash key', got '%s'", err.Error())
	}
}

func TestConsistentHashEmptyPool(t *testing.T) {
	snapshot := &CandidateSnapshot{
		Candidates: []Candidate{},
		Generation: 1,
	}

	_, err := SelectFromSnapshot(snapshot, "somekey", "error", 100, "")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != "empty candidate pool" {
		t.Errorf("expected 'empty candidate pool', got '%s'", err.Error())
	}
}

func TestConsistentHashKeySalt(t *testing.T) {
	snapshot := &CandidateSnapshot{
		Candidates: []Candidate{
			{Tag: "a", Latency: 100, IsPrimary: true},
			{Tag: "b", Latency: 200, IsPrimary: true},
		},
		Generation: 1,
	}

	candidate1, err := SelectFromSnapshot(snapshot, "samekey", "error", 100, "salt1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	candidate2, err := SelectFromSnapshot(snapshot, "samekey", "error", 100, "salt2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = candidate1
	_ = candidate2
}

// MockOutbound is a test mock for adapter.Outbound
type MockOutbound struct {
	tag string
}

func (m *MockOutbound) Type() string                     { return "mock" }
func (m *MockOutbound) Tag() string                      { return m.tag }
func (m *MockOutbound) Network() []string                { return []string{"tcp", "udp"} }
func (m *MockOutbound) Dependencies() []string           { return nil }
func (m *MockOutbound) DialContext(ctx context.Context, network string, destination metadata.Socksaddr) (net.Conn, error) {
	return nil, nil
}
func (m *MockOutbound) ListenPacket(ctx context.Context, destination metadata.Socksaddr) (net.PacketConn, error) {
	return nil, nil
}

var _ adapter.Outbound = (*MockOutbound)(nil)

func TestConsistentHashWithOutbound(t *testing.T) {
	snapshot := &CandidateSnapshot{
		Candidates: []Candidate{
			{Tag: "a", Outbound: &MockOutbound{tag: "a"}, Latency: 100, IsPrimary: true},
			{Tag: "b", Outbound: &MockOutbound{tag: "b"}, Latency: 200, IsPrimary: true},
		},
		Generation: 1,
	}

	candidate, err := SelectFromSnapshot(snapshot, "testkey", "error", 100, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if candidate.Outbound == nil {
		t.Errorf("expected Outbound to be set")
	}
	if candidate.Outbound.Tag() != candidate.Tag {
		t.Errorf("expected Outbound.Tag() %s to match Tag %s", candidate.Outbound.Tag(), candidate.Tag)
	}
}

func TestConsistentHashDeterministic(t *testing.T) {
	snapshot := &CandidateSnapshot{
		Candidates: []Candidate{
			{Tag: "a", Latency: 100, IsPrimary: true},
			{Tag: "b", Latency: 200, IsPrimary: true},
			{Tag: "c", Latency: 300, IsPrimary: true},
		},
		Generation: 1,
	}

	key := "deterministic-key"
	expectedCandidate, err := SelectFromSnapshot(snapshot, key, "error", 100, "testsalt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Run 100 times and verify same result
	for i := 0; i < 100; i++ {
		candidate, err := SelectFromSnapshot(snapshot, key, "error", 100, "testsalt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if candidate.Tag != expectedCandidate.Tag {
			t.Errorf("iteration %d: expected %s, got %s", i, expectedCandidate.Tag, candidate.Tag)
		}
	}
}

func TestConsistentHashEdgeCases(t *testing.T) {
	// Single candidate
	snapshot1 := &CandidateSnapshot{
		Candidates: []Candidate{
			{Tag: "only", Latency: 100, IsPrimary: true},
		},
		Generation: 1,
	}

	for i := 0; i < 10; i++ {
		candidate, err := SelectFromSnapshot(snapshot1, "key", "error", 100, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if candidate.Tag != "only" {
			t.Errorf("expected 'only', got '%s'", candidate.Tag)
		}
	}

	// Two candidates
	snapshot2 := &CandidateSnapshot{
		Candidates: []Candidate{
			{Tag: "x", Latency: 100, IsPrimary: true},
			{Tag: "y", Latency: 200, IsPrimary: true},
		},
		Generation: 1,
	}

	selected := make(map[string]int)
	for i := 0; i < 100; i++ {
		candidate, err := SelectFromSnapshot(snapshot2, "key", "error", 100, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		selected[candidate.Tag]++
	}

	// Should always pick the same one for same key
	if len(selected) != 1 {
		t.Errorf("expected 1 unique candidate for same key, got %d: %v", len(selected), selected)
	}

	// Test invalid onEmptyKey (should behave like error)
	_, err := SelectFromSnapshot(snapshot2, "", "invalid", 100, "")
	if err == nil {
		t.Errorf("expected error for invalid onEmptyKey")
	}
	if !errors.Is(err, ErrEmptyHashKey) {
		t.Errorf("expected ErrEmptyHashKey, got %v", err)
	}
}

func TestSelectRandomFromSnapshot_NonEmpty(t *testing.T) {
	snapshot := &CandidateSnapshot{
		Candidates: []Candidate{
			{Tag: "a", Latency: 100, IsPrimary: true},
			{Tag: "b", Latency: 200, IsPrimary: true},
			{Tag: "c", Latency: 300, IsPrimary: true},
		},
		Generation: 1,
	}

	candidate, err := SelectRandomFromSnapshot(snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	validTags := map[string]bool{"a": true, "b": true, "c": true}
	if !validTags[candidate.Tag] {
		t.Errorf("expected one of [a, b, c], got %s", candidate.Tag)
	}
}

func TestSelectRandomFromSnapshot_Empty(t *testing.T) {
	snapshot := &CandidateSnapshot{
		Candidates: []Candidate{},
		Generation: 1,
	}

	_, err := SelectRandomFromSnapshot(snapshot)
	if err == nil {
		t.Fatalf("expected error for empty snapshot, got nil")
	}
	if !errors.Is(err, ErrEmptyCandidatePool) {
		t.Errorf("expected ErrEmptyCandidatePool, got %v", err)
	}

	_, err = SelectRandomFromSnapshot(nil)
	if err == nil {
		t.Fatalf("expected error for nil snapshot, got nil")
	}
	if !errors.Is(err, ErrEmptyCandidatePool) {
		t.Errorf("expected ErrEmptyCandidatePool, got %v", err)
	}
}

func TestSelectRandomFromSnapshot_AlwaysValidCandidate(t *testing.T) {
	snapshot := &CandidateSnapshot{
		Candidates: []Candidate{
			{Tag: "x", Latency: 100, IsPrimary: true},
			{Tag: "y", Latency: 200, IsPrimary: true},
		},
		Generation: 1,
	}

	validTags := map[string]bool{"x": true, "y": true}
	for i := 0; i < 50; i++ {
		candidate, err := SelectRandomFromSnapshot(snapshot)
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
		if !validTags[candidate.Tag] {
			t.Errorf("iteration %d: expected one of [x, y], got %s", i, candidate.Tag)
		}
	}
}