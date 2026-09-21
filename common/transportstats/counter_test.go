package transportstats

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCounterTrackerFirstSampleIsBaseline(t *testing.T) {
	t.Parallel()
	tracker := NewCounterTracker()
	require.Equal(t, Counters{}, tracker.Delta(1, Counters{Sent: 1000, Lost: 10}))
}

func TestCounterTrackerReportsIncrements(t *testing.T) {
	t.Parallel()
	tracker := NewCounterTracker()
	tracker.Delta(1, Counters{Sent: 1000, Lost: 10})
	require.Equal(t, Counters{Sent: 500, Lost: 5}, tracker.Delta(1, Counters{Sent: 1500, Lost: 15}))
	require.Equal(t, Counters{Sent: 100, Lost: 0}, tracker.Delta(1, Counters{Sent: 1600, Lost: 15}))
}

func TestCounterTrackerKeysAreIndependent(t *testing.T) {
	t.Parallel()
	tracker := NewCounterTracker()
	tracker.Delta(1, Counters{Sent: 100})
	tracker.Delta(2, Counters{Sent: 5000})
	require.Equal(t, Counters{Sent: 50}, tracker.Delta(1, Counters{Sent: 150}))
	require.Equal(t, Counters{Sent: 10}, tracker.Delta(2, Counters{Sent: 5010}))
}

func TestCounterTrackerRebaselinesOnReset(t *testing.T) {
	t.Parallel()
	tracker := NewCounterTracker()
	tracker.Delta(1, Counters{Sent: 1000, Lost: 10})
	// The connection behind the key was replaced, so counters restarted.
	require.Equal(t, Counters{}, tracker.Delta(1, Counters{Sent: 20, Lost: 0}))
	require.Equal(t, Counters{Sent: 30}, tracker.Delta(1, Counters{Sent: 50, Lost: 0}))
}

func TestCounterTrackerRebaselinesWhenOnlyLossResets(t *testing.T) {
	t.Parallel()
	tracker := NewCounterTracker()
	tracker.Delta(1, Counters{Sent: 1000, Lost: 10})
	require.Equal(t, Counters{}, tracker.Delta(1, Counters{Sent: 1100, Lost: 2}))
}

func TestCounterTrackerForget(t *testing.T) {
	t.Parallel()
	tracker := NewCounterTracker()
	tracker.Delta(1, Counters{Sent: 1000})
	tracker.Forget(1)
	require.Equal(t, Counters{}, tracker.Delta(1, Counters{Sent: 1000}), "forgotten key restarts as a baseline")
}

func TestCounterTrackerRetain(t *testing.T) {
	t.Parallel()
	tracker := NewCounterTracker()
	tracker.Delta(1, Counters{Sent: 100})
	tracker.Delta(2, Counters{Sent: 100})
	tracker.Retain(func(key uint64) bool { return key == 1 })
	require.Equal(t, Counters{Sent: 10}, tracker.Delta(1, Counters{Sent: 110}))
	require.Equal(t, Counters{}, tracker.Delta(2, Counters{Sent: 110}))
}

func TestCounterTrackerEvictsStaleKeys(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	tracker := newCounterTrackerWithClock(clock.Now)
	tracker.Delta(1, Counters{Sent: 100})
	clock.Advance(staleCounterTimeout + BucketDuration)
	// Adding a new key sweeps the abandoned one.
	tracker.Delta(2, Counters{Sent: 100})
	tracker.access.Lock()
	_, stillThere := tracker.states[1]
	tracker.access.Unlock()
	require.False(t, stillThere)
}

func TestCounterTrackerDoesNotEvictLiveKeys(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	tracker := newCounterTrackerWithClock(clock.Now)
	tracker.Delta(1, Counters{Sent: 100})
	for i := 0; i < 10; i++ {
		clock.Advance(BucketDuration)
		tracker.Delta(1, Counters{Sent: uint64(100 + i)})
	}
	clock.Advance(staleCounterTimeout - BucketDuration)
	tracker.Delta(2, Counters{Sent: 1})
	tracker.access.Lock()
	_, stillThere := tracker.states[1]
	tracker.access.Unlock()
	require.True(t, stillThere)
}

func TestCounterTrackerConcurrentAccess(t *testing.T) {
	t.Parallel()
	tracker := NewCounterTracker()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < 300; j++ {
				key := uint64((worker + j) % 16)
				tracker.Delta(key, Counters{Sent: uint64(j), Lost: uint64(j / 10)})
				switch j % 31 {
				case 0:
					tracker.Forget(key)
				case 7:
					tracker.Retain(func(k uint64) bool { return k%2 == 0 })
				}
			}
		}(i)
	}
	wg.Wait()
}

// Sweeping only when a new key appears would never reclaim a dead connection in
// a deployment whose key set is stable.
func TestCounterTrackerSweepsWithoutNewKeys(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	tracker := newCounterTrackerWithClock(clock.Now)
	tracker.Delta(1, Counters{Sent: 100})
	tracker.Delta(2, Counters{Sent: 100})
	// Key 2 keeps reporting; key 1's connection is gone.
	for i := 0; i < 40; i++ {
		clock.Advance(BucketDuration)
		tracker.Delta(2, Counters{Sent: uint64(100 + i)})
	}
	tracker.access.Lock()
	_, stillThere := tracker.states[1]
	_, live := tracker.states[2]
	tracker.access.Unlock()
	require.False(t, stillThere, "an abandoned key must be reclaimed")
	require.True(t, live)
}
