package log

import (
	"context"
)

// StructuredLogger provides methods for logging with structured events
type StructuredLogger interface {
	ContextLogger

	// InfoContextWithEvent logs info with structured event data
	InfoContextWithEvent(ctx context.Context, event interface{}, args ...any)
	// DebugContextWithEvent logs debug with structured event data
	DebugContextWithEvent(ctx context.Context, event interface{}, args ...any)
	// ErrorContextWithEvent logs error with structured event data
	ErrorContextWithEvent(ctx context.Context, event interface{}, args ...any)
}

// WithConnectionEvent creates a log entry with connection event
func WithConnectionEvent(logger ContextLogger, ctx context.Context, level Level, event *ConnectionEvent, args ...any) {
	if ml, ok := logger.(*multiOutputLogger); ok {
		ml.LogWithEvent(ctx, level, event.ToStructuredEvent(), args)
	} else {
		// Fallback to regular logging
		logger.Log(ctx, level, args)
	}
}

// WithDNSEvent creates a log entry with DNS event
func WithDNSEvent(logger ContextLogger, ctx context.Context, level Level, event *DNSEvent, args ...any) {
	if ml, ok := logger.(*multiOutputLogger); ok {
		ml.LogWithEvent(ctx, level, event.ToStructuredEvent(), args)
	} else {
		// Fallback to regular logging
		logger.Log(ctx, level, args)
	}
}

// WithRouterMatchEvent creates a log entry with router match event
func WithRouterMatchEvent(logger ContextLogger, ctx context.Context, level Level, event *RouterMatchEvent, args ...any) {
	if ml, ok := logger.(*multiOutputLogger); ok {
		ml.LogWithEvent(ctx, level, event.ToStructuredEvent(), args)
	} else {
		// Fallback to regular logging
		logger.Log(ctx, level, args)
	}
}

// WithTransferEvent creates a log entry with transfer event
func WithTransferEvent(logger ContextLogger, ctx context.Context, level Level, event *TransferEvent, args ...any) {
	if ml, ok := logger.(*multiOutputLogger); ok {
		ml.LogWithEvent(ctx, level, event.ToStructuredEvent(), args)
	} else {
		// Fallback to regular logging
		logger.Log(ctx, level, args)
	}
}

// ToStructuredEvent converts ConnectionEvent to StructuredEvent
func (e *ConnectionEvent) ToStructuredEvent() *StructuredEvent {
	return &StructuredEvent{
		Type: EventTypeConnection,
		Data: e.ToMap(),
	}
}

// ToStructuredEvent converts DNSEvent to StructuredEvent
func (e *DNSEvent) ToStructuredEvent() *StructuredEvent {
	return &StructuredEvent{
		Type: EventTypeDNS,
		Data: e.ToMap(),
	}
}

// ToStructuredEvent converts RouterMatchEvent to StructuredEvent
func (e *RouterMatchEvent) ToStructuredEvent() *StructuredEvent {
	return &StructuredEvent{
		Type: EventTypeRouterMatch,
		Data: e.ToMap(),
	}
}

// ToStructuredEvent converts TransferEvent to StructuredEvent
func (e *TransferEvent) ToStructuredEvent() *StructuredEvent {
	return &StructuredEvent{
		Type: EventTypeTransfer,
		Data: e.ToMap(),
	}
}
