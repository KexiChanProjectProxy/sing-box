package anytls

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/common/listener"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/common/uot"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	anytls "github.com/anytls/sing-anytls"
	"github.com/anytls/sing-anytls/padding"
)

func RegisterInbound(registry *inbound.Registry) {
	inbound.Register[option.AnyTLSInboundOptions](registry, C.TypeAnyTLS, NewInbound)
}

type Inbound struct {
	inbound.Adapter
	tlsConfig tls.ServerConfig
	router    adapter.ConnectionRouterEx
	logger    logger.ContextLogger
	listener  *listener.Listener
	service   *anytls.Service
}

func NewInbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.AnyTLSInboundOptions) (adapter.Inbound, error) {
	inbound := &Inbound{
		Adapter: inbound.NewAdapter(C.TypeAnyTLS, tag),
		router:  uot.NewRouter(router, logger),
		logger:  logger,
	}

	if options.TLS != nil && options.TLS.Enabled {
		tlsConfig, err := tls.NewServer(ctx, logger, common.PtrValueOrDefault(options.TLS))
		if err != nil {
			return nil, err
		}
		inbound.tlsConfig = tlsConfig
	}

	paddingScheme := padding.DefaultPaddingScheme
	if len(options.PaddingScheme) > 0 {
		paddingScheme = []byte(strings.Join(options.PaddingScheme, "\n"))
	}

	var masqueradeHandler http.Handler
	if options.Masquerade != nil && options.Masquerade.Type != "" {
		switch options.Masquerade.Type {
		case C.AnyTLSMasqueradeTypeFile:
			masqueradeHandler = http.FileServer(http.Dir(options.Masquerade.FileOptions.Directory))
		case C.AnyTLSMasqueradeTypeProxy:
			masqueradeURL, err := url.Parse(options.Masquerade.ProxyOptions.URL)
			if err != nil {
				return nil, E.Cause(err, "parse masquerade URL")
			}
			masqueradeHandler = &httputil.ReverseProxy{
				Rewrite: func(r *httputil.ProxyRequest) {
					r.SetURL(masqueradeURL)
					if !options.Masquerade.ProxyOptions.RewriteHost {
						r.Out.Host = r.In.Host
					}

					// Add X-Forwarded-For header
					if clientIP, _, err := net.SplitHostPort(r.In.RemoteAddr); err == nil {
						if prior := r.In.Header.Get("X-Forwarded-For"); prior != "" {
							clientIP = prior + ", " + clientIP
						}
						r.Out.Header.Set("X-Forwarded-For", clientIP)
					}

					// Add X-Real-IP header
					if clientIP, _, err := net.SplitHostPort(r.In.RemoteAddr); err == nil {
						r.Out.Header.Set("X-Real-IP", clientIP)
					}
				},
				ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
					w.WriteHeader(http.StatusBadGateway)
				},
			}
		case C.AnyTLSMasqueradeTypeString:
			masqueradeHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Set headers first (before WriteHeader)
				for key, values := range options.Masquerade.StringOptions.Headers {
					for _, value := range values {
						w.Header().Add(key, value)
					}
				}
				// Then set status code
				if options.Masquerade.StringOptions.StatusCode != 0 {
					w.WriteHeader(options.Masquerade.StringOptions.StatusCode)
				}
				// Finally write body
				w.Write([]byte(options.Masquerade.StringOptions.Content))
			})
		case C.AnyTLSMasqueradeTypeRedirect:
			baseURL := options.Masquerade.RedirectOptions.URL
			statusCode := options.Masquerade.RedirectOptions.StatusCode
			if statusCode == 0 {
				statusCode = http.StatusFound // Default to 302
			}
			customHeaders := options.Masquerade.RedirectOptions.Headers
			appendRequestURI := options.Masquerade.RedirectOptions.AppendRequestURI

			// Parse base URL once during initialization
			redirectBaseURL, err := url.Parse(baseURL)
			if err != nil {
				return nil, E.Cause(err, "parse redirect URL")
			}

			masqueradeHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Build redirect URL
				redirectURL := baseURL
				if appendRequestURI {
					// Combine base URL with request URI
					targetURL := *redirectBaseURL
					targetURL.Path = strings.TrimRight(targetURL.Path, "/") + r.URL.Path
					targetURL.RawQuery = r.URL.RawQuery
					targetURL.Fragment = r.URL.Fragment
					redirectURL = targetURL.String()
				}

				// Prepare response body
				body := "<a href=\"" + redirectURL + "\">Moved</a>.\n"

				// Add custom headers
				for key, values := range customHeaders {
					for _, value := range values {
						w.Header().Add(key, value)
					}
				}

				// Set required headers
				w.Header().Set("Location", redirectURL)
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Header().Set("Content-Length", strconv.Itoa(len(body)))

				// Send redirect status
				w.WriteHeader(statusCode)

				// Write body
				w.Write([]byte(body))
			})
		default:
			return nil, E.New("unknown masquerade type: ", options.Masquerade.Type)
		}
	}

	var fallbackHandler N.TCPConnectionHandlerEx
	if masqueradeHandler != nil {
		fallbackHandler = &httpMasqueradeHandler{
			httpHandler: masqueradeHandler,
			logger:      logger,
		}
	}

	service, err := anytls.NewService(anytls.ServiceConfig{
		Users: common.Map(options.Users, func(it option.AnyTLSUser) anytls.User {
			return (anytls.User)(it)
		}),
		PaddingScheme:   paddingScheme,
		Handler:         (*inboundHandler)(inbound),
		FallbackHandler: fallbackHandler,
		Logger:          logger,
	})
	if err != nil {
		return nil, err
	}
	inbound.service = service
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
	if h.tlsConfig != nil {
		err := h.tlsConfig.Start()
		if err != nil {
			return err
		}
	}
	return h.listener.Start()
}

