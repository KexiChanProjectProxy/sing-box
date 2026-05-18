//go:build !linux

package ktls

import (
	"context"
	"github.com/sagernet/sing-box/log"

	E "github.com/sagernet/sing/common/exceptions"
	aTLS "github.com/sagernet/sing/common/tls"
)

func NewConn(ctx context.Context, logger log.StructuredLogger, conn aTLS.Conn, txOffload, rxOffload bool) (aTLS.Conn, error) {
	return nil, E.New("kTLS is only supported on Linux")
}
