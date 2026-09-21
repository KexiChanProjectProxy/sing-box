package group

import (
	"math"
	"sort"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/transportstats"
	"github.com/sagernet/sing-box/option"

	"github.com/stretchr/testify/require"
)

// fakeStats serves fixed values for every window, so a test states only the
// metric it cares about. A metric left at its zero value reads as missing.
type fakeStats struct {
	lossRate     float64
	rtt          float64
	rttVar       float64
	deliveryRate float64
}

func (s *fakeStats) value(v float64) (float64, bool) {
	if v == 0 {
		return 0, false
	}
	return v, true
}

func (s *fakeStats) LossRate(time.Duration) (float64, bool)     { return s.value(s.lossRate) }
func (s *fakeStats) RTT(time.Duration) (float64, bool)          { return s.value(s.rtt) }
func (s *fakeStats) RTTVar(time.Duration) (float64, bool)       { return s.value(s.rttVar) }
func (s *fakeStats) DeliveryRate(time.Duration) (float64, bool) { return s.value(s.deliveryRate) }

func scoreWith(t *testing.T, weights map[string]float64, sources map[string]metricSource, tags ...string) []Candidate {
	t.Helper()
	memberSorter, err := newSorter(weights)
	require.NoError(t, err)
	candidates := make([]Candidate, 0, len(tags))
	for _, tag := range tags {
		candidates = append(candidates, Candidate{Tag: tag, Latency: sources[tag].latency})
	}
	memberSorter.score(candidates, func(candidate Candidate) metricSource {
		return sources[candidate.Tag]
	})
	return candidates
}

func TestDefaultSorterScoresRawLatency(t *testing.T) {
	t.Parallel()
	candidates := []Candidate{{Tag: "a", Latency: 120}, {Tag: "b", Latency: 30}}
	defaultSorter().score(candidates, func(candidate Candidate) metricSource {
		return metricSource{latency: candidate.Latency}
	})
	require.InDelta(t, 120, candidates[0].Score, 1e-9)
	require.InDelta(t, 30, candidates[1].Score, 1e-9)
}

// The implicit sorter must order members exactly as ranking on delay alone did.
func TestDefaultSorterOrdersLikeLatency(t *testing.T) {
	t.Parallel()
	delays := []uint16{700, 1, 65535, 42, 42, 300}
	candidates := make([]Candidate, 0, len(delays))
	for i, delay := range delays {
		candidates = append(candidates, Candidate{Tag: string(rune('a' + i)), Latency: delay})
	}
	defaultSorter().score(candidates, func(candidate Candidate) metricSource {
		return metricSource{latency: candidate.Latency}
	})
	byScore := append([]Candidate(nil), candidates...)
	sortCandidatesByScore(byScore)
	byLatency := append([]Candidate(nil), candidates...)
	sortCandidatesByLatency(byLatency)
	require.Equal(t, byLatency, byScore)
}

func TestSorterWeightsAreMillisecondEquivalent(t *testing.T) {
	t.Parallel()
	sources := map[string]metricSource{
		// 2% loss at 20ms per percent adds 40ms to a 50ms dial.
		"a": {latency: 50, client: &fakeStats{lossRate: 2}},
		"b": {latency: 80, client: &fakeStats{lossRate: 0.5}},
	}
	got := scoreWith(t, map[string]float64{MetricLatency: 1, "client_loss_rate_1m": 20}, sources, "a", "b")
	require.InDelta(t, 90, got[0].Score, 1e-9)
	require.InDelta(t, 90, got[1].Score, 1e-9)
}

func TestSorterHigherIsBetterIsNegated(t *testing.T) {
	t.Parallel()
	sources := map[string]metricSource{
		"fast": {latency: 100, client: &fakeStats{deliveryRate: 200}},
		"slow": {latency: 100, client: &fakeStats{deliveryRate: 20}},
	}
	got := scoreWith(t, map[string]float64{MetricLatency: 1, "client_delivery_rate": 0.1}, sources, "fast", "slow")
	require.InDelta(t, 100-20, got[0].Score, 1e-9)
	require.InDelta(t, 100-2, got[1].Score, 1e-9)
	sortCandidatesByScore(got)
	require.Equal(t, "fast", got[0].Tag, "more throughput must rank better")
}

func TestSorterMissingMetricUsesPoolAverage(t *testing.T) {
	t.Parallel()
	sources := map[string]metricSource{
		"a": {latency: 100, client: &fakeStats{lossRate: 1}},
		"b": {latency: 100, client: &fakeStats{lossRate: 3}},
		// "c" reports no loss rate at all, for example a protocol that cannot.
		"c": {latency: 100},
	}
	got := scoreWith(t, map[string]float64{MetricLatency: 1, "client_loss_rate_1m": 10}, sources, "a", "b", "c")
	require.InDelta(t, 110, got[0].Score, 1e-9)
	require.InDelta(t, 130, got[1].Score, 1e-9)
	// The pool average of 1 and 3 is 2, so "c" scores 100 + 2*10.
	require.InDelta(t, 120, got[2].Score, 1e-9)
}

