package dialer

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/sagernet/sing-box/option"
)

func TestValidateIPv6SourceAddressRangeOptions_ValidPrefixesAndDefaultMode(t *testing.T) {
	testCases := []string{
		"2001:db8::/0",
		"2001:db8::/48",
		"2001:db8::/63",
		"2001:db8::/64",
	}

	for _, prefix := range testCases {
		t.Run(prefix, func(t *testing.T) {
			dialOptions := mustDialerOptionsFromJSON(t, `{"ipv6_source_address_range":"`+prefix+`"}`)
			err := validateIPv6SourceAddressRangeOptions(&dialOptions)
			if err != nil {
				t.Fatalf("validateIPv6SourceAddressRangeOptions() error = %v", err)
			}
			if dialOptions.IPv6SourceAddressMode != option.IPv6SourceAddressModeRandom {
				t.Fatalf("expected default mode %q, got %q", option.IPv6SourceAddressModeRandom, dialOptions.IPv6SourceAddressMode)
			}
		})
	}
}

func TestNewWithOptions_IPv6SourceAddressRangeValidationErrors(t *testing.T) {
	testCases := []struct {
		name        string
		jsonOptions string
	}{
		{
			name:        "prefix narrower than /64",
			jsonOptions: `{"ipv6_source_address_range":"2001:db8::/65"}`,
		},
		{
			name:        "ipv4 cidr rejected",
			jsonOptions: `{"ipv6_source_address_range":"10.0.0.0/8"}`,
		},
		{
			name:        "conflict with inet6_bind_prefix",
			jsonOptions: `{"ipv6_source_address_range":"2001:db8::/48","inet6_bind_prefix":"2001:db8:1::/64"}`,
		},
		{
			name:        "conflict with inet6_bind_address",
			jsonOptions: `{"ipv6_source_address_range":"2001:db8::/48","inet6_bind_address":"2001:db8::1"}`,
		},
		{
			name:        "unsupported mode",
			jsonOptions: `{"ipv6_source_address_range":"2001:db8::/48","ipv6_source_address_mode":"unknown"}`,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			dialOptions := mustDialerOptionsFromJSON(t, tt.jsonOptions)
			_, err := NewWithOptions(Options{
				Context:        context.Background(),
				Options:        dialOptions,
				DirectOutbound: true,
			})
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
		})
	}
}

func TestNewWithOptions_IPv6SourceAddressRangeIgnoredForNonDirect(t *testing.T) {
	dialOptions := mustDialerOptionsFromJSON(t, `{"ipv6_source_address_range":"2001:db8::/48","ipv6_source_address_mode":"unknown"}`)
	_, err := NewWithOptions(Options{
		Context: context.Background(),
		Options: dialOptions,
	})
	if err != nil {
		t.Fatalf("expected no validation error for non-direct dialer, got %v", err)
	}
}

func mustDialerOptionsFromJSON(t *testing.T, content string) option.DialerOptions {
	t.Helper()
	var dialOptions option.DialerOptions
	err := json.Unmarshal([]byte(content), &dialOptions)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return dialOptions
}
