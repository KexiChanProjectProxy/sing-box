package group

import (
	"context"
	"errors"
	"math/rand"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/interrupt"
	"github.com/sagernet/sing-box/common/transportstats"
	"github.com/sagernet/sing-box/common/urltest"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json/badoption"

	"github.com/stretchr/testify/require"
)

// statsStub is a leaf outbound whose protocol reports transport statistics.
type statsStub struct {
	seedStub
	access  sync.Mutex
	enabled []adapter.TransportStatsDirection
	readers map[adapter.TransportStatsDirection]adapter.TransportStatsReader
}

func newStatsStub(tag string) *statsStub {
	return &statsStub{
		seedStub: seedStub{tag: tag},
		readers:  make(map[adapter.TransportStatsDirection]adapter.TransportStatsReader),
	}
}

func (s *statsStub) EnableTransportStats(direction adapter.TransportStatsDirection) {
	s.access.Lock()
	defer s.access.Unlock()
	s.enabled = append(s.enabled, direction)
}

func (s *statsStub) TransportStats(direction adapter.TransportStatsDirection) adapter.TransportStatsReader {
	s.access.Lock()
	defer s.access.Unlock()
	reader, loaded := s.readers[direction]
	if !loaded {
		// An outbound that has nothing for this direction must return an
		// untyped nil, never a typed nil wrapped in the interface.
		return nil
	}
	return reader
}

func (s *statsStub) enabledDirections() []adapter.TransportStatsDirection {
	s.access.Lock()
	defer s.access.Unlock()
	return append([]adapter.TransportStatsDirection(nil), s.enabled...)
}

// groupStub is a nested group outbound that resolves to one of its members.
type groupStub struct {
	seedStub
	members []string
	current string
}

func (g *groupStub) Now() string   { return g.current }
func (g *groupStub) All() []string { return g.members }
func (g *groupStub) Type() string  { return "selector" }

// stubManager is the smallest OutboundManager that resolves tags.
type stubManager struct {
	outbounds map[string]adapter.Outbound
}

func (m *stubManager) Start(adapter.StartStage) error { return nil }
func (m *stubManager) Close() error                   { return nil }
func (m *stubManager) Outbounds() []adapter.Outbound  { return nil }
func (m *stubManager) Outbound(tag string) (adapter.Outbound, bool) {
	detour, loaded := m.outbounds[tag]
	return detour, loaded
}
func (m *stubManager) Default() adapter.Outbound { return nil }
func (m *stubManager) Remove(string) error       { return nil }
func (m *stubManager) Create(context.Context, adapter.Router, log.StructuredLogger, string, string, any) error {
	return errors.New("unused")
}

func newStatsBalance(t *testing.T, weights map[string]float64, members map[string]adapter.Outbound) *LoadBalance {
	t.Helper()
	memberSorter, err := newSorter(weights)
	require.NoError(t, err)
	tags := make([]string, 0, len(members))
	for tag := range members {
		tags = append(tags, tag)
	}
	lb := &LoadBalance{
		sorter:           memberSorter,
		logger:           log.NewNOPFactory().Logger(),
		primaryTags:      tags,
		primaryOutbounds: members,
		backupOutbounds:  map[string]adapter.Outbound{},
		outbound:         &stubManager{outbounds: members},
	}
	if memberSorter.usesLatencyWindow {
		lb.latencyStats = make(map[string]*transportstats.Recorder)
	}
	return lb
}

func TestEnableTransportStatsOnlyEnablesConfiguredDirections(t *testing.T) {
	t.Parallel()
	member := newStatsStub("a")
	lb := newStatsBalance(t, map[string]float64{MetricLatency: 1, "client_rtt": 1},
		map[string]adapter.Outbound{"a": member})
	lb.enableTransportStats()
	require.Equal(t, []adapter.TransportStatsDirection{adapter.TransportStatsClient}, member.enabledDirections())
	require.True(t, lb.statsEnabled)
}

