package transportstats

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeClock struct {
	access sync.Mutex
	now    time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Unix(1700000000, 0)}
}

func (c *fakeClock) Now() time.Time {
	c.access.Lock()
	defer c.access.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.access.Lock()
	defer c.access.Unlock()
	c.now = c.now.Add(d)
}

func TestRecorderEmpty(t *testing.T) {
	t.Parallel()
	r := NewRecorder()
	for _, read := range []func(time.Duration) (float64, bool){
		r.LossRate, r.RTT, r.RTTVar, r.DeliveryRate, r.Latency,
	} {
		value, ok := read(time.Minute)
		require.False(t, ok)
		require.Zero(t, value)
	}
}

func TestRecorderLossRate(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	r := newRecorderWithClock(clock.Now)
	// 100 packets on the wire, 10 of which were lost.
	r.AddCounters(100, 10)
	rate, ok := r.LossRate(time.Minute)
	require.True(t, ok)
	require.InDelta(t, 10, rate, 1e-9)
}

func TestRecorderLossRateSumsAcrossBuckets(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	r := newRecorderWithClock(clock.Now)
	for i := 0; i < 6; i++ {
		r.AddCounters(100, 5)
		clock.Advance(BucketDuration)
	}
	// The six samples landed in six distinct buckets; the clock has moved to a
	// seventh, so a one minute window still covers the last six.
	rate, ok := r.LossRate(time.Minute)
	require.True(t, ok)
	require.InDelta(t, 5, rate, 1e-9)
}

func TestRecorderWindowNarrowerThanHistory(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	r := newRecorderWithClock(clock.Now)
	// An old bucket with total loss, then two recent clean ones.
	r.AddCounters(100, 100)
	clock.Advance(time.Minute)
	r.AddCounters(100, 0)
	clock.Advance(BucketDuration)
	r.AddCounters(100, 0)

	rate, ok := r.LossRate(30 * time.Second)
	require.True(t, ok)
	require.InDelta(t, 0, rate, 1e-9, "30s window must not see the old bucket")

	rate, ok = r.LossRate(5 * time.Minute)
	require.True(t, ok)
	require.InDelta(t, 33.333333, rate, 1e-4, "5m window sees all three buckets")
}

func TestRecorderBucketsExpire(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	r := newRecorderWithClock(clock.Now)
	r.AddCounters(100, 100)
	clock.Advance(MaxWindow + BucketDuration)
	_, ok := r.LossRate(MaxWindow)
	require.False(t, ok, "samples older than the ring must not be readable")
}

func TestRecorderBucketRecycled(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	r := newRecorderWithClock(clock.Now)
	r.AddCounters(100, 100)
	// Advance exactly one full ring so the same slot is reused.
	clock.Advance(MaxWindow)
	r.AddCounters(100, 0)
	rate, ok := r.LossRate(MaxWindow)
	require.True(t, ok)
	require.InDelta(t, 0, rate, 1e-9, "recycled slot must not carry the old sample")
}

func TestRecorderRTTMean(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	r := newRecorderWithClock(clock.Now)
	r.AddRTT(100, 10)
	clock.Advance(BucketDuration)
	r.AddRTT(200, 30)

	rtt, ok := r.RTT(time.Minute)
	require.True(t, ok)
	require.InDelta(t, 150, rtt, 1e-9)

	rttVar, ok := r.RTTVar(time.Minute)
	require.True(t, ok)
	require.InDelta(t, 20, rttVar, 1e-9)
}

func TestRecorderRTTPartialSample(t *testing.T) {
	t.Parallel()
	r := NewRecorder()
	r.AddRTT(50, 0)
	rtt, ok := r.RTT(time.Minute)
	require.True(t, ok)
	require.InDelta(t, 50, rtt, 1e-9)
	_, ok = r.RTTVar(time.Minute)
	require.False(t, ok, "a zero variance sample must not be counted")
}

func TestRecorderDeliveryRatePeak(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	r := newRecorderWithClock(clock.Now)
	r.AddDeliveryRate(10)
	r.AddDeliveryRate(50)
	r.AddDeliveryRate(20)
	clock.Advance(BucketDuration)
	r.AddDeliveryRate(30)

	peak, ok := r.DeliveryRate(time.Minute)
	require.True(t, ok)
	require.InDelta(t, 50, peak, 1e-9)
}

func TestRecorderLatencyAverageAndReset(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	r := newRecorderWithClock(clock.Now)
	r.AddLatency(100)
	clock.Advance(BucketDuration)
	r.AddLatency(300)

	average, ok := r.Latency(time.Minute)
	require.True(t, ok)
	require.InDelta(t, 200, average, 1e-9)

	r.ResetLatency()
	_, ok = r.Latency(time.Minute)
	require.False(t, ok)
}

func TestRecorderResetLatencyKeepsTransportSamples(t *testing.T) {
	t.Parallel()
	r := NewRecorder()
	r.AddLatency(100)
	r.AddCounters(90, 10)
	r.AddRTT(40, 5)
	r.ResetLatency()

	_, ok := r.Latency(time.Minute)
	require.False(t, ok)
	_, ok = r.LossRate(time.Minute)
	require.True(t, ok)
	_, ok = r.RTT(time.Minute)
	require.True(t, ok)
}

