package group

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/interrupt"
	"github.com/sagernet/sing-box/common/urltest"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/stretchr/testify/require"
)

// sessionStub is a member that measures its connections, like anytls.
type sessionStub struct {
	failStub
	access   sync.Mutex
	latency  time.Duration
	measured bool
}

func newSessionStub(tag string, recorder *dialRecorder) *sessionStub {
	return &sessionStub{failStub: failStub{tag: tag, recorder: recorder}}
}

func (s *sessionStub) set(latency time.Duration) {
	s.access.Lock()
	defer s.access.Unlock()
	s.latency, s.measured = latency, true
}

func (s *sessionStub) clear() {
	s.access.Lock()
	defer s.access.Unlock()
	s.measured = false
}

func (s *sessionStub) SessionLatency() (time.Duration, bool) {
	s.access.Lock()
	defer s.access.Unlock()
	return s.latency, s.measured
}

var _ adapter.OutboundWithSessionLatency = (*sessionStub)(nil)

func storeDelay(history *urltest.HistoryStorage, tag string, delay uint16) {
	history.StoreURLTestHistory(tag, &adapter.URLTestHistory{Time: time.Now(), Delay: delay})
}

func TestSessionDelayRoundsUpAndNeverZero(t *testing.T) {
	t.Parallel()
	require.Equal(t, uint16(1), sessionDelay(0))
	require.Equal(t, uint16(1), sessionDelay(170*time.Microsecond))
	require.Equal(t, uint16(1), sessionDelay(time.Millisecond))
	require.Equal(t, uint16(2), sessionDelay(1200*time.Microsecond))
	require.Equal(t, uint16(65535), sessionDelay(time.Hour))
}

func TestMemberLatencySources(t *testing.T) {
	t.Parallel()
	history := urltest.NewHistoryStorage()
	measured := newSessionStub("anytls", &dialRecorder{})
	measured.set(12 * time.Millisecond)
	storeDelay(history, "anytls", 300)
	plain := &failStub{tag: "plain", recorder: &dialRecorder{}}
	storeDelay(history, "plain", 250)
	selector := &groupStub{seedStub: seedStub{tag: "selector"}, members: []string{"anytls"}, current: "anytls"}
	manager := &stubManager{outbounds: map[string]adapter.Outbound{"anytls": measured, "plain": plain, "selector": selector}}

	delay, source, ok := memberLatency(manager, history, measured, true)
	require.True(t, ok)
	require.Equal(t, uint16(12), delay)
	require.Equal(t, latencySourceSession, source)

	delay, source, ok = memberLatency(manager, history, measured, false)
	require.True(t, ok)
	require.Equal(t, uint16(300), delay, "url_test mode ignores session latency")
	require.Equal(t, latencySourceURLTest, source)

	delay, source, _ = memberLatency(manager, history, plain, true)
	require.Equal(t, uint16(250), delay, "a member without sessions falls back to its URL test")
	require.Equal(t, latencySourceURLTest, source)

	delay, source, _ = memberLatency(manager, history, selector, true)
	require.Equal(t, uint16(12), delay, "a group member is read through the outbound it routes to")
	require.Equal(t, latencySourceSession, source)

	measured.clear()
	delay, source, _ = memberLatency(manager, history, measured, true)
	require.Equal(t, uint16(300), delay, "nothing measured falls back to the URL test")
	require.Equal(t, latencySourceURLTest, source)

	_, _, ok = memberLatency(manager, history, &failStub{tag: "unknown"}, true)
	require.False(t, ok)
}

func newSessionURLTest(sessionMode bool, members ...adapter.Outbound) (*URLTest, *urltest.HistoryStorage) {
	history := urltest.NewHistoryStorage()
	group := &URLTestGroup{
		outbounds:      members,
		history:        history,
		tolerance:      50,
		interruptGroup: interrupt.NewGroup(),
		sessionLatency: sessionMode,
		logger:         log.NewNOPFactory().Logger(),
	}
	return &URLTest{group: group, logger: log.NewNOPFactory().Logger(), sessionLatency: sessionMode}, history
}

func dialOnce(t *testing.T, outbound *URLTest) {
	t.Helper()
	conn, err := outbound.DialContext(context.Background(), N.NetworkTCP, M.ParseSocksaddrHostPort("example.com", 443))
	require.NoError(t, err)
	_ = conn.Close()
}

