package group

import (
	"errors"
	"fmt"

	"github.com/sagernet/sing-box/adapter"
	commonurltest "github.com/sagernet/sing-box/common/urltest"
)

const maxOutboundResolveDepth = 32

var (
	errOutboundResolveCycle = errors.New("group outbound resolve cycle")
	errOutboundResolveDepth = errors.New("group outbound resolve depth exceeded")
)

type ResolvedOutbound struct {
	Chain []string
	Leaf  adapter.Outbound
}

type effectiveOutbound struct {
	configuredTag string
	outbound      adapter.Outbound
	historyKey    string
}

func ResolveOutboundByTag(outboundManager adapter.OutboundManager, tag string) (ResolvedOutbound, error) {
	if outboundManager == nil {
		return ResolvedOutbound{}, nil
	}
	detour, loaded := outboundManager.Outbound(tag)
	if !loaded {
		return ResolvedOutbound{}, fmt.Errorf("outbound not found: %s", tag)
	}
	return ResolveOutbound(detour)
}

func ResolveOutbound(detour adapter.Outbound) (ResolvedOutbound, error) {
	if detour == nil {
		return ResolvedOutbound{}, nil
	}

	chain := make([]string, 0, 4)
	visited := make(map[string]int)
	current := detour

	for {
		if len(chain) >= maxOutboundResolveDepth {
			return ResolvedOutbound{Chain: chain}, fmt.Errorf("%w: %s", errOutboundResolveDepth, current.Tag())
		}

		tag := current.Tag()
		if cycleIndex, seen := visited[tag]; seen {
			cycleChain := append(append([]string{}, chain[cycleIndex:]...), tag)
			return ResolvedOutbound{Chain: chain}, fmt.Errorf("%w: %v", errOutboundResolveCycle, cycleChain)
		}

		visited[tag] = len(chain)
		chain = append(chain, tag)

		group, isGroup := current.(adapter.OutboundGroup)
		if !isGroup {
			return ResolvedOutbound{Chain: chain, Leaf: current}, nil
		}

		nextTag := group.Now()
		if nextTag == "" {
			return ResolvedOutbound{Chain: chain}, nil
		}

		outboundManager := groupOutboundManager(current)
		if outboundManager == nil {
			return ResolvedOutbound{Chain: chain}, nil
		}

		nextOutbound, loaded := outboundManager.Outbound(nextTag)
		if !loaded {
			return ResolvedOutbound{Chain: chain}, nil
		}

		current = nextOutbound
	}
}

func resolveRealTag(detour adapter.Outbound) string {
	resolved, err := ResolveOutbound(detour)
	if err != nil || resolved.Leaf == nil {
		return ""
	}
	return resolved.Leaf.Tag()
}

func resolveEffectiveOutbound(detour adapter.Outbound) adapter.Outbound {
	resolved, err := ResolveOutbound(detour)
	if err != nil || resolved.Leaf == nil {
		return detour
	}
	return resolved.Leaf
}

func effectiveOutboundWithProbeConfig(detour adapter.Outbound, config commonurltest.ProbeConfig) effectiveOutbound {
	effective := resolveEffectiveOutbound(detour)
	return effectiveOutbound{
		configuredTag: detour.Tag(),
		outbound:      effective,
		historyKey:    resolveProbeHistoryKey(effective, config),
	}
}

func uniqueEffectiveOutbounds(detours []adapter.Outbound, config commonurltest.ProbeConfig) []effectiveOutbound {
	resolved := make([]effectiveOutbound, 0, len(detours))
	seen := make(map[string]struct{}, len(detours))
	for _, detour := range detours {
		effective := effectiveOutboundWithProbeConfig(detour, config)
		identity := effective.historyKey
		if identity == "" {
			identity = effective.outbound.Tag()
		}
		if _, exists := seen[identity]; exists {
			continue
		}
		seen[identity] = struct{}{}
		resolved = append(resolved, effective)
	}
	return resolved
}

func groupOutboundManager(detour adapter.Outbound) adapter.OutboundManager {
	switch outbound := detour.(type) {
	case *Selector:
		return outbound.outbound
	case *URLTest:
		return outbound.outbound
	case *LoadBalance:
		return outbound.outbound
	default:
		return nil
	}
}
