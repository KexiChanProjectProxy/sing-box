package adapter

import "net/netip"

// ASNReader provides ASN (Autonomous System Number) lookup functionality.
// Implementations should use MaxMind GeoLite2-ASN database or compatible format.
type ASNReader interface {
	// Lookup returns the Autonomous System Number for the given IP address.
	// Returns 0 if the IP is not found in the database.
	Lookup(addr netip.Addr) uint
}
