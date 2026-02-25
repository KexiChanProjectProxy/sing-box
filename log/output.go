package log

import (
	"context"
	"time"
)

// MetadataExtractor is a callback function to extract metadata from context
// This solves the import cycle: log cannot import adapter, but adapter can register an extractor
type MetadataExtractor func(ctx context.Context) map[string]interface{}

// LogEntry represents a structured log entry with all metadata
type LogEntry struct {
	Timestamp          time.Time
	Level              Level
	Message            string
	Tag                string
	ConnectionID       uint32
	ConnectionDuration time.Duration
	Metadata           map[string]interface{}
	Event              *StructuredEvent // Structured event data for connection/DNS/router logs
}

// Output interface for different output destinations
type Output interface {
	// Write writes a log entry to the output
	Write(entry LogEntry) error
	// Close flushes and closes the output
	Close() error
}
