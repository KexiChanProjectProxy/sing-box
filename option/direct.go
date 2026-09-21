package option

import (
	"context"
	"net/netip"

	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badoption"
)

type DirectInboundOptions struct {
	ListenOptions
	Network         NetworkList `json:"network,omitempty"`
	OverrideAddress string      `json:"override_address,omitempty"`
	OverridePort    uint16      `json:"override_port,omitempty"`
}

// Xlat464Options configures the direct-only 464XLAT/NAT64 dial path. The
// direct outbound accepts the JSON form
//
//	{"xlat464":{"prefix":"64:ff9b::/96","allow_ipv6":false}}
//
// where Prefix MUST be an IPv6 /96 prefix. IPv4, IPv4-mapped, and any
// other bit length are rejected. Prefix is a pointer so the nil
// (absent) case is preserved as a no-op.
type Xlat464Options struct {
	Prefix    *badoption.Prefix `json:"prefix"`
	AllowIPv6 bool              `json:"allow_ipv6,omitempty"`
}

type _DirectOutboundOptions struct {
	DialerOptions
	// Deprecated: Use Route Action instead
	OverrideAddress string `json:"override_address,omitempty" schema:"omit"`
	// Deprecated: Use Route Action instead
	OverridePort uint16 `json:"override_port,omitempty" schema:"omit"`
	// Deprecated: removed
	ProxyProtocol uint8 `json:"proxy_protocol,omitempty" schema:"omit"`
	// Xlat464 enables the direct-only 464XLAT/NAT64 dial path. nil means
	// the feature is absent and the outbound behaves like a normal
	// direct outbound.
	Xlat464 *Xlat464Options `json:"xlat464,omitempty"`
	// NonLocalBind lets sockets bind to addresses that are not assigned to a
	// local interface (IP_FREEBIND on Linux, IP_BINDANY on FreeBSD).
	NonLocalBind bool `json:"non_local_bind,omitempty"`
	// SourceBind selects the local bind address per client source IP. nil
	// means every connection uses the dial fields' bind addresses.
	SourceBind *SourceBindOptions `json:"source_bind,omitempty"`
}

// SourceBindRule maps clients whose source address is inside SourceIPCIDR
// to the given bind addresses. A single address is a fixed mapping; a prefix
// is a pool that each client is stably and randomly assigned an address from.
type SourceBindRule struct {
	SourceIPCIDR   badoption.Listable[*badoption.Prefixable] `json:"source_ip_cidr,omitempty"`
	Inet4Addresses badoption.Listable[*badoption.Prefixable] `json:"inet4_addresses,omitempty"`
	Inet6Addresses badoption.Listable[*badoption.Prefixable] `json:"inet6_addresses,omitempty"`
}

// SourceBindOptions configures per-client bind address selection for the
// direct outbound. Rules are tried in order; clients matching no rule are
// assigned a random address from the default pool, kept while the client
// keeps opening connections within TTL of each other.
type SourceBindOptions struct {
	Inet4Addresses badoption.Listable[*badoption.Prefixable] `json:"inet4_addresses,omitempty"`
	Inet6Addresses badoption.Listable[*badoption.Prefixable] `json:"inet6_addresses,omitempty"`
	TTL            badoption.Duration                        `json:"ttl,omitempty"`
	Rules          []SourceBindRule                          `json:"rules,omitempty"`
}

func (o *SourceBindOptions) Check() error {
	if o.TTL < 0 {
		return E.New("source_bind: ttl must not be negative")
	}
	if len(o.Inet4Addresses) == 0 && len(o.Inet6Addresses) == 0 && len(o.Rules) == 0 {
		return E.New("source_bind: at least one of inet4_addresses, inet6_addresses or rules is required")
	}
	if err := checkSourceBindAddresses("source_bind.inet4_addresses", o.Inet4Addresses, false); err != nil {
		return err
	}
	if err := checkSourceBindAddresses("source_bind.inet6_addresses", o.Inet6Addresses, true); err != nil {
		return err
	}
	for index, rule := range o.Rules {
		name := F.ToString("source_bind.rules[", index, "]")
		if len(rule.SourceIPCIDR) == 0 {
			return E.New(name, ": missing source_ip_cidr")
		}
		for _, prefix := range rule.SourceIPCIDR {
			if prefix == nil || !netip.Prefix(*prefix).IsValid() {
				return E.New(name, ": invalid source_ip_cidr")
			}
		}
		if len(rule.Inet4Addresses) == 0 && len(rule.Inet6Addresses) == 0 {
			return E.New(name, ": missing inet4_addresses and inet6_addresses")
		}
		if err := checkSourceBindAddresses(name+".inet4_addresses", rule.Inet4Addresses, false); err != nil {
			return err
		}
		if err := checkSourceBindAddresses(name+".inet6_addresses", rule.Inet6Addresses, true); err != nil {
			return err
		}
	}
	return nil
}

func checkSourceBindAddresses(name string, addresses []*badoption.Prefixable, isIPv6 bool) error {
	for _, address := range addresses {
		if address == nil {
			return E.New(name, ": invalid address")
		}
		prefix := netip.Prefix(*address)
		if !prefix.IsValid() {
			return E.New(name, ": invalid address")
		}
		addr := prefix.Addr()
		if addr.Is4In6() {
			return E.New(name, ": IPv4-mapped addresses are not supported: ", prefix)
		}
		if isIPv6 && !addr.Is6() {
			return E.New(name, ": not an IPv6 address: ", prefix)
		}
		if !isIPv6 && !addr.Is4() {
			return E.New(name, ": not an IPv4 address: ", prefix)
		}
		if addr.IsUnspecified() || addr.IsMulticast() {
			return E.New(name, ": unusable bind address: ", prefix)
		}
	}
	return nil
}

type DirectOutboundOptions _DirectOutboundOptions

func (d *DirectOutboundOptions) UnmarshalJSONContext(ctx context.Context, content []byte) error {
	err := json.UnmarshalDisallowUnknownFields(content, (*_DirectOutboundOptions)(d))
	if err != nil {
		return err
	}
	//nolint:staticcheck
	if d.OverrideAddress != "" || d.OverridePort != 0 {
		return E.New("destination override fields in direct outbound are deprecated in sing-box 1.11.0 and removed in sing-box 1.13.0, use route options instead")
	}
	if d.Xlat464 != nil {
		if d.Xlat464.Prefix == nil {
			return E.New("xlat464: prefix is required")
		}
		prefix := netip.Prefix(*d.Xlat464.Prefix)
		if !prefix.IsValid() {
			return E.New("xlat464: prefix is required")
		}
		addr := prefix.Addr()
		if addr.Is4In6() {
			return E.New("xlat464: IPv4-mapped prefixes are not supported")
		}
		if !addr.Is6() {
			return E.New("xlat464: prefix must be an IPv6 /96")
		}
		if prefix.Bits() != 96 {
			return E.New("xlat464: prefix must be an IPv6 /96")
		}
	}
	if d.SourceBind != nil {
		return d.SourceBind.Check()
	}
	return nil
}
