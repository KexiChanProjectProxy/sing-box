package option

import "github.com/sagernet/sing/common/json/badoption"

type AnyTLSInboundOptions struct {
	ListenOptions
	InboundTLSOptionsContainer
	Users         []AnyTLSUser               `json:"users,omitempty"`
	PaddingScheme badoption.Listable[string] `json:"padding_scheme,omitempty"`
	Fallback      *ServerOptions             `json:"fallback,omitempty"`
	DNS           *AnyTLSInboundDNSOptions   `json:"dns,omitempty"`
}

// AnyTLSInboundDNSOptions lets clients send DNS queries inside their session.
type AnyTLSInboundDNSOptions struct {
	Enabled bool `json:"enabled,omitempty"`
	// Server is the tag of the DNS server that answers, bypassing the DNS
	// rules. Empty sends queries through the DNS rules.
	Server string `json:"server,omitempty"`
}

// AnyTLSDNSServerOptions sends queries to the server of an AnyTLS outbound
// inside its sessions.
type AnyTLSDNSServerOptions struct {
	Outbound string `json:"outbound,omitempty"`
}

type AnyTLSUser struct {
	Name     string `json:"name,omitempty"`
	Password string `json:"password,omitempty"`
}

type AnyTLSOutboundOptions struct {
	DialerOptions
	ServerOptions
	ServerUnixOptions
	OutboundTLSOptionsContainer
	Password                    string             `json:"password,omitempty"`
	IdleSessionCheckInterval    badoption.Duration `json:"idle_session_check_interval,omitempty"`
	IdleSessionTimeout          badoption.Duration `json:"idle_session_timeout,omitempty"`
	MinIdleSession              int                `json:"min_idle_session,omitempty"`
	EnsureIdleSession           int                `json:"ensure_idle_session,omitempty"`
	MinIdleSessionForAge        int                `json:"min_idle_session_for_age,omitempty"`
	EnsureIdleSessionCreateRate int                `json:"ensure_idle_session_create_rate,omitempty"`
	Heartbeat                   badoption.Duration `json:"heartbeat,omitempty"`
	MaxConnectionLifetime       badoption.Duration `json:"max_connection_lifetime,omitempty"`
	ConnectionLifetimeJitter    badoption.Duration `json:"connection_lifetime_jitter,omitempty"`
	ClientMetadata              string             `json:"client_metadata,omitempty"`
	CloseAfterIdle              badoption.Duration `json:"close_after_idle,omitempty"`
	CloseAfterLifetime          badoption.Duration `json:"close_after_lifetime,omitempty"`
}
