package log

import (
	"context"
	"io"
	"os"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing/common"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/observable"
	"github.com/sagernet/sing/service/filemanager"
)

var _ Factory = (*defaultFactory)(nil)

type defaultFactory struct {
	ctx               context.Context
	formatter         Formatter
	platformFormatter Formatter
	writer            io.Writer
	file              *os.File
	filePath          string
	platformWriters   atomic.Pointer[[]PlatformWriter]
	needObservable    bool
	format            string
	level             Level
	subscriber        *observable.Subscriber[Entry]
	observer          *observable.Observer[Entry]
}

// NewDefaultFactory creates a new default factory for structured logging.
func NewDefaultFactory(
	ctx context.Context,
	formatter Formatter,
	writer io.Writer,
	filePath string,
	platformWriter PlatformWriter,
	needObservable bool,
	format string,
) ObservableFactory {
	factory := &defaultFactory{
		ctx:       ctx,
		formatter: formatter,
		platformFormatter: Formatter{
			BaseTime:         formatter.BaseTime,
			DisableLineBreak: true,
		},
		writer:         writer,
		filePath:       filePath,
		needObservable: needObservable,
		format:         format,
		level:          LevelTrace,
		subscriber:     observable.NewSubscriber[Entry](128),
	}
	if platformWriter != nil {
		factory.platformWriters.Store(&[]PlatformWriter{platformWriter})
	}
	/*if platformWriter != nil {
		factory.platformFormatter.DisableColors = platformWriter.DisableColors()
	}*/
	if needObservable {
		factory.observer = observable.NewObserver[Entry](factory.subscriber, 64)
	}
	return factory
}

func (f *defaultFactory) Start() error {
	if f.filePath != "" {
		logFile, err := filemanager.OpenFile(f.ctx, f.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		f.writer = logFile
		f.file = logFile
	}
	return nil
}

func (f *defaultFactory) Close() error {
	return common.Close(
		common.PtrOrNil(f.file),
		f.subscriber,
	)
}

func (f *defaultFactory) Level() Level {
	return f.level
}

func (f *defaultFactory) SetLevel(level Level) {
	f.level = level
}

func (f *defaultFactory) AttachPlatformWriter(writer PlatformWriter) {
	writers := append(f.loadPlatformWriters(), writer)
	f.platformWriters.Store(&writers)
}

func (f *defaultFactory) loadPlatformWriters() []PlatformWriter {
	writers := f.platformWriters.Load()
	if writers == nil {
		return nil
	}
	return *writers
}

func (f *defaultFactory) Logger() StructuredLogger {
	return f.NewLogger("")
}

func (f *defaultFactory) NewLogger(tag string) StructuredLogger {
	return &observableLogger{f, tag}
}

func (f *defaultFactory) Subscribe() (subscription observable.Subscription[Entry], done <-chan struct{}, err error) {
	return f.observer.Subscribe()
}

func (f *defaultFactory) UnSubscribe(sub observable.Subscription[Entry]) {
	f.observer.UnSubscribe(sub)
}

var _ StructuredLogger = (*observableLogger)(nil)

type observableLogger struct {
	*defaultFactory
	tag string
}

func (l *observableLogger) Log(ctx context.Context, level Level, args []any) {
	level = OverrideLevelFromContext(level, ctx)
	platformWriters := l.loadPlatformWriters()
	if level > l.level && len(platformWriters) == 0 && !l.needObservable {
		return
	}
	nowTime := time.Now()
	messageRaw := F.ToString(args...)
	rec := l.recordFromContext(ctx, level, "", messageRaw, nowTime, nil)
	if level <= l.level {
		var message string
		if l.format == "json" {
			message = l.formatter.FormatRecordJSON(rec)
		} else {
			message = l.formatter.FormatRecord(rec)
		}
		l.writer.Write([]byte(message))
		if l.needObservable {
			l.subscriber.Emit(Entry{level, messageRaw})
		}
		if level == LevelPanic {
			panic(message)
		}
		if level == LevelFatal {
			os.Exit(1)
		}
	}
	if len(platformWriters) > 0 {
		formatted := l.platformFormatter.FormatRecord(rec)
		for _, platformWriter := range platformWriters {
			platformWriter.WriteMessage(level, formatted)
		}
	}
}

func (l *observableLogger) Trace(args ...any) {
	l.TraceContext(context.Background(), args...)
}

func (l *observableLogger) Debug(args ...any) {
	l.DebugContext(context.Background(), args...)
}

func (l *observableLogger) Info(args ...any) {
	l.InfoContext(context.Background(), args...)
}

func (l *observableLogger) Warn(args ...any) {
	l.WarnContext(context.Background(), args...)
}

func (l *observableLogger) Error(args ...any) {
	l.ErrorContext(context.Background(), args...)
}

func (l *observableLogger) Fatal(args ...any) {
	l.FatalContext(context.Background(), args...)
}

func (l *observableLogger) Panic(args ...any) {
	l.PanicContext(context.Background(), args...)
}

func (l *observableLogger) TraceContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelTrace, args)
}

