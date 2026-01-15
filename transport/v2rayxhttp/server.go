package v2rayxhttp

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	aTLS "github.com/sagernet/sing/common/tls"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

var _ adapter.V2RayServerTransport = (*Server)(nil)

type Server struct {
	ctx            context.Context
	logger         logger.ContextLogger
	tlsConfig      tls.ServerConfig
	handler        adapter.V2RayServerTransportHandler
	httpServer     *http.Server
	h2Server       *http2.Server
	h2cHandler     http.Handler
	sessionManager *sessionManager
	config         *option.V2RayXHTTPOptions
	localAddr      net.Addr
}

func NewServer(ctx context.Context, logger logger.ContextLogger, options option.V2RayXHTTPOptions, tlsConfig tls.ServerConfig, handler adapter.V2RayServerTransportHandler) (*Server, error) {
	options = *normalizeConfig(&options)

	server := &Server{
		ctx:            ctx,
		logger:         logger,
		tlsConfig:      tlsConfig,
		handler:        handler,
		sessionManager: newSessionManager(),
		config:         &options,
		h2Server:       &http2.Server{},
	}

	server.httpServer = &http.Server{
		Handler:           server,
		ReadHeaderTimeout: C.TCPTimeout,
		MaxHeaderBytes:    http.DefaultMaxHeaderBytes,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
		ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			return log.ContextWithNewID(ctx)
		},
	}

	server.h2cHandler = h2c.NewHandler(server, server.h2Server)

	return server, nil
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	// Handle HTTP/2 cleartext preface
	if request.Method == "PRI" && len(request.Header) == 0 && request.URL.Path == "*" && request.Proto == "HTTP/2.0" {
		s.h2cHandler.ServeHTTP(writer, request)
		return
	}

	// Validate host header
	if s.config.Host != "" && request.Host != s.config.Host {
		s.logger.WarnContext(s.ctx, "xhttp: invalid host", "expected", s.config.Host, "got", request.Host)
		writer.WriteHeader(http.StatusNotFound)
		return
	}

	// Validate path prefix
	if !strings.HasPrefix(request.URL.Path, s.config.Path) {
		s.logger.WarnContext(s.ctx, "xhttp: invalid path", "expected prefix", s.config.Path, "got", request.URL.Path)
		writer.WriteHeader(http.StatusNotFound)
		return
	}

	// Validate x_padding query parameter
	paddingStr := request.URL.Query().Get("x_padding")
	paddingLen := int32(len(paddingStr))

	if paddingLen < s.config.XPaddingBytes.From || paddingLen > s.config.XPaddingBytes.To {
		s.logger.WarnContext(s.ctx, "xhttp: invalid x_padding length", "length", paddingLen)
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	// Extract session ID from path
	subpath := strings.TrimPrefix(request.URL.Path, s.config.Path)
	parts := strings.Split(subpath, "/")
	sessionID := ""
	if len(parts) > 0 && parts[0] != "" {
		sessionID = parts[0]
	}

	// Determine remote address
	remoteAddr := M.ParseSocksaddr(request.RemoteAddr)

	// Handle POST request (upload)
	if request.Method == "POST" && sessionID != "" {
		s.handleUpload(writer, request, sessionID, parts, remoteAddr)
		return
	}

	// Handle GET request (download)
	if request.Method == "GET" {
		s.handleDownload(writer, request, sessionID, remoteAddr)
		return
	}

	// Unsupported method
	s.logger.WarnContext(s.ctx, "xhttp: unsupported method", "method", request.Method)
	writer.WriteHeader(http.StatusMethodNotAllowed)
}