func TestEnableTransportStatsEnablesBothDirections(t *testing.T) {
	t.Parallel()
	member := newStatsStub("a")
	lb := newStatsBalance(t, map[string]float64{"client_rtt": 1, "server_rtt": 1},
		map[string]adapter.Outbound{"a": member})
	lb.enableTransportStats()
	require.ElementsMatch(t, []adapter.TransportStatsDirection{
		adapter.TransportStatsClient, adapter.TransportStatsServer,
	}, member.enabledDirections())
}

func TestEnableTransportStatsSkippedWithoutTransportKeys(t *testing.T) {
	t.Parallel()
	member := newStatsStub("a")
	lb := newStatsBalance(t, map[string]float64{MetricLatency: 1, "latency_avg_5m": 1},
		map[string]adapter.Outbound{"a": member})
	lb.enableTransportStats()
	require.Empty(t, member.enabledDirections(), "a latency only sorter must not start collection")
	require.False(t, lb.statsEnabled)
}

// A nested group carries traffic through one of its own members, so collection
// has to be enabled on those rather than on the group.
func TestEnableTransportStatsDescendsIntoNestedGroups(t *testing.T) {
	t.Parallel()
	leafA := newStatsStub("leaf-a")
	leafB := newStatsStub("leaf-b")
	nested := &groupStub{seedStub: seedStub{tag: "nested"}, members: []string{"leaf-a", "leaf-b"}, current: "leaf-a"}
	members := map[string]adapter.Outbound{"nested": nested, "leaf-a": leafA, "leaf-b": leafB}
	lb := &LoadBalance{
		sorter:           mustSorter(t, map[string]float64{"client_rtt": 1}),
		logger:           log.NewNOPFactory().Logger(),
		primaryTags:      []string{"nested"},
		primaryOutbounds: map[string]adapter.Outbound{"nested": nested},
		backupOutbounds:  map[string]adapter.Outbound{},
		outbound:         &stubManager{outbounds: members},
	}
	lb.enableTransportStats()
	require.Len(t, leafA.enabledDirections(), 1, "every reachable leaf must collect, not only the current one")
	require.Len(t, leafB.enabledDirections(), 1)
}

func TestEnableTransportStatsSurvivesGroupCycle(t *testing.T) {
	t.Parallel()
	// A group that lists itself would loop without the depth and seen guards.
	cyclic := &groupStub{seedStub: seedStub{tag: "loop"}, members: []string{"loop"}, current: "loop"}
	members := map[string]adapter.Outbound{"loop": cyclic}
	lb := &LoadBalance{
		sorter:           mustSorter(t, map[string]float64{"client_rtt": 1}),
		logger:           log.NewNOPFactory().Logger(),
		primaryTags:      []string{"loop"},
		primaryOutbounds: members,
		backupOutbounds:  map[string]adapter.Outbound{},
		outbound:         &stubManager{outbounds: members},
	}
	done := make(chan struct{})
	go func() {
		lb.enableTransportStats()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("enableTransportStats did not terminate on a group cycle")
	}
}

// Reading the wrong direction would be invisible without distinguishable
// readers, so give the two different values and assert which one lands.
func TestScoreCandidatesReadsTheRequestedDirection(t *testing.T) {
	t.Parallel()
	member := newStatsStub("a")
	member.readers[adapter.TransportStatsClient] = &fakeStats{rtt: 11}
	member.readers[adapter.TransportStatsServer] = &fakeStats{rtt: 97}
	lb := newStatsBalance(t, map[string]float64{"client_rtt": 1}, map[string]adapter.Outbound{"a": member})
	lb.enableTransportStats()

	candidates := []Candidate{{Tag: "a", Outbound: member, Latency: 5}}
	lb.scoreCandidates(candidates)
	require.InDelta(t, 11, candidates[0].Score, 1e-9, "client_rtt must read the client reader")
}

