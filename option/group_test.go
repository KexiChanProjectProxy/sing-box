package option

import (
	"testing"
	"time"

	"github.com/sagernet/sing/common/json"
	"github.com/stretchr/testify/require"
)

func TestLoadBalanceOutboundOptionsJSON(t *testing.T) {
	t.Parallel()

	var options LoadBalanceOutboundOptions
	err := json.Unmarshal([]byte(`{
		"primary_outbounds": ["a", "b"],
		"backup_outbounds": ["c", "d"],
		"url": "http://example.com/check",
		"interval": "10s",
		"timeout": "5s",
		"idle_timeout": "300s",
		"top_n": {
			"primary": 10
		},
		"strategy": "consistent_hash",
		"hash": {
			"key_parts": ["src_ip", "matched_ruleset_or_etld"],
			"virtual_nodes": 100,
			"on_empty_key": "random",
			"key_salt": "salt123"
		},
		"empty_pool_action": "error",
		"interrupt_exist_connections": true,
		"prefer_domain": true
	}`), &options)
	require.NoError(t, err)

	require.Equal(t, []string{"a", "b"}, options.PrimaryOutbounds)
	require.Equal(t, []string{"c", "d"}, options.BackupOutbounds)
	require.Equal(t, "http://example.com/check", options.URL)
	require.Equal(t, 10*time.Second, options.Interval.Build())
	require.Equal(t, 5*time.Second, options.Timeout.Build())
	require.Equal(t, 300*time.Second, options.IdleTimeout.Build())
	require.NotNil(t, options.TopN)
	require.Equal(t, 10, options.TopN.Primary)
	require.Equal(t, "consistent_hash", options.Strategy)
	require.NotNil(t, options.Hash)
	require.Equal(t, []string{"src_ip", "matched_ruleset_or_etld"}, options.Hash.KeyParts)
	require.Equal(t, 100, options.Hash.VirtualNodes)
	require.Equal(t, "random", options.Hash.OnEmptyKey)
	require.Equal(t, "salt123", options.Hash.KeySalt)
	require.Equal(t, "error", options.EmptyPoolAction)
	require.True(t, options.InterruptExistConnections)
	require.True(t, options.PreferDomain)
}

func TestLoadBalanceOutboundOptionsDefaults(t *testing.T) {
	t.Parallel()

	var options LoadBalanceOutboundOptions
	err := json.Unmarshal([]byte(`{"primary_outbounds":["a"]}`), &options)
	require.NoError(t, err)

	require.Equal(t, []string{"a"}, options.PrimaryOutbounds)
	require.Empty(t, options.Strategy)
	require.Nil(t, options.Hash)
	require.Empty(t, options.EmptyPoolAction)
	require.False(t, options.InterruptExistConnections)
	require.False(t, options.PreferDomain)
}

func TestLoadBalanceTopNOptionsDefaults(t *testing.T) {
	t.Parallel()

	var options LoadBalanceTopNOptions
	err := json.Unmarshal([]byte(`{}`), &options)
	require.NoError(t, err)

	require.Equal(t, 0, options.Primary)
}

func TestLoadBalanceHashOptionsDefaults(t *testing.T) {
	t.Parallel()

	var options LoadBalanceHashOptions
	err := json.Unmarshal([]byte(`{}`), &options)
	require.NoError(t, err)

	require.Equal(t, 0, options.VirtualNodes)
	require.Empty(t, options.OnEmptyKey)
}

func TestLoadBalanceOutboundOptionsPreferDomain(t *testing.T) {
	t.Parallel()

	var options LoadBalanceOutboundOptions
	err := json.Unmarshal([]byte(`{"primary_outbounds":["a"], "prefer_domain": true}`), &options)
	require.NoError(t, err)
	require.True(t, options.PreferDomain)
}

func TestLoadBalanceCheckInvalidStrategy(t *testing.T) {
	t.Parallel()

	var options LoadBalanceOutboundOptions
	err := json.Unmarshal([]byte(`{"primary_outbounds":["a"], "strategy": "round_robin"}`), &options)
	require.NoError(t, err)
	err = options.Check()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported strategy")
}

