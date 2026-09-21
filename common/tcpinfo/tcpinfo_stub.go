//go:build !linux

package tcpinfo

import "net"

// Supported reports whether Read can return data on this platform.
const Supported = false

// Read always returns false: TCP_INFO is only read on Linux.
func Read(net.Conn) (Info, bool) {
	return Info{}, false
}
