package log

// Additional EventType constants for new event categories
const (
	EventTypeHealthCheck        EventType = "health_check"
	EventTypeNetworkState       EventType = "network_state"
	EventTypeRuleSet            EventType = "rule_set"
	EventTypeTLS                EventType = "tls"
	EventTypeComponentLifecycle EventType = "component_lifecycle"
	EventTypeServerLifecycle    EventType = "server_lifecycle"
	EventTypeTransportProtocol  EventType = "transport_protocol"
	EventTypeHTTPRoute          EventType = "http_route"
	EventTypeAuth               EventType = "auth"
	EventTypeService            EventType = "service"
)

// ---------------------- HealthCheckEvent ----------------------

// HealthCheckEvent represents a health check event for outbound groups
type HealthCheckEvent struct {
	Action   string  `json:"action"`             // "start", "success", "failed", "timeout"
	Outbound string  `json:"outbound,omitempty"` // checked outbound tag
	URL      string  `json:"url,omitempty"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
	Error    string  `json:"error,omitempty"`
}

func NewHealthCheckEvent(action string) *HealthCheckEvent {
	return &HealthCheckEvent{Action: action}
}

func (e *HealthCheckEvent) WithOutbound(tag string) *HealthCheckEvent {
	if tag != "" {
		e.Outbound = tag
	}
	return e
}

func (e *HealthCheckEvent) WithURL(url string) *HealthCheckEvent {
	if url != "" {
		e.URL = url
	}
	return e
}

func (e *HealthCheckEvent) WithLatency(ms int64) *HealthCheckEvent {
	e.LatencyMs = ms
	return e
}

func (e *HealthCheckEvent) WithError(err error) *HealthCheckEvent {
	if err != nil {
		e.Error = err.Error()
	}
	return e
}

func (e *HealthCheckEvent) ToMap() map[string]interface{} {
	m := map[string]interface{}{"action": e.Action}
	if e.Outbound != "" {
		m["outbound"] = e.Outbound
	}
	if e.URL != "" {
		m["url"] = e.URL
	}
	if e.LatencyMs > 0 {
		m["latency_ms"] = e.LatencyMs
	}
	if e.Error != "" {
		m["error"] = e.Error
	}
	return m
}

func (e *HealthCheckEvent) ToStructuredEvent() *StructuredEvent {
	return &StructuredEvent{Type: EventTypeHealthCheck, Data: e.ToMap()}
}

// ---------------------- NetworkStateEvent ----------------------

// NetworkStateEvent represents a network state change event
type NetworkStateEvent struct {
	Action      string   `json:"action"`                // "update", "change", "error"
	Interfaces  []string `json:"interfaces,omitempty"`
	DefaultAddr string   `json:"default_addr,omitempty"`
	Error       string   `json:"error,omitempty"`
}

func NewNetworkStateEvent(action string) *NetworkStateEvent {
	return &NetworkStateEvent{Action: action}
}

func (e *NetworkStateEvent) WithInterfaces(ifaces []string) *NetworkStateEvent {
	if len(ifaces) > 0 {
		e.Interfaces = ifaces
	}
	return e
}

func (e *NetworkStateEvent) WithDefaultAddr(addr string) *NetworkStateEvent {
	if addr != "" {
		e.DefaultAddr = addr
	}
	return e
}

func (e *NetworkStateEvent) WithError(err error) *NetworkStateEvent {
	if err != nil {
		e.Error = err.Error()
	}
	return e
}

func (e *NetworkStateEvent) ToMap() map[string]interface{} {
	m := map[string]interface{}{"action": e.Action}
	if len(e.Interfaces) > 0 {
		m["interfaces"] = e.Interfaces
	}
	if e.DefaultAddr != "" {
		m["default_addr"] = e.DefaultAddr
	}
	if e.Error != "" {
		m["error"] = e.Error
	}
	return m
}

func (e *NetworkStateEvent) ToStructuredEvent() *StructuredEvent {
	return &StructuredEvent{Type: EventTypeNetworkState, Data: e.ToMap()}
}

// ---------------------- RuleSetEvent ----------------------

// RuleSetEvent represents a rule set lifecycle event
type RuleSetEvent struct {
	Action  string `json:"action"`            // "load", "update", "error", "download"
	Tag     string `json:"tag,omitempty"`
	Format  string `json:"format,omitempty"`
	URL     string `json:"url,omitempty"`
	Count   int    `json:"count,omitempty"`
	Error   string `json:"error,omitempty"`
}

func NewRuleSetEvent(action, tag string) *RuleSetEvent {
	return &RuleSetEvent{Action: action, Tag: tag}
}

func (e *RuleSetEvent) WithFormat(format string) *RuleSetEvent {
	if format != "" {
		e.Format = format
	}
	return e
}

func (e *RuleSetEvent) WithURL(url string) *RuleSetEvent {
	if url != "" {
		e.URL = url
	}
	return e
}

func (e *RuleSetEvent) WithCount(count int) *RuleSetEvent {
	e.Count = count
	return e
}

func (e *RuleSetEvent) WithError(err error) *RuleSetEvent {
	if err != nil {
		e.Error = err.Error()
	}
	return e
}

func (e *RuleSetEvent) ToMap() map[string]interface{} {
	m := map[string]interface{}{"action": e.Action}
	if e.Tag != "" {
		m["tag"] = e.Tag
	}
	if e.Format != "" {
		m["format"] = e.Format
	}
	if e.URL != "" {
		m["url"] = e.URL
	}
	if e.Count > 0 {
		m["count"] = e.Count
	}
	if e.Error != "" {
		m["error"] = e.Error
	}
	return m
}

func (e *RuleSetEvent) ToStructuredEvent() *StructuredEvent {
	return &StructuredEvent{Type: EventTypeRuleSet, Data: e.ToMap()}
}

// ---------------------- TLSEvent ----------------------

// TLSEvent represents a TLS lifecycle event (cert reload, ACME, etc.)
type TLSEvent struct {
	Action      string `json:"action"`                 // "cert_reload", "ech_reload", "acme_obtain", "acme_renew", "error"
	Domain      string `json:"domain,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Error       string `json:"error,omitempty"`
}

