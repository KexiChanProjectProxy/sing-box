package route

import (
	"context"
	"net/netip"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
)

type overrideIPLookuper interface {
	Lookup(ctx context.Context, domain string, options adapter.DNSQueryOptions) ([]netip.Addr, error)
}

type overrideIPTransportFinder interface {
	Transport(tag string) (adapter.DNSTransport, bool)
}

func ApplyOverrideIP(ctx context.Context, metadata *adapter.InboundContext, outbound adapter.Outbound, dns overrideIPLookuper, transports overrideIPTransportFinder) (context.Context, error) {
	if adapter.OverrideIPAppliedFromContext(ctx) {
		return ctx, nil
	}

	inherited := adapter.OverrideIPFromContext(ctx)
	var outboundOptions *option.OverrideIPOptions
	if outboundWithOverride, ok := outbound.(adapter.OutboundWithOverrideIP); ok {
		outboundOptions = outboundWithOverride.OverrideIP()
	}
	effective := inherited
	if effective == nil {
		effective = outboundOptions
	}
	if effective != nil {
		ctx = adapter.ContextWithOverrideIP(ctx, effective)
	}
	if effective == nil {
		return ctx, nil
	}

	switch metadata.Protocol {
	case C.ProtocolHTTP, C.ProtocolTLS, C.ProtocolQUIC:
	default:
		return ctx, nil
	}
	if metadata.Domain == "" {
		return ctx, nil
	}

	addr, err := netip.ParseAddr(metadata.Domain)
	if err == nil {
		if !overrideIPAddrAllowed(addr, C.DomainStrategy(effective.Strategy)) {
			ctx = adapter.ContextWithOverrideIPApplied(ctx)
			return ctx, nil
		}
		if metadata.Destination.Addr == addr {
			ctx = adapter.ContextWithOverrideIPApplied(ctx)
			return ctx, nil
		}
		metadata.Destination = M.Socksaddr{
			Addr: addr,
			Port: metadata.Destination.Port,
		}
		metadata.DestinationAddresses = []netip.Addr{addr}
		ctx = adapter.ContextWithOverrideIPApplied(ctx)
		return ctx, nil
	}

	queryOptions, err := overrideIPQueryOptions(effective, transports)
	if err != nil {
		return ctx, err
	}
	if dns == nil {
		return ctx, E.New("missing DNS router for override_ip")
	}
	addresses, err := dns.Lookup(adapter.WithContext(ctx, metadata), metadata.Domain, queryOptions)
	if err != nil {
		return ctx, err
	}
	if len(addresses) == 0 {
		return ctx, E.New("empty override_ip result for ", metadata.Domain)
	}
	metadata.DestinationAddresses = addresses
	metadata.Destination = M.SocksaddrFrom(addresses[0], metadata.Destination.Port)
	ctx = adapter.ContextWithOverrideIPApplied(ctx)
	return ctx, nil
}

func overrideIPQueryOptions(options *option.OverrideIPOptions, transports overrideIPTransportFinder) (adapter.DNSQueryOptions, error) {
	var transport adapter.DNSTransport
	if options.Server != "" {
		if transports == nil {
			return adapter.DNSQueryOptions{}, E.New("missing DNS transport manager for override_ip")
		}
		var loaded bool
		transport, loaded = transports.Transport(options.Server)
		if !loaded {
			return adapter.DNSQueryOptions{}, E.New("DNS server not found: ", options.Server)
		}
	}
	return adapter.DNSQueryOptions{
		Transport:              transport,
		Strategy:               C.DomainStrategy(options.Strategy),
		DisableCache:           options.DisableCache,
		DisableOptimisticCache: options.DisableOptimisticCache,
		RewriteTTL:             options.RewriteTTL,
		Timeout:                options.Timeout.Build(),
		ClientSubnet:           options.ClientSubnet.Build(netip.Prefix{}),
	}, nil
}

func overrideIPAddrAllowed(addr netip.Addr, strategy C.DomainStrategy) bool {
	addr = addr.Unmap()
	switch strategy {
	case C.DomainStrategyIPv4Only:
		return addr.Is4()
	case C.DomainStrategyIPv6Only:
		return addr.Is6()
	default:
		return true
	}
}
