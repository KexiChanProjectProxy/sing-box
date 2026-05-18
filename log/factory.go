package log

import (
	"context"

	commonLogger "github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/common/observable"
)

// Logger is the base logger interface.
type Logger commonLogger.Logger

// StructuredLogger extends ContextLogger with structured event methods (*Event, *EventContext) that accept an event name, message, and typed Field parameters.
type StructuredLogger interface {
	commonLogger.ContextLogger
	TraceEvent(event string, message string, fields ...Field)
	DebugEvent(event string, message string, fields ...Field)
	InfoEvent(event string, message string, fields ...Field)
	WarnEvent(event string, message string, fields ...Field)
	ErrorEvent(event string, message string, fields ...Field)
	FatalEvent(event string, message string, fields ...Field)
	PanicEvent(event string, message string, fields ...Field)
	TraceEventContext(ctx context.Context, event string, message string, fields ...Field)
	DebugEventContext(ctx context.Context, event string, message string, fields ...Field)
	InfoEventContext(ctx context.Context, event string, message string, fields ...Field)
	WarnEventContext(ctx context.Context, event string, message string, fields ...Field)
	ErrorEventContext(ctx context.Context, event string, message string, fields ...Field)
	FatalEventContext(ctx context.Context, event string, message string, fields ...Field)
	PanicEventContext(ctx context.Context, event string, message string, fields ...Field)
}

// Factory creates structured loggers. All loggers returned by the factory implement StructuredLogger.
type Factory interface {
	Start() error
	Close() error
	Level() Level
	SetLevel(level Level)
	Logger() StructuredLogger
	NewLogger(tag string) StructuredLogger
}

// ObservableFactory extends Factory with observable log subscription.
type ObservableFactory interface {
	Factory
	observable.Observable[Entry]
}

// Entry represents a log entry for observable subscription.
type Entry struct {
	Level   Level
	Message string
}
