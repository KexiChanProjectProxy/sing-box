package noisyshuttle

import (
	"net/netip"
	"time"

	"github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/json/badoption"
)

const (
	DefaultPassword = "test-password"
	DefaultServer   = "127.0.0.1"
)

func NoisyShuttleInboundConfig(tag string, port uint16, password string, certPem, keyPem string) option.Inbound {
	return option.Inbound{
		Type: constant.TypeNoisyShuttle,
		Tag:  tag,
		Options: &option.NoisyShuttleInboundOptions{
			ListenOptions: option.ListenOptions{
				Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
				ListenPort: port,
			},
			Users: []option.NoisyShuttleUser{
				{Name: "testuser", Password: password},
			},
			InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
				TLS: &option.InboundTLSOptions{
					Enabled:         true,
					ServerName:      "example.org",
					CertificatePath: certPem,
					KeyPath:         keyPem,
				},
			},
			Network: "tcp",
			Session: option.NoisyShuttleSessionOptions{
				Enabled:           true,
				MaxStreams:        16,
				MaxRequests:       0,
				IdleTimeout:       badoption.Duration(5 * time.Minute),
				MaxAge:            0,
				KeepaliveInterval: badoption.Duration(30 * time.Second),
			},
			Handshake: option.NoisyShuttleInboundHandshakeOptions{
				MaxPadding:  256,
				AuthTimeout: badoption.Duration(5 * time.Second),
			},
		},
	}
}

func NoisyShuttleOutboundConfig(tag, server string, port uint16, password string, certPem string) option.Outbound {
	return option.Outbound{
		Type: constant.TypeNoisyShuttle,
		Tag:  tag,
		Options: &option.NoisyShuttleOutboundOptions{
			ServerOptions: option.ServerOptions{
				Server:     server,
				ServerPort: port,
			},
			Password: password,
			Network:  "tcp",
			OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
				TLS: &option.OutboundTLSOptions{
					Enabled:         true,
					ServerName:      "example.org",
					CertificatePath: certPem,
				},
			},
			Session: option.NoisyShuttleSessionOptions{
				Enabled:           true,
				MaxStreams:        16,
				MaxRequests:       0,
				IdleTimeout:       badoption.Duration(5 * time.Minute),
				KeepaliveInterval: badoption.Duration(30 * time.Second),
			},
		},
	}
}

func MixedInboundConfig(tag string, port uint16) option.Inbound {
	return option.Inbound{
		Type: constant.TypeMixed,
		Tag:  tag,
		Options: &option.HTTPMixedInboundOptions{
			ListenOptions: option.ListenOptions{
				Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
				ListenPort: port,
			},
		},
	}
}

func DirectOutboundConfig(tag string) option.Outbound {
	return option.Outbound{
		Type: constant.TypeDirect,
		Tag:  tag,
	}
}

func BuildNoisyShuttleTestOptions(clientPort, serverPort uint16, certPem, keyPem string) option.Options {
	return option.Options{
		Inbounds: []option.Inbound{
			MixedInboundConfig("mixed-in", clientPort),
		},
		Outbounds: []option.Outbound{
			NoisyShuttleOutboundConfig("ns-out", DefaultServer, serverPort, DefaultPassword, certPem),
			DirectOutboundConfig("direct"),
		},
	}
}

func BuildNoisyShuttleInboundOnlyOptions(serverPort uint16, password, certPem, keyPem string) option.Options {
	return option.Options{
		Inbounds: []option.Inbound{
			NoisyShuttleInboundConfig("ns-in", serverPort, password, certPem, keyPem),
		},
		Outbounds: []option.Outbound{
			DirectOutboundConfig("direct"),
		},
	}
}

func BuildNoisyShuttleOutboundOnlyOptions(clientPort, serverPort uint16, certPem string) option.Options {
	return option.Options{
		Inbounds: []option.Inbound{
			MixedInboundConfig("mixed-in", clientPort),
		},
		Outbounds: []option.Outbound{
			NoisyShuttleOutboundConfig("ns-out", DefaultServer, serverPort, DefaultPassword, certPem),
		},
	}
}
