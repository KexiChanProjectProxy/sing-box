package noisyshuttle

import (
	"context"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func RegisterOutbound(registry *outbound.Registry) {
	outbound.Register(registry, C.TypeNoisyShuttle, NewOutbound)
}

type Outbound struct {
	outbound.Adapter
	dialer         N.Dialer
	serverAddr     M.Socksaddr
	logger         log.StructuredLogger
	tlsConfig      tls.Config
	tlsDialer      tls.Dialer
	network        []string
	sessionOpts    SessionOptions
	handshakeOpts  HandshakeOptions
	capabilities   uint16
	nextStreamID   uint32
	streamIDMux    sync.Mutex
	password       string
	sessionEnabled bool
	sessionPool    *SessionPool
}

type SessionOptions struct {
	MaxStreams        uint16
	MaxRequests       uint32
	IdleTimeout       time.Duration
	MaxAge            time.Duration
	KeepaliveInterval time.Duration
	KeepaliveTimeout  time.Duration
}

type HandshakeOptions struct {
	PaddingMin  uint16
	PaddingMax  uint16
	AuthTimeout time.Duration
}

func NewOutbound(ctx context.Context, router adapter.Router, logger log.StructuredLogger, tag string, options option.NoisyShuttleOutboundOptions) (adapter.Outbound, error) {
	dialer, err := dialer.New(ctx, options.DialerOptions, options.ServerIsDomain())
	if err != nil {
		return nil, err
	}
	capabilities := uint16(CapabilityKeepalive)
	if options.Session.Enabled {
		capabilities |= CapabilityReuse
	}
	networkList := options.Network
	if networkList == "" {
		networkList = "tcp"
	}
	networkSlice := networkList.Build()
	if len(networkSlice) == 0 {
		networkSlice = []string{N.NetworkTCP}
	}
	if containsUDP(networkSlice) {
		capabilities |= CapabilityUDPAssociate
	}
	handshakeOpts := HandshakeOptions{
		PaddingMin:  options.Handshake.PaddingMin,
		PaddingMax:  options.Handshake.PaddingMax,
		AuthTimeout: time.Duration(options.Handshake.AuthTimeout),
	}
	if handshakeOpts.PaddingMin == 0 {
		handshakeOpts.PaddingMin = DefaultClientMinPadding
	}
	if handshakeOpts.PaddingMax == 0 {
		handshakeOpts.PaddingMax = DefaultClientMaxPadding
	}
	if handshakeOpts.PaddingMin > handshakeOpts.PaddingMax {
		return nil, E.New("noisy-shuttle outbound: handshake padding_min must be <= padding_max")
	}
	if handshakeOpts.AuthTimeout == 0 {
		handshakeOpts.AuthTimeout = 5 * time.Second
	}
	sessionOpts := SessionOptions{
		MaxStreams:        options.Session.MaxStreams,
		MaxRequests:       options.Session.MaxRequests,
		IdleTimeout:       time.Duration(options.Session.IdleTimeout),
		MaxAge:            time.Duration(options.Session.MaxAge),
		KeepaliveInterval: time.Duration(options.Session.KeepaliveInterval),
		KeepaliveTimeout:  time.Duration(options.Session.KeepaliveTimeout),
	}
	if sessionOpts.MaxStreams == 0 {
		sessionOpts.MaxStreams = 16
	}
	if sessionOpts.IdleTimeout == 0 {
		sessionOpts.IdleTimeout = 5 * time.Minute
	}
	if sessionOpts.KeepaliveInterval == 0 {
		sessionOpts.KeepaliveInterval = 30 * time.Second
	}
	if sessionOpts.KeepaliveTimeout == 0 {
		sessionOpts.KeepaliveTimeout = 2 * sessionOpts.KeepaliveInterval
	}
	outbound := &Outbound{
		Adapter:        outbound.NewAdapterWithDialerOptions(C.TypeNoisyShuttle, tag, networkSlice, options.DialerOptions),
		dialer:         dialer,
		serverAddr:     options.ServerOptions.Build(),
		logger:         logger,
		network:        networkSlice,
		capabilities:   capabilities,
		sessionOpts:    sessionOpts,
		handshakeOpts:  handshakeOpts,
		nextStreamID:   1,
		password:       options.Password,
		sessionEnabled: options.Session.Enabled,
	}
	if outbound.sessionEnabled {
		outbound.sessionPool = NewSessionPool(outbound, logger)
	}
	if options.TLS != nil {
		outbound.tlsConfig, err = tls.NewClientWithOptions(tls.ClientOptions{
			Context:       ctx,
			Logger:        logger,
			ServerAddress: options.Server,
			Options:       common.PtrValueOrDefault(options.TLS),
		})
		if err != nil {
			return nil, err
		}
		outbound.tlsDialer = tls.NewDialer(outbound.dialer, outbound.tlsConfig)
	}
	return outbound, nil
}

func containsUDP(network []string) bool {
	for _, networkName := range network {
		if networkName == N.NetworkUDP {
			return true
		}
	}
	return false
}

func (h *Outbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if N.NetworkName(network) != N.NetworkTCP {
		return nil, E.Extend(N.ErrUnknownNetwork, network)
	}
	conn, err := h.openStream(ctx)
	if err != nil {
		return nil, err
	}
	requestData, err := EncodeOpenRequest(OpenRequest{Command: CommandConnect, Address: Address{Host: destination.AddrString()}, Port: destination.Port})
	if err != nil {
		conn.Close()
		return nil, err
	}
	if err := conn.writeFrame(FrameTypeOpenRequest, 0, conn.streamID, requestData); err != nil {
		conn.Close()
		return nil, err
	}
	frame, err := conn.readFrame()
	if err != nil {
		conn.Close()
		return nil, err
	}
	if frame.Type == FrameTypePing {
		_ = conn.writeFrame(FrameTypePong, 0, 0, frame.Payload)
		frame, err = conn.readFrame()
		if err != nil {
			conn.Close()
			return nil, err
		}
	}
	if frame.Type != FrameTypeOpenResponse {
		conn.Close()
		return nil, E.New("expected open response, got frame type: ", frame.Type)
	}
	response, err := DecodeOpenResponse(frame.Payload)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if response.Status != ErrorOK {
		conn.Close()
		return nil, E.New("connect failed: ", response.Message)
	}
	adapter.LogOutboundConnection(h.logger, ctx, destination)
	return conn, nil
}

func (h *Outbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	if !containsUDP(h.network) {
		return nil, E.New("UDP is not enabled")
	}
	conn, streamID, err := h.openUDPAssociation(ctx, destination)
	if err != nil {
		return nil, err
	}
	return &udpOutboundPacketConn{
		conn:        conn,
		streamID:    streamID,
		destination: destination,
		logger:      h.logger,
	}, nil
}

func (h *Outbound) openStream(ctx context.Context) (*streamConn, error) {
	if h.sessionPool != nil {
		return h.sessionPool.OpenStream(ctx)
	}
	return h.dialTunnel(ctx)
}

func (h *Outbound) dialTunnel(ctx context.Context) (*streamConn, error) {
	session, err := h.dialSession(ctx)
	if err != nil {
		return nil, err
	}
	streamID := h.allocateStreamID()
	return session.open(streamID), nil
}

func (h *Outbound) dialSession(ctx context.Context) (*clientSession, error) {
	var conn net.Conn
	var err error
	if h.tlsDialer != nil {
		conn, err = h.tlsDialer.DialTLSContext(ctx, h.serverAddr)
	} else {
		conn, err = h.dialer.DialContext(ctx, N.NetworkTCP, h.serverAddr)
	}
	if err != nil {
		return nil, err
	}
	paddingLen := int(h.handshakeOpts.PaddingMax)
	if h.handshakeOpts.PaddingMax > h.handshakeOpts.PaddingMin {
		paddingLen = int(h.handshakeOpts.PaddingMin) + rand.Intn(int(h.handshakeOpts.PaddingMax-h.handshakeOpts.PaddingMin)+1)
	}
	if err = EncodePreface(conn, h.password, paddingLen); err != nil {
		conn.Close()
		return nil, err
	}
	if h.handshakeOpts.AuthTimeout > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(h.handshakeOpts.AuthTimeout))
	}
	clientHello := EncodeHello(Hello{
		Version:      ProtocolVersion,
		Capabilities: h.capabilities,
		MaxStreams:   h.sessionOpts.MaxStreams,
	})
	if err := WriteBufferedFrame(conn, FrameTypeClientHello, 0, 0, clientHello); err != nil {
		_ = conn.SetReadDeadline(time.Time{})
		conn.Close()
		return nil, err
	}
	frame, err := ReadFrame(conn, MaxPayloadLength)
	_ = conn.SetReadDeadline(time.Time{})
	if err != nil {
		conn.Close()
		return nil, err
	}
	if frame.Type != FrameTypeServerHello {
		conn.Close()
		return nil, E.New("expected server hello, got frame type: ", frame.Type)
	}
	serverHello, err := DecodeHello(frame.Payload)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if serverHello.Version != ProtocolVersion {
		conn.Close()
		return nil, E.New("protocol version mismatch: expected ", ProtocolVersion, " got ", serverHello.Version)
	}
	if serverHello.Capabilities&^h.capabilities != 0 {
		conn.Close()
		return nil, E.New("server advertised unsupported capabilities: ", serverHello.Capabilities&^h.capabilities)
	}
	maxStreams := serverHello.MaxStreams
	if serverHello.Capabilities&CapabilityReuse == 0 || maxStreams == 0 {
		maxStreams = 1
	}
	session := newClientSession(conn, serverHello.Capabilities, maxStreams, h.sessionOpts)
	return session, nil
}

