//go:build !with_gvisor

package ipstack

import tun "github.com/sagernet/sing-tun"

// Available reports whether this build includes the gVisor stack NewDevice needs.
const Available = false

func NewDevice(options DeviceOptions) (Device, error) {
	return nil, tun.ErrGVisorNotIncluded
}
