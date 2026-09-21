package group

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/interrupt"
	"github.com/sagernet/sing-box/common/transportstats"
	"github.com/sagernet/sing-box/common/urltest"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type dialRecorder struct {
	mu    sync.Mutex
	calls []string
	hook  func(tag string)
}

func (r *dialRecorder) record(tag string) {
	r.mu.Lock()
	r.calls = append(r.calls, tag)
	hook := r.hook
	r.mu.Unlock()
	if hook != nil {
		hook(tag)
	}
}

func (r *dialRecorder) got() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return strings.Join(r.calls, ",")
}

type failStub struct {
	tag      string
	networks []string
	fail     bool
	recorder *dialRecorder
}

func (s *failStub) Type() string { return "stub" }
func (s *failStub) Tag() string  { return s.tag }
func (s *failStub) Network() []string {
	if s.networks != nil {
		return s.networks
	}
	return []string{N.NetworkTCP, N.NetworkUDP}
}
func (s *failStub) Dependencies() []string { return nil }

func (s *failStub) DialContext(context.Context, string, M.Socksaddr) (net.Conn, error) {
	s.recorder.record(s.tag)
	if s.fail {
		return nil, errors.New("dial " + s.tag)
	}
	client, server := net.Pipe()
	_ = server.Close()
	return client, nil
}

func (s *failStub) ListenPacket(context.Context, M.Socksaddr) (net.PacketConn, error) {
	s.recorder.record(s.tag)
	if s.fail {
		return nil, errors.New("listen " + s.tag)
	}
	return &nopPacketConn{}, nil
}

type nopPacketConn struct{}

func (c *nopPacketConn) ReadFrom([]byte) (int, net.Addr, error) { return 0, nil, errors.New("closed") }
func (c *nopPacketConn) WriteTo(p []byte, _ net.Addr) (int, error) {
	return len(p), nil
}
func (c *nopPacketConn) Close() error                     { return nil }
func (c *nopPacketConn) LocalAddr() net.Addr              { return &net.UDPAddr{} }
func (c *nopPacketConn) SetDeadline(time.Time) error      { return nil }
func (c *nopPacketConn) SetReadDeadline(time.Time) error  { return nil }
func (c *nopPacketConn) SetWriteDeadline(time.Time) error { return nil }

type memberSpec struct {
	tag      string
	delay    uint16
	fail     bool
	networks []string
}

type failoverFixture struct {
	lb       *LoadBalance
	recorder *dialRecorder
	stubs    map[string]*failStub
}

func newFailoverFixture(primary, backup []memberSpec) *failoverFixture {
	recorder := &dialRecorder{}
	history := urltest.NewHistoryStorage()
	lb := &LoadBalance{
		logger:           log.NewNOPFactory().Logger(),
		history:          history,
		interruptGroup:   interrupt.NewGroup(),
		primaryOutbounds: make(map[string]adapter.Outbound),
		backupOutbounds:  make(map[string]adapter.Outbound),
		emptyPoolAction:  "error",
		strategy:         "random",
	}
	stubs := make(map[string]*failStub)
	add := func(spec memberSpec, isPrimary bool) {
		stub := &failStub{tag: spec.tag, networks: spec.networks, fail: spec.fail, recorder: recorder}
		stubs[spec.tag] = stub
		lb.tags = append(lb.tags, spec.tag)
		if isPrimary {
			lb.primaryTags = append(lb.primaryTags, spec.tag)
			lb.primaryOutbounds[spec.tag] = stub
		} else {
			lb.backupTags = append(lb.backupTags, spec.tag)
			lb.backupOutbounds[spec.tag] = stub
		}
		if spec.delay != 0 {
			history.StoreURLTestHistory(spec.tag, &adapter.URLTestHistory{Time: time.Now(), Delay: spec.delay})
		}
	}
	for _, spec := range primary {
		add(spec, true)
	}
	for _, spec := range backup {
		add(spec, false)
	}
	return &failoverFixture{lb: lb, recorder: recorder, stubs: stubs}
}

func (f *failoverFixture) candidate(tag string) Candidate {
	_, isPrimary := f.lb.primaryOutbounds[tag]
	return Candidate{Tag: tag, Outbound: f.stubs[tag], IsPrimary: isPrimary}
}

