// Package authbypass lets proxy inbounds skip user authentication for
// connections from trusted source addresses.
package authbypass

import (
	"net/netip"

	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
)

// Authenticator picks the authenticator to use for a connection based on its
// source address.
type Authenticator struct {
	authenticator *auth.Authenticator
	bypass        []netip.Prefix
}

func New(users []auth.User, bypass badoption.Listable[*badoption.Prefixable]) *Authenticator {
	a := &Authenticator{authenticator: auth.NewAuthenticator(users)}
	for _, prefix := range bypass {
		if prefix == nil {
			continue
		}
		a.bypass = append(a.bypass, netip.Prefix(*prefix).Masked())
	}
	return a
}

// For returns nil (no authentication) when source matches a bypass prefix,
// and the configured authenticator otherwise.
func (a *Authenticator) For(source M.Socksaddr) *auth.Authenticator {
	if a.authenticator == nil || len(a.bypass) == 0 || !source.IsIP() {
		return a.authenticator
	}
	addr := source.Addr.Unmap()
	for _, prefix := range a.bypass {
		if prefix.Contains(addr) {
			return nil
		}
	}
	return a.authenticator
}
