package route

import (
	"net/netip"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	M "github.com/sagernet/sing/common/metadata"
)

func applyPreferDomain(metadata *adapter.InboundContext, outbound adapter.Outbound) {
	outboundWithPrefer, ok := outbound.(adapter.OutboundWithPreferDomain)
	if !ok || !outboundWithPrefer.PreferDomain() {
		return
	}

	switch metadata.Protocol {
	case C.ProtocolHTTP, C.ProtocolTLS, C.ProtocolQUIC:
	default:
		return
	}

	if metadata.Domain == "" {
		return
	}

	addr, err := netip.ParseAddr(metadata.Domain)
	if err == nil {
		if metadata.Destination.Addr == addr {
			return
		}
		metadata.Destination = M.Socksaddr{
			Addr: addr,
			Port: metadata.Destination.Port,
		}
	} else {
		if metadata.Destination.Fqdn == metadata.Domain {
			return
		}
		metadata.Destination = M.Socksaddr{
			Fqdn: metadata.Domain,
			Port: metadata.Destination.Port,
		}
	}
}