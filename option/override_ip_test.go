package option

import (
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common/json"
	"github.com/stretchr/testify/require"
)

func TestOverrideIPStringStrategy(t *testing.T) {
	t.Parallel()

	var options DialerOptions
	err := json.Unmarshal([]byte(`{"override_ip": "ipv4_only"}`), &options)
	require.NoError(t, err)
	require.NotNil(t, options.OverrideIP)
	require.Equal(t, DomainStrategy(C.DomainStrategyIPv4Only), options.OverrideIP.Strategy)
	require.Empty(t, options.OverrideIP.Server)
}

func TestOverrideIPObjectWithoutServer(t *testing.T) {
	t.Parallel()

	var options DialerOptions
	err := json.Unmarshal([]byte(`{"override_ip": {"strategy": "prefer_ipv6"}}`), &options)
	require.NoError(t, err)
	require.NotNil(t, options.OverrideIP)
	require.Equal(t, DomainStrategy(C.DomainStrategyPreferIPv6), options.OverrideIP.Strategy)
	require.Empty(t, options.OverrideIP.Server)
}

func TestOverrideIPObjectWithServer(t *testing.T) {
	t.Parallel()

	var options DialerOptions
	err := json.Unmarshal([]byte(`{"override_ip": {"strategy": "ipv6_only", "server": "local", "disable_cache": true}}`), &options)
	require.NoError(t, err)
	require.NotNil(t, options.OverrideIP)
	require.Equal(t, DomainStrategy(C.DomainStrategyIPv6Only), options.OverrideIP.Strategy)
	require.Equal(t, "local", options.OverrideIP.Server)
	require.True(t, options.OverrideIP.DisableCache)
}

func TestOverrideIPMissingStrategy(t *testing.T) {
	t.Parallel()

	var options DialerOptions
	err := json.Unmarshal([]byte(`{"override_ip": {}}`), &options)
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing override_ip.strategy")
}

func TestOverrideIPEmptyString(t *testing.T) {
	t.Parallel()

	var options DialerOptions
	err := json.Unmarshal([]byte(`{"override_ip": ""}`), &options)
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty override_ip strategy")
}

func TestOverrideIPUnknownStrategy(t *testing.T) {
	t.Parallel()

	var options DialerOptions
	err := json.Unmarshal([]byte(`{"override_ip": "as_is"}`), &options)
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty override_ip strategy")
}

func TestOverrideIPInvalidStrategy(t *testing.T) {
	t.Parallel()

	var options DialerOptions
	err := json.Unmarshal([]byte(`{"override_ip": "prefer_ip"}`), &options)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown domain strategy")
}

func TestOverrideIPMarshalString(t *testing.T) {
	t.Parallel()

	options := OverrideIPOptions{Strategy: DomainStrategy(C.DomainStrategyPreferIPv4)}
	raw, err := json.Marshal(options)
	require.NoError(t, err)
	require.Equal(t, `"prefer_ipv4"`, string(raw))
}

func TestOverrideIPMarshalObject(t *testing.T) {
	t.Parallel()

	options := OverrideIPOptions{
		Server:   "local",
		Strategy: DomainStrategy(C.DomainStrategyIPv4Only),
	}
	raw, err := json.Marshal(options)
	require.NoError(t, err)
	require.JSONEq(t, `{"server":"local","strategy":"ipv4_only"}`, string(raw))
}

func TestOverrideIPConflictWithPreferDomain(t *testing.T) {
	t.Parallel()

	err := CheckDestinationOverride(true, &OverrideIPOptions{Strategy: DomainStrategy(C.DomainStrategyIPv4Only)})
	require.Error(t, err)
	require.Contains(t, err.Error(), "mutually exclusive")

	require.NoError(t, CheckDestinationOverride(true, nil))
	require.NoError(t, CheckDestinationOverride(false, &OverrideIPOptions{Strategy: DomainStrategy(C.DomainStrategyIPv4Only)}))
}

func TestOverrideIPSelectorOutboundOptions(t *testing.T) {
	t.Parallel()

	var options SelectorOutboundOptions
	err := json.Unmarshal([]byte(`{"override_ip": "prefer_ipv4"}`), &options)
	require.NoError(t, err)
	require.NotNil(t, options.OverrideIP)
	require.Equal(t, DomainStrategy(C.DomainStrategyPreferIPv4), options.OverrideIP.Strategy)
}

func TestOverrideIPURLTestOutboundOptions(t *testing.T) {
	t.Parallel()

	var options URLTestOutboundOptions
	err := json.Unmarshal([]byte(`{"override_ip": {"strategy": "ipv4_only"}}`), &options)
	require.NoError(t, err)
	require.NotNil(t, options.OverrideIP)
	require.Equal(t, DomainStrategy(C.DomainStrategyIPv4Only), options.OverrideIP.Strategy)
}

func TestOverrideIPStubOptions(t *testing.T) {
	t.Parallel()

	var options StubOptions
	err := json.Unmarshal([]byte(`{"override_ip": "ipv6_only"}`), &options)
	require.NoError(t, err)
	require.NotNil(t, options.OverrideIP)
	require.Equal(t, DomainStrategy(C.DomainStrategyIPv6Only), options.OverrideIP.Strategy)
}
