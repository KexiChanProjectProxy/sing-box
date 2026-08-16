package group

import (
	"reflect"
	"testing"
)

func TestSelectTopNWithToleranceFirstSnapshot(t *testing.T) {
	t.Parallel()
	healthy := []Candidate{
		{Tag: "a", Latency: 50},
		{Tag: "b", Latency: 60},
		{Tag: "c", Latency: 70},
	}
	got := selectTopNWithTolerance(healthy, 1, 10, nil)
	want := []Candidate{{Tag: "a", Latency: 50}}
	if !sameCandidateTags(got, want) {
		t.Fatalf("first snapshot: got %v want %v", got, want)
	}
}

func TestSelectTopNWithToleranceKeepsIncumbentWithinBand(t *testing.T) {
	t.Parallel()
	healthy := []Candidate{
		{Tag: "b", Latency: 60},
		{Tag: "a", Latency: 70},
	}
	previous := []Candidate{{Tag: "a", Latency: 65}}
	got := selectTopNWithTolerance(healthy, 1, 10, previous)
	if !sameCandidateTags(got, []Candidate{{Tag: "a", Latency: 70}}) {
		t.Fatalf("incumbent within 10ms should stay, got %v", got)
	}
}

func TestSelectTopNWithToleranceEqualDeltaKeepsIncumbent(t *testing.T) {
	t.Parallel()
	healthy := []Candidate{
		{Tag: "b", Latency: 60},
		{Tag: "a", Latency: 70},
	}
	previous := []Candidate{{Tag: "a", Latency: 70}}
	got := selectTopNWithTolerance(healthy, 1, 10, previous)
	if !sameCandidateTags(got, []Candidate{{Tag: "a", Latency: 70}}) {
		t.Fatalf("delta == tolerance should keep incumbent, got %v", got)
	}
}

func TestSelectTopNWithToleranceSwitchesWhenFasterThanBand(t *testing.T) {
	t.Parallel()
	healthy := []Candidate{
		{Tag: "b", Latency: 50},
		{Tag: "a", Latency: 70},
	}
	previous := []Candidate{{Tag: "a", Latency: 70}}
	got := selectTopNWithTolerance(healthy, 1, 10, previous)
	if !sameCandidateTags(got, []Candidate{{Tag: "b", Latency: 50}}) {
		t.Fatalf("delta > tolerance should switch, got %v", got)
	}
}

func TestSelectTopNWithToleranceDropsUnhealthyIncumbent(t *testing.T) {
	t.Parallel()
	healthy := []Candidate{
		{Tag: "b", Latency: 80},
	}
	previous := []Candidate{{Tag: "a", Latency: 50}}
	got := selectTopNWithTolerance(healthy, 1, 10, previous)
	if !sameCandidateTags(got, []Candidate{{Tag: "b", Latency: 80}}) {
		t.Fatalf("unhealthy incumbent should drop, got %v", got)
	}
}

func TestSelectTopNWithToleranceAllOrZeroReturnsAll(t *testing.T) {
	t.Parallel()
	healthy := []Candidate{
		{Tag: "a", Latency: 50},
		{Tag: "b", Latency: 60},
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
		{Tag: "a", Latency: 61},
		{Tag: "c", Latency: 63},
		{Tag: "b", Latency: 64},
	}
	previous := []Candidate{
		{Tag: "a", Latency: 61},
		{Tag: "b", Latency: 62},
	}
	got := selectTopNWithTolerance(healthy, 2, 10, previous)
	if !sameCandidateTags(got, []Candidate{{Tag: "a", Latency: 61}, {Tag: "b", Latency: 64}}) {
		t.Fatalf("1ms boundary should keep incumbents, got %v", got)
	}
}

func TestSelectTopNWithToleranceTop2EvictsWorst(t *testing.T) {
	t.Parallel()
	healthy := []Candidate{
		{Tag: "c", Latency: 50},
		{Tag: "a", Latency: 61},
		{Tag: "b", Latency: 80},
	}
	previous := []Candidate{
		{Tag: "a", Latency: 61},
		{Tag: "b", Latency: 80},
	}
	got := selectTopNWithTolerance(healthy, 2, 10, previous)
	if !sameCandidateTags(got, []Candidate{{Tag: "c", Latency: 50}, {Tag: "a", Latency: 61}}) {
		t.Fatalf("newcomer faster than worst by >10ms should evict, got %v", got)
	}
}

func TestSelectTopNWithToleranceEvictsBothIncumbents(t *testing.T) {
	t.Parallel()
	healthy := []Candidate{
		{Tag: "c", Latency: 50},
		{Tag: "d", Latency: 51},
		{Tag: "a", Latency: 100},
		{Tag: "b", Latency: 100},
	}
	previous := []Candidate{
		{Tag: "a", Latency: 100},
		{Tag: "b", Latency: 100},
	}
	got := selectTopNWithTolerance(healthy, 2, 10, previous)
	if !sameCandidateTags(got, []Candidate{{Tag: "c", Latency: 50}, {Tag: "d", Latency: 51}}) {
		t.Fatalf("two faster newcomers should replace both, got %v", got)
	}
}

func TestSelectTopNWithToleranceUint16Overflow(t *testing.T) {
	t.Parallel()
	healthy := []Candidate{
		{Tag: "b", Latency: 65530},
		{Tag: "a", Latency: 65535},
	}
	previous := []Candidate{{Tag: "a", Latency: 65535}}
	got := selectTopNWithTolerance(healthy, 1, 10, previous)
	if !sameCandidateTags(got, []Candidate{{Tag: "a", Latency: 65535}}) {
		t.Fatalf("uint16 add must not wrap, got %v", got)
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
