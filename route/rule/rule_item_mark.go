package rule

import (
	"strconv"

	"github.com/sagernet/sing-box/adapter"
	F "github.com/sagernet/sing/common/format"
)

var _ RuleItem = (*MarkItem)(nil)

type MarkItem struct {
	value uint32
	mask  uint32
}

func NewMarkItem(value, mask uint32) *MarkItem {
	return &MarkItem{value: value, mask: mask}
}

func (r *MarkItem) Match(metadata *adapter.InboundContext) bool {
	return metadata.Mark&r.mask == r.value
}

func (r *MarkItem) String() string {
	if r.mask == 0xFFFFFFFF {
		return F.ToString("mark=0x", strconv.FormatUint(uint64(r.value), 16))
	}
	return F.ToString("mark=0x", strconv.FormatUint(uint64(r.value), 16), "/0x", strconv.FormatUint(uint64(r.mask), 16))
}

func (r *MarkItem) MatchType() string {
	return "mark"
}
