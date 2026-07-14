package direct

import (
	"context"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/route"
	B "github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func TestXLAT464RouteUDP(t *testing.T) {
	// Given
	logicalDestination := M.SocksaddrFrom(netip.MustParseAddr("192.0.2.1"), 53)
	synthesizedDestination := M.SocksaddrFrom(netip.MustParseAddr("64:ff9b::c000:201"), 53)
	base := &xlat464TestDialer{packetConn: &xlat464TestPacketConn{
		readPacketDestination:   synthesizedDestination,
		writePacketDestinations: make(chan M.Socksaddr, 1),
	}}
	outbound := newXLAT464UDPTestOutbound(t, base, true)
	application := newXLAT464RouteTestPacketConn()
	t.Cleanup(func() { xlat464NoError(t, application.Close()) })
	manager := route.NewConnectionManager(log.NewNOPFactory().Logger())

	manager.NewPacketConnection(context.Background(), outbound, application, adapter.InboundContext{
		Network:                  N.NetworkUDP,
		Destination:              M.ParseSocksaddrHostPort("example.com", 53),
		RouteOriginalDestination: logicalDestination,
		DestinationAddresses: []netip.Addr{
			netip.MustParseAddr("2001:db8::1"),
			logicalDestination.Addr,
		},
	}, nil)

	// When
	application.readPackets <- xlat464RouteTestPacket{payload: "request", destination: logicalDestination}

	// Then
	select {
	case got := <-base.packetConn.writePacketDestinations:
		if got != synthesizedDestination {
			t.Fatalf("packet socket destination: got %v, want %v", got, synthesizedDestination)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the packet socket write")
	}
	select {
	case reply := <-application.writePackets:
		if got, want := reply.destination, logicalDestination; got != want {
			t.Fatalf("application reply source: got %v, want %v", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the application reply")
	}
}

type xlat464RouteTestPacket struct {
	payload     string
	destination M.Socksaddr
}

type xlat464RouteTestPacketConn struct {
	readPackets  chan xlat464RouteTestPacket
	writePackets chan xlat464RouteTestPacket
	closed       chan struct{}
	closeOnce    sync.Once
}

func newXLAT464RouteTestPacketConn() *xlat464RouteTestPacketConn {
	return &xlat464RouteTestPacketConn{
		readPackets:  make(chan xlat464RouteTestPacket, 1),
		writePackets: make(chan xlat464RouteTestPacket, 1),
		closed:       make(chan struct{}),
	}
}

func (c *xlat464RouteTestPacketConn) ReadPacket(buffer *B.Buffer) (M.Socksaddr, error) {
	select {
	case packet := <-c.readPackets:
		_, err := buffer.WriteString(packet.payload)
		return packet.destination, err
	case <-c.closed:
		return M.Socksaddr{}, net.ErrClosed
	}
}

func (c *xlat464RouteTestPacketConn) WritePacket(buffer *B.Buffer, destination M.Socksaddr) error {
	packet := xlat464RouteTestPacket{payload: string(buffer.Bytes()), destination: destination}
	select {
	case <-c.closed:
		return net.ErrClosed
	case c.writePackets <- packet:
		return nil
	}
}

func (c *xlat464RouteTestPacketConn) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })
	return nil
}

func (c *xlat464RouteTestPacketConn) LocalAddr() net.Addr {
	return nil
}

func (c *xlat464RouteTestPacketConn) SetDeadline(time.Time) error {
	return nil
}

func (c *xlat464RouteTestPacketConn) SetReadDeadline(time.Time) error {
	return nil
}

func (c *xlat464RouteTestPacketConn) SetWriteDeadline(time.Time) error {
	return nil
}

var _ N.PacketConn = (*xlat464RouteTestPacketConn)(nil)
