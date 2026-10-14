package anytls

import (
	"testing"
	"time"

	"github.com/sagernet/sing-box/log"

	"github.com/anytls/sing-anytls/session"
	"github.com/stretchr/testify/require"
)

func fieldMap(fields []log.Field) map[string]any {
	m := make(map[string]any, len(fields))
	for _, field := range fields {
		m[field.Key()] = field.Value()
	}
	return m
}

func TestSessionFieldsReusedMeasuredLimited(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.FixedZone("test", -7*3600))
	created := now.Add(-90 * time.Second)
	fields := fieldMap(sessionFields(session.SessionInfo{
		Seq:           7,
		Reused:        true,
		IdleFor:       2 * time.Second,
		Latency:       session.Latency{Smoothed: 40 * time.Millisecond, Last: 50 * time.Millisecond, Samples: 3},
		Streams:       5,
		CreatedAt:     created,
		MaxLifetime:   2 * time.Minute,
		ExpiresAt:     created.Add(2 * time.Minute),
		IdleSessions:  2,
		TotalSessions: 4,
	}, now))
	require.Equal(t, uint64(7), fields["session"])
	require.Equal(t, "pool", fields["source"])
	require.Equal(t, uint64(5), fields["streams"])
	require.Equal(t, created.UTC().Format(time.RFC3339Nano), fields["created_at"])
	require.Equal(t, float64(90000), fields["age"])
	require.Equal(t, float64(2000), fields["idle_for"])
	require.Equal(t, uint64(3), fields["latency_samples"])
	require.Equal(t, float64(40), fields["latency"])
	require.Equal(t, float64(50), fields["latency_last"])
	require.Equal(t, float64(120000), fields["max_lifetime"])
	require.Equal(t, created.Add(2*time.Minute).UTC().Format(time.RFC3339Nano), fields["expires_at"])
	require.Equal(t, float64(30000), fields["lifetime_left"])
	require.Equal(t, 2, fields["idle_sessions"])
	require.Equal(t, 4, fields["total_sessions"])
}

func TestSessionFieldsNewUnmeasuredUnlimited(t *testing.T) {
	now := time.Now()
	fields := fieldMap(sessionFields(session.SessionInfo{Seq: 1, Streams: 1, CreatedAt: now, TotalSessions: 1}, now))
	require.Equal(t, "new", fields["source"])
	require.Equal(t, uint64(0), fields["latency_samples"])
	for _, key := range []string{"idle_for", "latency", "latency_last", "max_lifetime", "expires_at", "lifetime_left"} {
		require.NotContains(t, fields, key)
	}
}
