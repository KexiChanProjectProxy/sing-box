package log

import (
	"sync/atomic"
)

// MetricsSubscriber collects aggregate metrics from structured log events
type MetricsSubscriber struct {
	// Connection metrics
	ConnectionsTotal  atomic.Int64
	ConnectionsActive atomic.Int64
	ConnectionsFailed atomic.Int64

	// Transfer metrics
	UploadBytesTotal   atomic.Int64
	DownloadBytesTotal atomic.Int64

	// DNS metrics
	DNSQueriesTotal  atomic.Int64
	DNSCacheHits     atomic.Int64
	DNSFailures      atomic.Int64

	// Health check metrics
	HealthChecksTotal  atomic.Int64
	HealthChecksFailed atomic.Int64

	// Event counts by type
	eventCounts [16]atomic.Int64
	eventTypes  [16]EventType
	eventCount  int
}

// NewMetricsSubscriber creates a new MetricsSubscriber
func NewMetricsSubscriber() *MetricsSubscriber {
	m := &MetricsSubscriber{}
	types := []EventType{
		EventTypeConnection, EventTypeDNS, EventTypeRouterMatch,
		EventTypeTransfer, EventTypeProcessInfo, EventTypeHealthCheck,
		EventTypeNetworkState, EventTypeRuleSet, EventTypeTLS,
		EventTypeComponentLifecycle, EventTypeServerLifecycle,
		EventTypeTransportProtocol, EventTypeHTTPRoute, EventTypeAuth,
		EventTypeService,
	}
	for i, t := range types {
		if i >= len(m.eventTypes) {
			break
		}
		m.eventTypes[i] = t
	}
	m.eventCount = len(types)
	return m
}

// HandleEvent implements EventSubscriber
func (m *MetricsSubscriber) HandleEvent(entry LogEntry) {
	if entry.Event == nil {
		return
	}

	// Count by event type
	for i := 0; i < m.eventCount; i++ {
		if m.eventTypes[i] == entry.Event.Type {
			m.eventCounts[i].Add(1)
			break
		}
	}

	switch entry.Event.Type {
	case EventTypeConnection:
		m.handleConnection(entry.Event.Data)
	case EventTypeDNS:
		m.handleDNS(entry.Event.Data)
	case EventTypeHealthCheck:
		m.handleHealthCheck(entry.Event.Data)
	case EventTypeTransfer:
		m.handleTransfer(entry.Event.Data)
	}
}

func (m *MetricsSubscriber) handleConnection(data map[string]interface{}) {
	action, _ := data["action"].(string)
	switch action {
	case "start":
		m.ConnectionsTotal.Add(1)
		m.ConnectionsActive.Add(1)
	case "close":
		m.ConnectionsActive.Add(-1)
	case "error":
		m.ConnectionsFailed.Add(1)
		m.ConnectionsActive.Add(-1)
	}
}

func (m *MetricsSubscriber) handleDNS(data map[string]interface{}) {
	m.DNSQueriesTotal.Add(1)
	if cached, _ := data["cached"].(bool); cached {
		m.DNSCacheHits.Add(1)
	}
	if errStr, _ := data["error"].(string); errStr != "" {
		m.DNSFailures.Add(1)
	}
}

func (m *MetricsSubscriber) handleHealthCheck(data map[string]interface{}) {
	m.HealthChecksTotal.Add(1)
	action, _ := data["action"].(string)
	if action == "failed" || action == "timeout" {
		m.HealthChecksFailed.Add(1)
	}
}

func (m *MetricsSubscriber) handleTransfer(data map[string]interface{}) {
	if bytes, ok := data["bytes"].(int64); ok && bytes > 0 {
		dir, _ := data["direction"].(string)
		if dir == "upload" {
			m.UploadBytesTotal.Add(bytes)
		} else {
			m.DownloadBytesTotal.Add(bytes)
		}
	}
}

// Snapshot returns a map of all current metric values
func (m *MetricsSubscriber) Snapshot() map[string]int64 {
	snap := map[string]int64{
		"connections_total":    m.ConnectionsTotal.Load(),
		"connections_active":   m.ConnectionsActive.Load(),
		"connections_failed":   m.ConnectionsFailed.Load(),
		"upload_bytes_total":   m.UploadBytesTotal.Load(),
		"download_bytes_total": m.DownloadBytesTotal.Load(),
		"dns_queries_total":    m.DNSQueriesTotal.Load(),
		"dns_cache_hits":       m.DNSCacheHits.Load(),
		"dns_failures":         m.DNSFailures.Load(),
		"health_checks_total":  m.HealthChecksTotal.Load(),
		"health_checks_failed": m.HealthChecksFailed.Load(),
	}
	for i := 0; i < m.eventCount; i++ {
		if m.eventTypes[i] != "" {
			snap["event_count_"+string(m.eventTypes[i])] = m.eventCounts[i].Load()
		}
	}
	return snap
}
