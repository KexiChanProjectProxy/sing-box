package adapter

// GeositeReader provides geosite code lookup functionality for domains.
// Implementations should use sing-box geosite.db format.
type GeositeReader interface {
	// Lookup returns the first matching geosite code for the given domain.
	// Returns empty string if no match is found.
	Lookup(domain string) string
}
