package route

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"golang.org/x/net/publicsuffix"
)

const (
	hashKeyPartSourceIP              = "src_ip"
	hashKeyPartMatchedRuleSet        = "matched_ruleset"
	hashKeyPartMatchedRuleSetOrETLD  = "matched_ruleset_or_etld"
)

func DeriveHashKeyPart(metadata adapter.InboundContext, keyPart string) string {
	switch keyPart {
	case hashKeyPartSourceIP:
		if !metadata.Source.Addr.IsValid() {
			return ""
		}
		return metadata.Source.Addr.String()
	case hashKeyPartMatchedRuleSet:
		return metadata.MatchedRuleSetTag
	case hashKeyPartMatchedRuleSetOrETLD:
		if metadata.MatchedRuleSetTag != "" {
			return metadata.MatchedRuleSetTag
		}
		if metadata.Domain != "" {
			return registeredDomain(metadata.Domain)
		}
		if metadata.Destination.Fqdn != "" {
			return registeredDomain(metadata.Destination.Fqdn)
		}
	}
	return ""
}

func registeredDomain(domain string) string {
	domain = strings.TrimSuffix(strings.ToLower(domain), ".")
	if domain == "" {
		return ""
	}
	registeredDomain, err := publicsuffix.EffectiveTLDPlusOne(domain)
	if err == nil {
		return registeredDomain
	}
	return ""
}
