package group

import (
	"sync"
	"sync/atomic"
	"time"
)

// LatencyTracker tracks real connection latency and health per outbound
type LatencyTracker struct {
	mu        sync.RWMutex
	outbounds map[string]*OutboundStats

	// Configuration
	failureThreshold  uint32 // Consecutive failures to mark unhealthy
	recoveryThreshold uint32 // Consecutive successes to mark healthy
	historySize       int    // Latency history window size
	samplingRate      int    // Sample 1 in N connections
	samplingCounter   atomic.Uint64
}

// OutboundStats holds statistics for a single outbound
type OutboundStats struct {
	tag string
	mu  sync.RWMutex

	// Latency history (ring buffer for moving average)
	latencyHistory []time.Duration
	historyIndex   int
	historyFull    bool

	// Per-tier failure tracking
	tierFailures map[int]*TierFailureState
}

// TierFailureState tracks health state for one tier
type TierFailureState struct {
	consecutiveFailures  uint32
	consecutiveSuccesses uint32
	maxLatency           time.Duration
}

const (
	defaultFailureThreshold  = 3
	defaultRecoveryThreshold = 2
	defaultHistorySize       = 10
	defaultSamplingRate      = 1
)

// NewLatencyTracker creates a new latency tracker
func NewLatencyTracker(
	failureThreshold uint32,
	recoveryThreshold uint32,
	historySize int,
	samplingRate int,
) *LatencyTracker {
	if failureThreshold == 0 {
		failureThreshold = defaultFailureThreshold
	}
	if recoveryThreshold == 0 {
		recoveryThreshold = defaultRecoveryThreshold
	}
	if historySize <= 0 {
		historySize = defaultHistorySize
	}
	if samplingRate <= 0 {
		samplingRate = defaultSamplingRate
	}

	return &LatencyTracker{
		outbounds:         make(map[string]*OutboundStats),
		failureThreshold:  failureThreshold,
		recoveryThreshold: recoveryThreshold,
		historySize:       historySize,
		samplingRate:      samplingRate,
	}
}

// RegisterOutbound registers an outbound with tier-specific thresholds
func (lt *LatencyTracker) RegisterOutbound(tag string, tierLevel int, maxLatency time.Duration) {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	stats, exists := lt.outbounds[tag]
	if !exists {
		stats = &OutboundStats{
			tag:            tag,
			latencyHistory: make([]time.Duration, 0, lt.historySize),
			tierFailures:   make(map[int]*TierFailureState),
		}
		lt.outbounds[tag] = stats
	}

	stats.mu.Lock()
	defer stats.mu.Unlock()

	// Register tier threshold
	stats.tierFailures[tierLevel] = &TierFailureState{
		maxLatency: maxLatency,
	}
}

// ShouldSample returns true if this connection should be sampled
func (lt *LatencyTracker) ShouldSample() bool {
	if lt.samplingRate == 1 {
		return true
	}
	counter := lt.samplingCounter.Add(1)
	return (counter % uint64(lt.samplingRate)) == 0
}

// RecordLatency records connection latency and updates health status
func (lt *LatencyTracker) RecordLatency(
	tag string,
	tierLevel int,
	duration time.Duration,
	success bool,
) {
	lt.mu.RLock()
	stats, exists := lt.outbounds[tag]
	lt.mu.RUnlock()

	if !exists {
		return
	}

	stats.mu.Lock()
	defer stats.mu.Unlock()

	// Update latency history on success
	if success {
		stats.addLatencyLocked(duration)
	}

	// Update per-tier failure state
	tierState, exists := stats.tierFailures[tierLevel]
	if !exists {
		return // Outbound not configured for this tier
	}

	if !success || duration > tierState.maxLatency {
		// Failed or too slow for this tier
		tierState.consecutiveFailures++
		tierState.consecutiveSuccesses = 0
	} else {
		// Success within threshold
		tierState.consecutiveFailures = 0
		tierState.consecutiveSuccesses++
	}
}

// IsHealthyForTier checks if outbound is healthy for specific tier
func (lt *LatencyTracker) IsHealthyForTier(tag string, tierLevel int) bool {
	lt.mu.RLock()
	stats, exists := lt.outbounds[tag]
	lt.mu.RUnlock()

	if !exists {
		return true // Unknown = optimistic (cold start)
	}

	stats.mu.RLock()
	defer stats.mu.RUnlock()

	tierState, exists := stats.tierFailures[tierLevel]
	if !exists {
		return true // Not in this tier
	}

	// Unhealthy if consecutive failures exceed threshold
	if tierState.consecutiveFailures >= lt.failureThreshold {
		return false
	}

	return true
}

// GetAverageLatency returns moving average latency
func (lt *LatencyTracker) GetAverageLatency(tag string) time.Duration {
	lt.mu.RLock()
	stats, exists := lt.outbounds[tag]
	lt.mu.RUnlock()

	if !exists || len(stats.latencyHistory) == 0 {
		return 0 // Unknown = will sort to end
	}

	stats.mu.RLock()
	defer stats.mu.RUnlock()

	var sum time.Duration
	count := len(stats.latencyHistory)
	for i := 0; i < count; i++ {
		sum += stats.latencyHistory[i]
	}

	if count == 0 {
		return 0
	}

	return sum / time.Duration(count)
}

// GetLatencyHistory returns a copy of latency history for monitoring
func (lt *LatencyTracker) GetLatencyHistory(tag string) []time.Duration {
	lt.mu.RLock()
	stats, exists := lt.outbounds[tag]
	lt.mu.RUnlock()

	if !exists {
		return nil
	}

	stats.mu.RLock()
	defer stats.mu.RUnlock()

	history := make([]time.Duration, len(stats.latencyHistory))
	copy(history, stats.latencyHistory)
	return history
}

// GetTierStats returns failure/success counts for a tier
func (lt *LatencyTracker) GetTierStats(tag string, tierLevel int) (failures, successes uint32, exists bool) {
	lt.mu.RLock()
	stats, ok := lt.outbounds[tag]
	lt.mu.RUnlock()

	if !ok {
		return 0, 0, false
	}

	stats.mu.RLock()
	defer stats.mu.RUnlock()

	tierState, ok := stats.tierFailures[tierLevel]
	if !ok {
		return 0, 0, false
	}

	return tierState.consecutiveFailures, tierState.consecutiveSuccesses, true
}

// addLatencyLocked adds latency to ring buffer (caller must hold lock)
func (s *OutboundStats) addLatencyLocked(duration time.Duration) {
	if len(s.latencyHistory) < cap(s.latencyHistory) {
		// Still filling buffer
		s.latencyHistory = append(s.latencyHistory, duration)
	} else {
		// Ring buffer full, overwrite oldest
		s.latencyHistory[s.historyIndex] = duration
		s.historyIndex = (s.historyIndex + 1) % len(s.latencyHistory)
		s.historyFull = true
	}
}

// Reset clears all statistics (useful for testing)
func (lt *LatencyTracker) Reset() {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	for _, stats := range lt.outbounds {
		stats.mu.Lock()
		stats.latencyHistory = stats.latencyHistory[:0]
		stats.historyIndex = 0
		stats.historyFull = false
		for _, tierState := range stats.tierFailures {
			tierState.consecutiveFailures = 0
			tierState.consecutiveSuccesses = 0
		}
		stats.mu.Unlock()
	}
}
