//go:build with_quic

package cloudflarewarp

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/sagernet/sing-box/transport/cloudflarewarp/warptest"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/stretchr/testify/require"
)

type systemDialer struct{}

func (systemDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	var dialer net.Dialer
	return dialer.DialContext(ctx, network, destination.String())
}

func (systemDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return net.ListenPacket("udp", "")
}

var _ N.Dialer = systemDialer{}

func newTestTransport(t *testing.T, server *warptest.Server, key *ecdsa.PrivateKey, pinned *ecdsa.PublicKey) Transport {
	t.Helper()
	return NewHTTP3Transport(HTTP3Options{
		Dialer:     systemDialer{},
		ServerAddr: M.SocksaddrFromNet(server.Addr()),
		TLSConfig: func() (*tls.Config, error) {
			return NewTLSConfig(TLSOptions{PrivateKey: key, EndpointPublicKey: pinned})
		},
		ConnectTimeout: 5 * time.Second,
	})
}

func newKey(t *testing.T) *ecdsa.PrivateKey {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return key
}

func packetBuffer(payload []byte) *buf.Buffer {
	buffer := buf.NewSize(PacketHeadroom + len(payload))
	buffer.Resize(PacketHeadroom, 0)
	buffer.Write(payload)
	return buffer
}

func TestHTTP3SessionExchangesPackets(t *testing.T) {
	clientKey := newKey(t)
	server, err := warptest.NewServer(warptest.Options{ClientPublicKey: &clientKey.PublicKey})
	require.NoError(t, err)
	defer server.Close()

	session, err := newTestTransport(t, server, clientKey, server.PublicKey).Connect(context.Background())
	require.NoError(t, err)
	defer session.Close()

	var serverSession *warptest.Session
	select {
	case serverSession = <-server.Sessions:
	case <-time.After(5 * time.Second):
		t.Fatal("no server session")
	}
	request := serverSession.Request
	require.Equal(t, "cloudflareaccess.com", request.Host)
	require.Equal(t, "?1", request.Header.Get("Capsule-Protocol"))

	packet := []byte{0x45, 0, 0, 20, 1, 2, 3, 4}
	buffer := packetBuffer(packet)
	require.NoError(t, session.WritePacket(buffer))
	require.Equal(t, packet, buffer.Bytes(), "WritePacket must restore the buffer")
	buffer.Release()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	received, err := serverSession.ReadPacket(ctx)
	require.NoError(t, err)
	require.Equal(t, packet, received)

	require.NoError(t, serverSession.WriteDatagram(7, []byte("ignored")))
	reply := []byte{0x60, 0, 0, 0, 9, 9}
	require.NoError(t, serverSession.WritePacket(reply))
	received, err = session.ReadPacket()
	require.NoError(t, err)
	require.Equal(t, reply, received)

	serverSession.Close()
	select {
	case <-session.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("session did not end after the server closed the stream")
	}
	require.Error(t, session.Err())
	_, err = session.ReadPacket()
	require.Error(t, err)
}

func TestHTTP3SessionRejectedStatus(t *testing.T) {
	clientKey := newKey(t)
	server, err := warptest.NewServer(warptest.Options{Status: http.StatusForbidden})
	require.NoError(t, err)
	defer server.Close()

	_, err = newTestTransport(t, server, clientKey, server.PublicKey).Connect(context.Background())
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrEnrollmentRejected), err.Error())
}

func TestHTTP3SessionPinMismatch(t *testing.T) {
	clientKey := newKey(t)
	server, err := warptest.NewServer(warptest.Options{})
	require.NoError(t, err)
	defer server.Close()

	_, err = newTestTransport(t, server, clientKey, &newKey(t).PublicKey).Connect(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "endpoint_public_key")
}

func TestHTTP3SessionUnknownClientKey(t *testing.T) {
	server, err := warptest.NewServer(warptest.Options{ClientPublicKey: &newKey(t).PublicKey})
	require.NoError(t, err)
	defer server.Close()

	_, err = newTestTransport(t, server, newKey(t), server.PublicKey).Connect(context.Background())
	require.Error(t, err)
}

func TestHTTP3SessionLocalClose(t *testing.T) {
	clientKey := newKey(t)
	server, err := warptest.NewServer(warptest.Options{})
	require.NoError(t, err)
	defer server.Close()

	session, err := newTestTransport(t, server, clientKey, server.PublicKey).Connect(context.Background())
	require.NoError(t, err)
	readErr := make(chan error, 1)
	go func() {
		_, err := session.ReadPacket()
		readErr <- err
	}()
	require.NoError(t, session.Close())
	select {
	case err := <-readErr:
		require.ErrorIs(t, err, ErrSessionClosed)
	case <-time.After(5 * time.Second):
		t.Fatal("ReadPacket did not return after Close")
	}
}
