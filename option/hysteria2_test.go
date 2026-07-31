package option

import (
	"strings"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common/json"
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
