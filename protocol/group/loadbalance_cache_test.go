package group

import (
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/interrupt"
	"github.com/sagernet/sing-box/common/transportstats"
	"github.com/sagernet/sing-box/common/urltest"
)

func TestMemberProbeUsesNestedSnapshotMin(t *testing.T) {
	t.Parallel()
	child := &LoadBalance{
		tags: []string{"x", "y"},
	}
	child.snapshot.Store(&CandidateSnapshot{
		Candidates: []Candidate{
			{Tag: "x", Latency: 80, Outbound: &seedStub{tag: "x"}},
			{Tag: "y", Latency: 50, Outbound: &seedStub{tag: "y"}},
		},
	})
	parent := &LoadBalance{history: urltest.NewHistoryStorage()}
	delay, action := parent.memberProbe(child)
	if action != probeUseCache {
		t.Fatalf("action %d want probeUseCache", action)
	}
	if delay != 50 {
		t.Fatalf("delay %d want 50", delay)
	}
}

func TestMemberProbeSkipsNestedSeed(t *testing.T) {
	t.Parallel()
	child := &LoadBalance{
		tags:        []string{"leaf"},
		primaryTags: []string{"leaf"},
		primaryOutbounds: map[string]adapter.Outbound{
			"leaf": &seedStub{tag: "leaf"},
		},
		history: urltest.NewHistoryStorage(),
	}
	child.seedInitialSnapshot()
	parent := &LoadBalance{history: urltest.NewHistoryStorage()}
	delay, action := parent.memberProbe(child)
	if action != probeSkip {
		t.Fatalf("action %d delay %d want probeSkip", action, delay)
	}
}

func TestNestedMinDelayPicksMinimum(t *testing.T) {
	t.Parallel()
	child := &LoadBalance{}
	child.snapshot.Store(&CandidateSnapshot{
		Candidates: []Candidate{
			{Tag: "a", Latency: 80},
			{Tag: "b", Latency: 50},
		},
	})
	delay, ok := nestedMinDelay(child, urltest.NewHistoryStorage())
	if !ok || delay != 50 {
		t.Fatalf("got %d ok=%v want 50", delay, ok)
	}
}

func TestRebuildSnapshotKeepsSeedWhenNestedPending(t *testing.T) {
	t.Parallel()
	child := &LoadBalance{
		tags:        []string{"leaf"},
		primaryTags: []string{"leaf"},
		primaryOutbounds: map[string]adapter.Outbound{
			"leaf": &seedStub{tag: "leaf"},
		},
	}
	child.seedInitialSnapshot()
	parent := &LoadBalance{
		primaryTags: []string{"child"},
		primaryOutbounds: map[string]adapter.Outbound{
			"child": child,
		},
		emptyPoolAction: "error",
		history:         urltest.NewHistoryStorage(),
	}
	parent.seedInitialSnapshot()
	parent.rebuildSnapshot()
	got, err := parent.selectCandidate(adapter.InboundContext{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Tag != "child" {
		t.Fatalf("kept seed want child, got %q", got.Tag)
	}
}

func TestRebuildSnapshotRanksNestedByMinLatency(t *testing.T) {
	t.Parallel()
	slow := &LoadBalance{}
	slow.snapshot.Store(&CandidateSnapshot{
		Candidates: []Candidate{{Tag: "s1", Latency: 80, Outbound: &seedStub{tag: "s1"}}},
	})
	fast := &LoadBalance{}
	fast.snapshot.Store(&CandidateSnapshot{
		Candidates: []Candidate{{Tag: "f1", Latency: 50, Outbound: &seedStub{tag: "f1"}}},
	})
	parent := &LoadBalance{
		primaryTags: []string{"slow", "fast"},
		primaryOutbounds: map[string]adapter.Outbound{
			"slow": slow,
			"fast": fast,
		},
		topNPrimary:    1,
		history:        urltest.NewHistoryStorage(),
		interruptGroup: interrupt.NewGroup(),
	}
	parent.seedInitialSnapshot()
	parent.rebuildSnapshot()
	if parent.Now() != "fast" {
		t.Fatalf("Now()=%q want fast", parent.Now())
	}
}

func TestHealthyCandidatesScoresWithLatencyWindow(t *testing.T) {
	t.Parallel()
	memberSorter, err := newSorter(map[string]float64{MetricLatency: 0.5, "latency_avg_5m": 0.5})
	if err != nil {
		t.Fatal(err)
	}
	lb := &LoadBalance{
		sorter:       memberSorter,
		latencyStats: make(map[string]*transportstats.Recorder),
		history:      urltest.NewHistoryStorage(),
		primaryTags:  []string{"a", "b"},
		primaryOutbounds: map[string]adapter.Outbound{
			"a": &seedStub{tag: "a"},
			"b": &seedStub{tag: "b"},
		},
	}
	now := time.Now()
	lb.history.StoreURLTestHistory("a", &adapter.URLTestHistory{Time: now, Delay: 50})
	lb.history.StoreURLTestHistory("b", &adapter.URLTestHistory{Time: now, Delay: 10})
	lb.observeDelay("b", 100)
	lb.observeDelay("b", 100)
	lb.observeDelay("b", 10)
	got := lb.healthyCandidates(lb.primaryTags, lb.primaryOutbounds, true)

	var candidateB *Candidate
	for i := range got {
		if got[i].Tag == "b" {
			candidateB = &got[i]
		}
	}
	if candidateB == nil {
		t.Fatal("missing candidate b")
	}
	// Latency keeps the raw measurement, while the blended value moves to Score:
	// the window averages 70 and the latest sample is 10, so (70 + 10) / 2 = 40.
	if candidateB.Latency != 10 {
		t.Fatalf("b latency %d want raw 10", candidateB.Latency)
	}
	if candidateB.Score < 39.999 || candidateB.Score > 40.001 {
		t.Fatalf("b score %v want blended 40", candidateB.Score)
	}
	// "a" has no window history, so its window key falls back to its latest
	// delay of 50 and it ranks behind "b".
	if got[0].Tag != "b" {
		t.Fatalf("ranking got %v want b first", got)
	}
}
