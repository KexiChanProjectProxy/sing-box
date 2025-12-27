//go:build !go1.23

package listener

import (
	"net"
	"time"

	"github.com/sagernet/sing/common/control"
)

func setKeepAliveConfig(listener *net.ListenConfig, idle time.Duration, interval time.Duration) {
	// If both idle and interval are 0, use system defaults
	if idle == 0 && interval == 0 {
		// Setting KeepAlive to -1 enables TCP keep-alive with system defaults
		listener.KeepAlive = -1
		// Do not call SetKeepAlivePeriod to preserve system defaults
		return
	}

	// Use specified values
	listener.KeepAlive = idle
	listener.Control = control.Append(listener.Control, control.SetKeepAlivePeriod(idle, interval))
}
