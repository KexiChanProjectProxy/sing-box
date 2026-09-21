//go:build !linux && !freebsd

package dialer

import (
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
)

func nonLocalBind() (control.Func, error) {
	return nil, E.New("`non_local_bind` is only supported on Linux and FreeBSD")
}
