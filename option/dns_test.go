package option

import (
	"context"
	"testing"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/service"

	"github.com/stretchr/testify/require"
)

type stubDNSTransportOptionsRegistry struct{}

func (stubDNSTransportOptionsRegistry) OptionTypes() []string {
	return []string{C.DNSTypeUDP, C.DNSTypeFakeIP}
}

func (stubDNSTransportOptionsRegistry) CreateOptions(transportType string) (any, bool) {
	switch transportType {
	case C.DNSTypeUDP:
		return new(RemoteDNSServerOptions), true
	case C.DNSTypeFakeIP:
		return new(FakeIPDNSServerOptions), true
	default:
		return nil, false
	}
}

func TestDNSOptionsRejectsLegacyFakeIPOptions(t *testing.T) {
	t.Parallel()

	ctx := service.ContextWith[DNSTransportOptionsRegistry](context.Background(), stubDNSTransportOptionsRegistry{})
	var options DNSOptions
	err := json.UnmarshalContext(ctx, []byte(`{
		"fakeip": {
			"enabled": true,
			"inet4_range": "198.18.0.0/15"
		}
	}`), &options)
	require.EqualError(t, err, legacyDNSFakeIPRemovedMessage)
}

func TestDNSServerOptionsRejectsLegacyFormats(t *testing.T) {
	t.Parallel()

	ctx := service.ContextWith[DNSTransportOptionsRegistry](context.Background(), stubDNSTransportOptionsRegistry{})
	testCases := []string{
		`{"address":"1.1.1.1"}`,
		`{"type":"legacy","address":"1.1.1.1"}`,
	}
	for _, content := range testCases {
		var options DNSServerOptions
		err := json.UnmarshalContext(ctx, []byte(content), &options)
		require.EqualError(t, err, legacyDNSServerRemovedMessage)
	}
}

func TestOptimisticDNSOptionsMarshal(t *testing.T) {
	t.Parallel()

	var options OptimisticDNSOptions
	require.NoError(t, json.Unmarshal([]byte(`true`), &options))
	require.True(t, options.Enabled)
	content, err := json.Marshal(options)
	require.NoError(t, err)
	require.JSONEq(t, `true`, string(content))

	options = OptimisticDNSOptions{}
	require.NoError(t, json.Unmarshal([]byte(`{"enabled":true,"client_timeout":"200ms"}`), &options))
	require.EqualValues(t, 200*time.Millisecond, options.ClientTimeout)
	content, err = json.Marshal(options)
	require.NoError(t, err)
	require.JSONEq(t, `{"enabled":true,"client_timeout":"200ms"}`, string(content))
}

func TestDNSRuleActionCacheOptions(t *testing.T) {
	t.Parallel()

	for _, action := range []string{`"action":"route","server":"a"`, `"action":"evaluate","server":"a"`, `"action":"route-options"`} {
		var rule DNSRuleAction
		err := json.UnmarshalContext(context.Background(), []byte(`{`+action+`,"min_ttl":60,"max_ttl":600,"optimistic":{"enabled":true,"timeout":"1h"},"prefetch":false}`), &rule)
		require.NoError(t, err, action)

		err = json.UnmarshalContext(context.Background(), []byte(`{`+action+`,"min_ttl":600,"max_ttl":60}`), &DNSRuleAction{})
		require.ErrorContains(t, err, "min_ttl", action)

		err = json.UnmarshalContext(context.Background(), []byte(`{`+action+`,"optimistic":true,"disable_optimistic_cache":true}`), &DNSRuleAction{})
		require.ErrorContains(t, err, "mutually exclusive", action)
	}
}
