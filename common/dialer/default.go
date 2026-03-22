package dialer

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"syscall"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/listener"
	C "github.com/sagernet/sing-box/constant"
	boxLog "github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"

	"github.com/database64128/tfo-go/v2"
)

var (
	_ ParallelInterfaceDialer = (*DefaultDialer)(nil)
	_ WireGuardListener       = (*DefaultDialer)(nil)
)

type DefaultDialer struct {
	dialer4                tfo.Dialer
	dialer6                tfo.Dialer
	udpDialer4             net.Dialer
	udpDialer6             net.Dialer
	udpListener            net.ListenConfig
	udpAddr4               string
	udpAddr6               string
	netns                  string
	connectionManager      adapter.ConnectionManager
	networkManager         adapter.NetworkManager
	networkStrategy        *C.NetworkStrategy
	defaultNetworkStrategy bool
	networkType            []C.InterfaceType
	fallbackNetworkType    []C.InterfaceType
	networkFallbackDelay   time.Duration
	networkLastFallback    common.TypedValue[time.Time]
	logger                 boxLog.ContextLogger
	ipv6SourceAddressRange netip.Prefix
	ipv6SourceAddressMode  option.IPv6SourceAddressMode
}

func NewDefault(ctx context.Context, options option.DialerOptions, loggers ...boxLog.ContextLogger) (*DefaultDialer, error) {
	connectionManager := service.FromContext[adapter.ConnectionManager](ctx)
	networkManager := service.FromContext[adapter.NetworkManager](ctx)
	platformInterface := service.FromContext[adapter.PlatformInterface](ctx)
	var logger boxLog.ContextLogger
	if len(loggers) > 0 {
		logger = loggers[0]
	}
	if logger == nil {
		logger = boxLog.StdLogger()
		if contextLogger := service.FromContext[boxLog.ContextLogger](ctx); contextLogger != nil {
			logger = contextLogger
		}
	}

	var (
		dialer                 net.Dialer
		listener               net.ListenConfig
		interfaceFinder        control.InterfaceFinder
		networkStrategy        *C.NetworkStrategy
		defaultNetworkStrategy bool
		networkType            []C.InterfaceType
		fallbackNetworkType    []C.InterfaceType
		networkFallbackDelay   time.Duration
	)
	if networkManager != nil {
		interfaceFinder = networkManager.InterfaceFinder()
	} else {
		interfaceFinder = control.NewDefaultInterfaceFinder()
	}
	if options.BindInterface != "" {
		if !(C.IsLinux || C.IsDarwin || C.IsWindows) {
			return nil, E.New("`bind_interface` is only supported on Linux, macOS and Windows")
		}
		bindFunc := control.BindToInterface(interfaceFinder, options.BindInterface, -1)
		dialer.Control = control.Append(dialer.Control, bindFunc)
		listener.Control = control.Append(listener.Control, bindFunc)
	}
	if options.RoutingMark > 0 {
		if !C.IsLinux {
			return nil, E.New("`routing_mark` is only supported on Linux")
		}
		dialer.Control = control.Append(dialer.Control, setMarkWrapper(networkManager, uint32(options.RoutingMark), false))
		listener.Control = control.Append(listener.Control, setMarkWrapper(networkManager, uint32(options.RoutingMark), false))
	}
	disableDefaultBind := options.BindInterface != "" || options.Inet4BindAddress != nil || options.Inet6BindAddress != nil
	if disableDefaultBind || options.TCPFastOpen {
		if options.NetworkStrategy != nil || len(options.NetworkType) > 0 && options.FallbackNetworkType == nil && options.FallbackDelay == 0 {
			return nil, E.New("`network_strategy` is conflict with `bind_interface`, `inet4_bind_address`, `inet6_bind_address` and `tcp_fast_open`")
		}
	}

	if networkManager != nil {
		defaultOptions := networkManager.DefaultOptions()
		if defaultOptions.BindInterface != "" && !disableDefaultBind {
			bindFunc := control.BindToInterface(networkManager.InterfaceFinder(), defaultOptions.BindInterface, -1)
			dialer.Control = control.Append(dialer.Control, bindFunc)
			listener.Control = control.Append(listener.Control, bindFunc)
		} else if networkManager.AutoDetectInterface() && !disableDefaultBind {
			if platformInterface != nil {
				networkStrategy = (*C.NetworkStrategy)(options.NetworkStrategy)
				networkType = common.Map(options.NetworkType, option.InterfaceType.Build)
				fallbackNetworkType = common.Map(options.FallbackNetworkType, option.InterfaceType.Build)
				if networkStrategy == nil && len(networkType) == 0 && len(fallbackNetworkType) == 0 {
					networkStrategy = defaultOptions.NetworkStrategy
					networkType = defaultOptions.NetworkType
					fallbackNetworkType = defaultOptions.FallbackNetworkType
				}
				networkFallbackDelay = time.Duration(options.FallbackDelay)
				if networkFallbackDelay == 0 && defaultOptions.FallbackDelay != 0 {
					networkFallbackDelay = defaultOptions.FallbackDelay
				}
				if networkStrategy == nil {
					networkStrategy = common.Ptr(C.NetworkStrategyDefault)
					defaultNetworkStrategy = true
				}
				bindFunc := networkManager.ProtectFunc()
				dialer.Control = control.Append(dialer.Control, bindFunc)
				listener.Control = control.Append(listener.Control, bindFunc)
			} else {
				bindFunc := networkManager.AutoDetectInterfaceFunc()
				dialer.Control = control.Append(dialer.Control, bindFunc)
				listener.Control = control.Append(listener.Control, bindFunc)
			}
		}
		if options.RoutingMark == 0 && defaultOptions.RoutingMark != 0 {
			dialer.Control = control.Append(dialer.Control, setMarkWrapper(networkManager, defaultOptions.RoutingMark, true))
			listener.Control = control.Append(listener.Control, setMarkWrapper(networkManager, defaultOptions.RoutingMark, true))
		}
	}
	if networkManager != nil {
		markFunc := networkManager.AutoRedirectOutputMarkFunc()
		dialer.Control = control.Append(dialer.Control, markFunc)
		listener.Control = control.Append(listener.Control, markFunc)
	}
	if options.ReuseAddr {
		listener.Control = control.Append(listener.Control, control.ReuseAddr())
	}
	if options.ProtectPath != "" {
		dialer.Control = control.Append(dialer.Control, control.ProtectPath(options.ProtectPath))
		listener.Control = control.Append(listener.Control, control.ProtectPath(options.ProtectPath))
	}
	if options.BindAddressNoPort {
		if !C.IsLinux {
			return nil, E.New("`bind_address_no_port` is only supported on Linux")
		}
		dialer.Control = control.Append(dialer.Control, control.BindAddressNoPort())
	}
	if options.ConnectTimeout != 0 {
		dialer.Timeout = time.Duration(options.ConnectTimeout)
	} else {
		dialer.Timeout = C.TCPConnectTimeout
	}
	if !options.DisableTCPKeepAlive {
		var defaultKeepAlive, defaultKeepAliveInterval time.Duration
		if networkManager != nil {
			defaultOpts := networkManager.DefaultOptions()
			defaultKeepAlive = defaultOpts.TCPKeepAlive
			defaultKeepAliveInterval = defaultOpts.TCPKeepAliveInterval
		}
		keepIdle := time.Duration(options.TCPKeepAlive)
		if keepIdle == 0 {
			if defaultKeepAlive != 0 {
				keepIdle = defaultKeepAlive
			} else {
				keepIdle = C.TCPKeepAliveInitial
			}
		}
		keepInterval := time.Duration(options.TCPKeepAliveInterval)
		if keepInterval == 0 {
			if defaultKeepAliveInterval != 0 {
				keepInterval = defaultKeepAliveInterval
			} else {
				keepInterval = C.TCPKeepAliveInterval
			}
		}
		dialer.KeepAliveConfig = net.KeepAliveConfig{
			Enable:   true,
			Idle:     keepIdle,
			Interval: keepInterval,
		}
	}
	var udpFragment bool
	if options.UDPFragment != nil {
		udpFragment = *options.UDPFragment
	} else {
		udpFragment = options.UDPFragmentDefault
	}
	if !udpFragment {
		dialer.Control = control.Append(dialer.Control, control.DisableUDPFragment())
		listener.Control = control.Append(listener.Control, control.DisableUDPFragment())
	}
	var (
		dialer4    = dialer
		udpDialer4 = dialer
		udpAddr4   string
	)
	if options.Inet4BindAddress != nil {
		bindAddr := options.Inet4BindAddress.Build(netip.IPv4Unspecified())
		dialer4.LocalAddr = &net.TCPAddr{IP: bindAddr.AsSlice()}
		udpDialer4.LocalAddr = &net.UDPAddr{IP: bindAddr.AsSlice()}
		udpAddr4 = M.SocksaddrFrom(bindAddr, 0).String()
	}
	var (
		dialer6    = dialer
		udpDialer6 = dialer
		udpAddr6   string
	)
	if options.Inet6BindAddress != nil {
		bindAddr := options.Inet6BindAddress.Build(netip.IPv6Unspecified())
		dialer6.LocalAddr = &net.TCPAddr{IP: bindAddr.AsSlice()}
		udpDialer6.LocalAddr = &net.UDPAddr{IP: bindAddr.AsSlice()}
		udpAddr6 = M.SocksaddrFrom(bindAddr, 0).String()
	}
	if options.TCPMultiPath {
		dialer4.SetMultipathTCP(true)
	}
	tcpDialer4 := tfo.Dialer{Dialer: dialer4, DisableTFO: !options.TCPFastOpen}
	tcpDialer6 := tfo.Dialer{Dialer: dialer6, DisableTFO: !options.TCPFastOpen}
	var ipv6SourceAddressRange netip.Prefix
	if options.IPv6SourceAddressRange != nil {
		ipv6SourceAddressRange = options.IPv6SourceAddressRange.Build(netip.Prefix{}).Masked()
	}
	return &DefaultDialer{
		dialer4:                tcpDialer4,
		dialer6:                tcpDialer6,
		udpDialer4:             udpDialer4,
		udpDialer6:             udpDialer6,
		udpListener:            listener,
		udpAddr4:               udpAddr4,
		udpAddr6:               udpAddr6,
		netns:                  options.NetNs,
		connectionManager:      connectionManager,
		networkManager:         networkManager,
		networkStrategy:        networkStrategy,
		defaultNetworkStrategy: defaultNetworkStrategy,
		networkType:            networkType,
		fallbackNetworkType:    fallbackNetworkType,
		networkFallbackDelay:   networkFallbackDelay,
		logger:                 logger,
		ipv6SourceAddressRange: ipv6SourceAddressRange,
		ipv6SourceAddressMode:  options.IPv6SourceAddressMode,
	}, nil
}

