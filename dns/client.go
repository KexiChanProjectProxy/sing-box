package dns

import (
	"context"
	"encoding/binary"
	"errors"
	"hash/fnv"
	"net"
	"net/netip"
	"strconv"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/compatible"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/task"
	"github.com/sagernet/sing/contrab/freelru"
	"github.com/sagernet/sing/contrab/maphash"
	"github.com/sagernet/sing/service"

	"github.com/miekg/dns"
)

var (
	ErrNoRawSupport           = E.New("no raw query support by current transport")
	ErrNotCached              = E.New("not cached")
	ErrResponseRejected       = E.New("response rejected")
	ErrResponseRejectedCached = E.Extend(ErrResponseRejected, "cached")
)

var _ adapter.DNSClient = (*Client)(nil)

// DefaultOptimisticTimeout is how long an expired entry may be served when optimistic
// caching is enabled without a timeout.
const DefaultOptimisticTimeout = 3 * 24 * time.Hour

// DefaultCacheCapacity is the LRU capacity used when cache_capacity is not set.
const DefaultCacheCapacity = 1024

type Client struct {
	ctx                     context.Context
	timeout                 time.Duration
	disableCache            bool
	disableExpire           bool
	minTTL                  uint32
	maxTTL                  uint32
	optimisticTimeout       time.Duration
	optimisticClientTimeout time.Duration
	optimisticRetention     time.Duration
	prefetch                bool
	cacheCapacity           uint32
	clientSubnet            netip.Prefix
	rdrc                    adapter.RDRCStore
	initRDRCFunc            func() adapter.RDRCStore
	dnsCache                adapter.DNSCacheStore
	initDNSCacheFunc        func() adapter.DNSCacheStore
	networkManager          adapter.NetworkManager
	logger                  log.StructuredLogger
	cache                   *freelru.Cache[dnsCacheKey, *dns.Msg]
	cacheLock               compatible.Map[dnsExchangeKey, chan struct{}]
	backgroundRefresh       compatible.Map[dnsCacheKey, *refreshTask]
}

type ClientOptions struct {
	Context       context.Context
	Timeout       time.Duration
	DisableCache  bool
	DisableExpire bool
	MinTTL        uint32
	MaxTTL        uint32
	// OptimisticTimeout enables optimistic caching for every query when non-zero.
	OptimisticTimeout time.Duration
	// OptimisticClientTimeout is how long a query that hits an expired entry waits
	// for the refresh before the expired entry is returned.
	OptimisticClientTimeout time.Duration
	// OptimisticRetention keeps expired entries for at least this long, so that
	// queries whose rule enables optimistic caching can still serve them.
	OptimisticRetention time.Duration
	Prefetch            bool
	CacheCapacity       uint32
	ClientSubnet        netip.Prefix
	RDRC                func() adapter.RDRCStore
	DNSCache            func() adapter.DNSCacheStore
	Logger              log.StructuredLogger
}

func NewClient(options ClientOptions) *Client {
	cacheCapacity := options.CacheCapacity
	if cacheCapacity == 0 {
		cacheCapacity = DefaultCacheCapacity
	}
	client := &Client{
		ctx:                     options.Context,
		timeout:                 options.Timeout,
		disableCache:            options.DisableCache,
		disableExpire:           options.DisableExpire,
		minTTL:                  options.MinTTL,
		maxTTL:                  options.MaxTTL,
		optimisticTimeout:       options.OptimisticTimeout,
		optimisticClientTimeout: options.OptimisticClientTimeout,
		optimisticRetention:     max(options.OptimisticRetention, options.OptimisticTimeout),
		prefetch:                options.Prefetch,
		cacheCapacity:           cacheCapacity,
		clientSubnet:            options.ClientSubnet,
		initRDRCFunc:            options.RDRC,
		initDNSCacheFunc:        options.DNSCache,
		logger:                  options.Logger,
	}
	if client.timeout == 0 {
		client.timeout = C.DNSTimeout
	}
	if !client.disableCache && client.initDNSCacheFunc == nil {
		client.initializeMemoryCache()
	}
	return client
}

type dnsCacheKey struct {
	dns.Question
	transportTag string
	clientSubnet netip.Prefix
	environment  uint64
}

type dnsExchangeKey struct {
	dnsCacheKey
	timeout time.Duration
}

func (k dnsCacheKey) persistentName() string {
	name := k.transportTag
	if k.clientSubnet.IsValid() {
		name += "\x00" + k.clientSubnet.String()
	}
	if k.environment != 0 {
		name += "\x01" + strconv.FormatUint(k.environment, 36)
	}
	return name
}

