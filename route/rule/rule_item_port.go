package rule

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
	F "github.com/sagernet/sing/common/format"
)

var _ RuleItem = (*PortItem)(nil)

type PortItem struct {
	ports    []uint16
	portMap  map[uint16]bool
	isSource bool
}

func NewPortItem(isSource bool, ports []uint16) *PortItem {
	portMap := make(map[uint16]bool)
	for _, port := range ports {
		portMap[port] = true
	}
	return &PortItem{
		ports:    ports,
		portMap:  portMap,
		isSource: isSource,
	}
}

func (r *PortItem) Match(metadata *adapter.InboundContext) bool {
	if r.isSource {
		return r.portMap[metadata.Source.Port]
	} else {
		return r.portMap[metadata.Destination.Port]
	}
}

func (r *PortItem) String() string {
	var description string
	if r.isSource {
		description = "source_port="
	} else {
		description = "port="
	}
	pLen := len(r.ports)
	if pLen == 1 {
		description += F.ToString(r.ports[0])
	} else {
		description += "[" + strings.Join(F.MapToString(r.ports), " ") + "]"
	}
	return description
}

var _ RuleItem = (*CompositePortMatcher)(nil)

type CompositePortMatcher struct {
	items []RuleItem
}

func (c *CompositePortMatcher) Match(metadata *adapter.InboundContext) bool {
	for _, item := range c.items {
		if item.Match(metadata) {
			return true // OR logic - any match succeeds
		}
	}
	return false
}

func (c *CompositePortMatcher) String() string {
	var parts []string
	for _, item := range c.items {
		parts = append(parts, item.String())
	}
	return F.ToString("composite(", strings.Join(parts, ","), ")")
}
