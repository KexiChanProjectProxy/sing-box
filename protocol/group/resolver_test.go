package group

import (
	"errors"
	"fmt"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	adapterOutbound "github.com/sagernet/sing-box/adapter/outbound"
	N "github.com/sagernet/sing/common/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveNestedLeafAndChain(t *testing.T) {
	manager := &mockOutboundManager{outbounds: make(map[string]adapter.Outbound)}
	leaf := &mockOutbound{tag: "proxy-a", network: []string{N.NetworkTCP, N.NetworkUDP}}
	lb := &LoadBalance{
		Adapter:  adapterOutbound.NewAdapter("loadbalance", "lb-a", []string{N.NetworkTCP, N.NetworkUDP}, []string{"proxy-a"}),
		outbound: manager,
	}
	lb.candidateState.Store(&candidateSnapshot{
		primaryCandidates: []adapter.Outbound{leaf},
		activeTier:        "primary",
	})
	urlTest := &URLTest{
		Adapter:  adapterOutbound.NewAdapter("urltest", "urltest-a", []string{N.NetworkTCP, N.NetworkUDP}, []string{"lb-a"}),
		outbound: manager,
		group: &URLTestGroup{
			selectedOutboundTCP: lb,
			selectedOutboundUDP: lb,
		},
	}
	selector := &Selector{
		Adapter:   adapterOutbound.NewAdapter("selector", "selector-a", nil, []string{"urltest-a"}),
		outbound:  manager,
		tags:      []string{"urltest-a"},
		outbounds: map[string]adapter.Outbound{"urltest-a": urlTest},
	}
	selector.selected.Store(urlTest)

	manager.outbounds["selector-a"] = selector
	manager.outbounds["urltest-a"] = urlTest
	manager.outbounds["lb-a"] = lb
	manager.outbounds["proxy-a"] = leaf

	resolved, err := ResolveOutboundByTag(manager, "selector-a")
	require.NoError(t, err)
	require.NotNil(t, resolved.Leaf)
	assert.Equal(t, "proxy-a", resolved.Leaf.Tag())
	assert.Equal(t, []string{"selector-a", "urltest-a", "lb-a", "proxy-a"}, resolved.Chain)
	assert.Equal(t, "proxy-a", RealTag(selector))
}

func TestResolveCycle(t *testing.T) {
	manager := &mockOutboundManager{outbounds: make(map[string]adapter.Outbound)}
	selector := &Selector{
		Adapter:   adapterOutbound.NewAdapter("selector", "selector-a", nil, []string{"urltest-a"}),
		outbound:  manager,
		tags:      []string{"urltest-a"},
		outbounds: make(map[string]adapter.Outbound),
	}
	urlTest := &URLTest{
		Adapter:  adapterOutbound.NewAdapter("urltest", "urltest-a", []string{N.NetworkTCP, N.NetworkUDP}, []string{"selector-a"}),
		outbound: manager,
		group: &URLTestGroup{
			selectedOutboundTCP: selector,
		},
	}
	selector.outbounds["urltest-a"] = urlTest
	selector.selected.Store(urlTest)

	manager.outbounds["selector-a"] = selector
	manager.outbounds["urltest-a"] = urlTest

	resolved, err := ResolveOutboundByTag(manager, "selector-a")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errOutboundResolveCycle))
	assert.Equal(t, []string{"selector-a", "urltest-a"}, resolved.Chain)
	assert.Equal(t, "", RealTag(selector))
}

func TestResolveDepth(t *testing.T) {
	manager := &mockOutboundManager{outbounds: make(map[string]adapter.Outbound)}

	for i := 0; i < maxOutboundResolveDepth+1; i++ {
		tag := fmt.Sprintf("selector-%d", i)
		nextTag := fmt.Sprintf("selector-%d", i+1)
		selector := &Selector{
			Adapter:   adapterOutbound.NewAdapter("selector", tag, nil, []string{nextTag}),
			outbound:  manager,
			tags:      []string{nextTag},
			outbounds: make(map[string]adapter.Outbound),
		}
		manager.outbounds[tag] = selector
	}

	leaf := &mockOutbound{tag: "proxy-a", network: []string{N.NetworkTCP}}
	manager.outbounds[fmt.Sprintf("selector-%d", maxOutboundResolveDepth+1)] = leaf

	for i := 0; i < maxOutboundResolveDepth+1; i++ {
		tag := fmt.Sprintf("selector-%d", i)
		nextTag := fmt.Sprintf("selector-%d", i+1)
		selector := manager.outbounds[tag].(*Selector)
		next := manager.outbounds[nextTag]
		selector.outbounds[nextTag] = next
		selector.selected.Store(next)
	}

	resolved, err := ResolveOutboundByTag(manager, "selector-0")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errOutboundResolveDepth))
	assert.Len(t, resolved.Chain, maxOutboundResolveDepth)
	assert.Equal(t, "selector-0", resolved.Chain[0])
	assert.Equal(t, fmt.Sprintf("selector-%d", maxOutboundResolveDepth-1), resolved.Chain[len(resolved.Chain)-1])
	assert.Equal(t, "", RealTag(manager.outbounds["selector-0"]))
}
