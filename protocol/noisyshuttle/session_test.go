package noisyshuttle

import (
	"context"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func TestOutboundSessionReuse(t *testing.T) {
	server := newReusableMockServer(t, "secret", 4, true)
	defer server.Close()
	outbound := newTestSessionOutbound(t, server.Addr().String(), "secret", SessionOptions{MaxStreams: 4, KeepaliveInterval: time.Hour}, true)

	for i := 0; i < 2; i++ {
		conn, err := outbound.DialContext(context.Background(), N.NetworkTCP, M.ParseSocksaddrHostPort("example.com", 443))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = conn.Write([]byte("ping")); err != nil {
			t.Fatal(err)
		}
		buf := make([]byte, 4)
		if _, err = io.ReadFull(conn, buf); err != nil {
			t.Fatal(err)
		}
		_ = conn.Close()
	}
	if got := atomic.LoadInt32(&server.accepted); got != 1 {
		t.Fatalf("expected one underlying session, got %d", got)
	}
}

func TestOutboundSessionMaxStreamsExceededOpensNewSession(t *testing.T) {
	server := newReusableMockServer(t, "secret", 1, true)
	defer server.Close()
	outbound := newTestSessionOutbound(t, server.Addr().String(), "secret", SessionOptions{MaxStreams: 1, KeepaliveInterval: time.Hour}, true)

	for i := 0; i < 2; i++ {
		conn, err := outbound.DialContext(context.Background(), N.NetworkTCP, M.ParseSocksaddrHostPort("example.com", 443))
		if err != nil {
			t.Fatal(err)
		}
		_ = conn.Close()
	}
	if got := atomic.LoadInt32(&server.accepted); got != 2 {
		t.Fatalf("expected two underlying sessions, got %d", got)
	}
}

func TestOutboundSessionMaxAgeExceededOpensNewSession(t *testing.T) {
	server := newReusableMockServer(t, "secret", 4, true)
	defer server.Close()
	outbound := newTestSessionOutbound(t, server.Addr().String(), "secret", SessionOptions{MaxStreams: 4, MaxAge: 30 * time.Millisecond, KeepaliveInterval: time.Hour}, true)

	conn, err := outbound.DialContext(context.Background(), N.NetworkTCP, M.ParseSocksaddrHostPort("example.com", 443))
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	time.Sleep(50 * time.Millisecond)
	conn, err = outbound.DialContext(context.Background(), N.NetworkTCP, M.ParseSocksaddrHostPort("example.com", 443))
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	if got := atomic.LoadInt32(&server.accepted); got != 2 {
		t.Fatalf("expected max_age to force new session, got %d", got)
	}
}

func TestOutboundKeepaliveTimeoutClosesSession(t *testing.T) {
	server := newReusableMockServer(t, "secret", 4, false)
	defer server.Close()
	outbound := newTestSessionOutbound(t, server.Addr().String(), "secret", SessionOptions{MaxStreams: 4, KeepaliveInterval: 20 * time.Millisecond}, true)

	conn, err := outbound.DialContext(context.Background(), N.NetworkTCP, M.ParseSocksaddrHostPort("example.com", 443))
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	time.Sleep(80 * time.Millisecond)
	outbound.sessionPool.mu.Lock()
	session := outbound.sessionPool.session
	outbound.sessionPool.mu.Unlock()
	if session != nil {
		t.Fatal("expected keepalive timeout to remove session from pool")
	}
}

func TestOutboundIdleTimeoutClosesSession(t *testing.T) {
	server := newReusableMockServer(t, "secret", 4, true)
	defer server.Close()
	outbound := newTestSessionOutbound(t, server.Addr().String(), "secret", SessionOptions{MaxStreams: 4, IdleTimeout: 30 * time.Millisecond, KeepaliveInterval: time.Hour}, true)

	conn, err := outbound.DialContext(context.Background(), N.NetworkTCP, M.ParseSocksaddrHostPort("example.com", 443))
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	time.Sleep(80 * time.Millisecond)
	outbound.sessionPool.mu.Lock()
	session := outbound.sessionPool.session
	outbound.sessionPool.mu.Unlock()
	if session != nil {
		t.Fatal("expected idle timeout to remove session from pool")
	}
}

func newTestSessionOutbound(t *testing.T, server string, password string, session SessionOptions, enabled bool) *Outbound {
	t.Helper()
	serverAddr := M.ParseSocksaddr(server)
	options := option.NoisyShuttleOutboundOptions{
		ServerOptions: option.ServerOptions{Server: serverAddr.AddrString(), ServerPort: serverAddr.Port},
		Password:      password,
		Network:       option.NetworkList(N.NetworkTCP),
	}
	options.Session.Enabled = enabled
	options.Session.MaxStreams = session.MaxStreams
	options.Session.MaxRequests = session.MaxRequests
	options.Session.IdleTimeout = badoption.Duration(session.IdleTimeout)
	options.Session.MaxAge = badoption.Duration(session.MaxAge)
	options.Session.KeepaliveInterval = badoption.Duration(session.KeepaliveInterval)
	outbound, err := NewOutbound(context.Background(), nil, log.NewNOPFactory().NewLogger("noisyshuttle-test"), "test", options)
	if err != nil {
		t.Fatal(err)
	}
	return outbound.(*Outbound)
}

type reusableMockServer struct {
	listener net.Listener
	done     chan struct{}
	password string
	max      uint16
	pong     bool
	accepted int32
}

func newReusableMockServer(t *testing.T, password string, maxStreams uint16, pong bool) *reusableMockServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &reusableMockServer{listener: listener, done: make(chan struct{}), password: password, max: maxStreams, pong: pong}
	go server.serve()
	return server
}

