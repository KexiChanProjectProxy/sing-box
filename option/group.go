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
	PrimaryOutbounds          []string                      `json:"primary_outbounds"`
	BackupOutbounds           []string                      `json:"backup_outbounds,omitempty"`
	URL                       string                        `json:"url,omitempty"`
	Interval                  badoption.Duration            `json:"interval,omitempty"`
	Timeout                   badoption.Duration            `json:"timeout,omitempty"`
	IdleTimeout               badoption.Duration            `json:"idle_timeout,omitempty"`
	TopN                      LoadBalanceTopNOptions        `json:"top_n"`
	Tolerance                 uint16                        `json:"tolerance,omitempty"`
	Strategy                  string                        `json:"strategy"`
	Hash                      *LoadBalanceHashOptions       `json:"hash,omitempty"`
	Hysteresis                *LoadBalanceHysteresisOptions `json:"hysteresis,omitempty"`
	EmptyPoolAction           string                        `json:"empty_pool_action,omitempty"`
	InterruptExistConnections bool                          `json:"interrupt_exist_connections,omitempty"`
}

type LoadBalanceTopNOptions struct {
	Primary int `json:"primary"`
	Backup  int `json:"backup,omitempty"`
}

// LoadBalanceHashOptions configures consistent hash-based routing for LoadBalance outbound.
//
// Supported key_parts values:
//   - "src_ip": Source IP address
//   - "dst_ip": Destination IP address
//   - "src_port": Source port number
//   - "dst_port": Destination port number
//   - "network": Network type (tcp/udp)
//   - "domain": Full destination domain name
//   - "inbound_tag": Tag of the inbound that accepted the connection
//   - "matched_ruleset": Tag of the ruleset that matched this connection (if any)
//   - "etld_plus_one": eTLD+1 of the destination domain (e.g., example.com from a.b.example.com)
//   - "matched_ruleset_or_etld": Smart fallback - use matched ruleset if available, otherwise eTLD+1
//   - "dst_asn": Destination IP's Autonomous System Number (requires ASN database)
//   - "dst_geosite": Destination domain's geosite category (requires geosite database)
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
