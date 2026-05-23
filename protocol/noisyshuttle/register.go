package noisyshuttle

import (
	"context"
	"encoding/hex"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/common/listener"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func RegisterInbound(registry *inbound.Registry) {
	inbound.Register(registry, C.TypeNoisyShuttle, NewInbound)
}

type replayFilter struct {
	mu       sync.Mutex
	entries  map[string]struct{}
	order    []string
	maxSize  int
}

func newReplayFilter(maxSize int) *replayFilter {
	if maxSize <= 0 {
		maxSize = 256
	}
	return &replayFilter{
		entries: make(map[string]struct{}),
		order:   make([]string, 0, maxSize),
		maxSize: maxSize,
	}
}

func (f *replayFilter) checkAndAdd(hash string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.entries[hash]; exists {
		return false
	}
	if len(f.entries) >= f.maxSize {
		oldest := f.order[0]
		delete(f.entries, oldest)
		f.order = f.order[1:]
	}
	f.entries[hash] = struct{}{}
	f.order = append(f.order, hash)
	return true
}

type Inbound struct {
	inbound.Adapter
	ctx               context.Context
	router            adapter.ConnectionRouterEx
	logger            log.StructuredLogger
	listener          *listener.Listener
	tlsConfig         tls.ServerConfig
	users             []option.NoisyShuttleUser
	fallbackAddr      M.Socksaddr
	maxPadding        int
	authTimeout       time.Duration
	sessionOpts       SessionOptions
	capabilities      uint16
	activeSessions    sync.WaitGroup
	activeSessionsMux sync.Mutex
	closing           bool
	natManager        *NATManager
	network           []string
	replayFilter      *replayFilter
}

func NewInbound(ctx context.Context, router adapter.Router, logger log.StructuredLogger, tag string, options option.NoisyShuttleInboundOptions) (adapter.Inbound, error) {
	maxPadding := int(options.Handshake.MaxPadding)
	if maxPadding == 0 {
		maxPadding = DefaultServerMaxPadding
	}
	authTimeout := time.Duration(options.Handshake.AuthTimeout)
	if authTimeout == 0 {
		authTimeout = 5 * time.Second
	}
	maxStreams := options.Session.MaxStreams
	if maxStreams == 0 {
		maxStreams = 1
	}
	sessionOpts := SessionOptions{
		MaxStreams:        maxStreams,
		MaxRequests:       options.Session.MaxRequests,
		IdleTimeout:       time.Duration(options.Session.IdleTimeout),
		MaxAge:            time.Duration(options.Session.MaxAge),
		KeepaliveInterval: time.Duration(options.Session.KeepaliveInterval),
		KeepaliveTimeout:  time.Duration(options.Session.KeepaliveTimeout),
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
	network := options.Network.Build()
	if len(network) == 0 {
		network = []string{N.NetworkTCP}
	}
	capabilities := uint16(CapabilityKeepalive)
	if options.Session.Enabled && options.Session.MaxStreams > 1 {
		capabilities |= CapabilityReuse
	}
	if containsUDP(network) {
		capabilities |= CapabilityUDPAssociate
	}
	inbound := &Inbound{
		Adapter:      inbound.NewAdapter(C.TypeNoisyShuttle, tag),
		ctx:          ctx,
		router:       router,
		logger:       logger,
		users:        options.Users,
		maxPadding:   maxPadding,
		authTimeout:  authTimeout,
		sessionOpts:  sessionOpts,
		capabilities: capabilities,
		network:      network,
		replayFilter: newReplayFilter(256),
	}
	natConfig := &NATManagerConfig{
		MaxMappings:   16,
		IdleTimeout:   time.Duration(options.UDPTimeout),
		MaxPacketSize: int(options.UDPMaxPacketSize),
	}
	if natConfig.IdleTimeout <= 0 {
		natConfig.IdleTimeout = 60 * time.Second
	}
	if natConfig.MaxPacketSize <= 0 {
		natConfig.MaxPacketSize = 1500
	}
	inbound.natManager = NewNATManager(natConfig)
	if options.TLS == nil {
		return nil, E.New("noisy-shuttle inbound: TLS is required")
	}
	tlsConfig, err := tls.NewServerWithOptions(tls.ServerOptions{
		Context:        ctx,
		Logger:         logger,
		Options:        common.PtrValueOrDefault(options.TLS),
		KTLSCompatible: true,
	})
	if err != nil {
		return nil, err
	}
	inbound.tlsConfig = tlsConfig
	if options.Fallback != nil && options.Fallback.Server != "" {
		fallbackAddr := options.Fallback.Build()
		if !fallbackAddr.IsValid() {
			return nil, E.New("invalid fallback address: ", fallbackAddr)
		}
		inbound.fallbackAddr = fallbackAddr
	}
	inbound.router = router
	inbound.listener = listener.New(listener.Options{
		Context:           ctx,
		Logger:            logger,
		Network:           []string{N.NetworkTCP},
		Listen:            options.ListenOptions,
		ConnectionHandler: inbound,
	})
	return inbound, nil
}

func (h *Inbound) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	if err := h.tlsConfig.Start(); err != nil {
		return E.Cause(err, "create TLS config")
	}
	return h.listener.Start()
}

