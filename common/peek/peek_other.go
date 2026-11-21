//go:build !linux

package peek

import (
	"net"
	"time"
)

func clientHello(conn net.Conn, timeout time.Duration) (net.Conn, bool, error) {
	return readClientHello(conn, timeout)
}