func lastDial(recorder *dialRecorder) string {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return recorder.calls[len(recorder.calls)-1]
}

// In session mode every connection goes to the member whose connections are
// fastest at that moment, with no health check in between and no tolerance
// towards the member chosen before.
func TestURLTestSessionModeSelectsPerConnection(t *testing.T) {
	t.Parallel()
	recorder := &dialRecorder{}
	a := newSessionStub("a", recorder)
	b := newSessionStub("b", recorder)
	a.set(20 * time.Millisecond)
	b.set(10 * time.Millisecond)
	outbound, _ := newSessionURLTest(true, a, b)
	outbound.group.selectedOutboundTCP = a

	dialOnce(t, outbound)
	require.Equal(t, "b", lastDial(recorder))

	b.set(25 * time.Millisecond)
	dialOnce(t, outbound)
	require.Equal(t, "a", lastDial(recorder), "a 5ms difference is below tolerance but still switches")
}

func TestURLTestSessionModeFallsBackToURLTest(t *testing.T) {
	t.Parallel()
	recorder := &dialRecorder{}
	measured := newSessionStub("anytls", recorder)
	plain := &failStub{tag: "plain", recorder: recorder}
	outbound, history := newSessionURLTest(true, measured, plain)
	storeDelay(history, "plain", 30)
	storeDelay(history, "anytls", 400)

	measured.set(10 * time.Millisecond)
	dialOnce(t, outbound)
	require.Equal(t, "anytls", lastDial(recorder))

	measured.clear()
	dialOnce(t, outbound)
	require.Equal(t, "plain", lastDial(recorder), "without sessions anytls is ranked on its URL test")
}

func TestURLTestDefaultModeIgnoresSessionLatency(t *testing.T) {
	t.Parallel()
	recorder := &dialRecorder{}
	a := newSessionStub("a", recorder)
	b := newSessionStub("b", recorder)
	b.set(time.Millisecond)
	outbound, _ := newSessionURLTest(false, a, b)
	outbound.group.selectedOutboundTCP = a

	dialOnce(t, outbound)
	require.Equal(t, "a", lastDial(recorder), "the default mode dials the member chosen by the last check")
}

func TestLoadBalanceSessionModeRanksPerConnection(t *testing.T) {
	t.Parallel()
	recorder := &dialRecorder{}
	a := newSessionStub("a", recorder)
	b := newSessionStub("b", recorder)
	c := &failStub{tag: "c", recorder: recorder}
	history := urltest.NewHistoryStorage()
	storeDelay(history, "a", 200)
	storeDelay(history, "b", 200)
	storeDelay(history, "c", 15)
	lb := &LoadBalance{
		logger:           log.NewNOPFactory().Logger(),
		history:          history,
		interruptGroup:   interrupt.NewGroup(),
		primaryTags:      []string{"a", "b", "c"},
		primaryOutbounds: map[string]adapter.Outbound{"a": a, "b": b, "c": c},
		backupOutbounds:  map[string]adapter.Outbound{},
		topNPrimary:      1,
		strategy:         "random",
		emptyPoolAction:  "error",
		sessionLatency:   true,
	}
	lb.rebuildSnapshot()
	published := lb.snapshot.Load()
	require.Equal(t, "c", published.Candidates[0].Tag)

	pick := func() string {
		candidate, found, err := lb.pickCandidate(adapter.InboundContext{Network: N.NetworkTCP})
		require.NoError(t, err)
		require.True(t, found)
		return candidate.Tag
	}
	a.set(8 * time.Millisecond)
	b.set(12 * time.Millisecond)
	require.Equal(t, "a", pick())
	b.set(3 * time.Millisecond)
	require.Equal(t, "b", pick())
	b.clear()
	a.clear()
	require.Equal(t, "c", pick(), "without sessions every member is ranked on its URL test")

	require.Same(t, published, lb.snapshot.Load(), "per-connection ranking does not republish the pool")
}

func TestLatencyModeOptionChecked(t *testing.T) {
	t.Parallel()
	require.NoError(t, option.CheckLatencyMode(""))
	require.NoError(t, option.CheckLatencyMode(option.LatencyModeSession))
	require.Error(t, option.CheckLatencyMode("rtt"))
	err := option.LoadBalanceOutboundOptions{PrimaryOutbounds: []string{"a"}, LatencyMode: "rtt"}.Check()
	require.ErrorContains(t, err, "latency_mode")
}