func TestScoreCandidatesResolvesNestedGroupSelection(t *testing.T) {
	t.Parallel()
	leafA := newStatsStub("leaf-a")
	leafA.readers[adapter.TransportStatsClient] = &fakeStats{rtt: 20}
	leafB := newStatsStub("leaf-b")
	leafB.readers[adapter.TransportStatsClient] = &fakeStats{rtt: 80}
	nested := &groupStub{seedStub: seedStub{tag: "nested"}, members: []string{"leaf-a", "leaf-b"}, current: "leaf-a"}
	members := map[string]adapter.Outbound{"nested": nested, "leaf-a": leafA, "leaf-b": leafB}
	lb := &LoadBalance{
		sorter:           mustSorter(t, map[string]float64{"client_rtt": 1}),
		logger:           log.NewNOPFactory().Logger(),
		primaryTags:      []string{"nested"},
		primaryOutbounds: map[string]adapter.Outbound{"nested": nested},
		backupOutbounds:  map[string]adapter.Outbound{},
		outbound:         &stubManager{outbounds: members},
	}
	lb.enableTransportStats()

	candidates := []Candidate{{Tag: "nested", Outbound: nested, Latency: 5}}
	lb.scoreCandidates(candidates)
	require.InDelta(t, 20, candidates[0].Score, 1e-9, "a nested group is scored on its current selection")

	nested.current = "leaf-b"
	lb.scoreCandidates(candidates)
	require.InDelta(t, 80, candidates[0].Score, 1e-9, "the selection is re-resolved on each scoring round")
}

func TestScoreCandidatesWithoutStatsEnabledReadsNothing(t *testing.T) {
	t.Parallel()
	member := newStatsStub("a")
	member.readers[adapter.TransportStatsClient] = &fakeStats{rtt: 999}
	lb := newStatsBalance(t, map[string]float64{MetricLatency: 1}, map[string]adapter.Outbound{"a": member})
	lb.enableTransportStats()
	candidates := []Candidate{{Tag: "a", Outbound: member, Latency: 42}}
	lb.scoreCandidates(candidates)
	require.InDelta(t, 42, candidates[0].Score, 1e-9)
}

// A metric no member can report contributes the same amount to every score, so
// the members it cannot tell apart must still be ordered by their raw delay
// rather than alphabetically by tag.
func TestScoreTiesBreakOnLatencyNotTag(t *testing.T) {
	t.Parallel()
	// Tags are deliberately in the opposite order to the delays.
	candidates := []Candidate{
		{Tag: "a", Latency: 300},
		{Tag: "b", Latency: 100},
	}
	memberSorter := mustSorter(t, map[string]float64{"client_rtt": 1, "server_loss_rate_1m": 20})
	memberSorter.score(candidates, func(Candidate) metricSource {
		return metricSource{}
	})
	require.InDelta(t, candidates[0].Score, candidates[1].Score, 1e-9, "an unreportable pool scores equally")
	sortCandidatesByScore(candidates)
	require.Equal(t, "b", candidates[0].Tag, "must rank on delay, not alphabetically")
}

// The same applies when only some members report: the rest share the pool
// average and must not be ordered by tag among themselves.
func TestScorePartialCoverageTiesBreakOnLatency(t *testing.T) {
	t.Parallel()
	candidates := []Candidate{
		{Tag: "a-slow", Latency: 500},
		{Tag: "z-fast", Latency: 20},
		{Tag: "m-reports", Latency: 300},
	}
	sources := map[string]metricSource{
		"a-slow":    {latency: 500},
		"z-fast":    {latency: 20},
		"m-reports": {latency: 300, client: &fakeStats{rtt: 50}},
	}
	memberSorter := mustSorter(t, map[string]float64{"client_rtt": 1})
	memberSorter.score(candidates, func(candidate Candidate) metricSource {
		return sources[candidate.Tag]
	})
	sortCandidatesByScore(candidates)
	require.Equal(t, "z-fast", candidates[0].Tag, "the fastest member must not lose to a tag tie-break")
}

