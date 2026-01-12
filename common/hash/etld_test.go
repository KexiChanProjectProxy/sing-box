package hash

import (
	"testing"
)

func TestExtractETLDPlusOne_StandardDomains(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple domain",
			input:    "example.com",
			expected: "example.com",
		},
		{
			name:     "subdomain",
			input:    "www.example.com",
			expected: "example.com",
		},
		{
			name:     "multiple subdomains",
			input:    "a.b.c.example.com",
			expected: "example.com",
		},
		{
			name:     "domain with port",
			input:    "www.example.com:8080",
			expected: "example.com",
		},
		{
			name:     "domain with trailing dot",
			input:    "www.example.com.",
			expected: "example.com",
		},
		{
			name:     "uppercase domain",
			input:    "WWW.EXAMPLE.COM",
			expected: "example.com",
		},
		{
			name:     "mixed case domain",
			input:    "Www.Example.Com",
			expected: "example.com",
		},
		{
			name:     "domain with whitespace",
			input:    "  example.com  ",
			expected: "example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractETLDPlusOne(tt.input)
			if result != tt.expected {
				t.Errorf("ExtractETLDPlusOne(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractETLDPlusOne_MultiPartTLDs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "co.uk TLD",
			input:    "example.co.uk",
			expected: "example.co.uk",
		},
		{
			name:     "subdomain with co.uk",
			input:    "www.example.co.uk",
			expected: "example.co.uk",
		},
		{
			name:     "multiple subdomains with co.uk",
			input:    "a.b.example.co.uk",
			expected: "example.co.uk",
		},
		{
			name:     "gov.uk TLD",
			input:    "example.gov.uk",
			expected: "example.gov.uk",
		},
		{
			name:     "ac.jp TLD",
			input:    "example.ac.jp",
			expected: "example.ac.jp",
		},
		{
			name:     "subdomain with ac.jp",
			input:    "www.example.ac.jp",
			expected: "example.ac.jp",
		},
		{
			name:     "com.cn TLD",
			input:    "example.com.cn",
			expected: "example.com.cn",
		},
		{
			name:     "co.za TLD",
			input:    "www.example.co.za",
			expected: "example.co.za",
		},
		{
			name:     "org.au TLD",
			input:    "subdomain.example.org.au",
			expected: "example.org.au",
		},
		{
			name:     "should not extract TLD itself",
			input:    "co.uk",
			expected: "co.uk", // publicsuffix returns the input if it's the TLD itself
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractETLDPlusOne(tt.input)
			if result != tt.expected {
				t.Errorf("ExtractETLDPlusOne(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractETLDPlusOne_IPAddresses(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "IPv4 address",
			input:    "192.168.1.1",
			expected: "-",
		},
		{
			name:     "IPv4 with port",
			input:    "192.168.1.1:8080",
			expected: "-",
		},
		{
			name:     "IPv6 address",
			input:    "2001:db8::1",
			expected: "-",
		},
		{
			name:     "IPv6 loopback",
			input:    "::1",
			expected: "-",
		},
		{
			name:     "IPv6 in brackets",
			input:    "[2001:db8::1]",
			expected: "-",
		},
		{
			name:     "IPv6 in brackets with port",
			input:    "[2001:db8::1]:8080",
			expected: "-",
		},
		{
			name:     "IPv6 full address",
			input:    "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			expected: "-",
		},
		{
			name:     "localhost IPv4",
			input:    "127.0.0.1",
			expected: "-",
		},
		{
			name:     "zero IPv4",
			input:    "0.0.0.0",
			expected: "-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractETLDPlusOne(tt.input)
			if result != tt.expected {
				t.Errorf("ExtractETLDPlusOne(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractETLDPlusOne_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "-",
		},
		{
			name:     "localhost",
			input:    "localhost",
			expected: "localhost",
		},
		{
			name:     "localhost with port",
			input:    "localhost:8080",
			expected: "localhost",
		},
		{
			name:     "single label",
			input:    "example",
			expected: "example",
		},
		{
			name:     "just TLD",
			input:    "com",
			expected: "com",
		},
		{
			name:     "invalid domain with special chars",
			input:    "example@test.com",
			expected: "example@test.com", // Returns as-is if publicsuffix fails
		},
		{
			name:     "domain with underscores",
			input:    "sub_domain.example.com",
			expected: "example.com",
		},
		{
			name:     "domain with hyphens",
			input:    "sub-domain.example.com",
			expected: "example.com",
		},
		{
			name:     "very long subdomain chain",
			input:    "a.b.c.d.e.f.g.h.i.j.example.com",
			expected: "example.com",
		},
		{
			name:     "domain with numbers",
			input:    "123.456.example.com",
			expected: "example.com",
		},
		{
			name:     "punycode domain",
			input:    "xn--n3h.example.com", // ☃.example.com in punycode
			expected: "example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractETLDPlusOne(tt.input)
			if result != tt.expected {
				t.Errorf("ExtractETLDPlusOne(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeDomain(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "lowercase",
			input:    "EXAMPLE.COM",
			expected: "example.com",
		},
		{
			name:     "remove port",
			input:    "example.com:8080",
			expected: "example.com",
		},
		{
			name:     "remove trailing dot",
			input:    "example.com.",
			expected: "example.com",
		},
		{
			name:     "trim whitespace",
			input:    "  example.com  ",
			expected: "example.com",
		},
		{
			name:     "all normalizations",
			input:    "  EXAMPLE.COM:8080.  ",
			expected: "example.com",
		},
		{
			name:     "IPv6 in brackets with port",
			input:    "[2001:db8::1]:8080",
			expected: "[2001:db8::1]",
		},
		{
			name:     "IPv6 without brackets keeps colons",
			input:    "2001:db8::1",
			expected: "2001:db8::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeDomain(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeDomain(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
