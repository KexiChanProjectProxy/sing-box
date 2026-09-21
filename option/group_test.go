package option

import (
	"math"
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
		"tolerance": 10,
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
	require.Equal(t, uint16(10), options.Tolerance)
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
	require.Equal(t, uint16(0), options.Tolerance)
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

func TestLoadBalanceOutboundOptionsOverrideIP(t *testing.T) {
	t.Parallel()

	var options LoadBalanceOutboundOptions
	err := json.Unmarshal([]byte(`{"primary_outbounds":["a"], "override_ip": "ipv4_only"}`), &options)
	require.NoError(t, err)
	require.NotNil(t, options.OverrideIP)
	require.Equal(t, "ipv4_only", options.OverrideIP.Strategy.String())
}

func TestLoadBalanceCheckPreferDomainOverrideIPConflict(t *testing.T) {
	t.Parallel()

	options := LoadBalanceOutboundOptions{
		PrimaryOutbounds: []string{"a"},
		PreferDomain:     true,
		OverrideIP:       &OverrideIPOptions{Strategy: DomainStrategy(1)},
	}
	err := options.Check()
	require.Error(t, err)
	require.Contains(t, err.Error(), "mutually exclusive")
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

func TestLoadBalanceWeightedDelayJSON(t *testing.T) {
	t.Parallel()

	var options LoadBalanceOutboundOptions
	err := json.Unmarshal([]byte(`{
		"primary_outbounds": ["a"],
		"weighted_delay": {"window": 10, "window_weight": 7, "last_weight": 3}
	}`), &options)
	require.NoError(t, err)
	require.NotNil(t, options.WeightedDelay)
	require.Equal(t, 10, options.WeightedDelay.Window)
	require.Equal(t, uint16(7), options.WeightedDelay.WindowWeight)
	require.Equal(t, uint16(3), options.WeightedDelay.LastWeight)
	require.NoError(t, options.Check())
	require.Equal(t, 10, options.WeightedDelay.Window)
	require.Equal(t, uint16(7), options.WeightedDelay.WindowWeight)
	require.Equal(t, uint16(3), options.WeightedDelay.LastWeight)

	encoded, err := json.Marshal(&options)
	require.NoError(t, err)
	var roundTrip LoadBalanceOutboundOptions
	require.NoError(t, json.Unmarshal(encoded, &roundTrip))
	require.NotNil(t, roundTrip.WeightedDelay)
	require.Equal(t, 10, roundTrip.WeightedDelay.Window)
	require.Equal(t, uint16(7), roundTrip.WeightedDelay.WindowWeight)
	require.Equal(t, uint16(3), roundTrip.WeightedDelay.LastWeight)
}

func TestLoadBalanceWeightedDelayDefaults(t *testing.T) {
	t.Parallel()

	var options LoadBalanceOutboundOptions
	err := json.Unmarshal([]byte(`{"primary_outbounds":["a"],"weighted_delay":{}}`), &options)
	require.NoError(t, err)
	require.NotNil(t, options.WeightedDelay)
	require.NoError(t, options.Check())
	require.Equal(t, 5, options.WeightedDelay.Window)
	require.Equal(t, uint16(1), options.WeightedDelay.WindowWeight)
	require.Equal(t, uint16(1), options.WeightedDelay.LastWeight)
}

func TestLoadBalanceWeightedDelayNegativeWindow(t *testing.T) {
	t.Parallel()

	var options LoadBalanceOutboundOptions
	err := json.Unmarshal([]byte(`{"primary_outbounds":["a"],"weighted_delay":{"window":-1}}`), &options)
	require.NoError(t, err)
	err = options.Check()
	require.Error(t, err)
	require.Contains(t, err.Error(), "negative")
}

func TestLoadBalanceWeightedDelayWindowTooLarge(t *testing.T) {
	t.Parallel()

	var options LoadBalanceOutboundOptions
	err := json.Unmarshal([]byte(`{"primary_outbounds":["a"],"weighted_delay":{"window":65}}`), &options)
	require.NoError(t, err)
	err = options.Check()
	require.Error(t, err)
	require.Contains(t, err.Error(), "greater than 64")
}

func TestLoadBalanceSorterJSON(t *testing.T) {
	t.Parallel()

	var options LoadBalanceOutboundOptions
	err := json.Unmarshal([]byte(`{
		"primary_outbounds": ["a"],
		"sorter": {
			"latency_avg_1m": 1,
			"client_rtt": 0.5,
			"server_loss_rate_1m": 20,
			"server_delivery_rate": 0.1
		}
	}`), &options)
	require.NoError(t, err)
	require.NoError(t, options.Check())
	require.Len(t, options.Sorter, 4)
	require.Equal(t, float64(1), options.Sorter["latency_avg_1m"])
	require.Equal(t, 0.5, options.Sorter["client_rtt"])

	encoded, err := json.Marshal(&options)
	require.NoError(t, err)
	var roundTrip LoadBalanceOutboundOptions
	require.NoError(t, json.Unmarshal(encoded, &roundTrip))
	require.Equal(t, options.Sorter, roundTrip.Sorter)
}

func TestLoadBalanceSorterEveryKeyAccepted(t *testing.T) {
	t.Parallel()

	for _, key := range LoadBalanceSorterKeys {
		options := LoadBalanceOutboundOptions{
			PrimaryOutbounds: []string{"a"},
			Sorter:           map[string]float64{key: 1},
		}
		require.NoError(t, options.Check(), key)
	}
}

func TestLoadBalanceSorterRejectsUnknownKey(t *testing.T) {
	t.Parallel()

	options := LoadBalanceOutboundOptions{
		PrimaryOutbounds: []string{"a"},
		Sorter:           map[string]float64{"latency_avg_2m": 1},
	}
	require.ErrorContains(t, options.Check(), "unsupported sorter key")
}

func TestLoadBalanceSorterRejectsNegativeWeight(t *testing.T) {
	t.Parallel()

	options := LoadBalanceOutboundOptions{
		PrimaryOutbounds: []string{"a"},
		Sorter:           map[string]float64{"latency": -1},
	}
	require.ErrorContains(t, options.Check(), "negative weight")
}

func TestLoadBalanceSorterRejectsNonFiniteWeight(t *testing.T) {
	t.Parallel()

	for _, weight := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		options := LoadBalanceOutboundOptions{
			PrimaryOutbounds: []string{"a"},
			Sorter:           map[string]float64{"latency": weight},
		}
		require.ErrorContains(t, options.Check(), "non-finite weight")
	}
}

func TestLoadBalanceSorterRejectsAllZeroWeights(t *testing.T) {
	t.Parallel()

	options := LoadBalanceOutboundOptions{
		PrimaryOutbounds: []string{"a"},
		Sorter:           map[string]float64{"latency": 0, "client_rtt": 0},
	}
	require.ErrorContains(t, options.Check(), "no key with a positive weight")
}

func TestLoadBalanceSorterConflictsWithWeightedDelay(t *testing.T) {
	t.Parallel()

	options := LoadBalanceOutboundOptions{
		PrimaryOutbounds: []string{"a"},
		Sorter:           map[string]float64{"latency": 1},
		WeightedDelay:    &LoadBalanceWeightedDelayOptions{Window: 5},
	}
	require.ErrorContains(t, options.Check(), "mutually exclusive")
}

func TestLoadBalanceSorterWeightsCopiesConfiguration(t *testing.T) {
	t.Parallel()

	options := LoadBalanceOutboundOptions{
		PrimaryOutbounds: []string{"a"},
		Sorter:           map[string]float64{"latency": 1},
	}
	weights := options.SorterWeights()
	weights["latency"] = 99
	weights["client_rtt"] = 1
	require.Equal(t, map[string]float64{"latency": 1}, options.Sorter, "the caller must not be able to mutate the options")
}

func TestLoadBalanceSorterWeightsUnsetIsNil(t *testing.T) {
	t.Parallel()

	options := LoadBalanceOutboundOptions{PrimaryOutbounds: []string{"a"}}
	require.NoError(t, options.Check())
	require.Nil(t, options.SorterWeights())
}

func TestLoadBalanceSorterRejectsEmptyObject(t *testing.T) {
	t.Parallel()

	var options LoadBalanceOutboundOptions
	err := json.Unmarshal([]byte(`{"primary_outbounds":["a"],"sorter":{}}`), &options)
	require.NoError(t, err)
	require.ErrorContains(t, options.Check(), "sorter is empty")
}

func TestLoadBalanceSorterEmptyObjectStillConflictsWithWeightedDelay(t *testing.T) {
	t.Parallel()

	options := LoadBalanceOutboundOptions{
		PrimaryOutbounds: []string{"a"},
		Sorter:           map[string]float64{},
		WeightedDelay:    &LoadBalanceWeightedDelayOptions{},
	}
	require.ErrorContains(t, options.Check(), "mutually exclusive")
}
