package option

import (
	"encoding/json"
	"testing"
)

func TestDialerOptionsUnmarshal_InvalidIPv6SourceAddressRange(t *testing.T) {
	var dialOptions DialerOptions
	err := json.Unmarshal([]byte(`{"ipv6_source_address_range":"not-a-cidr"}`), &dialOptions)
	if err == nil {
		t.Fatal("expected unmarshal error for malformed ipv6_source_address_range")
	}
}
