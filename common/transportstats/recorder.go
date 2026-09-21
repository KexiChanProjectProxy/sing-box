// Package transportstats aggregates per-outbound transport quality samples
// (latency, RTT, loss and delivery rate) into a fixed sliding window of time
// buckets, so that a load balancer can rank outbounds over 30s/1m/5m windows.
//
// Values are stored in the units the ranking uses directly: milliseconds for
// latency/RTT/RTT variance, and Mbps for delivery rate. Callers convert from
// whatever their transport reports (TCP_INFO microseconds, QUIC durations,
// bytes per second) before recording.
package transportstats

import (
	"sync"
	"time"
)

const (
	// BucketDuration is the width of one time bucket.
	BucketDuration = 10 * time.Second
	// BucketCount is the number of buckets kept in the ring.
	BucketCount = 30
	// MaxWindow is the longest window a Recorder can answer.
	MaxWindow = BucketDuration * BucketCount
)

type bucket struct {
	index int64

	sentPackets uint64
	lostPackets uint64

	rttSum      float64
	rttCount    uint64
	rttVarSum   float64
	rttVarCount uint64

	deliveryRate    float64
	hasDeliveryRate bool

	latencySum   float64
	latencyCount uint64
}

// Recorder is a sliding window of transport quality samples for one outbound
// and one direction. It is safe for concurrent use.
type Recorder struct {
	access sync.Mutex
	now    func() time.Time
	// base anchors bucket numbering. Elapsed time is measured from it rather
	// than from the epoch, so that the ring follows Go's monotonic reading and
	// a wall clock step, such as an NTP correction, cannot blind the window.
	base time.Time
	// highest is the newest bucket index reached so far, used to detect a clock
	// that moved backwards.
	highest int64
	buckets [BucketCount]bucket
}

// NewRecorder returns an empty Recorder using the wall clock.
func NewRecorder() *Recorder {
	return newRecorderWithClock(time.Now)
}

// NewRecorderWithClock returns an empty Recorder reading time from now, for
// callers that supply their own clock.
func NewRecorderWithClock(now func() time.Time) *Recorder {
	return newRecorderWithClock(now)
}

func newRecorderWithClock(now func() time.Time) *Recorder {
	r := &Recorder{now: now, base: now()}
	r.reset()
	return r
}

// bucketIndex numbers buckets by elapsed time since the recorder was created.
//
// time.Now carries a monotonic reading, so a wall clock correction cannot move
// this backwards for the default clock. A caller that supplies its own clock
// can still hand back an earlier instant, and the samples already recorded are
// then stamped against a timeline that no longer exists, so the ring is
// discarded rather than kept and misread. The caller must hold access.
func (r *Recorder) bucketIndex() int64 {
	index := int64(r.now().Sub(r.base) / BucketDuration)
	if index < r.highest {
		r.reset()
	}
	r.highest = index
	return index
}

// reset empties every bucket. The caller must hold access.
func (r *Recorder) reset() {
	for i := range r.buckets {
		r.buckets[i] = bucket{index: -1}
	}
}

// current returns the bucket for the current instant, recycling the slot when
// it still holds an older bucket. The caller must hold access.
func (r *Recorder) current() *bucket {
	index := r.bucketIndex()
	slot := index % BucketCount
	if slot < 0 {
		slot += BucketCount
	}
	b := &r.buckets[slot]
	if b.index != index {
		*b = bucket{index: index}
	}
	return b
}

// forEach visits every live bucket within window. The caller must hold access.
//
// The current bucket is partial, so a window covers between window-BucketDuration
// and window of real time. Reading is therefore never stale by more than one
// bucket, and never reaches further back than the window names.
func (r *Recorder) forEach(window time.Duration, visit func(b *bucket)) {
	if window <= 0 || window > MaxWindow {
		window = MaxWindow
	}
	newest := r.bucketIndex()
	span := int64((window + BucketDuration - 1) / BucketDuration)
	oldest := newest - span + 1
	for i := range r.buckets {
		b := &r.buckets[i]
		if b.index < oldest || b.index > newest {
			continue
		}
		visit(b)
	}
}

