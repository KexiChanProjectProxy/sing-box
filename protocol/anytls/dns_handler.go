package anytls

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common/auth"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"

	mDNS "github.com/miekg/dns"
)

// maxDNSMessageSize is the largest DNS message a frame can carry.
const maxDNSMessageSize = 0xFFFF

// exchangeDNS answers a DNS query a client sent inside its session. ctx is the
// connection's context, carrying the listener metadata and the user.
//
// The answer goes through the DNS client, so a query sent to a server shares
// its cache and its in-flight deduplication with every other query for that
// server, from sing-box itself or from other clients.
func (h *Inbound) exchangeDNS(ctx context.Context, request []byte) ([]byte, error) {
	var message mDNS.Msg
	err := message.Unpack(request)
	if err != nil {
		return nil, E.Cause(err, "unpack DNS query")
	}
	if message.Response || message.Opcode != mDNS.OpcodeQuery || len(message.Question) != 1 {
		return packDNSResponse(failedDNSResponse(&message, mDNS.RcodeFormatError))
	}

	var metadata adapter.InboundContext
	metadata.Inbound = h.Tag()
	metadata.InboundType = h.Type()
	//nolint:staticcheck
	metadata.InboundDetour = h.listener.ListenOptions().Detour
	if connectionMetadata := adapter.ContextFrom(ctx); connectionMetadata != nil {
		metadata.Source = connectionMetadata.Source
	}
	if userName, _ := auth.UserFromContext[string](ctx); userName != "" {
		metadata.User = userName
	}
	metadata.Network = N.NetworkTCP
	metadata.Protocol = C.ProtocolDNS

	ctx, cancel := context.WithTimeout(ctx, C.DNSTimeout)
	defer cancel()
	ctx = adapter.WithContext(ctx, &metadata)
	question := message.Question[0]
	h.logger.DebugEventContext(ctx, "anytls.dns.query", "DNS query in session",
		log.String("user", metadata.User),
		log.String("domain", question.Name),
		log.String("type", mDNS.TypeToString[question.Qtype]))

	response, err := h.dnsRouter.Exchange(ctx, &message, adapter.DNSQueryOptions{Transport: h.dnsServer})
	if err != nil {
		if E.IsClosedOrCanceled(err) {
			return nil, err
		}
		h.logger.DebugEventContext(ctx, "anytls.dns.error", "DNS query in session failed", log.Err(err))
		response = failedDNSResponse(&message, mDNS.RcodeServerFailure)
	}
	return packDNSResponse(response)
}

// failedDNSResponse answers request with rcode, keeping its id and question.
func failedDNSResponse(request *mDNS.Msg, rcode int) *mDNS.Msg {
	response := new(mDNS.Msg)
	response.Id = request.Id
	response.Response = true
	response.Opcode = request.Opcode
	response.RecursionDesired = request.RecursionDesired
	response.RecursionAvailable = true
	response.Rcode = rcode
	if len(request.Question) == 1 {
		response.Question = request.Question
	}
	return response
}

// packDNSResponse packs response, truncating it to fit a frame. The session is
// a reliable channel, so the UDP size limits do not apply.
func packDNSResponse(response *mDNS.Msg) ([]byte, error) {
	if response.Len() > maxDNSMessageSize {
		response = response.Copy()
		response.Truncate(maxDNSMessageSize)
	}
	return response.Pack()
}
