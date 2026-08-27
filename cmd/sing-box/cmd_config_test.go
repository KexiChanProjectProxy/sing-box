package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sagernet/sing-box/include"
	"github.com/stretchr/testify/require"
)

func TestIsConfigFileName(t *testing.T) {
	t.Parallel()

	require.True(t, isConfigFileName("config.json"))
	require.True(t, isConfigFileName("inbound.yaml"))
	require.True(t, isConfigFileName("outbound.yml"))
	require.False(t, isConfigFileName("readme.md"))
	require.False(t, isConfigFileName("config.json.bak"))
}

func TestIsYAMLConfigPath(t *testing.T) {
	t.Parallel()

	require.True(t, isYAMLConfigPath("config.yaml"))
	require.True(t, isYAMLConfigPath("config.YML"))
	require.False(t, isYAMLConfigPath("config.json"))
	require.False(t, isYAMLConfigPath("stdin"))
}

func TestDefaultConfigPath(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	require.Equal(t, "config.json", defaultConfigPath())
	require.NoError(t, os.WriteFile("config.yml", []byte("log:\n  level: info\n"), 0o644))
	require.Equal(t, "config.yml", defaultConfigPath())
	require.NoError(t, os.WriteFile("config.yaml", []byte("log:\n  level: debug\n"), 0o644))
	require.Equal(t, "config.yaml", defaultConfigPath())
	require.NoError(t, os.WriteFile("config.json", []byte(`{"log":{"level":"warn"}}`), 0o644))
	require.Equal(t, "config.json", defaultConfigPath())
}

func TestReadConfigAtYAML(t *testing.T) {
	globalCtx = include.Context(context.Background())
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("log:\n  level: info\n  timestamp: true\n"), 0o644))

	entry, err := readConfigAt(path)
	require.NoError(t, err)
	require.Equal(t, "info", entry.options.Log.Level)
	require.True(t, entry.options.Log.Timestamp)
	require.True(t, entry.writeYAML())
}

func TestReadConfigDirectoryMixesJSONAndYAML(t *testing.T) {
	globalCtx = include.Context(context.Background())
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "10-log.yaml"), []byte("log:\n  level: info\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "20-ntp.json"), []byte(`{"ntp":{"enabled":true,"server":"time.apple.com"}}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.md"), []byte("ignore"), 0o644))

	oldPaths, oldDirs := configPaths, configDirectories
	t.Cleanup(func() {
		configPaths, configDirectories = oldPaths, oldDirs
	})
	configPaths = nil
	configDirectories = []string{dir}

	options, err := readConfigAndMerge()
	require.NoError(t, err)
	require.Equal(t, "info", options.Log.Level)
	require.NotNil(t, options.NTP)
	require.True(t, options.NTP.Enabled)
	require.Equal(t, "time.apple.com", options.NTP.Server)
}

func TestEncodeConfigYAML(t *testing.T) {
	globalCtx = include.Context(context.Background())
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	require.NoError(t, os.WriteFile(path, []byte("log:\n  level: info\n"), 0o644))
	entry, err := readConfigAt(path)
	require.NoError(t, err)
	encoded, err := encodeConfig(entry.options, true)
	require.NoError(t, err)
	require.Contains(t, string(encoded), "level: info")

	out := filepath.Join(dir, "formatted.yaml")
	require.NoError(t, os.WriteFile(out, encoded, 0o644))
	roundTrip, err := readConfigAt(out)
	require.NoError(t, err)
	require.Equal(t, "info", roundTrip.options.Log.Level)
	require.True(t, roundTrip.writeYAML())
}
