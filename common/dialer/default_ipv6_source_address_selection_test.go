package dialer

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"syscall"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	boxLog "github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
)

func TestDefaultDialerIPv6SourceAddressSelectionIPv4Bypass(t *testing.T) {
	d := &DefaultDialer{
		logger:                 boxLog.StdLogger(),
		ipv6SourceAddressRange: netip.MustParsePrefix("2001:db8::/48"),
		ipv6SourceAddressMode:  option.IPv6SourceAddressMode("hash_5tuple"),
	}

	candidate, loaded := d.selectIPv6SourceAddressCandidate(context.Background(), M.SocksaddrFrom(netip.MustParseAddr("198.51.100.1"), 443))
	if loaded {
		t.Fatalf("expected IPv4 destination to bypass selection, got %s", candidate.Address)
	}
}

func TestDefaultDialerIPv6SourceAddressSelectionNoCandidate(t *testing.T) {
	d := &DefaultDialer{
		logger:                 boxLog.StdLogger(),
		ipv6SourceAddressRange: netip.MustParsePrefix("2001:db8::/48"),
		ipv6SourceAddressMode:  option.IPv6SourceAddressMode("hash_5tuple"),
	}

	candidate, loaded := d.selectIPv6SourceAddressCandidate(context.Background(), M.SocksaddrFrom(netip.MustParseAddr("2001:db8::53"), 443))
	if loaded {
		t.Fatalf("expected no candidate without 5-tuple metadata, got %s", candidate.Address)
	}
}

func TestDefaultDialerIPv6SourceAddressSelectionIPv6Candidate(t *testing.T) {
	d := &DefaultDialer{
		logger:                 boxLog.StdLogger(),
		ipv6SourceAddressRange: netip.MustParsePrefix("2001:db8:abcd::/48"),
		ipv6SourceAddressMode:  option.IPv6SourceAddressModeRandom,
	}

	destination := M.SocksaddrFrom(netip.MustParseAddr("2001:db8::53"), 443)
	candidate, loaded := d.selectIPv6SourceAddressCandidate(context.Background(), destination)
	if !loaded {
		t.Fatal("expected IPv6 destination to select source candidate")
	}
	if !d.ipv6SourceAddressRange.Contains(candidate.Address) {
		t.Fatalf("selected address %s is outside configured range %s", candidate.Address, d.ipv6SourceAddressRange)
	}
	if !candidate.Prefix64.Contains(candidate.Address) {
		t.Fatalf("selected address %s is outside derived prefix %s", candidate.Address, candidate.Prefix64)
	}
}

func TestIsIPv6SourceAddressBindFailed(t *testing.T) {
	testCases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "syscall eaddrnotavail",
			err:  syscall.EADDRNOTAVAIL,
			want: true,
		},
		{
			name: "wrapped bind operation",
			err:  &net.OpError{Op: "bind", Err: errors.New("bind failed")},
			want: true,
		},
		{
			name: "wrapped in sing exception",
			err:  E.Cause(syscall.EAFNOSUPPORT, "dial eth0"),
			want: true,
		},
		{
			name: "generic error",
			err:  errors.New("connection refused"),
			want: false,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			if got := isIPv6SourceAddressBindFailed(tt.err); got != tt.want {
				t.Fatalf("isIPv6SourceAddressBindFailed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultDialerIPv6SourceAddressSelectionHashUsesInboundContext(t *testing.T) {
	d := &DefaultDialer{
		logger:                 boxLog.StdLogger(),
		ipv6SourceAddressRange: netip.MustParsePrefix("2001:db8:abcd::/48"),
		ipv6SourceAddressMode:  option.IPv6SourceAddressMode("hash_5tuple"),
	}

	ctx := adapter.WithContext(context.Background(), &adapter.InboundContext{
		Network:     "tcp",
		Source:      M.SocksaddrFrom(netip.MustParseAddr("192.0.2.10"), 12345),
		Destination: M.SocksaddrFrom(netip.MustParseAddr("2001:db8::53"), 443),
	})

	candidate, loaded := d.selectIPv6SourceAddressCandidate(ctx, M.SocksaddrFrom(netip.MustParseAddr("2001:db8::53"), 443))
	if !loaded {
		t.Fatal("expected hash mode to produce candidate with complete inbound metadata")
	}
	if !d.ipv6SourceAddressRange.Contains(candidate.Address) {
		t.Fatalf("selected address %s is outside configured range %s", candidate.Address, d.ipv6SourceAddressRange)
	}
}
