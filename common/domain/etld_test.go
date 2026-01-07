package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestExtractETLDPlusOne tests the main eTLD+1 extraction function
func TestExtractETLDPlusOne(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
		description string
	}{
		// Basic domains
		{
			name:        "simple .com domain",
			input:       "example.com",
			expected:    "example.com",
			description: "Simple domain should return itself",
		},
		{
			name:        "simple .org domain",
			input:       "mozilla.org",
			expected:    "mozilla.org",
			description: "Org domain should return itself",
		},

		// Subdomains
		{
			name:        "one level subdomain",
			input:       "www.example.com",
			expected:    "example.com",
			description: "Single subdomain should extract eTLD+1",
		},
		{
			name:        "two level subdomain",
			input:       "a.b.example.com",
			expected:    "example.com",
			description: "Multi-level subdomain should extract eTLD+1",
		},
		{
			name:        "deep subdomain",
			input:       "x.y.z.w.example.com",
			expected:    "example.com",
			description: "Deep subdomain should extract eTLD+1",
		},

		// Multi-part TLDs (.co.uk, .gov.uk, etc.)
		{
			name:        ".co.uk domain",
			input:       "example.co.uk",
			expected:    "example.co.uk",
			description: "UK domain should preserve co.uk suffix",
		},
		{
			name:        ".co.uk subdomain",
			input:       "www.example.co.uk",
			expected:    "example.co.uk",
			description: "UK subdomain should extract eTLD+1 with co.uk",
		},
		{
			name:        "deep .co.uk subdomain",
			input:       "a.b.example.co.uk",
			expected:    "example.co.uk",
			description: "Deep UK subdomain should extract eTLD+1",
		},
		{
			name:        ".gov.uk domain",
			input:       "example.gov.uk",
			expected:    "example.gov.uk",
			description: "Gov.uk domain should be handled correctly",
		},

		// Port stripping
		{
			name:        "domain with standard HTTPS port",
			input:       "example.com:443",
			expected:    "example.com",
			description: "Port 443 should be stripped",
		},
		{
			name:        "domain with standard HTTP port",
			input:       "example.com:80",
			expected:    "example.com",
			description: "Port 80 should be stripped",
		},
		{
			name:        "domain with custom port",
			input:       "example.com:8080",
			expected:    "example.com",
			description: "Custom port should be stripped",
		},
		{
			name:        "subdomain with port",
			input:       "www.example.com:443",
			expected:    "example.com",
			description: "Port should be stripped before eTLD extraction",
		},

		// Case normalization
		{
			name:        "uppercase domain",
			input:       "EXAMPLE.COM",
			expected:    "example.com",
			description: "Uppercase should be normalized to lowercase",
		},
		{
			name:        "mixed case domain",
			input:       "ExAmPlE.CoM",
			expected:    "example.com",
			description: "Mixed case should be normalized to lowercase",
		},
		{
			name:        "uppercase with port",
			input:       "EXAMPLE.COM:443",
			expected:    "example.com",
			description: "Uppercase with port should be normalized",
		},

		// Trailing dots
		{
			name:        "domain with trailing dot",
			input:       "example.com.",
			expected:    "example.com",
			description: "Trailing dot should be stripped",
		},
		{
			name:        "subdomain with trailing dot",
			input:       "www.example.com.",
			expected:    "example.com",
			description: "Subdomain with trailing dot should work",
		},
		{
			name:        "domain with port and trailing dot",
			input:       "example.com:443.",
			expected:    "example.com",
			description: "Port and trailing dot should both be stripped",
		},

		// IP addresses
		{
			name:        "IPv4 address",
			input:       "192.168.1.1",
			expected:    "-",
			description: "IPv4 address should return placeholder",
		},
		{
			name:        "IPv4 with port",
			input:       "192.168.1.1:8080",
			expected:    "-",
			description: "IPv4 with port should return placeholder",
		},
		{
			name:        "localhost IP",
			input:       "127.0.0.1",
			expected:    "-",
			description: "Localhost IP should return placeholder",
		},
		{
			name:        "IPv6 address",
			input:       "2001:db8::1",
			expected:    "-",
			description: "IPv6 address should return placeholder",
		},
		{
			name:        "IPv6 loopback",
			input:       "::1",
			expected:    "-",
			description: "IPv6 loopback should return placeholder",
		},

		// Edge cases
		{
			name:        "empty string",
			input:       "",
			expected:    "-",
			description: "Empty string should return placeholder",
		},
		{
			name:        "just a dot",
			input:       ".",
			expected:    "-",
			description: "Single dot should return placeholder",
		},
		{
			name:        "just a port",
			input:       ":443",
			expected:    "-",
			description: "Just port should return placeholder",
		},

		// Real-world examples
		{
			name:        "GitHub domain",
			input:       "github.com",
			expected:    "github.com",
			description: "GitHub domain should work",
		},
		{
			name:        "GitHub subdomain",
			input:       "api.github.com",
			expected:    "github.com",
			description: "GitHub API subdomain should extract github.com",
		},
		{
			name:        "Google domain",
			input:       "google.com",
			expected:    "google.com",
			description: "Google domain should work",
		},
		{
			name:        "Google mail",
			input:       "mail.google.com",
			expected:    "google.com",
			description: "Gmail should extract google.com",
		},
		{
			name:        "Google UK",
			input:       "www.google.co.uk",
			expected:    "google.co.uk",
			description: "Google UK should preserve co.uk",
		},
		{
			name:        "CDN subdomain",
			input:       "cdn1.example.com",
			expected:    "example.com",
			description: "CDN subdomain should extract base domain",
		},
		{
			name:        "Deep CDN path",
			input:       "s3.us-west-2.amazonaws.com",
			expected:    "s3.us-west-2.amazonaws.com",
			description: "AWS S3 regional endpoint is itself an eTLD+1 (PSL wildcard entry)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ExtractETLDPlusOne(tc.input)
			assert.Equal(t, tc.expected, result, tc.description)
		})
	}
}

