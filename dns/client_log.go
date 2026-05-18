package dns

import (
	"context"
	"strings"

	"github.com/sagernet/sing-box/log"

	"github.com/miekg/dns"
)

func logCachedResponse(logger log.StructuredLogger, ctx context.Context, response *dns.Msg, ttl int) {
	if logger == nil || len(response.Question) == 0 {
		return
	}
	domain := FqdnToDomain(response.Question[0].Name)
	logger.DebugEventContext(ctx, "dns.cache.hit", "cached response", log.String("domain", domain), log.String("rcode", dns.RcodeToString[response.Rcode]), log.Int("ttl", ttl))
	for _, recordList := range [][]dns.RR{response.Answer, response.Ns, response.Extra} {
		for _, record := range recordList {
			logger.InfoEventContext(ctx, "dns.cache.record", "cached record", log.String("record_type", dns.Type(record.Header().Rrtype).String()), log.String("record", FormatQuestion(record.String())))
		}
	}
}

func logOptimisticResponse(logger log.StructuredLogger, ctx context.Context, response *dns.Msg) {
	if logger == nil || len(response.Question) == 0 {
		return
	}
	domain := FqdnToDomain(response.Question[0].Name)
	logger.DebugEventContext(ctx, "dns.cache.optimistic", "optimistic response", log.String("domain", domain), log.String("rcode", dns.RcodeToString[response.Rcode]))
	for _, recordList := range [][]dns.RR{response.Answer, response.Ns, response.Extra} {
		for _, record := range recordList {
			logger.InfoEventContext(ctx, "dns.cache.optimistic.record", "optimistic record", log.String("record_type", dns.Type(record.Header().Rrtype).String()), log.String("record", FormatQuestion(record.String())))
		}
	}
}

func logExchangedResponse(logger log.StructuredLogger, ctx context.Context, response *dns.Msg, ttl uint32) {
	if logger == nil || len(response.Question) == 0 {
		return
	}
	domain := FqdnToDomain(response.Question[0].Name)
	logger.DebugEventContext(ctx, "dns.response", "exchanged response", log.String("domain", domain), log.String("rcode", dns.RcodeToString[response.Rcode]), log.Uint("ttl", uint(ttl)))
	for _, recordList := range [][]dns.RR{response.Answer, response.Ns, response.Extra} {
		for _, record := range recordList {
			logger.InfoEventContext(ctx, "dns.response.record", "exchanged record", log.String("record_type", dns.Type(record.Header().Rrtype).String()), log.String("record", FormatQuestion(record.String())))
		}
	}
}

func logRefreshedResponse(logger log.StructuredLogger, ctx context.Context, response *dns.Msg, ttl uint32) {
	if logger == nil || len(response.Question) == 0 {
		return
	}
	domain := FqdnToDomain(response.Question[0].Name)
	logger.DebugEventContext(ctx, "dns.cache.refresh", "refreshed response", log.String("domain", domain), log.String("rcode", dns.RcodeToString[response.Rcode]), log.Uint("ttl", uint(ttl)))
	for _, recordList := range [][]dns.RR{response.Answer, response.Ns, response.Extra} {
		for _, record := range recordList {
			logger.InfoEventContext(ctx, "dns.cache.refresh.record", "refreshed record", log.String("record_type", dns.Type(record.Header().Rrtype).String()), log.String("record", FormatQuestion(record.String())))
		}
	}
}

func logRejectedResponse(logger log.StructuredLogger, ctx context.Context, response *dns.Msg) {
	if logger == nil || len(response.Question) == 0 {
		return
	}
	for _, recordList := range [][]dns.RR{response.Answer, response.Ns, response.Extra} {
		for _, record := range recordList {
			logger.InfoEventContext(ctx, "dns.response.rejected.record", "rejected record", log.String("record_type", dns.Type(record.Header().Rrtype).String()), log.String("record", FormatQuestion(record.String())))
		}
	}
}

func FqdnToDomain(fqdn string) string {
	if dns.IsFqdn(fqdn) {
		return fqdn[:len(fqdn)-1]
	}
	return fqdn
}

func FormatQuestion(string string) string {
	for strings.HasPrefix(string, ";") {
		string = string[1:]
	}
	string = strings.ReplaceAll(string, "\t", " ")
	string = strings.ReplaceAll(string, "\n", " ")
	string = strings.ReplaceAll(string, ";; ", " ")
	string = strings.ReplaceAll(string, "; ", " ")

	for strings.Contains(string, "  ") {
		string = strings.ReplaceAll(string, "  ", " ")
	}
	return strings.TrimSpace(string)
}
