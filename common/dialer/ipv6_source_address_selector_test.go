package dialer

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
)

func TestIPv6SourceAddressSelectorRandomRangeContainment(t *testing.T) {
	testCases := []string{
		"::/0",
		"2001:db8:abcd::/48",
		"2001:db8:abcd:1234::/63",
		"2001:db8:abcd:1234::/64",
	}

	for _, prefixString := range testCases {
		t.Run(prefixString, func(t *testing.T) {
			prefix := netip.MustParsePrefix(prefixString)
			candidate, loaded := selectIPv6SourceAddressCandidateWithReader(
				prefix,
				option.IPv6SourceAddressModeRandom,
				nil,
				randomReaderFromUint64(0x0123456789abcdef, 0xfedcba9876543210),
			)
			if !loaded {
				t.Fatal("expected candidate, got no candidate")
			}
			if !prefix.Contains(candidate.Address) {
				t.Fatalf("generated address %s is not in configured prefix %s", candidate.Address, prefix)
			}
			if candidate.Prefix64.Bits() != 64 {
				t.Fatalf("expected derived prefix length 64, got %d", candidate.Prefix64.Bits())
			}
			if !candidate.Prefix64.Contains(candidate.Address) {
				t.Fatalf("generated address %s is not in derived prefix %s", candidate.Address, candidate.Prefix64)
			}
			if !prefix.Contains(candidate.Prefix64.Addr()) {
				t.Fatalf("derived prefix %s is not inside configured prefix %s", candidate.Prefix64, prefix)
			}
			if prefix.Bits() == 64 && candidate.Prefix64 != prefix.Masked() {
				t.Fatalf("expected fixed /64 %s, got %s", prefix.Masked(), candidate.Prefix64)
			}
		})
	}
}

func TestIPv6SourceAddressSelectorRandomVariesPrefixAndHost(t *testing.T) {
	prefix := netip.MustParsePrefix("2001:db8:abcd::/48")

	first, loaded := selectIPv6SourceAddressCandidateWithReader(
		prefix,
		option.IPv6SourceAddressModeRandom,
		nil,
		randomReaderFromUint64(0x0000000000001111, 0x0000000000002222),
	)
	if !loaded {
		t.Fatal("expected first candidate, got no candidate")
	}

	second, loaded := selectIPv6SourceAddressCandidateWithReader(
		prefix,
		option.IPv6SourceAddressModeRandom,
		nil,
		randomReaderFromUint64(0x0000000000003333, 0x0000000000004444),
	)
	if !loaded {
		t.Fatal("expected second candidate, got no candidate")
	}

	if first.Prefix64 == second.Prefix64 {
		t.Fatalf("expected different derived /64 prefixes, got %s and %s", first.Prefix64, second.Prefix64)
	}
	if first.Address == second.Address {
		t.Fatalf("expected different random addresses, got %s and %s", first.Address, second.Address)
	}
}

func TestIPv6SourceAddressSelectorRandom64KeepsConfiguredPrefix(t *testing.T) {
	prefix := netip.MustParsePrefix("2001:db8:abcd:1234::/64")

	first, loaded := selectIPv6SourceAddressCandidateWithReader(
		prefix,
		option.IPv6SourceAddressModeRandom,
		nil,
		randomReaderFromUint64(0x0000000000001111),
	)
	if !loaded {
		t.Fatal("expected first candidate, got no candidate")
	}

	second, loaded := selectIPv6SourceAddressCandidateWithReader(
		prefix,
		option.IPv6SourceAddressModeRandom,
		nil,
		randomReaderFromUint64(0x0000000000002222),
	)
	if !loaded {
		t.Fatal("expected second candidate, got no candidate")
	}

	if first.Prefix64 != prefix.Masked() || second.Prefix64 != prefix.Masked() {
		t.Fatalf("expected fixed configured /64 %s, got %s and %s", prefix.Masked(), first.Prefix64, second.Prefix64)
	}
	if first.Address == second.Address {
		t.Fatalf("expected different randomized /128 addresses, got %s and %s", first.Address, second.Address)
	}
}

