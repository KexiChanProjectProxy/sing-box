//go:build with_quic

package cloudflarewarp

import (
	"context"
	"crypto/ecdsa"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/iponly"
	boxTLS "github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/common/warpapi"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	warptransport "github.com/sagernet/sing-box/transport/cloudflarewarp"
	"github.com/sagernet/sing-box/transport/ipstack"
	"github.com/sagernet/sing-quic"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

const (
	readyWaitTimeout       = 10 * time.Second
	minReconnectDelay      = time.Second
	maxReconnectDelay      = 30 * time.Second
	maxAPIRetryDelay       = 5 * time.Minute
	stableSessionTime      = time.Minute
	tooLargeLogInterval    = time.Minute
	defaultServerPort      = warpapi.DefaultPort
	registrationTimeout    = time.Minute
	ephemeralDeleteTimeout = 5 * time.Second
)

func RegisterOutbound(registry *outbound.Registry) {
	outbound.Register[option.CloudflareWARPOutboundOptions](registry, C.TypeCloudflareWARP, NewOutbound)
}

var (
	_ adapter.Outbound                   = (*Outbound)(nil)
	_ adapter.Lifecycle                  = (*Outbound)(nil)
	_ adapter.InterfaceUpdateListener    = (*Outbound)(nil)
	_ dialer.PacketDialerWithDestination = (*Outbound)(nil)
)

type Outbound struct {
	outbound.Adapter
	ctx          context.Context
	cancel       context.CancelFunc
	logger       log.StructuredLogger
	options      option.CloudflareWARPOutboundOptions
	dialer       N.Dialer
	apiAccess    sync.Mutex
	apiHTTP      *http.Client
	dnsRouter    adapter.DNSRouter
	cacheFile    adapter.CacheFile
	baseTLS      *tls.Config
	device       ipstack.Device
	dependencies []string
	static       *warpapi.Registration

	access       sync.Mutex
	session      warptransport.Session
	readyCh      chan struct{}
	registration *warpapi.Registration
	privateKey   *ecdsa.PrivateKey
	started      bool
	loopDone     chan struct{}
	kick         chan struct{}
	kicked       atomic.Bool
	lastTooLarge atomic.Int64
	closeOnce    sync.Once
}

func NewOutbound(ctx context.Context, router adapter.Router, logger log.StructuredLogger, tag string, options option.CloudflareWARPOutboundOptions) (adapter.Outbound, error) {
	err := options.Validate()
	if err != nil {
		return nil, err
	}
	o := &Outbound{
		Adapter:      outbound.NewAdapterWithDialerOptions(C.TypeCloudflareWARP, tag, options.Network.Build(), options.DialerOptions),
		logger:       logger,
		options:      options,
		dnsRouter:    service.FromContext[adapter.DNSRouter](ctx),
		dependencies: dependencies(options),
		readyCh:      make(chan struct{}),
		loopDone:     make(chan struct{}),
		kick:         make(chan struct{}, 1),
	}
	o.ctx, o.cancel = context.WithCancel(ctx)
	if options.StaticMode() {
		o.static, err = staticRegistration(options)
		if err != nil {
			return nil, err
		}
	}
	o.dialer, err = dialer.NewWithOptions(dialer.Options{
		Context:          ctx,
		Options:          options.DialerOptions,
		RemoteIsDomain:   options.ServerIsDomain(),
		ResolverOnDetour: true,
		NewDialer:        true,
	})
	if err != nil {
		return nil, err
	}
	tlsOptions := common.PtrValueOrDefault(options.TLS)
	tlsOptions.Enabled = true
	if tlsOptions.ServerName == "" {
		tlsOptions.ServerName = warptransport.DefaultServerName
	}
	tlsConfig, err := boxTLS.NewClientWithOptions(boxTLS.ClientOptions{
		Context:       ctx,
		Logger:        logger,
		ServerAddress: tlsOptions.ServerName,
		Options:       tlsOptions,
	})
	if err != nil {
		return nil, err
	}
	o.baseTLS, err = tlsConfig.STDConfig()
	if err != nil {
		return nil, err
	}
	var addresses []netip.Prefix
	if o.static != nil {
		addresses = o.static.Addresses()
	}
	o.device, err = ipstack.NewDevice(ipstack.DeviceOptions{
		Context:        ctx,
		Logger:         logger,
		MTU:            options.MTU,
		Addresses:      addresses,
		PacketHeadroom: warptransport.PacketHeadroom,
	})
	if err != nil {
		return nil, E.Cause(err, "cloudflare-warp requires the with_gvisor build tag")
	}
	o.device.SetPacketWriter(o.writePacketBuffers)
	return o, nil
}

func dependencies(options option.CloudflareWARPOutboundOptions) []string {
	var tags []string
	for _, tag := range []string{options.Detour, options.APIDetour} {
		if tag != "" && !common.Contains(tags, tag) {
			tags = append(tags, tag)
		}
	}
	return tags
}

func staticRegistration(options option.CloudflareWARPOutboundOptions) (*warpapi.Registration, error) {
	_, err := warpapi.ParsePrivateKey(options.PrivateKey)
	if err != nil {
		return nil, E.Cause(err, "cloudflare-warp: private_key")
	}
	registration := &warpapi.Registration{
		Version:     warpapi.RegistrationVersion,
		DeviceID:    options.DeviceID,
		AccessToken: options.AccessToken,
		PrivateKey:  options.PrivateKey,
		License:     "",
	}
	for _, prefix := range options.Address {
		if prefix.Addr().Is4() {
			registration.AddressV4 = prefix.Addr()
		} else {
			registration.AddressV6 = prefix.Addr()
		}
	}
	if options.EndpointPublicKey != "" {
		registration.EndpointPublicKey, err = warpapi.NormalizePublicKey(options.EndpointPublicKey)
		if err != nil {
			return nil, E.Cause(err, "cloudflare-warp: endpoint_public_key")
		}
	}
	return registration, nil
}

func (o *Outbound) Dependencies() []string {
	return o.dependencies
}

func (o *Outbound) Start(stage adapter.StartStage) error {
	switch stage {
	case adapter.StartStateStart:
		// The cache file service is registered after outbounds are created,
		// so it can only be looked up once the box starts.
		if o.static == nil && !o.options.Ephemeral {
			o.cacheFile = service.FromContext[adapter.CacheFile](o.ctx)
			if o.cacheFile == nil {
				return E.New("cloudflare-warp: automatic registration requires experimental.cache_file.enabled; ",
					"alternatively set ephemeral, or private_key and address from `sing-box generate warp-registration`")
			}
		}
		return o.device.Start()
	case adapter.StartStatePostStart:
		o.access.Lock()
		o.started = true
		o.access.Unlock()
		go o.run()
	}
	return nil
}

func (o *Outbound) Close() error {
	var err error
	o.closeOnce.Do(func() {
		err = o.close()
	})
	return err
}

func (o *Outbound) close() error {
	o.access.Lock()
	session := o.session
	started := o.started
	registration := o.registration
	o.access.Unlock()
	// Delete before tearing anything down so DNS that is routed through this
	// tunnel can still resolve the API host.
	o.deleteEphemeral(registration)
	o.cancel()
	if session != nil {
		session.Close()
	}
	if started {
		<-o.loopDone
	}
	return o.device.Close()
}

func (o *Outbound) InterfaceUpdated(ctx context.Context) {
	o.access.Lock()
	session := o.session
	o.access.Unlock()
	o.kicked.Store(true)
	select {
	case o.kick <- struct{}{}:
	default:
	}
	if session != nil {
		session.Close()
	}
}

func (o *Outbound) run() {
	defer close(o.loopDone)
	var (
		delay        time.Duration
		reregistered bool
		licenseDone  bool
	)
	for {
		if delay > 0 && !o.sleep(delay) {
			return
		}
		if o.ctx.Err() != nil {
			return
		}
		o.kicked.Store(false)
		registration, privateKey, err := o.loadRegistration(&licenseDone)
		if err != nil {
			if o.ctx.Err() != nil {
				return
			}
			delay = nextDelay(delay, 5*time.Second, maxAPIRetryDelay)
			o.logger.ErrorEvent("warp.register.error", "device registration failed", log.Err(err), log.Duration("retry", delay))
			continue
		}
		serverAddress := o.serverAddress(registration)
		session, err := o.connect(registration, privateKey, serverAddress)
		if err != nil {
			if o.ctx.Err() != nil {
				return
			}
			if errors.Is(err, warptransport.ErrEnrollmentRejected) {
				if o.static == nil && !reregistered {
					reregistered = true
					o.logger.WarnEvent("warp.auth.rejected", "Cloudflare rejected the cached device, registering a new one", log.Err(err))
					o.resetRegistration()
					delay = 0
					continue
				}
				delay = nextDelay(delay, 5*time.Second, maxAPIRetryDelay)
				o.logger.ErrorEvent("warp.auth.rejected", "Cloudflare rejected the device credentials", log.Err(err), log.Duration("retry", delay))
				continue
			}
			delay = nextDelay(delay, minReconnectDelay, maxReconnectDelay)
			o.logger.WarnEvent("warp.connect.error", "tunnel connection failed", log.Addr("server", serverAddress), log.Err(err), log.Duration("retry", delay))
			continue
		}
		connectedAt := time.Now()
		o.setSession(session)
		o.logger.InfoEvent("warp.connected", "tunnel connected", log.Addr("server", serverAddress))
		o.readLoop(session)
		o.clearSession(session)
		session.Close()
		if o.ctx.Err() != nil {
			return
		}
		cause := session.Err()
		switch {
		case o.kicked.Load():
			delay = 0
		case time.Since(connectedAt) >= stableSessionTime:
			delay = minReconnectDelay
		default:
			delay = nextDelay(delay, minReconnectDelay, maxReconnectDelay)
		}
		o.logger.InfoEvent("warp.disconnected", "tunnel disconnected", log.Err(cause), log.Duration("retry", delay))
	}
}

func nextDelay(current time.Duration, minimum time.Duration, maximum time.Duration) time.Duration {
	if current < minimum {
		return minimum
	}
	return min(current*2, maximum)
}

func (o *Outbound) sleep(delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-o.kick:
		return o.ctx.Err() == nil
	case <-o.ctx.Done():
		return false
	}
}

