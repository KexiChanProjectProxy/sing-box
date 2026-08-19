package firefoxvpn

import (
	"context"
	"errors"
	"net"
	"os"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/dialer"
	boxTLS "github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func RegisterOutbound(registry *outbound.Registry) {
	outbound.Register[option.FirefoxVPNOutboundOptions](registry, C.TypeFirefoxVPN, NewOutbound)
}

type Outbound struct {
	outbound.Adapter
	ctx            context.Context
	logger         log.StructuredLogger
	dialer         N.Dialer
	serverAddr     M.Socksaddr
	tlsConfig      boxTLS.Config
	authController *AuthController
	dependencies   []string

	sessionMu       sync.RWMutex
	session         *Session
	retiredSessions []*Session
	closed          bool
}

func NewOutbound(ctx context.Context, _ adapter.Router, logger log.StructuredLogger, tag string, options option.FirefoxVPNOutboundOptions) (adapter.Outbound, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	if options.TLS == nil || !options.TLS.Enabled {
		return nil, C.ErrTLSRequired
	}
	outboundDialer, err := dialer.NewWithOptions(dialer.Options{
		Context:        ctx,
		Options:        options.DialerOptions,
		RemoteIsDomain: options.ServerIsDomain(),
	})
	if err != nil {
		return nil, err
	}
	tlsConfig, err := boxTLS.NewClient(ctx, logger, options.Server, common.PtrValueOrDefault(options.TLS))
	if err != nil {
		return nil, err
	}
	authController, err := newAuthControllerWithLogger(ctx, logger, options, nil)
	if err != nil {
		return nil, err
	}
	return &Outbound{
		Adapter:        newFirefoxAdapter(tag, options),
		ctx:            ctx,
		logger:         logger,
		dialer:         outboundDialer,
		serverAddr:     options.ServerOptions.Build(),
		tlsConfig:      tlsConfig,
		authController: authController,
		dependencies:   newFirefoxDependencies(options),
	}, nil
}

func newFirefoxAdapter(tag string, options option.FirefoxVPNOutboundOptions) outbound.Adapter {
	return outbound.NewAdapter(C.TypeFirefoxVPN, tag, []string{N.NetworkTCP}, newFirefoxDependencies(options))
}

func newFirefoxDependencies(options option.FirefoxVPNOutboundOptions) []string {
	dependencies := make([]string, 0, 2)
	appendDependency := func(tag string) {
		if tag == "" {
			return
		}
		for _, dependency := range dependencies {
			if dependency == tag {
				return
			}
		}
		dependencies = append(dependencies, tag)
	}
	appendDependency(options.DialerOptions.Detour)
	appendDependency(options.APIDetour)
	return dependencies
}

func (o *Outbound) Dependencies() []string {
	return o.dependencies
}

func (o *Outbound) Start() error {
	return o.authController.Start(o.ctx)
}

func (o *Outbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if N.NetworkName(network) != N.NetworkTCP {
		return nil, E.Extend(N.ErrUnknownNetwork, network)
	}
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = o.Tag()
	metadata.Destination = destination
	proxyPass, err := o.authController.GetProxyPass(ctx)
	if err != nil {
		return nil, err
	}
	session, err := o.sessionForToken(proxyPass.Token)
	if err != nil {
		return nil, err
	}
	adapter.LogOutboundConnection(o.logger, ctx, destination)
	return session.DialContext(ctx, destination)
}

func (o *Outbound) ListenPacket(_ context.Context, _ M.Socksaddr) (net.PacketConn, error) {
	return nil, os.ErrInvalid
}

func (o *Outbound) Close() error {
	o.sessionMu.Lock()
	if o.closed {
		o.sessionMu.Unlock()
		return nil
	}
	o.closed = true
	currentSession := o.session
	retiredSessions := o.retiredSessions
	o.session = nil
	o.retiredSessions = nil
	o.sessionMu.Unlock()

	var closeErr error
	if o.authController != nil {
		closeErr = errors.Join(closeErr, o.authController.Close())
	}
	if currentSession != nil {
		closeErr = errors.Join(closeErr, currentSession.Close())
	}
	for _, retiredSession := range retiredSessions {
		closeErr = errors.Join(closeErr, retiredSession.Close())
	}
	return closeErr
}

func (o *Outbound) sessionForToken(proxyPassToken string) (*Session, error) {
	o.sessionMu.RLock()
	if o.closed {
		o.sessionMu.RUnlock()
		return nil, net.ErrClosed
	}
	if o.session != nil && o.session.ProxyPassToken() == proxyPassToken {
		session := o.session
		o.sessionMu.RUnlock()
		return session, nil
	}
	o.sessionMu.RUnlock()

	newSession := NewSession(SessionOptions{
		ServerAddr:           o.serverAddr,
		TLSConfig:            o.tlsConfig,
		Dialer:               o.dialer,
		ProxyPassToken:       proxyPassToken,
		DialTimeout:          defaultSessionDialTimeout,
		StreamTimeout:        defaultSessionStreamTimeout,
		MaxConcurrentStreams: defaultSessionMaxConcurrentStreams,
	})

	o.sessionMu.Lock()
	defer o.sessionMu.Unlock()
	if o.closed {
		_ = newSession.Close()
		return nil, net.ErrClosed
	}
	if o.session != nil && o.session.ProxyPassToken() == proxyPassToken {
		_ = newSession.Close()
		return o.session, nil
	}
	if o.session != nil {
		o.retiredSessions = append(o.retiredSessions, o.session)
	}
	o.session = newSession
	return newSession, nil
}
