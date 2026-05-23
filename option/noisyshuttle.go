package option

import (
	"time"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json/badoption"
)

type NoisyShuttleInboundOptions struct {
	ListenOptions
	Users           []NoisyShuttleUser                 `json:"users,omitempty"`
	InboundTLSOptionsContainer
	Fallback        *ServerOptions                     `json:"fallback,omitempty"`
	Network         NetworkList                        `json:"network,omitempty"`
	Session         NoisyShuttleSessionOptions         `json:"session,omitempty"`
	Handshake       NoisyShuttleInboundHandshakeOptions `json:"handshake,omitempty"`
	UDPTimeout      badoption.Duration                 `json:"udp_timeout,omitempty"`
	UDPMaxPacketSize int                               `json:"udp_max_packet_size,omitempty"`
}

type NoisyShuttleUser struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type NoisyShuttleOutboundOptions struct {
	DialerOptions
	ServerOptions
	Password         string                         `json:"password"`
	Network          NetworkList                    `json:"network,omitempty"`
	OutboundTLSOptionsContainer
	Session          NoisyShuttleSessionOptions      `json:"session,omitempty"`
	Handshake        NoisyShuttleOutboundHandshakeOptions `json:"handshake,omitempty"`
	UDPTimeout       badoption.Duration              `json:"udp_timeout,omitempty"`
	UDPMaxPacketSize int                            `json:"udp_max_packet_size,omitempty"`
}

type NoisyShuttleSessionOptions struct {
	Enabled           bool               `json:"enabled,omitempty"`
	MaxStreams        uint16             `json:"max_streams,omitempty"`
	MaxRequests       uint32             `json:"max_requests,omitempty"`
	IdleTimeout       badoption.Duration `json:"idle_timeout,omitempty"`
	MaxAge            badoption.Duration `json:"max_age,omitempty"`
	KeepaliveInterval badoption.Duration `json:"keepalive_interval,omitempty"`
	KeepaliveTimeout  badoption.Duration `json:"keepalive_timeout,omitempty"`
}

type NoisyShuttleInboundHandshakeOptions struct {
	MaxPadding  uint16             `json:"max_padding,omitempty"`
	AuthTimeout badoption.Duration `json:"auth_timeout,omitempty"`
}

type NoisyShuttleOutboundHandshakeOptions struct {
	PaddingMin  uint16             `json:"padding_min,omitempty"`
	PaddingMax  uint16             `json:"padding_max,omitempty"`
	AuthTimeout badoption.Duration `json:"auth_timeout,omitempty"`
}

func (o *NoisyShuttleInboundOptions) Validate() error {
	if len(o.Users) == 0 {
		return E.New("noisy-shuttle inbound: users is required and must contain at least one user")
	}
	for i, user := range o.Users {
		if user.Password == "" {
			return E.New("noisy-shuttle inbound: users[", i, "].password is required")
		}
	}
	if o.Handshake.MaxPadding > 4096 {
		return E.New("noisy-shuttle inbound: handshake.max_padding must be <= 4096")
	}
	if o.Handshake.AuthTimeout != 0 && (o.Handshake.AuthTimeout.Build() < time.Second || o.Handshake.AuthTimeout.Build() > 60*time.Second) {
		return E.New("noisy-shuttle inbound: handshake.auth_timeout must be between 1s and 60s, or 0 to use default")
	}
	return nil
}

func (o *NoisyShuttleOutboundOptions) Validate() error {
	if o.Password == "" {
		return E.New("noisy-shuttle outbound: password is required")
	}
	if o.ServerOptions.ServerPort == 0 {
		return E.New("noisy-shuttle outbound: server_port is required and must be non-zero")
	}
	if o.Handshake.PaddingMin > 4096 {
		return E.New("noisy-shuttle outbound: handshake.padding_min must be <= 4096")
	}
	if o.Handshake.PaddingMax > 4096 {
		return E.New("noisy-shuttle outbound: handshake.padding_max must be <= 4096")
	}
	if o.Handshake.AuthTimeout != 0 && (o.Handshake.AuthTimeout.Build() < time.Second || o.Handshake.AuthTimeout.Build() > 60*time.Second) {
		return E.New("noisy-shuttle outbound: handshake.auth_timeout must be between 1s and 60s, or 0 to use default")
	}
	return nil
}

func (o *NoisyShuttleSessionOptions) Validate() error {
	if o.Enabled && o.MaxStreams == 0 {
		return E.New("noisy-shuttle session: max_streams must be >= 1 when session is enabled")
	}
	if o.Enabled && o.MaxStreams > 65535 {
		return E.New("noisy-shuttle session: max_streams must be <= 65535")
	}
	if o.Enabled {
		if o.KeepaliveInterval != 0 && o.KeepaliveInterval.Build() < time.Second {
			return E.New("noisy-shuttle session: keepalive_interval must be >= 1s when session is enabled")
		}
	} else {
		if o.KeepaliveInterval != 0 {
			return E.New("noisy-shuttle session: keepalive_interval must be 0 when session is disabled")
		}
	}
	if o.IdleTimeout != 0 && o.IdleTimeout.Build() < time.Second {
		return E.New("noisy-shuttle session: idle_timeout must be >= 1s or 0 (disabled)")
	}
	if o.MaxAge != 0 && o.MaxAge.Build() < time.Second {
		return E.New("noisy-shuttle session: max_age must be >= 1s or 0 (disabled)")
	}
	if o.IdleTimeout != 0 && o.MaxAge != 0 && o.IdleTimeout.Build() > o.MaxAge.Build() {
		return E.New("noisy-shuttle session: idle_timeout must be <= max_age")
	}
	return nil
}
