package option

import (
	"context"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/service"

	"github.com/stretchr/testify/require"
)

type cloudflareWARPTestOutboundRegistry struct{}

func (cloudflareWARPTestOutboundRegistry) OptionTypes() []string {
	return []string{C.TypeCloudflareWARP}
}

func (cloudflareWARPTestOutboundRegistry) CreateOptions(outboundType string) (any, bool) {
	if outboundType == C.TypeCloudflareWARP {
		return new(CloudflareWARPOutboundOptions), true
	}
	return nil, false
}

func TestCloudflareWARPOptionsRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := service.ContextWith[OutboundOptionsRegistry](context.Background(), cloudflareWARPTestOutboundRegistry{})
	var outbound Outbound
	err := json.UnmarshalContext(ctx, []byte(`{
		"type": "cloudflare-warp",
		"tag": "warp",
		"server": "162.159.198.1",
		"server_port": 443,
		"private_key": "MHcCAQEE",
		"address": ["172.16.0.2/32", "2606:4700:110:8a36::1/128"],
		"endpoint_public_key": "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE",
		"device_id": "device",
		"access_token": "token",
		"license": "license",
		"api_detour": "direct",
		"mtu": 1280,
		"keep_alive_period": "30s",
		"tls": {"server_name": "consumer-masque.cloudflareclient.com"}
	}`), &outbound)
	require.NoError(t, err)
	options, ok := outbound.Options.(*CloudflareWARPOutboundOptions)
	require.True(t, ok)
	require.True(t, options.StaticMode())
	require.Len(t, options.Address, 2)
	require.Equal(t, "direct", options.APIDetour)
	require.Equal(t, uint32(1280), options.MTU)

	data, err := json.MarshalContext(ctx, &outbound)
	require.NoError(t, err)
	var roundTripped Outbound
	err = json.UnmarshalContext(ctx, data, &roundTripped)
	require.NoError(t, err)
	roundTrippedData, err := json.MarshalContext(ctx, &roundTripped)
	require.NoError(t, err)
	require.JSONEq(t, string(data), string(roundTrippedData))
	require.Equal(t, options.PrivateKey, roundTripped.Options.(*CloudflareWARPOutboundOptions).PrivateKey)
}

func TestCloudflareWARPOptionsAutoMode(t *testing.T) {
	t.Parallel()

	var options CloudflareWARPOutboundOptions
	err := json.Unmarshal([]byte(`{"license":"abc","access_jwt":"jwt","device_name":"box","ephemeral":true}`), &options)
	require.NoError(t, err)
	require.False(t, options.StaticMode())
}

func TestCloudflareWARPValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		jsonContent string
		errorText   string
	}{
		{"key without address", `{"private_key":"k"}`, "private_key and address must be set together"},
		{"address without key", `{"address":"172.16.0.2/32"}`, "private_key and address must be set together"},
		{"device id without token", `{"private_key":"k","address":"172.16.0.2/32","device_id":"d"}`, "device_id and access_token must be set together"},
		{"device id in auto mode", `{"device_id":"d","access_token":"t"}`, "device_id and access_token require private_key and address"},
		{"license without device", `{"private_key":"k","address":"172.16.0.2/32","license":"l"}`, "license requires device_id and access_token"},
		{"ephemeral in static mode", `{"private_key":"k","address":"172.16.0.2/32","ephemeral":true}`, "ephemeral is only used for automatic registration"},
		{"jwt in static mode", `{"private_key":"k","address":"172.16.0.2/32","access_jwt":"j"}`, "access_jwt is only used for automatic registration"},
		{"non host address", `{"private_key":"k","address":"172.16.0.0/24"}`, "address must be a single host prefix"},
		{"two ipv4 addresses", `{"private_key":"k","address":["172.16.0.2/32","172.16.0.3/32"]}`, "at most one IPv4 address"},
		{"bad mtu", `{"mtu":100}`, "mtu must be between"},
		{"utls", `{"tls":{"utls":{"enabled":true}}}`, "tls.utls is not supported over QUIC"},
		{"reality", `{"tls":{"reality":{"enabled":true}}}`, "tls.reality is not supported over QUIC"},
		{"alpn", `{"tls":{"alpn":"h2"}}`, "tls.alpn is fixed to h3"},
		{"client cert", `{"tls":{"client_key_path":"/k"}}`, "tls client certificates are generated from private_key"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			var options CloudflareWARPOutboundOptions
			err := json.Unmarshal([]byte(testCase.jsonContent), &options)
			require.Error(t, err)
			require.Contains(t, err.Error(), testCase.errorText)
		})
	}
}
