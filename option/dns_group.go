package option

import "github.com/sagernet/sing/common/json/badoption"

// GroupDNSServerOptions spreads queries over other DNS servers.
type GroupDNSServerOptions struct {
	Servers   badoption.Listable[string] `json:"servers,omitempty"`
	Fallbacks badoption.Listable[string] `json:"fallbacks,omitempty"`
	Mode      string                     `json:"mode,omitempty"`
}