func (o *Outbound) serverAddress(registration *warpapi.Registration) M.Socksaddr {
	port := o.options.ServerPort
	if port == 0 {
		port = defaultServerPort
	}
	if o.options.Server != "" {
		return M.ParseSocksaddrHostPort(o.options.Server, port)
	}
	switch {
	case registration.EndpointV4.IsValid():
		return M.SocksaddrFrom(registration.EndpointV4, port)
	case registration.EndpointV6.IsValid():
		return M.SocksaddrFrom(registration.EndpointV6, port)
	default:
		return M.ParseSocksaddrHostPort(warpapi.DefaultEndpointV4, port)
	}
}

func (o *Outbound) connect(registration *warpapi.Registration, privateKey *ecdsa.PrivateKey, serverAddress M.Socksaddr) (warptransport.Session, error) {
	var pinnedKey *ecdsa.PublicKey
	if registration.EndpointPublicKey != "" {
		var err error
		pinnedKey, err = warpapi.ParsePublicKey(registration.EndpointPublicKey)
		if err != nil {
			return nil, E.Cause(err, "endpoint public key")
		}
	}
	insecure := o.options.TLS != nil && o.options.TLS.Insecure
	transport := warptransport.NewHTTP3Transport(warptransport.HTTP3Options{
		Dialer:     o.dialer,
		ServerAddr: serverAddress,
		TLSConfig: func() (*tls.Config, error) {
			return warptransport.NewTLSConfig(warptransport.TLSOptions{
				Base:              o.baseTLS,
				PrivateKey:        privateKey,
				EndpointPublicKey: pinnedKey,
				Insecure:          insecure,
			})
		},
		QUIC: qtls.QUICOptions{
			IdleTimeout:             time.Duration(o.options.IdleTimeout),
			KeepAlivePeriod:         time.Duration(o.options.KeepAlivePeriod),
			StreamReceiveWindow:     o.options.StreamReceiveWindow.Value(),
			ConnectionReceiveWindow: o.options.ConnectionReceiveWindow.Value(),
			InitialPacketSize:       o.options.InitialPacketSize,
			DisablePathMTUDiscovery: o.options.DisablePathMTUDiscovery,
		},
	})
	return transport.Connect(o.ctx)
}