// Equal scores must also pick the right member to evict.
func TestWorstCandidateBreaksTiesOnLatency(t *testing.T) {
	t.Parallel()
	candidates := []Candidate{
		{Tag: "a", Latency: 20, Score: 50},
		{Tag: "b", Latency: 900, Score: 50},
		{Tag: "c", Latency: 100, Score: 50},
	}
	require.Equal(t, 1, worstCandidateIndex(candidates))
}

func TestSorterNaNScoreRanksLast(t *testing.T) {
	t.Parallel()
	// Two enormous weights of opposite sign produce +Inf + -Inf = NaN.
	memberSorter := mustSorter(t, map[string]float64{"client_rtt": 1e308, "client_delivery_rate": 1e308})
	candidates := []Candidate{
		{Tag: "broken", Latency: 10},
		{Tag: "fine", Latency: 20},
	}
	memberSorter.score(candidates, func(candidate Candidate) metricSource {
		if candidate.Tag == "broken" {
			return metricSource{client: &fakeStats{rtt: 10, deliveryRate: 10}}
		}
		return metricSource{client: &fakeStats{rtt: 1, deliveryRate: 0.000001}}
	})
	sortCandidatesByScore(candidates)
	require.Equal(t, "fine", candidates[0].Tag, "a NaN score must never win the pool")
}

// A failed probe or dial resets the history, and the next round must then score
// on the fresh sample alone rather than on anything left over.
func TestResetWindowThenScoreUsesLatestOnly(t *testing.T) {
	t.Parallel()
	lb := &LoadBalance{
		sorter:       mustSorter(t, map[string]float64{"latency_avg_5m": 0.5, MetricLatency: 0.5}),
		latencyStats: make(map[string]*transportstats.Recorder),
	}
	lb.observeDelay("a", 900)
	lb.observeDelay("a", 900)
	lb.resetWindow("a")
	lb.observeDelay("a", 40)

	candidates := []Candidate{{Tag: "a", Latency: 40}}
	lb.scoreCandidates(candidates)
	require.InDelta(t, 40, candidates[0].Score, 1e-9, "the pre-reset samples must not survive")
}

// The two window keys must actually cover different spans, and each key must be
// wired to its own width. Asserting on the recorder alone would not notice the
// metric table mapping both keys to the same window, so score through the
// sorter instead.
func TestLatencyWindowKeysCoverDifferentSpans(t *testing.T) {
	t.Parallel()
	scoreWithKey := func(key string) float64 {
		clock := &testClock{now: time.Unix(1700000000, 0)}
		lb := &LoadBalance{
			sorter:                 mustSorter(t, map[string]float64{key: 1}),
			latencyStats:           make(map[string]*transportstats.Recorder),
			latencyRecorderFactory: func() *transportstats.Recorder { return transportstats.NewRecorderWithClock(clock.Now) },
		}
		lb.observeDelay("a", 400)
		clock.advance(4 * time.Minute)
		lb.observeDelay("a", 20)
		candidates := []Candidate{{Tag: "a", Latency: 20}}
		lb.scoreCandidates(candidates)
		return candidates[0].Score
	}
	require.InDelta(t, 20, scoreWithKey("latency_avg_1m"), 1e-9,
		"a one minute window must drop the four minute old sample")
	require.InDelta(t, 210, scoreWithKey("latency_avg_5m"), 1e-9,
		"a five minute window must average both samples")
}

type testClock struct {
	access sync.Mutex
	now    time.Time
}

func (c *testClock) Now() time.Time {
	c.access.Lock()
	defer c.access.Unlock()
	return c.now
}

func (c *testClock) advance(d time.Duration) {
	c.access.Lock()
	defer c.access.Unlock()
	c.now = c.now.Add(d)
}

