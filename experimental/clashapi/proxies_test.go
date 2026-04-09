package clashapi

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/observable"

	"github.com/stretchr/testify/require"
)

type clashAPITestHistoryStorage struct {
	stored  map[string]*adapter.URLTestHistory
	storeMu []string
	deleteMu []string
}

func newClashAPITestHistoryStorage() *clashAPITestHistoryStorage {
	return &clashAPITestHistoryStorage{stored: make(map[string]*adapter.URLTestHistory)}
}

func (s *clashAPITestHistoryStorage) SetHook(hook *observable.Subscriber[struct{}]) {}
func (s *clashAPITestHistoryStorage) LoadURLTestHistory(tag string) *adapter.URLTestHistory {
	return s.stored[tag]
}
func (s *clashAPITestHistoryStorage) DeleteURLTestHistory(tag string) {
	s.deleteMu = append(s.deleteMu, tag)
	delete(s.stored, tag)
}
func (s *clashAPITestHistoryStorage) StoreURLTestHistory(tag string, history *adapter.URLTestHistory) {
	s.storeMu = append(s.storeMu, tag)
	s.stored[tag] = history
}
func (s *clashAPITestHistoryStorage) Close() error { return nil }

type clashAPITestOutbound struct {
	tag         string
	typ         string
	network     []string
	now         string
	all         []string
	dialErr     error
	dialCount   int
	listenCount int
}

func (o *clashAPITestOutbound) Type() string           { return o.typ }
func (o *clashAPITestOutbound) Tag() string            { return o.tag }
func (o *clashAPITestOutbound) Network() []string      { return o.network }
func (o *clashAPITestOutbound) Dependencies() []string { return nil }
func (o *clashAPITestOutbound) Now() string            { return o.now }
func (o *clashAPITestOutbound) All() []string          { return append([]string(nil), o.all...) }
func (o *clashAPITestOutbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	o.dialCount++
	if o.dialErr != nil {
		return nil, o.dialErr
	}
	return (&net.Dialer{}).DialContext(ctx, network, destination.String())
}
func (o *clashAPITestOutbound) ListenPacket(context.Context, M.Socksaddr) (net.PacketConn, error) {
	o.listenCount++
	return nil, nil
}

type clashAPITestOutboundManager struct {
	outbounds map[string]adapter.Outbound
	defaultOutbound adapter.Outbound
}

func (m *clashAPITestOutboundManager) Start(adapter.StartStage) error { return nil }
func (m *clashAPITestOutboundManager) Close() error { return nil }
func (m *clashAPITestOutboundManager) Outbounds() []adapter.Outbound {
	result := make([]adapter.Outbound, 0, len(m.outbounds))
	for _, outbound := range m.outbounds {
		result = append(result, outbound)
	}
	return result
}
func (m *clashAPITestOutboundManager) Outbound(tag string) (adapter.Outbound, bool) {
	outbound, loaded := m.outbounds[tag]
	return outbound, loaded
}
func (m *clashAPITestOutboundManager) Default() adapter.Outbound { return m.defaultOutbound }
func (m *clashAPITestOutboundManager) Remove(string) error { return nil }
func (m *clashAPITestOutboundManager) Create(context.Context, adapter.Router, log.ContextLogger, string, string, any) error {
	return nil
}

func TestGetProxyDelayUsesFallbackHistoryKeyWhenNestedGroupUnresolved(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer testServer.Close()

	history := newClashAPITestHistoryStorage()
	proxy := &clashAPITestOutbound{
		tag:     "selector-a",
		typ:     "selector",
		network: []string{"tcp", "udp"},
	}
	manager := &clashAPITestOutboundManager{outbounds: map[string]adapter.Outbound{proxy.Tag(): proxy}, defaultOutbound: proxy}
	server := &Server{outbound: manager, logger: log.NewNOPFactory().Logger(), urlTestHistory: history}

	req := httptest.NewRequest(http.MethodGet, "/proxies/selector-a/delay?url="+testServer.URL+"&timeout=1000", nil)
	req = req.WithContext(context.WithValue(req.Context(), CtxKeyProxy, proxy))
	w := httptest.NewRecorder()

	getProxyDelay(server)(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, []string{"selector-a"}, history.storeMu)
	require.Empty(t, history.deleteMu)
	require.NotNil(t, history.LoadURLTestHistory("selector-a"))
	require.Equal(t, 1, proxy.dialCount, "unresolved group delay should still probe through the fallback target")
}


