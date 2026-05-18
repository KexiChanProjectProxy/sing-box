package route

import (
	"context"
	"net/netip"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	M "github.com/sagernet/sing/common/metadata"
)

func ApplyPreferDomain(ctx context.Context, metadata *adapter.InboundContext, outbound adapter.Outbound) context.Context {
	inheritedPreferDomain := adapter.PreferDomainFromContext(ctx)
	outboundWithPrefer, ok := outbound.(adapter.OutboundWithPreferDomain)
	outboundPreferDomain := ok && outboundWithPrefer.PreferDomain()
	effectivePreferDomain := inheritedPreferDomain || outboundPreferDomain

	ctx = adapter.ContextWithPreferDomain(ctx, effectivePreferDomain)

	if !effectivePreferDomain {
		return ctx
	}

	switch metadata.Protocol {
	case C.ProtocolHTTP, C.ProtocolTLS, C.ProtocolQUIC:
	default:
		return ctx
	}

	if metadata.Domain == "" {
		return ctx
	}

	addr, err := netip.ParseAddr(metadata.Domain)
	if err == nil {
		if metadata.Destination.Addr == addr {
			return ctx
		}
		metadata.Destination = M.Socksaddr{
			Addr: addr,
			Port: metadata.Destination.Port,
		}
	} else {
		if metadata.Destination.Fqdn == metadata.Domain {
			return ctx
		}
		metadata.Destination = M.Socksaddr{
			Fqdn: metadata.Domain,
			Port: metadata.Destination.Port,
		}
	}

	return ctx
}