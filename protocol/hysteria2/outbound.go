package hysteria2

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/common/transportstats"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/protocol/tuic"
	qtls "github.com/sagernet/sing-quic"
	"github.com/sagernet/sing-quic/hysteria"
	"github.com/sagernet/sing-quic/hysteria2"
	"github.com/sagernet/sing-quic/hysteria2/realm"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

func RegisterOutbound(registry *outbound.Registry) {
	outbound.Register[option.Hysteria2OutboundOptions](registry, C.TypeHysteria2, NewOutbound)
}

var (
	_ adapter.Outbound                   = (*tuic.Outbound)(nil)
	_ adapter.InterfaceUpdateListener    = (*tuic.Outbound)(nil)
	_ adapter.OutboundWithTransportStats = (*Outbound)(nil)
)

type Outbound struct {
	outbound.Adapter
	ctx    context.Context
	logger log.StructuredLogger
	client *hysteria2.Client

	stats       *transportstats.Collector
	statsStart  sync.Once
	statsCancel context.CancelFunc
	statsAccess sync.Mutex
}

func NewOutbound(ctx context.Context, router adapter.Router, logger log.StructuredLogger, tag string, options option.Hysteria2OutboundOptions) (adapter.Outbound, error) {
	options.UDPFragmentDefault = true
	if options.TLS == nil || !options.TLS.Enabled {
		return nil, C.ErrTLSRequired
	}
	tlsServerAddress, tlsOptions, err := outboundTLSOptions(options)
	if err != nil {
		return nil, err
	}
	tlsConfig, err := tls.NewClient(ctx, logger, tlsServerAddress, tlsOptions)
	if err != nil {
		return nil, err
	}
	var salamanderPassword string
	var geckoPassword string
	var geckoMinPacketSize, geckoMaxPacketSize int
	if options.Obfs != nil {
		if options.Obfs.Password == "" {
			return nil, E.New("missing obfs password")
		}
		switch options.Obfs.Type {
		case hysteria2.ObfsTypeSalamander:
			salamanderPassword = options.Obfs.Password
		case hysteria2.ObfsTypeGecko:
			geckoPassword = options.Obfs.Password
			geckoMinPacketSize = options.Obfs.GeckoOptions.MinPacketSize
			geckoMaxPacketSize = options.Obfs.GeckoOptions.MaxPacketSize
		default:
			return nil, E.New("unknown obfs type: ", options.Obfs.Type)
		}
	}
	realmSTUNServersIsDomain := options.Realm != nil && options.Realm.STUNServersIsDomain()
	outboundDialer, err := dialer.NewWithOptions(dialer.Options{
		Context:          ctx,
		Options:          options.DialerOptions,
		RemoteIsDomain:   options.ServerIsDomain() || realmSTUNServersIsDomain,
		ResolverOnDetour: realmSTUNServersIsDomain,
		NewDialer:        realmSTUNServersIsDomain,
	})
	if err != nil {
		return nil, err
	}
	var realmOptions *realm.Options
	if options.Realm != nil {
		var queryOptions adapter.DNSQueryOptions
		if realmSTUNServersIsDomain {
			queryOptions = outboundDialer.(dialer.ResolveDialer).QueryOptions()
		}
		var httpClientTransport adapter.HTTPTransport
		httpClientTransport, err = service.FromContext[adapter.HTTPClientManager](ctx).ResolveTransport(ctx, logger, common.PtrValueOrDefault(options.Realm.HTTPClient))
		if err != nil {
			return nil, E.Cause(err, "create realm http client")
		}
		dnsRouter := service.FromContext[adapter.DNSRouter](ctx)
		realmOptions = &realm.Options{
			ServerURL:   options.Realm.ServerURL,
			Token:       options.Realm.Token,
			RealmID:     options.Realm.RealmID,
			STUNServers: options.Realm.STUNServers,
			HTTPClient:  &http.Client{Transport: httpClientTransport},
			Resolver: func(ctx context.Context, host string, ipv4, ipv6 bool) ([]netip.Addr, error) {
				dnsOptions := queryOptions
				switch {
				case ipv4 && !ipv6:
					dnsOptions.Strategy = C.DomainStrategyIPv4Only
				case !ipv4 && ipv6:
					dnsOptions.Strategy = C.DomainStrategyIPv6Only
				}
				return dnsRouter.Lookup(ctx, host, dnsOptions)
			},
			Logger:    logger,
			IPVersion: options.Realm.IPVersion,
		}
		if err := applyHysteria2RealmExtras(realmOptions, *options.Realm); err != nil {
			return nil, err
		}
		if options.Realm.PortMapping != nil && options.Realm.PortMapping.Enabled {
			realmOptions.PortMapping = &realm.PortMappingOptions{
				Timeout:  time.Duration(options.Realm.PortMapping.Timeout),
				Lifetime: time.Duration(options.Realm.PortMapping.Lifetime),
			}
		}
	}
	networkList := options.Network.Build()
	client, err := hysteria2.NewClient(hysteria2.ClientOptions{
		Context:            ctx,
		Dialer:             outboundDialer,
		Logger:             logger,
		BrutalDebug:        options.BrutalDebug,
		ServerAddress:      options.ServerOptions.Build(),
		ServerPorts:        options.ServerPorts,
		HopInterval:        time.Duration(options.HopInterval),
		HopIntervalMax:     time.Duration(options.HopIntervalMax),
		SendBPS:            uint64(options.UpMbps * hysteria.MbpsToBps),
		ReceiveBPS:         uint64(options.DownMbps * hysteria.MbpsToBps),
		SalamanderPassword: salamanderPassword,
		GeckoPassword:      geckoPassword,
		GeckoMinPacketSize: geckoMinPacketSize,
		GeckoMaxPacketSize: geckoMaxPacketSize,
		Password:           options.Password,
		TLSConfig:          tlsConfig,
		QUICOptions: qtls.QUICOptions{
			IdleTimeout:             options.IdleTimeout.Build(),
			KeepAlivePeriod:         options.KeepAlivePeriod.Build(),
			StreamReceiveWindow:     options.StreamReceiveWindow.Value(),
			ConnectionReceiveWindow: options.ConnectionReceiveWindow.Value(),
			MaxConcurrentStreams:    options.MaxConcurrentStreams,
			InitialPacketSize:       options.InitialPacketSize,
			DisablePathMTUDiscovery: options.DisablePathMTUDiscovery,
		},
		UDPDisabled:  !common.Contains(networkList, N.NetworkUDP),
		BBRProfile:   options.BBRProfile,
		ChromeParrot: !options.DisableChromeParrot,
		RealmOptions: realmOptions,
	})
	if err != nil {
		return nil, err
	}
	return &Outbound{
		Adapter: outbound.NewAdapterWithDialerOptions(C.TypeHysteria2, tag, networkList, options.DialerOptions),
		ctx:     ctx,
		logger:  logger,
		client:  client,
		stats:   transportstats.NewCollector(),
	}, nil
}

