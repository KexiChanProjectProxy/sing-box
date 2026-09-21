package hysteria2

import (
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/transportstats"
	"github.com/sagernet/sing-quic/hysteria2"

	"github.com/stretchr/testify/require"
)

// The outbounds convert between the two direction types by value, so they
// must stay numbered alike.
func TestTransportStatsDirectionsAligned(t *testing.T) {
	t.Parallel()
	require.EqualValues(t, adapter.TransportStatsClient, transportstats.DirectionClient)
	require.EqualValues(t, adapter.TransportStatsServer, transportstats.DirectionServer)
}

func TestQUICSample(t *testing.T) {
	t.Parallel()
	sample := quicSample(hysteria2.TransportStats{
		ConnectionID: 3,
		SmoothedRTT:  40 * time.Millisecond,
		RTTVariance:  5 * time.Millisecond,
		PacketsSent:  900,
		PacketsLost:  9,
		BytesSent:    1 << 20,
	})
	require.EqualValues(t, 3, sample.Key)
	require.EqualValues(t, 900, sample.Sent)
	require.EqualValues(t, 9, sample.Lost)
	require.EqualValues(t, 1<<20, sample.BytesSent)
	require.Equal(t, 40*time.Millisecond, sample.RTT)
	require.Equal(t, 5*time.Millisecond, sample.RTTVar)
	require.Zero(t, sample.DeliveryRate, "throughput is derived from bytes sent")
}
