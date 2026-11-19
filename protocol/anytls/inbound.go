package anytls

import (
	"context"
	"net"
	"strings"

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
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"

	anytls "github.com/anytls/sing-anytls"
	"github.com/anytls/sing-anytls/padding"
)

func RegisterInbound(registry *inbound.Registry) {
	inbound.Register[option.AnyTLSInboundOptions](registry, C.TypeAnyTLS, NewInbound)
}

type Inbound struct {
	inbound.Adapter
	tlsConfig    tls.ServerConfig
	router       adapter.ConnectionRouterEx
	logger       log.StructuredLogger
	listener     *listener.Listener
	service      *anytls.Service
	fallbackAddr M.Socksaddr

	// dnsRouter is set when clients may send DNS queries inside their
	// session, and dnsServer when they go to one server rather than through
	// the DNS rules.
	dnsRouter           adapter.DNSRouter
	dnsTransportManager adapter.DNSTransportManager
	dnsServerTag        string
	dnsServer           adapter.DNSTransport

	// trackingRouter accounts probe traffic, and is nil when the router
	// cannot.
	trackingRouter adapter.ConnectionTrackingRouter
}

func NewInbound(ctx context.Context, router adapter.Router, logger log.StructuredLogger, tag string, options option.AnyTLSInboundOptions) (adapter.Inbound, error) {
	inbound := &Inbound{
		Adapter: inbound.NewAdapter(C.TypeAnyTLS, tag),
		router:  uot.NewRouter(router, logger),
		logger:  logger,
	}
	inbound.trackingRouter, _ = router.(adapter.ConnectionTrackingRouter)

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

	var fallbackHandler N.TCPConnectionHandlerEx
	if options.Fallback != nil && options.Fallback.Server != "" {
		inbound.fallbackAddr = options.Fallback.Build()
		if !inbound.fallbackAddr.IsValid() {
			return nil, E.New("invalid fallback address: ", inbound.fallbackAddr)
		}
		fallbackHandler = adapter.NewUpstreamContextHandler(inbound.fallbackConnection, nil)
	}

	var dnsHandler func(ctx context.Context, request []byte) ([]byte, error)
	if options.DNS != nil && options.DNS.Enabled {
		inbound.dnsRouter = service.FromContext[adapter.DNSRouter](ctx)
		inbound.dnsTransportManager = service.FromContext[adapter.DNSTransportManager](ctx)
		if inbound.dnsRouter == nil || inbound.dnsTransportManager == nil {
			return nil, E.New("dns: missing DNS router")
		}
		inbound.dnsServerTag = options.DNS.Server
		dnsHandler = inbound.exchangeDNS
	}

	anytlsService, err := anytls.NewService(anytls.ServiceConfig{
		Users: common.Map(options.Users, func(it option.AnyTLSUser) anytls.User {
			return anytls.User(it)
		}),
		PaddingScheme:   paddingScheme,
		Handler:         (*inboundHandler)(inbound),
		FallbackHandler: fallbackHandler,
		Logger:          logger,
		StatsProvider:   serverStats,
		DNSHandler:      dnsHandler,
		Probe:           inbound.probeServerConfig(options.Probe),
	})
	if err != nil {
		return nil, err
	}
	inbound.service = anytlsService
	inbound.listener = listener.New(listener.Options{
		Context:           ctx,
		Logger:            logger,
		Network:           []string{N.NetworkTCP},
		Listen:            options.ListenOptions,
		ConnectionHandler: inbound,
	})
	return inbound, nil
}

func (h *Inbound) Start(stage adapter.StartStage, scope *adapter.Scope) error {
	if stage == adapter.StartStateInitialize && h.dnsServerTag != "" {
		// DNS servers are all created by now, and no connection can arrive
		// before the listeners start, so the field is set before any read.
		transport, loaded := h.dnsTransportManager.Transport(h.dnsServerTag)
		if !loaded {
			return E.New("dns: DNS server not found: ", h.dnsServerTag)
		}
		h.dnsServer = transport
	}
	if stage != adapter.StartStateStart {
		return nil
	}
	if h.tlsConfig != nil {
		err := h.tlsConfig.Start()
		if err != nil {
			return err
		}
		scope.Add(h.tlsConfig.Close)
	}
	err := h.listener.Start()
	if err != nil {
		return err
	}
	scope.Add(h.listener.Close)
	return nil
}

func (h *Inbound) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	if h.tlsConfig != nil {
		tlsConn, err := tls.ServerHandshake(ctx, conn, h.tlsConfig)
		if err != nil {
			N.CloseOnHandshakeFailure(conn, onClose, err)
			adapter.LogConnectionError(h.logger, ctx, err, metadata.Source)

			return
		}
		conn = tlsConn
	}
	err := h.service.NewConnection(adapter.WithContext(ctx, &metadata), conn, metadata.Source, onClose)
	if err != nil {
		N.CloseOnHandshakeFailure(conn, onClose, err)
		adapter.LogConnectionError(h.logger, ctx, err, metadata.Source)

	}
}

func (h *Inbound) fallbackConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	metadata.Inbound = h.Tag()
	metadata.InboundType = h.Type()
	//nolint:staticcheck
	metadata.InboundDetour = h.listener.ListenOptions().Detour
	metadata.Destination = h.fallbackAddr
	adapter.LogInboundConnection(h.logger, ctx, metadata)
	h.router.RouteConnectionEx(ctx, conn, metadata, onClose)
}

type inboundHandler Inbound

func (h *inboundHandler) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	var metadata adapter.InboundContext
	metadata.Inbound = h.Tag()
	metadata.InboundType = h.Type()
	//nolint:staticcheck
	metadata.InboundDetour = h.listener.ListenOptions().Detour
	//nolint:staticcheck
	metadata.Source = source
	metadata.Destination = destination.Unwrap()
	if userName, _ := auth.UserFromContext[string](ctx); userName != "" {
		metadata.User = userName
	}
	adapter.LogInboundConnection(h.logger, ctx, metadata)
	h.router.RouteConnectionEx(ctx, conn, metadata, onClose)
}
