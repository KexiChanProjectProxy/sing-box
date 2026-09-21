package dialer

import (
	"syscall"

	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/unix"
)

// nonLocalBind lets a socket bind to an address that is not assigned to any
// local interface. IP_BINDANY / IPV6_BINDANY require PRIV_NETINET_BINDANY.
func nonLocalBind() (control.Func, error) {
	return func(network, address string, conn syscall.RawConn) error {
		return control.Raw(conn, func(fd uintptr) error {
			domain, err := unix.GetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_DOMAIN)
			if err == nil && domain == unix.AF_INET6 {
				err = unix.SetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_BINDANY, 1)
				if err != nil {
					return E.Cause(err, "set IPV6_BINDANY")
				}
				return nil
			}
			err = unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_BINDANY, 1)
			if err != nil {
				return E.Cause(err, "set IP_BINDANY")
			}
			return nil
		})
	}, nil
}
