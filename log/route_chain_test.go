package log

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStructuredChainRouterMatch(t *testing.T) {
	event := NewRouterMatchEvent(3, "domain=example.com", "route(selector-a)").
		WithOutbound("selector-a").
		WithResolvedChain("proxy-a", []string{"selector-a", "urltest-a", "lb-a", "proxy-a"}).
		WithMatched(true)

	data := event.ToMap()
	require.Equal(t, "selector-a", data["outbound"])
	require.Equal(t, "proxy-a", data["resolved_outbound"])
	require.Equal(t, []string{"selector-a", "urltest-a", "lb-a", "proxy-a"}, data["outbound_chain"])
}

func TestStructuredChainConnectionEventConfiguredGroupVsLeaf(t *testing.T) {
	event := NewConnectionEvent("outbound", "error").
		WithOutbound("selector-a", "selector").
		WithRoute("domain=example.com", "route(selector-a)").
		WithResolvedChain("proxy-a", "mock", []string{"selector-a", "urltest-a", "lb-a", "proxy-a"})

	data := event.ToMap()
	require.Equal(t, "selector-a", data["outbound"])
	require.Equal(t, "selector", data["outbound_type"])
	require.Equal(t, "domain=example.com", data["route_rule"])
	require.Equal(t, "route(selector-a)", data["route_action"])
	require.Equal(t, "proxy-a", data["resolved_outbound"])
	require.Equal(t, "mock", data["resolved_outbound_type"])
	require.Equal(t, []string{"selector-a", "urltest-a", "lb-a", "proxy-a"}, data["outbound_chain"])
}
