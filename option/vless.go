package option

import (
	"github.com/sagernet/sing/common/json/badoption"
)

type VLESSConnectionPoolOptions struct {
	// Pool sizing
	EnsureIdleSession           int                `json:"ensure_idle_session,omitempty"`
	EnsureIdleSessionCreateRate int                `json:"ensure_idle_session_create_rate,omitempty"`
	MinIdleSession              int                `json:"min_idle_session,omitempty"`
	MinIdleSessionForAge        int                `json:"min_idle_session_for_age,omitempty"`

	// Timeouts
	IdleSessionCheckInterval badoption.Duration `json:"idle_session_check_interval,omitempty"`
	IdleSessionTimeout       badoption.Duration `json:"idle_session_timeout,omitempty"`

	// Lifetime rotation
	MaxConnectionLifetime    badoption.Duration `json:"max_connection_lifetime,omitempty"`
	ConnectionLifetimeJitter badoption.Duration `json:"connection_lifetime_jitter,omitempty"`

	// Heartbeat (TCP-level, not VLESS protocol)
	Heartbeat badoption.Duration `json:"heartbeat,omitempty"`
}

type VLESSInboundOptions struct {
	ListenOptions
	Users []VLESSUser `json:"users,omitempty"`
	InboundTLSOptionsContainer
	Multiplex *InboundMultiplexOptions `json:"multiplex,omitempty"`
	Transport *V2RayTransportOptions   `json:"transport,omitempty"`
}

type VLESSUser struct {
	Name string `json:"name"`
	UUID string `json:"uuid"`
	Flow string `json:"flow,omitempty"`
}

type VLESSOutboundOptions struct {
	DialerOptions
	ServerOptions
	UUID    string      `json:"uuid"`
	Flow    string      `json:"flow,omitempty"`
	Network NetworkList `json:"network,omitempty"`
	OutboundTLSOptionsContainer
	Multiplex      *OutboundMultiplexOptions   `json:"multiplex,omitempty"`
	Transport      *V2RayTransportOptions      `json:"transport,omitempty"`
	PacketEncoding *string                     `json:"packet_encoding,omitempty"`
	ConnectionPool *VLESSConnectionPoolOptions `json:"connection_pool,omitempty"`
}
