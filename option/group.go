package option

import "github.com/sagernet/sing/common/json/badoption"

type SelectorOutboundOptions struct {
	Outbounds                 []string `json:"outbounds"`
	Default                   string   `json:"default,omitempty"`
	InterruptExistConnections bool     `json:"interrupt_exist_connections,omitempty"`
}

type URLTestOutboundOptions struct {
	Outbounds                 []string           `json:"outbounds"`
	URL                       string             `json:"url,omitempty"`
	Interval                  badoption.Duration `json:"interval,omitempty"`
	Tolerance                 uint16             `json:"tolerance,omitempty"`
	IdleTimeout               badoption.Duration `json:"idle_timeout,omitempty"`
	InterruptExistConnections bool               `json:"interrupt_exist_connections,omitempty"`
}

type LoadBalanceOutboundOptions struct {
	PrimaryOutbounds          []string                         `json:"primary_outbounds"`
	BackupOutbounds           []string                         `json:"backup_outbounds,omitempty"`
	URL                       string                           `json:"url,omitempty"`
	Interval                  badoption.Duration               `json:"interval,omitempty"`
	Timeout                   badoption.Duration               `json:"timeout,omitempty"`
	IdleTimeout               badoption.Duration               `json:"idle_timeout,omitempty"`
	TopN                      LoadBalanceTopNOptions           `json:"top_n"`
	Strategy                  string                           `json:"strategy"`
	Hash                      *LoadBalanceHashOptions          `json:"hash,omitempty"`
	Hysteresis                *LoadBalanceHysteresisOptions    `json:"hysteresis,omitempty"`
	EmptyPoolAction           string                           `json:"empty_pool_action,omitempty"`
	InterruptExistConnections bool                             `json:"interrupt_exist_connections,omitempty"`
}

type LoadBalanceTopNOptions struct {
	Primary int `json:"primary"`
	Backup  int `json:"backup,omitempty"`
}

type LoadBalanceHashOptions struct {
	KeyParts     []string `json:"key_parts,omitempty"`
	VirtualNodes int      `json:"virtual_nodes,omitempty"`
	OnEmptyKey   string   `json:"on_empty_key,omitempty"`
	KeySalt      string   `json:"key_salt,omitempty"`
}

type LoadBalanceHysteresisOptions struct {
	PrimaryFailures uint32             `json:"primary_failures,omitempty"`
	BackupHoldTime  badoption.Duration `json:"backup_hold_time,omitempty"`
}