func (h *Inbound) Close() error {
	h.activeSessionsMux.Lock()
	h.closing = true
	h.activeSessionsMux.Unlock()
	err := common.Close(h.listener, h.tlsConfig)
	h.activeSessions.Wait()
	return err
}

func (h *Inbound) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	h.activeSessionsMux.Lock()
	if h.closing {
		h.activeSessionsMux.Unlock()
		N.CloseOnHandshakeFailure(conn, onClose, os.ErrClosed)
		return
	}
	h.activeSessions.Add(1)
	h.activeSessionsMux.Unlock()
	defer h.activeSessions.Done()

	tlsConn, err := tls.ServerHandshake(ctx, conn, h.tlsConfig)
	if err != nil {
		N.CloseOnHandshakeFailure(conn, onClose, err)
		h.logger.ErrorEventContext(ctx, "protocol.message", F.ToString(E.Cause(err, "process connection from ", metadata.Source, ": TLS handshake")))
		return
	}
	conn = tlsConn
	if h.authTimeout > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(h.authTimeout))
	}
	passwordHash, _, err := DecodePreface(conn, h.maxPadding)
	if err != nil {
		h.handleAuthFailure(ctx, conn, metadata, onClose, err, "preface decode failed")
		return
	}
	hashHex := hex.EncodeToString(passwordHash[:])
	if !h.replayFilter.checkAndAdd(hashHex) {
		h.handleAuthFailure(ctx, conn, metadata, onClose, E.New("replay detected"), "replay attack detected")
		return
	}
	userIndex := h.matchUser(passwordHash)
	if userIndex < 0 {
		h.handleAuthFailure(ctx, conn, metadata, onClose, E.New("authentication failed"), "invalid credentials")
		return
	}
	if h.authTimeout > 0 {
		_ = conn.SetReadDeadline(time.Time{})
	}
	userName := h.users[userIndex].Name
	if userName == "" {
		userName = F.ToString(userIndex)
	}
	h.logger.InfoEventContext(ctx, "protocol.message", F.ToString("[", userName, "] session accepted from ", metadata.Source))
	ctx = auth.ContextWithUser(ctx, userIndex)
	if err := h.handleSession(ctx, conn, metadata, onClose, userIndex); err != nil {
		N.CloseOnHandshakeFailure(conn, onClose, err)
		if !E.IsClosedOrCanceled(err) && err != io.EOF {
			h.logger.ErrorEventContext(ctx, "protocol.message", F.ToString(E.Cause(err, "process connection from ", metadata.Source)))
		}
	}
}

func (h *Inbound) matchUser(received [64]byte) int {
	matched := -1
	for index, user := range h.users {
		if VerifyPrefaceHash(received, user.Password) {
			matched = index
		}
	}
	return matched
}

func (h *Inbound) handleAuthFailure(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc, cause error, reason string) {
	if h.authTimeout > 0 {
		_ = conn.SetReadDeadline(time.Time{})
	}
	h.logger.WarnEventContext(ctx, "protocol.message", F.ToString("authentication failed from ", metadata.Source, ": ", reason))
	if h.fallbackAddr.IsValid() {
		metadata.Inbound = h.Tag()
		metadata.InboundType = h.Type()
		metadata.Destination = h.fallbackAddr
		h.logger.InfoEventContext(ctx, "protocol.message", F.ToString("fallback connection to ", h.fallbackAddr))
		h.router.RouteConnectionEx(ctx, conn, metadata, onClose)
		return
	}
	N.CloseOnHandshakeFailure(conn, onClose, cause)
}

