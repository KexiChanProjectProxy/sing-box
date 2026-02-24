package domain

import (
	"net"
	"strings"

	"golang.org/x/net/publicsuffix"
)

// ExtractETLDPlusOne extracts the eTLD+1 (effective Top-Level Domain plus one label)
// from a domain name using the Public Suffix List.
//
// Examples:
//   - "example.com" -> "example.com"
//   - "a.b.example.com" -> "example.com"
//   - "example.co.uk" -> "example.co.uk"
//   - "a.b.example.co.uk" -> "example.co.uk"
//   - "Example.COM:443" -> "example.com"
//   - "192.168.1.1" -> "-" (IP addresses are not domains)
//   - "" -> "-" (empty input)
//
// The function normalizes the input domain by:
//   - Converting to lowercase
//   - Stripping trailing dots
//   - Stripping port numbers (e.g., "example.com:443" -> "example.com")
//
// If the input is an IP address or cannot be parsed, returns "-" as a placeholder.
// If PSL extraction fails but the domain is valid, returns the full normalized domain.
func ExtractETLDPlusOne(rawDomain string) string {
	// Handle empty input
	if rawDomain == "" {
		return "-"
	}

	// Normalize: lowercase
	domain := strings.ToLower(rawDomain)

	// Strip trailing dots first (before port stripping, as port may have trailing dot like :443.)
	domain = strings.TrimSuffix(domain, ".")

	// Strip port if present
	domain = stripPort(domain)

	// Check if the result is empty after normalization
	if domain == "" {
		return "-"
	}

	// Check if it's an IP address
	if net.ParseIP(domain) != nil {
		return "-"
	}

	// Try to extract eTLD+1 using Public Suffix List
	etldPlusOne, err := publicsuffix.EffectiveTLDPlusOne(domain)
	if err != nil {
		// Fallback: if PSL extraction fails, return the full normalized domain
		// This can happen for:
		// - Invalid domains
		// - TLDs not in the PSL
		// - Domains with unknown suffixes
		return domain
	}

	return etldPlusOne
}

// stripPort removes the port from a host:port string.
// If no port is present, returns the original string unchanged.
func stripPort(hostPort string) string {
	// Handle IPv6 addresses like [::1]:8080
	if strings.HasPrefix(hostPort, "[") {
		// Find the closing bracket
		closeBracket := strings.Index(hostPort, "]")
		if closeBracket == -1 {
			// Malformed, return as-is
			return hostPort
		}
		// Check if there's a port after the bracket
		remainder := hostPort[closeBracket+1:]
		if strings.HasPrefix(remainder, ":") {
			// Return just the IPv6 address without brackets
			return hostPort[1:closeBracket]
		}
		// No port, return the IPv6 address without brackets
		return hostPort[1:closeBracket]
	}

	// Check if it's a bare IPv6 address (contains colons but no brackets)
	// IPv6 addresses like "2001:db8::1" or "::1" should not be processed for port stripping
	colonCount := strings.Count(hostPort, ":")
	if colonCount > 1 {
		// Multiple colons indicate IPv6 address - don't try to strip port
		// (Domains can't have colons, only IPv6 addresses can)
		return hostPort
	}

	// For regular domains/IPv4, split by last colon
	lastColon := strings.LastIndex(hostPort, ":")
	if lastColon == -1 {
		// No port
		return hostPort
	}

	// Check if what follows the colon looks like a port (all digits)
	potentialPort := hostPort[lastColon+1:]
	if potentialPort == "" {
		// Colon at the end, strip it
		return hostPort[:lastColon]
	}

	// Check if it's all digits (port number)
	for _, ch := range potentialPort {
		if ch < '0' || ch > '9' {
			// Not a port, might be part of the domain (unlikely but possible)
			return hostPort
		}
	}

	// It's a port, strip it
	return hostPort[:lastColon]
}