func (o *Outbound) readLoop(session warptransport.Session) {
	for {
		packet, err := session.ReadPacket()
		if err != nil {
			return
		}
		err = o.device.WriteInboundBuffers([]*buf.Buffer{buf.As(packet)})
		if err != nil {
			o.logger.DebugEvent("warp.packet.invalid", "dropped invalid packet from tunnel", log.Err(err))
		}
	}
}

func (o *Outbound) setSession(session warptransport.Session) {
	o.access.Lock()
	defer o.access.Unlock()
	o.session = session
	close(o.readyCh)
}

func (o *Outbound) clearSession(session warptransport.Session) {
	o.access.Lock()
	defer o.access.Unlock()
	if o.session == session {
		o.session = nil
		o.readyCh = make(chan struct{})
	}
}

func (o *Outbound) currentSession() warptransport.Session {
	o.access.Lock()
	defer o.access.Unlock()
	return o.session
}

func (o *Outbound) waitReady(ctx context.Context) error {
	timer := time.NewTimer(readyWaitTimeout)
	defer timer.Stop()
	for {
		o.access.Lock()
		session := o.session
		readyCh := o.readyCh
		o.access.Unlock()
		if session != nil {
			return nil
		}
		select {
		case <-readyCh:
		case <-ctx.Done():
			return ctx.Err()
		case <-o.ctx.Done():
			return net.ErrClosed
		case <-timer.C:
			return E.New("cloudflare-warp tunnel is not connected")
		}
	}
}

