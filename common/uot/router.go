package uot

import (
	"context"
	"net"
	"net/netip"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
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
		if request.IsConnect {
			r.logger.InfoEventContext(ctx, "common.uot.message", F.ToString("inbound UoT connect connection to ", request.Destination))

		} else {
			r.logger.InfoEventContext(ctx, "common.uot.message", F.ToString("inbound UoT connection to ", request.Destination))

		}
		metadata.Domain = metadata.Destination.Fqdn
		metadata.Destination = request.Destination
		return r.router.RoutePacketConnection(ctx, uot.NewConn(conn, *request), metadata)
	case uot.LegacyMagicAddress:
		r.logger.InfoEventContext(ctx, "common.uot.message", F.ToString("inbound legacy UoT connection"))

		metadata.Domain = metadata.Destination.Fqdn
		metadata.Destination = M.Socksaddr{Addr: netip.IPv4Unspecified()}
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
			r.logger.ErrorEventContext(ctx, "common.uot.message", F.ToString("process connection from ", metadata.Source, ": ", err))

			N.CloseOnHandshakeFailure(conn, onClose, err)
			return
		}
		if request.IsConnect {
			r.logger.InfoEventContext(ctx, "common.uot.message", F.ToString("inbound UoT connect connection to ", request.Destination))

		} else {
			r.logger.InfoEventContext(ctx, "common.uot.message", F.ToString("inbound UoT connection to ", request.Destination))

		}
		metadata.Domain = metadata.Destination.Fqdn
		metadata.Destination = request.Destination
		r.router.RoutePacketConnectionEx(ctx, uot.NewConn(conn, *request), metadata, onClose)
		return
	case uot.LegacyMagicAddress:
		r.logger.InfoEventContext(ctx, "common.uot.message", F.ToString("inbound legacy UoT connection"))

		metadata.Domain = metadata.Destination.Fqdn
		metadata.Destination = M.Socksaddr{Addr: netip.IPv4Unspecified()}
		r.RoutePacketConnectionEx(ctx, uot.NewConn(conn, uot.Request{}), metadata, onClose)
		return
	}
	r.router.RouteConnectionEx(ctx, conn, metadata, onClose)
}

func (r *Router) RoutePacketConnectionEx(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	r.router.RoutePacketConnectionEx(ctx, conn, metadata, onClose)
}
