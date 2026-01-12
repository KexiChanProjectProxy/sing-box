package hash

import (
	"net"
	"strings"

	"golang.org/x/net/publicsuffix"
)

// ExtractETLDPlusOne extracts the eTLD+1 (effective top-level domain plus one label)
// from a given domain using the Public Suffix List.
//
// Examples:
//   - "a.b.example.com" → "example.com"
//   - "a.b.example.co.uk" → "example.co.uk" (not "co.uk")
//   - "192.168.1.1" → "-" (IP placeholder)
//   - "localhost" → "localhost"
//
// This is useful for hash-based routing where you want to maintain session affinity
// per top-level domain rather than per full subdomain.
func ExtractETLDPlusOne(domain string) string {
	if domain == "" {
		return "-"
	}

	// Normalize the domain
	domain = normalizeDomain(domain)

	// Check if it's an IP address
	if net.ParseIP(domain) != nil {
		return "-"
	}

	// Check if it's an IPv6 address in brackets
	if strings.HasPrefix(domain, "[") && strings.HasSuffix(domain, "]") {
		trimmed := strings.Trim(domain, "[]")
		if net.ParseIP(trimmed) != nil {
			return "-"
		}
	}

	// Extract eTLD+1 using publicsuffix
	etld1, err := publicsuffix.EffectiveTLDPlusOne(domain)
	if err != nil {
		// If extraction fails (e.g., for localhost, invalid domains),
		// return the normalized domain as-is
		return domain
	}

	return etld1
}

// normalizeDomain normalizes a domain name for processing:
//   - Converts to lowercase
//   - Strips port numbers
//   - Removes trailing dots
//   - Trims whitespace
func normalizeDomain(domain string) string {
	// Trim whitespace
	domain = strings.TrimSpace(domain)

	// Convert to lowercase
	domain = strings.ToLower(domain)

	// Remove port if present
	// Handle both regular domains and IPv6 addresses in brackets
	if strings.HasPrefix(domain, "[") {
		// IPv6 with port: [::1]:8080
		if idx := strings.LastIndex(domain, "]:"); idx != -1 {
			domain = domain[:idx+1] // Keep the brackets
		}
	} else {
		// Regular domain with port: example.com:8080
		if idx := strings.LastIndex(domain, ":"); idx != -1 {
			// Make sure it's not an IPv6 address without brackets
			// IPv6 without brackets will have multiple colons
			colonCount := strings.Count(domain, ":")
			if colonCount == 1 {
				// Single colon = port separator
				domain = domain[:idx]
			}
			// Multiple colons = IPv6, keep as-is
		}
	}

	// Remove trailing dots
	domain = strings.TrimSuffix(domain, ".")

	return domain
}
