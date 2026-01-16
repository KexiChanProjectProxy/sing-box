package router

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"net/netip"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

func RegisterInbound(registry *inbound.Registry) {
	inbound.Register[option.RouterInboundOptions](registry, C.TypeRouter, NewInbound)
}

var _ adapter.TCPInjectableInbound = (*Inbound)(nil)

type Inbound struct {
	inbound.Adapter
	ctx            context.Context
	router         adapter.Router
	inboundManager adapter.InboundManager
	logger         log.ContextLogger

	// HTTP server components
	ginEngine  *gin.Engine
	httpServer *http.Server

	// Routing configuration
	routes   []*compiledRoute
	fallback *fallbackHandler

	// Connection management
	hijackedConns sync.Map
	maxBodySize   int64

	// Network listener
	tcpListener net.Listener
	listenAddr  string
}

type compiledRoute struct {
	name          string
	matcher       *routeMatcher
	targetInbound string
	stripPrefix   string
	priority      int
}

func NewInbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.RouterInboundOptions) (adapter.Inbound, error) {
	// Set gin to release mode to reduce overhead
	gin.SetMode(gin.ReleaseMode)

	inbound := &Inbound{
		Adapter:        inbound.NewAdapter(C.TypeRouter, tag),
		ctx:            ctx,
		router:         router,
		inboundManager: service.FromContext[adapter.InboundManager](ctx),
		logger:         logger,
		ginEngine:      gin.New(),
		maxBodySize:    int64(options.MaxRequestBodySize),
	}

	if inbound.inboundManager == nil {
		return nil, E.New("inbound manager not found in context")
	}

	// Compile routes
	for i, routeOpt := range options.Routes {
		if routeOpt.Name == "" {
			return nil, E.New("route #", i, " is missing name")
		}
		if routeOpt.Target == "" {
			return nil, E.New("route ", routeOpt.Name, " is missing target inbound")
		}

		matcher, err := newRouteMatcher(routeOpt.Match)
		if err != nil {
			return nil, E.Cause(err, "compile route ", routeOpt.Name)
		}

		compiled := &compiledRoute{
			name:          routeOpt.Name,
			matcher:       matcher,
			targetInbound: routeOpt.Target,
			stripPrefix:   routeOpt.StripPathPrefix,
			priority:      routeOpt.Priority,
		}
		inbound.routes = append(inbound.routes, compiled)
	}

	// Sort routes by priority (higher first)
	sort.Slice(inbound.routes, func(i, j int) bool {
		return inbound.routes[i].priority > inbound.routes[j].priority
	})

	// Setup fallback
	if options.Fallback != nil {
		fb, err := newFallbackHandler(options.Fallback)
		if err != nil {
			return nil, E.Cause(err, "setup fallback")
		}
		inbound.fallback = fb
	} else {
		// Default fallback: reject with 404
		inbound.fallback = &fallbackHandler{
			fallbackType: "reject",
			statusCode:   404,
		}
	}

	// Setup gin engine
	inbound.setupGinEngine(options)

	// Create HTTP server with timeouts
	readTimeout := time.Duration(30 * time.Second)
	writeTimeout := time.Duration(30 * time.Second)
	idleTimeout := time.Duration(120 * time.Second)

	if options.Timeout != nil {
		if options.Timeout.Read > 0 {
			readTimeout = time.Duration(options.Timeout.Read)
		}
		if options.Timeout.Write > 0 {
			writeTimeout = time.Duration(options.Timeout.Write)
		}
		if options.Timeout.Idle > 0 {
			idleTimeout = time.Duration(options.Timeout.Idle)
		}
	}

	inbound.httpServer = &http.Server{
		Handler:      inbound.ginEngine,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
		BaseContext: func(listener net.Listener) context.Context {
			return ctx
		},
	}

	// Determine listen address - if no port specified, work as internal-only inbound
	if options.ListenPort > 0 {
		listenAddr := "0.0.0.0"
		if options.Listen != nil {
			listenAddr = options.Listen.Build(netip.IPv4Unspecified()).String()
		}
		inbound.listenAddr = net.JoinHostPort(listenAddr, strconv.Itoa(int(options.ListenPort)))
	} else {
		// Internal-only mode - no listener
		logger.Info("router inbound configured as internal-only (no listen port)")
	}

	return inbound, nil
}

func (r *Inbound) setupGinEngine(options option.RouterInboundOptions) {
	// Disable gin's default logger and use our own
	r.ginEngine.Use(gin.Recovery())

	// Set max body size if configured
	if r.maxBodySize > 0 {
		r.ginEngine.Use(func(c *gin.Context) {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, r.maxBodySize)
			c.Next()
		})
	}

	// Handle all requests with our main handler
	r.ginEngine.NoRoute(r.handleHTTPRequest)
}

func (r *Inbound) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}

	// Only create TCP listener if listenAddr is configured
	if r.listenAddr != "" {
		listener, err := net.Listen("tcp", r.listenAddr)
		if err != nil {
			return E.Cause(err, "listen on ", r.listenAddr)
		}
		r.tcpListener = listener

		// Start HTTP server in goroutine
		go func() {
			err := r.httpServer.Serve(r.tcpListener)
			if err != nil && err != http.ErrServerClosed {
				r.logger.Error("HTTP server error: ", err)
			}
		}()

		r.logger.Info("router inbound started on ", r.tcpListener.Addr())
	} else {
		r.logger.Info("router inbound started in internal-only mode")
	}

	return nil
}

