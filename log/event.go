package log

import (
	"github.com/miekg/dns"
	M "github.com/sagernet/sing/common/metadata"
)

// EventType represents the type of structured log event
type EventType string

const (
	EventTypeConnection    EventType = "connection"
	EventTypeDNS           EventType = "dns"
	EventTypeRouterMatch   EventType = "router_match"
	EventTypeProcessInfo   EventType = "process_info"
	EventTypeTransfer      EventType = "transfer"
)

// StructuredEvent represents structured log data
type StructuredEvent struct {
	Type EventType              `json:"type"`
	Data map[string]interface{} `json:"data"`
}

// ConnectionEvent represents a connection event
type ConnectionEvent struct {
	Direction      string   `json:"direction"` // "inbound" or "outbound"
	Action         string   `json:"action"`    // "start", "success", "error", "close"
	Source         string   `json:"source,omitempty"`
	SourcePort     uint16   `json:"source_port,omitempty"`
	Destination    string   `json:"destination,omitempty"`
	DestPort       uint16   `json:"dest_port,omitempty"`
	Domain         string   `json:"domain,omitempty"`
	Network        string   `json:"network,omitempty"` // "tcp" or "udp"
	Inbound        string   `json:"inbound,omitempty"`
	InboundType    string   `json:"inbound_type,omitempty"`
	Outbound       string   `json:"outbound,omitempty"`
	OutboundType   string   `json:"outbound_type,omitempty"`
	User           string   `json:"user,omitempty"`
	Protocol       string   `json:"protocol,omitempty"`
	Client         string   `json:"client,omitempty"`
	Error          string   `json:"error,omitempty"`
	UploadBytes    int64    `json:"upload_bytes,omitempty"`
	DownloadBytes  int64    `json:"download_bytes,omitempty"`
	DestAddresses  []string `json:"dest_addresses,omitempty"`
}

// DNSEvent represents a DNS query/response event
type DNSEvent struct {
	Action      string   `json:"action"` // "query", "exchange", "cached", "rejected"
	Domain      string   `json:"domain"`
	QueryType   string   `json:"query_type,omitempty"`
	Transport   string   `json:"transport,omitempty"`
	Rcode       string   `json:"rcode,omitempty"`
	RcodeNum    int      `json:"rcode_num,omitempty"`
	TTL         uint32   `json:"ttl,omitempty"`
	Cached      bool     `json:"cached"`
	Rejected    bool     `json:"rejected"`
	Answers     []string `json:"answers,omitempty"`
	Error       string   `json:"error,omitempty"`
}

// RouterMatchEvent represents a router rule matching event
type RouterMatchEvent struct {
	RuleIndex   int    `json:"rule_index"`
	Rule        string `json:"rule"`
	Action      string `json:"action"`
	Outbound    string `json:"outbound,omitempty"`
	Matched     bool   `json:"matched"`
}

// TransferEvent represents data transfer progress
type TransferEvent struct {
	Direction string `json:"direction"` // "upload" or "download"
	Status    string `json:"status"`    // "started", "finished", "closed", "error"
	Bytes     int64  `json:"bytes,omitempty"`
	Error     string `json:"error,omitempty"`
}

// NewConnectionEvent creates a structured connection event
func NewConnectionEvent(direction, action string) *ConnectionEvent {
	return &ConnectionEvent{
		Direction: direction,
		Action:    action,
	}
}

// NewDNSEvent creates a structured DNS event
func NewDNSEvent(action, domain string) *DNSEvent {
	return &DNSEvent{
		Action: action,
		Domain: domain,
	}
}

// NewRouterMatchEvent creates a structured router match event
func NewRouterMatchEvent(ruleIndex int, rule, action string) *RouterMatchEvent {
	return &RouterMatchEvent{
		RuleIndex: ruleIndex,
		Rule:      rule,
		Action:    action,
	}
}

// NewTransferEvent creates a structured transfer event
func NewTransferEvent(direction, status string) *TransferEvent {
	return &TransferEvent{
		Direction: direction,
		Status:    status,
	}
}

// WithSource sets the source address
func (e *ConnectionEvent) WithSource(addr M.Socksaddr) *ConnectionEvent {
	if addr.IsValid() {
		e.Source = addr.Addr.String()
		e.SourcePort = addr.Port
	}
	return e
}

// WithDestination sets the destination address
func (e *ConnectionEvent) WithDestination(addr M.Socksaddr) *ConnectionEvent {
	if addr.IsValid() {
		e.Destination = addr.Addr.String()
		e.DestPort = addr.Port
		if addr.IsFqdn() {
			e.Domain = addr.Fqdn
		}
	}
	return e
}

// WithNetwork sets the network type
func (e *ConnectionEvent) WithNetwork(network string) *ConnectionEvent {
	e.Network = network
	return e
}

// WithInbound sets the inbound information
func (e *ConnectionEvent) WithInbound(tag, inboundType string) *ConnectionEvent {
	e.Inbound = tag
	e.InboundType = inboundType
	return e
}

// WithOutbound sets the outbound information
func (e *ConnectionEvent) WithOutbound(tag, outboundType string) *ConnectionEvent {
	e.Outbound = tag
	e.OutboundType = outboundType
	return e
}

// WithUser sets the user
func (e *ConnectionEvent) WithUser(user string) *ConnectionEvent {
	if user != "" {
		e.User = user
	}
	return e
}

// WithProtocol sets the sniffed protocol
func (e *ConnectionEvent) WithProtocol(protocol, client string) *ConnectionEvent {
	if protocol != "" {
		e.Protocol = protocol
	}
	if client != "" {
		e.Client = client
	}
	return e
}

