package firefoxvpn

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	boxTLS "github.com/sagernet/sing-box/common/tls"
	M "github.com/sagernet/sing/common/metadata"

	"golang.org/x/net/http2"
)

type clientConn interface {
	Close() error
	RoundTrip(*http.Request) (*http.Response, error)
	State() http2.ClientConnState
}

func (s *Session) ensureOpen(ctx context.Context) error {
	for {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return net.ErrClosed
		}
		if s.clientConn != nil {
			state := s.clientConn.State()
			if !state.Closed && !state.Closing {
				s.mu.Unlock()
				return nil
			}
			clientConn := s.clientConn
			rawConn := s.rawConn
			s.clientConn = nil
			s.rawConn = nil
			s.mu.Unlock()
			if clientConn != nil {
				_ = clientConn.Close()
			}
			if rawConn != nil {
				_ = rawConn.Close()
			}
			continue
		}
		if s.opening {
			ready := s.ready
			s.mu.Unlock()
			select {
			case <-ready:
				continue
			case <-ctx.Done():
				return ctx.Err()
			case <-s.closeCh:
				return net.ErrClosed
			}
		}
		ready := make(chan struct{})
		s.opening = true
		s.ready = ready
		s.mu.Unlock()

		rawConn, clientConn, err := s.openClientConn(ctx)

		s.mu.Lock()
		if err == nil {
			s.rawConn = rawConn
			s.clientConn = clientConn
			s.localAddr = rawConn.LocalAddr()
		}
		s.opening = false
		s.ready = nil
		close(ready)
		s.mu.Unlock()
		return err
	}
}

func (s *Session) openClientConn(ctx context.Context) (net.Conn, clientConn, error) {
	openCtx, cancel := s.withDialTimeout(ctx)
	defer cancel()
	tlsConfig := s.tlsConfig.Clone()
	if len(tlsConfig.NextProtos()) == 0 {
		tlsConfig.SetNextProtos([]string{http2.NextProtoTLS})
	}
	tlsDialer := boxTLS.NewDialer(s.dialer, tlsConfig)
	rawConn, err := tlsDialer.DialTLSContext(openCtx, s.serverAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("open firefox-vpn upstream TLS session: %w", err)
	}
	if rawConn.ConnectionState().NegotiatedProtocol != http2.NextProtoTLS {
		_ = rawConn.Close()
		return nil, nil, fmt.Errorf("open firefox-vpn upstream TLS session: negotiated protocol %q, want %q", rawConn.ConnectionState().NegotiatedProtocol, http2.NextProtoTLS)
	}
	clientConn, err := new(http2.Transport).NewClientConn(rawConn)
	if err != nil {
		_ = rawConn.Close()
		return nil, nil, fmt.Errorf("create firefox-vpn HTTP/2 client connection: %w", err)
	}
	return rawConn, clientConn, nil
}

func (s *Session) openTunnel(ctx context.Context, destination M.Socksaddr, release func()) (net.Conn, error) {
	s.mu.Lock()
	clientConn := s.clientConn
	localAddr := s.localAddr
	s.mu.Unlock()
	if clientConn == nil {
		return nil, net.ErrClosed
	}

	authority := destination.String()
	requestCtx, requestCancel := context.WithCancel(context.Background())
	pipeReader, pipeWriter := io.Pipe()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodConnect, "http://"+authority, pipeReader)
	if err != nil {
		requestCancel()
		_ = pipeReader.Close()
		_ = pipeWriter.Close()
		return nil, fmt.Errorf("create CONNECT request: %w", err)
	}
	request.Host = authority
	request.URL.Host = s.serverAddr.String()
	request.Header.Set("Proxy-Authorization", "Bearer "+s.proxyPassToken)

	type roundTripResult struct {
		response *http.Response
		err      error
	}
	resultCh := make(chan roundTripResult, 1)
	go func() {
		response, roundTripErr := clientConn.RoundTrip(request)
		resultCh <- roundTripResult{response: response, err: roundTripErr}
	}()

	timeout := time.NewTimer(s.streamTimeout)
	defer timeout.Stop()

	select {
	case <-ctx.Done():
		requestCancel()
		_ = pipeReader.CloseWithError(ctx.Err())
		_ = pipeWriter.CloseWithError(ctx.Err())
		return nil, ctx.Err()
	case <-s.closeCh:
		requestCancel()
		_ = pipeReader.CloseWithError(net.ErrClosed)
		_ = pipeWriter.CloseWithError(net.ErrClosed)
		return nil, net.ErrClosed
	case <-timeout.C:
		requestCancel()
		_ = pipeReader.CloseWithError(context.DeadlineExceeded)
		_ = pipeWriter.CloseWithError(context.DeadlineExceeded)
		return nil, fmt.Errorf("CONNECT %s timeout after %s: %w", authority, s.streamTimeout, context.DeadlineExceeded)
	case result := <-resultCh:
		if result.err != nil {
			requestCancel()
			_ = pipeReader.CloseWithError(result.err)
			_ = pipeWriter.CloseWithError(result.err)
			return nil, fmt.Errorf("CONNECT %s via firefox-vpn upstream: %w", authority, result.err)
		}
		if result.response.StatusCode < http.StatusOK || result.response.StatusCode >= http.StatusMultipleChoices {
			body, _ := io.ReadAll(io.LimitReader(result.response.Body, 2048))
			_ = result.response.Body.Close()
			requestCancel()
			_ = pipeReader.CloseWithError(io.EOF)
			_ = pipeWriter.CloseWithError(io.EOF)
			return nil, &ConnectResponseError{
				StatusCode: result.response.StatusCode,
				Status:     result.response.Status,
				Body:       strings.TrimSpace(string(body)),
			}
		}
		return newConnectConn(connectConnOptions{
			reader:      result.response.Body,
			writer:      pipeWriter,
			requestBody: pipeReader,
			cancel:      requestCancel,
			localAddr:   localAddr,
			remoteAddr:  destination,
			release:     release,
		}), nil
	}
}

func (s *Session) acquireSlot(ctx context.Context) (func(), error) {
	acquireCtx, cancel := s.withStreamTimeout(ctx)
	defer cancel()
	select {
	case s.slots <- struct{}{}:
		return func() { <-s.slots }, nil
	case <-acquireCtx.Done():
		return nil, acquireCtx.Err()
	case <-s.closeCh:
		return nil, net.ErrClosed
	}
}

func (s *Session) withDialTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	openCtx, cancel := context.WithTimeout(ctx, s.dialTimeout)
	stop := make(chan struct{})
	go func() {
		select {
		case <-s.closeCh:
			cancel()
		case <-stop:
		}
	}()
	return openCtx, func() {
		close(stop)
		cancel()
	}
}

func (s *Session) withStreamTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if s.streamTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, s.streamTimeout)
}

func (s *Session) dropConnection() {
	s.mu.Lock()
	clientConn := s.clientConn
	rawConn := s.rawConn
	s.clientConn = nil
	s.rawConn = nil
	s.mu.Unlock()
	if clientConn != nil {
		_ = normalizeSessionCloseError(clientConn.Close())
	}
	if rawConn != nil {
		_ = normalizeSessionCloseError(rawConn.Close())
	}
}
