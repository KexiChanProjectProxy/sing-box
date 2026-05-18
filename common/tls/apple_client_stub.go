//go:build !darwin || !cgo

package tls

import (
	"context"
	"github.com/sagernet/sing-box/log"

	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func newAppleClient(ctx context.Context, logger log.StructuredLogger, serverAddress string, options option.OutboundTLSOptions, allowEmptyServerName bool) (Config, error) {
	return nil, E.New("Apple TLS engine is not available on non-Apple platforms")
}