func (f *failoverFixture) dial(ctx context.Context, selected *Candidate) (Candidate, error) {
	found := selected != nil
	var sel Candidate
	if found {
		sel = *selected
	}
	return f.lb.walk(ctx, sel, found, N.NetworkTCP, func(ctx context.Context, detour adapter.Outbound) error {
		conn, err := detour.DialContext(ctx, N.NetworkTCP, M.Socksaddr{})
		if conn != nil {
			_ = conn.Close()
		}
		return err
	})
}

func (f *failoverFixture) listen(ctx context.Context, selected *Candidate) (Candidate, error) {
	found := selected != nil
	var sel Candidate
	if found {
		sel = *selected
	}
	return f.lb.walk(ctx, sel, found, N.NetworkUDP, func(ctx context.Context, detour adapter.Outbound) error {
		_, err := detour.ListenPacket(ctx, M.Socksaddr{})
		return err
	})
}

func attemptTags(attempts []Candidate) string {
	tags := make([]string, len(attempts))
	for i, candidate := range attempts {
		tags[i] = candidate.Tag
	}
	return strings.Join(tags, ",")
}

// PRD example: A=80 B=20 C=none | D=15 E=40.
func prdFixture(fail map[string]bool) *failoverFixture {
	return newFailoverFixture(
		[]memberSpec{{tag: "A", delay: 80, fail: fail["A"]}, {tag: "B", delay: 20, fail: fail["B"]}, {tag: "C", fail: fail["C"]}},
		[]memberSpec{{tag: "D", delay: 15, fail: fail["D"]}, {tag: "E", delay: 40, fail: fail["E"]}},
	)
}

func TestFailoverSelectedSucceedsOnlyDialsOnce(t *testing.T) {
	t.Parallel()
	f := prdFixture(nil)
	selected := f.candidate("A")
	used, err := f.dial(context.Background(), &selected)
	if err != nil {
		t.Fatal(err)
	}
	if used.Tag != "A" || f.recorder.got() != "A" {
		t.Fatalf("used %q calls %q", used.Tag, f.recorder.got())
	}
	if f.lb.history.LoadURLTestHistory("A") == nil {
		t.Fatal("history of successful member must be kept")
	}
}

func TestFailoverNextPrimaryByLowestDelay(t *testing.T) {
	t.Parallel()
	f := prdFixture(map[string]bool{"A": true})
	selected := f.candidate("A")
	used, err := f.dial(context.Background(), &selected)
	if err != nil {
		t.Fatal(err)
	}
	if used.Tag != "B" || f.recorder.got() != "A,B" {
		t.Fatalf("used %q calls %q", used.Tag, f.recorder.got())
	}
	if f.lb.history.LoadURLTestHistory("A") != nil {
		t.Fatal("failed member history must be deleted")
	}
}

func TestFailoverExhaustsPrimariesBeforeBackups(t *testing.T) {
	t.Parallel()
	f := prdFixture(map[string]bool{"A": true, "B": true, "C": true})
	selected := f.candidate("A")
	used, err := f.dial(context.Background(), &selected)
	if err != nil {
		t.Fatal(err)
	}
	if used.Tag != "D" || f.recorder.got() != "A,B,C,D" {
		t.Fatalf("used %q calls %q", used.Tag, f.recorder.got())
	}
}

func TestFailoverAllFailReturnsLastError(t *testing.T) {
	t.Parallel()
	f := prdFixture(map[string]bool{"A": true, "B": true, "C": true, "D": true, "E": true})
	selected := f.candidate("A")
	_, err := f.dial(context.Background(), &selected)
	if err == nil || err.Error() != "dial E" {
		t.Fatalf("err %v want dial E", err)
	}
	if f.recorder.got() != "A,B,C,D,E" {
		t.Fatalf("calls %q", f.recorder.got())
	}
}

func TestFailoverNoBackupFailsAfterPrimaries(t *testing.T) {
	t.Parallel()
	f := newFailoverFixture([]memberSpec{{tag: "A", delay: 10, fail: true}, {tag: "B", delay: 20, fail: true}}, nil)
	selected := f.candidate("B")
	_, err := f.dial(context.Background(), &selected)
	if err == nil || err.Error() != "dial A" {
		t.Fatalf("err %v want dial A", err)
	}
	if f.recorder.got() != "B,A" {
		t.Fatalf("calls %q", f.recorder.got())
	}
}

