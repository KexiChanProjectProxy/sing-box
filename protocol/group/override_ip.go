package group

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/route"
	"github.com/sagernet/sing/service"
)

func withOverrideIPContext(ctx context.Context, overrideIP *option.OverrideIPOptions) context.Context {
	if overrideIP != nil {
		return adapter.ContextWithOverrideIP(ctx, overrideIP)
	}
	return ctx
}

func applyGroupOverrideIP(ctx context.Context, metadata *adapter.InboundContext, outbound adapter.Outbound, overrideIP *option.OverrideIPOptions, serviceCtx context.Context) (context.Context, error) {
	ctx = withOverrideIPContext(ctx, overrideIP)
	if overrideIP == nil && adapter.OverrideIPFromContext(ctx) == nil {
		return ctx, nil
	}
	return route.ApplyOverrideIP(
		ctx,
		metadata,
		outbound,
		service.FromContext[adapter.DNSRouter](serviceCtx),
		service.FromContext[adapter.DNSTransportManager](serviceCtx),
	)
}
