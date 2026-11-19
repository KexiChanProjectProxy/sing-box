package route

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common/bufio"

	"github.com/stretchr/testify/require"
)

// A legacy packet route that fails must return and close the connection
// instead of waiting for a close callback no one will call.
func TestRoutePacketConnectionReturnsOnRouteError(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	router := &Router{ctx: ctx, logger: log.NewNOPFactory().Logger()}
	packetConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	defer packetConn.Close()

	returned := make(chan error, 1)
	go func() {
		//nolint:staticcheck
		returned <- router.RoutePacketConnection(ctx, bufio.NewPacketConn(packetConn), adapter.InboundContext{
			Inbound:       "in",
			LastInbound:   "loop",
			InboundDetour: "loop",
		})
	}()
	select {
	case err = <-returned:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("RoutePacketConnection did not return after a routing error")
	}
	_, err = packetConn.WriteTo([]byte{0}, packetConn.LocalAddr())
	require.ErrorIs(t, err, net.ErrClosed, "the connection must be closed")
}
