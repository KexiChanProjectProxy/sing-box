package group

import (
	"strconv"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	commonurltest "github.com/sagernet/sing-box/common/urltest"
)

func loadBalanceProbeConfig(lb *LoadBalance) commonurltest.ProbeConfig {
	link := lb.link
	interval := lb.interval
	timeout := lb.timeout
	tolerance := lb.tolerance
	strategy := lb.strategy
	emptyPoolAction := lb.emptyPoolAction
	topNPrimary := lb.topNPrimary
	topNBackup := lb.topNBackup
	hashVirtualNodes := lb.hashVirtualNodes
	hashOnEmptyKey := lb.hashOnEmptyKey
	hashKeySalt := lb.hashKeySalt
	hashKeyParts := lb.hashKeyParts
	hystPrimaryFailures := lb.hystPrimaryFailures
	hystBackupHoldTime := lb.hystBackupHoldTime

	if link == "" {
		link = commonurltest.DefaultLink
	}
	if interval == 0 {
		interval = defaultInterval
	}
	if timeout == 0 {
		timeout = defaultTimeout
	}
	if strategy == "" {
		strategy = strategyRandom
	}
	if emptyPoolAction == "" {
		emptyPoolAction = defaultEmptyPoolAction
	}
	if hashVirtualNodes == 0 {
		hashVirtualNodes = defaultVirtualNodes
	}
	if hashOnEmptyKey == "" {
		hashOnEmptyKey = defaultOnEmptyKey
	}
	if hystPrimaryFailures == 0 {
		hystPrimaryFailures = defaultPrimaryFailures
	}
	if hystBackupHoldTime == 0 {
		hystBackupHoldTime = defaultBackupHoldTime
	}

	policy := []string{
		"interval=" + interval.String(),
		"tolerance=" + strconv.FormatUint(uint64(tolerance), 10),
		"strategy=" + strategy,
		"empty_pool_action=" + emptyPoolAction,
		"top_n_primary=" + strconv.Itoa(topNPrimary),
		"top_n_backup=" + strconv.Itoa(topNBackup),
		"hash_virtual_nodes=" + strconv.Itoa(hashVirtualNodes),
		"hash_on_empty_key=" + hashOnEmptyKey,
		"hash_key_salt=" + hashKeySalt,
		"hash_key_parts=" + strings.Join(hashKeyParts, ","),
		"hyst_primary_failures=" + strconv.FormatUint(uint64(hystPrimaryFailures), 10),
		"hyst_backup_hold_time=" + hystBackupHoldTime.String(),
	}
	return commonurltest.NewProbeConfig("loadbalance", link, timeout, policy...)
}

func resolveProbeHistoryKey(detour adapter.Outbound, config commonurltest.ProbeConfig) string {
	resolved, err := ResolveOutbound(detour)
	if err != nil || resolved.Leaf == nil {
		return ""
	}
	return commonurltest.ProbeKey(resolved.Leaf.Tag(), config)
}

func loadProbeHistory(storage adapter.URLTestHistoryStorage, detour adapter.Outbound, config commonurltest.ProbeConfig) *adapter.URLTestHistory {
	key := resolveProbeHistoryKey(detour, config)
	if key == "" {
		return nil
	}
	return storage.LoadURLTestHistory(key)
}

func storeProbeHistory(storage adapter.URLTestHistoryStorage, detour adapter.Outbound, config commonurltest.ProbeConfig, history *adapter.URLTestHistory) bool {
	key := resolveProbeHistoryKey(detour, config)
	if key == "" {
		return false
	}
	storage.StoreURLTestHistory(key, history)
	return true
}

func deleteProbeHistory(storage adapter.URLTestHistoryStorage, detour adapter.Outbound, config commonurltest.ProbeConfig) bool {
	key := resolveProbeHistoryKey(detour, config)
	if key == "" {
		return false
	}
	storage.DeleteURLTestHistory(key)
	return true
}