func (c *Client) newCacheKey(transport adapter.DNSTransport, question dns.Question, message *dns.Msg, options adapter.DNSQueryOptions) dnsCacheKey {
	var clientSubnet netip.Prefix
	if !options.RemoveClientSubnet {
		clientSubnet = options.ClientSubnet
		if !clientSubnet.IsValid() {
			clientSubnet = c.clientSubnet
		}
		if !clientSubnet.IsValid() {
			clientSubnet = clientSubnetFromMessage(message)
		}
	}
	return dnsCacheKey{
		Question:     question,
		transportTag: transport.Tag(),
		clientSubnet: clientSubnet,
		environment:  c.environmentHash(transport),
	}
}

func (c *Client) finishCacheKey(transport adapter.DNSTransport, key dnsCacheKey) (dnsCacheKey, bool) {
	environment := c.environmentHash(transport)
	if environment == key.environment || key.environment == 0 {
		key.environment = environment
		return key, true
	}
	return key, false
}

func (c *Client) environmentHash(transport adapter.DNSTransport) uint64 {
	environmentTransport, withEnvironment := transport.(adapter.DNSTransportWithEnvironment)
	if !withEnvironment {
		return 0
	}
	var networkEnvironment uint64
	if c.networkManager != nil {
		networkEnvironment = c.networkManager.NetworkEnvironment()
	}
	environment := environmentTransport.Environment()
	if len(environment) == 0 {
		return networkEnvironment
	}
	digest := fnv.New64a()
	for _, entry := range environment {
		digest.Write([]byte(entry))
		digest.Write([]byte{0})
	}
	var hashBytes [8]byte
	binary.BigEndian.PutUint64(hashBytes[:], networkEnvironment)
	digest.Write(hashBytes[:])
	return digest.Sum64()
}

// SetOptimisticRetention keeps expired entries for at least the given duration.
// It must be called before Start.
func (c *Client) SetOptimisticRetention(retention time.Duration) {
	c.optimisticRetention = max(retention, c.optimisticTimeout)
}

// effectiveOptimistic returns the optimistic window and client timeout for a query:
// the rule's override if present, otherwise the global setting.
func (c *Client) effectiveOptimistic(options adapter.DNSQueryOptions) (time.Duration, time.Duration) {
	if options.DisableOptimisticCache || c.disableExpire {
		return 0, 0
	}
	override := options.Optimistic
	if override == nil {
		return c.optimisticTimeout, c.optimisticClientTimeout
	}
	if !override.Enabled {
		return 0, 0
	}
	window := override.Timeout
	if window == 0 {
		window = c.optimisticTimeout
	}
	if window == 0 {
		window = DefaultOptimisticTimeout
	}
	clientTimeout := override.ClientTimeout
	if clientTimeout == 0 {
		clientTimeout = c.optimisticClientTimeout
	}
	return window, clientTimeout
}

func (c *Client) effectivePrefetch(options adapter.DNSQueryOptions) bool {
	if c.disableExpire {
		return false
	}
	if options.Prefetch != nil {
		return *options.Prefetch
	}
	return c.prefetch
}

// effectiveTTLBounds returns the TTL bounds for a query. A bound set by the rule
// overrides the global one, and a global bound that conflicts with a rule bound
// is ignored.
func (c *Client) effectiveTTLBounds(options adapter.DNSQueryOptions) (uint32, uint32) {
	minTTL, maxTTL := options.MinTTL, options.MaxTTL
	if minTTL == 0 {
		minTTL = c.minTTL
		if maxTTL > 0 && minTTL > maxTTL {
			minTTL = 0
		}
	}
	if maxTTL == 0 {
		maxTTL = c.maxTTL
		if options.MinTTL > 0 && maxTTL > 0 && maxTTL < minTTL {
			maxTTL = 0
		}
	}
	return minTTL, maxTTL
}

func (c *Client) Start() {
	c.networkManager = service.FromContext[adapter.NetworkManager](c.ctx)
	if c.initRDRCFunc != nil {
		c.rdrc = c.initRDRCFunc()
	}
	if c.initDNSCacheFunc != nil {
		c.dnsCache = c.initDNSCacheFunc()
	}
	if c.dnsCache == nil {
		c.initializeMemoryCache()
	}
}

func (c *Client) initializeMemoryCache() {
	if c.disableCache || c.cache != nil {
		return
	}
	c.cache = common.Must1(freelru.New[dnsCacheKey, *dns.Msg](c.cacheCapacity, maphash.NewHasher[dnsCacheKey]().Hash32, true))
}

func extractNegativeTTL(response *dns.Msg) (uint32, bool) {
	for _, record := range response.Ns {
		if soa, isSOA := record.(*dns.SOA); isSOA {
			soaTTL := soa.Header().Ttl
			soaMinimum := soa.Minttl
			if soaTTL < soaMinimum {
				return soaTTL, true
			}
			return soaMinimum, true
		}
	}
	return 0, false
}