func (o *Outbound) writePacketBuffers(packetBuffers []*buf.Buffer) error {
	session := o.currentSession()
	defer buf.ReleaseMulti(packetBuffers)
	if session == nil {
		return nil
	}
	for _, packetBuffer := range packetBuffers {
		err := session.WritePacket(packetBuffer)
		if err == nil {
			continue
		}
		var tooLarge *warptransport.PacketTooLargeError
		if errors.As(err, &tooLarge) {
			now := time.Now().UnixNano()
			last := o.lastTooLarge.Load()
			if now-last >= int64(tooLargeLogInterval) && o.lastTooLarge.CompareAndSwap(last, now) {
				o.logger.WarnEvent("warp.datagram.too_large", "dropped packet larger than the tunnel allows; lower mtu",
					log.Int("size", packetBuffer.Len()), log.Int("max", tooLarge.MaxPacketSize))
			}
			continue
		}
		// The session is ending; the run loop reconnects.
		return nil
	}
	return nil
}

func (o *Outbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = o.Tag()
	metadata.Destination = destination
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		adapter.LogOutboundConnection(o.logger, ctx, destination)
	case N.NetworkUDP:
		adapter.LogOutboundPacket(o.logger, ctx, destination)
	default:
		return nil, E.Extend(N.ErrUnknownNetwork, network)
	}
	err := o.waitReady(ctx)
	if err != nil {
		return nil, err
	}
	if destination.IsDomain() {
		addresses, err := o.lookup(ctx, destination.Fqdn)
		if err != nil {
			return nil, err
		}
		return N.DialSerial(ctx, o.device, network, destination, addresses)
	}
	if !destination.Addr.IsValid() {
		return nil, E.New("invalid destination: ", destination)
	}
	return o.device.DialContext(ctx, network, destination)
}

func (o *Outbound) lookup(ctx context.Context, domain string) ([]netip.Addr, error) {
	if o.dnsRouter == nil {
		return nil, E.New("missing DNS router")
	}
	addresses, err := o.dnsRouter.Lookup(ctx, domain, adapter.DNSQueryOptions{})
	if err != nil {
		return nil, err
	}
	inet4Address, inet6Address := o.device.Addresses()
	filtered := common.Filter(addresses, func(address netip.Addr) bool {
		return (address.Is4() && inet4Address.IsValid()) || (!address.Is4() && inet6Address.IsValid())
	})
	if len(filtered) == 0 {
		return nil, E.New("no address of ", domain, " is reachable through the tunnel")
	}
	return filtered, nil
}

func (o *Outbound) ListenPacketWithDestination(ctx context.Context, destination M.Socksaddr) (net.PacketConn, netip.Addr, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = o.Tag()
	metadata.Destination = destination
	adapter.LogOutboundPacket(o.logger, ctx, destination)
	err := o.waitReady(ctx)
	if err != nil {
		return nil, netip.Addr{}, err
	}
	if destination.IsDomain() {
		addresses, err := o.lookup(ctx, destination.Fqdn)
		if err != nil {
			return nil, netip.Addr{}, err
		}
		packetConn, destinationAddress, err := N.ListenSerial(ctx, o.device, destination, addresses)
		if err != nil {
			return nil, netip.Addr{}, err
		}
		return iponly.NewPacketConn(o.logger, packetConn), destinationAddress, nil
	}
	packetConn, err := o.device.ListenPacket(ctx, destination)
	if err != nil {
		return nil, netip.Addr{}, err
	}
	if destination.IsIP() {
		return iponly.NewPacketConn(o.logger, packetConn), destination.Addr, nil
	}
	return iponly.NewPacketConn(o.logger, packetConn), netip.Addr{}, nil
}

func (o *Outbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	packetConn, destinationAddress, err := o.ListenPacketWithDestination(ctx, destination)
	if err != nil {
		return nil, err
	}
	if destinationAddress.IsValid() && destination != M.SocksaddrFrom(destinationAddress, destination.Port) {
		return bufio.NewNATPacketConn(bufio.NewPacketConn(packetConn), M.SocksaddrFrom(destinationAddress, destination.Port), destination), nil
	}
	return packetConn, nil
}
