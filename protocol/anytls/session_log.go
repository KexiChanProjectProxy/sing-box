package anytls

import (
	"context"
	"time"

	"github.com/sagernet/sing-box/log"

	"github.com/anytls/sing-anytls/session"
)

// logSelectedSession reports, at debug level, which session a new connection
// was placed on and what the pool knows about it.
func (h *Outbound) logSelectedSession(ctx context.Context, info session.SessionInfo) {
	h.logger.DebugEventContext(ctx, "anytls.session.selected", "anytls session selected", sessionFields(info, time.Now())...)
}

// sessionFields describes a selected session. Durations are in milliseconds.
// Latency fields are left out until the session has been measured, and
// lifetime fields when max_connection_lifetime is not set; lifetime_left is
// negative for an expired session that age cleanup has not closed yet.
// retire_at and retire_left appear with close_after_lifetime.
func sessionFields(info session.SessionInfo, now time.Time) []log.Field {
	source := "new"
	if info.Reused {
		source = "pool"
	}
	fields := make([]log.Field, 0, 16)
	fields = append(fields,
		log.Uint64("session", info.Seq),
		log.String("source", source),
		log.Uint64("streams", info.Streams),
		log.String("created_at", info.CreatedAt.UTC().Format(time.RFC3339Nano)),
		log.Duration("age", info.Age(now)),
	)
	if info.Reused {
		fields = append(fields, log.Duration("idle_for", info.IdleFor))
	}
	fields = append(fields, log.Uint64("latency_samples", uint64(info.Latency.Samples)))
	if info.Latency.Samples > 0 {
		fields = append(fields,
			log.Duration("latency", info.Latency.Smoothed),
			log.Duration("latency_last", info.Latency.Last),
		)
	}
	if left, limited := info.LifetimeLeft(now); limited {
		fields = append(fields,
			log.Duration("max_lifetime", info.MaxLifetime),
			log.String("expires_at", info.ExpiresAt.UTC().Format(time.RFC3339Nano)),
			log.Duration("lifetime_left", left),
		)
	}
	if !info.RetireAt.IsZero() {
		fields = append(fields,
			log.String("retire_at", info.RetireAt.UTC().Format(time.RFC3339Nano)),
			log.Duration("retire_left", info.RetireAt.Sub(now)),
		)
	}
	fields = append(fields,
		log.Int("idle_sessions", info.IdleSessions),
		log.Int("total_sessions", info.TotalSessions),
	)
	return fields
}
