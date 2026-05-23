package noisyshuttle

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTCPEchoServerSmoke(t *testing.T) {
	const testPort uint16 = 25100
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server, err := NewTCPEchoServer(testPort)
	require.NoError(t, err)
	defer server.Stop()

	require.Equal(t, "tcp", server.Addr().Network())

	conn, err := DialTCP(ctx, server.Addr().String())
	require.NoError(t, err)
	defer conn.Close()

	err = PingPong(t, conn, 5*time.Second)
	require.NoError(t, err)
}

func TestUDPEchoServerSmoke(t *testing.T) {
	const testPort uint16 = 25101
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server, err := NewUDPEchoServer(testPort)
	require.NoError(t, err)
	defer server.Stop()

	pc, err := DialUDP(ctx, server.Addr().String())
	require.NoError(t, err)
	defer pc.Close()

	err = PingPongUDP(t, pc, server.Addr(), 5*time.Second)
	require.NoError(t, err)
}

func TestTCPEchoServerMultipleConnections(t *testing.T) {
	const testPort uint16 = 25102
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	server, err := NewTCPEchoServer(testPort)
	require.NoError(t, err)
	defer server.Stop()

	for i := 0; i < 3; i++ {
		conn, err := DialTCP(ctx, server.Addr().String())
		require.NoError(t, err)
		err = PingPong(t, conn, 5*time.Second)
		require.NoError(t, err)
		conn.Close()
	}
}

func TestUDPEchoServerMultipleDatagrams(t *testing.T) {
	const testPort uint16 = 25103
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	server, err := NewUDPEchoServer(testPort)
	require.NoError(t, err)
	defer server.Stop()

	pc, err := DialUDP(ctx, server.Addr().String())
	require.NoError(t, err)
	defer pc.Close()

	for i := 0; i < 3; i++ {
		err = PingPongUDP(t, pc, server.Addr(), 5*time.Second)
		require.NoError(t, err)
	}
}

func TestTCPEchoServerStress(t *testing.T) {
	const testPort uint16 = 25104
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	server, err := NewTCPEchoServer(testPort)
	require.NoError(t, err)
	defer server.Stop()

	for i := 0; i < 10; i++ {
		conn, err := DialTCP(ctx, server.Addr().String())
		require.NoError(t, err)
		err = PingPong(t, conn, 5*time.Second)
		require.NoError(t, err)
		conn.Close()
	}
}

func TestSelfSignedCertificate(t *testing.T) {
	_, certPem, keyPem := CreateSelfSignedCertificate(t, "test.example.org")
	require.NotEmpty(t, certPem)
	require.NotEmpty(t, keyPem)
}

func TestWithRetry(t *testing.T) {
	var count int
	err := WithRetry(3, 10*time.Millisecond, func() error {
		count++
		if count < 2 {
			return context.DeadlineExceeded
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 2, count)
}

func TestListenPacketReuse(t *testing.T) {
	conn1, err := ListenPacket("udp", ":25105")
	require.NoError(t, err)
	defer conn1.Close()

	conn2, err := ListenPacket("udp", ":25105")
	require.NoError(t, err)
	defer conn2.Close()

	require.NotNil(t, conn1)
	require.NotNil(t, conn2)
}

func TestListenTCPReuse(t *testing.T) {
	l1, err := ListenTCP(":25106")
	require.NoError(t, err)
	defer l1.Close()

	l2, err := ListenTCP(":25106")
	require.NoError(t, err)
	defer l2.Close()

	require.NotNil(t, l1)
	require.NotNil(t, l2)
}

func TestLoopbackIP(t *testing.T) {
	ip := LoopbackIP()
	require.Equal(t, "127.0.0.1", ip.String())
	require.True(t, ip.IsLoopback())
}

func TestWithTimeout(t *testing.T) {
	ctx, cancel := WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	select {
	case <-time.After(200 * time.Millisecond):
		t.Fatal("context should have timed out")
	case <-ctx.Done():
		require.Equal(t, context.DeadlineExceeded, ctx.Err())
	}
}
