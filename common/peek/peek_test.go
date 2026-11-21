package peek

import (
	"bytes"
	"crypto/tls"
	"io"
	"math/rand/v2"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// clientHelloPrefix records the first bytes a crypto/tls client sends.
func clientHelloPrefix(t *testing.T, config *tls.Config) []byte {
	t.Helper()
	client, server := net.Pipe()
	defer server.Close()
	go func() {
		_ = tls.Client(client, config).Handshake()
		client.Close()
	}()
	prefix := make([]byte, ClientHelloPrefixLength)
	_, err := io.ReadFull(server, prefix)
	require.NoError(t, err)
	return prefix
}

func TestClassifyClientHello(t *testing.T) {
	t.Parallel()
	for _, config := range []*tls.Config{
		{ServerName: "example.org", MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12},
		{ServerName: "example.org", MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13},
	} {
		prefix := clientHelloPrefix(t, config)
		require.Equal(t, TLS, ClassifyClientHello(prefix), "%x", prefix)
		for n := 1; n < ClientHelloPrefixLength; n++ {
			require.Equal(t, Undecided, ClassifyClientHello(prefix[:n]), "%x", prefix[:n])
		}
	}

	valid := []byte{0x16, 0x03, 0x01, 0x02, 0x00, 0x01}
	for _, tc := range []struct {
		name   string
		prefix []byte
		want   Result
	}{
		{"empty", nil, Undecided},
		{"tls 1.0 record", []byte{0x16, 0x03, 0x01, 0x00, 0x30, 0x01}, TLS},
		{"ssl 3.0 record", []byte{0x16, 0x03, 0x00, 0x00, 0x30, 0x01}, TLS},
		{"record version 3.4", []byte{0x16, 0x03, 0x04, 0x00, 0x30, 0x01}, TLS},
		{"longest record", []byte{0x16, 0x03, 0x01, 0x48, 0x00, 0x01}, TLS},
		{"first byte", []byte{0x17}, NonTLS},
		{"major version", []byte{0x16, 0x02}, NonTLS},
		{"minor version b[2]", []byte{0x16, 0x03, 0x05}, NonTLS},
		{"minor version b[2] full", []byte{0x16, 0x03, 0x05, 0x02, 0x00, 0x01}, NonTLS},
		{"record too long, early", []byte{0x16, 0x03, 0x01, 0x49}, NonTLS},
		{"record too long", []byte{0x16, 0x03, 0x01, 0x48, 0x01}, NonTLS},
		{"handshake type b[5]", []byte{0x16, 0x03, 0x01, 0x02, 0x00, 0x02}, NonTLS},
		{"server hello b[5]", []byte{0x16, 0x03, 0x03, 0x00, 0x30, 0x00}, NonTLS},
	} {
		require.Equal(t, tc.want, ClassifyClientHello(tc.prefix), tc.name)
	}
	require.Equal(t, TLS, ClassifyClientHello(valid))
}

func TestClassifyRandom(t *testing.T) {
	t.Parallel()
	// A million random prefixes of 1 to 6 bytes, like the start of a
	// Shadowsocks salt, are non-TLS, or undecided while they still start
	// like a ClientHello. All six bytes match by chance about 3e-10 of the
	// time, so none of them is TLS. A fixed seed keeps the test
	// deterministic.
	const samples = 1_000_000
	random := rand.NewChaCha8([32]byte{'p', 'e', 'e', 'k'})
	sample := make([]byte, ClientHelloPrefixLength*samples)
	_, err := random.Read(sample)
	require.NoError(t, err)
	var undecided, nonTLS int
	for i := 0; i < samples; i++ {
		b := sample[i*ClientHelloPrefixLength : (i+1)*ClientHelloPrefixLength]
		n := 1 + i%ClientHelloPrefixLength
		switch ClassifyClientHello(b[:n]) {
		case TLS:
			t.Fatalf("random prefix %x taken for TLS", b[:n])
		case Undecided:
			if b[0] != 0x16 {
				t.Fatalf("prefix %x undecided", b[:n])
			}
			undecided++
		case NonTLS:
			nonTLS++
		}
	}
	require.Equal(t, samples, undecided+nonTLS)
	// The classifier itself does accept a matching prefix of the same
	// shape, so the loop above could have failed.
	require.Equal(t, TLS, ClassifyClientHello([]byte{0x16, 0x03, 0x03, 0x01, 0x00, 0x01}))
}

func BenchmarkClassifyClientHello(b *testing.B) {
	prefix := []byte{0x16, 0x03, 0x01, 0x02, 0x00, 0x01}
	for i := 0; i < b.N; i++ {
		ClassifyClientHello(prefix)
	}
}

func tcpPair(t *testing.T) (client net.Conn, server net.Conn) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, _ := listener.Accept()
		accepted <- conn
	}()
	client, err = net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	server = <-accepted
	require.NotNil(t, server)
	t.Cleanup(func() {
		client.Close()
		server.Close()
	})
	return
}

