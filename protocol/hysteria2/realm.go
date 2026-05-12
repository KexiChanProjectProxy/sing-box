package hysteria2

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"time"

	"github.com/sagernet/sing-box/adapter"
	boxService "github.com/sagernet/sing-box/adapter/service"
	"github.com/sagernet/sing-box/common/listener"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-quic/hysteria2/realm"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	aTLS "github.com/sagernet/sing/common/tls"
	sHTTP "github.com/sagernet/sing/protocol/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"golang.org/x/net/http2"
)

func RegisterRealmService(registry *boxService.Registry) {
	boxService.Register(registry, C.TypeHysteriaRealm, NewRealmService)
}

func buildRealmOptions(_ context.Context, logger log.ContextLogger, options *option.Hysteria2Realm) (*realm.Options, error) {
	if options == nil {
		return nil, nil
	}
	return &realm.Options{
		ServerURL:   options.ServerURL,
		Token:       options.Token,
		RealmID:     options.RealmID,
		STUNServers: options.STUNServers,
		HTTPClient:  http.DefaultClient,
		Logger:      logger,
	}, nil
}

func listenRealmPacket(ctx context.Context, baseListenOptions option.ListenOptions, listenPorts []uint16) (net.PacketConn, error) {
	if len(listenPorts) == 0 {
		var listenConfig net.ListenConfig
		return listenConfig.ListenPacket(ctx, N.NetworkUDP, "")
	}
	listenOptions := baseListenOptions
	bindAddr := listenOptions.Listen.Build(netip.AddrFrom4([4]byte{127, 0, 0, 1}))
	listenConfig := realmListenConfig(listenOptions)
	var errors []error
	for _, port := range listenPorts {
		address := M.SocksaddrFrom(bindAddr, port)
		conn, err := listener.ListenNetworkNamespace(listenOptions.NetNs, func() (net.PacketConn, error) {
			return listenConfig.ListenPacket(ctx, M.NetworkFromNetAddr(N.NetworkUDP, bindAddr), address.String())
		})
		if err == nil {
			return conn, nil
		}
		errors = append(errors, E.Cause(err, "listen realm port ", address))
	}
	return nil, E.Cause(E.Errors(errors...), "listen realm ports")
}

func realmListenConfig(options option.ListenOptions) net.ListenConfig {
	var listenConfig net.ListenConfig
	if options.ReuseAddr {
		listenConfig.Control = control.Append(listenConfig.Control, control.ReuseAddr())
	}
	if options.RoutingMark != 0 {
		listenConfig.Control = control.Append(listenConfig.Control, control.RoutingMark(uint32(options.RoutingMark)))
	}
	var udpFragment bool
	if options.UDPFragment != nil {
		udpFragment = *options.UDPFragment
	} else {
		udpFragment = options.UDPFragmentDefault
	}
	if !udpFragment {
		listenConfig.Control = control.Append(listenConfig.Control, control.DisableUDPFragment())
	}
	return listenConfig
}

func realmPreferIPVersion(preferIPVersion string) (C.DomainStrategy, error) {
	switch preferIPVersion {
	case "":
		return C.DomainStrategyAsIS, nil
	case "prefer_ipv4":
		return C.DomainStrategyPreferIPv4, nil
	case "prefer_ipv6":
		return C.DomainStrategyPreferIPv6, nil
	case "ipv4_only":
		return C.DomainStrategyIPv4Only, nil
	case "ipv6_only":
		return C.DomainStrategyIPv6Only, nil
	default:
		return C.DomainStrategyAsIS, E.New("unknown prefer_ip_version: ", preferIPVersion)
	}
}

func realmFallbackTimeout(fallbackTimeout string) (time.Duration, error) {
	if fallbackTimeout == "" {
		return 0, nil
	}
	timeout, err := time.ParseDuration(fallbackTimeout)
	if err != nil {
		return 0, E.Cause(err, "parse fallback_timeout")
	}
	if timeout < 0 {
		return 0, E.New("fallback_timeout cannot be negative")
	}
	return timeout, nil
}

type realmPreferDialer struct {
	N.Dialer
	strategy        C.DomainStrategy
	fallbackTimeout time.Duration
}

