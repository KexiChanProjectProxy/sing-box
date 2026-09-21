package group

import "github.com/sagernet/sing-box/common/transportstats"

// The health check history feeds the sorter's latency_avg_* keys. It is only
// kept when a configured key reads it, so a group ranking on the latest delay
// alone allocates nothing.

// newLatencyRecorder builds a history recorder. It is a field on the group so
// that tests can drive the window with their own clock.
func (l *LoadBalance) newLatencyRecorder() *transportstats.Recorder {
	if l.latencyRecorderFactory != nil {
		return l.latencyRecorderFactory()
	}
	return transportstats.NewRecorder()
}

// latencyRecorder returns the member's health check history, creating it on
// first use. It returns nil when no configured key reads the history.
func (l *LoadBalance) latencyRecorder(tag string) *transportstats.Recorder {
	if l.latencyStats == nil {
		return nil
	}
	l.latencyStatsMu.Lock()
	defer l.latencyStatsMu.Unlock()
	recorder := l.latencyStats[tag]
	if recorder == nil {
		recorder = l.newLatencyRecorder()
		l.latencyStats[tag] = recorder
	}
	return recorder
}

// latencyRecorderIfExists returns the member's health check history without
// creating it, so that scoring does not allocate for members that never
// reported a delay.
func (l *LoadBalance) latencyRecorderIfExists(tag string) *transportstats.Recorder {
	if l.latencyStats == nil {
		return nil
	}
	l.latencyStatsMu.Lock()
	defer l.latencyStatsMu.Unlock()
	return l.latencyStats[tag]
}

// observeDelay records a successful health check result.
func (l *LoadBalance) observeDelay(tag string, delay uint16) {
	if recorder := l.latencyRecorder(tag); recorder != nil {
		recorder.AddLatency(float64(delay))
	}
}

// resetWindow drops a member's health check history, which happens when it
// fails a probe or a dial, so that a recovering member is not ranked on
// averages from before the failure.
func (l *LoadBalance) resetWindow(tag string) {
	if recorder := l.latencyRecorderIfExists(tag); recorder != nil {
		recorder.ResetLatency()
	}
}
