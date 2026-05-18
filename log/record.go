package log

import (
	"strings"
	"time"
)

// reservedKeys enumerates the reserved envelope field names that cannot be used as field keys.
var reservedKeys = map[string]bool{
	"level": true, "message": true, "time": true,
	"logger": true, "event": true,
	"context_id": true, "context_age_ms": true,
}

// IsReservedKey reports whether key is a reserved envelope field name.
func IsReservedKey(key string) bool { return reservedKeys[key] }

// Record represents a structured log entry with envelope fields (time, level, logger, event, message) and optional domain fields.
type Record struct {
	Level        Level
	Message      string
	Tag          string
	Event        string
	Timestamp    time.Time
	ContextID    uint32
	ContextAgeMs float64
	Fields       []Field
}

// HasContext reports whether the record has a context ID.
func (r *Record) HasContext() bool { return r.ContextID != 0 }

// NormalizeKey converts CamelCase, kebab-case, and dot.case to lower_snake_case.
func NormalizeKey(key string) string {
	if key == "" {
		return key
	}
	var result strings.Builder
	result.Grow(len(key) + 4)
	for i := 0; i < len(key); i++ {
		c := key[i]
		if c >= 'A' && c <= 'Z' {
			needUnderscore := false
			if i > 0 {
				prevChar := key[i-1]
				if prevChar >= 'a' && prevChar <= 'z' {
					needUnderscore = true
				} else if i+1 < len(key) && prevChar >= 'A' && prevChar <= 'Z' && key[i+1] >= 'a' && key[i+1] <= 'z' {
					needUnderscore = true
				}
			}
			if needUnderscore {
				result.WriteByte('_')
			}
			result.WriteByte(c + 32)
		} else if c == '-' || c == ' ' || c == '.' {
			if result.Len() > 0 && result.String()[result.Len()-1] != '_' {
				result.WriteByte('_')
			}
		} else {
			result.WriteByte(c)
		}
	}
	return result.String()
}