// The probe goroutines write the history while the health check round reads it.
func TestLoadBalanceLatencyStatsConcurrent(t *testing.T) {
	t.Parallel()
	members := map[string]adapter.Outbound{}
	tags := []string{"a", "b", "c", "d"}
	for _, tag := range tags {
		members[tag] = &seedStub{tag: tag}
	}
	lb := &LoadBalance{
		sorter:           mustSorter(t, map[string]float64{"latency_avg_1m": 1, MetricLatency: 1}),
		latencyStats:     make(map[string]*transportstats.Recorder),
		primaryTags:      tags,
		primaryOutbounds: members,
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			random := rand.New(rand.NewSource(int64(seed)))
			for j := 0; j < 300; j++ {
				tag := tags[random.Intn(len(tags))]
				lb.observeDelay(tag, uint16(random.Intn(500)+1))
				if j%17 == 0 {
					lb.resetWindow(tag)
				}
			}
		}(i)
	}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 300; j++ {
				candidates := make([]Candidate, 0, len(tags))
				for _, tag := range tags {
					candidates = append(candidates, Candidate{Tag: tag, Latency: 50})
				}
				lb.scoreCandidates(candidates)
			}
		}()
	}
	wg.Wait()
}

// healthyCandidates filters unmeasured members, which is what keeps the default
// sorter equivalent to the old latency ranking: readLatest reports a zero delay
// as missing, and a missing latency would otherwise take the pool average.
func TestHealthyCandidatesNeverEmitsZeroLatency(t *testing.T) {
	t.Parallel()
	lb := newHealthFixture(t, map[string]uint16{"a": 0, "b": 120, "c": 0, "d": 30})
	got := lb.healthyCandidates(lb.primaryTags, lb.primaryOutbounds, true)
	require.Len(t, got, 2)
	for _, candidate := range got {
		require.NotZero(t, candidate.Latency)
		require.InDelta(t, float64(candidate.Latency), candidate.Score, 1e-9)
	}
	require.Equal(t, "d", got[0].Tag)
}

// The default sorter must order any pool exactly as ranking on delay alone did.
func TestDefaultSorterMatchesLatencyRankingProperty(t *testing.T) {
	t.Parallel()
	random := rand.New(rand.NewSource(20260920))
	for round := 0; round < 400; round++ {
		size := random.Intn(11) + 2
		delays := make(map[string]uint16, size)
		for i := 0; i < size; i++ {
			// Duplicates and extremes are deliberate: ties exercise the tag
			// tie-break, which is where the two orderings could diverge.
			delays[string(rune('a'+i))] = uint16(random.Intn(4)*random.Intn(65535) + 1)
		}
		lb := newHealthFixture(t, delays)
		got := lb.healthyCandidates(lb.primaryTags, lb.primaryOutbounds, true)

		want := make([]Candidate, len(got))
		copy(want, got)
		sortCandidatesByLatency(want)
		for i := range got {
			require.Equal(t, want[i].Tag, got[i].Tag, "round %d: score order must match latency order", round)
		}
	}
}

func mustSorter(t *testing.T, weights map[string]float64) *sorter {
	t.Helper()
	memberSorter, err := newSorter(weights)
	require.NoError(t, err)
	return memberSorter
}

// newHealthFixture builds a group whose members have the given stored delays,
// so that healthyCandidates can be driven end to end.
func newHealthFixture(t *testing.T, delays map[string]uint16) *LoadBalance {
	t.Helper()
	tags := make([]string, 0, len(delays))
	members := make(map[string]adapter.Outbound, len(delays))
	history := urltest.NewHistoryStorage()
	now := time.Now()
	for tag, delay := range delays {
		tags = append(tags, tag)
		members[tag] = &seedStub{tag: tag}
		history.StoreURLTestHistory(tag, &adapter.URLTestHistory{Time: now, Delay: delay})
	}
	sort.Strings(tags)
	return &LoadBalance{
		sorter:           defaultSorter(),
		history:          history,
		primaryTags:      tags,
		primaryOutbounds: members,
	}
}

