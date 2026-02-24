package option

import (
	"strconv"
	"strings"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
)

// MarkMatch represents a virtual route mark match with bitmask support.
// JSON formats:
//   - 1          → exact match (Value=1, Mask=0xFFFFFFFF)
//   - "0x1"      → hex exact match
//   - "0x1/0xff" → bitmask match
type MarkMatch struct {
	Value uint32
	Mask  uint32
}

func (m MarkMatch) MarshalJSON() ([]byte, error) {
	if m.Mask == 0xFFFFFFFF {
		return json.Marshal("0x" + strconv.FormatUint(uint64(m.Value), 16))
	}
	return json.Marshal("0x" + strconv.FormatUint(uint64(m.Value), 16) + "/0x" + strconv.FormatUint(uint64(m.Mask), 16))
}

func (m *MarkMatch) UnmarshalJSON(data []byte) error {
	var uintValue uint32
	if json.Unmarshal(data, &uintValue) == nil {
		m.Value = uintValue
		m.Mask = 0xFFFFFFFF
		return nil
	}
	var stringValue string
	if err := json.Unmarshal(data, &stringValue); err != nil {
		return E.New("invalid mark: expected number or string")
	}
	parts := strings.SplitN(stringValue, "/", 2)
	value, err := strconv.ParseUint(parts[0], 0, 32)
	if err != nil {
		return E.Cause(err, "invalid mark value")
	}
	m.Value = uint32(value)
	if len(parts) == 2 {
		mask, err := strconv.ParseUint(parts[1], 0, 32)
		if err != nil {
			return E.Cause(err, "invalid mark mask")
		}
		m.Mask = uint32(mask)
	} else {
		m.Mask = 0xFFFFFFFF
	}
	return nil
}