func (h *Outbound) openUDPAssociation(ctx context.Context, destination M.Socksaddr) (*streamConn, uint32, error) {
	conn, err := h.openStream(ctx)
	if err != nil {
		return nil, 0, err
	}
	streamID := conn.streamID
	requestData, err := EncodeOpenRequest(OpenRequest{
		Command: CommandUDPAssociate,
		Address: Address{
			Type: AddressTypeIPv4,
			Host: "0.0.0.0",
		},
		Port: 0,
	})
	if err != nil {
		conn.Close()
		return nil, 0, err
	}
	if err := conn.writeFrame(FrameTypeOpenRequest, 0, streamID, requestData); err != nil {
		conn.Close()
		return nil, 0, err
	}
	frame, err := conn.readFrame()
	if err != nil {
		conn.Close()
		return nil, 0, err
	}
	if frame.Type != FrameTypeOpenResponse {
		conn.Close()
		return nil, 0, E.New("expected open response, got frame type: ", frame.Type)
	}
	response, err := DecodeOpenResponse(frame.Payload)
	if err != nil {
		conn.Close()
		return nil, 0, err
	}
	if response.Status != ErrorOK {
		conn.Close()
		return nil, 0, E.New("UDP association failed: ", response.Message)
	}
	return conn, streamID, nil
}

