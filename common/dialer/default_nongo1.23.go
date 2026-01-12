//go:build !go1.23

package dialer

import (
	"net"

	"github.com/sagernet/sing/common/control"
)

func setKeepAliveConfig(dialer *net.Dialer) {
	// Use legacy KeepAlive field for Go < 1.23
	// Set to -1 to use system defaults instead of hardcoded values
	dialer.KeepAlive = -1
	// Use system-level TCP keep-alive settings for better adaptability
	dialer.Control = control.Append(dialer.Control, control.SetKeepAlivePeriod(-1, -1))
}
