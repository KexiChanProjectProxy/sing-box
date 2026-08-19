package log

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func testRecord() Record {
	return Record{Level: LevelInfo, Message: "connected", Tag: "dialer", Event: "connect", Timestamp: time.Unix(100, 200), Fields: []Field{String("UserName", "alice"), Int("attempt", 2)}}
}

func TestJSONFormatterDeterministic(t *testing.T) {
	formatter := Formatter{DisableLineBreak: true}
	first := formatter.FormatRecordJSON(testRecord())
	second := formatter.FormatRecordJSON(testRecord())
	if first != second {
		t.Fatalf("json output is not deterministic:\n%s\n%s", first, second)
	}
	if !strings.Contains(first, `"time":"1970-01-01T00:01:40.0000002Z","level":"info","logger":"dialer","event":"connect","message":"connected"`) {
		t.Fatalf("metadata order mismatch: %s", first)
	}
}

func TestJSONFormatterParseable(t *testing.T) {
	var decoded map[string]any
	if err := json.Unmarshal([]byte(Formatter{}.FormatRecordJSON(testRecord())), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["user_name"] != "alice" || decoded["attempt"].(float64) != 2 {
		t.Fatalf("unexpected decoded fields: %#v", decoded)
	}
}

func TestJSONFormatterNoANSI(t *testing.T) {
	output := Formatter{}.FormatRecordJSON(testRecord())
	if strings.Contains(output, "\x1b[") {
		t.Fatalf("json output contains ANSI escape: %q", output)
	}
}

func TestJSONFormatterFieldOrder(t *testing.T) {
	output := Formatter{DisableLineBreak: true}.FormatRecordJSON(Record{Level: LevelInfo, Message: "m", Timestamp: time.Unix(0, 0), Fields: []Field{String("first", "1"), String("second", "2")}})
	if !strings.Contains(output, `"message":"m","first":"1","second":"2"`) {
		t.Fatalf("field order mismatch: %s", output)
	}
}

func TestJSONFormatterAllFieldTypes(t *testing.T) {
	rec := Record{Level: LevelInfo, Message: "m", Timestamp: time.Unix(0, 0), Fields: []Field{String("s", "v"), Bool("b", true), Int("i", 1), Int64("i64", 2), Uint("u", 3), Uint64("u64", 4), Float64("f", 1.25), Duration("d", 2500*time.Microsecond), Err(errors.New("boom")), AnyStringer("stringer", testStringer("ok"))}}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(Formatter{}.FormatRecordJSON(rec)), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["s"] != "v" || decoded["b"] != true || decoded["i"].(float64) != 1 || decoded["i64"].(float64) != 2 || decoded["u"].(float64) != 3 || decoded["u64"].(float64) != 4 || decoded["f"].(float64) != 1.25 || decoded["d"].(float64) != 2.5 || decoded["error"] != "boom" || decoded["stringer"] != "ok" {
		t.Fatalf("unexpected decoded output: %#v", decoded)
	}
}

func TestJSONFormatterTimestamp(t *testing.T) {
	output := Formatter{}.FormatRecordJSON(Record{Level: LevelInfo, Message: "m", Timestamp: time.Unix(1, 2).UTC()})
	if !strings.Contains(output, `"time":"1970-01-01T00:00:01.000000002Z"`) {
		t.Fatalf("timestamp missing: %s", output)
	}
}

func TestJSONFormatterOmitsEmptyOptionals(t *testing.T) {
	output := Formatter{DisableLineBreak: true}.FormatRecordJSON(Record{Level: LevelInfo, Message: "m", Timestamp: time.Unix(0, 0)})
	if strings.Contains(output, "logger") || strings.Contains(output, "event") || strings.Contains(output, "context_id") {
		t.Fatalf("empty optionals not omitted: %s", output)
	}
}

func TestJSONFormatterContextID(t *testing.T) {
	output := Formatter{DisableLineBreak: true}.FormatRecordJSON(Record{Level: LevelInfo, Message: "m", Timestamp: time.Unix(0, 0), ContextID: 7, ContextAgeMs: 2.5})
	if !strings.Contains(output, `"context_id":7,"context_age_ms":2.5`) {
		t.Fatalf("context missing: %s", output)
	}
}
