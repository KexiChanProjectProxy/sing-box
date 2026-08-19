package noisyshuttle

import (
	"context"
	"io"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"
)

type SessionPool struct {
	outbound *Outbound
	logger   log.StructuredLogger
	mu       sync.Mutex
	session  *clientSession
}

func NewSessionPool(outbound *Outbound, logger log.StructuredLogger) *SessionPool {
	return &SessionPool{outbound: outbound, logger: logger}
}

func (p *SessionPool) OpenStream(ctx context.Context) (*streamConn, error) {
	for {
		p.mu.Lock()
		session := p.session
		if session != nil && !session.canOpen() {
			p.session = nil
			session.close()
			session = nil
		}
		if session != nil && session.tryAcquire() {
			streamID := p.outbound.allocateStreamID()
			p.mu.Unlock()
			p.logger.DebugEventContext(ctx, "noisyshuttle.session.hit", "session reuse hit")
			return session.open(streamID), nil
		}
		p.mu.Unlock()

		newSession, err := p.outbound.dialSession(ctx)
		if err != nil {
			return nil, err
		}
		p.mu.Lock()
		if p.session == nil || !p.session.canOpen() {
			p.session = newSession
			newSession.pool = p
			if newSession.keepaliveEnabled() || newSession.hasLifecycleTimers() {
				newSession.startBackground()
			}
			if newSession.tryAcquire() {
				streamID := p.outbound.allocateStreamID()
				p.mu.Unlock()
				p.logger.DebugEventContext(ctx, "noisyshuttle.session.miss", "session reuse miss")
				return newSession.open(streamID), nil
			}
		}
		p.mu.Unlock()
		newSession.close()
	}
}

func (p *SessionPool) remove(session *clientSession) {
	p.mu.Lock()
	if p.session == session {
		p.session = nil
	}
	p.mu.Unlock()
}

type clientSession struct {
	conn         net.Conn
	capabilities uint16
	maxStreams   uint16
	maxRequests  uint32
	idleTimeout  time.Duration
	maxAge       time.Duration
	keepalive    time.Duration
	createdAt    time.Time

	pool *SessionPool

	mu           sync.Mutex
	writeMu      sync.Mutex
	active       bool
	closed       bool
	streamCount  uint16
	requestCount uint32
	lastActivity time.Time
	counter      uint32
}

func newClientSession(conn net.Conn, capabilities uint16, maxStreams uint16, opts SessionOptions) *clientSession {
	now := time.Now()
	return &clientSession{
		conn:         conn,
		capabilities: capabilities,
		maxStreams:   maxStreams,
		maxRequests:  opts.MaxRequests,
		idleTimeout:  opts.IdleTimeout,
		maxAge:       opts.MaxAge,
		keepalive:    opts.KeepaliveInterval,
		createdAt:    now,
		lastActivity: now,
	}
}

func (s *clientSession) canOpen() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.canOpenLocked(time.Now())
}

func (s *clientSession) canOpenLocked(now time.Time) bool {
	if s.closed || s.active {
		return false
	}
	if s.capabilities&CapabilityReuse == 0 && s.streamCount > 0 {
		return false
	}
	if s.streamCount >= s.maxStreams {
		return false
	}
	if s.maxRequests > 0 && s.requestCount >= s.maxRequests {
		return false
	}
	if s.maxAge > 0 && now.Sub(s.createdAt) >= s.maxAge {
		return false
	}
	if s.idleTimeout > 0 && now.Sub(s.lastActivity) >= s.idleTimeout {
		return false
	}
	return true
}

func (s *clientSession) tryAcquire() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if !s.canOpenLocked(now) {
		return false
	}
	s.active = true
	s.streamCount++
	s.requestCount++
	s.lastActivity = now
	return true
}

func (s *clientSession) open(streamID uint32) *streamConn {
	return &streamConn{Conn: s.conn, streamID: streamID, session: s}
}

func (s *clientSession) release(reusable bool) {
	s.mu.Lock()
	s.active = false
	s.lastActivity = time.Now()
	shouldClose := !reusable || !s.canOpenLocked(s.lastActivity)
	s.mu.Unlock()
	if shouldClose {
		s.close()
	}
}

