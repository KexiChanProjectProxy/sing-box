package cachefile

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"

	"github.com/stretchr/testify/require"
)

func TestCloudflareWARPRegistration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.db")
	for _, cacheID := range []string{"", "profile"} {
		openCache := func() (*CacheFile, *adapter.Scope) {
			logger := log.NewNOPFactory().Logger()
			cacheFile := New(context.Background(), logger, option.CacheFileOptions{Path: path, CacheID: cacheID})
			scope := adapter.NewScope(context.Background(), logger)
			require.NoError(t, cacheFile.Start(adapter.StartStateInitialize, scope))
			return cacheFile, scope
		}
		cacheFile, scope := openCache()
		_, err := cacheFile.LoadCloudflareWARPRegistration("warp")
		require.ErrorIs(t, err, os.ErrNotExist)
		require.NoError(t, cacheFile.StoreCloudflareWARPRegistration("warp", []byte(`{"version":1}`)))
		data, err := cacheFile.LoadCloudflareWARPRegistration("warp")
		require.NoError(t, err)
		require.Equal(t, `{"version":1}`, string(data))
		require.NoError(t, scope.Close())

		// The bucket must survive the unknown-bucket cleanup on start.
		cacheFile, scope = openCache()
		data, err = cacheFile.LoadCloudflareWARPRegistration("warp")
		require.NoError(t, err)
		require.Equal(t, `{"version":1}`, string(data))
		require.NoError(t, cacheFile.DeleteCloudflareWARPRegistration("warp"))
		_, err = cacheFile.LoadCloudflareWARPRegistration("warp")
		require.ErrorIs(t, err, os.ErrNotExist)
		require.NoError(t, cacheFile.DeleteCloudflareWARPRegistration("missing"))
		require.NoError(t, scope.Close())
	}
}
