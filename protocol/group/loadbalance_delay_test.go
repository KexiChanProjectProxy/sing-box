package group

import (
	"testing"
	"time"

	"github.com/sagernet/sing-box/common/transportstats"
	"github.com/sagernet/sing-box/option"

	"github.com/stretchr/testify/require"
)

func newLatencyWindowBalance(t *testing.T) *LoadBalance {
	t.Helper()
	memberSorter, err := newSorter(map[string]float64{"latency_avg_1m": 1})
	require.NoError(t, err)
	return &LoadBalance{
		sorter:       memberSorter,
		latencyStats: make(map[string]*transportstats.Recorder),
	}
}

func TestObserveDelayRecordsLatency(t *testing.T) {
	t.Parallel()
	lb := newLatencyWindowBalance(t)
	lb.observeDelay("x", 100)
	lb.observeDelay("x", 300)

	average, ok := lb.latencyRecorderIfExists("x").Latency(time.Minute)
	require.True(t, ok)
	require.InDelta(t, 200, average, 1e-9)
}

func TestResetWindowDropsLatency(t *testing.T) {
	t.Parallel()
	lb := newLatencyWindowBalance(t)
	lb.observeDelay("x", 100)
	lb.resetWindow("x")

	_, ok := lb.latencyRecorderIfExists("x").Latency(time.Minute)
	require.False(t, ok)
}

func TestLatencyWindowDisabledWhenUnused(t *testing.T) {
	t.Parallel()
	// The default sorter reads only the latest delay, so no history is kept.
	lb := &LoadBalance{sorter: defaultSorter()}
	lb.observeDelay("x", 100)
	lb.resetWindow("x")
	require.Nil(t, lb.latencyRecorderIfExists("x"))
	require.Nil(t, lb.latencyRecorder("x"))
}

func TestLatencyRecorderCreatedOnDemand(t *testing.T) {
	t.Parallel()
	lb := newLatencyWindowBalance(t)
	require.Nil(t, lb.latencyRecorderIfExists("x"), "scoring must not allocate for unseen members")
	require.NotNil(t, lb.latencyRecorder("x"))
	require.NotNil(t, lb.latencyRecorderIfExists("x"))
}

// weighted_delay is deprecated but still supported, and must keep blending the
// window average with the latest sample in the same proportions.
func TestWeightedDelayTranslatesToSorter(t *testing.T) {
	t.Parallel()
	options := option.LoadBalanceOutboundOptions{
		PrimaryOutbounds: []string{"a"},
		WeightedDelay:    &option.LoadBalanceWeightedDelayOptions{WindowWeight: 7, LastWeight: 3},
	}
	require.NoError(t, options.Check())

	memberSorter, err := newSorter(options.SorterWeights())
	require.NoError(t, err)
	require.False(t, memberSorter.isDefault)
	require.True(t, memberSorter.usesLatencyWindow)
	require.Empty(t, memberSorter.directions, "weighted_delay never reads transport statistics")

	weights := map[string]float64{}
	for _, key := range memberSorter.keys {
		weights[key.name] = key.weight
	}
	require.InDelta(t, 0.3, weights[MetricLatency], 1e-9)
	require.InDelta(t, 0.7, weights["latency_avg_5m"], 1e-9)
}

func TestWeightedDelayScoreMatchesOldBlend(t *testing.T) {
	t.Parallel()
	options := option.LoadBalanceOutboundOptions{
		PrimaryOutbounds: []string{"a"},
		WeightedDelay:    &option.LoadBalanceWeightedDelayOptions{WindowWeight: 1, LastWeight: 1},
	}
	require.NoError(t, options.Check())
	memberSorter, err := newSorter(options.SorterWeights())
	require.NoError(t, err)

	lb := &LoadBalance{sorter: memberSorter, latencyStats: make(map[string]*transportstats.Recorder)}
	// Three samples averaging 70, with the latest at 10.
	lb.observeDelay("a", 100)
	lb.observeDelay("a", 100)
	lb.observeDelay("a", 10)

	candidates := []Candidate{{Tag: "a", Latency: 10}}
	lb.scoreCandidates(candidates)
	// (70 + 10) / 2 = 40, the value the old sample window produced.
	require.InDelta(t, 40, candidates[0].Score, 1e-9)
}

func TestWeightedDelayDefaultsApplied(t *testing.T) {
	t.Parallel()
	options := option.LoadBalanceOutboundOptions{
		PrimaryOutbounds: []string{"a"},
		WeightedDelay:    &option.LoadBalanceWeightedDelayOptions{},
	}
	require.NoError(t, options.Check())
	weights := options.SorterWeights()
	require.InDelta(t, 0.5, weights[MetricLatency], 1e-9)
	require.InDelta(t, 0.5, weights["latency_avg_5m"], 1e-9)
}

// SorterWeights is documented as callable after Check, but must not divide by
// zero if it is reached before the defaults are filled in.
func TestWeightedDelayWeightsWithoutCheck(t *testing.T) {
	t.Parallel()
	options := option.LoadBalanceOutboundOptions{
		WeightedDelay: &option.LoadBalanceWeightedDelayOptions{},
	}
	weights := options.SorterWeights()
	require.InDelta(t, 0.5, weights[MetricLatency], 1e-9)
	require.InDelta(t, 0.5, weights["latency_avg_5m"], 1e-9)
}
