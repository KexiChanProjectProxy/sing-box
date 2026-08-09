package option

import (
	"context"
	"net/netip"

	E "github.com/sagernet/sing/common/exceptions"
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
//	{"xlat464":{"prefix":"64:ff9b::/96"}}
//
// where Prefix MUST be an IPv6 /96 prefix. IPv4, IPv4-mapped, and any
// other bit length are rejected. Prefix is a pointer so the nil
// (absent) case is preserved as a no-op.
type Xlat464Options struct {
	Prefix *badoption.Prefix `json:"prefix"`
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
	return nil
}
