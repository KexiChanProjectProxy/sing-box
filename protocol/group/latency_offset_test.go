package group

import (
	"context"
	"testing"
	"time"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	N "github.com/sagernet/sing/common/network"

	"github.com/stretchr/testify/require"
)

func TestCheckLatencyOffsetRejectsUnknownMember(t *testing.T) {
	t.Parallel()
	require.NoError(t, option.CheckLatencyOffset(nil, []string{"a"}))
	require.NoError(t, option.CheckLatencyOffset(map[string]int32{"a": 10, "b": -10}, []string{"a", "b"}))
	require.Error(t, option.CheckLatencyOffset(map[string]int32{"c": 10}, []string{"a", "b"}))
}

func TestNewLoadBalanceValidatesLatencyOffset(t *testing.T) {
	t.Parallel()
	_, err := newLoadBalanceForTest(t, option.LoadBalanceOutboundOptions{
		PrimaryOutbounds: []string{"a"},
		BackupOutbounds:  []string{"b"},
		LatencyOffset:    map[string]int32{"a": 10, "b": -10},
	})
	require.NoError(t, err, "primary and backup members both take an offset")

	_, err = newLoadBalanceForTest(t, option.LoadBalanceOutboundOptions{
		PrimaryOutbounds: []string{"a"},
		LatencyOffset:    map[string]int32{"typo": 10},
	})
	require.Error(t, err)
}

func TestNewURLTestValidatesLatencyOffset(t *testing.T) {
	t.Parallel()
	_, err := NewURLTest(context.Background(), nil, log.NewNOPFactory().Logger(), "urltest", option.URLTestOutboundOptions{
		Outbounds:     []string{"a", "b"},
		LatencyOffset: map[string]int32{"c": 10},
	})
	require.Error(t, err)
}

// A positive offset makes a faster member lose the top spot, a negative one
// lets a slower member win, and the reported latency stays the measured one.
func TestLoadBalanceLatencyOffsetShiftsRanking(t *testing.T) {
	t.Parallel()
	lb := newHealthFixture(t, map[string]uint16{"a": 50, "b": 80})
	lb.topNPrimary = 1

	candidates := lb.computeCandidates(nil, true)
	require.Equal(t, "a", candidates[0].Tag)

	lb.latencyOffset = map[string]int32{"a": 40}
	candidates = lb.computeCandidates(nil, true)
	require.Len(t, candidates, 1)
	require.Equal(t, "b", candidates[0].Tag, "a ranks as 90ms against b's 80ms")

	lb.latencyOffset = map[string]int32{"b": -40}
	candidates = lb.computeCandidates(nil, true)
	require.Equal(t, "b", candidates[0].Tag, "b ranks as 40ms against a's 50ms")
	require.Equal(t, uint16(80), candidates[0].Latency, "the measured latency is not shifted")
	require.Equal(t, float64(40), candidates[0].Score)
}

// A negative offset larger than the latency must not wrap around.
func TestLoadBalanceLatencyOffsetBelowZero(t *testing.T) {
	t.Parallel()
	lb := newHealthFixture(t, map[string]uint16{"a": 10, "b": 30})
	lb.topNPrimary = 1
	lb.latencyOffset = map[string]int32{"b": -1000}
	candidates := lb.computeCandidates(nil, true)
	require.Equal(t, "b", candidates[0].Tag)
	require.Equal(t, float64(-970), candidates[0].Score)
}

// The timeout reads the measured latency, so a negative offset cannot keep an
// unhealthy member in the pool.
func TestLoadBalanceLatencyOffsetDoesNotAffectHealth(t *testing.T) {
	t.Parallel()
	lb := newHealthFixture(t, map[string]uint16{"a": 50, "slow": 2000})
	lb.timeout = time.Second
	lb.latencyOffset = map[string]int32{"slow": -1900}
	candidates := lb.computeCandidates(nil, true)
	require.Len(t, candidates, 1)
	require.Equal(t, "a", candidates[0].Tag)
}