func computeTimeToLive(response *dns.Msg) uint32 {
	var timeToLive uint32
	if len(response.Answer) == 0 {
		if soaTTL, hasSOA := extractNegativeTTL(response); hasSOA {
			return soaTTL
		}
	}
	for _, recordList := range [][]dns.RR{response.Answer, response.Ns, response.Extra} {
		for _, record := range recordList {
			if record.Header().Rrtype == dns.TypeOPT {
				continue
			}
			if timeToLive == 0 || record.Header().Ttl > 0 && record.Header().Ttl < timeToLive {
				timeToLive = record.Header().Ttl
			}
		}
	}
	return timeToLive
}

func normalizeTTL(response *dns.Msg, timeToLive uint32) {
	for _, recordList := range [][]dns.RR{response.Answer, response.Ns, response.Extra} {
		for _, record := range recordList {
			if record.Header().Rrtype == dns.TypeOPT {
				continue
			}
			record.Header().Ttl = timeToLive
		}
	}
}

type exchangeStatus int

const (
	exchangeReady exchangeStatus = iota
	exchangeDone
	exchangeWait
)

type exchangeOperation struct {
	ctx             context.Context
	message         *dns.Msg
	question        dns.Question
	messageId       uint16
	options         adapter.DNSQueryOptions
	responseChecker func(response *dns.Msg) bool
	disableCache    bool
	cacheKey        dnsCacheKey
	releaseCond     func()
}

func (o *exchangeOperation) release() {
	if o.releaseCond != nil {
		o.releaseCond()
		o.releaseCond = nil
	}
}

func (c *Client) beginExchange(ctx context.Context, transport adapter.DNSTransport, message *dns.Msg, options adapter.DNSQueryOptions, responseChecker func(response *dns.Msg) bool, allowWait bool) (*exchangeOperation, *dns.Msg, exchangeStatus, error) {
	err := ctx.Err()
	if err != nil {
		return nil, nil, exchangeDone, err
	}
	if len(message.Question) == 0 {
		if c.logger != nil {
			c.logger.WarnEventContext(ctx, "dns.query.invalid", "bad question size", log.Int("question_size", len(message.Question)))
		}
		return nil, FixedResponseStatus(message, dns.RcodeFormatError), exchangeDone, nil
	}
	question := message.Question[0]
	if question.Qtype == dns.TypeA && options.Strategy == C.DomainStrategyIPv6Only || question.Qtype == dns.TypeAAAA && options.Strategy == C.DomainStrategyIPv4Only {
		if c.logger != nil {
			c.logger.DebugEventContext(ctx, "dns.strategy.rejected", "strategy rejected")
		}
		return nil, FixedResponseStatus(message, dns.RcodeSuccess), exchangeDone, nil
	}
	isSimpleRequest := len(message.Question) == 1 &&
		len(message.Ns) == 0 &&
		(len(message.Extra) == 0 || len(message.Extra) == 1 &&
			message.Extra[0].Header().Rrtype == dns.TypeOPT &&
			message.Extra[0].Header().Class > 0 &&
			message.Extra[0].Header().Ttl == 0 &&
			common.All(message.Extra[0].(*dns.OPT).Option, func(it dns.EDNS0) bool {
				return it.Option() == dns.EDNS0SUBNET
			}))
	message = c.prepareExchangeMessage(message, options)
	disableCache := !isSimpleRequest || c.disableCache || options.DisableCache
	operation := &exchangeOperation{
		message:         message,
		question:        question,
		messageId:       message.Id,
		options:         options,
		responseChecker: responseChecker,
		disableCache:    disableCache,
	}
	if !disableCache {
		cacheKey := c.newCacheKey(transport, question, message, options)
		operation.cacheKey = cacheKey
		exchangeKey := dnsExchangeKey{dnsCacheKey: cacheKey, timeout: options.Timeout}
		cond, loaded := c.cacheLock.LoadOrStore(exchangeKey, make(chan struct{}))
		if !loaded {
			operation.releaseCond = func() {
				c.cacheLock.Delete(exchangeKey)
				close(cond)
			}
		}
		optimisticWindow, optimisticClientTimeout := c.effectiveOptimistic(options)
		for {
			hit, cached := c.loadResponse(cacheKey, optimisticWindow)
			if cached {
				response := hit.response
				if hit.stale {
					if optimisticClientTimeout > 0 && !allowWait {
						operation.release()
						return nil, nil, exchangeWait, nil
					}
					task := c.backgroundRefreshDNS(transport, cacheKey, message.Copy(), options, responseChecker)
					operation.release()
					if optimisticClientTimeout > 0 {
						response = c.waitRefresh(ctx, task, response, optimisticClientTimeout)
					} else {
						logOptimisticResponse(c.logger, ctx, response)
					}
				} else {
					if hit.prefetch && c.effectivePrefetch(options) {
						c.backgroundRefreshDNS(transport, cacheKey, message.Copy(), options, responseChecker)
					}
					logCachedResponse(c.logger, ctx, response, hit.ttl)
					operation.release()
				}
				response.Id = message.Id
				return nil, response, exchangeDone, nil
			}
			if !loaded {
				break
			}
			if !allowWait {
				return nil, nil, exchangeWait, nil
			}
			select {
			case <-cond:
			case <-ctx.Done():
				return nil, nil, exchangeDone, ctx.Err()
			}
			err = ctx.Err()
			if err != nil {
				return nil, nil, exchangeDone, err
			}
			cacheKey = c.newCacheKey(transport, question, message, options)
			operation.cacheKey = cacheKey
			loaded = false
		}
	}

	contextTransport, transportTagLoaded := adapter.DNSTransportTagFromContext(ctx)
	if transportTagLoaded && transport.Tag() == contextTransport {
		operation.release()
		return nil, nil, exchangeDone, E.New("DNS query loopback in transport[", contextTransport, "]")
	}
	operation.ctx = adapter.ContextWithDNSTransportTag(ctx, transport.Tag())
	if !disableCache && responseChecker != nil && c.rdrc != nil {
		rejected := c.rdrc.LoadRDRC(transport.Tag(), question.Name, question.Qtype)
		if rejected {
			operation.release()
			return nil, nil, exchangeDone, ErrResponseRejectedCached
		}
	}
	return operation, nil, exchangeReady, nil
}

