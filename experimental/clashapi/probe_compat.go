package clashapi

import (
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/protocol/group"
)

func resolveManualDelayTarget(outboundManager adapter.OutboundManager, detour adapter.Outbound) (adapter.Outbound, string) {
	if detour == nil {
		return nil, ""
	}
	resolved, err := group.ResolveOutbound(detour)
	if err == nil && resolved.Leaf != nil {
		return resolved.Leaf, resolved.Leaf.Tag()
	}
	bestEffortTag := adapter.OutboundTag(detour)
	if bestEffortTag != "" {
		if outboundManager != nil {
			if fallback, loaded := outboundManager.Outbound(bestEffortTag); loaded {
				return fallback, bestEffortTag
			}
		}
		if detour.Tag() == bestEffortTag {
			return detour, bestEffortTag
		}
	}
	if detour.Tag() != "" {
		return detour, detour.Tag()
	}
	return detour, ""
}