// Tolerance compares shifted scores: an incumbent holds its place until a
// challenger is better by more than the tolerance after offsets.
func TestLoadBalanceLatencyOffsetWithTolerance(t *testing.T) {
	t.Parallel()
	lb := newHealthFixture(t, map[string]uint16{"a": 50, "b": 45})
	lb.topNPrimary = 1
	lb.tolerance = 10
	lb.latencyOffset = map[string]int32{"a": -20}
	previous := lb.computeCandidates(nil, true)
	require.Equal(t, "a", previous[0].Tag, "a ranks as 30ms")

	lb.latencyOffset = map[string]int32{"a": -5}
	candidates := lb.computeCandidates(previous, true)
	require.Equal(t, "a", candidates[0].Tag, "45 against 45 keeps the incumbent")

	lb.latencyOffset = map[string]int32{"a": 10}
	candidates = lb.computeCandidates(previous, true)
	require.Equal(t, "b", candidates[0].Tag, "60 against 45 is beyond the tolerance")
}

// When the sorter cannot tell two members apart, the tie is broken on the
// shifted latency, not the raw one, so an offset is never undone by it.
func TestLatencyOffsetBreaksScoreTies(t *testing.T) {
	t.Parallel()
	incumbent := Candidate{Tag: "a", Latency: 100, Offset: -60, Score: 7}
	challenger := Candidate{Tag: "b", Latency: 50, Score: 7}
	require.False(t, outranksBeyondTolerance(challenger, incumbent, 0), "a ranks as 40ms against b's 50ms")
	require.True(t, rankAfter(challenger, incumbent))

	candidates := []Candidate{challenger, incumbent}
	sortCandidatesByScore(candidates)
	require.Equal(t, "a", candidates[0].Tag)
}

// Fail-over walks the remaining members in order of latency plus offset.
func TestLoadBalanceLatencyOffsetOrdersFailover(t *testing.T) {
	t.Parallel()
	f := prdFixture(nil)
	// Without offsets: B(20) A(80) C(none) | D(15) E(40).
	require.Equal(t, "B,A,C,D,E", attemptTags(f.lb.failoverAttempts(nil, N.NetworkTCP)))

	f.lb.latencyOffset = map[string]int32{"B": 100, "E": -30}
	require.Equal(t, "A,B,C,E,D", attemptTags(f.lb.failoverAttempts(nil, N.NetworkTCP)))
}

func TestURLTestLatencyOffsetSelect(t *testing.T) {
	t.Parallel()
	recorder := &dialRecorder{}
	a := &failStub{tag: "a", recorder: recorder}
	b := &failStub{tag: "b", recorder: recorder}
	outbound, history := newSessionURLTest(false, a, b)
	group := outbound.group
	storeDelay(history, "a", 50)
	storeDelay(history, "b", 120)

	selected, ok := group.Select(N.NetworkTCP)
	require.True(t, ok)
	require.Equal(t, "a", selected.Tag())

	// The fixture's tolerance is 50ms, and Select applies it between members
	// even without an incumbent.
	group.latencyOffset = map[string]int32{"b": -100}
	selected, _ = group.Select(N.NetworkTCP)
	require.Equal(t, "a", selected.Tag(), "b at 20ms is not 50ms better than a")

	group.latencyOffset = map[string]int32{"b": -200}
	selected, _ = group.Select(N.NetworkTCP)
	require.Equal(t, "b", selected.Tag(), "b at -80ms is, and the sum does not wrap")

	group.selectedOutboundTCP = b
	group.latencyOffset = map[string]int32{"b": -200, "a": -20}
	selected, _ = group.Select(N.NetworkTCP)
	require.Equal(t, "b", selected.Tag(), "a at 30ms does not displace b at -80ms")

	group.latencyOffset = map[string]int32{"b": -200, "a": -1000}
	selected, _ = group.Select(N.NetworkTCP)
	require.Equal(t, "a", selected.Tag(), "a at -950ms does")
}

func TestURLTestLatencyOffsetSessionMode(t *testing.T) {
	t.Parallel()
	recorder := &dialRecorder{}
	a := newSessionStub("a", recorder)
	b := newSessionStub("b", recorder)
	a.set(10 * time.Millisecond)
	b.set(30 * time.Millisecond)
	outbound, _ := newSessionURLTest(true, a, b)

	dialOnce(t, outbound)
	require.Equal(t, "a", lastDial(recorder))

	outbound.group.latencyOffset = map[string]int32{"a": 25}
	dialOnce(t, outbound)
	require.Equal(t, "b", lastDial(recorder), "a ranks as 35ms against b's 30ms")

	selected, delay, _ := outbound.group.selectLowest(N.NetworkTCP)
	require.Equal(t, "b", selected.Tag())
	require.Equal(t, uint16(30), delay, "the reported delay is the measured one")
}
