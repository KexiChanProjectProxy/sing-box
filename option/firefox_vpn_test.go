package option

import (
	"context"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/service"
	"github.com/stretchr/testify/require"
)

type firefoxVPNTestOutboundRegistry struct{}

func (firefoxVPNTestOutboundRegistry) OptionTypes() []string {
	return []string{C.TypeTor, C.TypeFirefoxVPN}
}

func (firefoxVPNTestOutboundRegistry) CreateOptions(outboundType string) (any, bool) {
	switch outboundType {
	case C.TypeTor:
		return new(TorOutboundOptions), true
	case C.TypeFirefoxVPN:
		return new(FirefoxVPNOutboundOptions), true
	default:
		return nil, false
	}
}

func TestFirefoxVPNOptionsRegistryCharacterization(t *testing.T) {
	t.Parallel()

	ctx := service.ContextWith[OutboundOptionsRegistry](context.Background(), firefoxVPNTestOutboundRegistry{})
	var outbound Outbound
	err := json.UnmarshalContext(ctx, []byte(`{"type":"firefox-vpn","tag":"fx-vpn-out","email":"user@example.com","password":"pw","server":"vpn.example","server_port":443}`), &outbound)
	require.NoError(t, err)
	require.Equal(t, C.TypeFirefoxVPN, outbound.Type)
	require.IsType(t, &FirefoxVPNOutboundOptions{}, outbound.Options)
}

func TestFirefoxVPNOptionsRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := service.ContextWith[OutboundOptionsRegistry](context.Background(), firefoxVPNTestOutboundRegistry{})
	var outbound Outbound
	err := json.UnmarshalContext(ctx, []byte(`{
		"type": "firefox-vpn",
		"tag": "fx-vpn-out",
		"api_detour": "direct-api",
		"detour": "connect-detour",
		"email": "user@example.com",
		"password": "correct horse battery staple",
		"server": "203.0.113.10",
		"server_port": 443,
		"tls": {"enabled": true}
	}`), &outbound)
	require.NoError(t, err)
	require.Equal(t, C.TypeFirefoxVPN, outbound.Type)

	options, ok := outbound.Options.(*FirefoxVPNOutboundOptions)
	require.True(t, ok)
	require.Equal(t, "direct-api", options.APIDetour)
	require.Equal(t, "connect-detour", options.Detour)
	require.Equal(t, "user@example.com", options.Email)
	require.Equal(t, "correct horse battery staple", options.Password)
	require.Equal(t, "203.0.113.10", options.Server)
	require.Equal(t, uint16(443), options.ServerPort)
	require.NotNil(t, options.TLS)
	require.True(t, options.TLS.Enabled)

	data, err := json.MarshalContext(ctx, &outbound)
	require.NoError(t, err)

	var roundTripped Outbound
	err = json.UnmarshalContext(ctx, data, &roundTripped)
	require.NoError(t, err)
	require.Equal(t, outbound.Type, roundTripped.Type)
	require.Equal(t, outbound.Tag, roundTripped.Tag)
	require.Equal(t, outbound.Options, roundTripped.Options)
}

func TestFirefoxVPNValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		jsonContent string
		errorText   string
	}{
		{
			name:        "missing email",
			jsonContent: `{"password":"pw","server":"vpn.example","server_port":443}`,
			errorText:   "firefox-vpn outbound: email is required",
		},
		{
			name:        "missing password",
			jsonContent: `{"email":"user@example.com","server":"vpn.example","server_port":443}`,
			errorText:   "firefox-vpn outbound: password is required",
		},
		{
			name:        "missing server",
			jsonContent: `{"email":"user@example.com","password":"pw","server_port":443}`,
			errorText:   "firefox-vpn outbound: server is required",
		},
		{
			name:        "missing server port",
			jsonContent: `{"email":"user@example.com","password":"pw","server":"vpn.example"}`,
			errorText:   "firefox-vpn outbound: server_port is required and must be non-zero",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var options FirefoxVPNOutboundOptions
			err := json.Unmarshal([]byte(testCase.jsonContent), &options)
			require.Error(t, err)
			require.Contains(t, err.Error(), testCase.errorText)
		})
	}
}

func TestFirefoxVPNValidationRejectsPacketMode(t *testing.T) {
	t.Parallel()

	var options FirefoxVPNOutboundOptions
	err := json.Unmarshal([]byte(`{
		"email": "user@example.com",
		"password": "pw",
		"server": "vpn.example",
		"server_port": 443,
		"network": "udp"
	}`), &options)
	require.Error(t, err)
	require.Contains(t, err.Error(), "firefox-vpn outbound: packet network modes are not supported")
}
