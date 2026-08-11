package http

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	F "github.com/sagernet/sing/common/format"
	"net"
	std_http "net/http"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	sHTTP "github.com/sagernet/sing/protocol/http"
)

func RegisterOutbound(registry *outbound.Registry) {
	outbound.Register[option.HTTPOutboundOptions](registry, C.TypeHTTP, NewOutbound)
	outbound.Register[option.HTTPDynamicOutboundOptions](registry, C.TypeHTTPDynamic, NewDynamicOutbound)
}

type Outbound struct {
	outbound.Adapter
	logger log.StructuredLogger
	client *sHTTP.Client
}

func NewOutbound(ctx context.Context, router adapter.Router, logger log.StructuredLogger, tag string, options option.HTTPOutboundOptions) (adapter.Outbound, error) {
	outboundDialer, err := dialer.New(ctx, options.DialerOptions, options.ServerIsDomain())
	if err != nil {
		return nil, err
	}
	detour, err := tls.NewDialerFromOptions(ctx, logger, outboundDialer, options.Server, common.PtrValueOrDefault(options.TLS))
	if err != nil {
		return nil, err
	}
	return &Outbound{
		Adapter: outbound.NewAdapterWithDialerOptions(C.TypeHTTP, tag, []string{N.NetworkTCP}, options.DialerOptions),
		logger:  logger,
		client: sHTTP.NewClient(sHTTP.Options{
			Dialer:   detour,
			Server:   options.ServerOptions.Build(),
			Username: options.Username,
			Password: options.Password,
			Path:     options.Path,
			Headers:  options.Headers.Build(),
		}),
	}, nil
}

func (h *Outbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = h.Tag()
	metadata.Destination = destination
	h.logger.InfoEventContext(ctx, "protocol.message", F.ToString("outbound connection to ", destination))

	return h.client.DialContext(ctx, network, destination)
}

func (h *Outbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, os.ErrInvalid
}

type DynamicOutbound struct {
	outbound.Adapter
	logger   log.StructuredLogger
	dialer   N.Dialer
	server   M.Socksaddr
	username string
	path     string
	headers  std_http.Header
}

func NewDynamicOutbound(ctx context.Context, router adapter.Router, logger log.StructuredLogger, tag string, options option.HTTPDynamicOutboundOptions) (adapter.Outbound, error) {
	if options.Username == "" {
		return nil, E.New("http-dynamic outbound: username is required")
	}
	outboundDialer, err := dialer.New(ctx, options.DialerOptions, options.ServerIsDomain())
	if err != nil {
		return nil, err
	}
	detour, err := tls.NewDialerFromOptions(ctx, logger, outboundDialer, options.Server, common.PtrValueOrDefault(options.TLS))
	if err != nil {
		return nil, err
	}
	return &DynamicOutbound{
		Adapter:  outbound.NewAdapterWithDialerOptions(C.TypeHTTPDynamic, tag, []string{N.NetworkTCP}, options.DialerOptions),
		logger:   logger,
		dialer:   detour,
		server:   options.ServerOptions.Build(),
		username: options.Username,
		path:     options.Path,
		headers:  options.Headers.Build(),
	}, nil
}

func (h *DynamicOutbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = h.Tag()
	metadata.Destination = destination
	h.logger.InfoEventContext(ctx, "protocol.message", F.ToString("outbound connection to ", destination))

	password, err := dynamicHTTPPassword(metadata)
	if err != nil {
		return nil, err
	}
	client := sHTTP.NewClient(sHTTP.Options{
		Dialer:   h.dialer,
		Server:   h.server,
		Username: h.username,
		Password: password,
		Path:     h.path,
		Headers:  h.headers.Clone(),
	})
	return client.DialContext(ctx, network, destination)
}

func (h *DynamicOutbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, os.ErrInvalid
}

func dynamicHTTPPassword(metadata *adapter.InboundContext) (string, error) {
	if metadata == nil || metadata.User == "" {
		return "", E.New("http-dynamic outbound: inbound username is required")
	}
	if !metadata.Source.Addr.IsValid() {
		return "", E.New("http-dynamic outbound: inbound source IP is required")
	}
	sum := sha256.Sum256([]byte(metadata.User + metadata.Source.Addr.String()))
	var password [16]byte
	hex.Encode(password[:], sum[:8])
	return string(password[:]), nil
}