func (h *Inbound) Close() error {
	return common.Close(h.listener, h.tlsConfig)
}

func (h *Inbound) NewConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	if h.tlsConfig != nil {
		tlsConn, err := tls.ServerHandshake(ctx, conn, h.tlsConfig)
		if err != nil {
			N.CloseOnHandshakeFailure(conn, onClose, err)
			event := log.NewConnectionEvent("inbound", "error").WithSource(metadata.Source).WithError(err)
			log.WithConnectionEvent(h.logger, ctx, log.LevelError, event, E.Cause(err, "process connection from ", metadata.Source, ": TLS handshake"))
			return
		}
		conn = tlsConn
	}
	err := h.service.NewConnection(adapter.WithContext(ctx, &metadata), conn, metadata.Source, onClose)
	if err != nil {
		N.CloseOnHandshakeFailure(conn, onClose, err)
		event := log.NewConnectionEvent("inbound", "error").WithSource(metadata.Source).WithError(err)
		log.WithConnectionEvent(h.logger, ctx, log.LevelError, event, E.Cause(err, "process connection from ", metadata.Source))
	}
}

type inboundHandler Inbound

func (h *inboundHandler) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	var metadata adapter.InboundContext
	metadata.Inbound = h.Tag()
	metadata.InboundType = h.Type()
	//nolint:staticcheck
	metadata.InboundDetour = h.listener.ListenOptions().Detour
	//nolint:staticcheck
	metadata.InboundOptions = h.listener.ListenOptions().InboundOptions
	metadata.Source = source
	metadata.Destination = destination.Unwrap()
	if userName, _ := auth.UserFromContext[string](ctx); userName != "" {
		metadata.User = userName
		event := log.NewConnectionEvent("inbound", "start").WithDestination(metadata.Destination).WithUser(userName)
		log.WithConnectionEvent(h.logger, ctx, log.LevelInfo, event, "[", userName, "] inbound connection to ", metadata.Destination)
	} else {
		event := log.NewConnectionEvent("inbound", "start").WithDestination(metadata.Destination)
		log.WithConnectionEvent(h.logger, ctx, log.LevelInfo, event, "inbound connection to ", metadata.Destination)
	}
	h.router.RouteConnectionEx(ctx, conn, metadata, onClose)
}

type httpMasqueradeHandler struct {
	httpHandler http.Handler
	logger      logger.ContextLogger
}

func (h *httpMasqueradeHandler) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	defer func() {
		conn.Close()
		if onClose != nil {
			onClose(nil)
		}
	}()

	h.logger.DebugContext(ctx, "serving HTTP masquerade for connection from ", source)

	// Handle HTTP connection directly instead of using http.Server.Serve()
	// This avoids issues with singleConnListener
	err := http.Serve(&oneShotListener{conn: conn}, h.httpHandler)
	if err != nil && err != http.ErrServerClosed && err != io.EOF {
		h.logger.DebugContext(ctx, "HTTP masquerade error: ", err)
	}
}

// oneShotListener serves exactly one connection then returns EOF
type oneShotListener struct {
	conn   net.Conn
	served bool
	mu     sync.Mutex
	done   chan struct{}
}

func (l *oneShotListener) Accept() (net.Conn, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.served {
		// Already served, wait until closed
		if l.done != nil {
			<-l.done
		}
		return nil, io.EOF
	}

	if l.done == nil {
		l.done = make(chan struct{})
	}

	l.served = true
	return l.conn, nil
}

func (l *oneShotListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.done != nil {
		close(l.done)
	}
	return nil
}

func (l *oneShotListener) Addr() net.Addr {
	return l.conn.LocalAddr()
}
