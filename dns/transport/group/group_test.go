package group

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/service"

	mDNS "github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

type stubMember struct {
	adapter.DNSTransport
	tag        string
	memberType string
	delay      time.Duration
	err        error
	rcode      int
	calls      atomic.Int32
	cancelled  atomic.Int32
}

func (m *stubMember) Tag() string  { return m.tag }
func (m *stubMember) Type() string { return m.memberType }

func (m *stubMember) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	m.calls.Add(1)
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			m.cancelled.Add(1)
			return nil, ctx.Err()
		}
	}
	if m.err != nil {
		return nil, m.err
	}
	response := new(mDNS.Msg)
	response.SetReply(message)
	response.Rcode = m.rcode
	if m.rcode == mDNS.RcodeSuccess {
		response.Answer = append(response.Answer, &mDNS.TXT{
			Hdr: mDNS.RR_Header{Name: message.Question[0].Name, Rrtype: mDNS.TypeTXT, Class: mDNS.ClassINET, Ttl: 60},
			Txt: []string{m.tag},
		})
	}
	return response, nil
}

type stubManager struct {
	adapter.DNSTransportManager
	transports map[string]adapter.DNSTransport
}

func (m *stubManager) Transport(tag string) (adapter.DNSTransport, bool) {
	transport, loaded := m.transports[tag]
	return transport, loaded
}

func member(tag string) *stubMember {
	return &stubMember{tag: tag, memberType: C.DNSTypeUDP}
}

func newGroup(t *testing.T, options option.GroupDNSServerOptions, members ...*stubMember) *Transport {
	t.Helper()
	manager := &stubManager{transports: make(map[string]adapter.DNSTransport)}
	for _, m := range members {
		manager.transports[m.tag] = m
	}
	ctx := service.ContextWith[adapter.DNSTransportManager](context.Background(), manager)
	transport, err := NewTransport(ctx, log.NewNOPFactory().Logger(), "group", options)
	require.NoError(t, err)
	require.NoError(t, transport.Start(adapter.StartStateStart))
	return transport.(*Transport)
}

func query(t *testing.T, transport *Transport) (*mDNS.Msg, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	message := new(mDNS.Msg)
	message.SetQuestion("example.org.", mDNS.TypeTXT)
	return transport.Exchange(ctx, message)
}

func answeredBy(t *testing.T, response *mDNS.Msg) string {
	t.Helper()
	require.Len(t, response.Answer, 1)
	return response.Answer[0].(*mDNS.TXT).Txt[0]
}

func TestGroupDependenciesAreMembers(t *testing.T) {
	transport := newGroup(t, option.GroupDNSServerOptions{Servers: []string{"a", "b"}, Fallbacks: []string{"b", "c"}},
		member("a"), member("b"), member("c"))
	require.Equal(t, []string{"a", "b", "c"}, transport.Dependencies())
}

func TestGroupOptionsValidated(t *testing.T) {
	ctx := service.ContextWith[adapter.DNSTransportManager](context.Background(), &stubManager{})
	logger := log.NewNOPFactory().Logger()
	_, err := NewTransport(ctx, logger, "group", option.GroupDNSServerOptions{})
	require.ErrorContains(t, err, "missing servers")
	_, err = NewTransport(ctx, logger, "group", option.GroupDNSServerOptions{Servers: []string{"a"}, Mode: "fastest"})
	require.ErrorContains(t, err, "unknown mode")
	_, err = NewTransport(ctx, logger, "group", option.GroupDNSServerOptions{Servers: []string{"group"}})
	require.ErrorContains(t, err, "lists itself")
}

