//go:build linux

package peek

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

// assertPlainSocket checks that the connection handed on is the accepted
// socket itself, with no wrapper in front of it.
func assertPlainSocket(t *testing.T, accepted net.Conn, peeked net.Conn) {
	t.Helper()
	_, isTCPConn := peeked.(*net.TCPConn)
	require.True(t, isTCPConn, "%T", peeked)
	require.Same(t, accepted, peeked)
}
