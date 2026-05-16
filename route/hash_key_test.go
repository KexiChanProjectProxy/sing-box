package route

import (
	"net/netip"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	M "github.com/sagernet/sing/common/metadata"
)

func TestDeriveHashKeyPartMatchedRulesetOrETLD(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		metadata adapter.InboundContext
		expected string
	}{
		{
			name: "matched ruleset tag wins over etld",
			metadata: adapter.InboundContext{
				MatchedRuleSetTag: "geosite-private",
				Domain:            "www.Example.COM",
			},
			expected: "geosite-private",
		},
		{
			name: "domain fallback produces lowercase registered domain",
			metadata: adapter.InboundContext{
				Domain: "Example.COM",
			},
			expected: "example.com",
		},
		{
			name: "mixed-case subdomain normalizes to etld plus one",
			metadata: adapter.InboundContext{
				Domain: "API.News.Example.CO.UK",
			},
			expected: "example.co.uk",
		},
		{
			name: "destination fqdn fallback",
			metadata: adapter.InboundContext{
				Destination: M.Socksaddr{Fqdn: "WWW.Example.ORG"},
			},
			expected: "example.org",
		},
		{
			name: "ip destination returns empty",
			metadata: adapter.InboundContext{
				Destination: M.Socksaddr{Addr: netip.MustParseAddr("203.0.113.1")},
			},
			expected: "",
		},
		{
			name:     "empty domain returns empty",
			metadata: adapter.InboundContext{},
			expected: "",
		},
		{
			name: "no matched ruleset returns empty for ruleset part",
			metadata: adapter.InboundContext{
				Domain: "example.com",
			},
			expected: "example.com",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			actual := DeriveHashKeyPart(testCase.metadata, "matched_ruleset_or_etld")
			if actual != testCase.expected {
				t.Fatalf("expected %q, got %q", testCase.expected, actual)
			}
		})
	}
}

func TestDeriveHashKeyPartSourceIP(t *testing.T) {
	t.Parallel()

	metadata := adapter.InboundContext{
		Source: M.Socksaddr{Addr: netip.MustParseAddr("2001:db8::1")},
	}
	actual := DeriveHashKeyPart(metadata, "src_ip")
	if actual != "2001:db8::1" {
		t.Fatalf("expected source IP, got %q", actual)
	}
}

func TestDeriveHashKeyPartRulesetOnly(t *testing.T) {
	t.Parallel()

	metadata := adapter.InboundContext{Domain: "example.com"}
	actual := DeriveHashKeyPart(metadata, "matched_ruleset")
	if actual != "" {
		t.Fatalf("expected empty ruleset part without matched tag, got %q", actual)
	}
}
