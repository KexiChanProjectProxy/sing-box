package group

import "math"

type delayWindow struct {
	samples  []uint16
	weighted uint16
}

func computeWeightedDelay(samples []uint16, windowWeight, lastWeight uint16) uint16 {
	n := uint64(len(samples))
	last := uint64(samples[n-1])
	var sum uint64
	for _, sample := range samples {
		sum += uint64(sample)
	}
	weighted := (uint64(windowWeight)*sum + uint64(lastWeight)*last*n) / (uint64(windowWeight+lastWeight) * n)
	if weighted > math.MaxUint16 {
		return math.MaxUint16
	}
	return uint16(weighted)
}

func (l *LoadBalance) observeDelay(tag string, delay uint16) uint16 {
	if l.delayWindow == 0 {
		return delay
	}
	l.windowsMu.Lock()
	defer l.windowsMu.Unlock()
	window := l.windows[tag]
	if window == nil {
		window = &delayWindow{}
		l.windows[tag] = window
	}
	window.samples = append(window.samples, delay)
	for len(window.samples) > l.delayWindow {
		window.samples = window.samples[1:]
	}
	window.weighted = computeWeightedDelay(window.samples, l.windowWeight, l.lastWeight)
	return window.weighted
}

func (l *LoadBalance) resetWindow(tag string) {
	if l.delayWindow == 0 {
		return
	}
	l.windowsMu.Lock()
	delete(l.windows, tag)
	l.windowsMu.Unlock()
}

func (l *LoadBalance) rankedDelay(tag string, delay uint16) uint16 {
	if l.delayWindow == 0 {
		return delay
	}
	l.windowsMu.Lock()
	defer l.windowsMu.Unlock()
	window := l.windows[tag]
	if window == nil || len(window.samples) == 0 {
		return delay
	}
	return window.weighted
}
