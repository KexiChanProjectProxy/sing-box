package transportstats

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCollectorDisabledByDefault(t *testing.T) {
	t.Parallel()
	c := NewCollector()
	require.Nil(t, c.Recorder(DirectionClient))
	require.False(t, c.Enabled(DirectionServer))
	// Recording into a disabled direction is a no-op.
	c.Record(DirectionClient, []Sample{{Key: 1, Sent: 10}}, true)
	require.True(t, c.Enable(DirectionClient))
	require.False(t, c.Enable(DirectionClient), "a second enable reports no change")
	_, ok := c.Recorder(DirectionClient).LossRate(MaxWindow)
	require.False(t, ok)
}

func TestCollectorRejectsUnknownDirection(t *testing.T) {
	t.Parallel()
	c := NewCollector()
	require.False(t, c.Enable(Direction(7)))
	require.Nil(t, c.Recorder(Direction(-1)))
	c.Record(Direction(9), nil, true)
	c.Forget(Direction(9), 1)
}

func TestCollectorTurnsCumulativeCountersIntoLossRate(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	c := newCollectorWithClock(clock.Now)
	c.Enable(DirectionClient)
	c.Record(DirectionClient, []Sample{{Key: 1, Sent: 1000, Lost: 100}}, true)
	clock.Advance(BucketDuration)
	c.Record(DirectionClient, []Sample{{Key: 1, Sent: 1200, Lost: 110}}, true)
	rate, ok := c.Recorder(DirectionClient).LossRate(time.Minute)
	require.True(t, ok)
	require.InDelta(t, 5, rate, 1e-9, "only the increment since the baseline counts")
}

func TestCollectorSumsConnections(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	c := newCollectorWithClock(clock.Now)
	c.Enable(DirectionClient)
	c.Record(DirectionClient, []Sample{{Key: 1, Sent: 100}, {Key: 2, Sent: 100}}, true)
	clock.Advance(BucketDuration)
	c.Record(DirectionClient, []Sample{{Key: 1, Sent: 200, Lost: 10}, {Key: 2, Sent: 200, Lost: 30}}, true)
	rate, ok := c.Recorder(DirectionClient).LossRate(time.Minute)
	require.True(t, ok)
	require.InDelta(t, 20, rate, 1e-9)
}

func TestCollectorThroughputFromBytes(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	c := newCollectorWithClock(clock.Now)
	c.Enable(DirectionServer)
	c.Record(DirectionServer, []Sample{{Key: 1, BytesSent: 0, Sent: 1}}, true)
	clock.Advance(BucketDuration)
	// 12.5 MB over 10 s is 10 Mbps.
	c.Record(DirectionServer, []Sample{{Key: 1, BytesSent: 12_500_000, Sent: 10_000}}, true)
	rate, ok := c.Recorder(DirectionServer).DeliveryRate(time.Minute)
	require.True(t, ok)
	require.InDelta(t, 10, rate, 1e-9)
}

func TestCollectorPrefersReportedDeliveryRate(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	c := newCollectorWithClock(clock.Now)
	c.Enable(DirectionClient)
	c.Record(DirectionClient, []Sample{{Key: 1, Sent: 1, BytesSent: 1, DeliveryRate: 50}}, true)
	clock.Advance(BucketDuration)
	c.Record(DirectionClient, []Sample{{Key: 1, Sent: 1000, BytesSent: 1_000_000_000, DeliveryRate: 50}}, true)
	rate, ok := c.Recorder(DirectionClient).DeliveryRate(time.Minute)
	require.True(t, ok)
	require.InDelta(t, 50, rate, 1e-9, "a transport's own estimate wins over byte counts")
}

func TestCollectorRecordsRTT(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	c := newCollectorWithClock(clock.Now)
	c.Enable(DirectionClient)
	c.Record(DirectionClient, []Sample{{Key: 1, Sent: 1}, {Key: 2, Sent: 1}}, true)
	clock.Advance(BucketDuration)
	c.Record(DirectionClient, []Sample{
		{Key: 1, Sent: 100, RTT: 20 * time.Millisecond, RTTVar: 2 * time.Millisecond},
		{Key: 2, Sent: 100, RTT: 40 * time.Millisecond, RTTVar: 4 * time.Millisecond},
	}, true)
	rtt, ok := c.Recorder(DirectionClient).RTT(time.Minute)
	require.True(t, ok)
	require.InDelta(t, 30, rtt, 1e-9)
	rttVar, ok := c.Recorder(DirectionClient).RTTVar(time.Minute)
	require.True(t, ok)
	require.InDelta(t, 3, rttVar, 1e-9)
}

// An idle connection keeps reporting the RTT and delivery rate of its last
// burst; those must not be recorded again, or an old peak would pin the window.
func TestCollectorIgnoresEstimatesFromIdleConnections(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	c := newCollectorWithClock(clock.Now)
	c.Enable(DirectionClient)
	c.Record(DirectionClient, []Sample{{Key: 1, Sent: 10, RTT: time.Second, DeliveryRate: 900}}, true)
	for i := 0; i < 12; i++ {
		clock.Advance(BucketDuration)
		c.Record(DirectionClient, []Sample{{Key: 1, Sent: 10, RTT: time.Second, DeliveryRate: 900}}, true)
	}
	_, ok := c.Recorder(DirectionClient).RTT(time.Minute)
	require.False(t, ok, "a connection that sent nothing contributes no RTT")
	_, ok = c.Recorder(DirectionClient).DeliveryRate(time.Minute)
	require.False(t, ok, "nor a delivery rate")
}

