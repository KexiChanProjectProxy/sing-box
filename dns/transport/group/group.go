// Package group implements the "group" DNS server, which spreads queries over
// other DNS servers and falls back to more when they fail.
package group

import (
	"context"
	"errors"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/service"

	mDNS "github.com/miekg/dns"
)

const (
	ModeLoadBalance = "load_balance"
	ModeParallel    = "parallel"
	ModeSequential  = "sequential"
)

// memberTypes are the DNS server types a group may contain: plain DNS over UDP
// and TCP, DNS over TLS and DNS over HTTPS.
var memberTypes = []string{C.DNSTypeUDP, C.DNSTypeTCP, C.DNSTypeTLS, C.DNSTypeHTTPS}

// failurePenalty is the round-trip time recorded for a server that failed, so
// that load balancing avoids it until it answers again.
const failurePenalty = C.DNSTimeout

func RegisterTransport(registry *dns.TransportRegistry) {
	dns.RegisterTransport[option.GroupDNSServerOptions](registry, C.DNSTypeGroup, NewTransport)
}

var _ adapter.DNSTransport = (*Transport)(nil)

type Transport struct {
	dns.TransportAdapter
	logger       log.StructuredLogger
	manager      adapter.DNSTransportManager
	mode         string
	serverTags   []string
	fallbackTags []string
	servers      []adapter.DNSTransport
	fallbacks    []adapter.DNSTransport
	rttAccess    sync.Mutex
	rtt          map[string]time.Duration
	randomAccess sync.Mutex
	random       *rand.Rand
}

func NewTransport(ctx context.Context, logger log.StructuredLogger, tag string, options option.GroupDNSServerOptions) (adapter.DNSTransport, error) {
	if len(options.Servers) == 0 {
		return nil, E.New("missing servers")
	}
	mode := options.Mode
	switch mode {
	case "":
		mode = ModeLoadBalance
	case ModeLoadBalance, ModeParallel, ModeSequential:
	default:
		return nil, E.New("unknown mode: ", mode)
	}
	var dependencies []string
	for _, member := range append(slices.Clone(options.Servers), options.Fallbacks...) {
		if member == tag {
			return nil, E.New("server lists itself: ", member)
		}
		if !slices.Contains(dependencies, member) {
			dependencies = append(dependencies, member)
		}
	}
	return &Transport{
		TransportAdapter: dns.NewTransportAdapter(C.DNSTypeGroup, tag, dependencies),
		logger:           logger,
		manager:          service.FromContext[adapter.DNSTransportManager](ctx),
		mode:             mode,
		serverTags:       options.Servers,
		fallbackTags:     options.Fallbacks,
		rtt:              make(map[string]time.Duration),
		random:           rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())),
	}, nil
}

func (t *Transport) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	// Members are dependencies, so they have started by now.
	var err error
	t.servers, err = t.resolve(t.serverTags)
	if err != nil {
		return err
	}
	t.fallbacks, err = t.resolve(t.fallbackTags)
	return err
}

func (t *Transport) resolve(tags []string) ([]adapter.DNSTransport, error) {
	transports := make([]adapter.DNSTransport, 0, len(tags))
	for _, tag := range tags {
		transport, loaded := t.manager.Transport(tag)
		if !loaded {
			return nil, E.New("DNS server not found: ", tag)
		}
		if !slices.Contains(memberTypes, transport.Type()) {
			return nil, E.New("DNS server[", tag, "] type \"", transport.Type(), "\" is not supported in a group: only ", strings.Join(memberTypes, ", "))
		}
		transports = append(transports, transport)
	}
	return transports, nil
}

func (t *Transport) Close() error {
	return nil
}

// Reset forgets the measured round-trip times. The members are reset on their
// own, as every DNS server is.
func (t *Transport) Reset() {
	t.rttAccess.Lock()
	clear(t.rtt)
	t.rttAccess.Unlock()
}

func (t *Transport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	// The servers get their share of the time and leave the rest to the
	// fallbacks, so that servers which do not answer at all still leave time
	// to try them. Asking every server at once counts as one try.
	primaryTries := len(t.servers)
	if t.mode == ModeParallel {
		primaryTries = 1
	}
	primaryCtx, cancel := shareDeadline(ctx, primaryTries, primaryTries+len(t.fallbacks))
	var (
		response *mDNS.Msg
		err      error
	)
	switch t.mode {
	case ModeParallel:
		response, err = t.exchangeParallel(primaryCtx, message, t.servers)
	case ModeSequential:
		response, err = t.exchangeInOrder(primaryCtx, message, t.servers)
	default:
		response, err = t.exchangeInOrder(primaryCtx, message, t.weightedOrder(t.servers))
	}
	cancel()
	if accepted(response, err) || len(t.fallbacks) == 0 || ctx.Err() != nil {
		return response, err
	}
	t.logger.DebugContext(ctx, "all servers failed, trying fallbacks: ", err)
	fallbackResponse, fallbackErr := t.exchangeInOrder(ctx, message, t.fallbacks)
	if fallbackResponse == nil && fallbackErr != nil && response != nil {
		// A server failure answer is better than none.
		return response, nil
	}
	if fallbackErr != nil && err != nil {
		return nil, errors.Join(err, fallbackErr)
	}
	return fallbackResponse, fallbackErr
}

