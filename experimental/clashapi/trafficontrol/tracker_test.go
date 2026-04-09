package trafficontrol

import (
	"context"
	"net"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	group "github.com/sagernet/sing-box/protocol/group"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"

	"github.com/stretchr/testify/require"
)

func TestTrackerChainMatchesRuntimeResolverAndStaysStable(t *testing.T) {
	outboundManager, outer, inner := newNestedSelectorManager(t)
	resolved, err := group.ResolveOutbound(outer)
	require.NoError(t, err)
	require.NotNil(t, resolved.Leaf)

	left, right := net.Pipe()
	t.Cleanup(func() {
		_ = left.Close()
		_ = right.Close()
	})

	tracker := NewTCPTracker(left, NewManager(), adapter.InboundContext{Destination: M.ParseSocksaddrHostPort("example.com", 443)}, outboundManager, nil, outer)
	metadata := tracker.Metadata()

	require.Equal(t, resolved.Chain, metadata.Chain)
	require.Equal(t, resolved.Leaf.Tag(), metadata.Outbound)
	require.Equal(t, resolved.Leaf.Type(), metadata.OutboundType)

	require.True(t, inner.SelectOutbound("proxy-b"))
	updatedResolved, err := group.ResolveOutbound(outer)
	require.NoError(t, err)
	require.NotNil(t, updatedResolved.Leaf)
	require.Equal(t, []string{"selector-outer", "selector-inner", "proxy-b"}, updatedResolved.Chain)
	require.Equal(t, "proxy-b", updatedResolved.Leaf.Tag())

	require.Equal(t, []string{"selector-outer", "selector-inner", "proxy-a"}, metadata.Chain)
	require.Equal(t, "proxy-a", metadata.Outbound)
	require.Equal(t, "direct", metadata.OutboundType)
}

func TestResolveTrackerChainFallbackPreservesOuterToLeafOrder(t *testing.T) {
	leaf := &trackerTestOutbound{tag: "proxy-a", typ: "direct", network: []string{N.NetworkTCP, N.NetworkUDP}}
	inner := &trackerTestGroup{trackerTestOutbound: trackerTestOutbound{tag: "inner", typ: "mock-group", network: []string{N.NetworkTCP, N.NetworkUDP}}, current: leaf.Tag()}
	outer := &trackerTestGroup{trackerTestOutbound: trackerTestOutbound{tag: "outer", typ: "mock-group", network: []string{N.NetworkTCP, N.NetworkUDP}}, current: inner.Tag()}
	manager := &trackerTestOutboundManager{
		outbounds: map[string]adapter.Outbound{
			outer.Tag(): outer,
			inner.Tag(): inner,
			leaf.Tag():  leaf,
		},
		defaultOutbound: outer,
	}

	chain, outbound, outboundType := resolveTrackerChain(manager, outer)
	require.Equal(t, []string{"outer", "inner", "proxy-a"}, chain)
	require.Equal(t, "proxy-a", outbound)
	require.Equal(t, "direct", outboundType)
}

type trackerTestOutbound struct {
	tag     string
	typ     string
	network []string
}

func (o *trackerTestOutbound) Type() string           { return o.typ }
func (o *trackerTestOutbound) Tag() string            { return o.tag }
func (o *trackerTestOutbound) Network() []string      { return o.network }
func (o *trackerTestOutbound) Dependencies() []string { return nil }
func (o *trackerTestOutbound) DialContext(context.Context, string, M.Socksaddr) (net.Conn, error) {
	return nil, nil
}
func (o *trackerTestOutbound) ListenPacket(context.Context, M.Socksaddr) (net.PacketConn, error) {
	return nil, nil
}

type trackerTestGroup struct {
	trackerTestOutbound
	current string
}

func (g *trackerTestGroup) Now() string   { return g.current }
func (g *trackerTestGroup) All() []string { return []string{g.current} }

type trackerTestOutboundManager struct {
	outbounds       map[string]adapter.Outbound
	defaultOutbound adapter.Outbound
}

func (m *trackerTestOutboundManager) Start(adapter.StartStage) error { return nil }
func (m *trackerTestOutboundManager) Close() error                   { return nil }
func (m *trackerTestOutboundManager) Outbounds() []adapter.Outbound {
	result := make([]adapter.Outbound, 0, len(m.outbounds))
	for _, outbound := range m.outbounds {
		result = append(result, outbound)
	}
	return result
}
func (m *trackerTestOutboundManager) Outbound(tag string) (adapter.Outbound, bool) {
	outbound, loaded := m.outbounds[tag]
	return outbound, loaded
}
func (m *trackerTestOutboundManager) Default() adapter.Outbound { return m.defaultOutbound }
func (m *trackerTestOutboundManager) Remove(string) error       { return nil }
func (m *trackerTestOutboundManager) Create(context.Context, adapter.Router, log.ContextLogger, string, string, any) error {
	return nil
}

func newNestedSelectorManager(t *testing.T) (*trackerTestOutboundManager, *group.Selector, *group.Selector) {
	t.Helper()
	logger := log.NewNOPFactory().Logger()
	manager := &trackerTestOutboundManager{outbounds: make(map[string]adapter.Outbound)}
	ctx := service.ContextWith[adapter.OutboundManager](context.Background(), manager)

	leafA := &trackerTestOutbound{tag: "proxy-a", typ: "direct", network: []string{N.NetworkTCP, N.NetworkUDP}}
	leafB := &trackerTestOutbound{tag: "proxy-b", typ: "direct", network: []string{N.NetworkTCP, N.NetworkUDP}}
	manager.outbounds[leafA.Tag()] = leafA
	manager.outbounds[leafB.Tag()] = leafB

	rawInner, err := group.NewSelector(ctx, nil, logger, "selector-inner", option.SelectorOutboundOptions{Outbounds: []string{leafA.Tag(), leafB.Tag()}, Default: leafA.Tag()})
	require.NoError(t, err)
	inner := rawInner.(*group.Selector)
	manager.outbounds[inner.Tag()] = inner
	require.NoError(t, inner.Start())

	rawOuter, err := group.NewSelector(ctx, nil, logger, "selector-outer", option.SelectorOutboundOptions{Outbounds: []string{inner.Tag()}, Default: inner.Tag()})
	require.NoError(t, err)
	outer := rawOuter.(*group.Selector)
	manager.outbounds[outer.Tag()] = outer
	manager.defaultOutbound = outer
	require.NoError(t, outer.Start())

	return manager, outer, inner
}
