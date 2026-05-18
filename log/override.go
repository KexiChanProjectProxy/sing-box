package log

import (
	"context"
)

type overrideLevelKey struct{}

// ContextWithOverrideLevel returns a context with the given override level.
func ContextWithOverrideLevel(ctx context.Context, level Level) context.Context {
	return context.WithValue(ctx, (*overrideLevelKey)(nil), level)
}

// OverrideLevelFromContext returns the override level from the context, or the origin level if not set.
func OverrideLevelFromContext(origin Level, ctx context.Context) Level {
	level, loaded := ctx.Value((*overrideLevelKey)(nil)).(Level)
	if !loaded || origin > level {
		return origin
	}
	return level
}