func (c *Client) finishExchange(transport adapter.DNSTransport, operation *exchangeOperation, response *dns.Msg) (*dns.Msg, error) {
	ctx := operation.ctx
	question := operation.question
	disableCache := operation.disableCache || (response.Rcode != dns.RcodeSuccess && response.Rcode != dns.RcodeNameError)
	if operation.responseChecker != nil {
		var rejected bool
		if response.Rcode != dns.RcodeSuccess && response.Rcode != dns.RcodeNameError {
			rejected = true
		} else {
			rejected = !operation.responseChecker(response)
		}
		if rejected {
			if !disableCache && c.rdrc != nil {
				c.rdrc.SaveRDRCAsync(transport.Tag(), question.Name, question.Qtype, c.logger)
			}
			logRejectedResponse(c.logger, ctx, response)
			return response, ErrResponseRejected
		}
	}
	timeToLive := c.applyResponseOptions(question, response, operation.options)
	if !disableCache {
		cacheKey, storable := c.finishCacheKey(transport, operation.cacheKey)
		if storable {
			c.storeCache(cacheKey, response, timeToLive)
		}
	}
	response.Id = operation.messageId
	requestEDNSOpt := operation.message.IsEdns0()
	responseEDNSOpt := response.IsEdns0()
	if responseEDNSOpt != nil && (requestEDNSOpt == nil || requestEDNSOpt.Version() < responseEDNSOpt.Version()) {
		response.Extra = common.Filter(response.Extra, func(it dns.RR) bool {
			return it.Header().Rrtype != dns.TypeOPT
		})
		if requestEDNSOpt != nil {
			response.SetEdns0(responseEDNSOpt.UDPSize(), responseEDNSOpt.Do())
		}
	}
	logExchangedResponse(c.logger, ctx, response, timeToLive)
	return response, nil
}

func (c *Client) Exchange(ctx context.Context, transport adapter.DNSTransport, message *dns.Msg, options adapter.DNSQueryOptions, responseChecker func(response *dns.Msg) bool) (*dns.Msg, error) {
	if options.Timeout == 0 {
		options.Timeout = c.timeout
	}
	ctx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()
	operation, earlyResponse, status, err := c.beginExchange(ctx, transport, message, options, responseChecker, true)
	if status != exchangeReady {
		return earlyResponse, err
	}
	defer operation.release()
	response, err := c.exchangeToTransport(operation.ctx, transport, operation.message)
	if err != nil {
		return nil, err
	}
	return c.finishExchange(transport, operation, response)
}

func (c *Client) ExchangeAsync(ctx context.Context, transport adapter.DNSTransport, message *dns.Msg, options adapter.DNSQueryOptions, responseChecker func(response *dns.Msg) bool, callback func(response *dns.Msg, err error)) {
	if options.Timeout == 0 {
		options.Timeout = c.timeout
	}
	ctx, cancel := context.WithTimeout(ctx, options.Timeout)
	operation, earlyResponse, status, err := c.beginExchange(ctx, transport, message, options, responseChecker, false)
	switch status {
	case exchangeDone:
		cancel()
		callback(earlyResponse, err)
		return
	case exchangeWait:
		go func() {
			response, exchangeErr := c.Exchange(ctx, transport, message, options, responseChecker)
			cancel()
			callback(response, exchangeErr)
		}()
		return
	}
	finish := func(response *dns.Msg, exchangeErr error) {
		cancel()
		if exchangeErr != nil {
			operation.release()
			callback(nil, exchangeErr)
			return
		}
		finishedResponse, finishErr := c.finishExchange(transport, operation, response)
		operation.release()
		callback(finishedResponse, finishErr)
	}
	c.exchangeToTransportAsync(operation.ctx, transport, operation.message, finish)
}

