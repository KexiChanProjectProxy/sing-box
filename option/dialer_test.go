package option

import (
	"testing"

	"github.com/sagernet/sing/common/json"
	"github.com/stretchr/testify/require"
)

func TestPreferDomainDialerOptions(t *testing.T) {
	t.Parallel()

	var options DialerOptions
	err := json.Unmarshal([]byte(`{"prefer_domain": true}`), &options)
	require.NoError(t, err)
	require.True(t, options.PreferDomain)
}

func TestPreferDomainSelectorOutboundOptions(t *testing.T) {
	t.Parallel()

	var options SelectorOutboundOptions
	err := json.Unmarshal([]byte(`{"prefer_domain": true}`), &options)
	require.NoError(t, err)
	require.True(t, options.PreferDomain)
}

func TestPreferDomainURLTestOutboundOptions(t *testing.T) {
	t.Parallel()

	var options URLTestOutboundOptions
	err := json.Unmarshal([]byte(`{"prefer_domain": true}`), &options)
	require.NoError(t, err)
	require.True(t, options.PreferDomain)
}

func TestPreferDomainStubOptions(t *testing.T) {
	t.Parallel()

	var options StubOptions
	err := json.Unmarshal([]byte(`{"prefer_domain": true}`), &options)
	require.NoError(t, err)
	require.True(t, options.PreferDomain)
}

func TestPreferDomainFalseExplicit(t *testing.T) {
	t.Parallel()

	var dialer DialerOptions
	err := json.Unmarshal([]byte(`{"prefer_domain": false}`), &dialer)
	require.NoError(t, err)
	require.False(t, dialer.PreferDomain)

	var selector SelectorOutboundOptions
	err = json.Unmarshal([]byte(`{"prefer_domain": false}`), &selector)
	require.NoError(t, err)
	require.False(t, selector.PreferDomain)

	var urlTest URLTestOutboundOptions
	err = json.Unmarshal([]byte(`{"prefer_domain": false}`), &urlTest)
	require.NoError(t, err)
	require.False(t, urlTest.PreferDomain)

	var stub StubOptions
	err = json.Unmarshal([]byte(`{"prefer_domain": false}`), &stub)
	require.NoError(t, err)
	require.False(t, stub.PreferDomain)
}

func TestPreferDomainMisspelledKey(t *testing.T) {
	t.Parallel()

	var dialer DialerOptions
	err := json.Unmarshal([]byte(`{"perfer_domain": true}`), &dialer)
	require.NoError(t, err)
	require.False(t, dialer.PreferDomain)

	var selector SelectorOutboundOptions
	err = json.Unmarshal([]byte(`{"perfer_domain": true}`), &selector)
	require.NoError(t, err)
	require.False(t, selector.PreferDomain, "misspelled key should not set PreferDomain")

	var urlTest URLTestOutboundOptions
	err = json.Unmarshal([]byte(`{"perfer_domain": true}`), &urlTest)
	require.NoError(t, err)
	require.False(t, urlTest.PreferDomain, "misspelled key should not set PreferDomain")

	var stub StubOptions
	err = json.Unmarshal([]byte(`{"perfer_domain": true}`), &stub)
	require.NoError(t, err)
	require.False(t, stub.PreferDomain, "misspelled key should not set PreferDomain")
}

func TestPreferDomainWithOtherFields(t *testing.T) {
	t.Parallel()

	var dialer DialerOptions
	err := json.Unmarshal([]byte(`{
		"prefer_domain": true,
		"domain_strategy": "prefer_ipv4",
		"fallback_delay": "200ms"
	}`), &dialer)
	require.NoError(t, err)
	require.True(t, dialer.PreferDomain)

	var selector SelectorOutboundOptions
	err = json.Unmarshal([]byte(`{
		"prefer_domain": true,
		"default": "direct"
	}`), &selector)
	require.NoError(t, err)
	require.True(t, selector.PreferDomain)
	require.Equal(t, "direct", selector.Default)

	var urlTest URLTestOutboundOptions
	err = json.Unmarshal([]byte(`{
		"prefer_domain": true,
		"interval": "5m",
		"tolerance": 50
	}`), &urlTest)
	require.NoError(t, err)
	require.True(t, urlTest.PreferDomain)
}

func TestPreferDomainFalse(t *testing.T) {
	t.Parallel()

	var dialer DialerOptions
	err := json.Unmarshal([]byte(`{}`), &dialer)
	require.NoError(t, err)
	require.False(t, dialer.PreferDomain)

	var selector SelectorOutboundOptions
	err = json.Unmarshal([]byte(`{}`), &selector)
	require.NoError(t, err)
	require.False(t, selector.PreferDomain)

	var urlTest URLTestOutboundOptions
	err = json.Unmarshal([]byte(`{}`), &urlTest)
	require.NoError(t, err)
	require.False(t, urlTest.PreferDomain)

	var stub StubOptions
	err = json.Unmarshal([]byte(`{}`), &stub)
	require.NoError(t, err)
	require.False(t, stub.PreferDomain)
}