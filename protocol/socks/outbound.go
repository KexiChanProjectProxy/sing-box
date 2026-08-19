package socks

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/uot"
	"github.com/sagernet/sing/protocol/socks"
	"github.com/sagernet/sing/service"
)

func RegisterOutbound(registry *outbound.Registry) {
	outbound.Register[option.SOCKSOutboundOptions](registry, C.TypeSOCKS, NewOutbound)
}

var _ adapter.Outbound = (*Outbound)(nil)

type Outbound struct {
	outbound.Adapter
	dnsRouter adapter.DNSRouter
	logger    log.StructuredLogger
	client    *socks.Client
	resolve   bool
	uotClient *uot.Client
}

func NewOutbound(ctx context.Context, router adapter.Router, logger log.StructuredLogger, tag string, options option.SOCKSOutboundOptions) (adapter.Outbound, error) {
	var version socks.Version
	var err error
	if options.Version != "" {
		version, err = socks.ParseVersion(options.Version)
	} else {
		version = socks.Version5
	}
	if err != nil {
		return nil, err
	}
	outboundDialer, err := dialer.New(ctx, options.DialerOptions, options.ServerIsDomain())
	if err != nil {
		return nil, err
	}
	outbound := &Outbound{
		Adapter:   outbound.NewAdapterWithDialerOptions(C.TypeSOCKS, tag, options.Network.Build(), options.DialerOptions),
		dnsRouter: service.FromContext[adapter.DNSRouter](ctx),
		logger:    logger,
		client:    socks.NewClient(outboundDialer, options.ServerOptions.Build(), version, options.Username, options.Password),
		resolve:   version == socks.Version4,
	}
	uotOptions := common.PtrValueOrDefault(options.UDPOverTCP)
	if uotOptions.Enabled {
		outbound.uotClient = &uot.Client{
			Dialer:  outbound.client,
			Version: uotOptions.Version,
		}
	}
	return outbound, nil
}

func (h *Outbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = h.Tag()
	metadata.Destination = destination
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		adapter.LogOutboundConnection(h.logger, ctx, destination)

	case N.NetworkUDP:
		if h.uotClient != nil {
			adapter.LogOutboundPacket(h.logger, ctx, destination)

			return h.uotClient.DialContext(ctx, network, destination)
		}
		adapter.LogOutboundPacket(h.logger, ctx, destination)

	default:
		return nil, E.Extend(N.ErrUnknownNetwork, network)
	}
	if h.resolve && destination.IsDomain() {
		destinationAddresses, err := h.dnsRouter.Lookup(ctx, destination.Fqdn, adapter.DNSQueryOptions{})
		if err != nil {
			return nil, err
		}
		return N.DialSerial(ctx, h.client, network, destination, destinationAddresses)
	}
	return h.client.DialContext(ctx, network, destination)
}

func (h *Outbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = h.Tag()
	metadata.Destination = destination
	if h.uotClient != nil {
		adapter.LogOutboundPacket(h.logger, ctx, destination)

		return h.uotClient.ListenPacket(ctx, destination)
	}
	if h.resolve && destination.IsDomain() {
		destinationAddresses, err := h.dnsRouter.Lookup(ctx, destination.Fqdn, adapter.DNSQueryOptions{})
		if err != nil {
			return nil, err
		}
		packetConn, _, err := N.ListenSerial(ctx, h.client, destination, destinationAddresses)
		if err != nil {
			return nil, err
		}
		return packetConn, nil
	}
	adapter.LogOutboundPacket(h.logger, ctx, destination)

	return h.client.ListenPacket(ctx, destination)
}
