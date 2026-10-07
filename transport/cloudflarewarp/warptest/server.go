//go:build with_quic

// Package warptest provides a fake Cloudflare WARP MASQUE server for tests.
package warptest

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"math/big"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/http3"
	"github.com/sagernet/quic-go/quicvarint"
	E "github.com/sagernet/sing/common/exceptions"
)

type Options struct {
	// ClientPublicKey, when set, must match the client certificate key.
	ClientPublicKey *ecdsa.PublicKey
	// Status is the CONNECT-IP response status; zero means 200.
	Status int
	Body   string
}

type Server struct {
	PublicKey *ecdsa.PublicKey
	Sessions  chan *Session

	options   Options
	udpConn   *net.UDPConn
	transport *quic.Transport
	listener  *quic.EarlyListener
	h3Server  *http3.Server
	access    sync.Mutex
	requests  []*http.Request
}

type Session struct {
	Request *http.Request
	stream  *http3.Stream
	done    chan struct{}
	once    sync.Once
}

func NewServer(options Options) (*Server, error) {
	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		DNSNames:     []string{"consumer-masque.cloudflareclient.com"},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &serverKey.PublicKey, serverKey)
	if err != nil {
		return nil, err
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: serverKey}},
		NextProtos:   []string{http3.NextProtoH3},
		ClientAuth:   tls.RequireAnyClientCert,
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if options.ClientPublicKey == nil {
				return nil
			}
			leaf, err := x509.ParseCertificate(rawCerts[0])
			if err != nil {
				return err
			}
			publicKey, isECDSA := leaf.PublicKey.(*ecdsa.PublicKey)
			if !isECDSA || !publicKey.Equal(options.ClientPublicKey) {
				return E.New("unknown client key")
			}
			return nil
		},
	}
	udpConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		return nil, err
	}
	transport := &quic.Transport{Conn: udpConn}
	listener, err := transport.ListenEarly(http3.ConfigureTLSConfig(tlsConfig), &quic.Config{EnableDatagrams: true})
	if err != nil {
		udpConn.Close()
		return nil, err
	}
	server := &Server{
		PublicKey: &serverKey.PublicKey,
		Sessions:  make(chan *Session, 16),
		options:   options,
		udpConn:   udpConn,
		transport: transport,
		listener:  listener,
	}
	server.h3Server = &http3.Server{
		EnableDatagrams: true,
		Handler:         http.HandlerFunc(server.handle),
	}
	go server.h3Server.ServeListener(listener)
	return server, nil
}

func (s *Server) Addr() *net.UDPAddr {
	return s.udpConn.LocalAddr().(*net.UDPAddr)
}

func (s *Server) Requests() []*http.Request {
	s.access.Lock()
	defer s.access.Unlock()
	return append([]*http.Request(nil), s.requests...)
}

func (s *Server) handle(writer http.ResponseWriter, request *http.Request) {
	s.access.Lock()
	s.requests = append(s.requests, request)
	s.access.Unlock()
	if request.Method != http.MethodConnect || request.Proto != "cf-connect-ip" {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	if s.options.Status != 0 && s.options.Status != http.StatusOK {
		writer.WriteHeader(s.options.Status)
		writer.Write([]byte(s.options.Body))
		return
	}
	writer.WriteHeader(http.StatusOK)
	stream := writer.(http3.HTTPStreamer).HTTPStream()
	session := &Session{Request: request, stream: stream, done: make(chan struct{})}
	s.Sessions <- session
	go func() {
		// Drain the request body so a client close ends the session.
		buffer := make([]byte, 1024)
		for {
			_, err := stream.Read(buffer)
			if err != nil {
				session.Close()
				return
			}
		}
	}()
	<-session.done
}

// ReadPacket returns the next IP packet with context ID 0, skipping others.
func (s *Session) ReadPacket(ctx context.Context) ([]byte, error) {
	for {
		datagram, err := s.stream.ReceiveDatagram(ctx)
		if err != nil {
			return nil, err
		}
		contextID, n, err := quicvarint.Parse(datagram)
		if err != nil {
			return nil, err
		}
		if contextID == 0 {
			return datagram[n:], nil
		}
	}
}

func (s *Session) WritePacket(packet []byte) error {
	return s.WriteDatagram(0, packet)
}

func (s *Session) WriteDatagram(contextID uint64, payload []byte) error {
	return s.stream.SendDatagram(append(quicvarint.Append(nil, contextID), payload...))
}

// Close ends the CONNECT-IP stream from the server side.
func (s *Session) Close() {
	s.once.Do(func() {
		s.stream.Close()
		s.stream.CancelRead(quic.StreamErrorCode(http3.ErrCodeNoError))
		close(s.done)
	})
}

func (s *Server) Close() error {
	s.h3Server.Close()
	s.listener.Close()
	s.transport.Close()
	return s.udpConn.Close()
}
