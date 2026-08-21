package option

import "github.com/sagernet/sing/common/json/badoption"

type ClickHouseServiceOptions struct {
	Server   string                  `json:"server"`
	Database string                  `json:"database,omitempty"`
	Table    string                  `json:"table"`
	Username string                  `json:"username,omitempty"`
	Password string                  `json:"password,omitempty"`
	Secure   bool                    `json:"secure,omitempty"`
	Detour   string                  `json:"detour,omitempty" reference:"outbound"`
	Batch    *ClickHouseBatchOptions `json:"batch,omitempty"`
}

type ClickHouseBatchOptions struct {
	MaxEntries int                `json:"max_entries,omitempty"`
	MaxWait    badoption.Duration `json:"max_wait,omitempty"`
}