func (s *clientSession) markActivity() {
	s.mu.Lock()
	s.lastActivity = time.Now()
	s.mu.Unlock()
}

func (s *clientSession) writeFrame(frameType byte, flags byte, streamID uint32, payload []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.markActivity()
	return WriteBufferedFrame(s.conn, frameType, flags, streamID, payload)
}

func (s *clientSession) close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.mu.Unlock()
	if s.pool != nil {
		s.pool.remove(s)
	}
	_ = s.conn.Close()
}

func (s *clientSession) keepaliveEnabled() bool {
	return s.capabilities&CapabilityKeepalive != 0 && s.keepalive > 0
}

func (s *clientSession) hasLifecycleTimers() bool {
	return s.maxAge > 0 || s.idleTimeout > 0
}

func (s *clientSession) startBackground() {
	go s.backgroundLoop()
}

func (s *clientSession) backgroundLoop() {
	interval := s.keepalive
	if interval <= 0 {
		interval = time.Second
	}
	if s.maxAge > 0 && s.maxAge < interval {
		interval = s.maxAge
	}
	if s.idleTimeout > 0 && s.idleTimeout < interval {
		interval = s.idleTimeout
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		if !s.checkLifecycle() {
			return
		}
		if s.keepaliveEnabled() && !s.sendKeepalive() {
			return
		}
	}
}

func (s *clientSession) checkLifecycle() bool {
	now := time.Now()
	s.mu.Lock()
	closed := s.closed
	tooOld := s.maxAge > 0 && now.Sub(s.createdAt) >= s.maxAge
	idle := !s.active && s.idleTimeout > 0 && now.Sub(s.lastActivity) >= s.idleTimeout
	s.mu.Unlock()
	if closed {
		return false
	}
	if tooOld || idle {
		s.close()
		return false
	}
	return true
}

func (s *clientSession) sendKeepalive() bool {
	s.mu.Lock()
	if s.closed || s.active {
		s.mu.Unlock()
		return !s.closed
	}
	counter := s.counter + 1
	s.counter = counter
	s.mu.Unlock()
	ping := Ping{Timestamp: uint64(time.Now().UnixMilli()), Counter: counter, Token: rand.Uint32()}
	if err := s.writeFrame(FrameTypePing, 0, 0, EncodePing(ping)); err != nil {
		s.close()
		return false
	}
	_ = s.conn.SetReadDeadline(time.Now().Add(s.keepalive * 2))
	frame, err := ReadFrame(s.conn, MaxPayloadLength)
	_ = s.conn.SetReadDeadline(time.Time{})
	if err != nil || frame.Type != FrameTypePong {
		s.close()
		return false
	}
	pong, err := DecodePong(frame.Payload)
	if err != nil || ValidatePong(ping, pong) != nil {
		s.close()
		return false
	}
	s.markActivity()
	return true
}

type inboundSession struct {
	conn         net.Conn
	capabilities uint16
	maxStreams   uint16
	maxRequests  uint32
	idleTimeout  time.Duration
	maxAge       time.Duration
	keepalive    time.Duration
	createdAt    time.Time
	logger       log.StructuredLogger

	mu           sync.Mutex
	writeMu      sync.Mutex
	active       int32
	closed       bool
	streamCount  uint16
	requestCount uint32
	lastActivity time.Time
	counter      uint32
	closeReason  string
}

func newInboundSession(conn net.Conn, capabilities uint16, maxStreams uint16, opts SessionOptions, logger log.StructuredLogger) *inboundSession {
	now := time.Now()
	return &inboundSession{conn: conn, capabilities: capabilities, maxStreams: maxStreams, maxRequests: opts.MaxRequests, idleTimeout: opts.IdleTimeout, maxAge: opts.MaxAge, keepalive: opts.KeepaliveInterval, createdAt: now, lastActivity: now, logger: logger}
}

