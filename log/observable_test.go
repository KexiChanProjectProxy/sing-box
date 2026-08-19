package log

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func newTestFactory(buf *bytes.Buffer, format string) ObservableFactory {
	return NewDefaultFactory(context.Background(), Formatter{}, buf, "", nil, false, format)
}

func TestStructuredLoggerEmitsFields(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestFactory(&buf, "json").NewLogger("test")
	logger.InfoEvent("login", "accepted", String("UserName", "alice"), Int("attempt", 1))
	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["logger"] != "test" || decoded["event"] != "login" || decoded["message"] != "accepted" || decoded["user_name"] != "alice" || decoded["attempt"].(float64) != 1 {
		t.Fatalf("missing structured fields: %#v", decoded)
	}
}

func TestVariadicLoggerCompatibilityMessageOnly(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestFactory(&buf, "json").NewLogger("")
	logger.Info("a", 1)
	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["message"] != "a1" {
		t.Fatalf("variadic formatting changed: %#v", decoded)
	}
}
func TestStructuredLoggerAllLevels(t *testing.T) {
	for _, level := range []Level{LevelTrace, LevelDebug, LevelInfo, LevelWarn, LevelError, LevelPanic} {
		var buf bytes.Buffer
		logger := newTestFactory(&buf, "json").NewLogger("")
		func() {
			defer func() { _ = recover() }()
			switch level {
			case LevelTrace:
				logger.TraceEvent("event", "message")
			case LevelDebug:
				logger.DebugEvent("event", "message")
			case LevelInfo:
				logger.InfoEvent("event", "message")
			case LevelWarn:
				logger.WarnEvent("event", "message")
			case LevelError:
				logger.ErrorEvent("event", "message")
			case LevelPanic:
				logger.PanicEvent("event", "message")
			}
		}()
		if !strings.Contains(buf.String(), FormatLevel(level)) && !strings.Contains(strings.ToLower(buf.String()), FormatLevel(level)) {
			t.Fatalf("level %s not emitted: %q", FormatLevel(level), buf.String())
		}
	}
}

func TestStructuredLoggerContextID(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestFactory(&buf, "json").NewLogger("")
	ctx := ContextWithID(context.Background(), ID{ID: 42, CreatedAt: time.Now().Add(-time.Second)})
	logger.InfoEventContext(ctx, "event", "message")
	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["context_id"].(float64) != 42 || decoded["context_age_ms"].(float64) <= 0 {
		t.Fatalf("context not emitted: %#v", decoded)
	}
}

func TestStructuredLoggerJSONFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestFactory(&buf, "json").NewLogger("api")
	logger.InfoEvent("request", "ok", Bool("cached", true))
	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["logger"] != "api" || decoded["event"] != "request" || decoded["cached"] != true {
		t.Fatalf("unexpected json output: %#v", decoded)
	}
}