func NewTLSEvent(action string) *TLSEvent {
	return &TLSEvent{Action: action}
}

func (e *TLSEvent) WithDomain(domain string) *TLSEvent {
	if domain != "" {
		e.Domain = domain
	}
	return e
}

func (e *TLSEvent) WithFingerprint(fp string) *TLSEvent {
	if fp != "" {
		e.Fingerprint = fp
	}
	return e
}

func (e *TLSEvent) WithError(err error) *TLSEvent {
	if err != nil {
		e.Error = err.Error()
	}
	return e
}

func (e *TLSEvent) ToMap() map[string]interface{} {
	m := map[string]interface{}{"action": e.Action}
	if e.Domain != "" {
		m["domain"] = e.Domain
	}
	if e.Fingerprint != "" {
		m["fingerprint"] = e.Fingerprint
	}
	if e.Error != "" {
		m["error"] = e.Error
	}
	return m
}

func (e *TLSEvent) ToStructuredEvent() *StructuredEvent {
	return &StructuredEvent{Type: EventTypeTLS, Data: e.ToMap()}
}

// ---------------------- ComponentLifecycleEvent ----------------------

// ComponentLifecycleEvent represents start/stop/update events for adapters and core components
type ComponentLifecycleEvent struct {
	Action        string `json:"action"`                  // "start", "close", "add", "remove", "update"
	ComponentType string `json:"component_type"`          // "inbound", "outbound", "endpoint", "service"
	Tag           string `json:"tag,omitempty"`
	Error         string `json:"error,omitempty"`
}

func NewComponentLifecycleEvent(action, componentType string) *ComponentLifecycleEvent {
	return &ComponentLifecycleEvent{Action: action, ComponentType: componentType}
}

func (e *ComponentLifecycleEvent) WithTag(tag string) *ComponentLifecycleEvent {
	if tag != "" {
		e.Tag = tag
	}
	return e
}

func (e *ComponentLifecycleEvent) WithError(err error) *ComponentLifecycleEvent {
	if err != nil {
		e.Error = err.Error()
	}
	return e
}

