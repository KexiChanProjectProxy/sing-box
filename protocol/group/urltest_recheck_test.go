package group

import (
	"context"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/urltest"
	"github.com/sagernet/sing-box/log"
)

const recheckInterval = 30 * time.Second

func TestURLTestHistoryFresh(t *testing.T) {
	t.Parallel()
	now := time.Now()
	cases := []struct {
		name  string
		age   time.Duration
		fresh bool
	}{
		{"just measured", time.Second, true},
		{"quarter interval", recheckInterval / 4, true},
		{"just over half", recheckInterval/2 + time.Second, false},
		// What the next tick sees: the result was stored when the previous
		// probe finished, a little after the previous tick.
		{"next tick", recheckInterval - 2*time.Second, false},
		{"whole interval", recheckInterval, false},
	}
	for _, c := range cases {
		history := &adapter.URLTestHistory{Time: now.Add(-c.age), Delay: 50}
		if got := urlTestHistoryFresh(history, recheckInterval); got != c.fresh {
			t.Errorf("%s: fresh = %v, want %v", c.name, got, c.fresh)
		}
	}
	if urlTestHistoryFresh(nil, recheckInterval) {
		t.Error("nil history is fresh")
	}
}

// A urltest member whose result is one tick old is probed again rather than
// skipped until the tick after.
func TestURLTestReprobesAtNextTick(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name   string
		age    time.Duration
		probed bool
	}{
		{"next tick", recheckInterval - 2*time.Second, true},
		{"fresh", time.Second, false},
	} {
		history := urltest.NewHistoryStorage()
		history.StoreURLTestHistory("a", &adapter.URLTestHistory{Time: time.Now().Add(-c.age), Delay: 50})
		// The stub fails to dial, so a probe deletes the stored result.
		urlTestOutbounds(context.Background(), nil, history, log.NewNOPFactory().Logger(),
			[]adapter.Outbound{&seedStub{tag: "a"}}, "http://127.0.0.1:1/", recheckInterval, false, urlTestHooks{})
		probed := history.LoadURLTestHistory("a") == nil
		if probed != c.probed {
			t.Errorf("%s: probed = %v, want %v", c.name, probed, c.probed)
		}
	}
}

// A loadbalance leaf member is probed again at the next tick, and reusing a
// fresh result keeps the time it was measured.
func TestLoadBalanceLeafReprobesAtNextTick(t *testing.T) {
	t.Parallel()
	lb := &LoadBalance{history: urltest.NewHistoryStorage(), interval: recheckInterval}
	leaf := &seedStub{tag: "a"}

	measured := time.Now().Add(-(recheckInterval - 2*time.Second))
	lb.history.StoreURLTestHistory("a", &adapter.URLTestHistory{Time: measured, Delay: 50})
	if _, _, action := lb.memberProbe(leaf); action != probeHTTP {
		t.Fatalf("result one tick old: action %d, want probeHTTP", action)
	}

	measured = time.Now().Add(-time.Second)
	lb.history.StoreURLTestHistory("a", &adapter.URLTestHistory{Time: measured, Delay: 50})
	delay, at, action := lb.memberProbe(leaf)
	if action != probeUseCache || delay != 50 || !at.Equal(measured) {
		t.Fatalf("fresh result: delay %d at %v action %d, want 50 at %v probeUseCache", delay, at, action, measured)
	}

	urlTestOutbounds(context.Background(), nil, lb.history, log.NewNOPFactory().Logger(),
		[]adapter.Outbound{leaf}, "http://127.0.0.1:1/", recheckInterval, false, urlTestHooks{memberProbe: lb.memberProbe})
	stored := lb.history.LoadURLTestHistory("a")
	if stored == nil || !stored.Time.Equal(measured) {
		t.Fatalf("reused result stored at %v, want the measured time %v", stored, measured)
	}
}
