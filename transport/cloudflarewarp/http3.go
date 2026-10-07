//go:build with_quic

package cloudflarewarp

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/http3"
	"github.com/sagernet/quic-go/quicvarint"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-quic"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

const (
	connectProtocol           = "cf-connect-ip"
	connectURL                = "https://cloudflareaccess.com/"
	settingH3Datagram00       = 0x276
	defaultKeepAlivePeriod    = 30 * time.Second
	defaultConnectionIDLength = 20
	settingsWaitTimeout       = 3 * time.Second
)

type HTTP3Options struct {
	Dialer     N.Dialer
	ServerAddr M.Socksaddr
	// TLSConfig is called for every connection so each gets a fresh client
	// certificate.
	TLSConfig      func() (*tls.Config, error)
	QUIC           qtls.QUICOptions
	ConnectTimeout time.Duration
}

type http3Transport struct {
	options HTTP3Options
}

func NewHTTP3Transport(options HTTP3Options) Transport {
	if options.ConnectTimeout == 0 {
		options.ConnectTimeout = C.TCPTimeout
	}
	return &http3Transport{options: options}
}

func (t *http3Transport) Connect(ctx context.Context) (Session, error) {
	ctx, cancel := context.WithTimeout(ctx, t.options.ConnectTimeout)
	defer cancel()
	tlsConfig, err := t.options.TLSConfig()
	if err != nil {
		return nil, err
	}
	udpConn, err := t.options.Dialer.DialContext(ctx, N.NetworkUDP, t.options.ServerAddr)
	if err != nil {
		return nil, E.Cause(err, "dial UDP")
	}
	qtls.SetDesiredBufferSizes(udpConn)
	packetConn := bufio.NewUnbindPacketConn(udpConn)
	transport := &quic.Transport{
		Conn: packetConn,
		// Cloudflare occasionally closes connections with PROTOCOL_VIOLATION
		// when the client uses zero-length connection IDs.
		ConnectionIDLength: defaultConnectionIDLength,
	}
	transport.SetSingleUse(true)
	transport.SetCreatedConn(true)
	quicConfig := &quic.Config{
		EnableDatagrams: true,
		KeepAlivePeriod: defaultKeepAlivePeriod,
	}
	qtls.ApplyQUICOptions(quicConfig, t.options.QUIC)
	var remoteAddr net.Addr = udpConn.RemoteAddr()
	if remoteAddr == nil || t.options.ServerAddr.IsIP() {
		remoteAddr = t.options.ServerAddr.UDPAddr()
	}
	quicConn, err := transport.Dial(ctx, remoteAddr, tlsConfig, quicConfig)
	if err != nil {
		transport.Close()
		udpConn.Close()
		return nil, wrapHandshakeError(err)
	}
	session, err := openSession(ctx, quicConn)
	if err != nil {
		quicConn.CloseWithError(0, "")
		transport.Close()
		udpConn.Close()
		return nil, err
	}
	go func() {
		<-quicConn.Context().Done()
		transport.Close()
		udpConn.Close()
	}()
	return session, nil
}

func wrapHandshakeError(err error) error {
	if strings.Contains(err.Error(), "tls: access denied") {
		return E.Cause(ErrEnrollmentRejected, err.Error())
	}
	return E.Cause(qtls.WrapError(err), "QUIC handshake")
}

