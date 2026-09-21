package adapter

import "time"

// TransportStatsDirection selects whose observation of a connection is read.
type TransportStatsDirection int

const (
	// TransportStatsClient is measured locally and describes the client to
	// server direction.
	TransportStatsClient TransportStatsDirection = iota
	// TransportStatsServer is measured by the proxy server and reported back
	// over the protocol, and describes the server to client direction.
	TransportStatsServer
)

func (d TransportStatsDirection) String() string {
	if d == TransportStatsServer {
		return "server"
	}
	return "client"
}

// TransportStatsReader reads transport quality metrics aggregated over a
// sliding time window. Every reader returns false when it has no sample for the
// requested window, which a caller must treat as missing rather than as zero.
//
// RTT and RTT variance are in milliseconds, loss rate is a percentage, and
// delivery rate is in Mbps.
type TransportStatsReader interface {
	LossRate(window time.Duration) (float64, bool)
	RTT(window time.Duration) (float64, bool)
	RTTVar(window time.Duration) (float64, bool)
	DeliveryRate(window time.Duration) (float64, bool)
}

// OutboundWithTransportStats is implemented by outbounds whose protocol can
// report transport quality metrics for their own connections.
//
// Collection is off by default: a group outbound calls EnableTransportStats for
// the directions it ranks on, and only then does the outbound sample its
// connections or negotiate reporting with the server. Enabling a direction that
// the outbound cannot serve is not an error; TransportStats simply keeps
// returning nil for it.
type OutboundWithTransportStats interface {
	Outbound
	EnableTransportStats(direction TransportStatsDirection)
	TransportStats(direction TransportStatsDirection) TransportStatsReader
}
