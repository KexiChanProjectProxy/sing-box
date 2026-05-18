package log

// PlatformWriter is an interface for platform-specific log writing.
type PlatformWriter interface {
	WriteMessage(level Level, message string)
}
