package warpapi

import (
	"net/netip"
	"time"
)

// AccountData is the device object returned by the registration API.
type AccountData struct {
	ID      string  `json:"id"`
	Type    string  `json:"type,omitempty"`
	Model   string  `json:"model,omitempty"`
	Name    string  `json:"name,omitempty"`
	Key     string  `json:"key,omitempty"`
	KeyType string  `json:"key_type,omitempty"`
	TunType string  `json:"tunnel_type,omitempty"`
	Account Account `json:"account"`
	Config  Config  `json:"config"`
	Token   string  `json:"token,omitempty"`
	Enabled bool    `json:"enabled,omitempty"`
	Created string  `json:"created,omitempty"`
	Updated string  `json:"updated,omitempty"`
}

type Account struct {
	ID          string `json:"id"`
	AccountType string `json:"account_type,omitempty"`
	WarpPlus    bool   `json:"warp_plus,omitempty"`
	License     string `json:"license,omitempty"`
}

type Config struct {
	ClientID  string `json:"client_id,omitempty"`
	Peers     []Peer `json:"peers"`
	Interface struct {
		Addresses struct {
			V4 string `json:"v4"`
			V6 string `json:"v6"`
		} `json:"addresses"`
	} `json:"interface"`
}

type Peer struct {
	PublicKey string `json:"public_key"`
	Endpoint  struct {
		V4    string `json:"v4"`
		V6    string `json:"v6"`
		Host  string `json:"host,omitempty"`
		Ports []int  `json:"ports,omitempty"`
	} `json:"endpoint"`
}

const RegistrationVersion = 1

// Registration is everything needed to connect, as persisted in the cache file.
type Registration struct {
	Version           int        `json:"version"`
	DeviceID          string     `json:"device_id"`
	AccessToken       string     `json:"access_token"`
	PrivateKey        string     `json:"private_key"`
	AccountID         string     `json:"account_id,omitempty"`
	License           string     `json:"license,omitempty"`
	EndpointV4        netip.Addr `json:"endpoint_v4,omitzero"`
	EndpointV6        netip.Addr `json:"endpoint_v6,omitzero"`
	EndpointPublicKey string     `json:"endpoint_public_key,omitempty"`
	AddressV4         netip.Addr `json:"address_v4,omitzero"`
	AddressV6         netip.Addr `json:"address_v6,omitzero"`
	CreatedAt         time.Time  `json:"created_at,omitzero"`
}

// Addresses returns the tunnel addresses as host prefixes.
func (r *Registration) Addresses() []netip.Prefix {
	var addresses []netip.Prefix
	if r.AddressV4.IsValid() {
		addresses = append(addresses, netip.PrefixFrom(r.AddressV4, 32))
	}
	if r.AddressV6.IsValid() {
		addresses = append(addresses, netip.PrefixFrom(r.AddressV6, 128))
	}
	return addresses
}

// Redacted returns a copy that is safe to log.
func (r Registration) Redacted() Registration {
	if r.AccessToken != "" {
		r.AccessToken = "[redacted]"
	}
	if r.PrivateKey != "" {
		r.PrivateKey = "[redacted]"
	}
	if r.License != "" {
		r.License = "[redacted]"
	}
	return r
}
