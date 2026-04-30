package log

import (
	"context"
	"testing"

	M "github.com/sagernet/sing/common/metadata"
	"github.com/stretchr/testify/require"
)

func TestConnectionEventToStructuredEvent(t *testing.T) {
	event := NewConnectionEvent("inbound", "start").
		WithSource(M.Socksaddr{}).
		WithDestination(M.Socksaddr{}).
		WithNetwork("tcp").
		WithInbound("demo", "tun")

	structured := event.ToStructuredEvent()
	require.Equal(t, EventTypeConnection, structured.Type)
	require.NotNil(t, structured.Data)
	require.Equal(t, "inbound", structured.Data["direction"])
	require.Equal(t, "start", structured.Data["action"])
	require.Equal(t, "tcp", structured.Data["network"])
	require.Equal(t, "demo", structured.Data["inbound"])
}

func TestDNSEventToStructuredEvent(t *testing.T) {
	event := NewDNSEvent("exchange", "example.com").
		WithQueryType(1).
		WithTransport("https").
		WithResponse(0, 300).
		WithAnswers([]string{"1.2.3.4", "5.6.7.8"})

	structured := event.ToStructuredEvent()
	require.Equal(t, EventTypeDNS, structured.Type)
	require.NotNil(t, structured.Data)
	require.Equal(t, "exchange", structured.Data["action"])
	require.Equal(t, "example.com", structured.Data["domain"])
	require.Equal(t, "A", structured.Data["query_type"])
	require.Equal(t, "NOERROR", structured.Data["rcode"])
	require.Equal(t, []string{"1.2.3.4", "5.6.7.8"}, structured.Data["answers"])
}

func TestRouterMatchEventToStructuredEvent(t *testing.T) {
	event := NewRouterMatchEvent(5, "domain=example.com", "route(selector-a)").
		WithOutbound("selector-a").
		WithResolvedChain("proxy-a", []string{"selector-a", "urltest-a"}).
		WithMatched(true).
		WithMatchType("domain", "example.com")

	structured := event.ToStructuredEvent()
	require.Equal(t, EventTypeRouterMatch, structured.Type)
	require.NotNil(t, structured.Data)
	require.Equal(t, 5, structured.Data["rule_index"])
	require.Equal(t, "domain=example.com", structured.Data["rule"])
	require.Equal(t, "selector-a", structured.Data["outbound"])
	require.Equal(t, "proxy-a", structured.Data["resolved_outbound"])
	require.True(t, structured.Data["matched"].(bool))
	require.Equal(t, "domain", structured.Data["match_type"])
}

func TestProcessInfoEventToStructuredEvent(t *testing.T) {
	event := NewProcessInfoEvent("found").
		WithProcessPath("/usr/bin/firefox").
		WithUserName("user").
		WithUserId(1000)

	structured := event.ToStructuredEvent()
	require.Equal(t, EventTypeProcessInfo, structured.Type)
	require.NotNil(t, structured.Data)
	require.Equal(t, "found", structured.Data["action"])
	require.Equal(t, "/usr/bin/firefox", structured.Data["process_path"])
	require.Equal(t, "user", structured.Data["user_name"])
	require.Equal(t, int32(1000), structured.Data["user_id"])
}

func TestTransferEventToStructuredEvent(t *testing.T) {
	event := NewTransferEvent("upload", "finished").
		WithBytes(1024 * 1024)

	structured := event.ToStructuredEvent()
	require.Equal(t, EventTypeTransfer, structured.Type)
	require.NotNil(t, structured.Data)
	require.Equal(t, "upload", structured.Data["direction"])
	require.Equal(t, "finished", structured.Data["status"])
	require.Equal(t, int64(1048576), structured.Data["bytes"])
}

type mockContextLogger struct {
	lastLevel   Level
	lastMessage string
	lastArgs    []any
}

