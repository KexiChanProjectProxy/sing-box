package firefoxvpn

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	boxTLS "github.com/sagernet/sing-box/common/tls"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

const (
	defaultSessionDialTimeout          = 10 * time.Second
	defaultSessionStreamTimeout        = 10 * time.Second
	defaultSessionMaxConcurrentStreams = 16
)

type SessionOptions struct {
	ServerAddr           M.Socksaddr
	TLSConfig            boxTLS.Config
	Dialer               N.Dialer
	ProxyPassToken       string
	DialTimeout          time.Duration
	StreamTimeout        time.Duration
	MaxConcurrentStreams int
}

type Session struct {
	serverAddr     M.Socksaddr
	tlsConfig      boxTLS.Config
	dialer         N.Dialer
	proxyPassToken string
	dialTimeout    time.Duration
	streamTimeout  time.Duration

	slots     chan struct{}
	closeOnce sync.Once
	closeCh   chan struct{}

	mu         sync.Mutex
	opening    bool
	ready      chan struct{}
	rawConn    net.Conn
	clientConn clientConn
	localAddr  net.Addr
	closed     bool
}

type ConnectResponseError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *ConnectResponseError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("proxy CONNECT failed: %s", e.Status)
	}
	return fmt.Sprintf("proxy CONNECT failed: %s: %s", e.Status, e.Body)
}

func NewSession(options SessionOptions) *Session {
	dialTimeout := options.DialTimeout
	if dialTimeout <= 0 {
		dialTimeout = defaultSessionDialTimeout
	}
	streamTimeout := options.StreamTimeout
	if streamTimeout <= 0 {
		streamTimeout = defaultSessionStreamTimeout
	}
	maxConcurrentStreams := options.MaxConcurrentStreams
	if maxConcurrentStreams <= 0 {
		maxConcurrentStreams = defaultSessionMaxConcurrentStreams
	}
	return &Session{
		serverAddr:     options.ServerAddr,
		tlsConfig:      options.TLSConfig,
		dialer:         options.Dialer,
		proxyPassToken: options.ProxyPassToken,
		dialTimeout:    dialTimeout,
		streamTimeout:  streamTimeout,
		slots:          make(chan struct{}, maxConcurrentStreams),
		closeCh:        make(chan struct{}),
	}
}

func (s *Session) ProxyPassToken() string {
	return s.proxyPassToken
}

func (s *Session) DialContext(ctx context.Context, destination M.Socksaddr) (net.Conn, error) {
	if err := s.ensureOpen(ctx); err != nil {
		return nil, err
	}
	release, err := s.acquireSlot(ctx)
	if err != nil {
		return nil, err
	}
	conn, err := s.openTunnel(ctx, destination, release)
	if err != nil {
		release()
		var responseErr *ConnectResponseError
		if !errors.As(err, &responseErr) {
			s.dropConnection()
		}
		return nil, err
	}
	return conn, nil
}

func (s *Session) Close() error {
	var closeErr error
	s.closeOnce.Do(func() {
		close(s.closeCh)
		s.mu.Lock()
		s.closed = true
		clientConn := s.clientConn
		rawConn := s.rawConn
		s.clientConn = nil
		s.rawConn = nil
		s.mu.Unlock()
		if clientConn != nil {
			closeErr = errors.Join(closeErr, normalizeSessionCloseError(clientConn.Close()))
		}
		if rawConn != nil {
			closeErr = errors.Join(closeErr, normalizeSessionCloseError(rawConn.Close()))
		}
	})
	return closeErr
}

func normalizeSessionCloseError(err error) error {
	if err == nil || errors.Is(err, net.ErrClosed) {
		return nil
	}
	if strings.Contains(err.Error(), "use of closed network connection") {
		return nil
	}
	return err
}
