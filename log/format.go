package log

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	F "github.com/sagernet/sing/common/format"
)

// Formatter formats log records as JSONL.
type Formatter struct {
	DisableLineBreak bool
}

// FormatRecordJSON formats a Record as a single JSONL line with deterministic key order.
func (f Formatter) FormatRecordJSON(rec Record) string {
	var buf bytes.Buffer
	buf.WriteByte('{')
	first := true
	writeComma := func() {
		if first {
			first = false
			return
		}
		buf.WriteByte(',')
	}
	writeString := func(key string, value string) {
		writeComma()
		writeJSONString(&buf, key)
		buf.WriteByte(':')
		writeJSONString(&buf, value)
	}
	writeString("time", rec.Timestamp.UTC().Format(time.RFC3339Nano))
	writeString("level", FormatLevel(rec.Level))
	if rec.Tag != "" {
		writeString("logger", rec.Tag)
	}
	if rec.Event != "" {
		writeString("event", rec.Event)
	}
	writeString("message", rec.Message)
	if rec.HasContext() {
		writeComma()
		buf.WriteString(`"context_id":`)
		buf.WriteString(strconv.FormatUint(uint64(rec.ContextID), 10))
		writeComma()
		buf.WriteString(`"context_age_ms":`)
		buf.WriteString(strconv.FormatFloat(rec.ContextAgeMs, 'f', -1, 64))
	}
	for _, field := range rec.Fields {
		writeComma()
		writeJSONString(&buf, field.Key())
		buf.WriteByte(':')
		f.writeJSONValue(&buf, field)
	}
	buf.WriteByte('}')
	if !f.DisableLineBreak {
		buf.WriteByte('\n')
	}
	return buf.String()
}

func (f Formatter) writeJSONValue(buf *bytes.Buffer, field Field) {
	switch field.Type() {
	case FieldTypeString:
		writeJSONString(buf, field.StringValue())
	case FieldTypeBool:
		buf.WriteString(strconv.FormatBool(field.BoolValue()))
	case FieldTypeInt:
		buf.WriteString(strconv.Itoa(field.IntValue()))
	case FieldTypeInt64:
		buf.WriteString(strconv.FormatInt(field.Int64Value(), 10))
	case FieldTypeUint:
		buf.WriteString(strconv.FormatUint(uint64(field.UintValue()), 10))
	case FieldTypeUint64:
		buf.WriteString(strconv.FormatUint(field.Uint64Value(), 10))
	case FieldTypeFloat64:
		buf.WriteString(strconv.FormatFloat(field.Float64Value(), 'f', -1, 64))
	case FieldTypeDuration:
		buf.WriteString(strconv.FormatFloat(field.DurationMs(), 'f', -1, 64))
	case FieldTypeError, FieldTypeStringer:
		writeJSONString(buf, field.StringValue())
	default:
		writeJSONString(buf, fmt.Sprint(field.Value()))
	}
}

func writeJSONString(buf *bytes.Buffer, value string) {
	encoded, _ := json.Marshal(value)
	buf.Write(encoded)
}

func FormatDuration(duration time.Duration) string {
	if duration < time.Second {
		return F.ToString(duration.Milliseconds(), "ms")
	} else if duration < time.Minute {
		return F.ToString(int64(duration.Seconds()), ".", int64(duration.Seconds()*100)%100, "s")
	} else {
		return F.ToString(int64(duration.Minutes()), "m", int64(duration.Seconds())%60, "s")
	}
}