func (e *ComponentLifecycleEvent) ToMap() map[string]interface{} {
	m := map[string]interface{}{
		"action":         e.Action,
		"component_type": e.ComponentType,
	}
	if e.Tag != "" {
		m["tag"] = e.Tag
	}
	if e.Error != "" {
		m["error"] = e.Error
	}
	return m
}

func (e *ComponentLifecycleEvent) ToStructuredEvent() *StructuredEvent {
	return &StructuredEvent{Type: EventTypeComponentLifecycle, Data: e.ToMap()}
}

// ---------------------- ServerLifecycleEvent ----------------------

// ServerLifecycleEvent represents listener/server start/stop events
type ServerLifecycleEvent struct {
	Action  string `json:"action"`           // "listen", "close", "accept", "error"
	Network string `json:"network,omitempty"` // "tcp", "udp"
	Address string `json:"address,omitempty"`
	Error   string `json:"error,omitempty"`
}

func NewServerLifecycleEvent(action string) *ServerLifecycleEvent {
	return &ServerLifecycleEvent{Action: action}
}

func (e *ServerLifecycleEvent) WithNetwork(network string) *ServerLifecycleEvent {
	if network != "" {
		e.Network = network
	}
	return e
}

func (e *ServerLifecycleEvent) WithAddress(addr string) *ServerLifecycleEvent {
	if addr != "" {
		e.Address = addr
	}
	return e
}

func (e *ServerLifecycleEvent) WithError(err error) *ServerLifecycleEvent {
	if err != nil {
		e.Error = err.Error()
	}
	return e
}

func (e *ServerLifecycleEvent) ToMap() map[string]interface{} {
	m := map[string]interface{}{"action": e.Action}
	if e.Network != "" {
		m["network"] = e.Network
	}
	if e.Address != "" {
		m["address"] = e.Address
	}
	if e.Error != "" {
		m["error"] = e.Error
	}
	return m
}

func (e *ServerLifecycleEvent) ToStructuredEvent() *StructuredEvent {
	return &StructuredEvent{Type: EventTypeServerLifecycle, Data: e.ToMap()}
}

// ---------------------- TransportProtocolEvent ----------------------

// TransportProtocolEvent represents transport-layer protocol events (mux, xhttp, etc.)
type TransportProtocolEvent struct {
	Action    string `json:"action"`              // "accept", "connect", "close", "error"
	Transport string `json:"transport,omitempty"` // "xhttp", "mux", "wireguard"
	Direction string `json:"direction,omitempty"` // "inbound", "outbound"
	Error     string `json:"error,omitempty"`
}

func NewTransportProtocolEvent(action, transport string) *TransportProtocolEvent {
	return &TransportProtocolEvent{Action: action, Transport: transport}
}

func (e *TransportProtocolEvent) WithDirection(direction string) *TransportProtocolEvent {
	if direction != "" {
		e.Direction = direction
	}
	return e
}

func (e *TransportProtocolEvent) WithError(err error) *TransportProtocolEvent {
	if err != nil {
		e.Error = err.Error()
	}
	return e
}

func (e *TransportProtocolEvent) ToMap() map[string]interface{} {
	m := map[string]interface{}{"action": e.Action}
	if e.Transport != "" {
		m["transport"] = e.Transport
	}
	if e.Direction != "" {
		m["direction"] = e.Direction
	}
	if e.Error != "" {
		m["error"] = e.Error
	}
	return m
}

func (e *TransportProtocolEvent) ToStructuredEvent() *StructuredEvent {
	return &StructuredEvent{Type: EventTypeTransportProtocol, Data: e.ToMap()}
}

// ---------------------- HTTPRouteEvent ----------------------

// HTTPRouteEvent represents an HTTP routing/proxying event
type HTTPRouteEvent struct {
	Action      string `json:"action"`               // "route", "connect", "upgrade", "error"
	Method      string `json:"method,omitempty"`
	URL         string `json:"url,omitempty"`
	Host        string `json:"host,omitempty"`
	StatusCode  int    `json:"status_code,omitempty"`
	Protocol    string `json:"protocol,omitempty"` // "http/1.1", "h2", "h3"
	Error       string `json:"error,omitempty"`
}

