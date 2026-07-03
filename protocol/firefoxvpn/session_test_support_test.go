package firefoxvpn

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	boxTLS "github.com/sagernet/sing-box/common/tls"
	boxlog "github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

type sessionTestConfig struct {
	dialTimeout          time.Duration
	streamTimeout        time.Duration
	maxConcurrentStreams int
	dialer               N.Dialer
}

type sessionTestServer struct {
	server *httptest.Server

	connectionCount atomic.Int32

	mu           sync.Mutex
	statusByHost map[string]int
	delayByHost  map[string]time.Duration
}

func newSessionTestServer(t *testing.T) *sessionTestServer {
	t.Helper()
	state := &sessionTestServer{
		statusByHost: make(map[string]int),
		delayByHost:  make(map[string]time.Duration),
	}
	unstarted := httptest.NewUnstartedServer(http.HandlerFunc(state.handle))
	unstarted.EnableHTTP2 = true
	unstarted.Config.ConnState = func(_ net.Conn, connState http.ConnState) {
		if connState == http.StateNew {
			state.connectionCount.Add(1)
		}
	}
	unstarted.StartTLS()
	state.server = unstarted
	t.Cleanup(state.server.Close)
	return state
}

func (s *sessionTestServer) setStatus(authority string, statusCode int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statusByHost[authority] = statusCode
}

func (s *sessionTestServer) setResponseDelay(authority string, delay time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.delayByHost[authority] = delay
}

func (s *sessionTestServer) handle(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodConnect {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if request.ProtoMajor != 2 {
		writer.WriteHeader(http.StatusHTTPVersionNotSupported)
		return
	}
	if request.Header.Get("Proxy-Authorization") != "Bearer proxy-pass-token" {
		writer.WriteHeader(http.StatusProxyAuthRequired)
		_, _ = writer.Write([]byte("missing proxy auth"))
		return
	}

	s.mu.Lock()
	statusCode := s.statusByHost[request.Host]
	delay := s.delayByHost[request.Host]
	s.mu.Unlock()

	if delay > 0 {
		time.Sleep(delay)
	}
	if statusCode != 0 {
		writer.WriteHeader(statusCode)
		_, _ = writer.Write([]byte("request blocked"))
		return
	}

	writer.WriteHeader(http.StatusOK)
	flusher, ok := writer.(http.Flusher)
	if ok {
		flusher.Flush()
	}
	_, _ = io.Copy(flushWriter{Writer: writer, Flusher: flusher}, request.Body)
}

func (s *sessionTestServer) newSession(t *testing.T, config sessionTestConfig) *Session {
	t.Helper()
	parsedURL, err := url.Parse(s.server.URL)
	require.NoError(t, err)
	tlsConfig, err := boxTLS.NewClient(t.Context(), boxlog.NewNOPFactory().Logger(), parsedURL.Hostname(), option.OutboundTLSOptions{
		Enabled:    true,
		Insecure:   true,
		ServerName: parsedURL.Hostname(),
	})
	require.NoError(t, err)
	dialer := config.dialer
	if dialer == nil {
		dialer = testDialer{}
	}
	return NewSession(SessionOptions{
		ServerAddr:           M.ParseSocksaddr(parsedURL.Host),
		TLSConfig:            tlsConfig,
		Dialer:               dialer,
		ProxyPassToken:       "proxy-pass-token",
		DialTimeout:          config.dialTimeout,
		StreamTimeout:        config.streamTimeout,
		MaxConcurrentStreams: config.maxConcurrentStreams,
	})
}

func newDialTimeoutSession(t *testing.T, dialTimeout time.Duration) *Session {
	t.Helper()
	tlsConfig, err := boxTLS.NewClient(t.Context(), boxlog.NewNOPFactory().Logger(), "vpn.example.test", option.OutboundTLSOptions{
		Enabled:    true,
		Insecure:   true,
		ServerName: "vpn.example.test",
	})
	require.NoError(t, err)
	return NewSession(SessionOptions{
		ServerAddr:           M.ParseSocksaddr("vpn.example.test:443"),
		TLSConfig:            tlsConfig,
		Dialer:               blockingDialer{},
		ProxyPassToken:       "proxy-pass-token",
		DialTimeout:          dialTimeout,
		StreamTimeout:        time.Second,
		MaxConcurrentStreams: 1,
	})
}

type flushWriter struct {
	Writer  io.Writer
	Flusher http.Flusher
}

func (w flushWriter) Write(buffer []byte) (int, error) {
	n, err := w.Writer.Write(buffer)
	if err == nil && w.Flusher != nil {
		w.Flusher.Flush()
	}
	return n, err
}

type testDialer struct{}

func (testDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	var dialer net.Dialer
	return dialer.DialContext(ctx, network, destination.String())
}

func (testDialer) ListenPacket(context.Context, M.Socksaddr) (net.PacketConn, error) {
	return nil, os.ErrInvalid
}

type blockingDialer struct{}

func (blockingDialer) DialContext(ctx context.Context, _ string, _ M.Socksaddr) (net.Conn, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (blockingDialer) ListenPacket(context.Context, M.Socksaddr) (net.PacketConn, error) {
	return nil, os.ErrInvalid
}

func roundTripPayload(conn net.Conn, payload string) error {
	if _, err := conn.Write([]byte(payload)); err != nil {
		return err
	}
	response := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, response); err != nil {
		return err
	}
	if string(response) != payload {
		return fmt.Errorf("unexpected tunnel payload: got %q want %q", string(response), payload)
	}
	return nil
}
