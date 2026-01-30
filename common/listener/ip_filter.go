package listener

import (
	"net/netip"

	E "github.com/sagernet/sing/common/exceptions"
	"go4.org/netipx"
)

type IPFilter struct {
	whitelist    *netipx.IPSet
	blacklist    *netipx.IPSet
	hasWhitelist bool
	hasBlacklist bool
}

// NewIPFilter creates an IP filter from whitelist and blacklist strings
// Format: ["192.168.1.0/24", "10.0.0.1", "2001:db8::/32"]
// Note: nil = no filter, []string{} = empty list (whitelist: deny all, blacklist: no effect)
func NewIPFilter(whitelistStrings, blacklistStrings []string) (*IPFilter, error) {
	filter := &IPFilter{
		hasWhitelist: whitelistStrings != nil,
		hasBlacklist: blacklistStrings != nil,
	}

	// Build whitelist IPSet (same pattern as rule_item_cidr.go:21-58)
	if filter.hasWhitelist {
		var builder netipx.IPSetBuilder
		for i, ipStr := range whitelistStrings {
			// Try CIDR first
			prefix, err := netip.ParsePrefix(ipStr)
			if err == nil {
				builder.AddPrefix(prefix)
				continue
			}
			// Try individual IP
			addr, addrErr := netip.ParseAddr(ipStr)
			if addrErr == nil {
				builder.Add(addr)
				continue
			}
			return nil, E.Cause(err, "parse whitelist[", i, "]: ", ipStr)
		}
		ipSet, err := builder.IPSet()
		if err != nil {
			return nil, E.Cause(err, "build whitelist IPSet")
		}
		filter.whitelist = ipSet
	}

	// Build blacklist IPSet
	if filter.hasBlacklist {
		var builder netipx.IPSetBuilder
		for i, ipStr := range blacklistStrings {
			prefix, err := netip.ParsePrefix(ipStr)
			if err == nil {
				builder.AddPrefix(prefix)
				continue
			}
			addr, addrErr := netip.ParseAddr(ipStr)
			if addrErr == nil {
				builder.Add(addr)
				continue
			}
			return nil, E.Cause(err, "parse blacklist[", i, "]: ", ipStr)
		}
		ipSet, err := builder.IPSet()
		if err != nil {
			return nil, E.Cause(err, "build blacklist IPSet")
		}
		filter.blacklist = ipSet
	}

	return filter, nil
}

// Allow checks if the given IP address is allowed
// Logic:
//   - If whitelist exists: allow only if in whitelist
//   - Else if blacklist exists: allow only if NOT in blacklist
//   - Else: allow all (no filter)
func (f *IPFilter) Allow(addr netip.Addr) bool {
	if f.hasWhitelist {
		return f.whitelist.Contains(addr)
	}
	if f.hasBlacklist {
		return !f.blacklist.Contains(addr)
	}
	return true // No filter configured
}
