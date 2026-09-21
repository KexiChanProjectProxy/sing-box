package group

import (
	"reflect"
	"testing"
)

// cand builds a candidate the way the default sorter scores one, where the
// score is the raw delay in milliseconds.
func cand(tag string, latency uint16) Candidate {
	return Candidate{Tag: tag, Latency: latency, Score: float64(latency)}
}

func TestSelectTopNWithToleranceFirstSnapshot(t *testing.T) {
	t.Parallel()
	healthy := []Candidate{
		cand("a", 50),
		cand("b", 60),
		cand("c", 70),
	}
	got := selectTopNWithTolerance(healthy, 1, 10, nil)
	want := []Candidate{cand("a", 50)}
	if !sameCandidateTags(got, want) {
		t.Fatalf("first snapshot: got %v want %v", got, want)
	}
}

func TestSelectTopNWithToleranceKeepsIncumbentWithinBand(t *testing.T) {
	t.Parallel()
	healthy := []Candidate{
		cand("b", 60),
		cand("a", 70),
	}
	previous := []Candidate{cand("a", 65)}
	got := selectTopNWithTolerance(healthy, 1, 10, previous)
	if !sameCandidateTags(got, []Candidate{cand("a", 70)}) {
		t.Fatalf("incumbent within 10ms should stay, got %v", got)
	}
}

func TestSelectTopNWithToleranceEqualDeltaKeepsIncumbent(t *testing.T) {
	t.Parallel()
	healthy := []Candidate{
		cand("b", 60),
		cand("a", 70),
	}
	previous := []Candidate{cand("a", 70)}
	got := selectTopNWithTolerance(healthy, 1, 10, previous)
	if !sameCandidateTags(got, []Candidate{cand("a", 70)}) {
		t.Fatalf("delta == tolerance should keep incumbent, got %v", got)
	}
}

func TestSelectTopNWithToleranceSwitchesWhenFasterThanBand(t *testing.T) {
	t.Parallel()
	healthy := []Candidate{
		cand("b", 50),
		cand("a", 70),
	}
	previous := []Candidate{cand("a", 70)}
	got := selectTopNWithTolerance(healthy, 1, 10, previous)
	if !sameCandidateTags(got, []Candidate{cand("b", 50)}) {
		t.Fatalf("delta > tolerance should switch, got %v", got)
	}
}

func TestSelectTopNWithToleranceDropsUnhealthyIncumbent(t *testing.T) {
	t.Parallel()
	healthy := []Candidate{
		cand("b", 80),
	}
	previous := []Candidate{cand("a", 50)}
	got := selectTopNWithTolerance(healthy, 1, 10, previous)
	if !sameCandidateTags(got, []Candidate{cand("b", 80)}) {
		t.Fatalf("unhealthy incumbent should drop, got %v", got)
	}
}

func TestSelectTopNWithToleranceAllOrZeroReturnsAll(t *testing.T) {
	t.Parallel()
	healthy := []Candidate{
		cand("a", 50),
		cand("b", 60),
	}
	if got := selectTopNWithTolerance(healthy, 0, 10, nil); !reflect.DeepEqual(got, healthy) {
		t.Fatalf("n=0 should return all, got %v", got)
	}
	if got := selectTopNWithTolerance(healthy, 2, 10, nil); !reflect.DeepEqual(got, healthy) {
		t.Fatalf("n>=len should return all, got %v", got)
	}
	if got := selectTopNWithTolerance(nil, 1, 10, nil); got != nil {
		t.Fatalf("empty healthy should be nil, got %v", got)
	}
}

func TestSelectTopNWithToleranceTop2Boundary(t *testing.T) {
	t.Parallel()
	healthy := []Candidate{
		cand("a", 61),
		cand("c", 63),
		cand("b", 64),
	}
	previous := []Candidate{
		cand("a", 61),
		cand("b", 62),
	}
	got := selectTopNWithTolerance(healthy, 2, 10, previous)
	if !sameCandidateTags(got, []Candidate{cand("a", 61), cand("b", 64)}) {
		t.Fatalf("1ms boundary should keep incumbents, got %v", got)
	}
}

