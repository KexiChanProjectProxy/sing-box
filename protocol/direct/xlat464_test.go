package direct

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	B "github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

func TestXLAT464Synthesize(t *testing.T) {
	mapper, err := newXLAT464AddressMapper(xlat464TestOptions("64:ff9b::/96"))
	xlat464NoError(t, err)

	want := netip.MustParseAddr("64:ff9b::c000:201")
	for _, address := range []netip.Addr{
		netip.MustParseAddr("192.0.2.1"),
		netip.MustParseAddr("::ffff:192.0.2.1"),
	} {
		if got := mapper.synthesize(address); got.As16() != want.As16() {
			t.Fatalf("synthesize %v: got %v, want %v", address, got, want)
		}
	}

	nativeIPv6 := netip.MustParseAddr("2001:db8::1")
	if got := mapper.synthesize(nativeIPv6); got != nativeIPv6 {
		t.Fatalf("native IPv6 changed: got %v, want %v", got, nativeIPv6)
	}
}

func TestXLAT464Reverse(t *testing.T) {
	mapper, err := newXLAT464AddressMapper(xlat464TestOptions("64:ff9b::/96"))
	xlat464NoError(t, err)
	prefixed := netip.MustParseAddr("64:ff9b::c000:201")
	if got, want := mapper.reverse(prefixed), netip.MustParseAddr("192.0.2.1"); got != want {
		t.Fatalf("reverse: got %v, want %v", got, want)
	}
	for _, address := range []netip.Addr{netip.MustParseAddr("2001:db8::1"), netip.MustParseAddr("::ffff:192.0.2.1")} {
		if got := mapper.reverse(address); got != address {
			t.Fatalf("unexpected reverse mapping: got %v, want %v", got, address)
		}
	}
	if mapper.prefix.Contains(netip.MustParseAddr("::ffff:192.0.2.1")) {
		t.Fatal("/96 prefix unexpectedly contains IPv4-mapped address")
	}
}

func TestXLAT464RejectInvalidPrefix(t *testing.T) {
	for _, options := range []option.Xlat464Options{{}, xlat464TestOptions("192.0.2.0/24"), xlat464TestOptions("2001:db8::/64")} {
		if _, err := newXLAT464AddressMapper(options); err == nil {
			t.Fatal("expected invalid prefix error")
		}
	}
}

func TestXLAT464DialerMapsDestinations(t *testing.T) {
	base := &xlat464TestDialer{packetConn: &xlat464TestPacketConn{}}
	dialer, err := newXLAT464Dialer(base, xlat464TestOptions("64:ff9b::/96"))
	xlat464NoError(t, err)
	ctx := context.Background()
	ipv4 := M.SocksaddrFrom(netip.MustParseAddr("192.0.2.1"), 443)
	want := M.SocksaddrFrom(netip.MustParseAddr("64:ff9b::c000:201"), 443)

	if _, err = dialer.DialContext(ctx, N.NetworkTCP, ipv4); err != nil {
		t.Fatal(err)
	}
	if got := base.lastDestination(); got != want {
		t.Fatalf("DialContext destination: got %v, want %v", got, want)
	}
	if _, err = dialer.ListenPacket(ctx, M.SocksaddrFrom(netip.MustParseAddr("::ffff:192.0.2.1"), 443)); err != nil {
		t.Fatal(err)
	}
	if got := base.lastDestination(); got != want {
		t.Fatalf("ListenPacket destination: got %v, want %v", got, want)
	}
	if _, err = dialer.DialParallelInterface(ctx, N.NetworkTCP, ipv4, nil, nil, nil, 0); err != nil {
		t.Fatal(err)
	}
	if got := base.lastDestination(); got != want {
		t.Fatalf("DialParallelInterface destination: got %v, want %v", got, want)
	}
	if _, err = dialer.ListenSerialInterfacePacket(ctx, ipv4, nil, nil, nil, 0); err != nil {
		t.Fatal(err)
	}
	if got := base.lastDestination(); got != want {
		t.Fatalf("ListenSerialInterfacePacket destination: got %v, want %v", got, want)
	}

	nativeIPv6 := M.SocksaddrFrom(netip.MustParseAddr("2001:db8::1"), 443)
	if _, err = dialer.DialContext(ctx, N.NetworkTCP, nativeIPv6); err != nil {
		t.Fatal(err)
	}
	if got := base.lastDestination(); got != nativeIPv6 {
		t.Fatalf("native IPv6 destination: got %v, want %v", got, nativeIPv6)
	}
}

