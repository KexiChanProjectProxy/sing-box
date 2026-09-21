package option

import (
	"math"

	"github.com/sagernet/sing/common/json/badoption"

	E "github.com/sagernet/sing/common/exceptions"
)

type SelectorOutboundOptions struct {
	Outbounds                 []string           `json:"outbounds" reference:"outbound"`
	Default                   string             `json:"default,omitempty" reference:"outbound"`
	InterruptExistConnections bool               `json:"interrupt_exist_connections,omitempty"`
	PreferDomain              bool               `json:"prefer_domain,omitempty"`
	OverrideIP                *OverrideIPOptions `json:"override_ip,omitempty"`
}

type URLTestOutboundOptions struct {
	Outbounds                 []string           `json:"outbounds" reference:"outbound"`
	URL                       string             `json:"url,omitempty"`
	Interval                  badoption.Duration `json:"interval,omitempty"`
	Tolerance                 uint16             `json:"tolerance,omitempty"`
	IdleTimeout               badoption.Duration `json:"idle_timeout,omitempty"`
	InterruptExistConnections bool               `json:"interrupt_exist_connections,omitempty"`
	PreferDomain              bool               `json:"prefer_domain,omitempty"`
	OverrideIP                *OverrideIPOptions `json:"override_ip,omitempty"`
}

// LoadBalanceWeightedDelayOptions is deprecated in favour of Sorter, which
// expresses the same blend of recent and historical delay with time based
// windows instead of a sample count.
type LoadBalanceWeightedDelayOptions struct {
	Window       int    `json:"window,omitempty"`
	WindowWeight uint16 `json:"window_weight,omitempty"`
	LastWeight   uint16 `json:"last_weight,omitempty"`
}

// LoadBalanceSorterKeys is the closed set of keys accepted by
// LoadBalanceOutboundOptions.Sorter. The ranking implementation in
// protocol/group holds the matching table of how each key is measured, and a
// test there keeps the two in step.
var LoadBalanceSorterKeys = []string{
	"latency",
	"latency_avg_1m",
	"latency_avg_5m",

	"client_rtt",
	"client_rttvar",
	"client_loss_rate_30s",
	"client_loss_rate_1m",
	"client_loss_rate_5m",
	"client_delivery_rate",

	"server_rtt",
	"server_rttvar",
	"server_loss_rate_30s",
	"server_loss_rate_1m",
	"server_loss_rate_5m",
	"server_delivery_rate",
}

func loadBalanceSorterKeySet() map[string]bool {
	set := make(map[string]bool, len(LoadBalanceSorterKeys))
	for _, key := range LoadBalanceSorterKeys {
		set[key] = true
	}
	return set
}

type LoadBalanceOutboundOptions struct {
	PrimaryOutbounds          []string                         `json:"primary_outbounds" reference:"outbound"`
	BackupOutbounds           []string                         `json:"backup_outbounds,omitempty" reference:"outbound"`
	URL                       string                           `json:"url,omitempty"`
	Interval                  badoption.Duration               `json:"interval,omitempty"`
	Timeout                   badoption.Duration               `json:"timeout,omitempty"`
	IdleTimeout               badoption.Duration               `json:"idle_timeout,omitempty"`
	Tolerance                 uint16                           `json:"tolerance,omitempty"`
	Sorter                    map[string]float64               `json:"sorter,omitempty"`
	WeightedDelay             *LoadBalanceWeightedDelayOptions `json:"weighted_delay,omitempty"`
	TopN                      *LoadBalanceTopNOptions          `json:"top_n,omitempty"`
	Strategy                  string                           `json:"strategy,omitempty"`
	Hash                      *LoadBalanceHashOptions          `json:"hash,omitempty"`
	EmptyPoolAction           string                           `json:"empty_pool_action,omitempty"`
	InterruptExistConnections bool                             `json:"interrupt_exist_connections,omitempty"`
	PreferDomain              bool                             `json:"prefer_domain,omitempty"`
	OverrideIP                *OverrideIPOptions               `json:"override_ip,omitempty"`
}

type LoadBalanceTopNOptions struct {
	Primary int `json:"primary,omitempty"`
	Backup  int `json:"backup,omitempty"`
}

type LoadBalanceHashOptions struct {
	KeyParts     []string `json:"key_parts,omitempty"`
	VirtualNodes int      `json:"virtual_nodes,omitempty"`
	OnEmptyKey   string   `json:"on_empty_key,omitempty"`
	KeySalt      string   `json:"key_salt,omitempty"`
}

