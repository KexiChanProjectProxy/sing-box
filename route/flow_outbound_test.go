package route

import (
	"context"
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/interrupt"
	N "github.com/sagernet/sing/common/network"

	"github.com/stretchr/testify/require"
)

type flowTestOutbound struct {
	mockOutbound
	networks []string
}

func (o *flowTestOutbound) Network() []string { return o.networks }

type flowTestGroup struct {
	flowTestOutbound
	selected       adapter.Outbound
	perConnection  bool
	interruptGroup *interrupt.Group
}

func newFlowTestGroup(tag string, selected adapter.Outbound, perConnection bool) *flowTestGroup {
	return &flowTestGroup{
		// Groups list TCP and UDP only, like loadbalance and urltest.
		flowTestOutbound: flowTestOutbound{mockOutbound{tag: tag}, []string{N.NetworkTCP, N.NetworkUDP}},
		selected:         selected,
		perConnection:    perConnection,
		interruptGroup:   interrupt.NewGroup(),
	}
}

func (g *flowTestGroup) All() []string                    { return []string{g.selected.Tag()} }
func (g *flowTestGroup) Selected(string) adapter.Outbound { return g.selected }
func (g *flowTestGroup) SelectsPerConnection() bool       { return g.perConnection }
func (g *flowTestGroup) AttachConnection(closer io.Closer) func() {
	return g.interruptGroup.Add(closer, true)
}

// Routing stops at a group that picks per connection, but a TUN flow never
// reaches the group's dial, so it is resolved to the group's current member.
// Groups do not list ICMP, so only the final member is checked for it.
func TestResolveFlowOutboundDescendsIntoSelectingGroups(t *testing.T) {
	t.Parallel()
	leaf := &flowTestOutbound{mockOutbound{tag: "direct"}, []string{N.NetworkTCP, N.NetworkUDP, N.NetworkICMP}}
	inner := newFlowTestGroup("inner", leaf, true)
	balance := newFlowTestGroup("balance", inner, true)
	outer := newFlowTestGroup("outer", balance, false)

	chain, err := resolveOutbound(outer, N.NetworkUDP)
	require.NoError(t, err)
	require.Equal(t, []adapter.Outbound{outer, balance}, chain)

	for _, network := range []string{N.NetworkUDP, N.NetworkICMP} {
		chain, err = resolveFlowOutbound(outer, network)
		require.NoError(t, err, network)
		require.Equal(t, []adapter.Outbound{outer, balance, inner, leaf}, chain, network)
	}

	proxy := &flowTestOutbound{mockOutbound{tag: "proxy"}, []string{N.NetworkTCP, N.NetworkUDP}}
	_, err = resolveFlowOutbound(newFlowTestGroup("group", proxy, true), N.NetworkICMP)
	require.Error(t, err, "a member without ICMP cannot carry a ping")
}

type closeRecorder struct {
	closed atomic.Bool
}

func (c *closeRecorder) Close() error {
	c.closed.Store(true)
	return nil
}

// A group switch while the route is still applying destination overrides
// closes the connection, cancels the routing context and stops the route.
func TestRouteInterrupterStopsRouteOnGroupSwitch(t *testing.T) {
	t.Parallel()
	leaf := &flowTestOutbound{mockOutbound{tag: "a"}, []string{N.NetworkTCP}}
	group := newFlowTestGroup("selector", leaf, false)
	conn := &closeRecorder{}
	ctx, interrupter := attachInterrupt(context.Background(), []adapter.Outbound{group, leaf}, conn)
	require.NoError(t, interrupter.Err())

	group.interruptGroup.Interrupt(true)
	require.True(t, conn.closed.Load())
	require.ErrorIs(t, ctx.Err(), context.Canceled)
	require.True(t, errors.Is(interrupter.Err(), net.ErrClosed))

	interrupter.Detach()
	interrupter.Detach()
}

// Detaching removes the connection, so a later switch leaves it alone.
func TestRouteInterrupterDetach(t *testing.T) {
	t.Parallel()
	leaf := &flowTestOutbound{mockOutbound{tag: "a"}, []string{N.NetworkTCP}}
	group := newFlowTestGroup("selector", leaf, false)
	conn := &closeRecorder{}
	_, interrupter := attachInterrupt(context.Background(), []adapter.Outbound{group, leaf}, conn)
	interrupter.Detach()
	group.interruptGroup.Interrupt(true)
	require.False(t, conn.closed.Load())
	require.NoError(t, interrupter.Err())
}
