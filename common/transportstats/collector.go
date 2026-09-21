package transportstats

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Direction indexes a Collector. It mirrors adapter.TransportStatsDirection
// without importing it, so that this package stays free of the adapter layer.
type Direction int

const (
	DirectionClient Direction = iota
	DirectionServer
	directionCount
)

// MinActivePackets is how many packets a connection must send between two
// polls for that poll's sample to count.
//
// Below it the connection is carrying only keepalives, heartbeats and the
// statistics exchange itself, or its path has stalled. Such a sample is
// dropped unless it looks like a stall, so a member that carries little
// traffic is scored on the pool average rather than on a handful of packets.
const MinActivePackets = 32

// stalled reports whether a quiet sample lost at least half of what it sent,
// and at least two packets, which is what a failing path looks like once its retransmissions have
// backed off.
func stalled(delta Counters) bool {
	return delta.Lost >= 2 && delta.Lost*2 >= delta.Sent
}

// Sample is one connection's cumulative counters and current estimates, as a
// transport reports them.
type Sample struct {
	// Key identifies the connection the counters belong to. Counters are
	// cumulative per key and restart when the connection behind a key changes.
	Key uint64
	// Sent and Lost follow the AddCounters contract: sent includes the packets
	// later reported lost.
	Sent uint64
	Lost uint64
	// BytesSent is cumulative and is turned into throughput when DeliveryRate
	// is not reported.
	BytesSent uint64
	RTT       time.Duration
	RTTVar    time.Duration
	// DeliveryRate is a transport's own estimate in Mbps, or zero when it has
	// none.
	DeliveryRate float64
}

type directionState struct {
	enabled  atomic.Bool
	recorder *Recorder
	access   sync.Mutex
	counters *CounterTracker
	lastPoll time.Time
}

// Collector turns periodic connection snapshots into per-direction sliding
// windows. It is safe for concurrent use.
type Collector struct {
	now        func() time.Time
	directions [directionCount]*directionState
}

// NewCollector returns a Collector with every direction disabled.
func NewCollector() *Collector {
	return newCollectorWithClock(time.Now)
}

func newCollectorWithClock(now func() time.Time) *Collector {
	c := &Collector{now: now}
	for i := range c.directions {
		c.directions[i] = &directionState{
			recorder: newRecorderWithClock(now),
			counters: newCounterTrackerWithClock(now),
		}
	}
	return c
}

func (c *Collector) state(direction Direction) *directionState {
	if direction < 0 || direction >= directionCount {
		return nil
	}
	return c.directions[direction]
}

// Enable turns a direction on and reports whether it was off before.
func (c *Collector) Enable(direction Direction) bool {
	state := c.state(direction)
	if state == nil {
		return false
	}
	return state.enabled.CompareAndSwap(false, true)
}

// Enabled reports whether a direction is on.
func (c *Collector) Enabled(direction Direction) bool {
	state := c.state(direction)
	return state != nil && state.enabled.Load()
}

// Recorder returns the window of an enabled direction, and nil otherwise.
func (c *Collector) Recorder(direction Direction) *Recorder {
	state := c.state(direction)
	if state == nil || !state.enabled.Load() {
		return nil
	}
	return state.recorder
}

// Record folds one poll of connection snapshots into a direction's window.
//
// When complete is true the samples are every live connection, so keys absent
// from them are forgotten. A caller whose samples arrive one connection at a
// time passes false and calls Forget when a connection closes.
func (c *Collector) Record(direction Direction, samples []Sample, complete bool) {
	state := c.state(direction)
	if state == nil || !state.enabled.Load() {
		return
	}
	state.access.Lock()
	defer state.access.Unlock()
	now := c.now()
	var sent, lost, bytesSent uint64
	var reportedRate bool
	live := make(map[uint64]bool, len(samples))
	for _, sample := range samples {
		live[sample.Key] = true
		delta := state.counters.Delta(sample.Key, Counters{Sent: sample.Sent, Lost: sample.Lost, Bytes: sample.BytesSent})
		if sample.DeliveryRate > 0 {
			// The transport estimates its own rate, so byte counts must not
			// stand in for it, even in a poll whose samples are all skipped.
			reportedRate = true
		}
		if delta.Sent < MinActivePackets {
			// Too little was sent since the last poll for its RTT or delivery
			// rate to describe the path; they are left over from older traffic.
			// The counters are kept only when the sample looks like a stall,
			// at least half of it lost: a connection whose path has mostly died also
			// sends little, as its retransmissions back off, and dropping it
			// would hide exactly the members losing the most. An occasional
			// loss among a few keepalives is dropped like any other quiet
			// sample, since counting it alone would read as a huge loss rate.
			if stalled(delta) {
				sent += delta.Sent
				lost += delta.Lost
			}
			continue
		}
		sent += delta.Sent
		lost += delta.Lost
		bytesSent += delta.Bytes
		state.recorder.AddRTT(milliseconds(sample.RTT), milliseconds(sample.RTTVar))
		if sample.DeliveryRate > 0 {
			state.recorder.AddDeliveryRate(sample.DeliveryRate)
		}
	}
	state.recorder.AddCounters(sent, lost)
	if !reportedRate && bytesSent > 0 && !state.lastPoll.IsZero() {
		if elapsed := now.Sub(state.lastPoll); elapsed > 0 {
			state.recorder.AddDeliveryRate(float64(bytesSent) * 8 / elapsed.Seconds() / 1e6)
		}
	}
	state.lastPoll = now
	if complete {
		state.counters.Retain(func(key uint64) bool { return live[key] })
	}
}

// Forget drops the baseline of a connection that closed.
func (c *Collector) Forget(direction Direction, key uint64) {
	state := c.state(direction)
	if state == nil {
		return
	}
	state.counters.Forget(key)
}

// Run calls poll once per bucket until ctx is done.
func Run(ctx context.Context, poll func(ctx context.Context)) {
	ticker := time.NewTicker(BucketDuration)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			poll(ctx)
		}
	}
}

func milliseconds(duration time.Duration) float64 {
	return float64(duration) / float64(time.Millisecond)
}