func (m *mockContextLogger) Log(ctx context.Context, level Level, args []any) {
	m.lastLevel = level
	m.lastMessage = ""
	if len(args) > 0 {
		m.lastMessage = args[0].(string)
	}
	if len(args) > 1 {
		m.lastArgs = args[1:]
	}
}

func (m *mockContextLogger) Trace(args ...any)                          {}
func (m *mockContextLogger) Debug(args ...any)                          {}
func (m *mockContextLogger) Info(args ...any)                           {}
func (m *mockContextLogger) Warn(args ...any)                           {}
func (m *mockContextLogger) Error(args ...any)                          {}
func (m *mockContextLogger) Fatal(args ...any)                          {}
func (m *mockContextLogger) Panic(args ...any)                          {}
func (m *mockContextLogger) TraceContext(ctx context.Context, args ...any) {
	m.lastLevel = LevelTrace
	if len(args) > 0 {
		m.lastMessage = args[0].(string)
	}
}
func (m *mockContextLogger) DebugContext(ctx context.Context, args ...any) {
	m.lastLevel = LevelDebug
	if len(args) > 0 {
		m.lastMessage = args[0].(string)
	}
}
func (m *mockContextLogger) InfoContext(ctx context.Context, args ...any) {
	m.lastLevel = LevelInfo
	if len(args) > 0 {
		m.lastMessage = args[0].(string)
	}
}
func (m *mockContextLogger) WarnContext(ctx context.Context, args ...any) {
	m.lastLevel = LevelWarn
	if len(args) > 0 {
		m.lastMessage = args[0].(string)
	}
}
func (m *mockContextLogger) ErrorContext(ctx context.Context, args ...any) {
	m.lastLevel = LevelError
	if len(args) > 0 {
		m.lastMessage = args[0].(string)
	}
}
func (m *mockContextLogger) FatalContext(ctx context.Context, args ...any) {
	m.lastLevel = LevelFatal
	if len(args) > 0 {
		m.lastMessage = args[0].(string)
	}
}
func (m *mockContextLogger) PanicContext(ctx context.Context, args ...any) {
	m.lastLevel = LevelPanic
	if len(args) > 0 {
		m.lastMessage = args[0].(string)
	}
}

func TestWithConnectionEventFallback(t *testing.T) {
	mock := &mockContextLogger{}
	ctx := context.Background()

	event := NewConnectionEvent("inbound", "start").
		WithSource(M.Socksaddr{})

	WithConnectionEvent(mock, ctx, LevelInfo, event, "connection established")

	require.Equal(t, LevelInfo, mock.lastLevel)
}

func TestWithRouterMatchEventFallback(t *testing.T) {
	mock := &mockContextLogger{}
	ctx := context.Background()

	event := NewRouterMatchEvent(3, "domain=example.com", "route(selector-a)").
		WithMatched(true)

	WithRouterMatchEvent(mock, ctx, LevelDebug, event, "rule matched")

	require.Equal(t, LevelDebug, mock.lastLevel)
}

func TestWithDNSEventFallback(t *testing.T) {
	mock := &mockContextLogger{}
	ctx := context.Background()

	event := NewDNSEvent("exchange", "example.com").
		WithQueryType(1)

	WithDNSEvent(mock, ctx, LevelInfo, event, "dns query")

	require.Equal(t, LevelInfo, mock.lastLevel)
}

func TestWithProcessInfoEventFallback(t *testing.T) {
	mock := &mockContextLogger{}
	ctx := context.Background()

	event := NewProcessInfoEvent("found").
		WithProcessPath("/usr/bin/firefox")

	WithProcessInfoEvent(mock, ctx, LevelInfo, event, "process found")

	require.Equal(t, LevelInfo, mock.lastLevel)
}

func TestWithTransferEventFallback(t *testing.T) {
	mock := &mockContextLogger{}
	ctx := context.Background()

	event := NewTransferEvent("upload", "finished").
		WithBytes(2048)

	WithTransferEvent(mock, ctx, LevelInfo, event, "transfer done")

	require.Equal(t, LevelInfo, mock.lastLevel)
}