func newLoadBalanceForTest(t *testing.T, options option.LoadBalanceOutboundOptions) (*LoadBalance, error) {
	t.Helper()
	if len(options.PrimaryOutbounds) == 0 {
		options.PrimaryOutbounds = []string{"a"}
	}
	created, err := NewLoadBalance(context.Background(), nil, log.NewNOPFactory().Logger(), "lb", options)
	if err != nil {
		return nil, err
	}
	return created.(*LoadBalance), nil
}

func TestNewLoadBalanceRejectsInvalidSorter(t *testing.T) {
	t.Parallel()
	for name, sorterOptions := range map[string]map[string]float64{
		"unknown key": {"latency_avg_2m": 1},
		"negative":    {MetricLatency: -1},
		"all zero":    {MetricLatency: 0},
	} {
		_, err := newLoadBalanceForTest(t, option.LoadBalanceOutboundOptions{Sorter: sorterOptions})
		require.Error(t, err, name)
	}
}

func TestNewLoadBalanceDefaultsToLatencySorter(t *testing.T) {
	t.Parallel()
	lb, err := newLoadBalanceForTest(t, option.LoadBalanceOutboundOptions{})
	require.NoError(t, err)
	require.True(t, lb.memberSorter().isDefault)
	require.Nil(t, lb.latencyStats, "a latency only group keeps no history")
	require.Empty(t, lb.memberSorter().directions)
}

func TestNewLoadBalanceAllocatesHistoryOnlyWhenNeeded(t *testing.T) {
	t.Parallel()
	windowed, err := newLoadBalanceForTest(t, option.LoadBalanceOutboundOptions{
		Sorter: map[string]float64{"latency_avg_1m": 1},
	})
	require.NoError(t, err)
	require.NotNil(t, windowed.latencyStats)

	plain, err := newLoadBalanceForTest(t, option.LoadBalanceOutboundOptions{
		Sorter: map[string]float64{MetricLatency: 1, "client_rtt": 1},
	})
	require.NoError(t, err)
	require.Nil(t, plain.latencyStats)
	require.Equal(t, []adapter.TransportStatsDirection{adapter.TransportStatsClient}, plain.memberSorter().directions)
}

func TestNewLoadBalanceTranslatesWeightedDelay(t *testing.T) {
	t.Parallel()
	lb, err := newLoadBalanceForTest(t, option.LoadBalanceOutboundOptions{
		WeightedDelay: &option.LoadBalanceWeightedDelayOptions{WindowWeight: 3, LastWeight: 1},
	})
	require.NoError(t, err)
	require.False(t, lb.memberSorter().isDefault)
	require.True(t, lb.memberSorter().usesLatencyWindow)
	require.NotNil(t, lb.latencyStats)

	weights := map[string]float64{}
	for _, key := range lb.memberSorter().keys {
		weights[key.name] = key.weight
	}
	require.InDelta(t, 0.25, weights[MetricLatency], 1e-9)
	require.InDelta(t, 0.75, weights["latency_avg_5m"], 1e-9)
}

func TestNewLoadBalanceRejectsSorterWithWeightedDelay(t *testing.T) {
	t.Parallel()
	_, err := newLoadBalanceForTest(t, option.LoadBalanceOutboundOptions{
		Sorter:        map[string]float64{MetricLatency: 1},
		WeightedDelay: &option.LoadBalanceWeightedDelayOptions{},
	})
	require.ErrorContains(t, err, "mutually exclusive")
}