// TestStripPort tests the port stripping helper function
func TestStripPort(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		// Basic cases
		{
			name:     "domain with port",
			input:    "example.com:443",
			expected: "example.com",
		},
		{
			name:     "domain without port",
			input:    "example.com",
			expected: "example.com",
		},

		// Various ports
		{
			name:     "port 80",
			input:    "example.com:80",
			expected: "example.com",
		},
		{
			name:     "port 8080",
			input:    "example.com:8080",
			expected: "example.com",
		},
		{
			name:     "high port",
			input:    "example.com:65535",
			expected: "example.com",
		},

		// IPv4
		{
			name:     "IPv4 with port",
			input:    "192.168.1.1:8080",
			expected: "192.168.1.1",
		},
		{
			name:     "IPv4 without port",
			input:    "192.168.1.1",
			expected: "192.168.1.1",
		},

		// IPv6
		{
			name:     "IPv6 with brackets and port",
			input:    "[2001:db8::1]:8080",
			expected: "2001:db8::1",
		},
		{
			name:     "IPv6 with brackets no port",
			input:    "[2001:db8::1]",
			expected: "2001:db8::1",
		},
		{
			name:     "IPv6 loopback with port",
			input:    "[::1]:443",
			expected: "::1",
		},

		// Edge cases
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "just colon",
			input:    ":",
			expected: "",
		},
		{
			name:     "trailing colon no port",
			input:    "example.com:",
			expected: "example.com",
		},
		{
			name:     "malformed IPv6",
			input:    "[::1",
			expected: "[::1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := stripPort(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestExtractETLDPlusOneConsistency tests that the function is deterministic
func TestExtractETLDPlusOneConsistency(t *testing.T) {
	testInputs := []string{
		"www.example.com",
		"api.example.com:443",
		"EXAMPLE.COM",
		"a.b.c.example.co.uk",
	}

	for _, input := range testInputs {
		t.Run(input, func(t *testing.T) {
			result1 := ExtractETLDPlusOne(input)
			result2 := ExtractETLDPlusOne(input)
			result3 := ExtractETLDPlusOne(input)

			assert.Equal(t, result1, result2, "Function should be deterministic")
			assert.Equal(t, result2, result3, "Function should be deterministic")
		})
	}
}

// TestExtractETLDPlusOneNormalization tests that different inputs normalize to the same result
func TestExtractETLDPlusOneNormalization(t *testing.T) {
	testCases := []struct {
		name     string
		inputs   []string
		expected string
	}{
		{
			name: "case variations",
			inputs: []string{
				"example.com",
				"EXAMPLE.COM",
				"Example.Com",
				"ExAmPlE.cOm",
			},
			expected: "example.com",
		},
		{
			name: "with and without port",
			inputs: []string{
				"example.com",
				"example.com:443",
				"example.com:80",
				"example.com:8080",
			},
			expected: "example.com",
		},
		{
			name: "with and without trailing dot",
			inputs: []string{
				"example.com",
				"example.com.",
			},
			expected: "example.com",
		},
		{
			name: "subdomain variations",
			inputs: []string{
				"www.example.com",
				"api.example.com",
				"cdn.example.com",
			},
			expected: "example.com",
		},
		{
			name: "complex normalization",
			inputs: []string{
				"www.example.com",
				"WWW.EXAMPLE.COM:443",
				"www.Example.Com.",
				"WWW.example.COM:80.",
			},
			expected: "example.com",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, input := range tc.inputs {
				result := ExtractETLDPlusOne(input)
				assert.Equal(t, tc.expected, result, "Input %q should normalize to %q", input, tc.expected)
			}
		})
	}
}