func TestGroupMemberTypes(t *testing.T) {
	for _, memberType := range []string{C.DNSTypeUDP, C.DNSTypeTCP, C.DNSTypeTLS, C.DNSTypeHTTPS} {
		m := member("m")
		m.memberType = memberType
		newGroup(t, option.GroupDNSServerOptions{Servers: []string{"m"}}, m)
	}
	for _, memberType := range []string{C.DNSTypeQUIC, C.DNSTypeHTTP3, C.DNSTypeFakeIP, C.DNSTypeHosts, C.DNSTypeLocal, C.DNSTypeGroup, C.DNSTypeAnyTLS} {
		t.Run(memberType, func(t *testing.T) {
			m := member("m")
			m.memberType = memberType
			manager := &stubManager{transports: map[string]adapter.DNSTransport{"m": m}}
			ctx := service.ContextWith[adapter.DNSTransportManager](context.Background(), manager)
			transport, err := NewTransport(ctx, log.NewNOPFactory().Logger(), "group", option.GroupDNSServerOptions{Servers: []string{"m"}})
			require.NoError(t, err)
			err = transport.Start(adapter.StartStateStart)
			require.ErrorContains(t, err, "only udp, tcp, tls, https")
			require.ErrorContains(t, err, memberType)
		})
	}
	// A fallback is held to the same rule.
	bad := member("bad")
	bad.memberType = C.DNSTypeQUIC
	manager := &stubManager{transports: map[string]adapter.DNSTransport{"a": member("a"), "bad": bad}}
	ctx := service.ContextWith[adapter.DNSTransportManager](context.Background(), manager)
	transport, err := NewTransport(ctx, log.NewNOPFactory().Logger(), "group", option.GroupDNSServerOptions{Servers: []string{"a"}, Fallbacks: []string{"bad"}})
	require.NoError(t, err)
	require.Error(t, transport.Start(adapter.StartStateStart))
}

func TestGroupSequential(t *testing.T) {
	a, b := member("a"), member("b")
	a.err = errors.New("down")
	transport := newGroup(t, option.GroupDNSServerOptions{Servers: []string{"a", "b"}, Mode: ModeSequential}, a, b)
	response, err := query(t, transport)
	require.NoError(t, err)
	require.Equal(t, "b", answeredBy(t, response))
	require.EqualValues(t, 1, a.calls.Load())
}

func TestGroupServerFailureMovesOn(t *testing.T) {
	a, b := member("a"), member("b")
	a.rcode = mDNS.RcodeServerFailure
	transport := newGroup(t, option.GroupDNSServerOptions{Servers: []string{"a", "b"}, Mode: ModeSequential}, a, b)
	response, err := query(t, transport)
	require.NoError(t, err)
	require.Equal(t, "b", answeredBy(t, response))
}

func TestGroupNameErrorIsAnAnswer(t *testing.T) {
	a, b := member("a"), member("b")
	a.rcode = mDNS.RcodeNameError
	transport := newGroup(t, option.GroupDNSServerOptions{Servers: []string{"a", "b"}, Mode: ModeSequential}, a, b)
	response, err := query(t, transport)
	require.NoError(t, err)
	require.Equal(t, mDNS.RcodeNameError, response.Rcode)
	require.Zero(t, b.calls.Load())
}

func TestGroupAllServerFailureReturnsTheAnswer(t *testing.T) {
	a := member("a")
	a.rcode = mDNS.RcodeServerFailure
	transport := newGroup(t, option.GroupDNSServerOptions{Servers: []string{"a"}}, a)
	response, err := query(t, transport)
	require.NoError(t, err)
	require.Equal(t, mDNS.RcodeServerFailure, response.Rcode)
}

func TestGroupParallelTakesFirstAnswer(t *testing.T) {
	slow, fast := member("slow"), member("fast")
	slow.delay = time.Second
	fast.delay = 10 * time.Millisecond
	transport := newGroup(t, option.GroupDNSServerOptions{Servers: []string{"slow", "fast"}, Mode: ModeParallel}, slow, fast)
	start := time.Now()
	response, err := query(t, transport)
	require.NoError(t, err)
	require.Equal(t, "fast", answeredBy(t, response))
	require.Less(t, time.Since(start), 500*time.Millisecond)
	require.Eventually(t, func() bool { return slow.cancelled.Load() == 1 }, time.Second, 10*time.Millisecond)
	// The cancelled server is not penalised for losing the race.
	transport.rttAccess.Lock()
	_, slowMeasured := transport.rtt["slow"]
	transport.rttAccess.Unlock()
	require.False(t, slowMeasured)
}

