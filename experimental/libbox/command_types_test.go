package libbox

import (
	"testing"

	"github.com/sagernet/sing-box/daemon"
	"github.com/stretchr/testify/require"
)

func TestConnectionFromGRPCPreservesResolvedChainOrder(t *testing.T) {
	connection := connectionFromGRPC(&daemon.Connection{
		Id:           "conn-1",
		Outbound:     "proxy-a",
		OutboundType: "direct",
		ChainList:    []string{"selector-a", "selector-b", "proxy-a"},
	})

	require.Equal(t, []string{"selector-a", "selector-b", "proxy-a"}, connection.ChainList)
	require.Equal(t, "proxy-a", connection.Outbound)
	require.Equal(t, "direct", connection.OutboundType)
}
