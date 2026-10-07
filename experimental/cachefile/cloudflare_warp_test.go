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
		openCache := func() *CacheFile {
			cacheFile := New(context.Background(), log.NewNOPFactory().Logger(), option.CacheFileOptions{Path: path, CacheID: cacheID})
			require.NoError(t, cacheFile.Start(adapter.StartStateInitialize))
			return cacheFile
		}
		cacheFile := openCache()
		_, err := cacheFile.LoadCloudflareWARPRegistration("warp")
		require.ErrorIs(t, err, os.ErrNotExist)
		require.NoError(t, cacheFile.StoreCloudflareWARPRegistration("warp", []byte(`{"version":1}`)))
		data, err := cacheFile.LoadCloudflareWARPRegistration("warp")
		require.NoError(t, err)
		require.Equal(t, `{"version":1}`, string(data))
		require.NoError(t, cacheFile.Close())

		// The bucket must survive the unknown-bucket cleanup on start.
		cacheFile = openCache()
		data, err = cacheFile.LoadCloudflareWARPRegistration("warp")
		require.NoError(t, err)
		require.Equal(t, `{"version":1}`, string(data))
		require.NoError(t, cacheFile.DeleteCloudflareWARPRegistration("warp"))
		_, err = cacheFile.LoadCloudflareWARPRegistration("warp")
		require.ErrorIs(t, err, os.ErrNotExist)
		require.NoError(t, cacheFile.DeleteCloudflareWARPRegistration("missing"))
		require.NoError(t, cacheFile.Close())
	}
}