func TestXLAT464FilterParallelAddresses(t *testing.T) {
	base := &xlat464TestDialer{packetConn: &xlat464TestPacketConn{}}
	dialer, err := newXLAT464Dialer(base, xlat464TestOptions("64:ff9b::/96"))
	xlat464NoError(t, err)
	addresses := []netip.Addr{netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("2001:db8::1")}
	want := M.SocksaddrFrom(netip.MustParseAddr("64:ff9b::c000:201"), 443)
	destination := M.ParseSocksaddrHostPort("example.com", 443)

	if _, err = dialer.DialParallelNetwork(context.Background(), N.NetworkTCP, destination, addresses, nil, nil, nil, 0); err != nil {
		t.Fatal(err)
	}
	if got := base.lastDestination(); got != want {
		t.Fatalf("DialParallelNetwork destination: got %v, want %v", got, want)
	}
	if _, got, err := dialer.ListenSerialNetworkPacket(context.Background(), destination, addresses, nil, nil, nil, 0); err != nil {
		t.Fatal(err)
	} else if got != netip.MustParseAddr("192.0.2.1") {
		t.Fatalf("ListenSerialNetworkPacket address: got %v", got)
	}
	if got := base.lastDestination(); got != want {
		t.Fatalf("ListenSerialNetworkPacket destination: got %v, want %v", got, want)
	}
	if _, err = dialer.DialParallelNetwork(context.Background(), N.NetworkTCP, destination, []netip.Addr{netip.MustParseAddr("2001:db8::1")}, nil, nil, nil, 0); err == nil {
		t.Fatal("expected IPv6-only address list rejection")
	}
}

func TestXLAT464PacketConn(t *testing.T) {
	basePacketConn := &xlat464TestPacketConn{localAddr: net.UDPAddrFromAddrPort(netip.MustParseAddrPort("[::]:1234"))}
	base := &xlat464TestDialer{packetConn: basePacketConn}
	dialer, err := newXLAT464Dialer(base, xlat464TestOptions("64:ff9b::/96"))
	xlat464NoError(t, err)
	conn, err := dialer.ListenPacket(context.Background(), M.SocksaddrFrom(netip.MustParseAddr("192.0.2.1"), 53))
	xlat464NoError(t, err)
	if got := base.lastDestination(); got != M.SocksaddrFrom(netip.MustParseAddr("64:ff9b::c000:201"), 53) {
		t.Fatalf("ListenPacket destination: got %v", got)
	}

	prefixed := net.UDPAddrFromAddrPort(netip.MustParseAddrPort("[64:ff9b::c000:201]:53"))
	if _, err = conn.WriteTo([]byte("request"), net.UDPAddrFromAddrPort(netip.MustParseAddrPort("192.0.2.1:53"))); err != nil {
		t.Fatal(err)
	}
	if got := basePacketConn.writeToAddr.(*net.UDPAddr).AddrPort(); got != prefixed.AddrPort() {
		t.Fatalf("WriteTo address: got %v, want %v", got, prefixed.AddrPort())
	}
	basePacketConn.readFromAddr = prefixed
	if _, source, err := conn.ReadFrom(make([]byte, 64)); err != nil {
		t.Fatal(err)
	} else if got := source.(*net.UDPAddr).AddrPort(); got != netip.MustParseAddrPort("192.0.2.1:53") {
		t.Fatalf("ReadFrom address: got %v", got)
	}
	nonPrefixed := net.UDPAddrFromAddrPort(netip.MustParseAddrPort("[2001:db8::1]:53"))
	basePacketConn.readFromAddr = nonPrefixed
	if _, source, err := conn.ReadFrom(make([]byte, 64)); err != nil {
		t.Fatal(err)
	} else if got := source.(*net.UDPAddr).AddrPort(); got != nonPrefixed.AddrPort() {
		t.Fatalf("ReadFrom non-prefixed address: got %v", got)
	}

	netPacketConn := conn.(N.NetPacketConn)
	buffer := B.New()
	defer buffer.Release()
	if err = netPacketConn.WritePacket(buffer, M.SocksaddrFrom(netip.MustParseAddr("192.0.2.1"), 53)); err != nil {
		t.Fatal(err)
	}
	if got := basePacketConn.writePacketDestination; got != M.SocksaddrFrom(netip.MustParseAddr("64:ff9b::c000:201"), 53) {
		t.Fatalf("WritePacket destination: got %v", got)
	}
	basePacketConn.readPacketDestination = M.SocksaddrFrom(netip.MustParseAddr("64:ff9b::c000:201"), 53)
	if got, err := netPacketConn.ReadPacket(buffer); err != nil {
		t.Fatal(err)
	} else if want := M.SocksaddrFrom(netip.MustParseAddr("192.0.2.1"), 53); got != want {
		t.Fatalf("ReadPacket destination: got %v, want %v", got, want)
	}

	deadline := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)
	xlat464NoError(t, conn.SetDeadline(deadline))
	xlat464NoError(t, conn.SetReadDeadline(deadline))
	xlat464NoError(t, conn.SetWriteDeadline(deadline))
	if basePacketConn.deadline != deadline || basePacketConn.readDeadline != deadline || basePacketConn.writeDeadline != deadline {
		t.Fatal("deadlines were not delegated")
	}
	if conn.LocalAddr() != basePacketConn.localAddr {
		t.Fatal("LocalAddr was not delegated")
	}
	xlat464NoError(t, conn.Close())
	if !basePacketConn.closed {
		t.Fatal("Close was not delegated")
	}
}

