package log

import (
	"context"
	"os"

	"github.com/sagernet/sing/common/observable"
)

var _ ObservableFactory = (*nopFactory)(nil)

type nopFactory struct{}

// NewNOPFactory returns a factory that discards all log output.
func NewNOPFactory() ObservableFactory {
	return (*nopFactory)(nil)
}

func (f *nopFactory) Start() error {
	return nil
}

func (f *nopFactory) Close() error {
	return nil
}

func (f *nopFactory) Level() Level {
	return LevelTrace
}

func (f *nopFactory) SetLevel(level Level) {
}

func (f *nopFactory) Logger() StructuredLogger {
	return f
}

func (f *nopFactory) NewLogger(tag string) StructuredLogger {
	return f
}

func (f *nopFactory) Trace(args ...any) {
}

func (f *nopFactory) Debug(args ...any) {
}

func (f *nopFactory) Info(args ...any) {
}

func (f *nopFactory) Warn(args ...any) {
}

func (f *nopFactory) Error(args ...any) {
}

func (f *nopFactory) Fatal(args ...any) {
}

func (f *nopFactory) Panic(args ...any) {
}

func (f *nopFactory) TraceContext(ctx context.Context, args ...any) {
}

func (f *nopFactory) DebugContext(ctx context.Context, args ...any) {
}

func (f *nopFactory) InfoContext(ctx context.Context, args ...any) {
}

func (f *nopFactory) WarnContext(ctx context.Context, args ...any) {
}

func (f *nopFactory) ErrorContext(ctx context.Context, args ...any) {
}

func (f *nopFactory) FatalContext(ctx context.Context, args ...any) {
}

func (f *nopFactory) PanicContext(ctx context.Context, args ...any) {
}

func (f *nopFactory) TraceEvent(event string, message string, fields ...Field) {
}

func (f *nopFactory) DebugEvent(event string, message string, fields ...Field) {
}

func (f *nopFactory) InfoEvent(event string, message string, fields ...Field) {
}

func (f *nopFactory) WarnEvent(event string, message string, fields ...Field) {
}

func (f *nopFactory) ErrorEvent(event string, message string, fields ...Field) {
}

func (f *nopFactory) FatalEvent(event string, message string, fields ...Field) {
}

func (f *nopFactory) PanicEvent(event string, message string, fields ...Field) {
}

func (f *nopFactory) TraceEventContext(ctx context.Context, event string, message string, fields ...Field) {
}

func (f *nopFactory) DebugEventContext(ctx context.Context, event string, message string, fields ...Field) {
}

func (f *nopFactory) InfoEventContext(ctx context.Context, event string, message string, fields ...Field) {
}

func (f *nopFactory) WarnEventContext(ctx context.Context, event string, message string, fields ...Field) {
}

func (f *nopFactory) ErrorEventContext(ctx context.Context, event string, message string, fields ...Field) {
}

func (f *nopFactory) FatalEventContext(ctx context.Context, event string, message string, fields ...Field) {
}

func (f *nopFactory) PanicEventContext(ctx context.Context, event string, message string, fields ...Field) {
}

func (f *nopFactory) Subscribe() (subscription observable.Subscription[Entry], done <-chan struct{}, err error) {
	return nil, nil, os.ErrInvalid
}

func (f *nopFactory) UnSubscribe(subscription observable.Subscription[Entry]) {
}