func (s *Server) handleUpload(writer http.ResponseWriter, request *http.Request, sessionID string, parts []string, remoteAddr M.Socksaddr) {
	// Get or create session
	session := s.sessionManager.getOrCreateSession(sessionID, int(s.config.ScMaxBufferedPosts))

	// Extract sequence number from path
	seq := uint64(0)
	if len(parts) > 1 && parts[1] != "" {
		var err error
		seq, err = strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			s.logger.ErrorContext(s.ctx, "xhttp: invalid sequence number", "seq", parts[1], "error", err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	// Read payload from request body
	maxBytes := int64(getNormalizedValue(s.config.ScMaxEachPostBytes))
	payload, err := io.ReadAll(io.LimitReader(request.Body, maxBytes+1))
	if err != nil {
		s.logger.ErrorContext(s.ctx, "xhttp: failed to read upload payload", "error", err)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	if int64(len(payload)) > maxBytes {
		s.logger.ErrorContext(s.ctx, "xhttp: upload payload too large", "size", len(payload), "max", maxBytes)
		writer.WriteHeader(http.StatusRequestEntityTooLarge)
		return
	}

	// Push packet to upload queue
	err = session.uploadQueue.Push(Packet{
		Payload: payload,
		Seq:     seq,
	})

	if err != nil {
		s.logger.ErrorContext(s.ctx, "xhttp: failed to push packet", "error", err)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	writer.WriteHeader(http.StatusOK)
}

func (s *Server) handleDownload(writer http.ResponseWriter, request *http.Request, sessionID string, remoteAddr M.Socksaddr) {
	var session *httpSession
	if sessionID != "" {
		// Get existing session
		var ok bool
		session, ok = s.sessionManager.getSession(sessionID)
		if !ok {
			s.logger.ErrorContext(s.ctx, "xhttp: session not found", "sessionID", sessionID)
			writer.WriteHeader(http.StatusNotFound)
			return
		}

		// Mark session as fully connected
		close(session.isFullyConnected)

		// Session will be deleted when connection closes
		defer s.sessionManager.deleteSession(sessionID)
	}

	// Set response headers
	writer.Header().Set("X-Accel-Buffering", "no")
	writer.Header().Set("Cache-Control", "no-store")

	if !s.config.NoGRPCHeader {
		writer.Header().Set("Content-Type", "text/event-stream")
	}

	// Add custom headers
	for key, values := range s.config.Headers.Build() {
		for _, value := range values {
			writer.Header().Set(key, value)
		}
	}

	writer.WriteHeader(http.StatusOK)
	if flusher, ok := writer.(http.Flusher); ok {
		flusher.Flush()
	}

	// Create HTTP server connection wrapper
	httpSC := &httpServerConn{
		ResponseWriter: writer,
		Reader:         request.Body,
		done:           make(chan struct{}),
	}

	// Create split connection
	conn := &splitConn{
		writer:     httpSC,
		reader:     httpSC,
		remoteAddr: remoteAddr.TCPAddr(),
		localAddr:  s.localAddr,
	}

	// For packet-up mode, read from upload queue
	if sessionID != "" {
		conn.reader = session.uploadQueue
	}

	// Pass connection to handler
	s.handler.NewConnectionEx(s.ctx, conn, remoteAddr, M.Socksaddr{}, nil)

	// Wait for connection to close
	select {
	case <-request.Context().Done():
	case <-httpSC.Done():
	}

	httpSC.Close()
}

func (s *Server) Network() []string {
	return []string{N.NetworkTCP}
}

func (s *Server) Serve(listener net.Listener) error {
	// Store local address
	if tcpListener, ok := listener.(*net.TCPListener); ok {
		s.localAddr = tcpListener.Addr()
	}

	if s.tlsConfig != nil {
		if len(s.tlsConfig.NextProtos()) == 0 {
			s.tlsConfig.SetNextProtos([]string{http2.NextProtoTLS, "http/1.1"})
		}
		listener = aTLS.NewListener(listener, s.tlsConfig)
	}

	return s.httpServer.Serve(listener)
}

func (s *Server) ServePacket(listener net.PacketConn) error {
	return os.ErrInvalid
}

func (s *Server) Close() error {
	return common.Close(common.PtrOrNil(s.httpServer))
}

// httpServerConn wraps http.ResponseWriter to implement io.ReadWriteCloser
type httpServerConn struct {
	mu   sync.Mutex
	done chan struct{}
	io.Reader
	http.ResponseWriter
}

func (c *httpServerConn) Write(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-c.done:
		return 0, io.ErrClosedPipe
	default:
	}

	n, err := c.ResponseWriter.Write(b)
	if err == nil {
		if flusher, ok := c.ResponseWriter.(http.Flusher); ok {
			flusher.Flush()
		}
	}
	return n, err
}

func (c *httpServerConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-c.done:
		return nil
	default:
		close(c.done)
		return nil
	}
}

func (c *httpServerConn) Done() <-chan struct{} {
	return c.done
}
