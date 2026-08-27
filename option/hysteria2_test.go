package option

import (
	"strings"
	"testing"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badoption"
	"github.com/stretchr/testify/require"
)

func TestHysteria2MasqueradeProxyXObjectFormXForwarded(t *testing.T) {
	t.Parallel()

	input := `{"type":"proxy","url":"https://upstream.example","x_forwarded":true}`
	var m Hysteria2Masquerade
	err := json.Unmarshal([]byte(input), &m)
	require.NoError(t, err)
	require.Equal(t, C.Hysterai2MasqueradeTypeProxy, m.Type)
	require.Equal(t, "https://upstream.example", m.ProxyOptions.URL)
	require.True(t, m.ProxyOptions.XForwarded)
}

func TestHysteria2MasqueradeProxyXObjectFormXForwardedOmitted(t *testing.T) {
	t.Parallel()

	input := `{"type":"proxy","url":"https://upstream.example"}`
	var m Hysteria2Masquerade
	err := json.Unmarshal([]byte(input), &m)
	require.NoError(t, err)
	require.Equal(t, C.Hysterai2MasqueradeTypeProxy, m.Type)
	require.False(t, m.ProxyOptions.XForwarded)
}

func TestHysteria2MasqueradeProxyXObjectFormMarshalXForwarded(t *testing.T) {
	t.Parallel()

	m := Hysteria2Masquerade{
		Type: C.Hysterai2MasqueradeTypeProxy,
		ProxyOptions: Hysteria2MasqueradeProxy{
			URL:        "https://upstream.example",
			XForwarded: true,
		},
	}
	data, err := json.Marshal(m)
	require.NoError(t, err)
	s := string(data)
	require.Contains(t, s, `"x_forwarded":true`)
	require.Contains(t, s, `"url":"https://upstream.example"`)
}

func TestHysteria2MasqueradeProxyXStringForm(t *testing.T) {
	t.Parallel()

	input := `"https://upstream.example"`
	var m Hysteria2Masquerade
	err := json.Unmarshal([]byte(input), &m)
	require.NoError(t, err)
	require.Equal(t, C.Hysterai2MasqueradeTypeProxy, m.Type)
	require.Equal(t, "https://upstream.example", m.ProxyOptions.URL)
	require.False(t, m.ProxyOptions.XForwarded)
}

func TestHysteria2MasqueradeProxyXObjectFormXForwardedFalse(t *testing.T) {
	t.Parallel()

	input := `{"type":"proxy","url":"https://upstream.example","x_forwarded":false}`
	var m Hysteria2Masquerade
	err := json.Unmarshal([]byte(input), &m)
	require.NoError(t, err)
	require.False(t, m.ProxyOptions.XForwarded)
}

func TestHysteria2MasqueradeProxyXObjectFormMarshalOmitempty(t *testing.T) {
	t.Parallel()

	m := Hysteria2Masquerade{
		Type: C.Hysterai2MasqueradeTypeProxy,
		ProxyOptions: Hysteria2MasqueradeProxy{
			URL: "https://upstream.example",
		},
	}
	data, err := json.Marshal(m)
	require.NoError(t, err)
	require.False(t, strings.Contains(string(data), "x_forwarded"))
}

func TestHysteria2RealmAlignmentFields(t *testing.T) {
	t.Parallel()
	input := `{
		"server_url":"https://realm.example.com",
		"realm_id":"slot",
		"stun_servers":["stun.example.com"],
		"prefer_ip_version":"v6",
		"fallback_timeout":"10s",
		"ipv6_api":"https://api6.ipify.org",
		"listen_ports":["60000-61000"]
	}`
	var r Hysteria2Realm
	err := json.Unmarshal([]byte(input), &r)
	require.NoError(t, err)
	require.Equal(t, "v6", r.PreferIPVersion)
	require.Equal(t, badoption.Duration(10*time.Second), r.FallbackTimeout)
	require.Equal(t, "https://api6.ipify.org", r.IPv6API)
	require.Equal(t, badoption.Listable[string]{"60000-61000"}, r.ListenPorts)
}