func TestRecorderIgnoresNonPositiveSamples(t *testing.T) {
	t.Parallel()
	r := NewRecorder()
	r.AddLatency(0)
	r.AddDeliveryRate(0)
	r.AddDeliveryRate(-1)
	r.AddRTT(0, 0)
	r.AddCounters(0, 0)

	for _, read := range []func(time.Duration) (float64, bool){
		r.LossRate, r.RTT, r.RTTVar, r.DeliveryRate, r.Latency,
	} {
		_, ok := read(time.Minute)
		require.False(t, ok)
	}
}

func TestRecorderWindowClampedToRing(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	r := newRecorderWithClock(clock.Now)
	r.AddCounters(100, 50)
	rate, ok := r.LossRate(365 * 24 * time.Hour)
	require.True(t, ok)
	require.InDelta(t, 50, rate, 1e-9)
}

func TestRecorderConcurrentAccess(t *testing.T) {
	t.Parallel()
	r := NewRecorder()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				r.AddCounters(100, 10)
				r.AddRTT(20, 2)
				r.AddDeliveryRate(5)
				r.AddLatency(30)
				r.LossRate(time.Minute)
				r.RTT(time.Minute)
				r.DeliveryRate(time.Minute)
				r.Latency(time.Minute)
			}
		}()
	}
	wg.Wait()
	// Read the whole ring: a loaded machine could otherwise push the earliest
	// samples out of a one minute window and change the ratio.
	rate, ok := r.LossRate(MaxWindow)
	require.True(t, ok)
	require.InDelta(t, 10, rate, 1e-9)
}

// Counter sources report packets sent including those later reported lost, so
// a loss of every packet is 100% rather than 50%.
func TestRecorderLossRateContractSentIncludesLost(t *testing.T) {
	t.Parallel()
	r := NewRecorder()
	r.AddCounters(100, 100)
	rate, ok := r.LossRate(time.Minute)
	require.True(t, ok)
	require.InDelta(t, 100, rate, 1e-9)
}

func TestRecorderLossRateNeverExceedsHundred(t *testing.T) {
	t.Parallel()
	r := NewRecorder()
	r.AddCounters(10, 40)
	rate, ok := r.LossRate(time.Minute)
	require.True(t, ok)
	require.InDelta(t, 100, rate, 1e-9)
}

func TestRecorderLossRateWithoutSentIsMissing(t *testing.T) {
	t.Parallel()
	r := NewRecorder()
	r.AddCounters(0, 5)
	_, ok := r.LossRate(time.Minute)
	require.False(t, ok, "a loss with no send count says nothing about a rate")
}

// time.Now carries a monotonic reading, so the default clock cannot step back.
// A caller supplied clock can, and the samples already recorded are stamped
// against a timeline that no longer exists, so the ring is discarded rather
// than kept and misread.
func TestRecorderDiscardsRingOnBackwardsClockStep(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	r := newRecorderWithClock(clock.Now)
	r.AddCounters(100, 10)
	clock.Advance(-2 * time.Minute)

	_, ok := r.LossRate(time.Minute)
	require.False(t, ok, "samples from the old timeline must be dropped")

	// The recorder keeps working on the new timeline.
	r.AddCounters(100, 30)
	rate, ok := r.LossRate(time.Minute)
	require.True(t, ok)
	require.InDelta(t, 30, rate, 1e-9)
}

// A forward jump followed by a return to real time must not pin every later
// sample into one bucket, which would stop anything ever ageing out.
func TestRecorderRecoversFromForwardThenBackwardClockStep(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	r := newRecorderWithClock(clock.Now)
	r.AddCounters(100, 1)
	clock.Advance(time.Hour)
	r.AddCounters(100, 50)
	clock.Advance(-time.Hour)

	// Ten minutes of clean traffic on the restored clock.
	for i := 0; i < 20; i++ {
		clock.Advance(30 * time.Second)
		r.AddCounters(100, 0)
	}
	rate, ok := r.LossRate(30 * time.Second)
	require.True(t, ok)
	require.InDelta(t, 0, rate, 1e-9, "the burst from the skewed period must have aged out")
}

// Loss is detected after the packet was counted as sent, so an interval may
// report more losses than sends. Those losses must survive to the window.
func TestRecorderKeepsLateDetectedLoss(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	r := newRecorderWithClock(clock.Now)
	r.AddCounters(1000, 0)
	clock.Advance(BucketDuration)
	// The sender went quiet and a burst was then declared lost.
	r.AddCounters(0, 100)

	rate, ok := r.LossRate(time.Minute)
	require.True(t, ok)
	require.InDelta(t, 10, rate, 1e-9, "late detected loss must not be discarded")
}

func TestRecorderLossRateClampedAtWindowEdge(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	r := newRecorderWithClock(clock.Now)
	// Sends fall outside the window, their losses land inside it.
	r.AddCounters(1000, 0)
	clock.Advance(50 * time.Second)
	r.AddCounters(10, 400)

	rate, ok := r.LossRate(30 * time.Second)
	require.True(t, ok)
	require.InDelta(t, 100, rate, 1e-9, "a ratio above 100% must be clamped when read")
}

func TestRecorderZeroWindowReadsWholeRing(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	r := newRecorderWithClock(clock.Now)
	r.AddCounters(100, 10)
	clock.Advance(4 * time.Minute)
	rate, ok := r.LossRate(0)
	require.True(t, ok)
	require.InDelta(t, 10, rate, 1e-9)
}
