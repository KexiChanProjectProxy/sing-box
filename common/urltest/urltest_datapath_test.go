package urltest

import (
	"context"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

type funcDialer struct {
	dial func(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error)
}

func (d funcDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return d.dial(ctx, network, destination)
}

func (d funcDialer) ListenPacket(context.Context, M.Socksaddr) (net.PacketConn, error) {
	return nil, errors.New("listen packet unused")
}

func head204(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func TestURLTestDataPathExcludesDialWait(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(head204))
	t.Cleanup(srv.Close)

	dialer := funcDialer{dial: func(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
		time.Sleep(150 * time.Millisecond)
		var d net.Dialer
		return d.DialContext(ctx, network, destination.String())
	}}
	ctx := context.Background()

	full, err := URLTest(ctx, srv.URL, dialer)
	if err != nil {
		t.Fatal(err)
	}
	if full < 100 {
		t.Fatalf("URLTest delay %d want >= 100", full)
	}
	data, err := URLTestDataPath(ctx, srv.URL, dialer)
	if err != nil {
		t.Fatal(err)
	}
	if data >= 100 {
		t.Fatalf("URLTestDataPath delay %d want < 100", data)
	}
}

type handshakeConn struct {
	net.Conn
	delayed atomic.Bool
}

func (c *handshakeConn) NeedHandshakeForWrite() bool { return true }

func (c *handshakeConn) Write(p []byte) (int, error) {
	if c.delayed.CompareAndSwap(false, true) {
		time.Sleep(150 * time.Millisecond)
	}
	return c.Conn.Write(p)
}

func TestURLTestDataPathExcludesWriteHandshake(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(head204))
	t.Cleanup(srv.Close)

	dialer := funcDialer{dial: func(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
		var d net.Dialer
		conn, err := d.DialContext(ctx, network, destination.String())
		if err != nil {
			return nil, err
		}
		return &handshakeConn{Conn: conn}, nil
	}}
	data, err := URLTestDataPath(context.Background(), srv.URL, dialer)
	if err != nil {
		t.Fatal(err)
	}
	if data >= 100 {
		t.Fatalf("URLTestDataPath delay %d want < 100", data)
	}
}

type testCertStore struct {
	pool *x509.CertPool
}

func (s *testCertStore) Name() string                   { return "test-certs" }
func (s *testCertStore) Start(adapter.StartStage) error { return nil }
func (s *testCertStore) Close() error                   { return nil }
func (s *testCertStore) Pool() *x509.CertPool           { return s.pool }
func (s *testCertStore) ExclusiveAnchors() bool         { return false }

type delayedReadConn struct {
	net.Conn
	once sync.Once
}

func (c *delayedReadConn) Read(p []byte) (int, error) {
	c.once.Do(func() { time.Sleep(150 * time.Millisecond) })
	return c.Conn.Read(p)
}

func TestURLTestDataPathHTTPSExcludesHandshake(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(head204))
	t.Cleanup(srv.Close)

	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	ctx := service.ContextWith[adapter.CertificateStore](context.Background(), &testCertStore{pool: pool})

	dialer := funcDialer{dial: func(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
		var d net.Dialer
		conn, err := d.DialContext(ctx, network, destination.String())
		if err != nil {
			return nil, err
		}
		return &delayedReadConn{Conn: conn}, nil
	}}

	full, err := URLTest(ctx, srv.URL, dialer)
	if err != nil {
		t.Fatal(err)
	}
	if full < 100 {
		t.Fatalf("URLTest delay %d want >= 100", full)
	}
	data, err := URLTestDataPath(ctx, srv.URL, dialer)
	if err != nil {
		t.Fatal(err)
	}
	if data >= 100 {
		t.Fatalf("URLTestDataPath delay %d want < 100", data)
	}
}

var _ N.EarlyWriter = (*handshakeConn)(nil)
