package adapter

import (
	"context"

	"github.com/sagernet/sing-box/log"
)

func init() {
	// Register the metadata extractor when adapter package is imported
	log.RegisterMetadataExtractor(extractInboundContextMetadata)
}

// extractInboundContextMetadata extracts metadata from InboundContext for structured logging
func extractInboundContextMetadata(ctx context.Context) map[string]interface{} {
	metadata := ContextFrom(ctx)
	if metadata == nil {
		return nil
	}

	result := make(map[string]interface{})

	// Network information
	if metadata.Network != "" {
		result["network"] = metadata.Network
	}

	// Source address
	if metadata.Source.IsValid() {
		result["source_ip"] = metadata.Source.Addr.String()
		result["source_port"] = metadata.Source.Port
	}

	// Destination address
	if metadata.Destination.IsValid() {
		if metadata.Destination.IsIP() {
			result["dest_ip"] = metadata.Destination.Addr.String()
		}
		if metadata.Destination.Port > 0 {
			result["dest_port"] = metadata.Destination.Port
		}
	}

	// Destination domain
	if metadata.Domain != "" {
		result["dest_domain"] = metadata.Domain
	} else if metadata.Destination.IsFqdn() {
		result["dest_domain"] = metadata.Destination.Fqdn
	}

	// Origin destination (before transparent proxy interception)
	if metadata.OriginDestination.IsValid() {
		result["origin_dest_ip"] = metadata.OriginDestination.Addr.String()
		if metadata.OriginDestination.Port > 0 {
			result["origin_dest_port"] = metadata.OriginDestination.Port
		}
	}

	// Inbound/Outbound tags
	if metadata.Inbound != "" {
		result["inbound_tag"] = metadata.Inbound
	}
	if metadata.InboundType != "" {
		result["inbound_type"] = metadata.InboundType
	}
	if metadata.Outbound != "" {
		result["outbound_tag"] = metadata.Outbound
	}
	if metadata.OutboundType != "" {
		result["outbound_type"] = metadata.OutboundType
	}

	// User
	if metadata.User != "" {
		result["user"] = metadata.User
	}

	// Protocol information
	if metadata.Protocol != "" {
		result["protocol"] = metadata.Protocol
	}

	// TLS client
	if metadata.Client != "" {
		result["tls_client"] = metadata.Client
	}

	// FakeIP
	if metadata.FakeIP {
		result["fake_ip"] = true
	}

	// DNS query type
	if metadata.QueryType > 0 {
		result["dns_query_type"] = metadata.QueryType
	}

	// Process information
	if metadata.ProcessInfo != nil {
		if metadata.ProcessInfo.ProcessPath != "" {
			result["process_path"] = metadata.ProcessInfo.ProcessPath
		}
		if metadata.ProcessInfo.ProcessID > 0 {
			result["process_id"] = metadata.ProcessInfo.ProcessID
		}
		if metadata.ProcessInfo.AndroidPackageName != "" {
			result["android_package_name"] = metadata.ProcessInfo.AndroidPackageName
		}
		if metadata.ProcessInfo.UserName != "" {
			result["user_name"] = metadata.ProcessInfo.UserName
		}
		if metadata.ProcessInfo.UserId != -1 {
			result["user_id"] = metadata.ProcessInfo.UserId
		}
	}

	// GeoIP
	if metadata.SourceGeoIPCode != "" {
		result["source_geoip"] = metadata.SourceGeoIPCode
	}
	if metadata.GeoIPCode != "" {
		result["dest_geoip"] = metadata.GeoIPCode
	}

	// Sniffer
	if len(metadata.SnifferNames) > 0 {
		result["sniffer_names"] = metadata.SnifferNames
	}
	if metadata.SniffError != nil {
		result["sniff_error"] = metadata.SniffError.Error()
	}

	// TLS fragment settings
	if metadata.TLSFragment {
		result["tls_fragment"] = true
	}
	if metadata.TLSFragmentFallbackDelay > 0 {
		result["tls_fragment_fallback_delay_ms"] = metadata.TLSFragmentFallbackDelay.Milliseconds()
	}
	if metadata.TLSRecordFragment {
		result["tls_record_fragment"] = true
	}

	// Network strategy
	if metadata.NetworkStrategy != nil {
		result["network_strategy"] = metadata.NetworkStrategy.String()
	}

	// Destination addresses
	if len(metadata.DestinationAddresses) > 0 {
		addrs := make([]string, len(metadata.DestinationAddresses))
		for i, addr := range metadata.DestinationAddresses {
			addrs[i] = addr.String()
		}
		result["dest_addresses"] = addrs
	}

	// IP version
	if metadata.IPVersion > 0 {
		result["ip_version"] = metadata.IPVersion
	}

	// Mark
	if metadata.Mark > 0 {
		result["mark"] = metadata.Mark
	}

	// UDP options
	if metadata.UDPConnect {
		result["udp_connect"] = true
	}
	if metadata.UDPTimeout > 0 {
		result["udp_timeout"] = metadata.UDPTimeout.String()
	}
	if metadata.UDPDisableDomainUnmapping {
		result["udp_disable_domain_unmapping"] = true
	}

	// Network type
	if len(metadata.NetworkType) > 0 {
		types := make([]string, len(metadata.NetworkType))
		for i, t := range metadata.NetworkType {
			types[i] = t.String()
		}
		result["network_type"] = types
	}
	if len(metadata.FallbackNetworkType) > 0 {
		types := make([]string, len(metadata.FallbackNetworkType))
		for i, t := range metadata.FallbackNetworkType {
			types[i] = t.String()
		}
		result["fallback_network_type"] = types
	}
	if metadata.FallbackDelay > 0 {
		result["fallback_delay_ms"] = metadata.FallbackDelay.Milliseconds()
	}

	// Matched ruleset
	if metadata.MatchedRuleSet != "" {
		result["matched_ruleset"] = metadata.MatchedRuleSet
	}

	if len(result) == 0 {
		return nil
	}

	return result
}
