//go:build !with_gvisor

package ipstack

import tun "github.com/sagernet/sing-tun"

func NewDevice(options DeviceOptions) (Device, error) {
	return nil, tun.ErrGVisorNotIncluded
}