type peekResult struct {
	conn  net.Conn
	isTLS bool
	err   error
	took  time.Duration
}

func peekAsync(conn net.Conn, timeout time.Duration) <-chan peekResult {
	done := make(chan peekResult, 1)
	go func() {
		start := time.Now()
		peeked, isTLS, err := ClientHello(conn, timeout)
		done <- peekResult{peeked, isTLS, err, time.Since(start)}
	}()
	return done
}

func readAll(t *testing.T, conn net.Conn, n int) []byte {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	data := make([]byte, n)
	_, err := io.ReadFull(conn, data)
	require.NoError(t, err)
	require.NoError(t, conn.SetReadDeadline(time.Time{}))
	return data
}

func TestClientHelloSplitWrite(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		payload []byte
		isTLS   bool
	}{
		{"tls", append([]byte{0x16, 0x03, 0x01, 0x02, 0x00, 0x01}, bytes.Repeat([]byte{0xAA}, 512)...), true},
		{"tls rejected at b[5]", append([]byte{0x16, 0x03, 0x01, 0x02, 0x00, 0x02}, bytes.Repeat([]byte{0xAA}, 512)...), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, server := tcpPair(t)
			done := peekAsync(server, 5*time.Second)
			_, err := client.Write(tc.payload[:1])
			require.NoError(t, err)
			select {
			case result := <-done:
				t.Fatalf("decided on one byte: %+v", result)
			case <-time.After(200 * time.Millisecond):
			}
			_, err = client.Write(tc.payload[1:])
			require.NoError(t, err)
			result := <-done
			require.NoError(t, result.err)
			require.Equal(t, tc.isTLS, result.isTLS)
			assertPlainSocket(t, server, result.conn)
			require.Equal(t, tc.payload, readAll(t, result.conn, len(tc.payload)))
		})
	}
}

func TestClientHelloEarlyDecision(t *testing.T) {
	t.Parallel()
	client, server := tcpPair(t)
	done := peekAsync(server, 5*time.Second)
	_, err := client.Write([]byte{0x00})
	require.NoError(t, err)
	select {
	case result := <-done:
		require.NoError(t, result.err)
		require.False(t, result.isTLS)
		assertPlainSocket(t, server, result.conn)
		require.Equal(t, []byte{0x00}, readAll(t, result.conn, 1))
	case <-time.After(time.Second):
		t.Fatal("first byte did not decide")
	}
}

func TestClientHelloTimeout(t *testing.T) {
	t.Parallel()
	client, server := tcpPair(t)
	// A prefix that is still a possible ClientHello.
	_, err := client.Write([]byte{0x16, 0x03})
	require.NoError(t, err)
	result := <-peekAsync(server, 300*time.Millisecond)
	require.NoError(t, result.err)
	require.False(t, result.isTLS)
	require.GreaterOrEqual(t, result.took, 250*time.Millisecond)
	assertPlainSocket(t, server, result.conn)
	// The deadline is cleared: reading waits for data, not for the deadline.
	go func() {
		time.Sleep(100 * time.Millisecond)
		_, _ = client.Write([]byte{0x05})
	}()
	data := make([]byte, 3)
	_, err = io.ReadFull(result.conn, data)
	require.NoError(t, err)
	require.Equal(t, []byte{0x16, 0x03, 0x05}, data)
}

func TestClientHelloEOF(t *testing.T) {
	t.Parallel()
	client, server := tcpPair(t)
	require.NoError(t, client.Close())
	result := <-peekAsync(server, 5*time.Second)
	require.ErrorIs(t, result.err, io.EOF)
}

func TestClientHelloPartialThenClose(t *testing.T) {
	t.Parallel()
	client, server := tcpPair(t)
	done := peekAsync(server, 5*time.Second)
	_, err := client.Write([]byte{0x16, 0x03, 0x01})
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	require.NoError(t, client.Close())
	select {
	case result := <-done:
		require.ErrorIs(t, result.err, io.ErrUnexpectedEOF)
		require.Less(t, result.took, 2*time.Second)
	case <-time.After(3 * time.Second):
		t.Fatal("peer close was not noticed")
	}
}

func TestClientHelloPartialAndCloseBeforePeek(t *testing.T) {
	t.Parallel()
	client, server := tcpPair(t)
	_, err := client.Write([]byte{0x16, 0x03, 0x01})
	require.NoError(t, err)
	require.NoError(t, client.Close())
	// Let the bytes and the FIN reach the socket before the peek starts.
	time.Sleep(100 * time.Millisecond)
	select {
	case result := <-peekAsync(server, 5*time.Second):
		require.ErrorIs(t, result.err, io.ErrUnexpectedEOF)
		require.Less(t, result.took, time.Second)
	case <-time.After(3 * time.Second):
		t.Fatal("peer close was not noticed")
	}
}

