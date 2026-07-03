package firefoxvpn

import (
	"context"
	"os"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	protocolfirefoxvpn "github.com/sagernet/sing-box/protocol/firefoxvpn"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
	"github.com/stretchr/testify/require"
)

func TestFirefoxVPNListenPacketRejected(t *testing.T) {
	t.Parallel()

	ob := newTestOutbound(t)
	_, err := ob.(*protocolfirefoxvpn.Outbound).ListenPacket(context.Background(), M.Socksaddr{})
	require.ErrorIs(t, err, os.ErrInvalid)
}

func TestFirefoxVPNDialContextRejectsNonTCP(t *testing.T) {
	t.Parallel()

	for _, network := range []string{"udp", "ip", "udp4", "udp6"} {
		network := network
		t.Run(network, func(t *testing.T) {
			t.Parallel()

			ob := newTestOutbound(t)
			_, err := ob.(*protocolfirefoxvpn.Outbound).DialContext(context.Background(), network, M.Socksaddr{})
			require.ErrorIs(t, err, N.ErrUnknownNetwork)
		})
	}
}

// newTestOutbound constructs a minimal Outbound without starting it,
// suitable for testing the TCP-only guardrails at the API surface.
func newTestOutbound(t *testing.T) adapter.Outbound {
	t.Helper()

	ctx := service.ContextWith[adapter.OutboundManager](context.Background(), &stubOutboundManager{})
	raw, err := protocolfirefoxvpn.NewOutbound(ctx, nil, log.NewNOPFactory().Logger(), "firefox-vpn", option.FirefoxVPNOutboundOptions{
		DialerOptions: option.DialerOptions{Detour: "direct"},
		ServerOptions: option.ServerOptions{
			Server:     "vpn.example.test",
			ServerPort: 443,
		},
		APIDetour: "direct",
		Email:     "user@example.com",
		Password:  "correct horse battery staple",
		OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
			TLS: &option.OutboundTLSOptions{Enabled: true, Insecure: true},
		},
	})
	require.NoError(t, err)
	return raw
}
