package noisyshuttle

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func TestOutboundTCPEcho(t *testing.T) {
	server := newMockNoisyServer(t, "secret")
	defer server.Close()

	outbound := newTestOutbound(t, server.Addr().String(), "secret")
	conn, err := outbound.DialContext(context.Background(), N.NetworkTCP, M.ParseSocksaddrHostPort("example.com", 443))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	payload := []byte("hello noisy shuttle")
	if _, err = conn.Write(payload); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, len(payload))
	if _, err = io.ReadFull(conn, buffer); err != nil {
		t.Fatal(err)
	}
	if string(buffer) != string(payload) {
		t.Fatalf("unexpected echo: %q", buffer)
	}
}

func TestOutboundBadAuthFailsClosed(t *testing.T) {
	server := newMockNoisyServer(t, "secret")
	defer server.Close()

	outbound := newTestOutbound(t, server.Addr().String(), "wrong")
	_, err := outbound.DialContext(context.Background(), N.NetworkTCP, M.ParseSocksaddrHostPort("example.com", 443))
	if err == nil {
		t.Fatal("expected bad auth to fail")
	}
}

func newTestOutbound(t *testing.T, server string, password string) *Outbound {
	t.Helper()
	serverAddr := M.ParseSocksaddr(server)
	outbound, err := NewOutbound(context.Background(), nil, log.NewNOPFactory().NewLogger("noisyshuttle-test"), "test", option.NoisyShuttleOutboundOptions{
		ServerOptions: option.ServerOptions{Server: serverAddr.AddrString(), ServerPort: serverAddr.Port},
		Password:      password,
		Network:       option.NetworkList(N.NetworkTCP),
	})
	if err != nil {
		t.Fatal(err)
	}
	return outbound.(*Outbound)
}

type mockNoisyServer struct {
	listener net.Listener
	done     chan struct{}
	password string
}

func newMockNoisyServer(t *testing.T, password string) *mockNoisyServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &mockNoisyServer{listener: listener, done: make(chan struct{}), password: password}
	go server.serve()
	return server
}

func (s *mockNoisyServer) Addr() net.Addr {
	return s.listener.Addr()
}

func (s *mockNoisyServer) Close() {
	_ = s.listener.Close()
	select {
	case <-s.done:
	case <-time.After(time.Second):
	}
}

func (s *mockNoisyServer) serve() {
	defer close(s.done)
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *mockNoisyServer) handle(conn net.Conn) {
	defer conn.Close()
	preface, _, err := DecodePreface(conn, DefaultServerMaxPadding)
	if err != nil || !VerifyPrefaceHash(preface, s.password) {
		return
	}
	frame, err := ReadFrame(conn, MaxPayloadLength)
	if err != nil || frame.Type != FrameTypeClientHello {
		return
	}
	clientHello, err := DecodeHello(frame.Payload)
	if err != nil || ValidateClientHello(clientHello) != nil {
		return
	}
	serverHello := Hello{Version: ProtocolVersion, Capabilities: clientHello.Capabilities, MaxStreams: clientHello.MaxStreams}
	if err = WriteBufferedFrame(conn, FrameTypeServerHello, 0, 0, EncodeHello(serverHello)); err != nil {
		return
	}
	frame, err = ReadFrame(conn, MaxPayloadLength)
	if err != nil || frame.Type != FrameTypeOpenRequest {
		return
	}
	request, err := DecodeOpenRequest(frame.Payload)
	if err != nil || request.Command != CommandConnect {
		payload, _ := EncodeOpenResponse(OpenResponse{Status: ErrorUnsupportedCommand, Message: "bad request"})
		_ = WriteBufferedFrame(conn, FrameTypeOpenResponse, 0, frame.StreamID, payload)
		return
	}
	payload, _ := EncodeOpenResponse(OpenResponse{Status: ErrorOK})
	if err = WriteBufferedFrame(conn, FrameTypeOpenResponse, 0, frame.StreamID, payload); err != nil {
		return
	}
	for {
		frame, err = ReadFrame(conn, MaxPayloadLength)
		if err != nil {
			return
		}
		switch frame.Type {
		case FrameTypeData:
			if err = WriteBufferedFrame(conn, FrameTypeData, 0, frame.StreamID, frame.Payload); err != nil {
				return
			}
		case FrameTypeEndResponse, FrameTypeEndRequest:
			_ = WriteBufferedFrame(conn, FrameTypeEndResponse, 0, frame.StreamID, nil)
			return
		case FrameTypePing:
			_ = WriteBufferedFrame(conn, FrameTypePong, 0, 0, frame.Payload)
		default:
			_ = WriteBufferedFrame(conn, FrameTypeReset, 0, frame.StreamID, nil)
			return
		}
	}
}
