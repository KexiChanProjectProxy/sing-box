package group

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	adapterOutbound "github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/interrupt"
	commonurltest "github.com/sagernet/sing-box/common/urltest"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestURLTestGroupProbesSharedLeafOnce(t *testing.T) {
	history := commonurltest.NewHistoryStorage()
	manager := &mockOutboundManager{outbounds: make(map[string]adapter.Outbound)}
	leaf := &mockOutbound{tag: "proxy-a", network: []string{N.NetworkTCP, N.NetworkUDP}, dialErr: errors.New("probe failed")}
	selectorA, selectorB := newSharedLeafSelectorsWithLeaf(manager, leaf)

	group := &URLTestGroup{
		ctx:            context.Background(),
		outbound:       manager,
		logger:         &mockLogger{},
		outbounds:      []adapter.Outbound{selectorA, selectorB},
		link:           commonurltest.DefaultLink,
		interval:       time.Minute,
		tolerance:      50,
		history:        history,
		interruptGroup: interrupt.NewGroup(),
		close:          make(chan struct{}),
	}

	result, err := group.URLTest(context.Background())
	require.NoError(t, err)
	assert.Empty(t, result)
	assert.Equal(t, 1, leaf.dialCount)
}

func TestURLTestDialContextUsesResolvedLeaf(t *testing.T) {
	history := commonurltest.NewHistoryStorage()
	now := time.Now()
	manager := &mockOutboundManager{outbounds: make(map[string]adapter.Outbound)}
	leaf := &mockOutbound{tag: "proxy-a", network: []string{N.NetworkTCP, N.NetworkUDP}}
	selectorA, selectorB := newSharedLeafSelectorsWithLeaf(manager, leaf)
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()
	leaf.dialConn = clientConn

	group := &URLTestGroup{
		ctx:            context.Background(),
		outbound:       manager,
		logger:         &mockLogger{},
		outbounds:      []adapter.Outbound{selectorA, selectorB},
		link:           commonurltest.DefaultLink,
		interval:       time.Minute,
		tolerance:      50,
		history:        history,
		interruptGroup: interrupt.NewGroup(),
		close:          make(chan struct{}),
	}
	storeProbeHistory(history, selectorA, group.probeConfig(), &adapter.URLTestHistory{Time: now, Delay: 10})

	urlOutbound := &URLTest{
		logger: &mockLogger{},
		group:  group,
	}

	conn, err := urlOutbound.DialContext(context.Background(), N.NetworkTCP, M.ParseSocksaddr("1.1.1.1:80"))
	require.NoError(t, err)
	require.NotNil(t, conn)
	_ = conn.Close()

	assert.Equal(t, 1, leaf.dialCount)
}

func TestURLTestSelectReusesResolvedLeafHistoryAcrossNestedParents(t *testing.T) {
	history := commonurltest.NewHistoryStorage()
	now := time.Now()
	manager := &mockOutboundManager{outbounds: make(map[string]adapter.Outbound)}
	leaf := &mockOutbound{tag: "proxy-a", network: []string{N.NetworkTCP, N.NetworkUDP}}
	selectorA, selectorB := newSharedLeafSelectorsWithLeaf(manager, leaf)

	sharedConfig := &URLTestGroup{
		outbound:  manager,
		outbounds: []adapter.Outbound{selectorB},
		link:      commonurltest.DefaultLink,
		interval:  time.Minute,
		tolerance: 50,
		history:   history,
	}
	storeProbeHistory(history, selectorA, sharedConfig.probeConfig(), &adapter.URLTestHistory{Time: now, Delay: 12})

	selected, exists := sharedConfig.Select(N.NetworkTCP)
	require.True(t, exists)
	require.NotNil(t, selected)
	assert.Same(t, leaf, selected)
}

func TestURLTestSelectDoesNotReuseResolvedLeafHistoryAcrossDifferentConfig(t *testing.T) {
	history := commonurltest.NewHistoryStorage()
	now := time.Now()
	manager := &mockOutboundManager{outbounds: make(map[string]adapter.Outbound)}
	leaf := &mockOutbound{tag: "proxy-a", network: []string{N.NetworkTCP, N.NetworkUDP}}
	selectorA, selectorB := newSharedLeafSelectorsWithLeaf(manager, leaf)

	storedConfig := &URLTestGroup{
		outbound:  manager,
		outbounds: []adapter.Outbound{selectorA},
		link:      commonurltest.DefaultLink,
		interval:  time.Minute,
		tolerance: 50,
		history:   history,
	}
	isolatedConfig := &URLTestGroup{
		outbound:  manager,
		outbounds: []adapter.Outbound{selectorB},
		link:      commonurltest.DefaultLink,
		interval:  2 * time.Minute,
		tolerance: 50,
		history:   history,
	}
	storeProbeHistory(history, selectorA, storedConfig.probeConfig(), &adapter.URLTestHistory{Time: now, Delay: 12})

	selected, exists := isolatedConfig.Select(N.NetworkTCP)
	require.False(t, exists)
	require.NotNil(t, selected)
	assert.Same(t, leaf, selected)
}

func TestURLTestSelectFallsBackToConfiguredNestedOutboundWhenLeafUnavailable(t *testing.T) {
	history := commonurltest.NewHistoryStorage()
	manager := &mockOutboundManager{outbounds: make(map[string]adapter.Outbound)}
	inner := &URLTest{
		Adapter:  adapterOutbound.NewAdapter("urltest", "inner-urltest", []string{N.NetworkTCP, N.NetworkUDP}, []string{"proxy-a"}),
		outbound: manager,
		logger:   &mockLogger{},
		group:    &URLTestGroup{},
	}
	manager.outbounds[inner.Tag()] = inner

	outer := &URLTestGroup{
		outbound:  manager,
		outbounds: []adapter.Outbound{inner},
		link:      commonurltest.DefaultLink,
		interval:  time.Minute,
		tolerance: 50,
		history:   history,
	}

	selected, exists := outer.Select(N.NetworkTCP)
	require.False(t, exists)
	assert.Same(t, inner, selected)
}

func newSharedLeafSelectorsWithLeaf(manager *mockOutboundManager, leaf *mockOutbound) (*Selector, *Selector) {
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
	return selectorA, selectorB
}
