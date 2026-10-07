package cloudflarewarp

import (
	"os"
	"reflect"
	"strings"
	"testing"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json"

	"github.com/stretchr/testify/require"
)

func TestCloudflareWARPExampleConfigParses(t *testing.T) {
	data, err := os.ReadFile("testdata/cloudflare-warp-example.json")
	require.NoError(t, err)
	var options option.Options
	err = json.UnmarshalContext(include.Context(t.Context()), data, &options)
	require.NoError(t, err)
	require.Len(t, options.Outbounds, 2)
	for _, outbound := range options.Outbounds {
		require.Equal(t, "cloudflare-warp", outbound.Type)
		require.IsType(t, &option.CloudflareWARPOutboundOptions{}, outbound.Options)
	}
}

// TestCloudflareWARPDocsCoverAllFields checks that every field declared on the
// options struct itself is documented in English and Chinese. Embedded dial,
// server, TLS and QUIC fields are documented in their shared sections.
func TestCloudflareWARPDocsCoverAllFields(t *testing.T) {
	documents := map[string]string{}
	for _, path := range []string{
		"../../docs/configuration/outbound/cloudflare-warp.md",
		"../../docs/configuration/outbound/cloudflare-warp.zh.md",
	} {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		documents[path] = string(data)
	}
	optionsType := reflect.TypeOf(option.CloudflareWARPOutboundOptions{})
	for i := 0; i < optionsType.NumField(); i++ {
		field := optionsType.Field(i)
		if field.Anonymous {
			continue
		}
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		for path, content := range documents {
			require.Contains(t, content, "#### "+name, "%s should document %q", path, name)
		}
	}
	for path, content := range documents {
		require.Contains(t, content, "#### server", path)
		require.Contains(t, content, "#### tls", path)
	}
}

// TestCloudflareWARPAutoRegistrationBoxCreate builds a box from the example
// configuration. The cache file service is registered after outbounds are
// created, so constructing the outbound must not require it yet.
func TestCloudflareWARPAutoRegistrationBoxCreate(t *testing.T) {
	data, err := os.ReadFile("testdata/cloudflare-warp-example.json")
	require.NoError(t, err)
	ctx := include.Context(t.Context())
	var options option.Options
	require.NoError(t, json.UnmarshalContext(ctx, data, &options))
	options.Experimental.CacheFile.Path = t.TempDir() + "/cache.db"
	instance, err := box.New(box.Options{Context: ctx, Options: options})
	require.NoError(t, err)
	require.NoError(t, instance.Close())
}