func (o LoadBalanceOutboundOptions) Check() error {
	if err := CheckDestinationOverride(o.PreferDomain, o.OverrideIP); err != nil {
		return err
	}
	if len(o.PrimaryOutbounds) == 0 {
		return E.New("missing primary_outbounds")
	}
	if o.Strategy != "" && o.Strategy != "consistent_hash" && o.Strategy != "random" {
		return E.New("unsupported strategy: ", o.Strategy)
	}
	if o.EmptyPoolAction != "" && o.EmptyPoolAction != "error" && o.EmptyPoolAction != "random" {
		return E.New("unsupported empty_pool_action: ", o.EmptyPoolAction)
	}
	if o.Hash != nil {
		if o.Hash.OnEmptyKey != "" && o.Hash.OnEmptyKey != "random" && o.Hash.OnEmptyKey != "error" {
			return E.New("unsupported hash.on_empty_key: ", o.Hash.OnEmptyKey)
		}
		for _, part := range o.Hash.KeyParts {
			switch part {
			case "src_ip", "matched_ruleset_or_etld":
				// valid
			default:
				return E.New("unsupported hash.key_parts entry: ", part)
			}
		}
	}
	// Check duplicate tags between primary and backup
	tagSet := make(map[string]bool)
	for _, tag := range o.PrimaryOutbounds {
		tagSet[tag] = true
	}
	for _, tag := range o.BackupOutbounds {
		if tagSet[tag] {
			return E.New("duplicate tag in primary and backup: ", tag)
		}
	}
	if o.TopN != nil && o.TopN.Backup != 0 {
		return E.New("top_n.backup is not supported")
	}
	if o.Sorter != nil {
		if o.WeightedDelay != nil {
			return E.New("sorter and weighted_delay are mutually exclusive; weighted_delay is deprecated")
		}
		if len(o.Sorter) == 0 {
			return E.New("sorter is empty")
		}
		keySet := loadBalanceSorterKeySet()
		var positive bool
		for key, weight := range o.Sorter {
			if !keySet[key] {
				return E.New("unsupported sorter key: ", key)
			}
			if math.IsNaN(weight) || math.IsInf(weight, 0) {
				return E.New("sorter key ", key, " has a non-finite weight")
			}
			if weight < 0 {
				return E.New("sorter key ", key, " has a negative weight")
			}
			if weight > 0 {
				positive = true
			}
		}
		if !positive {
			return E.New("sorter has no key with a positive weight")
		}
	}
	if o.WeightedDelay != nil {
		if o.WeightedDelay.Window < 0 {
			return E.New("weighted_delay.window is negative")
		}
		if o.WeightedDelay.Window == 0 {
			o.WeightedDelay.Window = 5
		}
		if o.WeightedDelay.WindowWeight == 0 {
			o.WeightedDelay.WindowWeight = 1
		}
		if o.WeightedDelay.LastWeight == 0 {
			o.WeightedDelay.LastWeight = 1
		}
		if o.WeightedDelay.Window > 64 {
			return E.New("weighted_delay.window is greater than 64")
		}
	}
	return nil
}

// SorterWeights returns the effective ranking weights: the configured sorter,
// or a deprecated weighted_delay translated into its sorter equivalent. It
// returns nil when neither is set, which means rank on the latest delay alone.
//
// weighted_delay blended the window average with the latest sample as
//
//	windowWeight/(windowWeight+lastWeight) * average + lastWeight/(...) * latest
//
// so it maps exactly onto latency_avg_5m and latency with those fractions. The
// blend is preserved but the window is not: weighted_delay counted samples,
// where latency_avg_5m covers a fixed five minutes.
//
// Call it after Check, which fills in the weighted_delay defaults.
func (o LoadBalanceOutboundOptions) SorterWeights() map[string]float64 {
	if len(o.Sorter) > 0 {
		weights := make(map[string]float64, len(o.Sorter))
		for key, weight := range o.Sorter {
			weights[key] = weight
		}
		return weights
	}
	if o.WeightedDelay == nil {
		return nil
	}
	windowWeight := float64(o.WeightedDelay.WindowWeight)
	lastWeight := float64(o.WeightedDelay.LastWeight)
	if windowWeight == 0 {
		windowWeight = 1
	}
	if lastWeight == 0 {
		lastWeight = 1
	}
	total := windowWeight + lastWeight
	return map[string]float64{
		"latency":        lastWeight / total,
		"latency_avg_5m": windowWeight / total,
	}
}
