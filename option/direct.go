package option

import (
	"context"
	"net/netip"

	"github.com/sagernet/sing-box/experimental/deprecated"
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

type _DirectOutboundOptions struct {
	DialerOptions
	// Deprecated: Use Route Action instead
	OverrideAddress string `json:"override_address,omitempty"`
	// Deprecated: Use Route Action instead
	OverridePort uint16 `json:"override_port,omitempty"`
	// Deprecated: removed
	ProxyProtocol  uint8              `json:"proxy_protocol,omitempty"`
	XLAT464Prefix *badoption.Prefix `json:"xlat464_prefix,omitempty"`
}

type DirectOutboundOptions _DirectOutboundOptions

func (d *DirectOutboundOptions) UnmarshalJSONContext(ctx context.Context, content []byte) error {
	err := json.UnmarshalDisallowUnknownFields(content, (*_DirectOutboundOptions)(d))
	if err != nil {
		return err
	}
	if d.OverrideAddress != "" || d.OverridePort != 0 {
		deprecated.Report(ctx, deprecated.OptionDestinationOverrideFields)
	}
	if d.XLAT464Prefix != nil {
		prefix := d.XLAT464Prefix.Build(netip.Prefix{})
		if prefix.IsValid() && prefix.Bits() != 96 {
			return E.New("xlat464_prefix must be a /96 prefix per RFC 6052")
		}
	}
	return nil
}
