package group

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
)

func withOverrideIPContext(ctx context.Context, overrideIP *option.OverrideIPOptions) context.Context {
	if overrideIP != nil {
		return adapter.ContextWithOverrideIP(ctx, overrideIP)
	}
	return ctx
}