func TestGroupParallelAllFail(t *testing.T) {
	a, b := member("a"), member("b")
	a.err = errors.New("a down")
	b.err = errors.New("b down")
	transport := newGroup(t, option.GroupDNSServerOptions{Servers: []string{"a", "b"}, Mode: ModeParallel}, a, b)
	_, err := query(t, transport)
	require.ErrorContains(t, err, "a down")
	require.ErrorContains(t, err, "b down")
}

func TestGroupFallbackOnlyAfterServersFail(t *testing.T) {
	a, fallback := member("a"), member("fallback")
	transport := newGroup(t, option.GroupDNSServerOptions{Servers: []string{"a"}, Fallbacks: []string{"fallback"}}, a, fallback)
	response, err := query(t, transport)
	require.NoError(t, err)
	require.Equal(t, "a", answeredBy(t, response))
	require.Zero(t, fallback.calls.Load())

	a.err = errors.New("down")
	response, err = query(t, transport)
	require.NoError(t, err)
	require.Equal(t, "fallback", answeredBy(t, response))
}

func TestGroupFallbackFailureKeepsBothErrors(t *testing.T) {
	a, fallback := member("a"), member("fallback")
	a.err = errors.New("primary down")
	fallback.err = errors.New("fallback down")
	transport := newGroup(t, option.GroupDNSServerOptions{Servers: []string{"a"}, Fallbacks: []string{"fallback"}}, a, fallback)
	_, err := query(t, transport)
	require.ErrorContains(t, err, "primary down")
	require.ErrorContains(t, err, "fallback down")
}

// A server that hangs gets a share of the time, not all of it, so the next
// server still has time to answer.
func TestGroupSequentialSharesTime(t *testing.T) {
	hang, ok := member("hang"), member("ok")
	hang.delay = time.Hour
	transport := newGroup(t, option.GroupDNSServerOptions{Servers: []string{"hang", "ok"}, Mode: ModeSequential}, hang, ok)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	message := new(mDNS.Msg)
	message.SetQuestion("example.org.", mDNS.TypeTXT)
	response, err := transport.Exchange(ctx, message)
	require.NoError(t, err)
	require.Equal(t, "ok", answeredBy(t, response))
}

func TestGroupLoadBalancePrefersFasterServer(t *testing.T) {
	fast, slow := member("fast"), member("slow")
	transport := newGroup(t, option.GroupDNSServerOptions{Servers: []string{"fast", "slow"}}, fast, slow)
	transport.recordRTT("fast", 5*time.Millisecond)
	transport.recordRTT("slow", 200*time.Millisecond)
	first := map[string]int{}
	for i := 0; i < 2000; i++ {
		first[transport.weightedOrder(transport.servers)[0].Tag()]++
	}
	// Weights are 1/5ms and 1/200ms, so fast leads about 40 times as often.
	require.Greater(t, first["fast"], 1800)
	require.Greater(t, first["slow"], 0)
}

func TestGroupLoadBalanceSpreadsAcrossUnmeasured(t *testing.T) {
	a, b := member("a"), member("b")
	transport := newGroup(t, option.GroupDNSServerOptions{Servers: []string{"a", "b"}}, a, b)
	first := map[string]int{}
	for i := 0; i < 1000; i++ {
		order := transport.weightedOrder(transport.servers)
		require.Len(t, order, 2)
		require.NotEqual(t, order[0].Tag(), order[1].Tag())
		first[order[0].Tag()]++
	}
	require.Greater(t, first["a"], 350)
	require.Greater(t, first["b"], 350)
}

