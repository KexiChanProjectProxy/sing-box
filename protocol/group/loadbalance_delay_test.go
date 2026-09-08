package group

import "testing"

func TestComputeWeightedDelayEqualWeights(t *testing.T) {
	t.Parallel()
	got := computeWeightedDelay([]uint16{10, 20, 30}, 1, 1)
	if got != 25 {
		t.Fatalf("got %d want 25", got)
	}
}

func TestComputeWeightedDelayUnequalWeights(t *testing.T) {
	t.Parallel()
	got := computeWeightedDelay([]uint16{10, 20, 30}, 7, 3)
	if got != 23 {
		t.Fatalf("got %d want 23", got)
	}
}

func TestObserveDelayWindowOfTwo(t *testing.T) {
	t.Parallel()
	lb := &LoadBalance{
		delayWindow:  2,
		windowWeight: 1,
		lastWeight:   1,
		windows:      make(map[string]*delayWindow),
	}
	lb.observeDelay("x", 10)
	lb.observeDelay("x", 20)
	got := lb.observeDelay("x", 30)
	window := lb.windows["x"]
	if window == nil {
		t.Fatal("missing window")
	}
	if len(window.samples) != 2 || window.samples[0] != 20 || window.samples[1] != 30 {
		t.Fatalf("samples %v want [20 30]", window.samples)
	}
	if got != 27 {
		t.Fatalf("weighted %d want 27", got)
	}
}

func TestResetWindowThenObserve(t *testing.T) {
	t.Parallel()
	lb := &LoadBalance{
		delayWindow:  5,
		windowWeight: 1,
		lastWeight:   1,
		windows:      make(map[string]*delayWindow),
	}
	lb.observeDelay("x", 10)
	lb.observeDelay("x", 20)
	lb.resetWindow("x")
	got := lb.observeDelay("x", 40)
	if got != 40 {
		t.Fatalf("got %d want 40", got)
	}
}

func TestObserveDelayDisabled(t *testing.T) {
	t.Parallel()
	lb := &LoadBalance{}
	got := lb.observeDelay("x", 15)
	if got != 15 {
		t.Fatalf("got %d want 15", got)
	}
	if lb.windows != nil {
		t.Fatal("disabled observe should not allocate windows")
	}
}