func (c *Client) Lookup(ctx context.Context, transport adapter.DNSTransport, domain string, options adapter.DNSQueryOptions, responseChecker func(response *dns.Msg) bool) ([]netip.Addr, error) {
	domain = FqdnToDomain(domain)
	dnsName := dns.Fqdn(domain)
	var strategy C.DomainStrategy
	if options.LookupStrategy != C.DomainStrategyAsIS {
		strategy = options.LookupStrategy
	} else {
		strategy = options.Strategy
	}
	lookupOptions := options
	if options.LookupStrategy != C.DomainStrategyAsIS {
		lookupOptions.Strategy = strategy
	}
	switch strategy {
	case C.DomainStrategyIPv4Only:
		return c.lookupToExchange(ctx, transport, dnsName, dns.TypeA, lookupOptions, responseChecker)
	case C.DomainStrategyIPv6Only:
		return c.lookupToExchange(ctx, transport, dnsName, dns.TypeAAAA, lookupOptions, responseChecker)
	}
	var response4 []netip.Addr
	var response6 []netip.Addr
	var group task.Group
	group.Append("exchange4", func(ctx context.Context) error {
		response, err := c.lookupToExchange(ctx, transport, dnsName, dns.TypeA, lookupOptions, responseChecker)
		if err != nil {
			return err
		}
		response4 = response
		return nil
	})
	group.Append("exchange6", func(ctx context.Context) error {
		response, err := c.lookupToExchange(ctx, transport, dnsName, dns.TypeAAAA, lookupOptions, responseChecker)
		if err != nil {
			return err
		}
		response6 = response
		return nil
	})
	err := group.Run(ctx)
	if len(response4) == 0 && len(response6) == 0 {
		return nil, err
	}
	return sortAddresses(response4, response6, strategy), nil
}

func (c *Client) ClearCache() {
	if c.cache != nil {
		c.cache.Purge()
	}
	if c.dnsCache != nil {
		err := c.dnsCache.ClearDNSCache()
		if err != nil && c.logger != nil {
			c.logger.WarnEvent("dns.cache.clear.error", "clear DNS cache", log.Err(err))
		}
	}
}

func sortAddresses(response4 []netip.Addr, response6 []netip.Addr, strategy C.DomainStrategy) []netip.Addr {
	if strategy == C.DomainStrategyPreferIPv6 {
		return append(response6, response4...)
	} else {
		return append(response4, response6...)
	}
}

func (c *Client) storeCache(key dnsCacheKey, message *dns.Msg, timeToLive uint32) {
	if timeToLive == 0 {
		return
	}
	if c.dnsCache != nil {
		packed, err := message.Pack()
		if err == nil {
			expireAt := time.Now().Add(time.Second * time.Duration(timeToLive))
			c.dnsCache.SaveDNSCacheAsync(key.persistentName(), key.Name, key.Qtype, packed, expireAt, c.logger)
		}
		return
	}
	if c.cache == nil {
		return
	}
	if c.disableExpire {
		c.cache.Add(key, message.Copy())
	} else {
		c.cache.AddWithLifetime(key, message.Copy(), time.Second*time.Duration(timeToLive))
	}
}

func (c *Client) lookupToExchange(ctx context.Context, transport adapter.DNSTransport, name string, qType uint16, options adapter.DNSQueryOptions, responseChecker func(response *dns.Msg) bool) ([]netip.Addr, error) {
	question := dns.Question{
		Name:   name,
		Qtype:  qType,
		Qclass: dns.ClassINET,
	}
	message := dns.Msg{
		MsgHdr: dns.MsgHdr{
			RecursionDesired: true,
		},
		Question: []dns.Question{question},
	}
	disableCache := c.disableCache || options.DisableCache
	if !disableCache {
		cachedAddresses, err := c.questionCache(ctx, transport, &message, options, responseChecker)
		if err != ErrNotCached {
			return cachedAddresses, err
		}
	}
	response, err := c.Exchange(ctx, transport, &message, options, responseChecker)
	if err != nil {
		return nil, err
	}
	if response.Rcode != dns.RcodeSuccess {
		return nil, RcodeError(response.Rcode)
	}
	return MessageToAddresses(response), nil
}

