package auth

import "net/netip"

// SourceInPrefixes reports whether source is contained in any of the given prefixes.
func SourceInPrefixes(source netip.Addr, prefixes []netip.Prefix) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(source) {
			return true
		}
	}
	return false
}
