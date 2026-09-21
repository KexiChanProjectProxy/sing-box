package dialer

import (
	"context"
	"net"
	"net/netip"
	"testing"

	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json/badoption"

	"github.com/stretchr/testify/require"
)

func TestDefaultDialerWithBindAddress(t *testing.T) {
	t.Parallel()

	// Given a dialer with a base IPv6 binding and a UDP bind port
	base, err := NewDefault(context.Background(), option.DialerOptions{AbstractDialerOptions: option.AbstractDialerOptions{
		Inet6BindAddress: (*badoption.Addr)(addrPointer("2001:db8::1")),
		UDPBindPort:      5353,
	}})
	require.NoError(t, err)

	// When only IPv4 is overridden
	clone := base.WithBindAddress(netip.MustParseAddr("192.0.2.10"), netip.Addr{})

	// Then IPv4 uses the new address and IPv6 keeps the base binding
	require.Equal(t, "192.0.2.10", clone.dialer4.LocalAddr.(*net.TCPAddr).IP.String())
	require.Equal(t, &net.UDPAddr{IP: net.ParseIP("192.0.2.10").To4(), Port: 5353}, clone.udpDialer4.LocalAddr)
	require.Equal(t, "192.0.2.10:5353", clone.udpAddr4)
	require.Equal(t, "2001:db8::1", clone.dialer6.LocalAddr.(*net.TCPAddr).IP.String())
	require.Equal(t, base.udpAddr6, clone.udpAddr6)

	// And the original is not modified
	require.Nil(t, base.dialer4.LocalAddr)
	require.Equal(t, "0.0.0.0:5353", base.udpAddr4)

	// When IPv6 is overridden on the clone
	clone6 := clone.WithBindAddress(netip.Addr{}, netip.MustParseAddr("2001:db8::99"))
	require.Equal(t, "2001:db8::99", clone6.dialer6.LocalAddr.(*net.TCPAddr).IP.String())
	require.Equal(t, "[2001:db8::99]:5353", clone6.udpAddr6)
	require.Equal(t, "192.0.2.10", clone6.dialer4.LocalAddr.(*net.TCPAddr).IP.String())
	require.Equal(t, "2001:db8::1", clone.dialer6.LocalAddr.(*net.TCPAddr).IP.String())
}

func addrPointer(address string) *netip.Addr {
	addr := netip.MustParseAddr(address)
	return &addr
}