func TestSelectTopNWithToleranceTop2EvictsWorst(t *testing.T) {
	t.Parallel()
	healthy := []Candidate{
		cand("c", 50),
		cand("a", 61),
		cand("b", 80),
	}
	previous := []Candidate{
		cand("a", 61),
		cand("b", 80),
	}
	got := selectTopNWithTolerance(healthy, 2, 10, previous)
	if !sameCandidateTags(got, []Candidate{cand("c", 50), cand("a", 61)}) {
		t.Fatalf("newcomer faster than worst by >10ms should evict, got %v", got)
	}
}

func TestSelectTopNWithToleranceEvictsBothIncumbents(t *testing.T) {
	t.Parallel()
	healthy := []Candidate{
		cand("c", 50),
		cand("d", 51),
		cand("a", 100),
		cand("b", 100),
	}
	previous := []Candidate{
		cand("a", 100),
		cand("b", 100),
	}
	got := selectTopNWithTolerance(healthy, 2, 10, previous)
	if !sameCandidateTags(got, []Candidate{cand("c", 50), cand("d", 51)}) {
		t.Fatalf("two faster newcomers should replace both, got %v", got)
	}
}

// Scores near the top of the old uint16 delay range must still compare
// correctly; the tolerance arithmetic used to need widening casts to avoid
// wrapping there.
func TestSelectTopNWithToleranceLargeScoresDoNotWrap(t *testing.T) {
	t.Parallel()
	healthy := []Candidate{
		cand("b", 65530),
		cand("a", 65535),
	}
	previous := []Candidate{cand("a", 65535)}
	got := selectTopNWithTolerance(healthy, 1, 10, previous)
	if !sameCandidateTags(got, []Candidate{cand("a", 65535)}) {
		t.Fatalf("large scores must not wrap, got %v", got)
	}
}

// A sorter can score a member above the delay range it was derived from, for
// example when a loss rate weight dominates. Eviction must follow the score,
// not the raw delay.
func TestSelectTopNWithToleranceUsesScoreNotLatency(t *testing.T) {
	t.Parallel()
	// "a" is the faster dial but scores far worse once loss is weighed in.
	// Ranking on Latency would keep the incumbent "a" (50 > 200+10 is false);
	// ranking on Score evicts it (850 > 210+10).
	healthy := []Candidate{
		{Tag: "b", Latency: 200, Score: 210},
		{Tag: "a", Latency: 50, Score: 850},
	}
	previous := []Candidate{{Tag: "a", Latency: 50, Score: 850}}
	got := selectTopNWithTolerance(healthy, 1, 10, previous)
	if len(got) != 1 || got[0].Tag != "b" {
		t.Fatalf("eviction must follow score, got %v", got)
	}
}

func TestSelectTopNWithToleranceFractionalScores(t *testing.T) {
	t.Parallel()
	// Inside the band: 70.2 - 60.4 = 9.8 < 10, so the incumbent stays.
	healthy := []Candidate{
		{Tag: "b", Score: 60.4},
		{Tag: "a", Score: 70.2},
	}
	previous := []Candidate{{Tag: "a", Score: 70.2}}
	got := selectTopNWithTolerance(healthy, 1, 10, previous)
	if len(got) != 1 || got[0].Tag != "a" {
		t.Fatalf("fractional delta inside tolerance should keep incumbent, got %v", got)
	}

	// Outside the band by a fraction: 70.6 - 60.4 = 10.2 > 10, so it switches.
	// Truncating either score to an integer would keep the incumbent instead.
	healthy = []Candidate{
		{Tag: "b", Score: 60.4},
		{Tag: "a", Score: 70.6},
	}
	previous = []Candidate{{Tag: "a", Score: 70.6}}
	got = selectTopNWithTolerance(healthy, 1, 10, previous)
	if len(got) != 1 || got[0].Tag != "b" {
		t.Fatalf("fractional delta outside tolerance should switch, got %v", got)
	}
}

func sameCandidateTags(got, want []Candidate) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i].Tag != want[i].Tag || got[i].Latency != want[i].Latency {
			return false
		}
	}
	return true
}
