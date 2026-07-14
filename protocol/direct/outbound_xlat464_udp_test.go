package direct

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/log"
	M "github.com/sagernet/sing/common/metadata"
)

func TestXLAT464UDPListenPacket(t *testing.T) {
	// Given
	base := &xlat464TestDialer{packetConn: &xlat464TestPacketConn{}}
	outbound := newXLAT464UDPTestOutbound(t, base, true)
	destination := M.SocksaddrFrom(netip.MustParseAddr("192.0.2.1"), 53)

	// When
	conn, err := outbound.ListenPacket(context.Background(), destination)

	// Then
	xlat464NoError(t, err)
	if got, want := base.lastDestination(), M.SocksaddrFrom(netip.MustParseAddr("64:ff9b::c000:201"), 53); got != want {
		t.Fatalf("packet listener destination: got %v, want %v", got, want)
	}
	prefixed := net.UDPAddrFromAddrPort(netip.MustParseAddrPort("[64:ff9b::c000:201]:53"))
	if _, err = conn.WriteTo([]byte("request"), net.UDPAddrFromAddrPort(netip.MustParseAddrPort("192.0.2.1:53"))); err != nil {
		t.Fatal(err)
	}
	if got := base.packetConn.writeToAddr.(*net.UDPAddr).AddrPort(); got != prefixed.AddrPort() {
		t.Fatalf("packet destination: got %v, want %v", got, prefixed.AddrPort())
	}
	base.packetConn.readFromAddr = prefixed
	if _, source, err := conn.ReadFrom(make([]byte, 64)); err != nil {
		t.Fatal(err)
	} else if got, want := source.(*net.UDPAddr).AddrPort(), netip.MustParseAddrPort("192.0.2.1:53"); got != want {
		t.Fatalf("reply source: got %v, want %v", got, want)
	}
}

func TestXLAT464UDPNonMatchingIPv6(t *testing.T) {
	// Given
	base := &xlat464TestDialer{packetConn: &xlat464TestPacketConn{readFromAddr: net.UDPAddrFromAddrPort(netip.MustParseAddrPort("[2001:db8::1]:53"))}}
	outbound := newXLAT464UDPTestOutbound(t, base, true)

	// When
	conn, err := outbound.ListenPacket(context.Background(), M.SocksaddrFrom(netip.MustParseAddr("192.0.2.1"), 53))

	// Then
	xlat464NoError(t, err)
	if _, source, err := conn.ReadFrom(make([]byte, 64)); err != nil {
		t.Fatal(err)
	} else if got, want := source.(*net.UDPAddr).AddrPort(), netip.MustParseAddrPort("[2001:db8::1]:53"); got != want {
		t.Fatalf("non-matching reply source: got %v, want %v", got, want)
	}
}

func TestXLAT464UDPDeadline(t *testing.T) {
	// Given
	base := &xlat464TestDialer{packetConn: &xlat464TestPacketConn{}}
	outbound := newXLAT464UDPTestOutbound(t, base, true)
	deadline := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)

	// When
	conn, err := outbound.ListenPacket(context.Background(), M.SocksaddrFrom(netip.MustParseAddr("192.0.2.1"), 53))
	xlat464NoError(t, err)
	xlat464NoError(t, conn.SetDeadline(deadline))
	xlat464NoError(t, conn.SetReadDeadline(deadline))
	xlat464NoError(t, conn.SetWriteDeadline(deadline))

	// Then
	if got := base.packetConn.deadline; got != deadline {
		t.Fatalf("deadline: got %v, want %v", got, deadline)
	}
	if got := base.packetConn.readDeadline; got != deadline {
		t.Fatalf("read deadline: got %v, want %v", got, deadline)
	}
	if got := base.packetConn.writeDeadline; got != deadline {
		t.Fatalf("write deadline: got %v, want %v", got, deadline)
	}
}

func TestXLAT464UDPListenSerialNetworkPacket(t *testing.T) {
	// Given
	base := &xlat464TestDialer{packetConn: &xlat464TestPacketConn{}}
	outbound := newXLAT464UDPTestOutbound(t, base, true)
	destination := M.ParseSocksaddrHostPort("example.com", 53)

	// When
	_, selectedAddress, err := outbound.ListenSerialNetworkPacket(context.Background(), destination, []netip.Addr{
		netip.MustParseAddr("2001:db8::1"),
		netip.MustParseAddr("192.0.2.1"),
	}, nil, nil, nil, 0)

	// Then
	xlat464NoError(t, err)
	if got, want := base.lastDestination(), M.SocksaddrFrom(netip.MustParseAddr("64:ff9b::c000:201"), 53); got != want {
		t.Fatalf("packet socket destination: got %v, want %v", got, want)
	}
	if got, want := selectedAddress, netip.MustParseAddr("192.0.2.1"); got != want {
		t.Fatalf("logical selected destination: got %v, want %v", got, want)
	}
}

func TestXLAT464UDPDisabled(t *testing.T) {
	// Given
	base := &xlat464TestDialer{packetConn: &xlat464TestPacketConn{}}
	outbound := newXLAT464UDPTestOutbound(t, base, false)
	destination := M.SocksaddrFrom(netip.MustParseAddr("192.0.2.1"), 53)

	// When
	conn, err := outbound.ListenPacket(context.Background(), destination)

	// Then
	xlat464NoError(t, err)
	if _, wrapped := conn.(*xlat464PacketConn); wrapped {
		t.Fatal("disabled packet listener is wrapped")
	}
	if got := base.lastDestination(); got != destination {
		t.Fatalf("packet listener destination: got %v, want %v", got, destination)
	}
	if _, err = conn.WriteTo([]byte("request"), net.UDPAddrFromAddrPort(netip.MustParseAddrPort("192.0.2.1:53"))); err != nil {
		t.Fatal(err)
	}
	if got, want := base.packetConn.writeToAddr.(*net.UDPAddr).AddrPort(), netip.MustParseAddrPort("192.0.2.1:53"); got != want {
		t.Fatalf("packet destination: got %v, want %v", got, want)
	}
}

func newXLAT464UDPTestOutbound(t *testing.T, base *xlat464TestDialer, enabled bool) *Outbound {
	t.Helper()
	var outboundDialer dialer.ParallelInterfaceDialer = base
	var mapper *xlat464AddressMapper
	if enabled {
		addressMapper, err := newXLAT464AddressMapper(xlat464TestOptions("64:ff9b::/96"))
		xlat464NoError(t, err)
		mapper = &addressMapper
		mappedDialer, err := newXLAT464Dialer(base, xlat464TestOptions("64:ff9b::/96"))
		xlat464NoError(t, err)
		outboundDialer = xlat464OutboundTestParallelDialer{mappedDialer}
	}
	return &Outbound{logger: log.NewNOPFactory().Logger(), dialer: outboundDialer, xlat464: mapper}
}
