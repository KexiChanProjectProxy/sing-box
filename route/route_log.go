package route

import (
	"context"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	group "github.com/sagernet/sing-box/protocol/group"
	F "github.com/sagernet/sing/common/format"
)

type routeLogContext struct {
	rule             string
	action           string
	configuredTag    string
	resolvedTag      string
	resolvedType     string
	outboundChain    []string
	resolutionFailed error
}

type routeLogContextKey struct{}

func withRouteLogContext(ctx context.Context, routeContext routeLogContext) context.Context {
	return context.WithValue(ctx, routeLogContextKey{}, routeContext)
}

func routeLogContextFrom(ctx context.Context) (routeLogContext, bool) {
	routeContext, ok := ctx.Value(routeLogContextKey{}).(routeLogContext)
	return routeContext, ok
}

func (r *Router) newRouteLogContextByTag(rule adapter.Rule, outboundTag string) routeLogContext {
	return r.newResolvedRouteLogContext(rule, outboundTag, nil)
}

func (r *Router) newRouteLogContext(rule adapter.Rule, outbound adapter.Outbound) routeLogContext {
	configuredTag := ""
	if outbound != nil {
		configuredTag = outbound.Tag()
	}
	return r.newResolvedRouteLogContext(rule, configuredTag, outbound)
}

func (r *Router) newResolvedRouteLogContext(rule adapter.Rule, configuredTag string, outbound adapter.Outbound) routeLogContext {
	routeContext := routeLogContext{configuredTag: configuredTag}
	if rule != nil {
		routeContext.rule = rule.String()
		if action := rule.Action(); action != nil {
			routeContext.action = action.String()
		}
	}

	var (
		resolved group.ResolvedOutbound
		err      error
	)
	if outbound != nil {
		resolved, err = group.ResolveOutbound(outbound)
	} else if configuredTag != "" {
		resolved, err = group.ResolveOutboundByTag(r.outbound, configuredTag)
	}
	if err != nil {
		routeContext.resolutionFailed = err
	}
	if len(resolved.Chain) > 0 {
		routeContext.outboundChain = append([]string(nil), resolved.Chain...)
	}
	if resolved.Leaf != nil {
		routeContext.resolvedTag = resolved.Leaf.Tag()
		routeContext.resolvedType = resolved.Leaf.Type()
	}
	return routeContext
}

func (c routeLogContext) actionLabel() string {
	if c.action != "" {
		return c.action
	}
	if c.configuredTag != "" {
		return F.ToString("default(", c.configuredTag, ")")
	}
	return ""
}

func (c routeLogContext) resolvedChainSuffix() string {
	if len(c.outboundChain) == 0 {
		return ""
	}
	return F.ToString(" -> ", strings.Join(c.outboundChain, " -> "))
}

func (c routeLogContext) plainRouteLabel() string {
	var builder strings.Builder
	if c.rule != "" {
		builder.WriteString(c.rule)
	}
	action := c.actionLabel()
	if action != "" {
		if builder.Len() > 0 {
			builder.WriteString(" => ")
		}
		builder.WriteString(action)
	}
	if len(c.outboundChain) > 0 {
		if builder.Len() > 0 {
			builder.WriteString(" -> ")
		}
		builder.WriteString(strings.Join(c.outboundChain, " -> "))
	}
	return builder.String()
}

func (c routeLogContext) applyToConnectionEvent(event *log.ConnectionEvent) *log.ConnectionEvent {
	if c.rule != "" {
		event.WithRoute(c.rule, c.actionLabel())
	} else if action := c.actionLabel(); action != "" {
		event.WithRoute("", action)
	}
	if len(c.outboundChain) > 0 || c.resolvedTag != "" {
		event.WithResolvedChain(c.resolvedTag, c.resolvedType, c.outboundChain)
	}
	return event
}

func (c routeLogContext) applyToRouterMatchEvent(event *log.RouterMatchEvent) *log.RouterMatchEvent {
	if len(c.outboundChain) > 0 || c.resolvedTag != "" {
		event.WithResolvedChain(c.resolvedTag, c.outboundChain)
	}
	return event
}