func setMarkWrapper(networkManager adapter.NetworkManager, mark uint32, isDefault bool) control.Func {
	if networkManager == nil {
		return control.RoutingMark(mark)
	}
	return func(network, address string, conn syscall.RawConn) error {
		if networkManager.AutoRedirectOutputMark() != 0 {
			if isDefault {
				return E.New("`route.default_mark` is conflict with `tun.auto_redirect`")
			} else {
				return E.New("`routing_mark` is conflict with `tun.auto_redirect`")
			}
		}
		return control.RoutingMark(mark)(network, address, conn)
	}
}

func (d *DefaultDialer) DialContext(ctx context.Context, network string, address M.Socksaddr) (net.Conn, error) {
	if !address.IsValid() {
		return nil, E.New("invalid address")
	} else if address.IsFqdn() {
		return nil, E.New("domain not resolved")
	}
	if d.networkStrategy == nil {
		return d.trackConn(listener.ListenNetworkNamespace[net.Conn](d.netns, func() (net.Conn, error) {
			return d.dialWithOptionalIPv6SourceAddress(ctx, network, address)
		}))
	} else {
		return d.DialParallelInterface(ctx, network, address, d.networkStrategy, d.networkType, d.fallbackNetworkType, d.networkFallbackDelay)
	}
}