func (h *Inbound) handleSession(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc, userIndex int) error {
	frame, err := ReadFrame(conn, MaxPayloadLength)
	if err != nil {
		return err
	}
	if frame.Type != FrameTypeClientHello {
		return E.New("expected client hello, got frame type: ", frame.Type)
	}
	clientHello, err := DecodeHello(frame.Payload)
	if err != nil {
		return err
	}
	if err := ValidateClientHello(clientHello); err != nil {
		return err
	}
	agreed := clientHello.Capabilities & h.capabilities
	serverMaxStreams := h.sessionOpts.MaxStreams
	if clientHello.MaxStreams > 0 && clientHello.MaxStreams < serverMaxStreams {
		serverMaxStreams = clientHello.MaxStreams
	}
	if agreed&CapabilityReuse == 0 {
		serverMaxStreams = 1
	}
	if err := WriteBufferedFrame(conn, FrameTypeServerHello, 0, 0, EncodeHello(Hello{Version: ProtocolVersion, Capabilities: agreed, MaxStreams: serverMaxStreams})); err != nil {
		return err
	}
	sessionOpts := h.sessionOpts
	sessionOpts.MaxStreams = serverMaxStreams
	session := newInboundSession(conn, agreed, serverMaxStreams, sessionOpts, h.logger)
	session.startKeepalive()
	for {
		if !session.checkLifecycle() {
			if session.closeReason != "" {
				h.logger.DebugEventContext(ctx, "protocol.message", F.ToString("session closed: ", session.closeReason))
			}
			return nil
		}
		frame, err = readFrameWithSessionActivity(conn, MaxPayloadLength, session.markActivity)
		if err != nil {
			if isClosedOrEOF(err) {
				return nil
			}
			return err
		}
		switch frame.Type {
		case FrameTypeOpenRequest:
			ok, code := session.canAccept()
			if !ok {
				_ = session.writeFrame(FrameTypeClose, code, 0, []byte{code})
				session.close()
				return nil
			}
			if err := h.handleOpenRequest(ctx, session, metadata, onClose, frame, userIndex); err != nil {
				return err
			}
			if agreed&CapabilityReuse == 0 {
				return nil
			}
		case FrameTypePing:
			if err := session.writeFrame(FrameTypePong, 0, 0, frame.Payload); err != nil {
				return err
			}
		case FrameTypePong:
			continue
		case FrameTypeClose:
			return nil
		default:
			return E.New("unexpected frame before open request: ", frame.Type)
		}
	}
}

func (h *Inbound) handleOpenRequest(ctx context.Context, session *inboundSession, metadata adapter.InboundContext, onClose N.CloseHandlerFunc, frame Frame, userIndex int) error {
	request, err := DecodeOpenRequest(frame.Payload)
	if err != nil {
		return h.writeOpenError(session, frame.StreamID, ErrorInvalidAddress, err)
	}
	if request.Command == CommandUDPAssociate {
		if !h.isUDPEnabled() {
			return h.writeOpenError(session, frame.StreamID, ErrorUnsupportedCommand, E.New("UDP is not enabled"))
		}
		return h.handleUDPAssociate(ctx, session.conn, metadata, onClose, frame, userIndex)
	}
	if request.Command != CommandConnect {
		return h.writeOpenError(session, frame.StreamID, ErrorUnsupportedCommand, E.New("unsupported command: ", request.Command))
	}
	destination := M.ParseSocksaddrHostPort(request.Address.Host, request.Port)
	if !destination.IsValid() {
		return h.writeOpenError(session, frame.StreamID, ErrorInvalidAddress, E.New("invalid destination: ", request.Address.Host, ":", request.Port))
	}
	response, err := EncodeOpenResponse(OpenResponse{Status: ErrorOK})
	if err != nil {
		return err
	}
	if err := session.writeFrame(FrameTypeOpenResponse, 0, frame.StreamID, response); err != nil {
		return err
	}
	metadata.Inbound = h.Tag()
	metadata.InboundType = h.Type()
	metadata.Destination = destination
	if userIndex >= 0 && userIndex < len(h.users) {
		user := h.users[userIndex].Name
		if user == "" {
			user = F.ToString(userIndex)
		} else {
			metadata.User = user
		}
		h.logger.InfoEventContext(ctx, "protocol.message", F.ToString("[", user, "] inbound connection to ", metadata.Destination))
	} else {
		h.logger.InfoEventContext(ctx, "protocol.message", F.ToString("inbound connection to ", metadata.Destination))
	}
	streamConn := &streamConn{Conn: session.conn, streamID: frame.StreamID, firstPayload: nil, inbound: session}
	h.router.RouteConnectionEx(adapter.WithContext(ctx, &metadata), streamConn, metadata, onClose)
	return nil
}

