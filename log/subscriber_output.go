package log

// LogOutputSubscriber wraps an Output as an EventSubscriber, allowing
// outputs to be dynamically registered through the event bus.
// Only entries that have an Event are forwarded (structured events only).
type LogOutputSubscriber struct {
	output Output
}

// NewLogOutputSubscriber wraps an Output as an EventSubscriber
func NewLogOutputSubscriber(output Output) *LogOutputSubscriber {
	return &LogOutputSubscriber{output: output}
}

// HandleEvent implements EventSubscriber
func (s *LogOutputSubscriber) HandleEvent(entry LogEntry) {
	if entry.Event == nil {
		return
	}
	// Best-effort: ignore write errors (output may be a file or HTTP sink)
	_ = s.output.Write(entry)
}
