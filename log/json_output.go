package log

import (
	"encoding/json"
	"io"
	"os"
	"time"

	"github.com/sagernet/sing/common"
)

var _ Output = (*JSONOutput)(nil)

// JSONOutput formats logs as JSON
type JSONOutput struct {
	writer   io.Writer
	encoder  *json.Encoder
	file     *os.File
	filePath string
	hostname string
	version  string
}

// NewJSONOutput creates a new JSON output
func NewJSONOutput(writer io.Writer, filePath, hostname, version string) Output {
	output := &JSONOutput{
		writer:   writer,
		filePath: filePath,
		hostname: hostname,
		version:  version,
	}
	if writer != nil {
		output.encoder = json.NewEncoder(writer)
	}
	return output
}

// Start opens the file if this is a file output
func (o *JSONOutput) Start() error {
	if o.filePath != "" && o.writer == nil {
		file, err := os.OpenFile(o.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		o.file = file
		o.writer = file
		o.encoder = json.NewEncoder(file)
	}
	return nil
}

// Write writes a JSON-formatted log entry
func (o *JSONOutput) Write(entry LogEntry) error {
	if o.encoder == nil {
		return nil
	}

	doc := o.buildJSONDocument(entry)
	return o.encoder.Encode(doc)
}

// Close flushes and closes the output
func (o *JSONOutput) Close() error {
	return common.Close(common.PtrOrNil(o.file))
}

// buildJSONDocument builds a JSON document from a LogEntry
func (o *JSONOutput) buildJSONDocument(entry LogEntry) map[string]interface{} {
	doc := make(map[string]interface{})

	// Top-level fields
	doc["@timestamp"] = entry.Timestamp.UTC().Format(time.RFC3339Nano)
	doc["level"] = FormatLevel(entry.Level)
	doc["message"] = entry.Message
	if entry.Tag != "" {
		doc["tag"] = entry.Tag
	}

	// Connection info
	if entry.ConnectionID != 0 {
		conn := make(map[string]interface{})
		conn["id"] = entry.ConnectionID
		if entry.ConnectionDuration > 0 {
			conn["duration_ms"] = entry.ConnectionDuration.Milliseconds()
		}

		// Add network type
		if network, ok := entry.Metadata["network"].(string); ok {
			conn["network"] = network
		}

		// Source
		if sourceIP, ok := entry.Metadata["source_ip"]; ok {
			source := make(map[string]interface{})
			source["ip"] = sourceIP
			if sourcePort, ok := entry.Metadata["source_port"]; ok {
				source["port"] = sourcePort
			}
			conn["source"] = source
		}

		// Destination
		if destIP, ok := entry.Metadata["dest_ip"]; ok {
			dest := make(map[string]interface{})
			dest["ip"] = destIP
			if destPort, ok := entry.Metadata["dest_port"]; ok {
				dest["port"] = destPort
			}
			if domain, ok := entry.Metadata["dest_domain"]; ok {
				dest["domain"] = domain
			}
			if addresses, ok := entry.Metadata["dest_addresses"].([]string); ok && len(addresses) > 0 {
				dest["addresses"] = addresses
			}
			conn["destination"] = dest
		}

		// Original destination (if different)
		if origIP, ok := entry.Metadata["origin_dest_ip"]; ok {
			origDest := make(map[string]interface{})
			origDest["ip"] = origIP
			if origPort, ok := entry.Metadata["origin_dest_port"]; ok {
				origDest["port"] = origPort
			}
			conn["original_destination"] = origDest
		}

		doc["connection"] = conn
	}

	// Inbound
	inbound := make(map[string]interface{})
	if tag, ok := entry.Metadata["inbound_tag"]; ok {
		inbound["tag"] = tag
	}
	if itype, ok := entry.Metadata["inbound_type"]; ok {
		inbound["type"] = itype
	}
	if user, ok := entry.Metadata["user"]; ok {
		inbound["user"] = user
	}
	if len(inbound) > 0 {
		doc["inbound"] = inbound
	}

	// Outbound
	outbound := make(map[string]interface{})
	if tag, ok := entry.Metadata["outbound_tag"]; ok {
		outbound["tag"] = tag
	}
	if ruleset, ok := entry.Metadata["matched_ruleset"]; ok {
		outbound["matched_rule"] = ruleset
	}
	if len(outbound) > 0 {
		doc["outbound"] = outbound
	}

	// Protocol
	protocol := make(map[string]interface{})
	if proto, ok := entry.Metadata["protocol"]; ok {
		protocol["name"] = proto
	}
	if snifferNames, ok := entry.Metadata["sniffer_names"].([]string); ok && len(snifferNames) > 0 {
		protocol["sniffer_names"] = snifferNames
	}
	if sniffError, ok := entry.Metadata["sniff_error"]; ok {
		protocol["sniff_error"] = sniffError
	}
	if len(protocol) > 0 {
		doc["protocol"] = protocol
	}

	// TLS
	tls := make(map[string]interface{})
	if client, ok := entry.Metadata["tls_client"]; ok {
		tls["client"] = client
	}
	if fragment, ok := entry.Metadata["tls_fragment"].(bool); ok && fragment {
		tls["fragment"] = true
	}
	if recordFragment, ok := entry.Metadata["tls_record_fragment"].(bool); ok && recordFragment {
		tls["record_fragment"] = true
	}
	if fallbackDelay, ok := entry.Metadata["tls_fragment_fallback_delay_ms"].(int64); ok {
		tls["fragment_fallback_delay_ms"] = fallbackDelay
	}
	if len(tls) > 0 {
		doc["tls"] = tls
	}

	// DNS
	dns := make(map[string]interface{})
	if queryType, ok := entry.Metadata["dns_query_type"]; ok {
		dns["query_type"] = queryType
	}
	if fakeIP, ok := entry.Metadata["fake_ip"].(bool); ok && fakeIP {
		dns["fake_ip"] = true
	}
	if len(dns) > 0 {
		doc["dns"] = dns
	}

	// Process
	if processInfo, ok := entry.Metadata["process"].(map[string]interface{}); ok && len(processInfo) > 0 {
		doc["process"] = processInfo
	}

	// Routing
	routing := make(map[string]interface{})
	if sourceGeoIP, ok := entry.Metadata["source_geoip"]; ok {
		routing["source_geoip"] = sourceGeoIP
	}
	if destGeoIP, ok := entry.Metadata["dest_geoip"]; ok {
		routing["dest_geoip"] = destGeoIP
	}
	if len(routing) > 0 {
		doc["routing"] = routing
	}

	// Network strategy
	networkStrategy := make(map[string]interface{})
	if strategy, ok := entry.Metadata["network_strategy"]; ok {
		networkStrategy["strategy"] = strategy
	}
	if networkTypes, ok := entry.Metadata["network_type"].([]string); ok && len(networkTypes) > 0 {
		networkStrategy["network_type"] = networkTypes
	}
	if fallbackNetworkTypes, ok := entry.Metadata["fallback_network_type"].([]string); ok && len(fallbackNetworkTypes) > 0 {
		networkStrategy["fallback_network_type"] = fallbackNetworkTypes
	}
	if fallbackDelay, ok := entry.Metadata["fallback_delay_ms"].(int64); ok {
		networkStrategy["fallback_delay_ms"] = fallbackDelay
	}
	if len(networkStrategy) > 0 {
		doc["network_strategy"] = networkStrategy
	}

	// Host info
	host := make(map[string]interface{})
	if o.hostname != "" {
		host["hostname"] = o.hostname
	}
	if o.version != "" {
		host["version"] = o.version
	}
	if len(host) > 0 {
		doc["host"] = host
	}

	// Structured event data (connection, DNS, router, etc.)
	if entry.Event != nil {
		event := make(map[string]interface{})
		event["type"] = string(entry.Event.Type)

		// Include event-specific data
		if entry.Event.Data != nil {
			for k, v := range entry.Event.Data {
				event[k] = v
			}
		}

		doc["event"] = event
	}

	return doc
}
