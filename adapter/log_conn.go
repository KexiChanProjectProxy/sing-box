package adapter

import (
	"context"

	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
)

func appendAddr(fields []log.Field, key string, addr M.Socksaddr) []log.Field {
	if !addr.IsValid() {
		return fields
	}
	return append(fields, log.Addr(key, addr))
}

func inboundConnectionFields(metadata InboundContext) []log.Field {
	fields := appendAddr(nil, "source", metadata.Source)
	fields = appendAddr(fields, "destination", metadata.Destination)
	fields = append(fields, log.OptionalString("user", metadata.User)...)
	if metadata.Network != "" {
		fields = append(fields, log.String("network", metadata.Network))
	}
	return fields
}

func LogInboundConnection(logger log.StructuredLogger, ctx context.Context, metadata InboundContext) {
	logger.InfoEventContext(ctx, "inbound.connection", "inbound connection", inboundConnectionFields(metadata)...)
}

func LogInboundPacket(logger log.StructuredLogger, ctx context.Context, metadata InboundContext) {
	logger.InfoEventContext(ctx, "inbound.packet", "inbound packet connection", inboundConnectionFields(metadata)...)
}

func LogOutboundConnection(logger log.StructuredLogger, ctx context.Context, destination M.Socksaddr) {
	logger.InfoEventContext(ctx, "outbound.connection", "outbound connection", log.Addr("destination", destination), log.String("network", "tcp"))
}

func LogOutboundPacket(logger log.StructuredLogger, ctx context.Context, destination M.Socksaddr) {
	logger.InfoEventContext(ctx, "outbound.packet", "outbound packet connection", log.Addr("destination", destination), log.String("network", "udp"))
}

func LogConnectionError(logger log.StructuredLogger, ctx context.Context, err error, source M.Socksaddr) {
	fields := []log.Field{log.Err(err)}
	fields = appendAddr(fields, "source", source)
	if E.IsClosedOrCanceled(err) {
		logger.DebugEventContext(ctx, "connection.closed", "connection closed", fields...)
		return
	}
	logger.ErrorEventContext(ctx, "connection.error", "process connection", fields...)
}
