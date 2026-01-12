//go:build go1.23

package dialer

import "net"

func setKeepAliveConfig(dialer *net.Dialer) {
	// Use Go 1.23's new KeepAliveConfig for proper TCP keep-alive
	dialer.KeepAliveConfig = net.KeepAliveConfig{
		Enable: true,
		// Idle is the time before the first keep-alive probe is sent
		// Interval is the time between subsequent keep-alive probes
		// Use default values (0) to let the OS decide
		Idle:     0, // Use OS default
		Interval: 0, // Use OS default
	}
}