func TestIPv6SourceAddressSelectorHashRangeContainment(t *testing.T) {
	metadata := newInboundContext5Tuple(
		"tcp",
		"192.0.2.10",
		12345,
		"2001:db8::53",
		443,
	)

	testCases := []string{
		"::/0",
		"2001:db8:abcd::/48",
		"2001:db8:abcd:1234::/63",
		"2001:db8:abcd:1234::/64",
	}

	for _, prefixString := range testCases {
		t.Run(prefixString, func(t *testing.T) {
			prefix := netip.MustParsePrefix(prefixString)
			candidate, loaded := selectIPv6SourceAddressCandidateWithReader(
				prefix,
				ipv6SourceAddressModeHash5Tuple,
				metadata,
				randomReaderFromUint64(0x0102030405060708),
			)
			if !loaded {
				t.Fatal("expected candidate, got no candidate")
			}
			if !prefix.Contains(candidate.Address) {
				t.Fatalf("generated address %s is not in configured prefix %s", candidate.Address, prefix)
			}
			if !candidate.Prefix64.Contains(candidate.Address) {
				t.Fatalf("generated address %s is not in derived prefix %s", candidate.Address, candidate.Prefix64)
			}
			if !prefix.Contains(candidate.Prefix64.Addr()) {
				t.Fatalf("derived prefix %s is not inside configured prefix %s", candidate.Prefix64, prefix)
			}
			if prefix.Bits() == 64 && candidate.Prefix64 != prefix.Masked() {
				t.Fatalf("expected fixed /64 %s, got %s", prefix.Masked(), candidate.Prefix64)
			}
		})
	}
}

func TestIPv6SourceAddressSelectorHashStablePrefix64(t *testing.T) {
	prefix := netip.MustParsePrefix("2001:db8:abcd::/48")
	metadata := newInboundContext5Tuple(
		"udp",
		"198.51.100.2",
		5353,
		"2001:db8::1",
		853,
	)

	first, loaded := selectIPv6SourceAddressCandidateWithReader(
		prefix,
		ipv6SourceAddressModeHash5Tuple,
		metadata,
		randomReaderFromUint64(0x0000000000001111),
	)
	if !loaded {
		t.Fatal("expected first candidate, got no candidate")
	}

	second, loaded := selectIPv6SourceAddressCandidateWithReader(
		prefix,
		ipv6SourceAddressModeHash5Tuple,
		metadata,
		randomReaderFromUint64(0x0000000000002222),
	)
	if !loaded {
		t.Fatal("expected second candidate, got no candidate")
	}

	if first.Prefix64 != second.Prefix64 {
		t.Fatalf("expected stable derived /64, got %s and %s", first.Prefix64, second.Prefix64)
	}
	if first.Address == second.Address {
		t.Fatalf("expected different randomized /128 addresses, got %s and %s", first.Address, second.Address)
	}
	firstBytes := first.Address.As16()
	secondBytes := second.Address.As16()
	if binary.BigEndian.Uint64(firstBytes[:8]) != binary.BigEndian.Uint64(secondBytes[:8]) {
		t.Fatalf("expected identical upper 64 bits, got %s and %s", first.Address, second.Address)
	}
}