// The first sample of a connection is only a baseline, so its estimates wait
// for the next poll too.
func TestCollectorFirstSampleIsBaselineOnly(t *testing.T) {
	t.Parallel()
	c := NewCollector()
	c.Enable(DirectionClient)
	c.Record(DirectionClient, []Sample{{Key: 1, Sent: 10, RTT: 20 * time.Millisecond}}, true)
	_, ok := c.Recorder(DirectionClient).RTT(time.Minute)
	require.False(t, ok)
}

// A complete poll forgets connections that closed, so a later connection that
// reuses a key is not diffed against a stale baseline.
func TestCollectorCompletePollForgetsClosedConnections(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	c := newCollectorWithClock(clock.Now)
	c.Enable(DirectionClient)
	c.Record(DirectionClient, []Sample{{Key: 1, Sent: 5000}}, true)
	clock.Advance(BucketDuration)
	c.Record(DirectionClient, nil, true)
	clock.Advance(BucketDuration)
	// Key 1 returns with counters far above its old baseline would allow.
	c.Record(DirectionClient, []Sample{{Key: 1, Sent: 9000, Lost: 4000}}, true)
	_, ok := c.Recorder(DirectionClient).LossRate(time.Minute)
	require.False(t, ok, "a reappearing key starts from a fresh baseline")
}

func TestCollectorForget(t *testing.T) {
	t.Parallel()
	c := NewCollector()
	c.Enable(DirectionServer)
	c.Record(DirectionServer, []Sample{{Key: 3, Sent: 100}}, false)
	c.Forget(DirectionServer, 3)
	c.Record(DirectionServer, []Sample{{Key: 3, Sent: 500, Lost: 400}}, false)
	_, ok := c.Recorder(DirectionServer).LossRate(time.Minute)
	require.False(t, ok)
}

// A connection carrying only keepalives must not be scored on them: a lost
// keepalive is usually found by the probe timer and never reported, so such a
// member would read as lossless, and its few bytes as a tiny throughput.
func TestCollectorIgnoresKeepaliveOnlyTraffic(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	c := newCollectorWithClock(clock.Now)
	c.Enable(DirectionClient)
	sent, bytesSent := uint64(0), uint64(0)
	for i := 0; i < 20; i++ {
		sent += 3
		bytesSent += 150
		c.Record(DirectionClient, []Sample{{Key: 1, Sent: sent, BytesSent: bytesSent, RTT: 30 * time.Millisecond}}, true)
		clock.Advance(BucketDuration)
	}
	recorder := c.Recorder(DirectionClient)
	for name, read := range map[string]func(time.Duration) (float64, bool){
		"loss": recorder.LossRate, "rtt": recorder.RTT, "delivery rate": recorder.DeliveryRate,
	} {
		_, ok := read(time.Minute)
		require.False(t, ok, "%s must be missing for keepalive only traffic", name)
	}
}

func TestCollectorCountsActiveTraffic(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	c := newCollectorWithClock(clock.Now)
	c.Enable(DirectionClient)
	c.Record(DirectionClient, []Sample{{Key: 1, Sent: 0}}, true)
	clock.Advance(BucketDuration)
	c.Record(DirectionClient, []Sample{{Key: 1, Sent: MinActivePackets, Lost: 8, RTT: 30 * time.Millisecond}}, true)
	rate, ok := c.Recorder(DirectionClient).LossRate(time.Minute)
	require.True(t, ok, "exactly the threshold counts")
	require.InDelta(t, 25, rate, 1e-9)
}

// A stalled path also sends little, as retransmissions back off. Its losses
// must still count, or the members losing the most would read as clean.
func TestCollectorKeepsLossFromStalledConnections(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	c := newCollectorWithClock(clock.Now)
	c.Enable(DirectionServer)
	c.Record(DirectionServer, []Sample{{Key: 1, Sent: 1000, Lost: 0}}, false)
	clock.Advance(BucketDuration)
	c.Record(DirectionServer, []Sample{{Key: 1, Sent: 1006, Lost: 5, RTT: time.Second, DeliveryRate: 0.01}}, false)
	recorder := c.Recorder(DirectionServer)
	rate, ok := recorder.LossRate(time.Minute)
	require.True(t, ok)
	require.InDelta(t, 5.0/6.0*100, rate, 1e-9)
	_, ok = recorder.RTT(time.Minute)
	require.False(t, ok, "RTT from a quiet sample is still dropped")
	_, ok = recorder.DeliveryRate(time.Minute)
	require.False(t, ok, "so is its delivery rate")
}

// An occasional retransmission among an idle member's few packets must not be
// scored on its own, or a healthy idle member would read as heavily lossy.
func TestCollectorIgnoresOccasionalLossOnQuietConnections(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	c := newCollectorWithClock(clock.Now)
	c.Enable(DirectionServer)
	sent, lost := uint64(1000), uint64(0)
	c.Record(DirectionServer, []Sample{{Key: 1, Sent: sent, Lost: lost}}, false)
	for i := 0; i < 29; i++ {
		clock.Advance(BucketDuration)
		sent += 2
		if i == 10 {
			lost++
		}
		c.Record(DirectionServer, []Sample{{Key: 1, Sent: sent, Lost: lost}}, false)
	}
	_, ok := c.Recorder(DirectionServer).LossRate(MaxWindow)
	require.False(t, ok, "one retransmission among quiet samples must leave the member on the pool average")
}

func TestStalled(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		delta Counters
		want  bool
	}{
		{Counters{Sent: 6, Lost: 5}, true},
		{Counters{Sent: 4, Lost: 2}, true},
		{Counters{Sent: 2, Lost: 1}, false},
		{Counters{Sent: 10, Lost: 4}, false},
		{Counters{Sent: 0, Lost: 3}, true},
		{Counters{}, false},
	} {
		require.Equal(t, testCase.want, stalled(testCase.delta), "%+v", testCase.delta)
	}
}
