package option

import (
	"github.com/sagernet/sing/common/json"
)

// PreferDomainOptions configures per-outbound domain preference.
// When enabled, the outbound overrides IP destinations with the sniffed
// domain name before connecting.
//
// JSON forms:
//   - true               → enabled, no mark condition
//   - {"mark": "0x1"}    → enabled, only when mark matches
type PreferDomainOptions struct {
	Enabled bool
	Mark    *MarkMatch
}

func (p PreferDomainOptions) MarshalJSON() ([]byte, error) {
	if p.Mark == nil {
		return json.Marshal(p.Enabled)
	}
	return json.Marshal(struct {
		Mark *MarkMatch `json:"mark,omitempty"`
	}{Mark: p.Mark})
}

func (p *PreferDomainOptions) UnmarshalJSON(data []byte) error {
	var boolValue bool
	if json.Unmarshal(data, &boolValue) == nil {
		p.Enabled = boolValue
		return nil
	}
	var raw struct {
		Mark *MarkMatch `json:"mark,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	p.Enabled = true
	p.Mark = raw.Mark
	return nil
}
