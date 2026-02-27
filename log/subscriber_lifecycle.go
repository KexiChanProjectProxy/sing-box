package log

import (
	"sync"
	"time"
)

// ConnectionStats tracks the lifecycle of a single connection
type ConnectionStats struct {
	ConnectionID uint32
	InboundTag   string
	OutboundTag  string
	Network      string
	Source       string
	Destination  string
	Domain       string
	StartedAt    time.Time
	ClosedAt     time.Time
	UploadBytes  int64
	DownloadBytes int64
	Phases       []string // phases seen: "inbound_start", "dns", "router_match", "outbound_start", "transfer", "close"
	Error        string
}

// LifecycleCompleteFunc is called when a connection lifecycle completes
type LifecycleCompleteFunc func(stats *ConnectionStats)

// LifecycleTracker subscribes to events and correlates them by connection ID
type LifecycleTracker struct {
	mu         sync.Mutex
	connections map[uint32]*ConnectionStats
	onComplete LifecycleCompleteFunc
	ttl        time.Duration
	stopCh     chan struct{}
}

// NewLifecycleTracker creates a new LifecycleTracker.
// onComplete is called when a connection closes or times out.
// ttl is the maximum duration to track a connection without seeing a close event.
func NewLifecycleTracker(onComplete LifecycleCompleteFunc, ttl time.Duration) *LifecycleTracker {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	t := &LifecycleTracker{
		connections: make(map[uint32]*ConnectionStats),
		onComplete:  onComplete,
		ttl:         ttl,
		stopCh:      make(chan struct{}),
	}
	go t.cleanupLoop()
	return t
}

// HandleEvent implements EventSubscriber
func (t *LifecycleTracker) HandleEvent(entry LogEntry) {
	if entry.ConnectionID == 0 || entry.Event == nil {
		return
	}
	id := entry.ConnectionID

	t.mu.Lock()
	defer t.mu.Unlock()

	stats, exists := t.connections[id]
	if !exists {
		stats = &ConnectionStats{
			ConnectionID: id,
			StartedAt:    entry.Timestamp,
		}
		t.connections[id] = stats
	}

	switch entry.Event.Type {
	case EventTypeConnection:
		t.handleConnectionEvent(stats, entry)
	case EventTypeDNS:
		stats.addPhase("dns")
	case EventTypeRouterMatch:
		stats.addPhase("router_match")
	case EventTypeTransfer:
		t.handleTransferEvent(stats, entry)
	}

	// Check if connection is complete
	if stats.isClosed() {
		delete(t.connections, id)
		if t.onComplete != nil {
			// Call outside lock to avoid deadlock
			go t.onComplete(stats)
		}
	}
}

func (t *LifecycleTracker) handleConnectionEvent(stats *ConnectionStats, entry LogEntry) {
	data := entry.Event.Data
	direction, _ := data["direction"].(string)
	action, _ := data["action"].(string)

	if source, ok := data["source"].(string); ok && source != "" {
		stats.Source = source
	}
	if dest, ok := data["destination"].(string); ok && dest != "" {
		stats.Destination = dest
	}
	if domain, ok := data["domain"].(string); ok && domain != "" {
		stats.Domain = domain
	}
	if network, ok := data["network"].(string); ok && network != "" {
		stats.Network = network
	}
	if inbound, ok := data["inbound"].(string); ok && inbound != "" {
		stats.InboundTag = inbound
	}
	if outbound, ok := data["outbound"].(string); ok && outbound != "" {
		stats.OutboundTag = outbound
	}
	if errStr, ok := data["error"].(string); ok && errStr != "" {
		stats.Error = errStr
	}

	phase := direction + "_" + action
	stats.addPhase(phase)

	if action == "close" || action == "error" {
		stats.ClosedAt = entry.Timestamp
	}
}

func (t *LifecycleTracker) handleTransferEvent(stats *ConnectionStats, entry LogEntry) {
	data := entry.Event.Data
	stats.addPhase("transfer")
	if bytes, ok := data["bytes"].(int64); ok {
		dir, _ := data["direction"].(string)
		if dir == "upload" {
			stats.UploadBytes += bytes
		} else {
			stats.DownloadBytes += bytes
		}
	}
}

func (s *ConnectionStats) addPhase(phase string) {
	for _, p := range s.Phases {
		if p == phase {
			return
		}
	}
	s.Phases = append(s.Phases, phase)
}

func (s *ConnectionStats) isClosed() bool {
	return !s.ClosedAt.IsZero()
}

// Stop stops the cleanup goroutine
func (t *LifecycleTracker) Stop() {
	close(t.stopCh)
}

// cleanupLoop periodically evicts connections that have exceeded the TTL
func (t *LifecycleTracker) cleanupLoop() {
	ticker := time.NewTicker(t.ttl / 2)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			t.evictExpired()
		case <-t.stopCh:
			return
		}
	}
}

func (t *LifecycleTracker) evictExpired() {
	now := time.Now()
	t.mu.Lock()
	var expired []*ConnectionStats
	for id, stats := range t.connections {
		if now.Sub(stats.StartedAt) > t.ttl {
			expired = append(expired, stats)
			delete(t.connections, id)
		}
	}
	t.mu.Unlock()

	if t.onComplete != nil {
		for _, stats := range expired {
			t.onComplete(stats)
		}
	}
}

// ActiveCount returns the number of currently tracked connections
func (t *LifecycleTracker) ActiveCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.connections)
}
