package group

import (
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/urltest"
)

// Sources of a member's latency, as reported in selection events.
const (
	latencySourceSession = "session"
	latencySourceURLTest = "url_test"
)

// realOutbound follows group members to the outbound they currently route
// through, like RealTag. Without an outbound manager, as in unit tests, the
// member itself is returned.
func realOutbound(manager adapter.OutboundManager, detour adapter.Outbound) adapter.Outbound {
	if manager == nil {
		return detour
	}
	for range 16 {
		group, isGroup := detour.(adapter.OutboundGroup)
		if !isGroup {
			return detour
		}
		next, loaded := manager.Outbound(group.Now())
		if !loaded {
			return detour
		}
		detour = next
	}
	return detour
}

// sessionDelay converts a live session latency to the millisecond delay the
// groups rank on. It rounds up and never returns zero, because a zero delay
// means "no result" throughout the group code and a LAN session can measure
// well under a millisecond.
func sessionDelay(latency time.Duration) uint16 {
	ms := (latency + time.Millisecond - 1) / time.Millisecond
	switch {
	case ms < 1:
		return 1
	case ms > 65535:
		return 65535
	default:
		return uint16(ms)
	}
}

// memberLatency returns the latency a member is ranked on. In session mode an
// outbound that measures its connections reports the latency of the one a new
// dial would use; any other member, or one with nothing measured yet, falls
// back to its URL test result. source names where the value came from.
func memberLatency(manager adapter.OutboundManager, history *urltest.HistoryStorage, detour adapter.Outbound, sessionMode bool) (delay uint16, source string, ok bool) {
	target := realOutbound(manager, detour)
	if sessionMode {
		if delay, loaded := liveSessionDelay(target); loaded {
			return delay, latencySourceSession, true
		}
	}
	stored := history.LoadURLTestHistory(target.Tag())
	if stored == nil || stored.Delay == 0 {
		return 0, "", false
	}
	return stored.Delay, latencySourceURLTest, true
}

// liveSessionDelay is the session latency of an outbound that measures its
// connections, or false when it does not or has nothing measured.
func liveSessionDelay(target adapter.Outbound) (uint16, bool) {
	measured, isMeasured := target.(adapter.OutboundWithSessionLatency)
	if !isMeasured {
		return 0, false
	}
	latency, loaded := measured.SessionLatency()
	if !loaded {
		return 0, false
	}
	return sessionDelay(latency), true
}
