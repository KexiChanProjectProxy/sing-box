package connlog

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	F "github.com/sagernet/sing/common/format"
)

// LogInboundConnection logs an inbound TCP connection with structured event data
// This emits a ConnectionEvent with action="start" and direction="inbound"
func LogInboundConnection(logger log.ContextLogger, ctx context.Context, metadata *adapter.InboundContext) {
	event := log.NewConnectionEvent("inbound", "start").
		WithSource(metadata.Source).
		WithDestination(metadata.Destination).
		WithNetwork(metadata.Network)

	if metadata.Inbound != "" {
		event.WithInbound(metadata.Inbound, metadata.InboundType)
	}
	if metadata.User != "" {
		event.WithUser(metadata.User)
	}
	if metadata.Protocol != "" {
		event.WithProtocol(metadata.Protocol, metadata.Client)
	}
	if metadata.Domain != "" {
		event.WithDestination(metadata.Destination)
	}
	if len(metadata.DestinationAddresses) > 0 {
		addrs := make([]string, len(metadata.DestinationAddresses))
		for i, addr := range metadata.DestinationAddresses {
			addrs[i] = addr.String()
		}
		event.WithDestAddresses(addrs)
	}

	message := "inbound connection from "
	if metadata.Source.IsValid() {
		message += F.ToString(metadata.Source.Addr, ":", metadata.Source.Port)
	}
	if metadata.Destination.IsIP() {
		message += F.ToString(" to ", metadata.Destination.Addr, ":", metadata.Destination.Port)
	} else if metadata.Destination.IsFqdn() {
		message += F.ToString(" to ", metadata.Destination.Fqdn, ":", metadata.Destination.Port)
	}
	if metadata.Domain != "" {
		message += F.ToString(" (domain: ", metadata.Domain, ")")
	}

	log.WithConnectionEvent(logger, ctx, log.LevelDebug, event, message)
}

// LogInboundPacketConnection logs an inbound UDP packet connection with structured event data
// This emits a ConnectionEvent with action="start" and direction="inbound"
func LogInboundPacketConnection(logger log.ContextLogger, ctx context.Context, metadata *adapter.InboundContext) {
	event := log.NewConnectionEvent("inbound", "start").
		WithSource(metadata.Source).
		WithDestination(metadata.Destination).
		WithNetwork(metadata.Network)

	if metadata.Inbound != "" {
		event.WithInbound(metadata.Inbound, metadata.InboundType)
	}
	if metadata.User != "" {
		event.WithUser(metadata.User)
	}
	if metadata.Protocol != "" {
		event.WithProtocol(metadata.Protocol, metadata.Client)
	}
	if metadata.Domain != "" {
		event.WithDestination(metadata.Destination)
	}
	if len(metadata.DestinationAddresses) > 0 {
		addrs := make([]string, len(metadata.DestinationAddresses))
		for i, addr := range metadata.DestinationAddresses {
			addrs[i] = addr.String()
		}
		event.WithDestAddresses(addrs)
	}

	message := "inbound packet connection from "
	if metadata.Source.IsValid() {
		message += F.ToString(metadata.Source.Addr, ":", metadata.Source.Port)
	}
	if metadata.Destination.IsIP() {
		message += F.ToString(" to ", metadata.Destination.Addr, ":", metadata.Destination.Port)
	} else if metadata.Destination.IsFqdn() {
		message += F.ToString(" to ", metadata.Destination.Fqdn, ":", metadata.Destination.Port)
	}
	if metadata.Domain != "" {
		message += F.ToString(" (domain: ", metadata.Domain, ")")
	}

	log.WithConnectionEvent(logger, ctx, log.LevelDebug, event, message)
}
