package anytls_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"

	anytls "github.com/anytls/sing-anytls"
	"github.com/stretchr/testify/require"
)

func freePort(t *testing.T) uint16 {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	return uint16(listener.Addr().(*net.TCPAddr).Port)
}

func startEchoServer(t *testing.T) M.Socksaddr {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_, _ = io.Copy(conn, conn)
			}()
		}
	}()
	return M.SocksaddrFromNet(listener.Addr())
}

// startBox returns the box and how to stop it, in the order a reload does:
// cancel the context, then close.
func startBox(t *testing.T, port uint16) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(include.Context(context.Background()))
	options, err := option.UnmarshalContext(ctx, []byte(fmt.Sprintf(`{
		"log": {"disabled": true},
		"inbounds": [{
			"type": "anytls",
			"listen": "127.0.0.1",
			"listen_port": %d,
			"users": [{"name": "u1", "password": "p1"}]
		}]
	}`, port)))
	require.NoError(t, err)
	instance, err := box.New(box.Options{Context: ctx, Options: options})
	require.NoError(t, err)
	require.NoError(t, instance.Start())
	var stopped atomic.Bool
	stop := func() {
		if stopped.CompareAndSwap(false, true) {
			cancel()
			require.NoError(t, instance.Close())
		}
	}
	t.Cleanup(stop)
	return stop
}

// newClient returns a client and the number of sessions it has dialed.
func newClient(t *testing.T, port uint16, password string) (*anytls.Client, *atomic.Int32) {
	t.Helper()
	var dials atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	client, err := anytls.NewClient(ctx, anytls.ClientConfig{
		Password: password,
		DialOut: func(ctx context.Context) (net.Conn, error) {
			dials.Add(1)
			var dialer net.Dialer
			return dialer.DialContext(ctx, "tcp", fmt.Sprintf("127.0.0.1:%d", port))
		},
		Logger: logger.NOP(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })
	return client, &dials
}

func requireEcho(t *testing.T, conn net.Conn) {
	t.Helper()
	require.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)))
	_, err := conn.Write([]byte("ping"))
	require.NoError(t, err)
	reply := make([]byte, 4)
	_, err = io.ReadFull(conn, reply)
	require.NoError(t, err)
	require.Equal(t, "ping", string(reply))
}

// poolSession leaves one idle session in the client's pool.
func poolSession(t *testing.T, client *anytls.Client, destination M.Socksaddr) {
	t.Helper()
	conn, err := client.CreateProxy(context.Background(), destination)
	require.NoError(t, err)
	requireEcho(t, conn)
	require.NoError(t, conn.Close())
}

// A session the client pooled before the box was replaced must not be handed
// out afterwards: it would run every stream on the closed box.
func TestRecreateDropsPooledSession(t *testing.T) {
	destination := startEchoServer(t)
	port := freePort(t)
	stopOldBox := startBox(t, port)
	client, dials := newClient(t, port, "p1")
	poolSession(t, client, destination)
	require.EqualValues(t, 1, dials.Load())

	stopOldBox()
	startBox(t, port)
	// What a client sees of the close takes a moment to arrive.
	time.Sleep(200 * time.Millisecond)

	conn, err := client.CreateProxy(context.Background(), destination)
	require.NoError(t, err)
	defer conn.Close()
	requireEcho(t, conn)
	require.EqualValues(t, 2, dials.Load(), "stream reused a session of the closed box")
}