func TestFailoverAttemptOrderFromBackupSelected(t *testing.T) {
	t.Parallel()
	f := prdFixture(nil)
	selected := f.candidate("D")
	got := attemptTags(f.lb.failoverAttempts(&selected, N.NetworkTCP))
	if got != "D,B,A,C,E" {
		t.Fatalf("attempts %q want D,B,A,C,E", got)
	}
}

func TestFailoverUnmeasuredAfterMeasuredByTag(t *testing.T) {
	t.Parallel()
	f := newFailoverFixture([]memberSpec{{tag: "z"}, {tag: "y", delay: 50}, {tag: "b"}, {tag: "x", delay: 50}, {tag: "w", delay: 90}}, nil)
	got := attemptTags(f.lb.failoverAttempts(nil, N.NetworkTCP))
	if got != "x,y,w,b,z" {
		t.Fatalf("attempts %q want x,y,w,b,z", got)
	}
}

// Fail-over orders on the raw last delay, not on the sorter, so neither the
// latency history nor the health timeout may reorder or drop an attempt.
func TestFailoverIgnoresSorterAndTimeout(t *testing.T) {
	t.Parallel()
	f := newFailoverFixture([]memberSpec{{tag: "a", delay: 30}, {tag: "b", delay: 60000}}, nil)
	f.lb.timeout = time.Second
	memberSorter, err := newSorter(map[string]float64{"latency_avg_5m": 1})
	if err != nil {
		t.Fatal(err)
	}
	f.lb.sorter = memberSorter
	f.lb.latencyStats = make(map[string]*transportstats.Recorder)
	// A history that would rank "a" last if the sorter were consulted.
	f.lb.observeDelay("a", 900)
	got := attemptTags(f.lb.failoverAttempts(nil, N.NetworkTCP))
	if got != "a,b" {
		t.Fatalf("attempts %q want a,b", got)
	}
}

func TestFailoverEmptyKeyErrorDoesNotWalk(t *testing.T) {
	t.Parallel()
	f := prdFixture(nil)
	f.lb.strategy = "consistent_hash"
	f.lb.hash = &option.LoadBalanceHashOptions{KeyParts: []string{"src_ip"}, OnEmptyKey: "error"}
	f.lb.seedInitialSnapshot()
	_, found, err := f.lb.pickCandidate(adapter.InboundContext{})
	if err == nil || found || !errors.Is(err, ErrEmptyHashKey) && !strings.Contains(err.Error(), ErrEmptyHashKey.Error()) {
		t.Fatalf("found %v err %v want empty hash key", found, err)
	}
	if f.recorder.got() != "" {
		t.Fatalf("unexpected dials %q", f.recorder.got())
	}
}

func TestFailoverEmptySnapshotErrorActionWalksByDelay(t *testing.T) {
	t.Parallel()
	f := prdFixture(map[string]bool{"B": true, "A": true})
	f.lb.snapshot.Store(&CandidateSnapshot{})
	selected, found, err := f.lb.pickCandidate(adapter.InboundContext{})
	if err != nil || found {
		t.Fatalf("found %v err %v", found, err)
	}
	used, err := f.lb.walk(context.Background(), selected, found, N.NetworkTCP, func(ctx context.Context, detour adapter.Outbound) error {
		_, dialErr := detour.DialContext(ctx, N.NetworkTCP, M.Socksaddr{})
		return dialErr
	})
	if err != nil {
		t.Fatal(err)
	}
	if used.Tag != "C" || f.recorder.got() != "B,A,C" {
		t.Fatalf("used %q calls %q", used.Tag, f.recorder.got())
	}
	if _, err := f.lb.selectCandidate(adapter.InboundContext{}); err == nil {
		t.Fatal("selectCandidate must still report empty pool")
	}
}

func TestFailoverEmptySnapshotRandomActionSelectsFirst(t *testing.T) {
	t.Parallel()
	f := prdFixture(nil)
	f.lb.emptyPoolAction = "random"
	f.lb.snapshot.Store(&CandidateSnapshot{})
	selected, found, err := f.lb.pickCandidate(adapter.InboundContext{})
	if err != nil || !found {
		t.Fatalf("found %v err %v", found, err)
	}
	attempts := f.lb.failoverAttempts(&selected, N.NetworkTCP)
	if len(attempts) != 5 || attempts[0].Tag != selected.Tag {
		t.Fatalf("attempts %q selected %q", attemptTags(attempts), selected.Tag)
	}
}

