package log

import (
	"fmt"
	"time"
)

// FieldType represents the type of a structured log field.
type FieldType uint8

// Field type constants for structured logging.
const (
	FieldTypeString FieldType = iota
	FieldTypeBool
	FieldTypeInt
	FieldTypeInt64
	FieldTypeUint
	FieldTypeUint64
	FieldTypeFloat64
	FieldTypeDuration
	FieldTypeError
	FieldTypeStringer
)

// Field is a key-value pair for structured logging. Keys are automatically normalized to lower_snake_case and must not collide with reserved envelope keys.
type Field struct {
	key   string
	value any
	type_ FieldType
}

// Key returns the field key.
func (f Field) Key() string { return f.key }

// Value returns the field value.
func (f Field) Value() any { return f.value }

// Type returns the field type.
func (f Field) Type() FieldType { return f.type_ }

// StringValue returns the string value of the field. For error values, it returns the error message.
func (f Field) StringValue() string {
	if value, ok := f.value.(string); ok {
		return value
	}
	if value, ok := f.value.(fmt.Stringer); ok {
		return value.String()
	}
	if value, ok := f.value.(error); ok {
		return value.Error()
	}
	return ""
}

// BoolValue returns the bool value of the field.
func (f Field) BoolValue() bool {
	value, _ := f.value.(bool)
	return value
}

// IntValue returns the int value of the field.
func (f Field) IntValue() int {
	value, _ := f.value.(int)
	return value
}

// Int64Value returns the int64 value of the field.
func (f Field) Int64Value() int64 {
	value, _ := f.value.(int64)
	return value
}

// UintValue returns the uint value of the field.
func (f Field) UintValue() uint {
	value, _ := f.value.(uint)
	return value
}

// Uint64Value returns the uint64 value of the field.
func (f Field) Uint64Value() uint64 {
	value, _ := f.value.(uint64)
	return value
}

// Float64Value returns the float64 value of the field.
func (f Field) Float64Value() float64 {
	value, _ := f.value.(float64)
	return value
}

// DurationMs returns the duration value of the field in milliseconds.
func (f Field) DurationMs() float64 {
	value, _ := f.value.(float64)
	return value
}

func newField(key string, value any, fieldType FieldType) Field {
	key = NormalizeKey(key)
	if IsReservedKey(key) {
		panic("log field key is reserved: " + key)
	}
	return Field{key: key, value: value, type_: fieldType}
}

// String creates a string field with the given key and value. The key is normalized to snake_case.
func String(key string, value string) Field {
	return newField(key, value, FieldTypeString)
}

// Bool creates a bool field with the given key and value. The key is normalized to snake_case.
func Bool(key string, value bool) Field {
	return newField(key, value, FieldTypeBool)
}

// Int creates an int field with the given key and value. The key is normalized to snake_case.
func Int(key string, value int) Field {
	return newField(key, value, FieldTypeInt)
}

// Int64 creates an int64 field with the given key and value. The key is normalized to snake_case.
func Int64(key string, value int64) Field {
	return newField(key, value, FieldTypeInt64)
}

// Uint creates a uint field with the given key and value. The key is normalized to snake_case.
func Uint(key string, value uint) Field {
	return newField(key, value, FieldTypeUint)
}

// Uint64 creates a uint64 field with the given key and value. The key is normalized to snake_case.
func Uint64(key string, value uint64) Field {
	return newField(key, value, FieldTypeUint64)
}

// Float64 creates a float64 field with the given key and value. The key is normalized to snake_case.
func Float64(key string, value float64) Field {
	return newField(key, value, FieldTypeFloat64)
}

// Duration creates a duration field stored as float64 milliseconds with the given key and value. The key is normalized to snake_case.
func Duration(key string, value time.Duration) Field {
	return newField(key, float64(value)/float64(time.Millisecond), FieldTypeDuration)
}

// Err creates an error field with key "error". Use ErrNamed for a custom key.
func Err(err error) Field {
	return newField("error", err, FieldTypeError)
}

// ErrNamed creates an error field with a custom key. The key is normalized to snake_case.
func ErrNamed(key string, err error) Field {
	return newField(key, err, FieldTypeError)
}

// AnyStringer creates a stringer field with the given key and value. The key is normalized to snake_case.
func AnyStringer(key string, value fmt.Stringer) Field {
	return newField(key, value, FieldTypeStringer)
}

// Addr creates a string field from an address Stringer. Use keys source or destination.
func Addr(key string, addr fmt.Stringer) Field {
	return String(key, addr.String())
}
