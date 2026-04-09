package route

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/stretchr/testify/require"
)

type routeTestSubscriber struct {
	entries chan log.LogEntry
}

func (s *routeTestSubscriber) HandleEvent(entry log.LogEntry) {
	s.entries <- entry
}

type routeTestOutbound struct {
	tag     string
	typ     string
	network []string
}

func (o *routeTestOutbound) Type() string      { return o.typ }
func (o *routeTestOutbound) Tag() string       { return o.tag }
func (o *routeTestOutbound) Network() []string { return o.network }
func (o *routeTestOutbound) Dependencies() []string {
	return nil
}
func (o *routeTestOutbound) DialContext(context.Context, string, M.Socksaddr) (net.Conn, error) {
	return nil, nil
}
func (o *routeTestOutbound) ListenPacket(context.Context, M.Socksaddr) (net.PacketConn, error) {
	return nil, nil
}

func TestRouterMatchChainFormatsConfiguredGroupAndLeaf(t *testing.T) {
	routeContext := routeLogContext{
		rule:          "domain=example.com",
		action:        "route(selector-a)",
		configuredTag: "selector-a",
		resolvedTag:   "proxy-a",
		resolvedType:  "mock",
		outboundChain: []string{"selector-a", "urltest-a", "lb-a", "proxy-a"},
	}

	event := routeContext.applyToRouterMatchEvent(
		log.NewRouterMatchEvent(1, routeContext.rule, routeContext.action).WithOutbound(routeContext.configuredTag).WithMatched(true),
	)

	require.Equal(t, " -> selector-a -> urltest-a -> lb-a -> proxy-a", routeContext.resolvedChainSuffix())
	require.Equal(t, "domain=example.com => route(selector-a) -> selector-a -> urltest-a -> lb-a -> proxy-a", routeContext.plainRouteLabel())
	require.Equal(t, "selector-a", event.Outbound)
	require.Equal(t, "proxy-a", event.ResolvedOutbound)
	require.Equal(t, []string{"selector-a", "urltest-a", "lb-a", "proxy-a"}, event.OutboundChain)
}

func TestRouteChainConnectionErrorIncludesRuleAndResolvedLeaf(t *testing.T) {
	routeContext := routeLogContext{
		rule:          "domain=example.com",
		action:        "route(selector-a)",
		configuredTag: "selector-a",
		resolvedTag:   "proxy-a",
		resolvedType:  "mock",
		outboundChain: []string{"selector-a", "urltest-a", "lb-a", "proxy-a"},
	}
	selectedOutbound := &routeTestOutbound{tag: "selector-a", typ: "selector", network: []string{N.NetworkTCP}}

	bus := log.NewEventBus()
	t.Cleanup(bus.Close)
	factory := log.NewMultiOutputFactoryWithBus(context.Background(), nil, log.Formatter{}, nil, false, bus)
	factory.SetLevel(log.LevelTrace)

	subscriber := &routeTestSubscriber{entries: make(chan log.LogEntry, 1)}
	subscriptionID := bus.Subscribe(subscriber, log.EventFilter{EventTypes: []log.EventType{log.EventTypeConnection}, MinLevel: log.LevelError}, 1)
	t.Cleanup(func() { bus.Unsubscribe(subscriptionID) })

	ctx := withRouteLogContext(context.Background(), routeContext)
	metadata := adapter.InboundContext{Network: N.NetworkTCP, Destination: M.ParseSocksaddrHostPort("example.com", 443)}
	err := errors.New("open connection to example.com:443 using outbound/selector[selector-a]")
	logOutboundConnectionError(factory.Logger(), ctx, metadata, selectedOutbound, err)

	select {
	case entry := <-subscriber.entries:
		require.Contains(t, entry.Message, "domain=example.com => route(selector-a) -> selector-a -> urltest-a -> lb-a -> proxy-a")
		require.Equal(t, "selector-a", entry.Event.Data["outbound"])
		require.Equal(t, "proxy-a", entry.Event.Data["resolved_outbound"])
		require.Equal(t, []string{"selector-a", "urltest-a", "lb-a", "proxy-a"}, entry.Event.Data["outbound_chain"])
		require.Equal(t, "domain=example.com", entry.Event.Data["route_rule"])
		require.Equal(t, "route(selector-a)", entry.Event.Data["route_action"])
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for connection event")
	}
}
