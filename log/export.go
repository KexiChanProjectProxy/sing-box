package log

import (
	"context"
	"os"
)

var std StructuredLogger

func init() {
	std = NewDefaultFactory(
		context.Background(),
		Formatter{},
		os.Stderr,
		"",
		nil,
		false,
		"json",
	).Logger()
}

// StdLogger returns the standard logger.
func StdLogger() StructuredLogger {
	return std
}

// SetStdLogger sets the standard logger.
func SetStdLogger(logger StructuredLogger) {
	std = logger
}

// Trace logs a trace message.
func Trace(args ...any) {
	std.Trace(args...)
}

// Debug logs a debug message.
func Debug(args ...any) {
	std.Debug(args...)
}

// Info logs an info message.
func Info(args ...any) {
	std.Info(args...)
}

// Warn logs a warning message.
func Warn(args ...any) {
	std.Warn(args...)
}

// Error logs an error message.
func Error(args ...any) {
	std.Error(args...)
}

// Fatal logs a fatal message.
func Fatal(args ...any) {
	std.Fatal(args...)
}

// Panic logs a panic message.
func Panic(args ...any) {
	std.Panic(args...)
}

// TraceContext logs a trace message with context.
func TraceContext(ctx context.Context, args ...any) {
	std.TraceContext(ctx, args...)
}

// DebugContext logs a debug message with context.
func DebugContext(ctx context.Context, args ...any) {
	std.DebugContext(ctx, args...)
}

// InfoContext logs an info message with context.
func InfoContext(ctx context.Context, args ...any) {
	std.InfoContext(ctx, args...)
}

// WarnContext logs a warning message with context.
func WarnContext(ctx context.Context, args ...any) {
	std.WarnContext(ctx, args...)
}

// ErrorContext logs an error message with context.
func ErrorContext(ctx context.Context, args ...any) {
	std.ErrorContext(ctx, args...)
}

// FatalContext logs a fatal message with context.
func FatalContext(ctx context.Context, args ...any) {
	std.FatalContext(ctx, args...)
}

// PanicContext logs a panic message with context.
func PanicContext(ctx context.Context, args ...any) {
	std.PanicContext(ctx, args...)
}

// TraceEvent logs a structured trace-level event with the given event name, message, and fields.
func TraceEvent(event string, message string, fields ...Field) {
	std.TraceEvent(event, message, fields...)
}

// DebugEvent logs a structured debug-level event with the given event name, message, and fields.
func DebugEvent(event string, message string, fields ...Field) {
	std.DebugEvent(event, message, fields...)
}

// InfoEvent logs a structured info-level event with the given event name, message, and fields.
func InfoEvent(event string, message string, fields ...Field) {
	std.InfoEvent(event, message, fields...)
}

// WarnEvent logs a structured warn-level event with the given event name, message, and fields.
func WarnEvent(event string, message string, fields ...Field) {
	std.WarnEvent(event, message, fields...)
}

// ErrorEvent logs a structured error-level event with the given event name, message, and fields.
func ErrorEvent(event string, message string, fields ...Field) {
	std.ErrorEvent(event, message, fields...)
}

// FatalEvent logs a structured fatal-level event with the given event name, message, and fields.
func FatalEvent(event string, message string, fields ...Field) {
	std.FatalEvent(event, message, fields...)
}

// PanicEvent logs a structured panic-level event with the given event name, message, and fields.
func PanicEvent(event string, message string, fields ...Field) {
	std.PanicEvent(event, message, fields...)
}

// TraceEventContext logs a structured trace-level event with context, event name, message, and fields.
func TraceEventContext(ctx context.Context, event string, message string, fields ...Field) {
	std.TraceEventContext(ctx, event, message, fields...)
}

// DebugEventContext logs a structured debug-level event with context, event name, message, and fields.
func DebugEventContext(ctx context.Context, event string, message string, fields ...Field) {
	std.DebugEventContext(ctx, event, message, fields...)
}

// InfoEventContext logs a structured info-level event with context, event name, message, and fields.
func InfoEventContext(ctx context.Context, event string, message string, fields ...Field) {
	std.InfoEventContext(ctx, event, message, fields...)
}

// WarnEventContext logs a structured warn-level event with context, event name, message, and fields.
func WarnEventContext(ctx context.Context, event string, message string, fields ...Field) {
	std.WarnEventContext(ctx, event, message, fields...)
}

// ErrorEventContext logs a structured error-level event with context, event name, message, and fields.
func ErrorEventContext(ctx context.Context, event string, message string, fields ...Field) {
	std.ErrorEventContext(ctx, event, message, fields...)
}

// FatalEventContext logs a structured fatal-level event with context, event name, message, and fields.
func FatalEventContext(ctx context.Context, event string, message string, fields ...Field) {
	std.FatalEventContext(ctx, event, message, fields...)
}

// PanicEventContext logs a structured panic-level event with context, event name, message, and fields.
func PanicEventContext(ctx context.Context, event string, message string, fields ...Field) {
	std.PanicEventContext(ctx, event, message, fields...)
}
