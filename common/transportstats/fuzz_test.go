package transportstats

import (
	"math"
	"testing"
	"time"
)

// FuzzRecorder drives a recorder through arbitrary sample and clock sequences
// and checks the invariants every reader must hold.
func FuzzRecorder(f *testing.F) {
	f.Add(uint64(100), uint64(10), int64(5), uint16(40), uint8(3))
	f.Add(uint64(0), uint64(0), int64(-1000), uint16(0), uint8(0))
	f.Add(uint64(math.MaxUint64), uint64(math.MaxUint64), int64(1<<40), uint16(65535), uint8(255))

	f.Fuzz(func(t *testing.T, sent, lost uint64, advanceSeconds int64, sample uint16, rounds uint8) {
		clock := newFakeClock()
		r := newRecorderWithClock(clock.Now)
		for i := 0; i < int(rounds%32); i++ {
			r.AddCounters(sent, lost)
			r.AddRTT(float64(sample), float64(sample)/4)
			r.AddDeliveryRate(float64(sample))
			r.AddLatency(float64(sample))
			// Clamp the step so the fake clock cannot overflow time.Time.
			clock.Advance(time.Duration(advanceSeconds%3600) * time.Second)
		}
		for _, window := range []time.Duration{
			0, time.Second, 30 * time.Second, time.Minute, 5 * time.Minute, MaxWindow, 24 * time.Hour,
		} {
			rate, ok := r.LossRate(window)
			if ok && (rate < 0 || rate > 100 || math.IsNaN(rate)) {
				t.Fatalf("loss rate %v out of range for window %v", rate, window)
			}
			for name, read := range map[string]func(time.Duration) (float64, bool){
				"rtt":           r.RTT,
				"rttvar":        r.RTTVar,
				"delivery_rate": r.DeliveryRate,
				"latency":       r.Latency,
			} {
				value, ok := read(window)
				if !ok {
					continue
				}
				if math.IsNaN(value) || math.IsInf(value, 0) {
					t.Fatalf("%s returned a non-finite %v for window %v", name, value, window)
				}
				if value < 0 {
					t.Fatalf("%s returned a negative %v for window %v", name, value, window)
				}
			}
		}
	})
}

// FuzzCounterTracker checks that increments are never fabricated, whatever the
// order of cumulative samples a transport reports.
func FuzzCounterTracker(f *testing.F) {
	f.Add(uint64(1), uint64(100), uint64(50), uint64(200), uint64(60))
	f.Add(uint64(7), uint64(0), uint64(0), uint64(0), uint64(0))
	f.Add(uint64(3), uint64(math.MaxUint64), uint64(1), uint64(0), uint64(math.MaxUint64))

	f.Fuzz(func(t *testing.T, key, firstSent, firstLost, secondSent, secondLost uint64) {
		tracker := NewCounterTracker()
		if delta := tracker.Delta(key, Counters{Sent: firstSent, Lost: firstLost}); delta != (Counters{}) {
			t.Fatalf("the first sample of a key must be a baseline, got %+v", delta)
		}
		delta := tracker.Delta(key, Counters{Sent: secondSent, Lost: secondLost})
		if secondSent < firstSent || secondLost < firstLost {
			if delta != (Counters{}) {
				t.Fatalf("a counter reset must report no increment, got %+v", delta)
			}
			return
		}
		if delta.Sent != secondSent-firstSent || delta.Lost != secondLost-firstLost {
			t.Fatalf("increment %+v does not match the samples", delta)
		}
	})
}
