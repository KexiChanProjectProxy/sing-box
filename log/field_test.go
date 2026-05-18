package log

import (
	"errors"
	"testing"
	"time"
)

type testStringer string

func (s testStringer) String() string { return string(s) }

func TestFieldStringConstructor(t *testing.T) {
	field := String("UserName", "alice")
	if field.Key() != "user_name" || field.StringValue() != "alice" || field.Type() != FieldTypeString {
		t.Fatalf("unexpected string field: %#v", field)
	}
}

func TestFieldBoolConstructor(t *testing.T) {
	field := Bool("enabled", true)
	if !field.BoolValue() || field.Type() != FieldTypeBool {
		t.Fatalf("unexpected bool field: %#v", field)
	}
}

func TestFieldIntConstructor(t *testing.T) {
	if field := Int("count", 3); field.IntValue() != 3 || field.Type() != FieldTypeInt {
		t.Fatalf("unexpected int field: %#v", field)
	}
	if field := Int64("count", 4); field.Int64Value() != 4 || field.Type() != FieldTypeInt64 {
		t.Fatalf("unexpected int64 field: %#v", field)
	}
	if field := Uint("count", 5); field.UintValue() != 5 || field.Type() != FieldTypeUint {
		t.Fatalf("unexpected uint field: %#v", field)
	}
	if field := Uint64("count", 6); field.Uint64Value() != 6 || field.Type() != FieldTypeUint64 {
		t.Fatalf("unexpected uint64 field: %#v", field)
	}
	if field := Float64("ratio", 1.5); field.Float64Value() != 1.5 || field.Type() != FieldTypeFloat64 {
		t.Fatalf("unexpected float field: %#v", field)
	}
}

func TestFieldDurationSerializesAsMs(t *testing.T) {
	field := Duration("elapsed", 1500*time.Microsecond)
	if field.DurationMs() != 1.5 || field.Type() != FieldTypeDuration {
		t.Fatalf("unexpected duration field: %#v", field)
	}
}

func TestFieldErrorSerializesToStableString(t *testing.T) {
	err := errors.New("boom")
	if field := Err(err); field.Key() != "error" || field.StringValue() != "boom" || field.Type() != FieldTypeError {
		t.Fatalf("unexpected error field: %#v", field)
	}
	if field := ErrNamed("Cause", err); field.Key() != "cause" || field.StringValue() != "boom" {
		t.Fatalf("unexpected named error field: %#v", field)
	}
}

func TestFieldKeyNormalization(t *testing.T) {
	cases := map[string]string{"CamelCase": "camel_case", "HTTPResponse": "http_response", "kebab-case": "kebab_case", "with space": "with_space", "a.b": "a_b"}
	for input, expected := range cases {
		if actual := NormalizeKey(input); actual != expected {
			t.Fatalf("NormalizeKey(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestFieldReservedKeyPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected reserved key panic")
		}
	}()
	_ = String("Message", "x")
}

func TestFieldUnsupportedValuePolicy(t *testing.T) {
	field := AnyStringer("custom", testStringer("stable"))
	if field.StringValue() != "stable" || field.Type() != FieldTypeStringer {
		t.Fatalf("unexpected stringer field: %#v", field)
	}
}
