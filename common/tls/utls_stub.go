//go:build !with_utls

package tls

import (
	"context"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func NewUTLSClient(ctx context.Context, logger log.StructuredLogger, serverAddress string, options option.OutboundTLSOptions) (Config, error) {
	return newUTLSClient(ctx, logger, serverAddress, options, false)
}

func newUTLSClient(ctx context.Context, logger log.StructuredLogger, serverAddress string, options option.OutboundTLSOptions, allowEmptyServerName bool) (Config, error) {
	return nil, E.New(`uTLS is not included in this build, rebuild with -tags with_utls`)
}

func NewRealityClient(ctx context.Context, logger log.StructuredLogger, serverAddress string, options option.OutboundTLSOptions) (Config, error) {
	return newRealityClient(ctx, logger, serverAddress, options, false)
}

func newRealityClient(ctx context.Context, logger log.StructuredLogger, serverAddress string, options option.OutboundTLSOptions, allowEmptyServerName bool) (Config, error) {
	return nil, E.New(`uTLS, which is required by reality is not included in this build, rebuild with -tags with_utls`)
}

func NewRealityServer(ctx context.Context, logger log.StructuredLogger, options option.InboundTLSOptions) (ServerConfig, error) {
	return nil, E.New(`uTLS, which is required by reality is not included in this build, rebuild with -tags with_utls`)
}
