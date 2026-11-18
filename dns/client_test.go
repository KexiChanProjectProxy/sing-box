package dns

import (
	"context"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"

	mDNS "github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

type cacheTestTransport struct {
	access  sync.Mutex
	ttl     uint32
	address netip.Addr
	delay   time.Duration
	err     error
	queries atomic.Int32
}

func (t *cacheTestTransport) Start(stage adapter.StartStage) error { return nil }
func (t *cacheTestTransport) Close() error                         { return nil }
func (t *cacheTestTransport) Type() string                         { return "test" }
func (t *cacheTestTransport) Tag() string                          { return "test" }
func (t *cacheTestTransport) Dependencies() []string               { return nil }
func (t *cacheTestTransport) Reset()                               {}

func (t *cacheTestTransport) set(address netip.Addr, delay time.Duration, err error) {
	t.access.Lock()
	defer t.access.Unlock()
	t.address = address
	t.delay = delay
	t.err = err
}

func (t *cacheTestTransport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	t.queries.Add(1)
	t.access.Lock()
	ttl, address, delay, err := t.ttl, t.address, t.delay, t.err
	t.access.Unlock()
	select {
	case <-time.After(delay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, err
	}
	return FixedResponse(message.Id, message.Question[0], []netip.Addr{address}, ttl), nil
}

func (t *cacheTestTransport) ExchangeAsync(ctx context.Context, message *mDNS.Msg, callback func(response *mDNS.Msg, err error)) {
	go func() {
		callback(t.Exchange(ctx, message))
	}()
}

var (
	cacheTestAddressOld = netip.MustParseAddr("192.0.2.1")
	cacheTestAddressNew = netip.MustParseAddr("192.0.2.2")
)

func newCacheTestClient(options ClientOptions) *Client {
	options.Context = context.Background()
	options.Logger = log.NewNOPFactory().Logger()
	client := NewClient(options)
	client.Start()
	return client
}

func cacheTestQuery() *mDNS.Msg {
	message := new(mDNS.Msg)
	message.SetQuestion("example.com.", mDNS.TypeA)
	return message
}

func responseTTL(t *testing.T, response *mDNS.Msg) uint32 {
	t.Helper()
	require.NotEmpty(t, response.Answer)
	return response.Answer[0].Header().Ttl
}

func cachedResponseAddress(t *testing.T, response *mDNS.Msg) netip.Addr {
	t.Helper()
	addresses := MessageToAddresses(response)
	require.Len(t, addresses, 1)
	return addresses[0]
}

// seedCache stores a response for the test query whose records carry originalTTL
// and which expires after lifetime.
func seedCache(client *Client, transport *cacheTestTransport, address netip.Addr, originalTTL uint32, lifetime time.Duration) {
	message := cacheTestQuery()
	key := client.newCacheKey(transport, message.Question[0], message, adapter.DNSQueryOptions{})
	response := FixedResponse(0, message.Question[0], []netip.Addr{address}, originalTTL)
	client.cache.AddWithLifetime(key, response, lifetime)
}

func TestClientTTLBounds(t *testing.T) {
	t.Parallel()
	ttl := func(value uint32) *uint32 { return &value }
	for _, testCase := range []struct {
		name        string
		global      ClientOptions
		query       adapter.DNSQueryOptions
		upstreamTTL uint32
		expectedTTL uint32
		cached      bool
	}{
		{name: "no bounds", upstreamTTL: 30, expectedTTL: 30, cached: true},
		{name: "min raises", global: ClientOptions{MinTTL: 300}, upstreamTTL: 30, expectedTTL: 300, cached: true},
		{name: "max lowers", global: ClientOptions{MaxTTL: 60}, upstreamTTL: 3600, expectedTTL: 60, cached: true},
		{name: "within bounds", global: ClientOptions{MinTTL: 10, MaxTTL: 600}, upstreamTTL: 30, expectedTTL: 30, cached: true},
		{name: "zero ttl untouched", global: ClientOptions{MinTTL: 300}, upstreamTTL: 0, expectedTTL: 0, cached: false},
		{name: "rewrite bypasses bounds", global: ClientOptions{MinTTL: 300, MaxTTL: 600}, query: adapter.DNSQueryOptions{RewriteTTL: ttl(5)}, upstreamTTL: 30, expectedTTL: 5, cached: true},
		{name: "rule min overrides global", global: ClientOptions{MinTTL: 300}, query: adapter.DNSQueryOptions{MinTTL: 60}, upstreamTTL: 30, expectedTTL: 60, cached: true},
		{name: "rule max overrides global", global: ClientOptions{MaxTTL: 60}, query: adapter.DNSQueryOptions{MaxTTL: 120}, upstreamTTL: 3600, expectedTTL: 120, cached: true},
		{name: "rule min above global max", global: ClientOptions{MaxTTL: 60}, query: adapter.DNSQueryOptions{MinTTL: 600}, upstreamTTL: 30, expectedTTL: 600, cached: true},
		{name: "rule max below global min", global: ClientOptions{MinTTL: 600}, query: adapter.DNSQueryOptions{MaxTTL: 60}, upstreamTTL: 3600, expectedTTL: 60, cached: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			client := newCacheTestClient(testCase.global)
			transport := &cacheTestTransport{ttl: testCase.upstreamTTL, address: cacheTestAddressOld}
			response, err := client.Exchange(context.Background(), transport, cacheTestQuery(), testCase.query, nil)
			require.NoError(t, err)
			require.Equal(t, testCase.expectedTTL, responseTTL(t, response))
			response, err = client.Exchange(context.Background(), transport, cacheTestQuery(), testCase.query, nil)
			require.NoError(t, err)
			if testCase.cached {
				require.EqualValues(t, 1, transport.queries.Load())
				require.LessOrEqual(t, responseTTL(t, response), testCase.expectedTTL)
			} else {
				require.EqualValues(t, 2, transport.queries.Load())
			}
		})
	}
}

func TestClientCacheCapacityBelowDefault(t *testing.T) {
	t.Parallel()
	client := newCacheTestClient(ClientOptions{CacheCapacity: 16})
	transport := &cacheTestTransport{ttl: 300, address: cacheTestAddressOld}
	for i := 0; i < 64; i++ {
		message := new(mDNS.Msg)
		message.SetQuestion(mDNS.Fqdn(E.New("host", i).Error()+".example.com"), mDNS.TypeA)
		_, err := client.Exchange(context.Background(), transport, message, adapter.DNSQueryOptions{}, nil)
		require.NoError(t, err)
	}
	require.LessOrEqual(t, client.cache.Len(), 16)
}

func TestClientOptimisticRuleOverride(t *testing.T) {
	t.Parallel()
	t.Run("rule enables", func(t *testing.T) {
		t.Parallel()
		client := newCacheTestClient(ClientOptions{OptimisticRetention: time.Hour})
		transport := &cacheTestTransport{ttl: 300, address: cacheTestAddressNew, delay: 200 * time.Millisecond}
		seedCache(client, transport, cacheTestAddressOld, 300, time.Millisecond)
		time.Sleep(10 * time.Millisecond)
		start := time.Now()
		response, err := client.Exchange(context.Background(), transport, cacheTestQuery(), adapter.DNSQueryOptions{
			Optimistic: &adapter.DNSOptimisticOptions{Enabled: true},
		}, nil)
		require.NoError(t, err)
		require.Less(t, time.Since(start), 100*time.Millisecond)
		require.Equal(t, cacheTestAddressOld, cachedResponseAddress(t, response))
		require.EqualValues(t, 1, responseTTL(t, response))
		require.Eventually(t, func() bool {
			hit, cached := client.loadResponse(client.newCacheKey(transport, cacheTestQuery().Question[0], cacheTestQuery(), adapter.DNSQueryOptions{}), 0)
			return cached && !hit.stale && MessageToAddresses(hit.response)[0] == cacheTestAddressNew
		}, time.Second, 10*time.Millisecond)
	})
	t.Run("query without override misses", func(t *testing.T) {
		t.Parallel()
		client := newCacheTestClient(ClientOptions{OptimisticRetention: time.Hour})
		transport := &cacheTestTransport{ttl: 300, address: cacheTestAddressNew}
		seedCache(client, transport, cacheTestAddressOld, 300, time.Millisecond)
		time.Sleep(10 * time.Millisecond)
		response, err := client.Exchange(context.Background(), transport, cacheTestQuery(), adapter.DNSQueryOptions{}, nil)
		require.NoError(t, err)
		require.Equal(t, cacheTestAddressNew, cachedResponseAddress(t, response))
	})
	t.Run("rule disables", func(t *testing.T) {
		t.Parallel()
		client := newCacheTestClient(ClientOptions{OptimisticTimeout: time.Hour})
		transport := &cacheTestTransport{ttl: 300, address: cacheTestAddressNew}
		seedCache(client, transport, cacheTestAddressOld, 300, time.Millisecond)
		time.Sleep(10 * time.Millisecond)
		response, err := client.Exchange(context.Background(), transport, cacheTestQuery(), adapter.DNSQueryOptions{
			Optimistic: &adapter.DNSOptimisticOptions{Enabled: false},
		}, nil)
		require.NoError(t, err)
		require.Equal(t, cacheTestAddressNew, cachedResponseAddress(t, response))
	})
	t.Run("expired past retention is dropped", func(t *testing.T) {
		t.Parallel()
		client := newCacheTestClient(ClientOptions{})
		transport := &cacheTestTransport{ttl: 300, address: cacheTestAddressNew}
		seedCache(client, transport, cacheTestAddressOld, 300, time.Millisecond)
		time.Sleep(10 * time.Millisecond)
		_, cached := client.loadResponse(client.newCacheKey(transport, cacheTestQuery().Question[0], cacheTestQuery(), adapter.DNSQueryOptions{}), 0)
		require.False(t, cached)
		require.Zero(t, client.cache.Len())
	})
}

func TestClientOptimisticClientTimeout(t *testing.T) {
	t.Parallel()
	optimistic := ClientOptions{OptimisticTimeout: time.Hour, OptimisticClientTimeout: 100 * time.Millisecond}
	t.Run("fresh answer in time", func(t *testing.T) {
		t.Parallel()
		client := newCacheTestClient(optimistic)
		transport := &cacheTestTransport{ttl: 300, address: cacheTestAddressNew, delay: 10 * time.Millisecond}
		seedCache(client, transport, cacheTestAddressOld, 300, time.Millisecond)
		time.Sleep(10 * time.Millisecond)
		response, err := client.Exchange(context.Background(), transport, cacheTestQuery(), adapter.DNSQueryOptions{}, nil)
		require.NoError(t, err)
		require.Equal(t, cacheTestAddressNew, cachedResponseAddress(t, response))
		require.EqualValues(t, 300, responseTTL(t, response))
	})
	t.Run("slow upstream serves stale", func(t *testing.T) {
		t.Parallel()
		client := newCacheTestClient(optimistic)
		transport := &cacheTestTransport{ttl: 300, address: cacheTestAddressNew, delay: 500 * time.Millisecond}
		seedCache(client, transport, cacheTestAddressOld, 300, time.Millisecond)
		time.Sleep(10 * time.Millisecond)
		start := time.Now()
		response, err := client.Exchange(context.Background(), transport, cacheTestQuery(), adapter.DNSQueryOptions{}, nil)
		require.NoError(t, err)
		elapsed := time.Since(start)
		require.GreaterOrEqual(t, elapsed, 90*time.Millisecond)
		require.Less(t, elapsed, 400*time.Millisecond)
		require.Equal(t, cacheTestAddressOld, cachedResponseAddress(t, response))
		require.Eventually(t, func() bool {
			response, err = client.Exchange(context.Background(), transport, cacheTestQuery(), adapter.DNSQueryOptions{}, nil)
			return err == nil && cachedResponseAddress(t, response) == cacheTestAddressNew
		}, 2*time.Second, 50*time.Millisecond)
		require.EqualValues(t, 1, transport.queries.Load())
	})
	t.Run("failing upstream serves stale", func(t *testing.T) {
		t.Parallel()
		client := newCacheTestClient(optimistic)
		transport := &cacheTestTransport{ttl: 300, address: cacheTestAddressNew, err: E.New("upstream down")}
		seedCache(client, transport, cacheTestAddressOld, 300, time.Millisecond)
		time.Sleep(10 * time.Millisecond)
		start := time.Now()
		response, err := client.Exchange(context.Background(), transport, cacheTestQuery(), adapter.DNSQueryOptions{}, nil)
		require.NoError(t, err)
		require.Less(t, time.Since(start), 90*time.Millisecond)
		require.Equal(t, cacheTestAddressOld, cachedResponseAddress(t, response))
	})
	t.Run("async", func(t *testing.T) {
		t.Parallel()
		client := newCacheTestClient(optimistic)
		transport := &cacheTestTransport{ttl: 300, address: cacheTestAddressNew, delay: 10 * time.Millisecond}
		seedCache(client, transport, cacheTestAddressOld, 300, time.Millisecond)
		time.Sleep(10 * time.Millisecond)
		done := make(chan *mDNS.Msg, 1)
		client.ExchangeAsync(context.Background(), transport, cacheTestQuery(), adapter.DNSQueryOptions{}, nil, func(response *mDNS.Msg, err error) {
			require.NoError(t, err)
			done <- response
		})
		select {
		case response := <-done:
			require.Equal(t, cacheTestAddressNew, cachedResponseAddress(t, response))
		case <-time.After(time.Second):
			t.Fatal("async exchange did not complete")
		}
	})
	t.Run("lookup", func(t *testing.T) {
		t.Parallel()
		client := newCacheTestClient(optimistic)
		transport := &cacheTestTransport{ttl: 300, address: cacheTestAddressNew, delay: 10 * time.Millisecond}
		seedCache(client, transport, cacheTestAddressOld, 300, time.Millisecond)
		time.Sleep(10 * time.Millisecond)
		addresses, err := client.Lookup(context.Background(), transport, "example.com", adapter.DNSQueryOptions{LookupStrategy: C.DomainStrategyIPv4Only}, nil)
		require.NoError(t, err)
		require.Equal(t, []netip.Addr{cacheTestAddressNew}, addresses)
	})
}

func TestClientPrefetch(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name     string
		global   bool
		rule     *bool
		lifetime time.Duration
		refresh  bool
	}{
		{name: "near expiry", global: true, lifetime: 5 * time.Second, refresh: true},
		{name: "plenty left", global: true, lifetime: 50 * time.Second, refresh: false},
		{name: "disabled", global: false, lifetime: 5 * time.Second, refresh: false},
		{name: "rule enables", global: false, rule: boolPointer(true), lifetime: 5 * time.Second, refresh: true},
		{name: "rule disables", global: true, rule: boolPointer(false), lifetime: 5 * time.Second, refresh: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			client := newCacheTestClient(ClientOptions{Prefetch: testCase.global})
			transport := &cacheTestTransport{ttl: 100, address: cacheTestAddressNew}
			seedCache(client, transport, cacheTestAddressOld, 100, testCase.lifetime)
			response, err := client.Exchange(context.Background(), transport, cacheTestQuery(), adapter.DNSQueryOptions{Prefetch: testCase.rule}, nil)
			require.NoError(t, err)
			require.Equal(t, cacheTestAddressOld, cachedResponseAddress(t, response))
			if testCase.refresh {
				require.Eventually(t, func() bool {
					response, err = client.Exchange(context.Background(), transport, cacheTestQuery(), adapter.DNSQueryOptions{Prefetch: boolPointer(false)}, nil)
					return err == nil && cachedResponseAddress(t, response) == cacheTestAddressNew
				}, time.Second, 10*time.Millisecond)
				require.EqualValues(t, 1, transport.queries.Load())
			} else {
				time.Sleep(50 * time.Millisecond)
				require.Zero(t, transport.queries.Load())
			}
		})
	}
}

func boolPointer(value bool) *bool {
	return &value
}
