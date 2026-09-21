package group

import (
	"math"
	"sort"
	"testing"

	"github.com/sagernet/sing-box/option"
)

// FuzzSorterScore checks that scoring holds its invariants for any accepted
// configuration and any metric readings: it never panics and never produces a
// NaN score. Scores may legitimately overflow to ±Inf with extreme weights;
// those still order consistently and fall through to the delay tie-break.
func FuzzSorterScore(f *testing.F) {
	f.Add(0, 1.0, 2.0, uint16(50), uint16(120), 3.5, 7.25)
	f.Add(5, 0.0, 1e308, uint16(0), uint16(65535), 0.0, 1e308)
	f.Add(14, math.SmallestNonzeroFloat64, 1.0, uint16(1), uint16(1), -1.0, 0.0)

	keys := option.LoadBalanceSorterKeys

	f.Fuzz(func(t *testing.T, keyIndex int, firstWeight, secondWeight float64, firstDelay, secondDelay uint16, firstValue, secondValue float64) {
		if math.IsNaN(firstWeight) || math.IsInf(firstWeight, 0) ||
			math.IsNaN(secondWeight) || math.IsInf(secondWeight, 0) ||
			firstWeight < 0 || secondWeight < 0 {
			return
		}
		if keyIndex < 0 {
			keyIndex = -keyIndex
		}
		chosen := keys[keyIndex%len(keys)]
		// chosen may be "latency", in which case the map holds one key.
		weights := map[string]float64{MetricLatency: firstWeight, chosen: secondWeight}
		var anyPositive bool
		for _, weight := range weights {
			if weight > 0 {
				anyPositive = true
			}
		}
		memberSorter, err := newSorter(weights)
		if err != nil {
			// The only configuration newSorter rejects here is an all zero set.
			if anyPositive {
				t.Fatalf("newSorter(%v) unexpectedly failed: %v", weights, err)
			}
			return
		}
		if !anyPositive {
			t.Fatalf("newSorter(%v) accepted an all zero weight set", weights)
		}

		sources := map[string]metricSource{
			"a": {latency: firstDelay, client: &fakeStats{
				lossRate: firstValue, rtt: firstValue, rttVar: firstValue, deliveryRate: firstValue,
			}},
			"b": {latency: secondDelay, client: &fakeStats{
				lossRate: secondValue, rtt: secondValue, rttVar: secondValue, deliveryRate: secondValue,
			}},
		}
		candidates := []Candidate{{Tag: "a", Latency: firstDelay}, {Tag: "b", Latency: secondDelay}}
		breakdowns := memberSorter.score(candidates, func(candidate Candidate) metricSource {
			return sources[candidate.Tag]
		})

		for i, candidate := range candidates {
			if math.IsNaN(candidate.Score) {
				t.Fatalf("candidate %s scored NaN", candidate.Tag)
			}
			if breakdowns[i].Tag != candidate.Tag {
				t.Fatalf("breakdown %d is for %s, want %s", i, breakdowns[i].Tag, candidate.Tag)
			}
		}

		// Sorting must be a strict weak ordering, which sort.Slice relies on.
		sortCandidatesByScore(candidates)
		if !sort.SliceIsSorted(candidates, func(i, j int) bool {
			return rankAfter(candidates[j], candidates[i])
		}) {
			t.Fatalf("scored candidates did not sort: %+v", candidates)
		}

		// Tolerance arithmetic must stay finite whatever the scores are.
		selected := selectTopNWithTolerance(candidates, 1, 10, nil)
		if len(selected) != 1 {
			t.Fatalf("top 1 of two candidates returned %d", len(selected))
		}
	})
}

// FuzzSorterWeightsRoundTrip checks that anything option validation accepts,
// the ranking implementation also accepts, and the reverse.
func FuzzSorterWeightsRoundTrip(f *testing.F) {
	f.Add("latency", 1.0)
	f.Add("client_rtt", 0.0)
	f.Add("not_a_key", 1.0)
	f.Add("server_delivery_rate", math.NaN())

	f.Fuzz(func(t *testing.T, key string, weight float64) {
		options := option.LoadBalanceOutboundOptions{
			PrimaryOutbounds: []string{"a"},
			Sorter:           map[string]float64{key: weight},
		}
		optionErr := options.Check()
		_, sorterErr := newSorter(options.SorterWeights())
		if (optionErr == nil) != (sorterErr == nil) {
			t.Fatalf("validation disagrees for %q=%v: option=%v sorter=%v", key, weight, optionErr, sorterErr)
		}
	})
}
