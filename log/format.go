package log

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	F "github.com/sagernet/sing/common/format"

	"github.com/logrusorgru/aurora"
)

// Formatter formats log entries for display.
type Formatter struct {
	BaseTime         time.Time
	DisableColors    bool
	DisableTimestamp bool
	FullTimestamp    bool
	TimestampFormat  string
	DisableLineBreak bool
	FormatMode       string
}

func (f Formatter) Format(ctx context.Context, level Level, tag string, message string, timestamp time.Time) string {
	levelString := strings.ToUpper(FormatLevel(level))
	if !f.DisableColors {
		switch level {
		case LevelDebug, LevelTrace:
			levelString = aurora.White(levelString).String()
		case LevelInfo:
			levelString = aurora.Cyan(levelString).String()
		case LevelWarn:
			levelString = aurora.Yellow(levelString).String()
		case LevelError, LevelFatal, LevelPanic:
			levelString = aurora.Red(levelString).String()
		}
	}
	if tag != "" {
		message = tag + ": " + message
	}
	var id ID
	var hasId bool
	if ctx != nil {
		id, hasId = IDFromContext(ctx)
	}
	if hasId {
		activeDuration := FormatDuration(time.Since(id.CreatedAt))
		if !f.DisableColors {
			var color aurora.Color
			color = aurora.Color(uint8(id.ID))
			color %= 215
			row := uint(color / 36)
			column := uint(color % 36)

			var r, g, b float32
			r = float32(row * 51)
			g = float32(column / 6 * 51)
			b = float32((column % 6) * 51)
			luma := 0.2126*r + 0.7152*g + 0.0722*b
			if luma < 60 {
				row = 5 - row
				column = 35 - column
				color = aurora.Color(row*36 + column)
			}
			color += 16
			color = color << 16
			color |= 1 << 14
			message = F.ToString("[", aurora.Colorize(id.ID, color).String(), " ", activeDuration, "] ", message)
		} else {
			message = F.ToString("[", id.ID, " ", activeDuration, "] ", message)
		}
	}
	switch {
	case f.DisableTimestamp:
		message = levelString + " " + message
	case f.FullTimestamp:
		message = timestamp.Format(f.TimestampFormat) + " " + levelString + " " + message
	default:
		message = levelString + "[" + xd(int(timestamp.Sub(f.BaseTime)/time.Second), 4) + "] " + message
	}
	if f.DisableLineBreak {
		if message[len(message)-1] == '\n' {
			message = message[:len(message)-1]
		}
	} else {
		if message[len(message)-1] != '\n' {
			message += "\n"
		}
	}
	return message
}

func (f Formatter) FormatWithSimple(ctx context.Context, level Level, tag string, message string, timestamp time.Time) (string, string) {
	levelString := strings.ToUpper(FormatLevel(level))
	if !f.DisableColors {
		switch level {
		case LevelDebug, LevelTrace:
			levelString = aurora.White(levelString).String()
		case LevelInfo:
			levelString = aurora.Cyan(levelString).String()
		case LevelWarn:
			levelString = aurora.Yellow(levelString).String()
		case LevelError, LevelFatal, LevelPanic:
			levelString = aurora.Red(levelString).String()
		}
	}
	if tag != "" {
		message = tag + ": " + message
	}
	messageSimple := message
	var id ID
	var hasId bool
	if ctx != nil {
		id, hasId = IDFromContext(ctx)
	}
	if hasId {
		activeDuration := FormatDuration(time.Since(id.CreatedAt))
		if !f.DisableColors {
			var color aurora.Color
			color = aurora.Color(uint8(id.ID))
			color %= 215
			row := uint(color / 36)
			column := uint(color % 36)

			var r, g, b float32
			r = float32(row * 51)
			g = float32(column / 6 * 51)
			b = float32((column % 6) * 51)
			luma := 0.2126*r + 0.7152*g + 0.0722*b
			if luma < 60 {
				row = 5 - row
				column = 35 - column
				color = aurora.Color(row*36 + column)
			}
			color += 16
			color = color << 16
			color |= 1 << 14
			message = F.ToString("[", aurora.Colorize(id.ID, color).String(), " ", activeDuration, "] ", message)
		} else {
			message = F.ToString("[", id.ID, " ", activeDuration, "] ", message)
		}
		messageSimple = F.ToString("[", id.ID, " ", activeDuration, "] ", messageSimple)

	}
	switch {
	case f.DisableTimestamp:
		message = levelString + " " + message
	case f.FullTimestamp:
		message = timestamp.Format(f.TimestampFormat) + " " + levelString + " " + message
	default:
		message = levelString + "[" + xd(int(timestamp.Sub(f.BaseTime)/time.Second), 4) + "] " + message
	}
	if message[len(message)-1] != '\n' {
		message += "\n"
	}
	return message, messageSimple
}

// FormatRecord formats a Record as a human-readable text line.
func (f Formatter) FormatRecord(rec Record) string {
	message := rec.Message
	if rec.Event != "" {
		message = rec.Event + ": " + message
	}
	for _, field := range rec.Fields {
		message += " " + field.Key() + "=" + f.formatFieldValue(field)
	}
	return f.Format(recordContext(rec), rec.Level, rec.Tag, message, rec.Timestamp)
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

func (f Formatter) formatFieldValue(field Field) string {
	switch field.Type() {
	case FieldTypeString:
		return field.StringValue()
	case FieldTypeBool:
		return strconv.FormatBool(field.BoolValue())
	case FieldTypeInt:
		return strconv.Itoa(field.IntValue())
	case FieldTypeInt64:
		return strconv.FormatInt(field.Int64Value(), 10)
	case FieldTypeUint:
		return strconv.FormatUint(uint64(field.UintValue()), 10)
	case FieldTypeUint64:
		return strconv.FormatUint(field.Uint64Value(), 10)
	case FieldTypeFloat64:
		return strconv.FormatFloat(field.Float64Value(), 'f', -1, 64)
	case FieldTypeDuration:
		return strconv.FormatFloat(field.DurationMs(), 'f', -1, 64)
	case FieldTypeError, FieldTypeStringer:
		return field.StringValue()
	default:
		return fmt.Sprint(field.Value())
	}
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

func recordContext(rec Record) context.Context {
	if !rec.HasContext() {
		return context.Background()
	}
	return ContextWithID(context.Background(), ID{ID: rec.ContextID, CreatedAt: rec.Timestamp.Add(-time.Duration(rec.ContextAgeMs * float64(time.Millisecond)))})
}

func xd(value int, x int) string {
	message := strconv.Itoa(value)
	for len(message) < x {
		message = "0" + message
	}
	return message
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