func (t *Transport) ExchangeAsync(ctx context.Context, message *mDNS.Msg, callback func(response *mDNS.Msg, err error)) {
	go func() {
		callback(t.Exchange(ctx, message))
	}()
}

// accepted reports whether an answer ends the search: anything but an error, a
// server failure or a refusal, which another server may well not share.
func accepted(response *mDNS.Msg, err error) bool {
	return err == nil && response != nil &&
		response.Rcode != mDNS.RcodeServerFailure && response.Rcode != mDNS.RcodeRefused
}

// exchangeInOrder tries servers one after another until one answers. Each try
// gets an equal share of the time left, so that one server that does not
// answer cannot use up the whole query.
func (t *Transport) exchangeInOrder(ctx context.Context, message *mDNS.Msg, servers []adapter.DNSTransport) (*mDNS.Msg, error) {
	var (
		lastResponse *mDNS.Msg
		errs         []error
	)
	for index, server := range servers {
		if ctx.Err() != nil {
			errs = append(errs, ctx.Err())
			break
		}
		tryCtx, cancel := shareDeadline(ctx, 1, len(servers)-index)
		response, err := t.exchangeOne(tryCtx, server, message)
		cancel()
		if accepted(response, err) {
			return response, nil
		}
		if err != nil {
			errs = append(errs, E.Cause(err, server.Tag()))
		} else {
			lastResponse = response
		}
	}
	if lastResponse != nil {
		return lastResponse, nil
	}
	return nil, errors.Join(errs...)
}

// shareDeadline gives tries out of totalTries their share of the time left
// before the deadline of ctx.
func shareDeadline(ctx context.Context, tries int, totalTries int) (context.Context, context.CancelFunc) {
	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline || tries >= totalTries {
		return context.WithCancel(ctx)
	}
	share := time.Until(deadline) * time.Duration(tries) / time.Duration(totalTries)
	return context.WithTimeout(ctx, share)
}

// exchangeParallel asks every server at once and takes the first answer.
func (t *Transport) exchangeParallel(ctx context.Context, message *mDNS.Msg, servers []adapter.DNSTransport) (*mDNS.Msg, error) {
	if len(servers) == 1 {
		return t.exchangeOne(ctx, servers[0], message)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	type result struct {
		server   adapter.DNSTransport
		response *mDNS.Msg
		err      error
	}
	results := make(chan result, len(servers))
	for _, server := range servers {
		go func() {
			response, err := t.exchangeOne(ctx, server, message)
			results <- result{server, response, err}
		}()
	}
	var (
		lastResponse *mDNS.Msg
		errs         []error
	)
	for range servers {
		r := <-results
		if accepted(r.response, r.err) {
			return r.response, nil
		}
		if r.err != nil {
			errs = append(errs, E.Cause(r.err, r.server.Tag()))
		} else {
			lastResponse = r.response
		}
	}
	if lastResponse != nil {
		return lastResponse, nil
	}
	return nil, errors.Join(errs...)
}

// exchangeOne sends a copy of message, since servers may change it, and
// records how long the server took.
func (t *Transport) exchangeOne(ctx context.Context, server adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
	start := time.Now()
	response, err := server.Exchange(ctx, message.Copy())
	elapsed := time.Since(start)
	switch {
	case accepted(response, err):
		t.recordRTT(server.Tag(), elapsed)
	case ctx.Err() != nil:
		// A try cancelled because another server answered says nothing about
		// this one. One that ran out of its share of the time took at least
		// that long, which is all that is known.
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.recordRTT(server.Tag(), elapsed)
		}
	default:
		// An error, a server failure or a refusal, however quick, must not
		// make the server look fast.
		t.recordRTT(server.Tag(), failurePenalty)
	}
	return response, err
}

// recordRTT updates a server's smoothed round-trip time.
func (t *Transport) recordRTT(tag string, sample time.Duration) {
	t.rttAccess.Lock()
	defer t.rttAccess.Unlock()
	current, loaded := t.rtt[tag]
	if !loaded {
		t.rtt[tag] = sample
		return
	}
	t.rtt[tag] = current + (sample-current)/4
}

// weightedOrder returns servers in a random order where each pick favours the
// faster servers, weighting each by the inverse of its round-trip time. A
// server not measured yet gets the weight of a 1 ms one, so that it is tried
// early and measured.
func (t *Transport) weightedOrder(servers []adapter.DNSTransport) []adapter.DNSTransport {
	if len(servers) <= 1 {
		return servers
	}
	weights := make([]float64, len(servers))
	t.rttAccess.Lock()
	for index, server := range servers {
		rtt, loaded := t.rtt[server.Tag()]
		if !loaded || rtt < time.Millisecond {
			rtt = time.Millisecond
		}
		weights[index] = 1 / rtt.Seconds()
	}
	t.rttAccess.Unlock()

	remaining := slices.Clone(servers)
	ordered := make([]adapter.DNSTransport, 0, len(servers))
	t.randomAccess.Lock()
	defer t.randomAccess.Unlock()
	for len(remaining) > 0 {
		var total float64
		for _, weight := range weights {
			total += weight
		}
		pick := t.random.Float64() * total
		index := 0
		for ; index < len(weights)-1; index++ {
			pick -= weights[index]
			if pick < 0 {
				break
			}
		}
		ordered = append(ordered, remaining[index])
		remaining = slices.Delete(remaining, index, index+1)
		weights = slices.Delete(weights, index, index+1)
	}
	return ordered
}