func TestClientHelloPartialThenHalfClose(t *testing.T) {
	t.Parallel()
	client, server := tcpPair(t)
	done := peekAsync(server, 5*time.Second)
	_, err := client.Write([]byte{0x16, 0x03, 0x01})
	require.NoError(t, err)
	require.NoError(t, client.(*net.TCPConn).CloseWrite())
	select {
	case result := <-done:
		require.ErrorIs(t, result.err, io.ErrUnexpectedEOF)
	case <-time.After(3 * time.Second):
		t.Fatal("peer close was not noticed")
	}
}

func TestClientHelloUnixSocket(t *testing.T) {
	t.Parallel()
	path := t.TempDir() + "/peek.sock"
	listener, err := net.Listen("unix", path)
	require.NoError(t, err)
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, _ := listener.Accept()
		accepted <- conn
	}()
	client, err := net.Dial("unix", path)
	require.NoError(t, err)
	defer client.Close()
	server := <-accepted
	require.NotNil(t, server)
	defer server.Close()
	payload := []byte{0x16, 0x03, 0x01, 0x02, 0x00, 0x01, 0xAA}
	_, err = client.Write(payload[:2])
	require.NoError(t, err)
	done := peekAsync(server, 5*time.Second)
	time.Sleep(100 * time.Millisecond)
	_, err = client.Write(payload[2:])
	require.NoError(t, err)
	result := <-done
	require.NoError(t, result.err)
	require.True(t, result.isTLS)
	require.Equal(t, payload, readAll(t, result.conn, len(payload)))
}

func TestClientHelloTimeoutWithoutBytes(t *testing.T) {
	t.Parallel()
	for _, wrap := range []bool{false, true} {
		client, server := tcpPair(t)
		var conn net.Conn = server
		if wrap {
			conn = wrappedConn{server}
		}
		result := <-peekAsync(conn, 200*time.Millisecond)
		require.NoError(t, result.err)
		require.False(t, result.isTLS)
		_, err := client.Write([]byte{0x01})
		require.NoError(t, err)
		require.Equal(t, []byte{0x01}, readAll(t, result.conn, 1))
	}
}

// wrappedConn hides the socket type, as a wrapper such as a proxy protocol
// reader would.
type wrappedConn struct {
	net.Conn
}

func TestClientHelloFallback(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		payload []byte
		isTLS   bool
	}{
		{"tls", []byte{0x16, 0x03, 0x01, 0x02, 0x00, 0x01, 0xAA, 0xBB}, true},
		{"non-tls", []byte{0x01, 0x02, 0x03, 0x04}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, server := tcpPair(t)
			_, err := client.Write(tc.payload)
			require.NoError(t, err)
			result := <-peekAsync(wrappedConn{server}, 5*time.Second)
			require.NoError(t, result.err)
			require.Equal(t, tc.isTLS, result.isTLS)
			require.Equal(t, tc.payload, readAll(t, result.conn, len(tc.payload)))
		})
	}
	t.Run("timeout", func(t *testing.T) {
		t.Parallel()
		client, server := tcpPair(t)
		_, err := client.Write([]byte{0x16})
		require.NoError(t, err)
		result := <-peekAsync(wrappedConn{server}, 200*time.Millisecond)
		require.NoError(t, result.err)
		require.False(t, result.isTLS)
		require.Equal(t, []byte{0x16}, readAll(t, result.conn, 1))
	})
	t.Run("partial then close", func(t *testing.T) {
		t.Parallel()
		client, server := tcpPair(t)
		_, err := client.Write([]byte{0x16, 0x03})
		require.NoError(t, err)
		require.NoError(t, client.Close())
		result := <-peekAsync(wrappedConn{server}, 5*time.Second)
		require.ErrorIs(t, result.err, io.ErrUnexpectedEOF)
	})
}

func BenchmarkClientHello(b *testing.B) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(b, err)
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, _ := listener.Accept()
		accepted <- conn
	}()
	client, err := net.Dial("tcp", listener.Addr().String())
	require.NoError(b, err)
	defer client.Close()
	server := <-accepted
	defer server.Close()
	_, err = client.Write([]byte{0x16, 0x03, 0x01, 0x02, 0x00, 0x01})
	require.NoError(b, err)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, isTLS, err := ClientHello(server, 2*time.Second)
		if err != nil || !isTLS {
			b.Fatal(err, isTLS)
		}
	}
}