func NewRealmPreferDialer(upstream N.Dialer, preferIPVersion string, fallbackTimeout string) (N.Dialer, error) {
	strategy, err := realmPreferIPVersion(preferIPVersion)
	if err != nil {
		return nil, err
	}
	timeout, err := realmFallbackTimeout(fallbackTimeout)
	if err != nil {
		return nil, err
	}
	if strategy == C.DomainStrategyAsIS && timeout == 0 {
		return upstream, nil
	}
	return &realmPreferDialer{Dialer: upstream, strategy: strategy, fallbackTimeout: timeout}, nil
}

func (d *realmPreferDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	if !destination.IsDomain() {
		return d.Dialer.ListenPacket(ctx, destination)
	}
	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", destination.Fqdn)
	if err != nil {
		return nil, E.Cause(err, "resolve realm destination: ", destination.Fqdn)
	}
	addresses = filterRealmAddresses(addresses, d.strategy)
	if len(addresses) == 0 {
		return nil, E.New("no addresses for realm destination: ", destination.Fqdn)
	}
	conn, destinationAddress, err := listenRealmResolvedPacket(ctx, d.Dialer, destination, addresses, d.strategy, d.fallbackTimeout)
	if err != nil {
		return nil, err
	}
	return bufio.NewNATPacketConn(bufio.NewPacketConn(conn), M.SocksaddrFrom(destinationAddress, destination.Port), destination), nil
}

func filterRealmAddresses(addresses []netip.Addr, strategy C.DomainStrategy) []netip.Addr {
	switch strategy {
	case C.DomainStrategyIPv4Only:
		return common.Filter(addresses, func(address netip.Addr) bool { return address.Is4() || address.Is4In6() })
	case C.DomainStrategyIPv6Only:
		return common.Filter(addresses, func(address netip.Addr) bool { return address.Is6() && !address.Is4In6() })
	case C.DomainStrategyPreferIPv4:
		return sortRealmAddresses(addresses, false)
	case C.DomainStrategyPreferIPv6:
		return sortRealmAddresses(addresses, true)
	default:
		return addresses
	}
}

func sortRealmAddresses(addresses []netip.Addr, preferIPv6 bool) []netip.Addr {
	addresses = slices.Clone(addresses)
	slices.SortStableFunc(addresses, func(left netip.Addr, right netip.Addr) int {
		leftIPv6 := left.Is6() && !left.Is4In6()
		rightIPv6 := right.Is6() && !right.Is4In6()
		if leftIPv6 == rightIPv6 {
			return 0
		}
		if leftIPv6 == preferIPv6 {
			return -1
		}
		return 1
	})
	return addresses
}

func listenRealmResolvedPacket(ctx context.Context, dialer N.Dialer, destination M.Socksaddr, addresses []netip.Addr, strategy C.DomainStrategy, fallbackTimeout time.Duration) (net.PacketConn, netip.Addr, error) {
	if strategy != C.DomainStrategyPreferIPv4 && strategy != C.DomainStrategyPreferIPv6 || fallbackTimeout == 0 {
		return N.ListenSerial(ctx, dialer, destination, addresses)
	}
	addresses4 := common.Filter(addresses, func(address netip.Addr) bool { return address.Is4() || address.Is4In6() })
	addresses6 := common.Filter(addresses, func(address netip.Addr) bool { return address.Is6() && !address.Is4In6() })
	if len(addresses4) == 0 || len(addresses6) == 0 {
		return N.ListenSerial(ctx, dialer, destination, addresses)
	}
	primary, fallback := addresses4, addresses6
	if strategy == C.DomainStrategyPreferIPv6 {
		primary, fallback = addresses6, addresses4
	}
	conn, address, err := listenRealmSerialPacket(ctx, dialer, destination, primary)
	if err == nil {
		return conn, address, nil
	}
	timer := time.NewTimer(fallbackTimeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, netip.Addr{}, ctx.Err()
	case <-timer.C:
	}
	fallbackConn, fallbackAddress, fallbackErr := listenRealmSerialPacket(ctx, dialer, destination, fallback)
	if fallbackErr == nil {
		return fallbackConn, fallbackAddress, nil
	}
	return nil, netip.Addr{}, E.Errors(err, fallbackErr)
}

func listenRealmSerialPacket(ctx context.Context, dialer N.Dialer, destination M.Socksaddr, addresses []netip.Addr) (net.PacketConn, netip.Addr, error) {
	var connErrors []error
	for _, address := range addresses {
		conn, err := dialer.ListenPacket(ctx, M.SocksaddrFrom(address, destination.Port))
		if err == nil {
			return conn, address, nil
		}
		connErrors = append(connErrors, err)
	}
	return nil, netip.Addr{}, E.Errors(connErrors...)
}

