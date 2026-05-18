package tls

import (
	"encoding/base64"
	"fmt"
	"math"
	"reflect"
	"time"

	"github.com/sagernet/sing-box/log"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ACMELazyLogWriter struct {
	Logger log.StructuredLogger
}

func (w *ACMELazyLogWriter) Enabled(level zapcore.Level) bool {
	return true
}

func (w *ACMELazyLogWriter) With(fields []zapcore.Field) zapcore.Core {
	return &ACMELogWriter{
		Logger: w.Logger,
		fields: zapFieldsToLogFields(fields),
	}
}

func (w *ACMELazyLogWriter) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if w.Enabled(entry.Level) {
		return checked.AddCore(entry, w)
	}
	return checked
}

func (w *ACMELazyLogWriter) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	return writeACMELog(w.Logger, entry, zapFieldsToLogFields(fields))
}

func (w *ACMELazyLogWriter) Sync() error { return nil }

type ACMELogWriter struct {
	Logger log.StructuredLogger
	fields []log.Field
}

func (w *ACMELogWriter) Enabled(level zapcore.Level) bool {
	return true
}

func (w *ACMELogWriter) With(fields []zapcore.Field) zapcore.Core {
	newFields := make([]log.Field, 0, len(w.fields)+len(fields))
	newFields = append(newFields, w.fields...)
	newFields = append(newFields, zapFieldsToLogFields(fields)...)
	return &ACMELogWriter{
		Logger: w.Logger,
		fields: newFields,
	}
}

func (w *ACMELogWriter) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if w.Enabled(entry.Level) {
		return checked.AddCore(entry, w)
	}
	return checked
}

func (w *ACMELogWriter) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	allFields := make([]log.Field, 0, len(w.fields)+len(fields))
	allFields = append(allFields, w.fields...)
	allFields = append(allFields, zapFieldsToLogFields(fields)...)
	return writeACMELog(w.Logger, entry, allFields)
}

func (w *ACMELogWriter) Sync() error { return nil }

func writeACMELog(logger log.StructuredLogger, entry zapcore.Entry, fields []log.Field) error {
	event := "acme"
	if entry.LoggerName != "" {
		fields = append(fields, log.String("zap_logger", entry.LoggerName))
	}
	if entry.Caller.Defined {
		fields = append(fields, log.String("caller", entry.Caller.TrimmedPath()))
	}
	if entry.Stack != "" {
		fields = append(fields, log.String("stack", entry.Stack))
	}
	message := entry.Message
	switch entry.Level {
	case zapcore.DebugLevel:
		logger.DebugEvent(event, message, fields...)
	case zapcore.InfoLevel:
		logger.InfoEvent(event, message, fields...)
	case zapcore.WarnLevel:
		logger.WarnEvent(event, message, fields...)
	case zapcore.ErrorLevel:
		logger.ErrorEvent(event, message, fields...)
	case zapcore.DPanicLevel, zapcore.PanicLevel:
		logger.PanicEvent(event, message, fields...)
	case zapcore.FatalLevel:
		logger.FatalEvent(event, message, fields...)
	default:
		if entry.Level < zapcore.DebugLevel {
			logger.TraceEvent(event, message, fields...)
		} else {
			logger.DebugEvent(event, message, fields...)
		}
	}
	return nil
}

func zapFieldsToLogFields(fields []zapcore.Field) []log.Field {
	if len(fields) == 0 {
		return nil
	}
	logFields := make([]log.Field, 0, len(fields))
	for _, field := range fields {
		logFields = append(logFields, zapFieldToLogField(field))
	}
	return logFields
}

func zapFieldToLogField(field zapcore.Field) log.Field {
	key := acmeLogFieldKey(field.Key)
	switch field.Type {
	case zapcore.StringType:
		return log.String(key, field.String)
	case zapcore.StringerType:
		if value, ok := field.Interface.(fmt.Stringer); ok {
			return log.AnyStringer(key, value)
		}
		return log.String(key, fmt.Sprint(field.Interface))
	case zapcore.BinaryType:
		if value, ok := field.Interface.([]byte); ok {
			return log.String(key, base64.StdEncoding.EncodeToString(value))
		}
		return log.String(key, fmt.Sprint(field.Interface))
	case zapcore.ByteStringType:
		if value, ok := field.Interface.([]byte); ok {
			return log.String(key, string(value))
		}
		return log.String(key, fmt.Sprint(field.Interface))
	case zapcore.BoolType:
		return log.Bool(key, field.Integer == 1)
	case zapcore.Int8Type, zapcore.Int16Type, zapcore.Int32Type, zapcore.Int64Type:
		return log.Int64(key, field.Integer)
	case zapcore.Uint8Type, zapcore.Uint16Type, zapcore.Uint32Type, zapcore.Uint64Type, zapcore.UintptrType:
		return log.Uint64(key, uint64(field.Integer))
	case zapcore.Float32Type:
		return log.Float64(key, float64(math.Float32frombits(uint32(field.Integer))))
	case zapcore.Float64Type:
		return log.Float64(key, math.Float64frombits(uint64(field.Integer)))
	case zapcore.DurationType:
		return log.Duration(key, time.Duration(field.Integer))
	case zapcore.TimeType:
		location, _ := field.Interface.(*time.Location)
		if location == nil {
			location = time.UTC
		}
		return log.String(key, time.Unix(0, field.Integer).In(location).Format(time.RFC3339Nano))
	case zapcore.TimeFullType:
		if value, ok := field.Interface.(time.Time); ok {
			return log.String(key, value.Format(time.RFC3339Nano))
		}
		return log.String(key, fmt.Sprint(field.Interface))
	case zapcore.ErrorType:
		if err, ok := field.Interface.(error); ok {
			return log.ErrNamed(key, err)
		}
		return log.String(key, fmt.Sprint(field.Interface))
	case zapcore.SkipType:
		return log.String(key, "")
	case zapcore.NamespaceType:
		return log.String(key, "")
	case zapcore.ReflectType, zapcore.ArrayMarshalerType, zapcore.ObjectMarshalerType, zapcore.InlineMarshalerType:
		return log.String(key, stringifyZapFieldInterface(field.Interface))
	case zapcore.Complex64Type, zapcore.Complex128Type:
		return log.String(key, stringifyZapFieldInterface(field.Interface))
	default:
		if field.String != "" {
			return log.String(key, field.String)
		}
		return log.String(key, fmt.Sprint(field.Interface))
	}
}

func stringifyZapFieldInterface(value any) string {
	if value == nil {
		return ""
	}
	if !reflect.ValueOf(value).IsValid() {
		return ""
	}
	return fmt.Sprint(value)
}

func acmeLogFieldKey(key string) string {
	key = log.NormalizeKey(key)
	if key == "" {
		return "zap_field"
	}
	if log.IsReservedKey(key) {
		return "zap_" + key
	}
	return key
}

func ACMEEncoderConfig() zapcore.EncoderConfig {
	config := zap.NewProductionEncoderConfig()
	config.TimeKey = zapcore.OmitKey
	return config
}