func (r *Inbound) Close() error {
	var errors []error

	// Shutdown HTTP server gracefully
	if r.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := r.httpServer.Shutdown(ctx); err != nil {
			errors = append(errors, E.Cause(err, "shutdown HTTP server"))
		}
	}

	// Close all hijacked connections
	r.hijackedConns.Range(func(key, value interface{}) bool {
		if conn, ok := value.(net.Conn); ok {
			conn.Close()
		}
		return true
	})

	// Close TCP listener
	if r.tcpListener != nil {
		if err := r.tcpListener.Close(); err != nil {
			errors = append(errors, E.Cause(err, "close TCP listener"))
		}
	}

	if len(errors) > 0 {
		return E.Errors(errors...)
	}
	return nil
}

func (r *Inbound) NewConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	// Handle injected connections by parsing HTTP and routing
	r.logger.DebugContext(ctx, "received injected connection from ", metadata.Inbound)

	// Process the connection through HTTP parsing and routing
	go r.handleInjectedConnection(ctx, conn, metadata, onClose)
}

// forwardToInbound forwards a connection to a target inbound
func (r *Inbound) forwardToInbound(ctx context.Context, conn net.Conn, targetTag string, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) error {
	// Check for loop
	if metadata.Inbound == targetTag || metadata.LastInbound == targetTag {
		return E.New("routing loop detected: ", targetTag)
	}

	// Get target inbound
	targetInbound, loaded := r.inboundManager.Get(targetTag)
	if !loaded {
		return E.New("target inbound not found: ", targetTag)
	}

	// Verify it's TCP injectable
	injectable, ok := targetInbound.(adapter.TCPInjectableInbound)
	if !ok {
		return E.New("target inbound is not TCP injectable: ", targetTag)
	}

	// Update metadata
	metadata.LastInbound = metadata.Inbound
	metadata.Inbound = r.Tag()
	metadata.InboundType = r.Type()
	metadata.InboundDetour = targetTag

	// Forward connection
	r.logger.InfoContext(ctx, "successfully forwarding connection to inbound: ", targetTag)
	injectable.NewConnectionEx(ctx, conn, metadata, onClose)
	return nil
}

// handleInjectedConnection processes connections injected from other inbounds
func (r *Inbound) handleInjectedConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	defer func() {
		if onClose != nil {
			onClose(nil)
		}
	}()

	// Read HTTP request from the connection
	reader := bufio.NewReader(conn)
	req, err := http.ReadRequest(reader)
	if err != nil {
		r.logger.ErrorContext(ctx, "failed to read HTTP request: ", err)
		conn.Close()
		return
	}

	r.logger.DebugContext(ctx, "parsed HTTP request: ", req.Method, " ", req.URL.Path)

	// Find matching route
	var matchedRoute *compiledRoute
	for _, route := range r.routes {
		if route.matcher.matches(req) {
			matchedRoute = route
			r.logger.InfoContext(ctx, "matched route: ", route.name, " for ", req.URL.Path)
			break
		}
	}

	// If no route matched, use fallback
	if matchedRoute == nil {
		r.logger.DebugContext(ctx, "no route matched for ", req.URL.Path, ", using fallback")
		// For injected connections with no match, we can't serve static files
		// So we just close or forward to fallback inbound if configured
		if r.fallback.fallbackType == "inbound" && r.fallback.targetTag != "" {
			// Wrap connection with buffered reader to preserve the HTTP request data
			wrappedConn := &injectedConn{
				Conn:   conn,
				reader: reader,
			}
			metadata.InboundDetour = r.fallback.targetTag
			if err := r.forwardToInbound(ctx, wrappedConn, r.fallback.targetTag, metadata, nil); err != nil {
				r.logger.ErrorContext(ctx, "failed to forward to fallback inbound: ", err)
				conn.Close()
			}
		} else {
			// No valid fallback for injected connections
			r.logger.WarnContext(ctx, "no fallback available for injected connection, closing")
			conn.Close()
		}
		return
	}

	// Forward to target inbound
	// Wrap connection with buffered reader to preserve the HTTP request data
	wrappedConn := &injectedConn{
		Conn:   conn,
		reader: reader,
	}

	// Update metadata
	metadata.Inbound = r.Tag()
	metadata.InboundType = r.Type()

	r.logger.InfoContext(ctx, "forwarding injected connection to ", matchedRoute.targetInbound)
	if err := r.forwardToInbound(ctx, wrappedConn, matchedRoute.targetInbound, metadata, nil); err != nil {
		r.logger.ErrorContext(ctx, "failed to forward to ", matchedRoute.targetInbound, ": ", err)
		conn.Close()
	}
}

// trackHijackedConn returns an onClose handler that tracks hijacked connections
func (r *Inbound) trackHijackedConn(conn net.Conn) N.CloseHandlerFunc {
	connID := conn.RemoteAddr().String()
	r.hijackedConns.Store(connID, conn)
	return func(err error) {
		r.hijackedConns.Delete(connID)
	}
}

// injectedConn wraps a connection with a buffered reader for injected connections
type injectedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *injectedConn) Read(p []byte) (n int, err error) {
	// Check if there's buffered data first
	if c.reader.Buffered() > 0 {
		return c.reader.Read(p)
	}
	// Fall back to raw connection
	return c.Conn.Read(p)
}