func (l *observableLogger) DebugContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelDebug, args)
}

func (l *observableLogger) InfoContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelInfo, args)
}

func (l *observableLogger) WarnContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelWarn, args)
}

func (l *observableLogger) ErrorContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelError, args)
}

func (l *observableLogger) FatalContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelFatal, args)
}

func (l *observableLogger) PanicContext(ctx context.Context, args ...any) {
	l.Log(ctx, LevelPanic, args)
}

func (l *observableLogger) TraceEvent(event string, message string, fields ...Field) {
	l.TraceEventContext(context.Background(), event, message, fields...)
}

func (l *observableLogger) DebugEvent(event string, message string, fields ...Field) {
	l.DebugEventContext(context.Background(), event, message, fields...)
}

func (l *observableLogger) InfoEvent(event string, message string, fields ...Field) {
	l.InfoEventContext(context.Background(), event, message, fields...)
}

func (l *observableLogger) WarnEvent(event string, message string, fields ...Field) {
	l.WarnEventContext(context.Background(), event, message, fields...)
}

func (l *observableLogger) ErrorEvent(event string, message string, fields ...Field) {
	l.ErrorEventContext(context.Background(), event, message, fields...)
}

func (l *observableLogger) FatalEvent(event string, message string, fields ...Field) {
	l.FatalEventContext(context.Background(), event, message, fields...)
}

func (l *observableLogger) PanicEvent(event string, message string, fields ...Field) {
	l.PanicEventContext(context.Background(), event, message, fields...)
}

func (l *observableLogger) TraceEventContext(ctx context.Context, event string, message string, fields ...Field) {
	l.logEvent(ctx, LevelTrace, event, message, fields)
}

func (l *observableLogger) DebugEventContext(ctx context.Context, event string, message string, fields ...Field) {
	l.logEvent(ctx, LevelDebug, event, message, fields)
}

func (l *observableLogger) InfoEventContext(ctx context.Context, event string, message string, fields ...Field) {
	l.logEvent(ctx, LevelInfo, event, message, fields)
}

func (l *observableLogger) WarnEventContext(ctx context.Context, event string, message string, fields ...Field) {
	l.logEvent(ctx, LevelWarn, event, message, fields)
}

func (l *observableLogger) ErrorEventContext(ctx context.Context, event string, message string, fields ...Field) {
	l.logEvent(ctx, LevelError, event, message, fields)
}

func (l *observableLogger) FatalEventContext(ctx context.Context, event string, message string, fields ...Field) {
	l.logEvent(ctx, LevelFatal, event, message, fields)
}

func (l *observableLogger) PanicEventContext(ctx context.Context, event string, message string, fields ...Field) {
	l.logEvent(ctx, LevelPanic, event, message, fields)
}

func (l *observableLogger) logEvent(ctx context.Context, level Level, event string, message string, fields []Field) {
	level = OverrideLevelFromContext(level, ctx)
	platformWriters := l.loadPlatformWriters()
	if level > l.level && len(platformWriters) == 0 && !l.needObservable {
		return
	}
	nowTime := time.Now()
	rec := l.recordFromContext(ctx, level, event, message, nowTime, fields)
	if level <= l.level {
		var formatted string
		if l.format == "json" {
			formatted = l.formatter.FormatRecordJSON(rec)
		} else {
			formatted = l.formatter.FormatRecord(rec)
		}
		l.writer.Write([]byte(formatted))
		if l.needObservable {
			l.subscriber.Emit(Entry{level, message})
		}
		if level == LevelPanic {
			panic(formatted)
		}
		if level == LevelFatal {
			os.Exit(1)
		}
	}
	if len(platformWriters) > 0 {
		formatted := l.platformFormatter.FormatRecord(rec)
		for _, platformWriter := range platformWriters {
			platformWriter.WriteMessage(level, formatted)
		}
	}
}

func (l *observableLogger) recordFromContext(ctx context.Context, level Level, event string, message string, timestamp time.Time, fields []Field) Record {
	rec := Record{Level: level, Message: message, Tag: l.tag, Event: event, Timestamp: timestamp, Fields: fields}
	if ctx != nil {
		if id, ok := IDFromContext(ctx); ok {
			rec.ContextID = id.ID
			rec.ContextAgeMs = float64(time.Since(id.CreatedAt)) / float64(time.Millisecond)
		}
	}
	return rec
}