// An interval more than half the window leaves at most two samples in it, so
// its average barely differs from the latest delay.
func TestNewLoadBalanceWarnsOnCoarseProbeInterval(t *testing.T) {
	t.Parallel()
	for name, testCase := range map[string]struct {
		interval  time.Duration
		key       string
		wantEvent bool
	}{
		"default interval with a one minute window":  {0, "latency_avg_1m", true},
		"default interval with a five minute window": {0, "latency_avg_5m", true},
		"tuned interval with a one minute window":    {20 * time.Second, "latency_avg_1m", false},
		"tuned interval with a five minute window":   {time.Minute, "latency_avg_5m", false},
		"coarse interval with a five minute window":  {4 * time.Minute, "latency_avg_5m", true},
		"latency only sorter":                        {0, MetricLatency, false},
	} {
		t.Run(name, func(t *testing.T) {
			logger, events := newCapturingLogger()
			options := option.LoadBalanceOutboundOptions{
				PrimaryOutbounds: []string{"a"},
				Interval:         badoption.Duration(testCase.interval),
				Sorter:           map[string]float64{testCase.key: 1},
			}
			_, err := NewLoadBalance(context.Background(), nil, logger, "lb", options)
			require.NoError(t, err)
			require.Equal(t, testCase.wantEvent, events.has("loadbalance.sorter.coarse_interval"))
		})
	}
}

func TestNewLoadBalanceWarnsOnWeightedDelayDeprecation(t *testing.T) {
	t.Parallel()
	logger, events := newCapturingLogger()
	_, err := NewLoadBalance(context.Background(), nil, logger, "lb", option.LoadBalanceOutboundOptions{
		PrimaryOutbounds: []string{"a"},
		WeightedDelay:    &option.LoadBalanceWeightedDelayOptions{},
	})
	require.NoError(t, err)
	require.True(t, events.has("loadbalance.weighted_delay.deprecated"))
}

// eventRecorder collects the structured event names a test triggered.
type eventRecorder struct {
	access sync.Mutex
	names  []string
}

func (r *eventRecorder) add(name string) {
	r.access.Lock()
	defer r.access.Unlock()
	r.names = append(r.names, name)
}

func (r *eventRecorder) has(name string) bool {
	r.access.Lock()
	defer r.access.Unlock()
	for _, seen := range r.names {
		if seen == name {
			return true
		}
	}
	return false
}

// capturingLogger records event names and discards everything else.
type capturingLogger struct {
	log.StructuredLogger
	events *eventRecorder
}

func (l *capturingLogger) WarnEvent(event string, _ string, _ ...log.Field) {
	l.events.add(event)
}

func (l *capturingLogger) DebugEvent(event string, _ string, _ ...log.Field) {
	l.events.add(event)
}

func newCapturingLogger() (log.StructuredLogger, *eventRecorder) {
	events := &eventRecorder{}
	return &capturingLogger{StructuredLogger: log.NewNOPFactory().Logger(), events: events}, events
}

func TestScoreCandidatesLogsBreakdownOnlyForConfiguredSorter(t *testing.T) {
	t.Parallel()
	logger, events := newCapturingLogger()
	lb := &LoadBalance{sorter: mustSorter(t, map[string]float64{MetricLatency: 1, "client_rtt": 1}), logger: logger}
	lb.scoreCandidates([]Candidate{{Tag: "a", Latency: 10}})
	require.True(t, events.has("loadbalance.score"))

	defaultLogger, defaultEvents := newCapturingLogger()
	plain := &LoadBalance{sorter: defaultSorter(), logger: defaultLogger}
	plain.scoreCandidates([]Candidate{{Tag: "a", Latency: 10}})
	require.False(t, defaultEvents.has("loadbalance.score"), "the default sorter has nothing to explain")
}

// The Clash API can rebuild the pool through PerformUpdateCheck while a health
// check round is rebuilding it, so rebuilds must be serialised.
func TestRebuildSnapshotConcurrent(t *testing.T) {
	t.Parallel()
	lb := newHealthFixture(t, map[string]uint16{"a": 40, "b": 90, "c": 20, "d": 60})
	lb.interruptGroup = interrupt.NewGroup()
	lb.topNPrimary = 2
	lb.tolerance = 10

	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				lb.rebuildSnapshot()
			}
		}()
	}
	wg.Wait()

	snapshot := lb.snapshot.Load()
	require.NotNil(t, snapshot)
	require.Len(t, snapshot.Candidates, 2)
	require.Equal(t, "c", snapshot.Candidates[0].Tag)
	require.Equal(t, "a", snapshot.Candidates[1].Tag)
	// A stable member set must settle on one generation rather than climbing
	// once per concurrent rebuild.
	require.EqualValues(t, 1, snapshot.Generation)
}