// WithError sets the error
func (e *ConnectionEvent) WithError(err error) *ConnectionEvent {
	if err != nil {
		e.Error = err.Error()
	}
	return e
}

// WithDestAddresses sets the resolved destination addresses
func (e *ConnectionEvent) WithDestAddresses(addrs []string) *ConnectionEvent {
	if len(addrs) > 0 {
		e.DestAddresses = addrs
	}
	return e
}

// WithTransferStats sets transfer statistics
func (e *ConnectionEvent) WithTransferStats(upload, download int64) *ConnectionEvent {
	if upload > 0 {
		e.UploadBytes = upload
	}
	if download > 0 {
		e.DownloadBytes = download
	}
	return e
}

// WithQueryType sets the DNS query type
func (e *DNSEvent) WithQueryType(queryType uint16) *DNSEvent {
	e.QueryType = dns.Type(queryType).String()
	return e
}

// WithTransport sets the DNS transport
func (e *DNSEvent) WithTransport(transport string) *DNSEvent {
	if transport != "" {
		e.Transport = transport
	}
	return e
}

// WithResponse sets the DNS response details
func (e *DNSEvent) WithResponse(rcode int, ttl uint32) *DNSEvent {
	e.Rcode = dns.RcodeToString[rcode]
	e.RcodeNum = rcode
	e.TTL = ttl
	return e
}

// WithAnswers sets the DNS answers
func (e *DNSEvent) WithAnswers(answers []string) *DNSEvent {
	if len(answers) > 0 {
		e.Answers = answers
	}
	return e
}

// WithCached marks the DNS response as cached
func (e *DNSEvent) WithCached() *DNSEvent {
	e.Cached = true
	return e
}

// WithRejected marks the DNS query as rejected
func (e *DNSEvent) WithRejected() *DNSEvent {
	e.Rejected = true
	return e
}

// WithError sets the error
func (e *DNSEvent) WithError(err error) *DNSEvent {
	if err != nil {
		e.Error = err.Error()
	}
	return e
}

// WithOutbound sets the matched outbound
func (e *RouterMatchEvent) WithOutbound(outbound string) *RouterMatchEvent {
	if outbound != "" {
		e.Outbound = outbound
	}
	return e
}

// WithMatched sets whether the rule matched
func (e *RouterMatchEvent) WithMatched(matched bool) *RouterMatchEvent {
	e.Matched = matched
	return e
}

// WithBytes sets the transfer bytes
func (e *TransferEvent) WithBytes(bytes int64) *TransferEvent {
	e.Bytes = bytes
	return e
}

// WithError sets the error
func (e *TransferEvent) WithError(err error) *TransferEvent {
	if err != nil {
		e.Error = err.Error()
	}
	return e
}

// ToMap converts ConnectionEvent to map
func (e *ConnectionEvent) ToMap() map[string]interface{} {
	m := make(map[string]interface{})
	m["direction"] = e.Direction
	m["action"] = e.Action
	if e.Source != "" {
		m["source"] = e.Source
		m["source_port"] = e.SourcePort
	}
	if e.Destination != "" {
		m["destination"] = e.Destination
		m["dest_port"] = e.DestPort
	}
	if e.Domain != "" {
		m["domain"] = e.Domain
	}
	if e.Network != "" {
		m["network"] = e.Network
	}
	if e.Inbound != "" {
		m["inbound"] = e.Inbound
	}
	if e.InboundType != "" {
		m["inbound_type"] = e.InboundType
	}
	if e.Outbound != "" {
		m["outbound"] = e.Outbound
	}
	if e.OutboundType != "" {
		m["outbound_type"] = e.OutboundType
	}
	if e.User != "" {
		m["user"] = e.User
	}
	if e.Protocol != "" {
		m["protocol"] = e.Protocol
	}
	if e.Client != "" {
		m["client"] = e.Client
	}
	if e.Error != "" {
		m["error"] = e.Error
	}
	if e.UploadBytes > 0 {
		m["upload_bytes"] = e.UploadBytes
	}
	if e.DownloadBytes > 0 {
		m["download_bytes"] = e.DownloadBytes
	}
	if len(e.DestAddresses) > 0 {
		m["dest_addresses"] = e.DestAddresses
	}
	return m
}

// ToMap converts DNSEvent to map
func (e *DNSEvent) ToMap() map[string]interface{} {
	m := make(map[string]interface{})
	m["action"] = e.Action
	m["domain"] = e.Domain
	if e.QueryType != "" {
		m["query_type"] = e.QueryType
	}
	if e.Transport != "" {
		m["transport"] = e.Transport
	}
	if e.Rcode != "" {
		m["rcode"] = e.Rcode
		m["rcode_num"] = e.RcodeNum
	}
	if e.TTL > 0 {
		m["ttl"] = e.TTL
	}
	m["cached"] = e.Cached
	m["rejected"] = e.Rejected
	if len(e.Answers) > 0 {
		m["answers"] = e.Answers
	}
	if e.Error != "" {
		m["error"] = e.Error
	}
	return m
}

// ToMap converts RouterMatchEvent to map
func (e *RouterMatchEvent) ToMap() map[string]interface{} {
	m := make(map[string]interface{})
	m["rule_index"] = e.RuleIndex
	m["rule"] = e.Rule
	m["action"] = e.Action
	if e.Outbound != "" {
		m["outbound"] = e.Outbound
	}
	m["matched"] = e.Matched
	return m
}

// ToMap converts TransferEvent to map
func (e *TransferEvent) ToMap() map[string]interface{} {
	m := make(map[string]interface{})
	m["direction"] = e.Direction
	m["status"] = e.Status
	if e.Bytes > 0 {
		m["bytes"] = e.Bytes
	}
	if e.Error != "" {
		m["error"] = e.Error
	}
	return m
}
