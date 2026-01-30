package rule

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
	F "github.com/sagernet/sing/common/format"
)

var _ RuleItem = (*InterfaceItem)(nil)

type InterfaceItem struct {
	interfaces   []string
	interfaceMap map[string]bool
}

func NewInterfaceItem(interfaces []string) *InterfaceItem {
	interfaceMap := make(map[string]bool)
	for _, iface := range interfaces {
		interfaceMap[iface] = true
	}
	return &InterfaceItem{
		interfaces:   interfaces,
		interfaceMap: interfaceMap,
	}
}

func (r *InterfaceItem) Match(metadata *adapter.InboundContext) bool {
	return r.interfaceMap[metadata.BindInterface]
}

func (r *InterfaceItem) String() string {
	if len(r.interfaces) == 1 {
		return F.ToString("interface=", r.interfaces[0])
	}
	return F.ToString("interface=[", strings.Join(r.interfaces, " "), "]")
}
