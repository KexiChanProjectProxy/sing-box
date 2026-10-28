package dialer

import (
	"context"
	"net"

	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ N.Dialer = (*unixServerDialer)(nil)

// unixServerDialer sends every TCP dial to a fixed Unix socket path and
// ignores the destination, so protocol clients and transports that dial
// their server over TCP work unchanged.
type unixServerDialer struct {
	dialer N.Dialer
	path   string
}

func newUnixServerDialer(options Options) (N.Dialer, error) {
	dialOptions := options.Options
	switch {
	case dialOptions.Detour != "" || options.DefaultOutbound:
		return nil, E.New("server_unix cannot be used with detour")
	case dialOptions.TCPFastOpen:
		return nil, E.New("tcp_fast_open is not supported with server_unix")
	case dialOptions.TCPMultiPath:
		return nil, E.New("tcp_multi_path is not supported with server_unix")
	case dialOptions.BindInterface != "":
		return nil, E.New("bind_interface is not supported with server_unix")
	case dialOptions.RoutingMark != 0:
		return nil, E.New("routing_mark is not supported with server_unix")
	case dialOptions.NetworkStrategy != nil:
		return nil, E.New("network_strategy is not supported with server_unix")
	}
	defaultDialer, err := NewDefault(options.Context, dialOptions)
	if err != nil {
		return nil, err
	}
	var dialer N.Dialer = defaultDialer
	if options.DialerWrapper != nil {
		dialer = options.DialerWrapper(defaultDialer)
	}
	return &unixServerDialer{dialer: dialer, path: options.ServerUnix}, nil
}

func (d *unixServerDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if N.NetworkName(network) != N.NetworkTCP {
		return nil, E.New("unix socket server does not support ", network)
	}
	conn, err := d.dialer.DialContext(ctx, C.NetworkUnix, M.Socksaddr{Fqdn: d.path})
	if err != nil {
		return nil, err
	}
	return &unixServerConn{conn}, nil
}

func (d *unixServerDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, E.New("unix socket server does not support udp")
}

var unspecifiedTCPAddr = &net.TCPAddr{IP: net.IPv4zero}

// unixServerConn reports an unspecified TCP local address instead of the
// unnamed Unix socket address, because callers such as the SOCKS server
// echo the outbound local address back to the client as an IP address.
type unixServerConn struct {
	net.Conn
}

func (c *unixServerConn) LocalAddr() net.Addr {
	return unspecifiedTCPAddr
}

func (c *unixServerConn) Upstream() any {
	return c.Conn
}

func (c *unixServerConn) ReaderReplaceable() bool {
	return true
}

func (c *unixServerConn) WriterReplaceable() bool {
	return true
}