func (h *Inbound) handleUDPAssociate(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc, frame Frame, userIndex int) error {
	if h.natManager.IsFull() {
		cause := E.New("max UDP associations reached")
		response, err := EncodeOpenResponse(OpenResponse{Status: ErrorMaxRequests, Message: cause.Error()})
		if err != nil {
			return err
		}
		if err := WriteBufferedFrame(conn, FrameTypeOpenResponse, 0, frame.StreamID, response); err != nil {
			return err
		}
		return cause
	}
	defaultDest := M.Socksaddr{}
	if request, err := DecodeOpenRequest(frame.Payload); err == nil && request.Port != 0 {
		defaultDest = M.ParseSocksaddrHostPort(request.Address.Host, request.Port)
	}
	response, err := EncodeOpenResponse(OpenResponse{Status: ErrorOK})
	if err != nil {
		return err
	}
	if err := WriteBufferedFrame(conn, FrameTypeOpenResponse, 0, frame.StreamID, response); err != nil {
		return err
	}
	userName := ""
	if userIndex >= 0 && userIndex < len(h.users) {
		userName = h.users[userIndex].Name
		if userName == "" {
			userName = F.ToString(userIndex)
		}
	}
	h.logger.InfoEventContext(ctx, "protocol.message", F.ToString("[", userName, "] UDP associate stream=", frame.StreamID))
	h.logger.DebugEventContext(ctx, "protocol.message", F.ToString("UDP mapping created for stream=", frame.StreamID))
	natEntry := &NATEntry{
		StreamID:     frame.StreamID,
		Destination:  defaultDest,
		LastActivity: time.Now(),
	}
	h.natManager.Put(frame.StreamID, natEntry)
	go h.handleUDPStream(ctx, conn, frame.StreamID, defaultDest, natEntry, onClose)
	return nil
}

func (h *Inbound) handleUDPStream(ctx context.Context, conn net.Conn, streamID uint32, defaultDest M.Socksaddr, natEntry *NATEntry, onClose N.CloseHandlerFunc) {
	defer func() {
		h.natManager.Remove(streamID)
		h.logger.DebugEventContext(ctx, "protocol.message", F.ToString("UDP mapping expired for stream=", streamID))
	}()
	for {
		frame, err := ReadFrame(conn, h.natManager.MaxPacketSize()+1024)
		if err != nil {
			return
		}
		if frame.StreamID != streamID {
			continue
		}
		h.natManager.TouchActivity(streamID)
		switch frame.Type {
		case FrameTypeData:
			addr, port, payload, err := DecodeUDPdatagram(frame.Payload)
			if err != nil {
				WriteBufferedFrame(conn, FrameTypeReset, 0, streamID, []byte{ErrorProtocol})
				return
			}
			if len(payload) > h.natManager.MaxPacketSize() {
				WriteBufferedFrame(conn, FrameTypeReset, 0, streamID, []byte{ErrorPayloadTooLarge})
				return
			}
			destination := defaultDest
			if port != 0 {
				destination = M.ParseSocksaddrHostPort(addr.Host, port)
			}
			h.logger.DebugEventContext(ctx, "protocol.message", F.ToString("UDP packet to ", destination))
		case FrameTypeEndRequest, FrameTypeEndResponse:
			return
		case FrameTypeReset:
			return
		case FrameTypePing:
			WriteBufferedFrame(conn, FrameTypePong, 0, 0, frame.Payload)
		default:
			WriteBufferedFrame(conn, FrameTypeReset, 0, streamID, []byte{ErrorUnknownFrame})
			return
		}
	}
}

func (h *Inbound) isUDPEnabled() bool {
	return (h.capabilities & CapabilityUDPAssociate) != 0
}

func (h *Inbound) writeOpenError(session *inboundSession, streamID uint32, status byte, cause error) error {
	response, err := EncodeOpenResponse(OpenResponse{Status: status, Message: cause.Error()})
	if err != nil {
		return err
	}
	if err := session.writeFrame(FrameTypeOpenResponse, 0, streamID, response); err != nil {
		return err
	}
	return cause
}
