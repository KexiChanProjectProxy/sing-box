package option

import (
	"testing"
	"time"

	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badoption"
	"github.com/stretchr/testify/require"
)

func TestAnyTLSOutboundOptions(t *testing.T) {
	t.Parallel()

	var options AnyTLSOutboundOptions
	err := json.Unmarshal([]byte(`{
		"idle_session_check_interval": "3s",
		"idle_session_timeout": "120s",
		"ensure_idle_session": 24,
		"heartbeat": "17s",
		"min_idle_session": 48,
		"min_idle_session_for_age": 8,
		"max_connection_lifetime": "3m",
		"connection_lifetime_jitter": "30s",
		"ensure_idle_session_create_rate": 16
	}`), &options)
	require.NoError(t, err)

	require.Equal(t, 3*time.Second, options.IdleSessionCheckInterval.Build())
	require.Equal(t, 120*time.Second, options.IdleSessionTimeout.Build())
	require.Equal(t, 17*time.Second, options.Heartbeat.Build())
	require.Equal(t, 3*time.Minute, options.MaxConnectionLifetime.Build())
	require.Equal(t, 30*time.Second, options.ConnectionLifetimeJitter.Build())

	require.Equal(t, 24, options.EnsureIdleSession)
	require.Equal(t, 48, options.MinIdleSession)
	require.Equal(t, 8, options.MinIdleSessionForAge)
	require.Equal(t, 16, options.EnsureIdleSessionCreateRate)
}

func TestAnyTLSOutboundOptionsJSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := AnyTLSOutboundOptions{
		IdleSessionCheckInterval:    badoption.Duration(3 * time.Second),
		IdleSessionTimeout:          badoption.Duration(120 * time.Second),
		EnsureIdleSession:           24,
		Heartbeat:                   badoption.Duration(17 * time.Second),
		MinIdleSession:              48,
		MinIdleSessionForAge:        8,
		MaxConnectionLifetime:       badoption.Duration(3 * time.Minute),
		ConnectionLifetimeJitter:    badoption.Duration(30 * time.Second),
		EnsureIdleSessionCreateRate: 16,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var unmarshaled AnyTLSOutboundOptions
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	require.Equal(t, original.EnsureIdleSession, unmarshaled.EnsureIdleSession)
	require.Equal(t, original.MinIdleSession, unmarshaled.MinIdleSession)
	require.Equal(t, original.MinIdleSessionForAge, unmarshaled.MinIdleSessionForAge)
	require.Equal(t, original.EnsureIdleSessionCreateRate, unmarshaled.EnsureIdleSessionCreateRate)

	require.Equal(t, original.IdleSessionCheckInterval.Build(), unmarshaled.IdleSessionCheckInterval.Build())
	require.Equal(t, original.IdleSessionTimeout.Build(), unmarshaled.IdleSessionTimeout.Build())
	require.Equal(t, original.Heartbeat.Build(), unmarshaled.Heartbeat.Build())
	require.Equal(t, original.MaxConnectionLifetime.Build(), unmarshaled.MaxConnectionLifetime.Build())
	require.Equal(t, original.ConnectionLifetimeJitter.Build(), unmarshaled.ConnectionLifetimeJitter.Build())
}