func (d *DefaultDialer) DialParallelInterface(ctx context.Context, network string, address M.Socksaddr, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.Conn, error) {
	if strategy == nil {
		strategy = d.networkStrategy
	}
	if strategy == nil {
		return d.DialContext(ctx, network, address)
	}
	if len(interfaceType) == 0 {
		interfaceType = d.networkType
	}
	if len(fallbackInterfaceType) == 0 {
		fallbackInterfaceType = d.fallbackNetworkType
	}
	if fallbackDelay == 0 {
		fallbackDelay = d.networkFallbackDelay
	}
	var dialer net.Dialer
	if N.NetworkName(network) == N.NetworkTCP {
		if address.IsIPv6() {
			dialer = d.dialer6.Dialer
		} else {
			dialer = d.dialer4.Dialer
		}
	} else {
		if address.IsIPv6() {
			dialer = d.udpDialer6
		} else {
			dialer = d.udpDialer4
		}
	}
	if candidate, loaded := d.selectIPv6SourceAddressCandidate(ctx, address); loaded {
		if N.NetworkName(network) == N.NetworkTCP {
			dialer.LocalAddr = &net.TCPAddr{IP: candidate.Address.AsSlice()}
		} else {
			dialer.LocalAddr = &net.UDPAddr{IP: candidate.Address.AsSlice()}
		}
		conn, err := d.dialParallelInterfaceWithDialer(ctx, dialer, network, address, strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
		if err == nil {
			return conn, nil
		}
		if isIPv6SourceAddressBindFailed(err) {
			d.warnIPv6SourceAddressFallback(ctx, boxLog.IPv6SourceAddressFallbackReasonBindFailed, candidate.Address, err)
		} else {
			return nil, err
		}
		if N.NetworkName(network) == N.NetworkTCP {
			dialer = d.dialer6.Dialer
		} else {
			dialer = d.udpDialer6
		}
		return d.dialParallelInterfaceWithDialer(ctx, dialer, network, address, strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
	}
	return d.dialParallelInterfaceWithDialer(ctx, dialer, network, address, strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
}

func (d *DefaultDialer) dialParallelInterfaceWithDialer(ctx context.Context, dialer net.Dialer, network string, address M.Socksaddr, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.Conn, error) {
	fastFallback := time.Since(d.networkLastFallback.Load()) < C.TCPTimeout
	var (
		conn      net.Conn
		isPrimary bool
		err       error
	)
	if !fastFallback {
		conn, isPrimary, err = d.dialParallelInterface(ctx, dialer, network, address.String(), *strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
	} else {
		conn, isPrimary, err = d.dialParallelInterfaceFastFallback(ctx, dialer, network, address.String(), *strategy, interfaceType, fallbackInterfaceType, fallbackDelay, d.networkLastFallback.Store)
	}
	if err != nil {
		// bind interface failed on legacy xiaomi systems
		if d.defaultNetworkStrategy && errors.Is(err, syscall.EPERM) {
			d.networkStrategy = nil
			return d.DialContext(ctx, network, address)
		} else {
			return nil, err
		}
	}
	if !fastFallback && !isPrimary {
		d.networkLastFallback.Store(time.Now())
	}
	return d.trackConn(conn, nil)
}

func (d *DefaultDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	if d.networkStrategy == nil {
		return d.trackPacketConn(listener.ListenNetworkNamespace[net.PacketConn](d.netns, func() (net.PacketConn, error) {
			return d.listenPacketWithOptionalIPv6SourceAddress(ctx, destination)
		}))
	} else {
		return d.ListenSerialInterfacePacket(ctx, destination, d.networkStrategy, d.networkType, d.fallbackNetworkType, d.networkFallbackDelay)
	}
}

func (d *DefaultDialer) DialerForICMPDestination(destination netip.Addr) net.Dialer {
	if !destination.Is6() {
		return d.dialer6.Dialer
	} else {
		return d.dialer4.Dialer
	}
}

func (d *DefaultDialer) ListenSerialInterfacePacket(ctx context.Context, destination M.Socksaddr, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.PacketConn, error) {
	if strategy == nil {
		strategy = d.networkStrategy
	}
	if strategy == nil {
		return d.ListenPacket(ctx, destination)
	}
	if len(interfaceType) == 0 {
		interfaceType = d.networkType
	}
	if len(fallbackInterfaceType) == 0 {
		fallbackInterfaceType = d.fallbackNetworkType
	}
	if fallbackDelay == 0 {
		fallbackDelay = d.networkFallbackDelay
	}
	network := N.NetworkUDP
	addr := ""
	if destination.IsIPv4() && !destination.Addr.IsUnspecified() {
		network += "4"
	}
	if candidate, loaded := d.selectIPv6SourceAddressCandidate(ctx, destination); loaded {
		addr = M.SocksaddrFrom(candidate.Address, 0).String()
		packetConn, err := d.listenSerialInterfacePacket(ctx, d.udpListener, network, addr, *strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
		if err == nil {
			return d.trackPacketConn(packetConn, nil)
		}
		if isIPv6SourceAddressBindFailed(err) {
			d.warnIPv6SourceAddressFallback(ctx, boxLog.IPv6SourceAddressFallbackReasonBindFailed, candidate.Address, err)
		} else {
			return nil, err
		}
	}
	packetConn, err := d.listenSerialInterfacePacket(ctx, d.udpListener, network, "", *strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
	if err != nil {
		// bind interface failed on legacy xiaomi systems
		if d.defaultNetworkStrategy && errors.Is(err, syscall.EPERM) {
			d.networkStrategy = nil
			return d.ListenPacket(ctx, destination)
		} else {
			return nil, err
		}
	}
	return d.trackPacketConn(packetConn, nil)
}

func (d *DefaultDialer) dialWithOptionalIPv6SourceAddress(ctx context.Context, network string, address M.Socksaddr) (net.Conn, error) {
	if candidate, loaded := d.selectIPv6SourceAddressCandidate(ctx, address); loaded {
		connection, err := d.dialWithSelectedIPv6SourceAddress(ctx, network, address, candidate.Address)
		if err == nil {
			return connection, nil
		}
		if isIPv6SourceAddressBindFailed(err) {
			d.warnIPv6SourceAddressFallback(ctx, boxLog.IPv6SourceAddressFallbackReasonBindFailed, candidate.Address, err)
		} else {
			return nil, err
		}
	}
	return d.dialWithoutSelectedIPv6SourceAddress(ctx, network, address)
}

func (d *DefaultDialer) dialWithoutSelectedIPv6SourceAddress(ctx context.Context, network string, address M.Socksaddr) (net.Conn, error) {
	switch N.NetworkName(network) {
	case N.NetworkUDP:
		if !address.IsIPv6() {
			return d.udpDialer4.DialContext(ctx, network, address.String())
		}
		return d.udpDialer6.DialContext(ctx, network, address.String())
	}
	if !address.IsIPv6() {
		return DialSlowContext(&d.dialer4, ctx, network, address)
	}
	return DialSlowContext(&d.dialer6, ctx, network, address)
}

func (d *DefaultDialer) dialWithSelectedIPv6SourceAddress(ctx context.Context, network string, address M.Socksaddr, selectedAddress netip.Addr) (net.Conn, error) {
	switch N.NetworkName(network) {
	case N.NetworkUDP:
		udpDialer := d.udpDialer6
		udpDialer.LocalAddr = &net.UDPAddr{IP: selectedAddress.AsSlice()}
		return udpDialer.DialContext(ctx, network, address.String())
	}
	tcpDialer := d.dialer6
	tcpDialer.Dialer.LocalAddr = &net.TCPAddr{IP: selectedAddress.AsSlice()}
	return DialSlowContext(&tcpDialer, ctx, network, address)
}

func (d *DefaultDialer) listenPacketWithOptionalIPv6SourceAddress(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	if candidate, loaded := d.selectIPv6SourceAddressCandidate(ctx, destination); loaded {
		packetConn, err := d.udpListener.ListenPacket(ctx, N.NetworkUDP, M.SocksaddrFrom(candidate.Address, 0).String())
		if err == nil {
			return packetConn, nil
		}
		if isIPv6SourceAddressBindFailed(err) {
			d.warnIPv6SourceAddressFallback(ctx, boxLog.IPv6SourceAddressFallbackReasonBindFailed, candidate.Address, err)
		} else {
			return nil, err
		}
	}
	if destination.IsIPv6() {
		return d.udpListener.ListenPacket(ctx, N.NetworkUDP, d.udpAddr6)
	}
	if destination.IsIPv4() && !destination.Addr.IsUnspecified() {
		return d.udpListener.ListenPacket(ctx, N.NetworkUDP+"4", d.udpAddr4)
	}
	return d.udpListener.ListenPacket(ctx, N.NetworkUDP, d.udpAddr4)
}

func (d *DefaultDialer) selectIPv6SourceAddressCandidate(ctx context.Context, destination M.Socksaddr) (IPv6SourceAddressCandidate, bool) {
	if !destination.IsIPv6() || !d.ipv6SourceAddressRange.IsValid() {
		return IPv6SourceAddressCandidate{}, false
	}
	candidate, loaded := SelectIPv6SourceAddressCandidate(d.ipv6SourceAddressRange, d.ipv6SourceAddressMode, adapter.ContextFrom(ctx))
	if !loaded || !candidate.Address.IsValid() {
		d.warnIPv6SourceAddressFallback(ctx, boxLog.IPv6SourceAddressFallbackReasonNoCandidate, netip.Addr{}, nil)
		return IPv6SourceAddressCandidate{}, false
	}
	return candidate, true
}

func (d *DefaultDialer) warnIPv6SourceAddressFallback(ctx context.Context, reason string, selectedAddress netip.Addr, err error) {
	if d.logger == nil || !d.ipv6SourceAddressRange.IsValid() {
		return
	}
	boxLog.WarnIPv6SourceAddressFallback(
		d.logger,
		ctx,
		d.ipv6SourceAddressRange.String(),
		string(d.ipv6SourceAddressMode),
		reason,
		selectedAddress,
		err,
	)
}

func isIPv6SourceAddressBindFailed(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EADDRNOTAVAIL) || errors.Is(err, syscall.EAFNOSUPPORT) || errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.EPERM) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return opErr.Op == "bind"
	}
	return false
}

func (d *DefaultDialer) WireGuardControl() control.Func {
	return d.udpListener.Control
}

func (d *DefaultDialer) TCPDialer6() net.Dialer {
	return d.dialer6.Dialer
}

func (d *DefaultDialer) UDPDialer6() net.Dialer {
	return d.udpDialer6
}

func (d *DefaultDialer) UDPListenerConfig() net.ListenConfig {
	return d.udpListener
}

func (d *DefaultDialer) NetNs() string {
	return d.netns
}

func (d *DefaultDialer) trackConn(conn net.Conn, err error) (net.Conn, error) {
	if d.connectionManager == nil || err != nil {
		return conn, err
	}
	return d.connectionManager.TrackConn(conn), nil
}

func (d *DefaultDialer) trackPacketConn(conn net.PacketConn, err error) (net.PacketConn, error) {
	if d.connectionManager == nil || err != nil {
		return conn, err
	}
	return d.connectionManager.TrackPacketConn(conn), nil
}