func NewHTTPRouteEvent(action string) *HTTPRouteEvent {
	return &HTTPRouteEvent{Action: action}
}

func (e *HTTPRouteEvent) WithMethod(method string) *HTTPRouteEvent {
	if method != "" {
		e.Method = method
	}
	return e
}

func (e *HTTPRouteEvent) WithURL(url string) *HTTPRouteEvent {
	if url != "" {
		e.URL = url
	}
	return e
}

func (e *HTTPRouteEvent) WithHost(host string) *HTTPRouteEvent {
	if host != "" {
		e.Host = host
	}
	return e
}

func (e *HTTPRouteEvent) WithStatusCode(code int) *HTTPRouteEvent {
	e.StatusCode = code
	return e
}

func (e *HTTPRouteEvent) WithProtocol(proto string) *HTTPRouteEvent {
	if proto != "" {
		e.Protocol = proto
	}
	return e
}

func (e *HTTPRouteEvent) WithError(err error) *HTTPRouteEvent {
	if err != nil {
		e.Error = err.Error()
	}
	return e
}

func (e *HTTPRouteEvent) ToMap() map[string]interface{} {
	m := map[string]interface{}{"action": e.Action}
	if e.Method != "" {
		m["method"] = e.Method
	}
	if e.URL != "" {
		m["url"] = e.URL
	}
	if e.Host != "" {
		m["host"] = e.Host
	}
	if e.StatusCode != 0 {
		m["status_code"] = e.StatusCode
	}
	if e.Protocol != "" {
		m["protocol"] = e.Protocol
	}
	if e.Error != "" {
		m["error"] = e.Error
	}
	return m
}

func (e *HTTPRouteEvent) ToStructuredEvent() *StructuredEvent {
	return &StructuredEvent{Type: EventTypeHTTPRoute, Data: e.ToMap()}
}

// ---------------------- AuthEvent ----------------------

// AuthEvent represents an authentication event
type AuthEvent struct {
	Action   string `json:"action"`            // "success", "failed", "denied"
	User     string `json:"user,omitempty"`
	Service  string `json:"service,omitempty"` // service name (ccm, ocm, etc.)
	Error    string `json:"error,omitempty"`
}

func NewAuthEvent(action string) *AuthEvent {
	return &AuthEvent{Action: action}
}

func (e *AuthEvent) WithUser(user string) *AuthEvent {
	if user != "" {
		e.User = user
	}
	return e
}

func (e *AuthEvent) WithService(service string) *AuthEvent {
	if service != "" {
		e.Service = service
	}
	return e
}

func (e *AuthEvent) WithError(err error) *AuthEvent {
	if err != nil {
		e.Error = err.Error()
	}
	return e
}

func (e *AuthEvent) ToMap() map[string]interface{} {
	m := map[string]interface{}{"action": e.Action}
	if e.User != "" {
		m["user"] = e.User
	}
	if e.Service != "" {
		m["service"] = e.Service
	}
	if e.Error != "" {
		m["error"] = e.Error
	}
	return m
}

func (e *AuthEvent) ToStructuredEvent() *StructuredEvent {
	return &StructuredEvent{Type: EventTypeAuth, Data: e.ToMap()}
}

// ---------------------- ServiceEvent ----------------------

// ServiceEvent represents a general service-level event
type ServiceEvent struct {
	Action  string `json:"action"`           // "start", "stop", "reload", "error"
	Service string `json:"service,omitempty"` // service name/tag
	Error   string `json:"error,omitempty"`
}

func NewServiceEvent(action, service string) *ServiceEvent {
	return &ServiceEvent{Action: action, Service: service}
}

func (e *ServiceEvent) WithError(err error) *ServiceEvent {
	if err != nil {
		e.Error = err.Error()
	}
	return e
}

func (e *ServiceEvent) ToMap() map[string]interface{} {
	m := map[string]interface{}{"action": e.Action}
	if e.Service != "" {
		m["service"] = e.Service
	}
	if e.Error != "" {
		m["error"] = e.Error
	}
	return m
}

func (e *ServiceEvent) ToStructuredEvent() *StructuredEvent {
	return &StructuredEvent{Type: EventTypeService, Data: e.ToMap()}
}