func (c *Client) questionCache(ctx context.Context, transport adapter.DNSTransport, message *dns.Msg, options adapter.DNSQueryOptions, responseChecker func(response *dns.Msg) bool) ([]netip.Addr, error) {
	question := message.Question[0]
	cacheKey := c.newCacheKey(transport, question, message, options)
	optimisticWindow, optimisticClientTimeout := c.effectiveOptimistic(options)
	hit, cached := c.loadResponse(cacheKey, optimisticWindow)
	if !cached {
		return nil, ErrNotCached
	}
	response := hit.response
	if hit.stale {
		task := c.backgroundRefreshDNS(transport, cacheKey, c.prepareExchangeMessage(message.Copy(), options), options, responseChecker)
		if optimisticClientTimeout > 0 {
			response = c.waitRefresh(ctx, task, response, optimisticClientTimeout)
		} else {
			logOptimisticResponse(c.logger, ctx, response)
		}
	} else if hit.prefetch && c.effectivePrefetch(options) {
		c.backgroundRefreshDNS(transport, cacheKey, c.prepareExchangeMessage(message.Copy(), options), options, responseChecker)
	}
	if response.Rcode != dns.RcodeSuccess {
		return nil, RcodeError(response.Rcode)
	}
	return MessageToAddresses(response), nil
}

type cacheHit struct {
	response *dns.Msg
	// ttl is the remaining TTL of a fresh entry in seconds.
	ttl int
	// stale is set when the entry has expired and is served optimistically.
	stale bool
	// prefetch is set when less than a tenth of the entry's TTL is left.
	prefetch bool
}

// loadResponse looks up a cached response. An expired entry is returned as stale
// while it is within the optimistic window, and dropped once it is past the
// retention every rule may need.
func (c *Client) loadResponse(key dnsCacheKey, optimisticWindow time.Duration) (cacheHit, bool) {
	if c.dnsCache != nil {
		return c.loadPersistentResponse(key, optimisticWindow)
	}
	if c.cache == nil {
		return cacheHit{}, false
	}
	if c.disableExpire {
		response, loaded := c.cache.Get(key)
		if !loaded {
			return cacheHit{}, false
		}
		return cacheHit{response: response.Copy()}, true
	}
	response, expireAt, loaded := c.cache.GetWithLifetimeNoExpire(key)
	if !loaded {
		return cacheHit{}, false
	}
	hit, cached, drop := c.checkExpiration(response.Copy(), expireAt, optimisticWindow)
	if drop {
		c.cache.Remove(key)
	}
	return hit, cached
}

func (c *Client) loadPersistentResponse(key dnsCacheKey, optimisticWindow time.Duration) (cacheHit, bool) {
	rawMessage, expireAt, loaded := c.dnsCache.LoadDNSCache(key.persistentName(), key.Name, key.Qtype)
	if !loaded {
		return cacheHit{}, false
	}
	response := new(dns.Msg)
	err := response.Unpack(rawMessage)
	if err != nil {
		return cacheHit{}, false
	}
	if c.disableExpire {
		return cacheHit{response: response}, true
	}
	hit, cached, _ := c.checkExpiration(response, expireAt, optimisticWindow)
	return hit, cached
}

// checkExpiration rewrites the TTLs of response, which must be a private copy of the
// cached message, for the time left until expireAt.
func (c *Client) checkExpiration(response *dns.Msg, expireAt time.Time, optimisticWindow time.Duration) (hit cacheHit, cached bool, drop bool) {
	timeNow := time.Now()
	if timeNow.After(expireAt) {
		if optimisticWindow > 0 && timeNow.Before(expireAt.Add(optimisticWindow)) {
			normalizeTTL(response, 1)
			return cacheHit{response: response, stale: true}, true, false
		}
		return cacheHit{}, false, !timeNow.Before(expireAt.Add(c.optimisticRetention))
	}
	remaining := expireAt.Sub(timeNow)
	hit = cacheHit{
		response: response,
		ttl:      max(int(remaining.Seconds()), 0),
	}
	if originalTTL, hasTTL := messageTTL(response); hasTTL && remaining*10 < time.Duration(originalTTL)*time.Second {
		hit.prefetch = true
	}
	normalizeTTL(response, uint32(hit.ttl))
	return hit, true, false
}

// messageTTL returns the TTL of the first record, which is the TTL the response was
// cached with since normalizeTTL gives every record the same value.
func messageTTL(response *dns.Msg) (uint32, bool) {
	for _, recordList := range [][]dns.RR{response.Answer, response.Ns, response.Extra} {
		for _, record := range recordList {
			if record.Header().Rrtype == dns.TypeOPT {
				continue
			}
			return record.Header().Ttl, true
		}
	}
	return 0, false
}

