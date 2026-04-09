package group

import (
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	adapterOutbound "github.com/sagernet/sing-box/adapter/outbound"
	commonurltest "github.com/sagernet/sing-box/common/urltest"
	C "github.com/sagernet/sing-box/constant"
	N "github.com/sagernet/sing/common/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProbeKeySharedLeafWithSameConfig(t *testing.T) {
	manager, selectorA, selectorB := newSharedLeafSelectors()
	history := commonurltest.NewHistoryStorage()
	now := time.Now()

	urlGroup := &URLTestGroup{
		outbound:  manager,
		link:      commonurltest.DefaultLink,
		interval:  time.Minute,
		tolerance: 50,
		history:   history,
	}
	otherURLTest := &URLTestGroup{
		outbound:  manager,
		link:      commonurltest.DefaultLink,
		interval:  time.Minute,
		tolerance: 50,
		history:   history,
	}

	urlKey := resolveProbeHistoryKey(selectorA, urlGroup.probeConfig())
	otherKey := resolveProbeHistoryKey(selectorB, otherURLTest.probeConfig())
	require.NotEmpty(t, urlKey)
	assert.Equal(t, urlKey, otherKey)

	stored := &adapter.URLTestHistory{Time: now, Delay: 42}
	history.StoreURLTestHistory(urlKey, stored)
	assert.Equal(t, stored, loadProbeHistory(history, selectorB, otherURLTest.probeConfig()))
}

func TestProbeIsolationDifferentConfig(t *testing.T) {
	manager, selectorA, selectorB := newSharedLeafSelectors()

	baseURLTest := &URLTestGroup{outbound: manager, link: commonurltest.DefaultLink, interval: time.Minute, tolerance: 50}
	differentInterval := &URLTestGroup{outbound: manager, link: commonurltest.DefaultLink, interval: 2 * time.Minute, tolerance: 50}
	differentTolerance := &URLTestGroup{outbound: manager, link: commonurltest.DefaultLink, interval: time.Minute, tolerance: 25}
	loadBalance := &LoadBalance{link: commonurltest.DefaultLink, timeout: C.TCPTimeout, interval: time.Minute, tolerance: 50, strategy: strategyRandom, emptyPoolAction: emptyPoolActionError, topNPrimary: 1}
	loadBalanceDifferentPolicy := &LoadBalance{link: commonurltest.DefaultLink, timeout: C.TCPTimeout, interval: time.Minute, tolerance: 50, strategy: strategyConsistentHash, emptyPoolAction: emptyPoolActionError, topNPrimary: 1, hashOnEmptyKey: onEmptyKeyHashEmpty, hashVirtualNodes: 100}

	keyA := resolveProbeHistoryKey(selectorA, baseURLTest.probeConfig())
	keyB := resolveProbeHistoryKey(selectorB, differentInterval.probeConfig())
	require.NotEmpty(t, keyA)
	require.NotEmpty(t, keyB)
	assert.NotEqual(t, keyA, keyB)

	keyC := resolveProbeHistoryKey(selectorB, differentTolerance.probeConfig())
	require.NotEmpty(t, keyC)
	assert.NotEqual(t, keyA, keyC)

	keyD := resolveProbeHistoryKey(selectorB, loadBalance.probeConfig())
	require.NotEmpty(t, keyD)
	assert.NotEqual(t, keyA, keyD)

	keyE := resolveProbeHistoryKey(selectorB, loadBalanceDifferentPolicy.probeConfig())
	require.NotEmpty(t, keyE)
	assert.NotEqual(t, keyD, keyE)
}

func TestProbeReuseLoadBalanceCollectTierStats(t *testing.T) {
	manager, selectorA, selectorB := newSharedLeafSelectors()
	history := commonurltest.NewHistoryStorage()
	now := time.Now()

	lbConfig := &LoadBalance{link: commonurltest.DefaultLink, timeout: C.TCPTimeout, interval: time.Minute, tolerance: 50, strategy: strategyRandom, emptyPoolAction: emptyPoolActionError, topNPrimary: 1}
	config := lbConfig.probeConfig()
	require.True(t, storeProbeHistory(history, selectorA, config, &adapter.URLTestHistory{Time: now, Delay: 15}))

	lb := &LoadBalance{
		logger:          &mockLogger{},
		outbound:        manager,
		history:         history,
		interval:        time.Minute,
		link:            commonurltest.DefaultLink,
		timeout:         C.TCPTimeout,
		tolerance:       50,
		strategy:        strategyRandom,
		emptyPoolAction: emptyPoolActionError,
		topNPrimary:     1,
		primaryTags:     []string{selectorB.Tag()},
	}

	stats := lb.collectTierStats(lb.primaryTags)
	require.Len(t, stats, 1)
	assert.False(t, stats[0].failure)
	assert.Equal(t, uint16(15), stats[0].delay)
}

func newSharedLeafSelectors() (*mockOutboundManager, *Selector, *Selector) {
	manager := &mockOutboundManager{outbounds: make(map[string]adapter.Outbound)}
	leaf := &mockOutbound{tag: "proxy-a", network: []string{N.NetworkTCP, N.NetworkUDP}}
	selectorA := &Selector{
		Adapter:   adapterOutbound.NewAdapter("selector", "selector-a", nil, []string{"proxy-a"}),
		outbound:  manager,
		tags:      []string{"proxy-a"},
		outbounds: map[string]adapter.Outbound{"proxy-a": leaf},
	}
	selectorA.selected.Store(leaf)
	selectorB := &Selector{
		Adapter:   adapterOutbound.NewAdapter("selector", "selector-b", nil, []string{"proxy-a"}),
		outbound:  manager,
		tags:      []string{"proxy-a"},
		outbounds: map[string]adapter.Outbound{"proxy-a": leaf},
	}
	selectorB.selected.Store(leaf)

	manager.outbounds[leaf.Tag()] = leaf
	manager.outbounds[selectorA.Tag()] = selectorA
	manager.outbounds[selectorB.Tag()] = selectorB
	return manager, selectorA, selectorB
}
