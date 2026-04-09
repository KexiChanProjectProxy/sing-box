package daemon

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/sagernet/sing-box/experimental/clashapi/trafficontrol"
	"github.com/stretchr/testify/require"
)

func TestBuildConnectionProtoPreservesResolvedChainOrder(t *testing.T) {
	upload := new(atomic.Int64)
	download := new(atomic.Int64)
	upload.Store(12)
	download.Store(34)

	connection := buildConnectionProto(&trafficontrol.TrackerMetadata{
		Upload:       upload,
		Download:     download,
		CreatedAt:    time.UnixMilli(100),
		Outbound:     "proxy-a",
		OutboundType: "direct",
		Chain:        []string{"selector-a", "selector-b", "proxy-a"},
	})

	require.Equal(t, []string{"selector-a", "selector-b", "proxy-a"}, connection.ChainList)
	require.Equal(t, "proxy-a", connection.Outbound)
	require.Equal(t, "direct", connection.OutboundType)
	require.EqualValues(t, 12, connection.UplinkTotal)
	require.EqualValues(t, 34, connection.DownlinkTotal)
}