func TestGetProxyDelayDeleteUsesFallbackHistoryKeyWhenNestedGroupUnresolved(t *testing.T) {
	history := newClashAPITestHistoryStorage()
	proxy := &clashAPITestOutbound{
		tag:     "selector-a",
		typ:     "selector",
		network: []string{"tcp", "udp"},
		dialErr: net.InvalidAddrError("boom"),
	}
	manager := &clashAPITestOutboundManager{outbounds: map[string]adapter.Outbound{proxy.Tag(): proxy}, defaultOutbound: proxy}
	server := &Server{outbound: manager, logger: log.NewNOPFactory().Logger(), urlTestHistory: history}

	req := httptest.NewRequest(http.MethodGet, "/proxies/selector-a/delay?url=https://example.com/generate_204&timeout=100", nil)
	req = req.WithContext(context.WithValue(req.Context(), CtxKeyProxy, proxy))
	w := httptest.NewRecorder()

	getProxyDelay(server)(w, req)

	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.Empty(t, history.storeMu)
	require.Equal(t, []string{"selector-a"}, history.deleteMu)
}

func TestGetGroupDelayUsesBestEffortFallbackOutboundForUnresolvedNestedGroup(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer testServer.Close()

	history := newClashAPITestHistoryStorage()
	inner := &clashAPITestOutbound{
		tag:     "inner-urltest",
		typ:     "urltest",
		network: []string{"tcp", "udp"},
	}
	outer := &clashAPITestOutbound{
		tag:     "selector-a",
		typ:     "selector",
		network: []string{"tcp", "udp"},
		now:     inner.Tag(),
	}
	parent := &clashAPITestOutbound{
		tag:     "group-a",
		typ:     "selector",
		network: []string{"tcp", "udp"},
		all:     []string{outer.Tag()},
	}
	manager := &clashAPITestOutboundManager{
		outbounds: map[string]adapter.Outbound{
			parent.Tag(): parent,
			outer.Tag():  outer,
			inner.Tag():  inner,
		},
		defaultOutbound: parent,
	}
	server := &Server{outbound: manager, logger: log.NewNOPFactory().Logger(), urlTestHistory: history}

	req := httptest.NewRequest(http.MethodGet, "/group/group-a/delay?url="+testServer.URL+"&timeout=1000", nil)
	req = req.WithContext(context.WithValue(req.Context(), CtxKeyProxy, parent))
	w := httptest.NewRecorder()

	getGroupDelay(server)(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, []string{"inner-urltest"}, history.storeMu)
	require.Empty(t, history.deleteMu)
	require.NotNil(t, history.LoadURLTestHistory("inner-urltest"))
	require.Equal(t, 1, inner.dialCount, "unresolved nested groups should still probe the best-effort fallback outbound")
	require.Zero(t, outer.dialCount)
}

func TestProxyInfoLoadsFallbackHistoryForUnresolvedGroup(t *testing.T) {
	history := newClashAPITestHistoryStorage()
	history.StoreURLTestHistory("selector-a", &adapter.URLTestHistory{Delay: 42})
	proxy := &clashAPITestOutbound{
		tag:     "selector-a",
		typ:     "selector",
		network: []string{"tcp", "udp"},
	}
	manager := &clashAPITestOutboundManager{outbounds: map[string]adapter.Outbound{proxy.Tag(): proxy}, defaultOutbound: proxy}
	server := &Server{outbound: manager, logger: log.NewNOPFactory().Logger(), urlTestHistory: history}

	info := proxyInfo(server, proxy)
	payload, err := info.MarshalJSON()
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(payload, &decoded))
	historyValue, exists := decoded["history"]
	require.True(t, exists)
	require.Len(t, historyValue.([]any), 1)
	entry := historyValue.([]any)[0].(map[string]any)
	require.Equal(t, float64(42), entry["delay"])
}

var _ adapter.OutboundGroup = (*clashAPITestOutbound)(nil)
var _ adapter.URLTestHistoryStorage = (*clashAPITestHistoryStorage)(nil)
var _ adapter.OutboundManager = (*clashAPITestOutboundManager)(nil)