func TestSorterMissingMetricForEveryMemberContributesNothing(t *testing.T) {
	t.Parallel()
	sources := map[string]metricSource{
		"a": {latency: 100},
		"b": {latency: 300},
	}
	got := scoreWith(t, map[string]float64{MetricLatency: 1, "server_loss_rate_1m": 50}, sources, "a", "b")
	require.InDelta(t, 100, got[0].Score, 1e-9)
	require.InDelta(t, 300, got[1].Score, 1e-9)
}

func TestSorterNilReaderIsMissing(t *testing.T) {
	t.Parallel()
	sources := map[string]metricSource{
		"a": {latency: 10, client: nil, server: nil},
	}
	got := scoreWith(t, map[string]float64{"client_rtt": 1, "server_rtt": 1, MetricLatency: 1}, sources, "a")
	require.InDelta(t, 10, got[0].Score, 1e-9)
}

func TestSorterUnmeasuredLatencyIsMissing(t *testing.T) {
	t.Parallel()
	// A candidate with no delay at all should not drag the score to zero and
	// win the pool; it takes the pool average instead.
	sources := map[string]metricSource{
		"measured":   {latency: 200},
		"unmeasured": {latency: 0},
	}
	got := scoreWith(t, map[string]float64{MetricLatency: 1}, sources, "measured", "unmeasured")
	require.InDelta(t, 200, got[0].Score, 1e-9)
	require.InDelta(t, 200, got[1].Score, 1e-9)
}

func TestSorterLatencyWindowFallsBackToLatest(t *testing.T) {
	t.Parallel()
	recorder := transportstats.NewRecorder()
	sources := map[string]metricSource{
		// A window with history.
		"a": {latency: 999, latencyWindow: recorder},
		// A window that exists but holds nothing yet.
		"b": {latency: 40, latencyWindow: transportstats.NewRecorder()},
		// No window at all.
		"c": {latency: 70},
	}
	recorder.AddLatency(10)
	recorder.AddLatency(30)
	got := scoreWith(t, map[string]float64{"latency_avg_1m": 1}, sources, "a", "b", "c")
	require.InDelta(t, 20, got[0].Score, 1e-9, "window average wins over the latest sample")
	require.InDelta(t, 40, got[1].Score, 1e-9, "an empty window falls back to the latest sample")
	require.InDelta(t, 70, got[2].Score, 1e-9, "no window falls back to the latest sample")
}

func TestSorterReadsEachDirectionSeparately(t *testing.T) {
	t.Parallel()
	sources := map[string]metricSource{
		"a": {
			latency: 0,
			client:  &fakeStats{rtt: 10, rttVar: 1, lossRate: 2, deliveryRate: 100},
			server:  &fakeStats{rtt: 50, rttVar: 9, lossRate: 8, deliveryRate: 5},
		},
	}
	for key, want := range map[string]float64{
		"client_rtt":           10,
		"client_rttvar":        1,
		"client_loss_rate_30s": 2,
		"client_loss_rate_1m":  2,
		"client_loss_rate_5m":  2,
		"client_delivery_rate": -100,
		"server_rtt":           50,
		"server_rttvar":        9,
		"server_loss_rate_30s": 8,
		"server_loss_rate_1m":  8,
		"server_loss_rate_5m":  8,
		"server_delivery_rate": -5,
	} {
		got := scoreWith(t, map[string]float64{key: 1}, sources, "a")
		require.InDelta(t, want, got[0].Score, 1e-9, key)
	}
}

func TestNewSorterRejectsBadInput(t *testing.T) {
	t.Parallel()
	for name, weights := range map[string]map[string]float64{
		"unknown key": {"latency_avg_2m": 1},
		"negative":    {MetricLatency: -1},
		"nan":         {MetricLatency: math.NaN()},
		"inf":         {MetricLatency: math.Inf(1)},
		"all zero":    {MetricLatency: 0, "client_rtt": 0},
		"single zero": {MetricLatency: 0},
	} {
		_, err := newSorter(weights)
		require.Error(t, err, name)
	}
}

func TestNewSorterDropsZeroWeights(t *testing.T) {
	t.Parallel()
	memberSorter, err := newSorter(map[string]float64{MetricLatency: 1, "client_rtt": 0})
	require.NoError(t, err)
	require.Len(t, memberSorter.keys, 1)
	require.Empty(t, memberSorter.directions, "a zero weighted key must not enable collection")
}

func TestNewSorterEmptyIsDefault(t *testing.T) {
	t.Parallel()
	memberSorter, err := newSorter(nil)
	require.NoError(t, err)
	require.True(t, memberSorter.isDefault)
	require.Empty(t, memberSorter.directions)
	require.False(t, memberSorter.usesLatencyWindow)
}

