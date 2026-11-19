package route

import (
	"io"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	N "github.com/sagernet/sing/common/network"

	"github.com/stretchr/testify/require"
)

type flowTestOutbound struct {
	mockOutbound
}

func (o *flowTestOutbound) Network() []string { return []string{N.NetworkTCP, N.NetworkUDP} }

type flowTestGroup struct {
	flowTestOutbound
	selected      adapter.Outbound
	perConnection bool
}

func (g *flowTestGroup) All() []string                     { return []string{g.selected.Tag()} }
func (g *flowTestGroup) Selected(string) adapter.Outbound  { return g.selected }
func (g *flowTestGroup) AttachConnection(io.Closer) func() { return func() {} }
func (g *flowTestGroup) SelectsPerConnection() bool        { return g.perConnection }

// Routing stops at a group that picks per connection, but a TUN flow never
// reaches the group's dial, so it is resolved to the group's current member.
func TestResolveFlowOutboundDescendsIntoSelectingGroup(t *testing.T) {
	t.Parallel()
	leaf := &flowTestOutbound{mockOutbound{tag: "leaf"}}
	balance := &flowTestGroup{flowTestOutbound: flowTestOutbound{mockOutbound{tag: "balance"}}, selected: leaf, perConnection: true}
	outer := &flowTestGroup{flowTestOutbound: flowTestOutbound{mockOutbound{tag: "outer"}}, selected: balance}

	chain, err := resolveOutbound(outer, N.NetworkUDP)
	require.NoError(t, err)
	require.Equal(t, []adapter.Outbound{outer, balance}, chain)

	chain, err = resolveFlowOutbound(outer, N.NetworkUDP)
	require.NoError(t, err)
	require.Equal(t, []adapter.Outbound{outer, balance, leaf}, chain)
}