func openSession(ctx context.Context, quicConn *quic.Conn) (*http3Session, error) {
	h3Transport := &http3.Transport{
		EnableDatagrams: true,
		// The deprecated SETTINGS_H3_DATAGRAM draft identifier, which the
		// official client still sends.
		AdditionalSettings: map[uint64]uint64{settingH3Datagram00: 1},
		DisableCompression: true,
	}
	clientConn := h3Transport.NewClientConn(quicConn)
	stream, err := clientConn.OpenRequestStream(ctx)
	if err != nil {
		return nil, E.Cause(err, "open request stream")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodConnect, connectURL, nil)
	if err != nil {
		return nil, err
	}
	request.Proto = connectProtocol
	request.Header.Set(http3.CapsuleProtocolHeader, "?1")
	request.Header.Set("User-Agent", "")
	err = stream.SendRequestHeader(request)
	if err != nil {
		return nil, E.Cause(err, "send CONNECT-IP request")
	}
	if deadline, loaded := ctx.Deadline(); loaded {
		stream.SetReadDeadline(deadline)
	}
	stopCancel := context.AfterFunc(ctx, func() {
		stream.CancelRead(quic.StreamErrorCode(http3.ErrCodeRequestCanceled))
	})
	response, err := stream.ReadResponse()
	stopCancel()
	if err != nil {
		return nil, E.Cause(err, "read CONNECT-IP response")
	}
	if response.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		stream.CancelRead(quic.StreamErrorCode(http3.ErrCodeNoError))
		stream.CancelWrite(quic.StreamErrorCode(http3.ErrCodeNoError))
		message := strings.TrimSpace(string(body))
		if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden ||
			strings.Contains(message, "tls: access denied") {
			return nil, E.Cause(ErrEnrollmentRejected, "CONNECT-IP status ", response.Status)
		}
		return nil, E.New("CONNECT-IP rejected: ", response.Status)
	}
	stream.SetReadDeadline(time.Time{})
	select {
	case <-clientConn.ReceivedSettings():
		if !clientConn.Settings().EnableDatagrams {
			stream.CancelRead(quic.StreamErrorCode(http3.ErrCodeNoError))
			return nil, E.New("server did not enable HTTP/3 datagrams")
		}
	case <-time.After(settingsWaitTimeout):
	case <-quicConn.Context().Done():
		return nil, E.Cause(context.Cause(quicConn.Context()), "connection closed")
	}
	sessionCtx, sessionCancel := context.WithCancelCause(context.Background())
	session := &http3Session{
		quicConn: quicConn,
		stream:   stream,
		ctx:      sessionCtx,
		cancel:   sessionCancel,
	}
	go session.drainCapsules()
	go func() {
		select {
		case <-quicConn.Context().Done():
			session.closeWithError(E.Cause(qtls.WrapError(context.Cause(quicConn.Context())), "connection closed"))
		case <-sessionCtx.Done():
		}
	}()
	return session, nil
}

type http3Session struct {
	quicConn  *quic.Conn
	stream    *http3.RequestStream
	ctx       context.Context
	cancel    context.CancelCauseFunc
	closeOnce sync.Once
}

func (s *http3Session) drainCapsules() {
	parser := http3.NewCapsuleParser(s.stream)
	for {
		_, reader, err := parser.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = E.New("server closed the tunnel stream")
			}
			s.closeWithError(err)
			return
		}
		// Cloudflare sends no capsules; ignore any that arrive.
		err = reader.Discard()
		if err != nil {
			s.closeWithError(err)
			return
		}
	}
}

func (s *http3Session) WritePacket(buffer *buf.Buffer) error {
	if s.ctx.Err() != nil {
		return s.Err()
	}
	buffer.ExtendHeader(PacketHeadroom)[0] = 0
	err := s.stream.SendDatagram(buffer.Bytes())
	buffer.Advance(PacketHeadroom)
	if err != nil {
		var tooLarge *quic.DatagramTooLargeError
		if errors.As(err, &tooLarge) {
			return &PacketTooLargeError{MaxPacketSize: int(tooLarge.MaxDatagramPayloadSize) - maxDatagramOverhead}
		}
		return err
	}
	return nil
}

// maxDatagramOverhead covers the quarter stream ID (at most 8 bytes, a single
// byte for the first request stream) and the context ID.
const maxDatagramOverhead = 2

func (s *http3Session) ReadPacket() ([]byte, error) {
	for {
		datagram, err := s.stream.ReceiveDatagram(s.ctx)
		if err != nil {
			if s.ctx.Err() != nil {
				return nil, s.Err()
			}
			return nil, err
		}
		contextID, n, err := quicvarint.Parse(datagram)
		if err != nil || contextID != 0 {
			continue
		}
		return datagram[n:], nil
	}
}

func (s *http3Session) Done() <-chan struct{} {
	return s.ctx.Done()
}

func (s *http3Session) Err() error {
	return context.Cause(s.ctx)
}

func (s *http3Session) closeWithError(err error) {
	s.closeOnce.Do(func() {
		s.cancel(err)
		s.stream.CancelRead(quic.StreamErrorCode(http3.ErrCodeNoError))
		s.stream.CancelWrite(quic.StreamErrorCode(http3.ErrCodeNoError))
		s.quicConn.CloseWithError(0, "")
	})
}

func (s *http3Session) Close() error {
	s.closeWithError(ErrSessionClosed)
	return nil
}
