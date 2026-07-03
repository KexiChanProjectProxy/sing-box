package option

import (
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
)

type FirefoxVPNOutboundOptions struct {
	DialerOptions
	ServerOptions
	OutboundTLSOptionsContainer
	APIDetour string `json:"api_detour,omitempty"`
	Email     string `json:"email"`
	Password  string `json:"password"`
}

type firefoxVPNOutboundOptionsJSON struct {
	DialerOptions
	ServerOptions
	OutboundTLSOptionsContainer
	APIDetour string `json:"api_detour,omitempty"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	Network   string `json:"network,omitempty"`
}

func (o *FirefoxVPNOutboundOptions) UnmarshalJSON(content []byte) error {
	var options firefoxVPNOutboundOptionsJSON
	err := json.Unmarshal(content, &options)
	if err != nil {
		return err
	}
	if options.Network != "" {
		return E.New("firefox-vpn outbound: packet network modes are not supported")
	}
	*o = FirefoxVPNOutboundOptions{
		DialerOptions:               options.DialerOptions,
		ServerOptions:               options.ServerOptions,
		OutboundTLSOptionsContainer: options.OutboundTLSOptionsContainer,
		APIDetour:                   options.APIDetour,
		Email:                       options.Email,
		Password:                    options.Password,
	}
	return o.Validate()
}

func (o *FirefoxVPNOutboundOptions) Validate() error {
	if o.Email == "" {
		return E.New("firefox-vpn outbound: email is required")
	}
	if o.Password == "" {
		return E.New("firefox-vpn outbound: password is required")
	}
	if o.ServerOptions.Server == "" {
		return E.New("firefox-vpn outbound: server is required")
	}
	if o.ServerOptions.ServerPort == 0 {
		return E.New("firefox-vpn outbound: server_port is required and must be non-zero")
	}
	return nil
}
