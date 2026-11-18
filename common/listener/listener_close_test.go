package listener

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/json/badoption"
	N "github.com/sagernet/sing/common/network"

	"github.com/stretchr/testify/require"
)

type blockingHandler struct {
	accepted chan net.Conn
	returned chan error
}

func (h *blockingHandler) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	h.accepted <- conn
	_, err := conn.Read(make([]byte, 1))
	h.returned <- err
}

func TestListenerCloseClosesServedConnections(t *testing.T) {
	handler := &blockingHandler{
		accepted: make(chan net.Conn, 1),
		returned: make(chan error, 1),
	}
	l := New(Options{
		Context: context.Background(),
		Logger:  log.NewNOPFactory().NewLogger("listener"),
		Network: []string{N.NetworkTCP},
		Listen: option.ListenOptions{
			Listen: common.Ptr(badoption.Addr(netip.AddrFrom4([4]byte{127, 0, 0, 1}))),
		},
		ConnectionHandler: handler,
	})
	require.NoError(t, l.Start())
	t.Cleanup(func() { l.Close() })

	client, err := net.Dial("tcp", l.TCPListener().Addr().String())
	require.NoError(t, err)
	defer client.Close()
	select {
	case <-handler.accepted:
	case <-time.After(5 * time.Second):
		t.Fatal("connection not accepted")
	}

	require.NoError(t, l.Close())
	select {
	case err = <-handler.returned:
		require.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("handler still running after Close")
	}
	// The peer sees the connection end too.
	require.NoError(t, client.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, err = client.Read(make([]byte, 1))
	require.Error(t, err)
	require.NotContains(t, err.Error(), "timeout")

	l.connAccess.Lock()
	require.Empty(t, l.conns)
	l.connAccess.Unlock()
}
