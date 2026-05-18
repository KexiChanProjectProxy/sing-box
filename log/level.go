package log

import (
	E "github.com/sagernet/sing/common/exceptions"
)

// Level represents the severity level of a log entry.
type Level = uint8

// Log level constants.
const (
	LevelPanic Level = iota
	LevelFatal
	LevelError
	LevelWarn
	LevelInfo
	LevelDebug
	LevelTrace
)

// FormatLevel returns the string representation of a log level.
func FormatLevel(level Level) string {
	switch level {
	case LevelTrace:
		return "trace"
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	case LevelFatal:
		return "fatal"
	case LevelPanic:
		return "panic"
	default:
		return "unknown"
	}
}

// ParseLevel parses a string into a log level.
func ParseLevel(level string) (Level, error) {
	switch level {
	case "trace":
		return LevelTrace, nil
	case "debug":
		return LevelDebug, nil
	case "info":
		return LevelInfo, nil
	case "warn", "warning":
		return LevelWarn, nil
	case "error":
		return LevelError, nil
	case "fatal":
		return LevelFatal, nil
	case "panic":
		return LevelPanic, nil
	default:
		return LevelTrace, E.New("unknown log level: ", level)
	}
}
