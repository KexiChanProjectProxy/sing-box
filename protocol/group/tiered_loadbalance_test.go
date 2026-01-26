package group

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestLatencyTracker tests the latency tracker component
func TestLatencyTracker(t *testing.T) {
	t.Run("RecordLatency", func(t *testing.T) {
		tracker := NewLatencyTracker(3, 2, 10, 1)
		tracker.RegisterOutbound("test1", 1, 100*time.Millisecond)

		// Record successful connection within threshold
		tracker.RecordLatency("test1", 1, 50*time.Millisecond, true)
		assert.True(t, tracker.IsHealthyForTier("test1", 1))

		// Record connection exceeding threshold
		tracker.RecordLatency("test1", 1, 150*time.Millisecond, true)
		assert.True(t, tracker.IsHealthyForTier("test1", 1)) // Still healthy (1 failure)

		tracker.RecordLatency("test1", 1, 150*time.Millisecond, true)
		assert.True(t, tracker.IsHealthyForTier("test1", 1)) // Still healthy (2 failures)

		tracker.RecordLatency("test1", 1, 150*time.Millisecond, true)
		assert.False(t, tracker.IsHealthyForTier("test1", 1)) // Now unhealthy (3 failures)
	})

	t.Run("PerTierFailureTracking", func(t *testing.T) {
		tracker := NewLatencyTracker(3, 2, 10, 1)
		tracker.RegisterOutbound("test1", 1, 100*time.Millisecond)
		tracker.RegisterOutbound("test1", 2, 200*time.Millisecond)

		// Record latency that fails tier 1 but passes tier 2
		tracker.RecordLatency("test1", 1, 150*time.Millisecond, true)
		tracker.RecordLatency("test1", 1, 150*time.Millisecond, true)
		tracker.RecordLatency("test1", 1, 150*time.Millisecond, true)

		// Should be unhealthy for tier 1
		assert.False(t, tracker.IsHealthyForTier("test1", 1))

		// But still healthy for tier 2 (no recordings yet)
		assert.True(t, tracker.IsHealthyForTier("test1", 2))

		// Now record for tier 2
		tracker.RecordLatency("test1", 2, 150*time.Millisecond, true)
		assert.True(t, tracker.IsHealthyForTier("test1", 2)) // Within tier 2 threshold
	})

	t.Run("AverageLatency", func(t *testing.T) {
		tracker := NewLatencyTracker(3, 2, 5, 1) // History size = 5
		tracker.RegisterOutbound("test1", 1, 100*time.Millisecond)

		// Record 5 latencies
		tracker.RecordLatency("test1", 1, 10*time.Millisecond, true)
		tracker.RecordLatency("test1", 1, 20*time.Millisecond, true)
		tracker.RecordLatency("test1", 1, 30*time.Millisecond, true)
		tracker.RecordLatency("test1", 1, 40*time.Millisecond, true)
		tracker.RecordLatency("test1", 1, 50*time.Millisecond, true)

		avg := tracker.GetAverageLatency("test1")
		expected := (10 + 20 + 30 + 40 + 50) / 5
		assert.Equal(t, time.Duration(expected)*time.Millisecond, avg)

		// Add 6th latency (ring buffer wraps)
		tracker.RecordLatency("test1", 1, 60*time.Millisecond, true)
		avg = tracker.GetAverageLatency("test1")
		// Should be average of last 5: 20, 30, 40, 50, 60
		expected = (20 + 30 + 40 + 50 + 60) / 5
		assert.Equal(t, time.Duration(expected)*time.Millisecond, avg)
	})

	t.Run("SamplingRate", func(t *testing.T) {
		tracker := NewLatencyTracker(3, 2, 10, 3) // Sample 1 in 3

		count := 0
		for i := 0; i < 100; i++ {
			if tracker.ShouldSample() {
				count++
			}
		}

		// Should sample approximately 33 times (±10% tolerance)
		assert.InDelta(t, 33, count, 10)
	})

	t.Run("Recovery", func(t *testing.T) {
		tracker := NewLatencyTracker(3, 2, 10, 1)
		tracker.RegisterOutbound("test1", 1, 100*time.Millisecond)

		// Make unhealthy
		tracker.RecordLatency("test1", 1, 150*time.Millisecond, true)
		tracker.RecordLatency("test1", 1, 150*time.Millisecond, true)
		tracker.RecordLatency("test1", 1, 150*time.Millisecond, true)
		assert.False(t, tracker.IsHealthyForTier("test1", 1))

		// Start recovering - need failure counter to go below threshold
		tracker.RecordLatency("test1", 1, 50*time.Millisecond, true)
		// Failure counter resets to 0
		assert.True(t, tracker.IsHealthyForTier("test1", 1)) // Now healthy again
	})

	t.Run("UnknownOutbound", func(t *testing.T) {
		tracker := NewLatencyTracker(3, 2, 10, 1)

		// Unknown outbound should be healthy (optimistic)
		assert.True(t, tracker.IsHealthyForTier("unknown", 1))

		// GetAverageLatency for unknown should return 0
		assert.Equal(t, time.Duration(0), tracker.GetAverageLatency("unknown"))
	})

	t.Run("FailureConnection", func(t *testing.T) {
		tracker := NewLatencyTracker(2, 1, 10, 1)
		tracker.RegisterOutbound("test1", 1, 100*time.Millisecond)

		// Record failed connection (success=false)
		tracker.RecordLatency("test1", 1, 50*time.Millisecond, false)
		assert.True(t, tracker.IsHealthyForTier("test1", 1)) // Still healthy (1 failure)

		tracker.RecordLatency("test1", 1, 50*time.Millisecond, false)
		assert.False(t, tracker.IsHealthyForTier("test1", 1)) // Now unhealthy (2 failures)
	})

	t.Run("MultipleOutbounds", func(t *testing.T) {
		tracker := NewLatencyTracker(3, 2, 10, 1)
		tracker.RegisterOutbound("out1", 1, 100*time.Millisecond)
		tracker.RegisterOutbound("out2", 1, 100*time.Millisecond)
		tracker.RegisterOutbound("out3", 1, 100*time.Millisecond)

		// Record different latencies for each
		tracker.RecordLatency("out1", 1, 30*time.Millisecond, true)
		tracker.RecordLatency("out2", 1, 50*time.Millisecond, true)
		tracker.RecordLatency("out3", 1, 80*time.Millisecond, true)

		// Verify averages
		assert.Equal(t, 30*time.Millisecond, tracker.GetAverageLatency("out1"))
		assert.Equal(t, 50*time.Millisecond, tracker.GetAverageLatency("out2"))
		assert.Equal(t, 80*time.Millisecond, tracker.GetAverageLatency("out3"))

		// All should be healthy
		assert.True(t, tracker.IsHealthyForTier("out1", 1))
		assert.True(t, tracker.IsHealthyForTier("out2", 1))
		assert.True(t, tracker.IsHealthyForTier("out3", 1))
	})

	t.Run("TierStats", func(t *testing.T) {
		tracker := NewLatencyTracker(3, 2, 10, 1)
		tracker.RegisterOutbound("test1", 1, 100*time.Millisecond)

		// Record some failures
		tracker.RecordLatency("test1", 1, 150*time.Millisecond, true)
		tracker.RecordLatency("test1", 1, 150*time.Millisecond, true)

		// Check stats
		failures, successes, exists := tracker.GetTierStats("test1", 1)
		assert.True(t, exists)
		assert.Equal(t, uint32(2), failures)
		assert.Equal(t, uint32(0), successes)

		// Record success
		tracker.RecordLatency("test1", 1, 50*time.Millisecond, true)
		failures, successes, exists = tracker.GetTierStats("test1", 1)
		assert.True(t, exists)
		assert.Equal(t, uint32(0), failures) // Reset on success
		assert.Equal(t, uint32(1), successes)
	})

	t.Run("Reset", func(t *testing.T) {
		tracker := NewLatencyTracker(3, 2, 10, 1)
		tracker.RegisterOutbound("test1", 1, 100*time.Millisecond)

		// Record some data
		tracker.RecordLatency("test1", 1, 50*time.Millisecond, true)
		tracker.RecordLatency("test1", 1, 60*time.Millisecond, true)
		assert.Greater(t, tracker.GetAverageLatency("test1"), time.Duration(0))

		// Reset
		tracker.Reset()

		// Average should be 0 after reset
		assert.Equal(t, time.Duration(0), tracker.GetAverageLatency("test1"))
	})
}
