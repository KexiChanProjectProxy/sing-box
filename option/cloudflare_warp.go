package option

import (
	"net/netip"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badoption"
)

type CloudflareWARPOutboundOptions struct {
	DialerOptions
	ServerOptions
	OutboundTLSOptionsContainer
	QUICOptions
	PrivateKey        string                           `json:"private_key,omitempty"`
	Address           badoption.Listable[netip.Prefix] `json:"address,omitempty"`
	EndpointPublicKey string                           `json:"endpoint_public_key,omitempty"`
	DeviceID          string                           `json:"device_id,omitempty"`
	AccessToken       string                           `json:"access_token,omitempty"`
	License           string                           `json:"license,omitempty"`
	AccessJWT         string                           `json:"access_jwt,omitempty"`
	DeviceName        string                           `json:"device_name,omitempty"`
	Ephemeral         bool                             `json:"ephemeral,omitempty"`
	APIDetour         string                           `json:"api_detour,omitempty"`
	Network           NetworkList                      `json:"network,omitempty"`
	MTU               uint32                           `json:"mtu,omitempty"`
}

type _CloudflareWARPOutboundOptions CloudflareWARPOutboundOptions

func (o *CloudflareWARPOutboundOptions) UnmarshalJSON(content []byte) error {
	err := json.Unmarshal(content, (*_CloudflareWARPOutboundOptions)(o))
	if err != nil {
		return err
	}
	return o.Validate()
}

// StaticMode reports whether the outbound uses credentials from the configuration
// instead of registering a device and persisting it in the cache file.
func (o *CloudflareWARPOutboundOptions) StaticMode() bool {
	return o.PrivateKey != "" || len(o.Address) > 0
}

func (o *CloudflareWARPOutboundOptions) Validate() error {
	const prefix = "cloudflare-warp outbound: "
	if (o.PrivateKey == "") != (len(o.Address) == 0) {
		return E.New(prefix, "private_key and address must be set together")
	}
	if (o.DeviceID == "") != (o.AccessToken == "") {
		return E.New(prefix, "device_id and access_token must be set together")
	}
	if o.StaticMode() {
		if o.License != "" && o.DeviceID == "" {
			return E.New(prefix, "license requires device_id and access_token when private_key is set")
		}
		if o.AccessJWT != "" {
			return E.New(prefix, "access_jwt is only used for automatic registration; remove it or remove private_key and address")
		}
		if o.DeviceName != "" {
			return E.New(prefix, "device_name is only used for automatic registration; remove it or remove private_key and address")
		}
		if o.Ephemeral {
			return E.New(prefix, "ephemeral is only used for automatic registration; remove it or remove private_key and address")
		}
		var hasInet4, hasInet6 bool
		for _, prefixAddress := range o.Address {
			if !prefixAddress.IsValid() || !prefixAddress.IsSingleIP() {
				return E.New(prefix, "address must be a single host prefix (/32 or /128): ", prefixAddress)
			}
			if prefixAddress.Addr().Is4() {
				if hasInet4 {
					return E.New(prefix, "at most one IPv4 address is allowed")
				}
				hasInet4 = true
			} else {
				if hasInet6 {
					return E.New(prefix, "at most one IPv6 address is allowed")
				}
				hasInet6 = true
			}
		}
	} else if o.DeviceID != "" {
		return E.New(prefix, "device_id and access_token require private_key and address")
	}
	if o.MTU != 0 && (o.MTU < 576 || o.MTU > 65535) {
		return E.New(prefix, "mtu must be between 576 and 65535")
	}
	if o.TLS != nil {
		tlsOptions := o.TLS
		switch {
		case tlsOptions.Engine != "" && tlsOptions.Engine != "go":
			return E.New(prefix, "tls.engine ", tlsOptions.Engine, " is not supported over QUIC")
		case tlsOptions.UTLS != nil && tlsOptions.UTLS.Enabled:
			return E.New(prefix, "tls.utls is not supported over QUIC")
		case tlsOptions.Reality != nil && tlsOptions.Reality.Enabled:
			return E.New(prefix, "tls.reality is not supported over QUIC")
		case tlsOptions.ECH != nil && tlsOptions.ECH.Enabled:
			return E.New(prefix, "tls.ech is not supported")
		case tlsOptions.KernelTx || tlsOptions.KernelRx:
			return E.New(prefix, "tls.kernel_tx and tls.kernel_rx are not supported over QUIC")
		case tlsOptions.Fragment || tlsOptions.RecordFragment:
			return E.New(prefix, "tls.fragment and tls.record_fragment are not supported over QUIC")
		case tlsOptions.Spoof != "":
			return E.New(prefix, "tls.spoof is not supported over QUIC")
		case tlsOptions.DisableSNI:
			return E.New(prefix, "tls.disable_sni is not supported")
		case len(tlsOptions.ALPN) > 0:
			return E.New(prefix, "tls.alpn is fixed to h3")
		case len(tlsOptions.ClientCertificate) > 0 || tlsOptions.ClientCertificatePath != "" ||
			len(tlsOptions.ClientKey) > 0 || tlsOptions.ClientKeyPath != "":
			return E.New(prefix, "tls client certificates are generated from private_key")
		}
	}
	return nil
}