// waitRefresh waits up to clientTimeout for a refresh of an expired entry and
// returns the refreshed response, or the stale one if the refresh is late or fails.
func (c *Client) waitRefresh(ctx context.Context, task *refreshTask, stale *dns.Msg, clientTimeout time.Duration) *dns.Msg {
	timer := time.NewTimer(clientTimeout)
	defer timer.Stop()
	select {
	case <-task.done:
		if task.err == nil {
			return task.response.Copy()
		}
	case <-timer.C:
	case <-ctx.Done():
	}
	logOptimisticResponse(c.logger, ctx, stale)
	return stale
}

func (c *Client) applyResponseOptions(question dns.Question, response *dns.Msg, options adapter.DNSQueryOptions) uint32 {
	if question.Qtype == dns.TypeHTTPS && (options.Strategy == C.DomainStrategyIPv4Only || options.Strategy == C.DomainStrategyIPv6Only) {
		for _, rr := range response.Answer {
			https, isHTTPS := rr.(*dns.HTTPS)
			if !isHTTPS {
				continue
			}
			content := https.SVCB
			content.Value = common.Filter(content.Value, func(it dns.SVCBKeyValue) bool {
				if options.Strategy == C.DomainStrategyIPv4Only {
					return it.Key() != dns.SVCB_IPV6HINT
				}
				return it.Key() != dns.SVCB_IPV4HINT
			})
			https.SVCB = content
		}
	}
	timeToLive := computeTimeToLive(response)
	if options.RewriteTTL != nil {
		timeToLive = *options.RewriteTTL
	} else if timeToLive > 0 {
		minTTL, maxTTL := c.effectiveTTLBounds(options)
		if minTTL > 0 && timeToLive < minTTL {
			timeToLive = minTTL
		}
		if maxTTL > 0 && timeToLive > maxTTL {
			timeToLive = maxTTL
		}
	}
	normalizeTTL(response, timeToLive)
	return timeToLive
}

type refreshTask struct {
	done     chan struct{}
	response *dns.Msg
	err      error
}

// backgroundRefreshDNS refreshes a cached entry in the background. Concurrent calls
// for the same key share one refresh, whose result is published when done is closed.
func (c *Client) backgroundRefreshDNS(transport adapter.DNSTransport, key dnsCacheKey, message *dns.Msg, options adapter.DNSQueryOptions, responseChecker func(response *dns.Msg) bool) *refreshTask {
	task := &refreshTask{done: make(chan struct{})}
	existing, loaded := c.backgroundRefresh.LoadOrStore(key, task)
	if loaded {
		return existing
	}
	go func() {
		defer func() {
			c.backgroundRefresh.Delete(key)
			close(task.done)
		}()
		timeout := options.Timeout
		if timeout == 0 {
			timeout = c.timeout
		}
		ctx := adapter.ContextWithDNSTransportTag(c.ctx, transport.Tag())
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		response, err := c.exchangeToTransport(ctx, transport, message)
		if err != nil {
			if c.logger != nil {
				c.logger.DebugEventContext(ctx, "dns.cache.refresh.error", "cache refresh failed", log.String("domain", FqdnToDomain(key.Name)), log.Err(err))
			}
			task.err = err
			return
		}
		if responseChecker != nil {
			var rejected bool
			if response.Rcode != dns.RcodeSuccess && response.Rcode != dns.RcodeNameError {
				rejected = true
			} else {
				rejected = !responseChecker(response)
			}
			if rejected {
				if c.logger != nil {
					c.logger.DebugEventContext(ctx, "dns.cache.refresh.rejected", "cache refresh rejected", log.String("domain", FqdnToDomain(key.Name)))
				}
				if c.rdrc != nil {
					c.rdrc.SaveRDRCAsync(transport.Tag(), key.Name, key.Qtype, c.logger)
				}
				task.err = ErrResponseRejected
				return
			}
		} else if response.Rcode != dns.RcodeSuccess && response.Rcode != dns.RcodeNameError {
			task.err = RcodeError(response.Rcode)
			return
		}
		storeKey, storable := c.finishCacheKey(transport, key)
		if !storable {
			task.err = E.New("network environment changed")
			return
		}
		timeToLive := c.applyResponseOptions(key.Question, response, options)
		c.storeCache(storeKey, response, timeToLive)
		logRefreshedResponse(c.logger, ctx, response, timeToLive)
		task.response = response
	}()
	return task
}

func (c *Client) prepareExchangeMessage(message *dns.Msg, options adapter.DNSQueryOptions) *dns.Msg {
	if options.RemoveClientSubnet {
		return removeClientSubnet(message)
	}
	clientSubnet := options.ClientSubnet
	if !clientSubnet.IsValid() {
		clientSubnet = c.clientSubnet
	}
	if clientSubnet.IsValid() {
		message = SetClientSubnet(message, clientSubnet)
	}
	return message
}