func TestIPv6SourceAddressSelectorHashUsesFull5Tuple(t *testing.T) {
	prefix := netip.MustParsePrefix("2001:db8:abcd::/48")
	base := newInboundContext5Tuple(
		"tcp",
		"192.0.2.10",
		12345,
		"2001:db8::53",
		443,
	)

	baseCandidate, loaded := selectIPv6SourceAddressCandidateWithReader(
		prefix,
		ipv6SourceAddressModeHash5Tuple,
		base,
		randomReaderFromUint64(0x0102030405060708),
	)
	if !loaded {
		t.Fatal("expected base candidate, got no candidate")
	}

	testCases := []struct {
		name     string
		metadata *adapter.InboundContext
	}{
		{
			name: "network",
			metadata: newInboundContext5Tuple(
				"udp",
				"192.0.2.10",
				12345,
				"2001:db8::53",
				443,
			),
		},
		{
			name: "source_ip",
			metadata: newInboundContext5Tuple(
				"tcp",
				"192.0.2.11",
				12345,
				"2001:db8::53",
				443,
			),
		},
		{
			name: "source_port",
			metadata: newInboundContext5Tuple(
				"tcp",
				"192.0.2.10",
				12346,
				"2001:db8::53",
				443,
			),
		},
		{
			name: "destination_ip",
			metadata: newInboundContext5Tuple(
				"tcp",
				"192.0.2.10",
				12345,
				"2001:db8::54",
				443,
			),
		},
		{
			name: "destination_port",
			metadata: newInboundContext5Tuple(
				"tcp",
				"192.0.2.10",
				12345,
				"2001:db8::53",
				444,
			),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			candidate, loaded := selectIPv6SourceAddressCandidateWithReader(
				prefix,
				ipv6SourceAddressModeHash5Tuple,
				tt.metadata,
				randomReaderFromUint64(0x0102030405060708),
			)
			if !loaded {
				t.Fatal("expected candidate, got no candidate")
			}
			if candidate.Prefix64 == baseCandidate.Prefix64 {
				t.Fatalf("expected %s change to alter derived /64, got same prefix %s", tt.name, candidate.Prefix64)
			}
		})
	}
}

func TestIPv6SourceAddressSelectorHashMissingMetadata(t *testing.T) {
	prefix := netip.MustParsePrefix("2001:db8:abcd::/48")

	testCases := []struct {
		name     string
		metadata *adapter.InboundContext
	}{
		{name: "nil metadata"},
		{
			name:     "missing network",
			metadata: newInboundContext5Tuple("", "192.0.2.10", 12345, "2001:db8::53", 443),
		},
		{
			name:     "missing source address",
			metadata: newInboundContext5Tuple("tcp", "", 12345, "2001:db8::53", 443),
		},
		{
			name:     "missing source port",
			metadata: newInboundContext5Tuple("tcp", "192.0.2.10", 0, "2001:db8::53", 443),
		},
		{
			name:     "missing destination address",
			metadata: newInboundContext5Tuple("tcp", "192.0.2.10", 12345, "", 443),
		},
		{
			name:     "missing destination port",
			metadata: newInboundContext5Tuple("tcp", "192.0.2.10", 12345, "2001:db8::53", 0),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			candidate, loaded := selectIPv6SourceAddressCandidateWithReader(
				prefix,
				ipv6SourceAddressModeHash5Tuple,
				tt.metadata,
				randomReaderFromUint64(0x0102030405060708),
			)
			if loaded {
				t.Fatalf("expected no candidate, got %s in %s", candidate.Address, candidate.Prefix64)
			}
			if candidate.Address.IsValid() {
				t.Fatalf("expected invalid address in no-candidate state, got %s", candidate.Address)
			}
			if candidate.Prefix64.IsValid() {
				t.Fatalf("expected invalid prefix in no-candidate state, got %s", candidate.Prefix64)
			}
		})
	}
}

func randomReaderFromUint64(values ...uint64) *bytes.Reader {
	bytesValue := make([]byte, 8*len(values))
	for index, value := range values {
		binary.BigEndian.PutUint64(bytesValue[index*8:], value)
	}
	return bytes.NewReader(bytesValue)
}

func newInboundContext5Tuple(network string, sourceIP string, sourcePort uint16, destinationIP string, destinationPort uint16) *adapter.InboundContext {
	var sourceAddr netip.Addr
	if sourceIP != "" {
		sourceAddr = netip.MustParseAddr(sourceIP)
	}
	var destinationAddr netip.Addr
	if destinationIP != "" {
		destinationAddr = netip.MustParseAddr(destinationIP)
	}
	return &adapter.InboundContext{
		Network:     network,
		Source:      M.SocksaddrFrom(sourceAddr, sourcePort),
		Destination: M.SocksaddrFrom(destinationAddr, destinationPort),
	}
}
