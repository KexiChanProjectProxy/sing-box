package dialer

import (
	"syscall"

	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/unix"
)

// nonLocalBind lets a socket bind to an address that is not assigned to any
// local interface, e.g. an address inside a prefix routed to the host with
// `ip route add local <prefix> dev lo`. IP_FREEBIND is honoured for IPv6
// sockets on every kernel; IPV6_FREEBIND (Linux 4.15+) is set as well when
// available.
func nonLocalBind() (control.Func, error) {
	return func(network, address string, conn syscall.RawConn) error {
		return control.Raw(conn, func(fd uintptr) error {
			err := unix.SetsockoptInt(int(fd), unix.SOL_IP, unix.IP_FREEBIND, 1)
			if err != nil {
				return E.Cause(err, "set IP_FREEBIND")
			}
			domain, err := unix.GetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_DOMAIN)
			if err == nil && domain == unix.AF_INET6 {
				err = unix.SetsockoptInt(int(fd), unix.SOL_IPV6, unix.IPV6_FREEBIND, 1)
				if err != nil && !E.IsMulti(err, unix.ENOPROTOOPT, unix.EINVAL) {
					return E.Cause(err, "set IPV6_FREEBIND")
				}
			}
			return nil
		})
	}, nil
}
