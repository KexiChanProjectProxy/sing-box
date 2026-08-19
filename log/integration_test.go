package log

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sagernet/sing-box/option"
)

func TestJSONLEndToEnd(t *testing.T) {
	var buf bytes.Buffer
	logger := newIntegrationLogger(t, "json", &buf)

	ctx := ContextWithID(context.Background(), ID{ID: 42, CreatedAt: time.Now().Add(-50 * time.Millisecond)})
	logger.InfoEventContext(ctx, "test.event", "test message",
		String("domain", "example.com"),
		Int("rcode", 0),
		Duration("elapsed", 100*time.Millisecond),
	)

	lines := jsonlLines(t, buf.String())
	if len(lines) != 1 {
		t.Fatalf("expected one JSONL record, got %d in %q", len(lines), buf.String())
	}
	record := parseJSONLine(t, lines[0])

	for _, field := range []string{"time", "level", "logger", "message", "event"} {
		if _, ok := record[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}
	assertJSONValue(t, record, "level", "info")
	assertJSONValue(t, record, "logger", "integration-test")
	assertJSONValue(t, record, "event", "test.event")
	assertJSONValue(t, record, "message", "test message")
	assertJSONValue(t, record, "context_id", float64(42))
	assertJSONNumber(t, record, "context_age_ms")
	assertJSONValue(t, record, "domain", "example.com")
	assertJSONValue(t, record, "rcode", float64(0))
	assertJSONValue(t, record, "elapsed", float64(100))
}

func TestJSONLMultipleRecords(t *testing.T) {
	var buf bytes.Buffer
	logger := newIntegrationLogger(t, "json", &buf)

	logger.InfoEvent("first.event", "first message", String("name", "alpha"))
	logger.WarnEvent("second.event", "second message", Int("attempt", 2))
	logger.ErrorEvent("third.event", "third message", Bool("failed", true))

	lines := jsonlLines(t, buf.String())
	if len(lines) != 3 {
		t.Fatalf("expected three JSONL records, got %d in %q", len(lines), buf.String())
	}

	wants := []struct {
		level   string
		event   string
		message string
	}{
		{level: "info", event: "first.event", message: "first message"},
		{level: "warn", event: "second.event", message: "second message"},
		{level: "error", event: "third.event", message: "third message"},
	}
	for i, line := range lines {
		record := parseJSONLine(t, line)
		assertJSONValue(t, record, "level", wants[i].level)
		assertJSONValue(t, record, "logger", "integration-test")
		assertJSONValue(t, record, "event", wants[i].event)
		assertJSONValue(t, record, "message", wants[i].message)
	}
}

func TestJSONLAllFieldTypes(t *testing.T) {
	var buf bytes.Buffer
	logger := newIntegrationLogger(t, "json", &buf)

	logger.InfoEvent("types.event", "typed fields",
		String("string", "value"),
		Bool("bool", true),
		Int("int", -1),
		Int64("int64", -2),
		Uint("uint", 3),
		Uint64("uint64", 4),
		Float64("float64", 1.25),
		Duration("duration", 2500*time.Microsecond),
		Err(errors.New("boom")),
	)

	record := parseJSONLine(t, jsonlLines(t, buf.String())[0])
	assertJSONValue(t, record, "string", "value")
	assertJSONValue(t, record, "bool", true)
	assertJSONValue(t, record, "int", float64(-1))
	assertJSONValue(t, record, "int64", float64(-2))
	assertJSONValue(t, record, "uint", float64(3))
	assertJSONValue(t, record, "uint64", float64(4))
	assertJSONValue(t, record, "float64", 1.25)
	assertJSONValue(t, record, "duration", 2.5)
	assertJSONValue(t, record, "error", "boom")

	for _, key := range []string{"string", "bool", "int", "int64", "uint", "uint64", "float64", "duration", "error"} {
		if _, ok := record[key]; !ok {
			t.Errorf("missing domain field %q", key)
		}
	}
}

func TestJSONLNoANSI(t *testing.T) {
	var buf bytes.Buffer
	logger := newIntegrationLogger(t, "json", &buf)

	logger.WarnEvent("ansi.event", "plain json", String("field", "value"))

	output := buf.String()
	if strings.Contains(output, "\x1b[") {
		t.Fatalf("json output contains ANSI escape: %q", output)
	}
	_ = parseJSONLine(t, jsonlLines(t, output)[0])
}

func TestJSONLDeterministicKeyOrder(t *testing.T) {
	var buf bytes.Buffer
	logger := newIntegrationLogger(t, "json", &buf)

	ctx := ContextWithID(context.Background(), ID{ID: 7, CreatedAt: time.Now().Add(-25 * time.Millisecond)})
	logger.InfoEventContext(ctx, "order.event", "ordered fields",
		String("first", "1"),
		String("second", "2"),
		String("third", "3"),
	)

	line := jsonlLines(t, buf.String())[0]
	_ = parseJSONLine(t, line)
	assertKeyOrder(t, line, []string{"time", "level", "logger", "event", "message", "context_id", "context_age_ms", "first", "second", "third"})
}

func newIntegrationLogger(t *testing.T, format string, buf *bytes.Buffer) StructuredLogger {
	t.Helper()
	factory, err := New(Options{
		Context:       context.Background(),
		Options:       option.LogOptions{Format: format, DisableColor: true},
		DefaultWriter: buf,
		Observable:    false,
		BaseTime:      time.Unix(0, 0),
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	return factory.NewLogger("integration-test")
}

func jsonlLines(t *testing.T, output string) []string {
	t.Helper()
	if output == "" {
		t.Fatal("expected log output, got empty string")
	}
	if !strings.HasSuffix(output, "\n") {
		t.Fatalf("expected JSONL output to end with newline, got %q", output)
	}
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != line || line == "" {
			t.Fatalf("line %d is not one compact JSON object: %q", i, line)
		}
	}
	return lines
}

func parseJSONLine(t *testing.T, line string) map[string]any {
	t.Helper()
	var record map[string]any
	decoder := json.NewDecoder(strings.NewReader(line))
	if err := decoder.Decode(&record); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %q", err, line)
	}
	if decoder.More() {
		t.Fatalf("line contains more than one JSON value: %q", line)
	}
	return record
}

func assertJSONValue(t *testing.T, record map[string]any, key string, want any) {
	t.Helper()
	if got := record[key]; got != want {
		t.Errorf("expected %s %v, got: %v", key, want, got)
	}
}

func assertJSONNumber(t *testing.T, record map[string]any, key string) {
	t.Helper()
	value, ok := record[key]
	if !ok {
		t.Fatalf("missing required field: %s", key)
	}
	if _, ok := value.(float64); !ok {
		t.Fatalf("expected %s to be a JSON number, got %T (%v)", key, value, value)
	}
}

func assertKeyOrder(t *testing.T, line string, keys []string) {
	t.Helper()
	last := -1
	for _, key := range keys {
		needle := `"` + key + `":`
		index := strings.Index(line, needle)
		if index == -1 {
			t.Fatalf("missing key %q in %s", key, line)
		}
		if index <= last {
			t.Fatalf("key %q appeared out of order in %s", key, line)
		}
		last = index
	}
}
