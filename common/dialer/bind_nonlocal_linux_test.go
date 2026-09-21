package dialer

import (
	"context"
	"errors"
	"net"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNonLocalBind binds to TEST-NET addresses that no interface owns. The
// plain bind must fail with EADDRNOTAVAIL, the IP_FREEBIND bind must succeed.
func TestNonLocalBind(t *testing.T) {
	t.Parallel()

	control, err := nonLocalBind()
	require.NoError(t, err)
	for _, testCase := range []struct{ network, address string }{
		{"udp4", "192.0.2.123:0"},
		{"tcp4", "192.0.2.123:0"},
		{"udp6", "[2001:db8::123]:0"},
		{"tcp6", "[2001:db8::123]:0"},
	} {
		t.Run(testCase.network, func(t *testing.T) {
			var plain net.ListenConfig
			conn, err := listen(plain, testCase.network, testCase.address)
			if err == nil {
				conn.Close()
				t.Skip("address unexpectedly local on this host")
			}
			if !errors.Is(err, syscall.EADDRNOTAVAIL) {
				t.Skipf("%s unavailable: %v", testCase.network, err)
			}
			conn, err = listen(net.ListenConfig{Control: control}, testCase.network, testCase.address)
			require.NoError(t, err)
			conn.Close()
		})
	}
}

type closer interface{ Close() error }

func listen(config net.ListenConfig, network string, address string) (closer, error) {
	if network[:3] == "udp" {
		return config.ListenPacket(context.Background(), network, address)
	}
	return config.Listen(context.Background(), network, address)
}