func (h *Outbound) allocateStreamID() uint32 {
	h.streamIDMux.Lock()
	defer h.streamIDMux.Unlock()
	id := h.nextStreamID
	h.nextStreamID++
	if h.nextStreamID == 0 {
		h.nextStreamID = 1
	}
	return id
}

func (h *Outbound) Close() error {
	return nil
}

func (h *Outbound) InterfaceUpdated() {}

type udpOutboundPacketConn struct {
	conn        *streamConn
	streamID    uint32
	destination M.Socksaddr
	logger      log.StructuredLogger
	readMux     sync.Mutex
	writeMux    sync.Mutex
}

func (c *udpOutboundPacketConn) ReadFrom(p []byte) (int, net.Addr, error) {
	c.readMux.Lock()
	defer c.readMux.Unlock()
	for {
		frame, err := ReadFrame(c.conn.Conn, MaxPayloadLength)
		if err != nil {
			return 0, nil, err
		}
		if frame.StreamID != c.streamID {
			continue
		}
		switch frame.Type {
		case FrameTypeData:
			addr, port, payload, err := DecodeUDPdatagram(frame.Payload)
			if err != nil {
				return 0, nil, err
			}
			dest := M.ParseSocksaddrHostPort(addr.Host, port)
			n := copy(p, payload)
			return n, dest.UDPAddr(), nil
		case FrameTypeReset:
			return 0, nil, E.New("stream reset")
		case FrameTypeEndRequest, FrameTypeEndResponse:
			return 0, nil, E.New("connection closed")
		default:
			continue
		}
	}
}

func (c *udpOutboundPacketConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	c.writeMux.Lock()
	defer c.writeMux.Unlock()
	var destination M.Socksaddr
	if udpAddr, ok := addr.(*net.UDPAddr); ok {
		destination = M.SocksaddrFromNet(udpAddr)
	} else {
		destination = M.ParseSocksaddr(addr.String())
	}
	datagram, err := EncodeUDPdatagram(Address{
		Type: AddressTypeIPv4,
		Host: destination.Addr.String(),
	}, destination.Port, p)
	if err != nil {
		return 0, err
	}
	if len(datagram) > MaxPayloadLength {
		return 0, E.New("UDP packet too large: ", len(datagram))
	}
	err = WriteBufferedFrame(c.conn.Conn, FrameTypeData, 0, c.streamID, datagram)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *udpOutboundPacketConn) Close() error {
	WriteBufferedFrame(c.conn.Conn, FrameTypeEndRequest, 0, c.streamID, nil)
	return c.conn.Close()
}

func (c *udpOutboundPacketConn) LocalAddr() net.Addr {
	return c.conn.Conn.LocalAddr()
}

func (c *udpOutboundPacketConn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *udpOutboundPacketConn) SetReadDeadline(t time.Time) error {
	return c.conn.Conn.SetReadDeadline(t)
}

func (c *udpOutboundPacketConn) SetWriteDeadline(t time.Time) error {
	return c.conn.Conn.SetWriteDeadline(t)
}

var _ net.PacketConn = (*udpOutboundPacketConn)(nil)