func outboundTLSOptions(options option.Hysteria2OutboundOptions) (string, option.OutboundTLSOptions, error) {
	tlsOptions := common.PtrValueOrDefault(options.TLS)
	if options.Realm == nil {
		return options.Server, tlsOptions, nil
	}
	if options.Server != "" || options.ServerPort != 0 || len(options.ServerPorts) > 0 {
		return "", tlsOptions, E.New("realm conflicts with server, server_port, and server_ports")
	}
	serverURL, err := url.Parse(options.Realm.ServerURL)
	if err != nil {
		return "", tlsOptions, E.Cause(err, "parse realm server_url")
	}
	serverName := serverURL.Hostname()
	if serverName == "" {
		return "", tlsOptions, E.New("missing host in realm server_url")
	}
	return serverName, tlsOptions, nil
}

func (h *Outbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		adapter.LogOutboundConnection(h.logger, ctx, destination)

		return h.client.DialConn(ctx, destination)
	case N.NetworkUDP:
		conn, err := h.ListenPacket(ctx, destination)
		if err != nil {
			return nil, err
		}
		return bufio.NewBindPacketConn(conn, destination), nil
	default:
		return nil, E.New("unsupported network: ", network)
	}
}

func (h *Outbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	adapter.LogOutboundPacket(h.logger, ctx, destination)

	return h.client.ListenPacket(ctx)
}

func (h *Outbound) InterfaceUpdated(ctx context.Context) {
	h.client.CloseWithError(E.New("network changed"))
}

func (h *Outbound) Close() error {
	h.statsAccess.Lock()
	if h.statsCancel != nil {
		h.statsCancel()
	}
	h.statsAccess.Unlock()
	return h.client.CloseWithError(os.ErrClosed)
}

// EnableTransportStats starts sampling the connection to the server.
//
// The client direction is this side's view as the sender of upstream traffic,
// read from the local QUIC connection. The server direction is the server's
// view as the sender of downstream traffic; it is only available from a server
// that agrees to report it, which is negotiated on the next connection.
func (h *Outbound) EnableTransportStats(direction adapter.TransportStatsDirection) {
	if direction == adapter.TransportStatsServer {
		h.client.RequestServerStats()
	}
	h.stats.Enable(transportstats.Direction(direction))
	h.statsStart.Do(func() {
		h.statsAccess.Lock()
		defer h.statsAccess.Unlock()
		ctx, cancel := context.WithCancel(h.ctx)
		h.statsCancel = cancel
		go transportstats.Run(ctx, h.pollTransportStats)
	})
}

func (h *Outbound) TransportStats(direction adapter.TransportStatsDirection) adapter.TransportStatsReader {
	recorder := h.stats.Recorder(transportstats.Direction(direction))
	if recorder == nil {
		return nil
	}
	return recorder
}

func (h *Outbound) pollTransportStats(ctx context.Context) {
	if h.stats.Enabled(transportstats.DirectionClient) {
		if stats, loaded := h.client.LocalStats(); loaded {
			h.stats.Record(transportstats.DirectionClient, []transportstats.Sample{quicSample(stats)}, true)
		}
	}
	if h.stats.Enabled(transportstats.DirectionServer) {
		if stats, loaded := h.client.ServerStats(ctx); loaded {
			h.stats.Record(transportstats.DirectionServer, []transportstats.Sample{quicSample(stats)}, true)
		}
	}
}

func quicSample(stats hysteria2.TransportStats) transportstats.Sample {
	return transportstats.Sample{
		Key:       stats.ConnectionID,
		Sent:      stats.PacketsSent,
		Lost:      stats.PacketsLost,
		BytesSent: stats.BytesSent,
		RTT:       stats.SmoothedRTT,
		RTTVar:    stats.RTTVariance,
	}
}