func (s *inboundSession) canAccept() (bool, byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if s.closed {
		return false, ErrorProtocol
	}
	if atomic.LoadInt32(&s.active) != 0 {
		return false, ErrorStreamIDReused
	}
	if s.streamCount >= s.maxStreams {
		return false, ErrorMaxRequests
	}
	if s.maxRequests > 0 && s.requestCount >= s.maxRequests {
		return false, ErrorMaxRequests
	}
	if s.maxAge > 0 && now.Sub(s.createdAt) >= s.maxAge {
		return false, ErrorMaxRequests
	}
	if s.idleTimeout > 0 && now.Sub(s.lastActivity) >= s.idleTimeout {
		return false, ErrorIdleTimeout
	}
	s.streamCount++
	s.requestCount++
	s.lastActivity = now
	atomic.StoreInt32(&s.active, 1)
	return true, ErrorOK
}

func (s *inboundSession) release() {
	atomic.StoreInt32(&s.active, 0)
	s.markActivity()
	if !s.reusable() {
		s.close()
	}
}

func (s *inboundSession) reusable() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.closed && s.capabilities&CapabilityReuse != 0 && s.streamCount < s.maxStreams && (s.maxRequests == 0 || s.requestCount < s.maxRequests) && (s.maxAge == 0 || time.Since(s.createdAt) < s.maxAge)
}

func (s *inboundSession) markActivity() {
	s.mu.Lock()
	s.lastActivity = time.Now()
	s.mu.Unlock()
}

func (s *inboundSession) writeFrame(frameType byte, flags byte, streamID uint32, payload []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.markActivity()
	return WriteBufferedFrame(s.conn, frameType, flags, streamID, payload)
}

func (s *inboundSession) close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.mu.Unlock()
	_ = s.conn.Close()
}

func (s *inboundSession) startKeepalive() {
	if s.capabilities&CapabilityKeepalive == 0 || s.keepalive <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(s.keepalive)
		defer ticker.Stop()
		for range ticker.C {
			if !s.checkLifecycle() {
				return
			}
			if !s.sendPing() {
				return
			}
		}
	}()
}

func (s *inboundSession) checkLifecycle() bool {
	now := time.Now()
	s.mu.Lock()
	closed := s.closed
	tooOld := s.maxAge > 0 && now.Sub(s.createdAt) >= s.maxAge
	idle := atomic.LoadInt32(&s.active) == 0 && s.idleTimeout > 0 && now.Sub(s.lastActivity) >= s.idleTimeout
	s.mu.Unlock()
	if closed {
		return false
	}
	if tooOld || idle {
		if tooOld {
			s.closeReason = "max age exceeded"
		} else {
			s.closeReason = "idle timeout"
		}
		s.close()
		return false
	}
	return true
}

func (s *inboundSession) sendPing() bool {
	if atomic.LoadInt32(&s.active) != 0 {
		return true
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return false
	}
	s.counter++
	counter := s.counter
	s.mu.Unlock()
	ping := Ping{Timestamp: uint64(time.Now().UnixMilli()), Counter: counter, Token: rand.Uint32()}
	if err := s.writeFrame(FrameTypePing, 0, 0, EncodePing(ping)); err != nil {
		s.closeReason = "keepalive timeout"
		s.close()
		return false
	}
	_ = s.conn.SetReadDeadline(time.Now().Add(s.keepalive * 2))
	frame, err := ReadFrame(s.conn, MaxPayloadLength)
	_ = s.conn.SetReadDeadline(time.Time{})
	if err != nil || frame.Type != FrameTypePong {
		s.closeReason = "keepalive timeout"
		s.close()
		return false
	}
	pong, err := DecodePong(frame.Payload)
	if err != nil || ValidatePong(ping, pong) != nil {
		s.closeReason = "keepalive timeout"
		s.close()
		return false
	}
	s.markActivity()
	return true
}

func readFrameWithSessionActivity(conn net.Conn, maxPayload int, mark func()) (Frame, error) {
	frame, err := ReadFrame(conn, maxPayload)
	if err == nil && mark != nil {
		mark()
	}
	return frame, err
}

func isClosedOrEOF(err error) bool {
	return err == io.EOF || E.IsClosedOrCanceled(err)
}
