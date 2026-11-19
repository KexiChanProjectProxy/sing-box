package dns

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common/bufio"

	"github.com/stretchr/testify/require"
)

// The router detaches a connection from its groups, and a legacy packet route
// returns, only once onClose runs, so the DNS outbound must call it.
func TestOutboundPacketConnectionCallsOnClose(t *testing.T) {
	t.Parallel()
	packetConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	packetConn.Close()
	closed := make(chan error, 1)
	outbound := &Outbound{}
	go outbound.NewPacketConnection(context.Background(), bufio.NewPacketConn(packetConn), adapter.InboundContext{}, func(err error) {
		closed <- err
	})
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("onClose was not called")
	}
}
