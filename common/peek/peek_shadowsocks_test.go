package peek

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"io"
	"net"
	"testing"
	"time"

	"github.com/sagernet/sing-shadowsocks/shadowaead_2022"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/stretchr/testify/require"
)

type shadowsocksHandler struct {
	done chan []byte
}

func (h *shadowsocksHandler) NewConnection(ctx context.Context, conn net.Conn, metadata M.Metadata) error {
	payload := make([]byte, 5)
	_, err := io.ReadFull(conn, payload)
	if err != nil {
		return err
	}
	h.done <- payload
	return nil
}

func (h *shadowsocksHandler) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata M.Metadata) error {
	return nil
}

func (h *shadowsocksHandler) NewError(ctx context.Context, err error) {}

// TestClientHelloShadowsocks2022 hands a peeked connection to a Shadowsocks
// 2022 server, which reads its whole request header in one read, on both the
// MSG_PEEK path and the read-and-replay fallback.
func TestClientHelloShadowsocks2022(t *testing.T) {
	t.Parallel()
	psk := make([]byte, 16)
	_, err := rand.Read(psk)
	require.NoError(t, err)
	password := base64.StdEncoding.EncodeToString(psk)
	for _, wrap := range []bool{false, true} {
		name := "socket"
		if wrap {
			name = "fallback"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			handler := &shadowsocksHandler{done: make(chan []byte, 1)}
			service, err := shadowaead_2022.NewServiceWithPassword("2022-blake3-aes-128-gcm", password, 60, handler, nil)
			require.NoError(t, err)
			method, err := shadowaead_2022.NewWithPassword("2022-blake3-aes-128-gcm", password, nil)
			require.NoError(t, err)

			client, server := tcpPair(t)
			clientConn := method.DialEarlyConn(client, M.ParseSocksaddrHostPort("example.org", 443))
			// One write: the request header and the payload.
			_, err = clientConn.Write([]byte("hello"))
			require.NoError(t, err)
			time.Sleep(50 * time.Millisecond)

			var conn net.Conn = server
			if wrap {
				conn = wrappedConn{server}
			}
			result := <-peekAsync(conn, 5*time.Second)
			require.NoError(t, result.err)
			require.False(t, result.isTLS)
			require.NoError(t, service.NewConnection(context.Background(), result.conn, M.Metadata{}))
			select {
			case payload := <-handler.done:
				require.Equal(t, []byte("hello"), payload)
			case <-time.After(5 * time.Second):
				t.Fatal("no payload")
			}
		})
	}
}
