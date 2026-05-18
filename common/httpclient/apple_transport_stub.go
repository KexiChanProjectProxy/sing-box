//go:build !darwin || !cgo

package httpclient

import (
	"context"
	"github.com/sagernet/sing-box/log"

	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
)

func newAppleTransport(ctx context.Context, logger log.StructuredLogger, rawDialer N.Dialer, options option.HTTPClientOptions) (innerTransport, error) {
	return nil, E.New("Apple HTTP engine is not available on non-Apple platforms")
}