func (s *reusableMockServer) Addr() net.Addr { return s.listener.Addr() }

func (s *reusableMockServer) Close() {
	_ = s.listener.Close()
	select {
	case <-s.done:
	case <-time.After(time.Second):
	}
}

func (s *reusableMockServer) serve() {
	defer close(s.done)
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		atomic.AddInt32(&s.accepted, 1)
		go s.handle(conn)
	}
}

func (s *reusableMockServer) handle(conn net.Conn) {
	defer conn.Close()
	preface, _, err := DecodePreface(conn, DefaultServerMaxPadding)
	if err != nil || !VerifyPrefaceHash(preface, s.password) {
		return
	}
	frame, err := ReadFrame(conn, MaxPayloadLength)
	if err != nil || frame.Type != FrameTypeClientHello {
		return
	}
	clientHello, _ := DecodeHello(frame.Payload)
	maxStreams := s.max
	if clientHello.MaxStreams < maxStreams {
		maxStreams = clientHello.MaxStreams
	}
	_ = WriteBufferedFrame(conn, FrameTypeServerHello, 0, 0, EncodeHello(Hello{Version: ProtocolVersion, Capabilities: clientHello.Capabilities, MaxStreams: maxStreams}))
	served := uint16(0)
	for served < maxStreams {
		frame, err = ReadFrame(conn, MaxPayloadLength)
		if err != nil {
			return
		}
		switch frame.Type {
		case FrameTypeOpenRequest:
			served++
			s.handleStream(conn, frame.StreamID)
		case FrameTypePing:
			if s.pong {
				_ = WriteBufferedFrame(conn, FrameTypePong, 0, 0, frame.Payload)
			}
		case FrameTypeClose:
			return
		}
	}
}

func (s *reusableMockServer) handleStream(conn net.Conn, streamID uint32) {
	payload, _ := EncodeOpenResponse(OpenResponse{Status: ErrorOK})
	_ = WriteBufferedFrame(conn, FrameTypeOpenResponse, 0, streamID, payload)
	for {
		frame, err := ReadFrame(conn, MaxPayloadLength)
		if err != nil {
			return
		}
		switch frame.Type {
		case FrameTypeData:
			_ = WriteBufferedFrame(conn, FrameTypeData, 0, frame.StreamID, frame.Payload)
		case FrameTypeEndResponse, FrameTypeEndRequest:
			_ = WriteBufferedFrame(conn, FrameTypeEndResponse, 0, frame.StreamID, nil)
			return
		case FrameTypePing:
			if s.pong {
				_ = WriteBufferedFrame(conn, FrameTypePong, 0, 0, frame.Payload)
			}
		}
	}
}
