package uot

import (
	"context"
	"net"
	"net/netip"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/uot"
)

var _ adapter.ConnectionRouterEx = (*Router)(nil)

type Router struct {
	router adapter.ConnectionRouterEx
	logger log.StructuredLogger
}

func NewRouter(router adapter.ConnectionRouterEx, logger log.StructuredLogger) *Router {
	return &Router{router, logger}
}

func (r *Router) RouteConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	switch metadata.Destination.Fqdn {
	case uot.MagicAddress:
		request, err := uot.ReadRequest(conn)
		if err != nil {
			return E.Cause(err, "read UoT request")
		}
		metadata.Domain = metadata.Destination.Fqdn
		metadata.Destination = request.Destination
		logUoTPacket(r.logger, ctx, metadata, request.IsConnect, false)
		return r.router.RoutePacketConnection(ctx, uot.NewConn(conn, *request), metadata)
	case uot.LegacyMagicAddress:
		metadata.Domain = metadata.Destination.Fqdn
		metadata.Destination = M.Socksaddr{Addr: netip.IPv4Unspecified()}
		logUoTPacket(r.logger, ctx, metadata, false, true)
		return r.RoutePacketConnection(ctx, uot.NewConn(conn, uot.Request{}), metadata)
	}
	return r.router.RouteConnection(ctx, conn, metadata)
}

func (r *Router) RoutePacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return r.router.RoutePacketConnection(ctx, conn, metadata)
}

func (r *Router) RouteConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	switch metadata.Destination.Fqdn {
	case uot.MagicAddress:
		request, err := uot.ReadRequest(conn)
		if err != nil {
			err = E.Cause(err, "UoT read request")
			adapter.LogConnectionError(r.logger, ctx, err, metadata.Source)

			N.CloseOnHandshakeFailure(conn, onClose, err)
			return
		}
		metadata.Domain = metadata.Destination.Fqdn
		metadata.Destination = request.Destination
		logUoTPacket(r.logger, ctx, metadata, request.IsConnect, false)
		r.router.RoutePacketConnectionEx(ctx, uot.NewConn(conn, *request), metadata, onClose)
		return
	case uot.LegacyMagicAddress:
		metadata.Domain = metadata.Destination.Fqdn
		metadata.Destination = M.Socksaddr{Addr: netip.IPv4Unspecified()}
		logUoTPacket(r.logger, ctx, metadata, false, true)
		r.RoutePacketConnectionEx(ctx, uot.NewConn(conn, uot.Request{}), metadata, onClose)
		return
	}
	r.router.RouteConnectionEx(ctx, conn, metadata, onClose)
}

func (r *Router) RoutePacketConnectionEx(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	r.router.RoutePacketConnectionEx(ctx, conn, metadata, onClose)
}

func logUoTPacket(logger log.StructuredLogger, ctx context.Context, metadata adapter.InboundContext, connect, legacy bool) {
	fields := []log.Field{log.Bool("uot", true)}
	if metadata.Source.IsValid() {
		fields = append(fields, log.Addr("source", metadata.Source))
	}
	if metadata.Destination.IsValid() {
		fields = append(fields, log.Addr("destination", metadata.Destination))
	}
	fields = append(fields, log.OptionalString("user", metadata.User)...)
	if metadata.Network != "" {
		fields = append(fields, log.String("network", metadata.Network))
	}
	if legacy {
		fields = append(fields, log.Bool("legacy", true))
	} else {
		fields = append(fields, log.Bool("connect", connect))
	}
	logger.InfoEventContext(ctx, "inbound.packet", "inbound packet connection", fields...)
}