func TestFailoverNoConfiguredMembersIsEmptyPool(t *testing.T) {
	t.Parallel()
	f := newFailoverFixture(nil, nil)
	_, err := f.dial(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "empty candidate pool") {
		t.Fatalf("err %v want empty candidate pool", err)
	}
}

func TestFailoverSkipsMembersWithoutUDP(t *testing.T) {
	t.Parallel()
	tcpOnly := []string{N.NetworkTCP}
	f := newFailoverFixture(
		[]memberSpec{{tag: "A", delay: 10, networks: tcpOnly}, {tag: "B", delay: 20, fail: true}, {tag: "C", delay: 30}},
		nil,
	)
	selected := f.candidate("A")
	used, err := f.listen(context.Background(), &selected)
	if err != nil {
		t.Fatal(err)
	}
	if used.Tag != "C" || f.recorder.got() != "B,C" {
		t.Fatalf("used %q calls %q", used.Tag, f.recorder.got())
	}
	if f.lb.history.LoadURLTestHistory("A") == nil {
		t.Fatal("skipped member history must be kept")
	}
}

func TestFailoverListenPacketSameOrder(t *testing.T) {
	t.Parallel()
	f := prdFixture(map[string]bool{"A": true, "B": true, "C": true, "D": true})
	selected := f.candidate("A")
	used, err := f.listen(context.Background(), &selected)
	if err != nil {
		t.Fatal(err)
	}
	if used.Tag != "E" || f.recorder.got() != "A,B,C,D,E" {
		t.Fatalf("used %q calls %q", used.Tag, f.recorder.got())
	}
}

func TestFailoverStopsOnContextCancel(t *testing.T) {
	t.Parallel()
	f := prdFixture(map[string]bool{"A": true, "B": true, "C": true, "D": true, "E": true})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.recorder.hook = func(tag string) {
		if tag == "B" {
			cancel()
		}
	}
	selected := f.candidate("A")
	_, err := f.dial(ctx, &selected)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err %v want context.Canceled", err)
	}
	if f.recorder.got() != "A,B" {
		t.Fatalf("calls %q want A,B", f.recorder.got())
	}
}

func TestFailoverNestedGroupIsSingleAttempt(t *testing.T) {
	t.Parallel()
	f := newFailoverFixture([]memberSpec{{tag: "leaf", delay: 50}}, nil)
	child := &LoadBalance{
		Adapter: outbound.NewAdapter(C.TypeLoadBalance, "child", []string{N.NetworkTCP, N.NetworkUDP}, []string{"inner"}),
	}
	child.snapshot.Store(&CandidateSnapshot{
		Candidates: []Candidate{{Tag: "inner", Latency: 10, Outbound: &seedStub{tag: "inner"}}},
	})
	f.lb.primaryTags = append([]string{"child"}, f.lb.primaryTags...)
	f.lb.primaryOutbounds["child"] = child
	childCalls := 0
	used, err := f.lb.walk(context.Background(), Candidate{}, false, N.NetworkTCP, func(ctx context.Context, detour adapter.Outbound) error {
		if detour == child {
			childCalls++
			return errors.New("child failed")
		}
		_, dialErr := detour.DialContext(ctx, N.NetworkTCP, M.Socksaddr{})
		return dialErr
	})
	if err != nil {
		t.Fatal(err)
	}
	if childCalls != 1 || used.Tag != "leaf" || f.recorder.got() != "leaf" {
		t.Fatalf("childCalls %d used %q calls %q", childCalls, used.Tag, f.recorder.got())
	}
}

func TestFailoverLeavesSnapshotUntouched(t *testing.T) {
	t.Parallel()
	f := prdFixture(map[string]bool{"A": true})
	f.lb.seedInitialSnapshot()
	before := f.lb.snapshot.Load()
	selected := f.candidate("A")
	if _, err := f.dial(context.Background(), &selected); err != nil {
		t.Fatal(err)
	}
	after := f.lb.snapshot.Load()
	if before != after || after.Generation != before.Generation {
		t.Fatal("walk must not rebuild snapshot")
	}
	next, found, err := f.lb.pickCandidate(adapter.InboundContext{})
	if err != nil || !found {
		t.Fatalf("found %v err %v", found, err)
	}
	if _, ok := f.lb.primaryOutbounds[next.Tag]; !ok {
		t.Fatalf("next pick %q not from snapshot", next.Tag)
	}
}
