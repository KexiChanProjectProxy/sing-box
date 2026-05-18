package log

import (
	"context"
	"math/rand"
	"time"
)

type idKey struct{}

// ID represents a log context identifier with creation timestamp.
type ID struct {
	ID        uint32
	CreatedAt time.Time
}

// ContextWithNewID returns a new context with a randomly generated ID.
func ContextWithNewID(ctx context.Context) context.Context {
	return ContextWithID(ctx, ID{
		ID:        rand.Uint32(),
		CreatedAt: time.Now(),
	})
}

// ContextWithID returns a context with the given ID.
func ContextWithID(ctx context.Context, id ID) context.Context {
	return context.WithValue(ctx, (*idKey)(nil), id)
}

// IDFromContext returns the ID from the context and whether it exists.
func IDFromContext(ctx context.Context) (ID, bool) {
	id, loaded := ctx.Value((*idKey)(nil)).(ID)
	return id, loaded
}