func TestGroupLoadBalanceAvoidsFailedServer(t *testing.T) {
	bad, good := member("bad"), member("good")
	bad.err = &net.OpError{Op: "dial", Err: errors.New("refused")}
	transport := newGroup(t, option.GroupDNSServerOptions{Servers: []string{"bad", "good"}}, bad, good)
	for i := 0; i < 50; i++ {
		response, err := query(t, transport)
		require.NoError(t, err)
		require.Equal(t, "good", answeredBy(t, response))
	}
	// After failing once, bad carries the failure penalty and is rarely tried
	// first again.
	require.Less(t, bad.calls.Load(), int32(10))
}

func TestGroupReset(t *testing.T) {
	transport := newGroup(t, option.GroupDNSServerOptions{Servers: []string{"a"}}, member("a"))
	transport.recordRTT("a", time.Second)
	transport.Reset()
	transport.rttAccess.Lock()
	defer transport.rttAccess.Unlock()
	require.Empty(t, transport.rtt)
}

func TestGroupConcurrentQueries(t *testing.T) {
	a, b := member("a"), member("b")
	transport := newGroup(t, option.GroupDNSServerOptions{Servers: []string{"a", "b"}}, a, b)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := query(t, transport)
			require.NoError(t, err)
		}()
	}
	wg.Wait()
}

func TestGroupRefusedMovesOn(t *testing.T) {
	a, b := member("a"), member("b")
	a.rcode = mDNS.RcodeRefused
	transport := newGroup(t, option.GroupDNSServerOptions{Servers: []string{"a", "b"}, Mode: ModeSequential}, a, b)
	response, err := query(t, transport)
	require.NoError(t, err)
	require.Equal(t, "b", answeredBy(t, response))
}

// Servers that never answer still leave the fallbacks time to answer.
func TestGroupFallbackAfterServersHang(t *testing.T) {
	for _, mode := range []string{ModeLoadBalance, ModeParallel, ModeSequential} {
		t.Run(mode, func(t *testing.T) {
			a, b, fallback := member("a"), member("b"), member("fallback")
			a.delay = time.Hour
			b.delay = time.Hour
			transport := newGroup(t, option.GroupDNSServerOptions{Servers: []string{"a", "b"}, Fallbacks: []string{"fallback"}, Mode: mode}, a, b, fallback)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			message := new(mDNS.Msg)
			message.SetQuestion("example.org.", mDNS.TypeTXT)
			response, err := transport.Exchange(ctx, message)
			require.NoError(t, err)
			require.Equal(t, "fallback", answeredBy(t, response))
		})
	}
}

// A server that fails fast must not look fast to load balancing.
func TestGroupFastServerFailureIsPenalised(t *testing.T) {
	broken, good := member("broken"), member("good")
	broken.rcode = mDNS.RcodeServerFailure
	transport := newGroup(t, option.GroupDNSServerOptions{Servers: []string{"broken", "good"}, Mode: ModeSequential}, broken, good)
	_, err := query(t, transport)
	require.NoError(t, err)
	transport.rttAccess.Lock()
	defer transport.rttAccess.Unlock()
	require.Equal(t, failurePenalty, transport.rtt["broken"])
	require.Less(t, transport.rtt["good"], time.Second)
}

// A server that runs out of its share of the time is recorded as slow by the
// time it had, not as failed.
func TestGroupShareTimeoutIsNotAFailure(t *testing.T) {
	slow, good := member("slow"), member("good")
	slow.delay = time.Hour
	transport := newGroup(t, option.GroupDNSServerOptions{Servers: []string{"slow", "good"}, Mode: ModeSequential}, slow, good)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	message := new(mDNS.Msg)
	message.SetQuestion("example.org.", mDNS.TypeTXT)
	_, err := transport.Exchange(ctx, message)
	require.NoError(t, err)
	transport.rttAccess.Lock()
	defer transport.rttAccess.Unlock()
	require.Less(t, transport.rtt["slow"], time.Second)
	require.Greater(t, transport.rtt["slow"], 50*time.Millisecond)
}
