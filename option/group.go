package option

import (
	"github.com/sagernet/sing/common/json/badoption"

	E "github.com/sagernet/sing/common/exceptions"
)

type SelectorOutboundOptions struct {
	Outbounds                 []string `json:"outbounds"`
	Default                   string   `json:"default,omitempty"`
	InterruptExistConnections bool     `json:"interrupt_exist_connections,omitempty"`
	PreferDomain              bool     `json:"prefer_domain,omitempty"`
}

type URLTestOutboundOptions struct {
	Outbounds                 []string           `json:"outbounds"`
	URL                       string             `json:"url,omitempty"`
	Interval                  badoption.Duration `json:"interval,omitempty"`
	Tolerance                 uint16             `json:"tolerance,omitempty"`
	IdleTimeout               badoption.Duration `json:"idle_timeout,omitempty"`
	InterruptExistConnections bool               `json:"interrupt_exist_connections,omitempty"`
	PreferDomain              bool               `json:"prefer_domain,omitempty"`
}

type LoadBalanceOutboundOptions struct {
	PrimaryOutbounds           []string                     `json:"primary_outbounds"`
	BackupOutbounds            []string                     `json:"backup_outbounds,omitempty"`
	URL                        string                       `json:"url,omitempty"`
	Interval                   badoption.Duration           `json:"interval,omitempty"`
	Timeout                    badoption.Duration           `json:"timeout,omitempty"`
	IdleTimeout                badoption.Duration           `json:"idle_timeout,omitempty"`
	TopN                       *LoadBalanceTopNOptions      `json:"top_n,omitempty"`
	Strategy                   string                       `json:"strategy,omitempty"`
	Hash                       *LoadBalanceHashOptions      `json:"hash,omitempty"`
	EmptyPoolAction            string                       `json:"empty_pool_action,omitempty"`
	InterruptExistConnections  bool                         `json:"interrupt_exist_connections,omitempty"`
	PreferDomain               bool                         `json:"prefer_domain,omitempty"`
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
	return nil
}