// With a sorter that cannot tell members apart, every score ties. A slow
// incumbent must still be displaced by a faster member beyond tolerance,
// otherwise the pool sticks to whoever was chosen first.
func TestTopNDisplacesSlowIncumbentOnTiedScores(t *testing.T) {
	t.Parallel()
	healthy := []Candidate{
		{Tag: "b", Latency: 100, Score: 0},
		{Tag: "a", Latency: 900, Score: 0},
	}
	previous := []Candidate{{Tag: "a"}}
	got := selectTopNWithTolerance(healthy, 1, 10, previous)
	require.Len(t, got, 1)
	require.Equal(t, "b", got[0].Tag)
}

func TestTopNKeepsIncumbentOnTiedScoresWithinTolerance(t *testing.T) {
	t.Parallel()
	healthy := []Candidate{
		{Tag: "b", Latency: 100, Score: 0},
		{Tag: "a", Latency: 108, Score: 0},
	}
	previous := []Candidate{{Tag: "a"}}
	got := selectTopNWithTolerance(healthy, 1, 10, previous)
	require.Equal(t, "a", got[0].Tag, "tolerance applies to the delay tie-break too")
}

// End to end: an unreportable sorter with top_n must follow latency changes.
func TestRebuildSnapshotUnreportableSorterFollowsLatency(t *testing.T) {
	t.Parallel()
	lb := newHealthFixture(t, map[string]uint16{"a": 50, "b": 100})
	lb.sorter = mustSorter(t, map[string]float64{"server_loss_rate_1m": 20})
	lb.interruptGroup = interrupt.NewGroup()
	lb.topNPrimary = 1
	lb.tolerance = 10
	lb.rebuildSnapshot()
	require.Equal(t, "a", lb.Now())

	lb.history.StoreURLTestHistory("a", &adapter.URLTestHistory{Time: time.Now(), Delay: 900})
	lb.rebuildSnapshot()
	require.Equal(t, "b", lb.Now(), "the degraded incumbent must be replaced")
}

// A group first reached through a path cut off by the depth limit must still be
// enabled when a shallower path reaches it.
func TestEnableTransportStatsRevisitsShallowerPath(t *testing.T) {
	t.Parallel()
	leaf := newStatsStub("leaf")
	target := &groupStub{seedStub: seedStub{tag: "target"}, members: []string{"leaf"}, current: "leaf"}
	members := map[string]adapter.Outbound{"leaf": leaf, "target": target}
	// A chain deep enough that "target" is reached at the depth limit first.
	var chainHead string
	previous := "target"
	for i := maxGroupDepth - 2; i >= 0; i-- {
		tag := "chain-" + string(rune('a'+i))
		members[tag] = &groupStub{seedStub: seedStub{tag: tag}, members: []string{previous}, current: previous}
		previous = tag
		chainHead = tag
	}
	lb := &LoadBalance{
		sorter:          mustSorter(t, map[string]float64{"client_rtt": 1}),
		logger:          log.NewNOPFactory().Logger(),
		outbound:        &stubManager{outbounds: members},
		backupOutbounds: map[string]adapter.Outbound{},
	}
	seen := make(map[string]int)
	lb.enableTransportStatsOn(members[chainHead], lb.memberSorter().directions, seen, 0)
	require.Empty(t, leaf.enabledDirections(), "the deep path is cut off before the leaf")
	lb.enableTransportStatsOn(target, lb.memberSorter().directions, seen, 0)
	require.Len(t, leaf.enabledDirections(), 1, "the shallow path must still reach the leaf")
}