func TestNewSorterDetectsNeeds(t *testing.T) {
	t.Parallel()
	memberSorter, err := newSorter(map[string]float64{
		"latency_avg_1m":       1,
		"client_rtt":           1,
		"server_loss_rate_30s": 1,
	})
	require.NoError(t, err)
	require.True(t, memberSorter.usesLatencyWindow)
	require.Equal(t, []adapter.TransportStatsDirection{
		adapter.TransportStatsClient,
		adapter.TransportStatsServer,
	}, memberSorter.directions)
}

func TestNewSorterLatencyKeysNeedNoCollection(t *testing.T) {
	t.Parallel()
	memberSorter, err := newSorter(map[string]float64{MetricLatency: 1, "latency_avg_5m": 1})
	require.NoError(t, err)
	require.Empty(t, memberSorter.directions)
}

func TestSorterKeyOrderIsDeterministic(t *testing.T) {
	t.Parallel()
	// Go randomises map iteration, so a sorter built twice from the same
	// weights must still read its keys in the same order.
	weights := map[string]float64{"server_rtt": 1, MetricLatency: 2, "client_rtt": 3}
	want := []string{"client_rtt", MetricLatency, "server_rtt"}
	for i := 0; i < 50; i++ {
		memberSorter, err := newSorter(weights)
		require.NoError(t, err)
		names := make([]string, 0, len(memberSorter.keys))
		for _, key := range memberSorter.keys {
			names = append(names, key.name)
			require.Equal(t, weights[key.name], key.weight)
		}
		require.Equal(t, want, names)
	}
}

func TestSorterBreakdownExplainsScore(t *testing.T) {
	t.Parallel()
	memberSorter, err := newSorter(map[string]float64{MetricLatency: 1, "client_loss_rate_1m": 10})
	require.NoError(t, err)
	candidates := []Candidate{{Tag: "a", Latency: 50}, {Tag: "b", Latency: 50}}
	sources := map[string]metricSource{
		"a": {latency: 50, client: &fakeStats{lossRate: 4}},
		"b": {latency: 50},
	}
	breakdowns := memberSorter.score(candidates, func(candidate Candidate) metricSource {
		return sources[candidate.Tag]
	})
	require.Len(t, breakdowns, 2)
	require.Equal(t, "a", breakdowns[0].Tag)
	require.InDelta(t, candidates[0].Score, breakdowns[0].Score, 1e-9)

	// Independently computed: 50ms of delay, plus 4% loss at 10ms per percent.
	contributions := map[string]float64{}
	for _, item := range breakdowns[0].Contributions {
		contributions[item.Key] = item.Weighted
	}
	require.InDelta(t, 50, contributions[MetricLatency], 1e-9)
	require.InDelta(t, 40, contributions["client_loss_rate_1m"], 1e-9)
	require.InDelta(t, 90, breakdowns[0].Score, 1e-9)

	byKey := map[string]contribution{}
	for _, item := range breakdowns[1].Contributions {
		byKey[item.Key] = item
	}
	require.True(t, byKey[MetricLatency].Measured)
	require.False(t, byKey["client_loss_rate_1m"].Measured, "b's loss rate is the pool estimate")
	require.InDelta(t, 4, byKey["client_loss_rate_1m"].Value, 1e-9)
}

func TestSorterScoreEmptyPool(t *testing.T) {
	t.Parallel()
	require.Nil(t, defaultSorter().score(nil, func(Candidate) metricSource { return metricSource{} }))
}

// A reader that returns a non-finite value must not poison the ranking.
type brokenStats struct{}

func (brokenStats) LossRate(time.Duration) (float64, bool)     { return math.NaN(), true }
func (brokenStats) RTT(time.Duration) (float64, bool)          { return math.Inf(1), true }
func (brokenStats) RTTVar(time.Duration) (float64, bool)       { return math.NaN(), true }
func (brokenStats) DeliveryRate(time.Duration) (float64, bool) { return math.Inf(-1), true }

func TestSorterIgnoresNonFiniteReadings(t *testing.T) {
	t.Parallel()
	sources := map[string]metricSource{
		"a": {latency: 100, client: brokenStats{}},
		"b": {latency: 200, client: &fakeStats{rtt: 10}},
	}
	got := scoreWith(t, map[string]float64{MetricLatency: 1, "client_rtt": 1}, sources, "a", "b")
	require.InDelta(t, 110, got[0].Score, 1e-9, "the broken reading falls back to the pool average")
	require.InDelta(t, 210, got[1].Score, 1e-9)
}

// option validates sorter keys without importing the ranking table, so the two
// lists have to be kept in step.
func TestSorterKeysMatchOptionKeys(t *testing.T) {
	t.Parallel()
	fromOptions := append([]string(nil), option.LoadBalanceSorterKeys...)
	sort.Strings(fromOptions)
	require.Equal(t, fromOptions, MetricKeys())
}

func TestCheckMetricKey(t *testing.T) {
	t.Parallel()
	for _, key := range option.LoadBalanceSorterKeys {
		require.NoError(t, CheckMetricKey(key), key)
	}
	require.Error(t, CheckMetricKey("latency_avg_2m"))
	require.Error(t, CheckMetricKey(""))
}
