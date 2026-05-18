package log

import (
	"reflect"
	"testing"
	"time"
)

func TestRecordBasicConstruction(t *testing.T) {
	now := time.Unix(123, 456)
	rec := Record{Level: LevelInfo, Message: "started", Tag: "core", Event: "startup", Timestamp: now}
	if rec.Level != LevelInfo || rec.Message != "started" || rec.Tag != "core" || rec.Event != "startup" || !rec.Timestamp.Equal(now) {
		t.Fatal("record fields were not preserved")
	}
}

func TestRecordHasContext(t *testing.T) {
	if (&Record{}).HasContext() {
		t.Fatal("empty record must not have context")
	}
	if !(&Record{ContextID: 1}).HasContext() {
		t.Fatal("record with context id must have context")
	}
}

func TestRecordReservedFieldCollision(t *testing.T) {
	for _, key := range []string{"level", "message", "time", "logger", "event", "context_id", "context_age_ms"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("expected panic for reserved key %q", key)
				}
			}()
			_ = String(key, "x")
		}()
	}
}

func TestRecordWithFields(t *testing.T) {
	rec := Record{Fields: []Field{String("UserName", "alice"), Int("attempt", 2)}}
	if len(rec.Fields) != 2 || rec.Fields[0].Key() != "user_name" || rec.Fields[1].IntValue() != 2 {
		t.Fatal("record fields were not preserved")
	}
}

func TestFieldOrderDeterministic(t *testing.T) {
	rec := Record{Fields: []Field{String("first", "1"), String("second", "2"), String("third", "3")}}
	keys := make([]string, 0, len(rec.Fields))
	for _, field := range rec.Fields {
		keys = append(keys, field.Key())
	}
	if !reflect.DeepEqual(keys, []string{"first", "second", "third"}) {
		t.Fatalf("field order changed: %v", keys)
	}
}

func TestRecordEmptyFields(t *testing.T) {
	rec := Record{}
	if rec.Fields != nil || len(rec.Fields) != 0 {
		t.Fatal("empty record should have no fields")
	}
}
