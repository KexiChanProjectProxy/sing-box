package log

import (
	"context"
	"errors"
	"time"

	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/observable"
)

var _ ObservableFactory = (*multiOutputFactory)(nil)

// multiOutputFactory implements a factory that writes to multiple outputs
type multiOutputFactory struct {
	ctx               context.Context
	outputs           []Output
	platformWriter    PlatformWriter
	platformFormatter Formatter
	needObservable    bool
	level             Level
	subscriber        *observable.Subscriber[Entry]
	observer          *observable.Observer[Entry]
}

// NewMultiOutputFactory creates a new multi-output factory
func NewMultiOutputFactory(
	ctx context.Context,
	outputs []Output,
	platformFormatter Formatter,
	platformWriter PlatformWriter,
	needObservable bool,
) ObservableFactory {
	factory := &multiOutputFactory{
		ctx:               ctx,
		outputs:           outputs,
		platformFormatter: platformFormatter,
		platformWriter:    platformWriter,
		needObservable:    needObservable,
		level:             LevelTrace,
		subscriber:        observable.NewSubscriber[Entry](128),
	}
	if needObservable {
		factory.observer = observable.NewObserver[Entry](factory.subscriber, 64)
	}
	return factory
}

// Start initializes all outputs
func (f *multiOutputFactory) Start() error {
	for _, output := range f.outputs {
		if starter, ok := output.(interface{ Start() error }); ok {
			if err := starter.Start(); err != nil {
				return err
			}
		}
	}
	return nil
}

// Close closes all outputs
func (f *multiOutputFactory) Close() error {
	var errors []error
	for _, output := range f.outputs {
		if err := output.Close(); err != nil {
			errors = append(errors, err)
		}
	}
	if err := f.subscriber.Close(); err != nil {
		errors = append(errors, err)
	}
	if len(errors) > 0 {
		return errors[0]
	}
	return nil
}

// Level returns the current log level
func (f *multiOutputFactory) Level() Level {
	return f.level
}

// SetLevel sets the log level
func (f *multiOutputFactory) SetLevel(level Level) {
	f.level = level
}

// Logger returns a logger without a tag
func (f *multiOutputFactory) Logger() ContextLogger {
	return f.NewLogger("")
}

// NewLogger returns a logger with a tag
func (f *multiOutputFactory) NewLogger(tag string) ContextLogger {
	return &multiOutputLogger{
		factory: f,
		tag:     tag,
	}
}

// Subscribe subscribes to log entries (for observable pattern)
func (f *multiOutputFactory) Subscribe() (subscription observable.Subscription[Entry], done <-chan struct{}, err error) {
	if f.observer == nil {
		return nil, nil, errors.New("observer not initialized")
	}
	return f.observer.Subscribe()
}

// UnSubscribe unsubscribes from log entries
func (f *multiOutputFactory) UnSubscribe(sub observable.Subscription[Entry]) {
	if f.observer != nil {
		f.observer.UnSubscribe(sub)
	}
}

// multiOutputLogger implements ContextLogger for the multi-output factory
type multiOutputLogger struct {
	factory *multiOutputFactory
	tag     string
}

// Log logs a message with the given level
func (l *multiOutputLogger) Log(ctx context.Context, level Level, args []any) {
	l.LogWithEvent(ctx, level, nil, args)
}

// LogWithEvent logs a message with the given level and structured event
func (l *multiOutputLogger) LogWithEvent(ctx context.Context, level Level, event *StructuredEvent, args []any) {
	// Apply level override from context
	level = OverrideLevelFromContext(level, ctx)
	if level > l.factory.level {
		return
	}

	nowTime := time.Now()
	message := F.ToString(args...)

	// Build log entry with metadata
	entry := l.buildLogEntry(ctx, level, message, nowTime)

	// Add structured event if provided
	if event != nil {
		entry.Event = event
	}

	// Write to all outputs (non-blocking)
	for _, output := range l.factory.outputs {
		go output.Write(entry)
	}

	// Emit to observable if needed
	if l.factory.needObservable {
		l.factory.subscriber.Emit(Entry{level, message})
	}

	// Write to platform writer if needed
	if l.factory.platformWriter != nil {
		platformMessage := l.factory.platformFormatter.Format(ctx, level, l.tag, message, nowTime)
		l.factory.platformWriter.WriteMessage(level, platformMessage)
	}
}

// buildLogEntry builds a LogEntry from context and message
func (l *multiOutputLogger) buildLogEntry(ctx context.Context, level Level, message string, timestamp time.Time) LogEntry {
	entry := LogEntry{
		Timestamp: timestamp,
		Level:     level,
		Message:   message,
		Tag:       l.tag,
		Metadata:  make(map[string]interface{}),
	}

	// Extract connection ID
	if id, hasId := IDFromContext(ctx); hasId {
		entry.ConnectionID = id.ID
		entry.ConnectionDuration = time.Since(id.CreatedAt)
	}

	// Note: InboundContext metadata is not automatically extracted to avoid import cycles.
	// Use structured events (ConnectionEvent, DNSEvent, etc.) to include rich metadata.

	return entry
}

// Convenience methods

func (l *multiOutputLogger) Trace(args ...any) {
	l.TraceContext(context.Background(), args...)
}

func (l *multiOutputLogger) Debug(args ...any) {
	l.DebugContext(context.Background(), args...)
}

func (l *multiOutputLogger) Info(args ...any) {
	l.InfoContext(context.Background(), args...)
}

func (l *multiOutputLogger) Warn(args ...any) {
	l.WarnContext(context.Background(), args...)
}

func (l *multiOutputLogger) Error(args ...any) {
	l.ErrorContext(context.Background(), args...)
}

func (l *multiOutputLogger) Fatal(args ...any) {
	l.FatalContext(context.Background(), args...)
}

func (l *multiOutputLogger) Panic(args ...any) {
	l.PanicContext(context.Background(), args...)
}

func (l *multiOutputLogger) TraceContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelTrace, args)
}

func (l *multiOutputLogger) DebugContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelDebug, args)
}

func (l *multiOutputLogger) InfoContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelInfo, args)
}

func (l *multiOutputLogger) WarnContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelWarn, args)
}

func (l *multiOutputLogger) ErrorContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelError, args)
}

func (l *multiOutputLogger) FatalContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelFatal, args)
}

func (l *multiOutputLogger) PanicContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelPanic, args)
}