type RealmService struct {
	boxService.Adapter
	ctx        context.Context
	cancel     context.CancelFunc
	logger     log.ContextLogger
	listener   *listener.Listener
	tlsConfig  tls.ServerConfig
	httpServer *http.Server
	server     *server
}

func NewRealmService(ctx context.Context, logger log.ContextLogger, tag string, options option.HysteriaRealmServiceOptions) (adapter.Service, error) {
	if len(options.Users) == 0 {
		return nil, E.New("missing users")
	}
	tokenMap := make(map[string]*realmUser, len(options.Users))
	for i, user := range options.Users {
		if user.Name == "" {
			return nil, E.New("missing name for user[", i, "]")
		}
		if user.Token == "" {
			return nil, E.New("missing token for user[", i, "]")
		}
		tokenMap[user.Token] = &realmUser{
			name:      user.Name,
			maxRealms: user.MaxRealms,
		}
	}
	server := newServer(logger, tokenMap)
	ctx, cancel := context.WithCancel(ctx)
	chiRouter := chi.NewRouter()
	chiRouter.Use(middleware.RequestSize(maxRequestBodyBytes))
	chiRouter.Use(func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.DebugContext(r.Context(), r.Method, " ", r.RequestURI, " ", sHTTP.SourceAddress(r))
			handler.ServeHTTP(w, r)
		})
	})
	chiRouter.Route("/v1/{id}", func(r chi.Router) {
		r.Use(validateRealmID)
		r.With(server.authUser).Post("/", server.handleRegister)
		r.With(server.authSession).Delete("/", server.handleDeregister)
		r.With(server.authSession).Get("/events", server.handleEvents)
		r.With(server.authSession).Post("/heartbeat", server.handleHeartbeat)
		r.With(server.authUser).Post("/connect", server.handleConnect)
		r.With(server.authSession).Post("/connects/{nonce}", server.handleConnectResponse)
	})
	chiRouter.NotFound(func(w http.ResponseWriter, r *http.Request) {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, render.M{"error": "not_found", "message": "unknown path"})
	})
	chiRouter.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		render.Status(r, http.StatusMethodNotAllowed)
		render.JSON(w, r, render.M{"error": "bad_request", "message": "method not allowed"})
	})
	s := &RealmService{
		Adapter: boxService.NewAdapter(C.TypeHysteriaRealm, tag),
		ctx:     ctx,
		cancel:  cancel,
		logger:  logger,
		listener: listener.New(listener.Options{
			Context: ctx,
			Logger:  logger,
			Network: []string{N.NetworkTCP},
			Listen:  options.ListenOptions,
		}),
		httpServer: &http.Server{
			Handler: chiRouter,
			ConnContext: func(ctx context.Context, _ net.Conn) context.Context {
				return log.ContextWithNewID(ctx)
			},
		},
		server: server,
	}
	if options.TLS != nil {
		tlsConfig, err := tls.NewServer(ctx, logger, common.PtrValueOrDefault(options.TLS))
		if err != nil {
			return nil, err
		}
		s.tlsConfig = tlsConfig
	}
	return s, nil
}

func (s *RealmService) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	if s.tlsConfig != nil {
		err := s.tlsConfig.Start()
		if err != nil {
			return E.Cause(err, "create TLS config")
		}
	}
	tcpListener, err := s.listener.ListenTCP()
	if err != nil {
		return err
	}
	if s.tlsConfig != nil {
		if !common.Contains(s.tlsConfig.NextProtos(), http2.NextProtoTLS) {
			s.tlsConfig.SetNextProtos(append([]string{"h2"}, s.tlsConfig.NextProtos()...))
		}
		tcpListener = aTLS.NewListener(tcpListener, s.tlsConfig)
	}
	go func() {
		err = s.httpServer.Serve(tcpListener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("serve error: ", err)
		}
	}()
	return nil
}

func (s *RealmService) Close() error {
	s.cancel()
	err := common.Close(common.PtrOrNil(s.httpServer))
	s.server.closeAll()
	return E.Errors(err, common.Close(
		common.PtrOrNil(s.listener),
		s.tlsConfig,
	))
}