func stripDNSPadding(response *dns.Msg) {
	for _, record := range response.Extra {
		opt, isOpt := record.(*dns.OPT)
		if !isOpt {
			continue
		}
		opt.Option = common.Filter(opt.Option, func(it dns.EDNS0) bool {
			return it.Option() != dns.EDNS0PADDING
		})
	}
}

func (c *Client) exchangeToTransport(ctx context.Context, transport adapter.DNSTransport, message *dns.Msg) (*dns.Msg, error) {
	response, err := transport.Exchange(ctx, message)
	if err == nil {
		stripDNSPadding(response)
		return response, nil
	}
	var rcodeError RcodeError
	if errors.As(err, &rcodeError) {
		return FixedResponseStatus(message, int(rcodeError)), nil
	}
	return nil, err
}

func (c *Client) exchangeToTransportAsync(ctx context.Context, transport adapter.DNSTransport, message *dns.Msg, callback func(response *dns.Msg, err error)) {
	transport.ExchangeAsync(ctx, message, func(response *dns.Msg, err error) {
		if err == nil {
			stripDNSPadding(response)
			callback(response, nil)
			return
		}
		var rcodeError RcodeError
		if errors.As(err, &rcodeError) {
			callback(FixedResponseStatus(message, int(rcodeError)), nil)
			return
		}
		callback(nil, err)
	})
}

func MessageToAddresses(response *dns.Msg) []netip.Addr {
	return adapter.DNSResponseAddresses(response)
}

func FixedResponseStatus(message *dns.Msg, rcode int) *dns.Msg {
	return &dns.Msg{
		MsgHdr: dns.MsgHdr{
			Id:                 message.Id,
			Response:           true,
			Authoritative:      true,
			RecursionDesired:   true,
			RecursionAvailable: true,
			Rcode:              rcode,
		},
		Question: message.Question,
	}
}

func FixedResponse(id uint16, question dns.Question, addresses []netip.Addr, timeToLive uint32) *dns.Msg {
	response := dns.Msg{
		MsgHdr: dns.MsgHdr{
			Id:                 id,
			Response:           true,
			Authoritative:      true,
			RecursionDesired:   true,
			RecursionAvailable: true,
			Rcode:              dns.RcodeSuccess,
		},
		Question: []dns.Question{question},
	}
	for _, address := range addresses {
		if address.Is4() && question.Qtype == dns.TypeA {
			response.Answer = append(response.Answer, &dns.A{
				Hdr: dns.RR_Header{
					Name:   question.Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    timeToLive,
				},
				A: address.AsSlice(),
			})
		} else if address.Is6() && question.Qtype == dns.TypeAAAA {
			response.Answer = append(response.Answer, &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   question.Name,
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    timeToLive,
				},
				AAAA: address.AsSlice(),
			})
		}
	}
	return &response
}

func FixedResponseCNAME(id uint16, question dns.Question, record string, timeToLive uint32) *dns.Msg {
	response := dns.Msg{
		MsgHdr: dns.MsgHdr{
			Id:                 id,
			Response:           true,
			Authoritative:      true,
			RecursionDesired:   true,
			RecursionAvailable: true,
			Rcode:              dns.RcodeSuccess,
		},
		Question: []dns.Question{question},
		Answer: []dns.RR{
			&dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   question.Name,
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
					Ttl:    timeToLive,
				},
				Target: record,
			},
		},
	}
	return &response
}

func FixedResponseTXT(id uint16, question dns.Question, records []string, timeToLive uint32) *dns.Msg {
	response := dns.Msg{
		MsgHdr: dns.MsgHdr{
			Id:                 id,
			Response:           true,
			Authoritative:      true,
			RecursionDesired:   true,
			RecursionAvailable: true,
			Rcode:              dns.RcodeSuccess,
		},
		Question: []dns.Question{question},
		Answer: []dns.RR{
			&dns.TXT{
				Hdr: dns.RR_Header{
					Name:   question.Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    timeToLive,
				},
				Txt: records,
			},
		},
	}
	return &response
}

func FixedResponseMX(id uint16, question dns.Question, records []*net.MX, timeToLive uint32) *dns.Msg {
	response := dns.Msg{
		MsgHdr: dns.MsgHdr{
			Id:                 id,
			Response:           true,
			Authoritative:      true,
			RecursionDesired:   true,
			RecursionAvailable: true,
			Rcode:              dns.RcodeSuccess,
		},
		Question: []dns.Question{question},
	}
	for _, record := range records {
		response.Answer = append(response.Answer, &dns.MX{
			Hdr: dns.RR_Header{
				Name:   question.Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    timeToLive,
			},
			Preference: record.Pref,
			Mx:         record.Host,
		})
	}
	return &response
}