// AddCounters records packet counter increments.
//
// sent is every packet put on the wire, retransmissions included, and lost is
// the subset of those that were reported lost or retransmitted. That is what
// both of the transports this serves report: TCP_INFO counts retransmitted
// segments in tcpi_segs_out, and a QUIC sender counts a lost packet in its sent
// total. A caller that has a delivered count instead must add the lost count
// back before reporting it here.
//
// Callers convert cumulative transport counters into increments with a
// CounterTracker.
func (r *Recorder) AddCounters(sent, lost uint64) {
	if sent == 0 && lost == 0 {
		return
	}
	// lost is deliberately not clamped to sent here. Loss is detected after the
	// packet was already counted as sent, often in a later sampling interval,
	// so an interval where a burst is declared lost legitimately reports more
	// losses than sends. Clamping per interval would discard exactly the spikes
	// worth ranking on; the ratio is clamped when the window is read instead.
	r.access.Lock()
	defer r.access.Unlock()
	b := r.current()
	b.sentPackets += sent
	b.lostPackets += lost
}

// AddRTT records a smoothed RTT sample and its variance, both in milliseconds.
// A non-positive value is skipped, so a source that only knows one of the two
// can pass zero for the other.
func (r *Recorder) AddRTT(rtt, rttVar float64) {
	if rtt <= 0 && rttVar <= 0 {
		return
	}
	r.access.Lock()
	defer r.access.Unlock()
	b := r.current()
	if rtt > 0 {
		b.rttSum += rtt
		b.rttCount++
	}
	if rttVar > 0 {
		b.rttVarSum += rttVar
		b.rttVarCount++
	}
}

// AddDeliveryRate records a delivery rate sample in Mbps, keeping the peak of
// the current bucket. Delivery rate is application limited most of the time, so
// the peak is a closer estimate of what the path can carry than the mean.
func (r *Recorder) AddDeliveryRate(mbps float64) {
	if mbps <= 0 {
		return
	}
	r.access.Lock()
	defer r.access.Unlock()
	b := r.current()
	if !b.hasDeliveryRate || mbps > b.deliveryRate {
		b.deliveryRate = mbps
		b.hasDeliveryRate = true
	}
}

// AddLatency records a health check latency sample in milliseconds.
func (r *Recorder) AddLatency(milliseconds float64) {
	if milliseconds <= 0 {
		return
	}
	r.access.Lock()
	defer r.access.Unlock()
	b := r.current()
	b.latencySum += milliseconds
	b.latencyCount++
}

// ResetLatency drops every latency sample, leaving transport samples intact. It
// is called when a member fails its health check, so that a recovering member
// is not ranked on stale averages.
func (r *Recorder) ResetLatency() {
	r.access.Lock()
	defer r.access.Unlock()
	for i := range r.buckets {
		r.buckets[i].latencySum = 0
		r.buckets[i].latencyCount = 0
	}
}

// LossRate returns the percentage of sent packets that were lost over window.
func (r *Recorder) LossRate(window time.Duration) (float64, bool) {
	r.access.Lock()
	defer r.access.Unlock()
	var sent, lost uint64
	r.forEach(window, func(b *bucket) {
		sent += b.sentPackets
		lost += b.lostPackets
	})
	if sent == 0 {
		return 0, false
	}
	if lost > sent {
		// Losses carried over from packets sent before the window opened.
		lost = sent
	}
	return float64(lost) * 100 / float64(sent), true
}

// RTT returns the mean smoothed RTT in milliseconds over window.
func (r *Recorder) RTT(window time.Duration) (float64, bool) {
	r.access.Lock()
	defer r.access.Unlock()
	var sum float64
	var count uint64
	r.forEach(window, func(b *bucket) {
		sum += b.rttSum
		count += b.rttCount
	})
	if count == 0 {
		return 0, false
	}
	return sum / float64(count), true
}

// RTTVar returns the mean RTT variance in milliseconds over window.
func (r *Recorder) RTTVar(window time.Duration) (float64, bool) {
	r.access.Lock()
	defer r.access.Unlock()
	var sum float64
	var count uint64
	r.forEach(window, func(b *bucket) {
		sum += b.rttVarSum
		count += b.rttVarCount
	})
	if count == 0 {
		return 0, false
	}
	return sum / float64(count), true
}

// DeliveryRate returns the peak delivery rate in Mbps over window.
func (r *Recorder) DeliveryRate(window time.Duration) (float64, bool) {
	r.access.Lock()
	defer r.access.Unlock()
	var peak float64
	var found bool
	r.forEach(window, func(b *bucket) {
		if b.hasDeliveryRate && (!found || b.deliveryRate > peak) {
			peak = b.deliveryRate
			found = true
		}
	})
	return peak, found
}

// Latency returns the mean health check latency in milliseconds over window.
func (r *Recorder) Latency(window time.Duration) (float64, bool) {
	r.access.Lock()
	defer r.access.Unlock()
	var sum float64
	var count uint64
	r.forEach(window, func(b *bucket) {
		sum += b.latencySum
		count += b.latencyCount
	})
	if count == 0 {
		return 0, false
	}
	return sum / float64(count), true
}
