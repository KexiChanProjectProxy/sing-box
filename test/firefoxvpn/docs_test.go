package firefoxvpn

import (
	"embed"
	"os"
	"testing"

	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/firefox-vpn-example.json
var exampleJSON embed.FS

func TestFirefoxVPNExampleConfigParses(t *testing.T) {
	t.Parallel()

	data, err := exampleJSON.ReadFile("testdata/firefox-vpn-example.json")
	require.NoError(t, err)

	var opts option.Options
	err = json.UnmarshalContext(include.Context(t.Context()), data, &opts)
	require.NoError(t, err)
	require.Len(t, opts.Outbounds, 1)
	require.Equal(t, "firefox-vpn", opts.Outbounds[0].Type)
}

// TestFirefoxVPNDocsCoverAllPublicFields verifies that every exported field
// of option.FirefoxVPNOutboundOptions is mentioned in both the English and
// Chinese documentation files. Fields inherited from DialerOptions,
// ServerOptions, and OutboundTLSOptionsContainer are documented in their
// respective shared sections (Dial Fields, TLS) and are excluded here.
func TestFirefoxVPNDocsCoverAllPublicFields(t *testing.T) {
	t.Parallel()

	enDocs := readDocFile(t, "../../docs/configuration/outbound/firefox-vpn.md")
	zhDocs := readDocFile(t, "../../docs/configuration/outbound/firefox-vpn.zh.md")

	fields := []string{
		"api_detour",
		"email",
		"password",
		"tls",
	}

	for _, field := range fields {
		t.Run("en_"+field, func(t *testing.T) {
			t.Parallel()
			require.Contains(t, enDocs, field,
				"English docs should mention field %q", field)
		})
		t.Run("zh_"+field, func(t *testing.T) {
			t.Parallel()
			require.Contains(t, zhDocs, field,
				"Chinese docs should mention field %q", field)
		})
	}
}

func readDocFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err, "failed to read %s", path)
	return string(data)
}
