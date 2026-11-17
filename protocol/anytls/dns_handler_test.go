package anytls

import (
	"context"
	"errors"
	"io"
	"net"
	"net/netip"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	adapterInbound "github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/common/listener"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common/auth"
	M "github.com/sagernet/sing/common/metadata"

	mDNS "github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

type stubDNSRouter struct {
	adapter.DNSRouter
	calls    int
	options  adapter.DNSQueryOptions
	metadata adapter.InboundContext
	respond  func(message *mDNS.Msg) (*mDNS.Msg, error)
}

func (r *stubDNSRouter) Exchange(ctx context.Context, message *mDNS.Msg, options adapter.DNSQueryOptions) (*mDNS.Msg, error) {
	r.calls++
	r.options = options
	if metadata := adapter.ContextFrom(ctx); metadata != nil {
		r.metadata = *metadata
	}
	return r.respond(message)
}

type stubDNSTransport struct {
	adapter.DNSTransport
}

func newDNSTestInbound(router *stubDNSRouter, server adapter.DNSTransport) *Inbound {
	return &Inbound{
		Adapter:   adapterInbound.NewAdapter(C.TypeAnyTLS, "anytls-in"),
		logger:    log.NewNOPFactory().Logger(),
		listener:  listener.New(listener.Options{}),
		dnsRouter: router,
		dnsServer: server,
	}
}

func packQuery(t *testing.T, edit func(message *mDNS.Msg)) []byte {
	t.Helper()
	message := new(mDNS.Msg)
	message.SetQuestion("example.org.", mDNS.TypeA)
	message.Id = 4242
	if edit != nil {
		edit(message)
	}
	request, err := message.Pack()
	require.NoError(t, err)
	return request
}

func unpackResponse(t *testing.T, response []byte) *mDNS.Msg {
	t.Helper()
	var message mDNS.Msg
	require.NoError(t, message.Unpack(response))
	return &message
}

func answer(message *mDNS.Msg) (*mDNS.Msg, error) {
	response := new(mDNS.Msg)
	response.SetReply(message)
	response.Answer = append(response.Answer, &mDNS.A{
		Hdr: mDNS.RR_Header{Name: message.Question[0].Name, Rrtype: mDNS.TypeA, Class: mDNS.ClassINET, Ttl: 60},
		A:   net.IPv4(192, 0, 2, 1),
	})
	return response, nil
}

func connectionContext() context.Context {
	ctx := auth.ContextWithUser(context.Background(), "alice")
	return adapter.WithContext(ctx, &adapter.InboundContext{
		Source: M.SocksaddrFrom(netip.MustParseAddr("198.51.100.7"), 51000),
	})
}

func TestExchangeDNSGoesToServer(t *testing.T) {
	router := &stubDNSRouter{respond: answer}
	server := &stubDNSTransport{}
	inbound := newDNSTestInbound(router, server)
	raw, err := inbound.exchangeDNS(connectionContext(), packQuery(t, nil))
	require.NoError(t, err)
	response := unpackResponse(t, raw)
	require.Equal(t, uint16(4242), response.Id)
	require.Equal(t, mDNS.RcodeSuccess, response.Rcode)
	require.Len(t, response.Answer, 1)
	require.Same(t, server, router.options.Transport)
	require.Equal(t, "anytls-in", router.metadata.Inbound)
	require.Equal(t, C.TypeAnyTLS, router.metadata.InboundType)
	require.Equal(t, "alice", router.metadata.User)
	require.Equal(t, "198.51.100.7", router.metadata.Source.Addr.String())
	require.Equal(t, C.ProtocolDNS, router.metadata.Protocol)
}

func TestExchangeDNSWithoutServerUsesRules(t *testing.T) {
	router := &stubDNSRouter{respond: answer}
	inbound := newDNSTestInbound(router, nil)
	_, err := inbound.exchangeDNS(connectionContext(), packQuery(t, nil))
	require.NoError(t, err)
	require.Nil(t, router.options.Transport)
}

func TestExchangeDNSRejectsMalformedQueries(t *testing.T) {
	for name, edit := range map[string]func(*mDNS.Msg){
		"response": func(message *mDNS.Msg) { message.Response = true },
		"opcode":   func(message *mDNS.Msg) { message.Opcode = mDNS.OpcodeUpdate },
		"two_questions": func(message *mDNS.Msg) {
			message.Question = append(message.Question, mDNS.Question{Name: "example.net.", Qtype: mDNS.TypeA, Qclass: mDNS.ClassINET})
		},
		"no_question": func(message *mDNS.Msg) { message.Question = nil },
	} {
		t.Run(name, func(t *testing.T) {
			router := &stubDNSRouter{respond: answer}
			inbound := newDNSTestInbound(router, nil)
			raw, err := inbound.exchangeDNS(connectionContext(), packQuery(t, edit))
			require.NoError(t, err)
			response := unpackResponse(t, raw)
			require.Equal(t, uint16(4242), response.Id)
			require.Equal(t, mDNS.RcodeFormatError, response.Rcode)
			require.Zero(t, router.calls)
		})
	}
}

func TestExchangeDNSGarbageIsAnError(t *testing.T) {
	router := &stubDNSRouter{respond: answer}
	inbound := newDNSTestInbound(router, nil)
	_, err := inbound.exchangeDNS(connectionContext(), []byte{1, 2, 3})
	require.Error(t, err)
	require.Zero(t, router.calls)
}

func TestExchangeDNSRouterFailureIsServerFailure(t *testing.T) {
	router := &stubDNSRouter{respond: func(*mDNS.Msg) (*mDNS.Msg, error) {
		return nil, errors.New("upstream down")
	}}
	inbound := newDNSTestInbound(router, nil)
	raw, err := inbound.exchangeDNS(connectionContext(), packQuery(t, nil))
	require.NoError(t, err)
	response := unpackResponse(t, raw)
	require.Equal(t, uint16(4242), response.Id)
	require.Equal(t, mDNS.RcodeServerFailure, response.Rcode)
	require.Equal(t, "example.org.", response.Question[0].Name)
}

// An upstream timeout or a reset connection is a server failure for the
// client, not a reason to leave it without an answer.
func TestExchangeDNSUpstreamTimeoutIsServerFailure(t *testing.T) {
	for _, upstreamErr := range []error{context.DeadlineExceeded, context.Canceled, io.EOF} {
		router := &stubDNSRouter{respond: func(*mDNS.Msg) (*mDNS.Msg, error) {
			return nil, upstreamErr
		}}
		inbound := newDNSTestInbound(router, nil)
		raw, err := inbound.exchangeDNS(connectionContext(), packQuery(t, nil))
		require.NoError(t, err)
		require.Equal(t, mDNS.RcodeServerFailure, unpackResponse(t, raw).Rcode)
	}
}

func TestExchangeDNSClosedSessionIsAnError(t *testing.T) {
	ctx, cancel := context.WithCancel(connectionContext())
	router := &stubDNSRouter{respond: func(*mDNS.Msg) (*mDNS.Msg, error) {
		cancel()
		return nil, context.Canceled
	}}
	inbound := newDNSTestInbound(router, nil)
	_, err := inbound.exchangeDNS(ctx, packQuery(t, nil))
	require.ErrorIs(t, err, context.Canceled)
}

// Link-only EDNS options are removed so that the query can use the cache;
// the client subnet and the DNSSEC OK bit are kept.
func TestExchangeDNSStripsHopByHopOptions(t *testing.T) {
	var forwarded *mDNS.Msg
	router := &stubDNSRouter{respond: func(message *mDNS.Msg) (*mDNS.Msg, error) {
		forwarded = message
		return answer(message)
	}}
	inbound := newDNSTestInbound(router, nil)
	_, err := inbound.exchangeDNS(connectionContext(), packQuery(t, func(message *mDNS.Msg) {
		message.SetEdns0(1232, true)
		opt := message.IsEdns0()
		opt.Option = append(opt.Option,
			&mDNS.EDNS0_COOKIE{Code: mDNS.EDNS0COOKIE, Cookie: "0123456789abcdef"},
			&mDNS.EDNS0_SUBNET{Code: mDNS.EDNS0SUBNET, Family: 1, SourceNetmask: 24, Address: net.IPv4(198, 51, 100, 0)},
			&mDNS.EDNS0_PADDING{Padding: make([]byte, 32)},
		)
	}))
	require.NoError(t, err)
	opt := forwarded.IsEdns0()
	require.NotNil(t, opt)
	require.True(t, opt.Do())
	require.Len(t, opt.Option, 1)
	require.Equal(t, uint16(mDNS.EDNS0SUBNET), opt.Option[0].Option())
}

// An answer too large for a frame is truncated with TC set rather than lost.
func TestExchangeDNSTruncatesOversizeAnswer(t *testing.T) {
	router := &stubDNSRouter{respond: func(message *mDNS.Msg) (*mDNS.Msg, error) {
		response := new(mDNS.Msg)
		response.SetReply(message)
		for i := 0; i < 5000; i++ {
			response.Answer = append(response.Answer, &mDNS.TXT{
				Hdr: mDNS.RR_Header{Name: message.Question[0].Name, Rrtype: mDNS.TypeTXT, Class: mDNS.ClassINET, Ttl: 60},
				Txt: []string{"0123456789abcdef0123456789abcdef"},
			})
		}
		return response, nil
	}}
	inbound := newDNSTestInbound(router, nil)
	raw, err := inbound.exchangeDNS(connectionContext(), packQuery(t, nil))
	require.NoError(t, err)
	require.LessOrEqual(t, len(raw), maxDNSMessageSize)
	response := unpackResponse(t, raw)
	require.True(t, response.Truncated)
	require.NotEmpty(t, response.Answer)
}
