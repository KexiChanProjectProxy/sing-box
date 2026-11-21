//go:build !linux

package peek

import (
	"net"
	"testing"
)

// assertPlainSocket has nothing to check off Linux, where the prefix is read
// and put back with a CachedConn.
func assertPlainSocket(t *testing.T, accepted net.Conn, peeked net.Conn) {
	t.Helper()
}
