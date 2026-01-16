package option

import (
	"github.com/sagernet/sing/common/json/badoption"
)

// RouterInboundOptions defines the configuration for router inbound
type RouterInboundOptions struct {
	ListenOptions
	InboundTLSOptionsContainer
	Routes             []RouteRule      `json:"routes"`
	Fallback           *FallbackOptions `json:"fallback,omitempty"`
	Timeout            *TimeoutOptions  `json:"timeout,omitempty"`
	MaxRequestBodySize int64            `json:"max_request_body_size,omitempty"` // In bytes
}

// RouteRule defines a single routing rule
type RouteRule struct {
	Name            string     `json:"name"`
	Match           RouteMatch `json:"match"`
	Target          string     `json:"target"`           // Target inbound tag (must be internal-only inbound)
	StripPathPrefix string     `json:"strip_path_prefix,omitempty"`
	Priority        int        `json:"priority,omitempty"` // Higher = evaluated first
}

// RouteMatch defines matching criteria
type RouteMatch struct {
	PathPrefix []string            `json:"path_prefix,omitempty"`
	PathRegex  []string            `json:"path_regex,omitempty"`
	Host       []string            `json:"host,omitempty"`
	Header     map[string][]string `json:"header,omitempty"`
	Method     []string            `json:"method,omitempty"`
}

// FallbackOptions defines fallback behavior
type FallbackOptions struct {
	Type       string   `json:"type"`                  // static, drop, reject, inbound
	Webroot    string   `json:"webroot,omitempty"`     // For static type
	Index      []string `json:"index,omitempty"`       // Default index files
	StatusCode int      `json:"status_code,omitempty"` // For reject type
	Target     string   `json:"target,omitempty"`      // For inbound type
}

// TimeoutOptions for HTTP server
type TimeoutOptions struct {
	Read  badoption.Duration `json:"read,omitempty"`
	Write badoption.Duration `json:"write,omitempty"`
	Idle  badoption.Duration `json:"idle,omitempty"`
}