func xlat464TestOptions(prefix string) option.Xlat464Options {
	prefixValue := badoption.Prefix(netip.MustParsePrefix(prefix))
	return option.Xlat464Options{Prefix: &prefixValue}
}

func xlat464NoError(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

type xlat464TestDialer struct {
	destinations []M.Socksaddr
	packetConn   *xlat464TestPacketConn
}

func (d *xlat464TestDialer) record(destination M.Socksaddr) {
	d.destinations = append(d.destinations, destination)
}
func (d *xlat464TestDialer) lastDestination() M.Socksaddr {
	return d.destinations[len(d.destinations)-1]
}
func (d *xlat464TestDialer) DialContext(_ context.Context, _ string, destination M.Socksaddr) (net.Conn, error) {
	d.record(destination)
	return nil, nil
}
func (d *xlat464TestDialer) ListenPacket(_ context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	d.record(destination)
	return d.packetConn, nil
}
func (d *xlat464TestDialer) DialParallelInterface(_ context.Context, _ string, destination M.Socksaddr, _ *C.NetworkStrategy, _ []C.InterfaceType, _ []C.InterfaceType, _ time.Duration) (net.Conn, error) {
	d.record(destination)
	return nil, nil
}
func (d *xlat464TestDialer) ListenSerialInterfacePacket(_ context.Context, destination M.Socksaddr, _ *C.NetworkStrategy, _ []C.InterfaceType, _ []C.InterfaceType, _ time.Duration) (net.PacketConn, error) {
	d.record(destination)
	return d.packetConn, nil
}

type xlat464TestPacketConn struct {
	readFromAddr, writeToAddr                     net.Addr
	readPacketDestination, writePacketDestination M.Socksaddr
	localAddr                                     net.Addr
	deadline, readDeadline, writeDeadline         time.Time
	closed                                        bool
}

func (c *xlat464TestPacketConn) ReadFrom(p []byte) (int, net.Addr, error) {
	return copy(p, "response"), c.readFromAddr, nil
}
func (c *xlat464TestPacketConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	c.writeToAddr = addr
	return len(p), nil
}
func (c *xlat464TestPacketConn) ReadPacket(buffer *B.Buffer) (M.Socksaddr, error) {
	_, _ = buffer.WriteString("response")
	return c.readPacketDestination, nil
}
func (c *xlat464TestPacketConn) WritePacket(_ *B.Buffer, destination M.Socksaddr) error {
	c.writePacketDestination = destination
	return nil
}
func (c *xlat464TestPacketConn) Close() error        { c.closed = true; return nil }
func (c *xlat464TestPacketConn) LocalAddr() net.Addr { return c.localAddr }
func (c *xlat464TestPacketConn) SetDeadline(deadline time.Time) error {
	c.deadline = deadline
	return nil
}
func (c *xlat464TestPacketConn) SetReadDeadline(deadline time.Time) error {
	c.readDeadline = deadline
	return nil
}
func (c *xlat464TestPacketConn) SetWriteDeadline(deadline time.Time) error {
	c.writeDeadline = deadline
	return nil
}

var _ N.NetPacketConn = (*xlat464TestPacketConn)(nil)
