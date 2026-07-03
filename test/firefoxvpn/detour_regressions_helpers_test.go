package firefoxvpn

import (
	"context"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks"
	"github.com/stretchr/testify/require"
)

type stubOutboundManager struct{}

func (m *stubOutboundManager) Start(_ adapter.StartStage) error           { return nil }
func (m *stubOutboundManager) Close() error                               { return nil }
func (m *stubOutboundManager) Outbounds() []adapter.Outbound              { return nil }
func (m *stubOutboundManager) Outbound(_ string) (adapter.Outbound, bool) { return nil, false }
func (m *stubOutboundManager) Default() adapter.Outbound                  { return nil }
func (m *stubOutboundManager) Remove(_ string) error                      { return nil }
func (m *stubOutboundManager) Create(_ context.Context, _ adapter.Router, _ log.StructuredLogger, _ string, _ string, _ any) error {
	return nil
}

func newFirefoxVPNOutboundOptions(detour string, apiDetour string, server string, port uint16, serverName string) option.FirefoxVPNOutboundOptions {
	return option.FirefoxVPNOutboundOptions{
		DialerOptions:               option.DialerOptions{Detour: detour},
		ServerOptions:               option.ServerOptions{Server: server, ServerPort: port},
		APIDetour:                   apiDetour,
		Email:                       "user@example.com",
		Password:                    "correct horse battery staple",
		OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: &option.OutboundTLSOptions{Enabled: true, Insecure: true, ServerName: serverName}},
	}
}

func firefoxVPNOutbound(tag string, outboundOptions option.FirefoxVPNOutboundOptions) option.Outbound {
	return option.Outbound{Type: C.TypeFirefoxVPN, Tag: tag, Options: &outboundOptions}
}

func firefoxVPNHarnessOptions(mixedPort uint16, outboundOptions option.FirefoxVPNOutboundOptions) option.Options {
	return option.Options{
		Inbounds: []option.Inbound{{
			Type: C.TypeMixed,
			Tag:  "mixed-in",
			Options: &option.HTTPMixedInboundOptions{ListenOptions: option.ListenOptions{
				Listen:     common.Ptr(badoption.Addr(netip.MustParseAddr("127.0.0.1"))),
				ListenPort: mixedPort,
			}},
		}},
		Outbounds: []option.Outbound{
			directOutbound("direct"),
			directOutbound("api-upstream"),
			directOutbound("data-upstream"),
			{Type: C.TypeFirefoxVPN, Tag: "firefox-vpn", Options: &outboundOptions},
		},
		Route: &option.RouteOptions{Rules: []option.Rule{inboundRouteRule("mixed-in", "firefox-vpn")}, Final: "direct"},
	}
}

func mixedFirefoxVPNOptions(mixedPort uint16, outbounds ...option.Outbound) option.Options {
	return option.Options{
		Inbounds: []option.Inbound{{
			Type: C.TypeMixed,
			Tag:  "mixed-in",
			Options: &option.HTTPMixedInboundOptions{ListenOptions: option.ListenOptions{
				Listen:     common.Ptr(badoption.Addr(netip.MustParseAddr("127.0.0.1"))),
				ListenPort: mixedPort,
			}},
		}},
		Outbounds: outbounds,
		Route:     &option.RouteOptions{Rules: []option.Rule{inboundRouteRule("mixed-in", "firefox-vpn")}, Final: "direct"},
	}
}

func socksDetourOutbound(tag string, port uint16) option.Outbound {
	return option.Outbound{
		Type: C.TypeSOCKS,
		Tag:  tag,
		Options: &option.SOCKSOutboundOptions{
			DialerOptions: option.DialerOptions{ConnectTimeout: badoption.Duration(5 * time.Second)},
			ServerOptions: option.ServerOptions{Server: "127.0.0.1", ServerPort: port},
		},
	}
}

func roundTripThroughFirefoxVPN(t *testing.T, mixedPort uint16, destination string, payload string) {
	t.Helper()
	client := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", mixedPort), socks.Version5, "", "")
	conn, err := client.DialContext(t.Context(), N.NetworkTCP, M.ParseSocksaddr(destination))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	_, err = conn.Write([]byte(payload))
	require.NoError(t, err)
	response := make([]byte, len(payload))
	_, err = io.ReadFull(conn, response)
	require.NoError(t, err)
	require.Equal(t, payload, string(response))
}

func assertNoSecretsOnDisk(t *testing.T, root string, secrets []string) {
	t.Helper()
	files := make([]string, 0)
	require.NoError(t, filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		require.NoError(t, err)
		if entry.IsDir() {
			return nil
		}
		files = append(files, path)
		contents, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		for _, secret := range secrets {
			if secret == "" {
				continue
			}
			require.NotContains(t, string(contents), secret)
		}
		return nil
	}))
	require.Empty(t, files)
}

func netPortString(port uint16) string {
	return strconv.Itoa(int(port))
}
