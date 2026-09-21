package transportstats

import (
	"sync"
	"time"
)

// Counters is a set of cumulative counters as reported by a transport.
type Counters struct {
	Sent  uint64
	Lost  uint64
	Bytes uint64
}

// counterState is the last sample seen for one source key.
type counterState struct {
	last    Counters
	updated time.Time
}

// staleCounterTimeout is how long a source key survives without a new sample.
// Sources sample every BucketDuration, so a key this old belongs to a
// connection that died without being forgotten.
const staleCounterTimeout = MaxWindow

// CounterTracker converts the cumulative counters of individual connections
// into per-outbound increments. Every connection or session reports its own
// counters starting from zero, so each one is tracked under its own key and the
// increments are summed by the caller into a single Recorder.
//
// It is safe for concurrent use.
type CounterTracker struct {
	access    sync.Mutex
	now       func() time.Time
	states    map[uint64]*counterState
	lastSweep time.Time
}

// NewCounterTracker returns an empty CounterTracker using the wall clock.
func NewCounterTracker() *CounterTracker {
	return newCounterTrackerWithClock(time.Now)
}

func newCounterTrackerWithClock(now func() time.Time) *CounterTracker {
	return &CounterTracker{
		now:       now,
		states:    make(map[uint64]*counterState),
		lastSweep: now(),
	}
}

// Delta records a cumulative sample for key and returns the increment since the
// previous sample. The first sample of a key only establishes the baseline and
// reports a zero increment, as does a sample whose counters moved backwards,
// which means the underlying connection was replaced under the same key.
func (t *CounterTracker) Delta(key uint64, sample Counters) Counters {
	t.access.Lock()
	defer t.access.Unlock()
	now := t.now()
	t.evictStaleLocked(now)
	state, loaded := t.states[key]
	if !loaded {
		t.states[key] = &counterState{last: sample, updated: now}
		return Counters{}
	}
	state.updated = now
	if sample.Sent < state.last.Sent || sample.Lost < state.last.Lost || sample.Bytes < state.last.Bytes {
		state.last = sample
		return Counters{}
	}
	delta := Counters{
		Sent:  sample.Sent - state.last.Sent,
		Lost:  sample.Lost - state.last.Lost,
		Bytes: sample.Bytes - state.last.Bytes,
	}
	state.last = sample
	return delta
}

// Forget drops the baseline of key, which is what a caller does when the
// connection or session behind it closes.
func (t *CounterTracker) Forget(key uint64) {
	t.access.Lock()
	defer t.access.Unlock()
	delete(t.states, key)
}

// Retain drops every key that is not live, for callers that sample a full set
// of connections at once.
func (t *CounterTracker) Retain(live func(key uint64) bool) {
	t.access.Lock()
	defer t.access.Unlock()
	for key := range t.states {
		if !live(key) {
			delete(t.states, key)
		}
	}
}

// evictStaleLocked drops keys whose connection went away without a Forget.
//
// It sweeps at most once per bucket rather than on every sample, so that a
// caller tracking many connections does not walk the whole map on each one, and
// so that a caller with a stable key set still sweeps. The caller must hold
// access.
func (t *CounterTracker) evictStaleLocked(now time.Time) {
	if now.Before(t.lastSweep) {
		// A caller supplied clock moved backwards; re-anchor rather than
		// suppressing every sweep until it catches up.
		t.lastSweep = now
		return
	}
	if now.Sub(t.lastSweep) < BucketDuration {
		return
	}
	t.lastSweep = now
	for key, state := range t.states {
		if now.Sub(state.updated) > staleCounterTimeout {
			delete(t.states, key)
		}
	}
}