func TestLoadBalanceCheckInvalidEmptyPoolAction(t *testing.T) {
	t.Parallel()

	var options LoadBalanceOutboundOptions
	err := json.Unmarshal([]byte(`{"primary_outbounds":["a"], "empty_pool_action": "direct"}`), &options)
	require.NoError(t, err)
	err = options.Check()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported empty_pool_action")
}

func TestLoadBalanceCheckInvalidOnEmptyKey(t *testing.T) {
	t.Parallel()

	var options LoadBalanceOutboundOptions
	err := json.Unmarshal([]byte(`{"primary_outbounds":["a"], "hash": {"on_empty_key": "fallback"}}`), &options)
	require.NoError(t, err)
	err = options.Check()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported hash.on_empty_key")
}

func TestLoadBalanceCheckRandomStrategy(t *testing.T) {
	t.Parallel()

	var options LoadBalanceOutboundOptions
	err := json.Unmarshal([]byte(`{"primary_outbounds":["a", "b"], "strategy": "random"}`), &options)
	require.NoError(t, err)
	err = options.Check()
	require.NoError(t, err, "random strategy should be accepted but got error: %v", err)
}

func TestLoadBalanceCheckRandomEmptyPoolAction(t *testing.T) {
	t.Parallel()

	var options LoadBalanceOutboundOptions
	err := json.Unmarshal([]byte(`{"primary_outbounds":["a", "b"], "empty_pool_action": "random"}`), &options)
	require.NoError(t, err)
	err = options.Check()
	require.NoError(t, err, "random empty_pool_action should be accepted but got error: %v", err)
}

func TestLoadBalanceCheckInvalidKeyPart(t *testing.T) {
	t.Parallel()

	var options LoadBalanceOutboundOptions
	err := json.Unmarshal([]byte(`{"primary_outbounds":["a"], "hash": {"key_parts": ["invalid"]}}`), &options)
	require.NoError(t, err)
	err = options.Check()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported hash.key_parts entry")
}

func TestLoadBalanceCheckMissingPrimaryOutbounds(t *testing.T) {
	t.Parallel()

	options := LoadBalanceOutboundOptions{}
	err := options.Check()
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing primary_outbounds")
}

func TestLoadBalanceCheckDuplicateTag(t *testing.T) {
	t.Parallel()

	var options LoadBalanceOutboundOptions
	err := json.Unmarshal([]byte(`{"primary_outbounds":["a","b"], "backup_outbounds":["b","c"]}`), &options)
	require.NoError(t, err)
	err = options.Check()
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate tag")
}

func TestLoadBalanceCheckTopNBackupRejected(t *testing.T) {
	t.Parallel()

	var options LoadBalanceOutboundOptions
	err := json.Unmarshal([]byte(`{"primary_outbounds":["a"],"top_n":{"primary":2,"backup":3}}`), &options)
	require.NoError(t, err)
	err = options.Check()
	require.Error(t, err)
	require.Contains(t, err.Error(), "top_n.backup")
}

func TestLoadBalanceCheckValid(t *testing.T) {
	t.Parallel()

	var options LoadBalanceOutboundOptions
	err := json.Unmarshal([]byte(`{
		"primary_outbounds": ["a", "b"],
		"backup_outbounds": ["c", "d"],
		"url": "http://example.com/check",
		"interval": "10s",
		"timeout": "5s",
		"idle_timeout": "300s",
		"top_n": {
			"primary": 10
		},
		"strategy": "consistent_hash",
		"hash": {
			"key_parts": ["src_ip", "matched_ruleset_or_etld"],
			"virtual_nodes": 100,
			"on_empty_key": "random",
			"key_salt": "salt123"
		},
		"empty_pool_action": "error",
		"interrupt_exist_connections": true,
		"prefer_domain": true
	}`), &options)
	require.NoError(t, err)
	err = options.Check()
	require.NoError(t, err)
}