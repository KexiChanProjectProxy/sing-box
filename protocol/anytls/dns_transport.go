package anytls

import (
	"context"
	"errors"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/service"

	anytls "github.com/anytls/sing-anytls"
	mDNS "github.com/miekg/dns"
)

func RegisterDNSTransport(registry *dns.TransportRegistry) {
	dns.RegisterTransport[option.AnyTLSDNSServerOptions](registry, C.DNSTypeAnyTLS, NewDNSTransport)
}

var _ adapter.DNSTransport = (*DNSTransport)(nil)

// DNSTransport sends queries to the server of an AnyTLS outbound inside its
// sessions, where the server answers them with its own DNS.
type DNSTransport struct {
	dns.TransportAdapter
	outboundTag     string
	outboundManager adapter.OutboundManager
	outbound        *Outbound
}

func NewDNSTransport(ctx context.Context, logger log.StructuredLogger, tag string, options option.AnyTLSDNSServerOptions) (adapter.DNSTransport, error) {
	if options.Outbound == "" {
		return nil, E.New("missing anytls outbound tag")
	}
	return &DNSTransport{
		// The outbound is not a DNS server, so it is not a dependency here;
		// outbounds are created before DNS servers start.
		TransportAdapter: dns.NewTransportAdapter(C.DNSTypeAnyTLS, tag, nil),
		outboundTag:      options.Outbound,
		outboundManager:  service.FromContext[adapter.OutboundManager](ctx),
	}, nil
}

func (t *DNSTransport) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateInitialize {
		return nil
	}
	rawOutbound, loaded := t.outboundManager.Outbound(t.outboundTag)
	if !loaded {
		return E.New("outbound not found: ", t.outboundTag)
	}
	outbound, isAnyTLS := rawOutbound.(*Outbound)
	if !isAnyTLS {
		return E.New("outbound is not anytls: ", t.outboundTag)
	}
	t.outbound = outbound
	return nil
}

func (t *DNSTransport) Close() error {
	return nil
}

func (t *DNSTransport) Reset() {
}

func (t *DNSTransport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, C.DNSTimeout)
		defer cancel()
	}
	request, err := message.Pack()
	if err != nil {
		return nil, err
	}
	rawResponse, err := t.outbound.ExchangeDNS(ctx, request)
	if err != nil {
		return nil, err
	}
	response := new(mDNS.Msg)
	err = response.Unpack(rawResponse)
	if err != nil {
		return nil, E.Cause(err, "unpack DNS response")
	}
	return response, nil
}

func (t *DNSTransport) ExchangeAsync(ctx context.Context, message *mDNS.Msg, callback func(response *mDNS.Msg, err error)) {
	go func() {
		callback(t.Exchange(ctx, message))
	}()
}

// ExchangeDNS sends a raw DNS message to the server inside a session.
func (h *Outbound) ExchangeDNS(ctx context.Context, request []byte) ([]byte, error) {
	h.statsAccess.Lock()
	client := h.client
	h.statsAccess.Unlock()
	if client == nil {
		return nil, E.New("anytls outbound not started")
	}
	response, err := client.ExchangeDNS(ctx, request)
	if errors.Is(err, anytls.ErrDNSUnsupported) {
		return nil, E.New("server of outbound/anytls[", h.Tag(), "] does not answer DNS: enable dns on its anytls inbound")
	}
	return response, err
}
