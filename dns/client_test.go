package dns

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/sagernet/sing-box/adapter"
	"github.com/stretchr/testify/assert"
)

type MockTransport struct {
	response  *dns.Msg
	delay     time.Duration
	calls     int32
	err       error
	exchangeFn func(ctx context.Context, message *dns.Msg) (*dns.Msg, error)
	tag       string
}

func (t *MockTransport) Name() string {
	return "mock"
}

func (t *MockTransport) Tag() string {
	if t.tag != "" {
		return t.tag
	}
	return "mock"
}

func (t *MockTransport) Type() string {
	return "mock"
}

func (t *MockTransport) Start(stage adapter.StartStage) error {
	return nil
}

func (t *MockTransport) Close() error {
	return nil
}

func (t *MockTransport) Dependencies() []string {
	return nil
}

func (t *MockTransport) Reset() {
}

func (t *MockTransport) Exchange(ctx context.Context, message *dns.Msg) (*dns.Msg, error) {
	time.Sleep(t.delay)
	atomic.AddInt32(&t.calls, 1)
	if t.err != nil {
		return nil, t.err
	}
	if t.exchangeFn != nil {
		return t.exchangeFn(ctx, message)
	}
	if t.response != nil {
		response := t.response.Copy()
		response.SetReply(message)
		// Ensure Rcode is preserved (SetReply should do this, but let's be explicit)
		response.Rcode = t.response.Rcode
		return response, nil
	}
	return nil, errors.New("mock transport error")
}

func (t *MockTransport) Calls() int {
	return int(atomic.LoadInt32(&t.calls))
}

func (t *MockTransport) ResetCalls() {
	atomic.StoreInt32(&t.calls, 0)
}

func TestClient_LazyRefresh(t *testing.T) {
	transport := &MockTransport{
		delay: 50 * time.Millisecond,
	}

	client := NewClient(ClientOptions{})

	// Setup initial response
	response1 := new(dns.Msg)
	response1.Answer = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   "example.com.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    60,
			},
			A: net.ParseIP("1.1.1.1"),
		},
	}
	transport.response = response1

	options := adapter.DNSQueryOptions{
		HoldValid: 1 * time.Second, // Changed from 200ms to >= 1s
	}

	question := dns.Question{
		Name:   "example.com.",
		Qtype:  dns.TypeA,
		Qclass: dns.ClassINET,
	}
	message := new(dns.Msg)
	message.SetQuestion(question.Name, question.Qtype)

	// 1. First query - should hit transport
	resp, err := client.Exchange(context.Background(), transport, message, options, nil)
	assert.NoError(t, err)
	assert.Equal(t, "1.1.1.1", resp.Answer[0].(*dns.A).A.String())
	assert.Equal(t, 1, transport.Calls())

	// 2. Query within hold time - should hit cache
	time.Sleep(500 * time.Millisecond)
	resp, err = client.Exchange(context.Background(), transport, message, options, nil)
	assert.NoError(t, err)
	assert.Equal(t, "1.1.1.1", resp.Answer[0].(*dns.A).A.String())
	assert.Equal(t, 1, transport.Calls()) // No new call

	// Update transport response for next refresh
	response2 := new(dns.Msg)
	response2.Answer = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   "example.com.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    60,
			},
			A: net.ParseIP("2.2.2.2"),
		},
	}
	transport.response = response2

	// 3. Query after hold time (expired) - should return stale data immediately and trigger refresh
	time.Sleep(600 * time.Millisecond) // Total > 1s
	start := time.Now()
	resp, err = client.Exchange(context.Background(), transport, message, options, nil)
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.Equal(t, "1.1.1.1", resp.Answer[0].(*dns.A).A.String()) // Stale data
	assert.Less(t, duration, 20*time.Millisecond)                 // Should be immediate (no transport delay)

	// Wait for background refresh to complete
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 2, transport.Calls()) // Background refresh happened

	// 4. Query again - should get new data
	resp, err = client.Exchange(context.Background(), transport, message, options, nil)
	assert.NoError(t, err)
	assert.Equal(t, "2.2.2.2", resp.Answer[0].(*dns.A).A.String()) // New data
}

func TestClient_Retry(t *testing.T) {
	transport := &MockTransport{
		delay: 10 * time.Millisecond,
		err:   errors.New("network error"),
	}

	client := NewClient(ClientOptions{})

	options := adapter.DNSQueryOptions{
		ResolveRetries: 3,
		ResolveTimeout: 50 * time.Millisecond,
	}

	question := dns.Question{
		Name:   "retry.com.",
		Qtype:  dns.TypeA,
		Qclass: dns.ClassINET,
	}
	message := new(dns.Msg)
	message.SetQuestion(question.Name, question.Qtype)

	// Should fail after retries
	_, err := client.Exchange(context.Background(), transport, message, options, nil)
	assert.Error(t, err)
	assert.Equal(t, 3, transport.Calls())
}

func TestClient_HoldNX(t *testing.T) {
	transport := &MockTransport{
		delay: 50 * time.Millisecond,
	}

	client := NewClient(ClientOptions{})

	// Setup NXDOMAIN response
	response1 := new(dns.Msg)
	response1.Rcode = dns.RcodeNameError
	transport.response = response1

	options := adapter.DNSQueryOptions{
		HoldNX: 1 * time.Second, // Changed from 200ms to >= 1s
	}

	question := dns.Question{
		Name:   "nx.com.",
		Qtype:  dns.TypeA,
		Qclass: dns.ClassINET,
	}
	message := new(dns.Msg)
	message.SetQuestion(question.Name, question.Qtype)

	// 1. First query - should hit transport
	resp, err := client.Exchange(context.Background(), transport, message, options, nil)
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeNameError, resp.Rcode)
	assert.Equal(t, 1, transport.Calls())

	// 2. Query within hold time - should hit cache
	time.Sleep(500 * time.Millisecond)
	resp, err = client.Exchange(context.Background(), transport, message, options, nil)
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeNameError, resp.Rcode)
	assert.Equal(t, 1, transport.Calls()) // No new call

	// 3. Query after hold time (expired) - should trigger refresh
	time.Sleep(600 * time.Millisecond) // Expired

	// Should return stale cached data immediately AND trigger background refresh
	resp, err = client.Exchange(context.Background(), transport, message, options, nil)
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeNameError, resp.Rcode)

	// Wait for background refresh to complete
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 2, transport.Calls()) // Background refresh happened
}

func TestClient_HoldRefused(t *testing.T) {
	transport := &MockTransport{
		delay: 50 * time.Millisecond,
	}

	client := NewClient(ClientOptions{})

	// Setup REFUSED response
	response1 := new(dns.Msg)
	response1.Rcode = dns.RcodeRefused
	transport.response = response1

	options := adapter.DNSQueryOptions{
		HoldRefused: 1 * time.Second,
	}

	question := dns.Question{
		Name:   "refused.com.",
		Qtype:  dns.TypeA,
		Qclass: dns.ClassINET,
	}
	message := new(dns.Msg)
	message.SetQuestion(question.Name, question.Qtype)

	// 1. First query - should hit transport
	resp, err := client.Exchange(context.Background(), transport, message, options, nil)
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeRefused, resp.Rcode)
	assert.Equal(t, 1, transport.Calls())

	// 2. Query within hold time - should hit cache
	time.Sleep(500 * time.Millisecond)
	resp, err = client.Exchange(context.Background(), transport, message, options, nil)
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeRefused, resp.Rcode)
	assert.Equal(t, 1, transport.Calls()) // No new call

	// 3. Query after hold time - should trigger background refresh
	time.Sleep(600 * time.Millisecond)
	resp, err = client.Exchange(context.Background(), transport, message, options, nil)
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeRefused, resp.Rcode)

	// Wait for background refresh to complete
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 2, transport.Calls())
}

func TestClient_HoldOther_SERVFAIL(t *testing.T) {
	transport := &MockTransport{
		delay: 50 * time.Millisecond,
	}

	client := NewClient(ClientOptions{})

	// Setup SERVFAIL response
	response1 := new(dns.Msg)
	response1.Rcode = dns.RcodeServerFailure
	transport.response = response1

	options := adapter.DNSQueryOptions{
		HoldOther: 1 * time.Second,
	}

	question := dns.Question{
		Name:   "servfail.com.",
		Qtype:  dns.TypeA,
		Qclass: dns.ClassINET,
	}
	message := new(dns.Msg)
	message.SetQuestion(question.Name, question.Qtype)

	// 1. First query - should hit transport
	resp, err := client.Exchange(context.Background(), transport, message, options, nil)
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeServerFailure, resp.Rcode)
	assert.Equal(t, 1, transport.Calls())

	// 2. Query within hold time - should hit cache
	time.Sleep(500 * time.Millisecond)
	resp, err = client.Exchange(context.Background(), transport, message, options, nil)
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeServerFailure, resp.Rcode)
	assert.Equal(t, 1, transport.Calls())

	// 3. Query after hold time - should trigger background refresh
	time.Sleep(600 * time.Millisecond)
	resp, err = client.Exchange(context.Background(), transport, message, options, nil)
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeServerFailure, resp.Rcode)

	// Wait for background refresh to complete
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 2, transport.Calls())
}

func TestClient_HoldTimeout_FallbackToStale(t *testing.T) {
	transport := &MockTransport{
		delay: 50 * time.Millisecond,
	}

	client := NewClient(ClientOptions{})

	// Setup initial successful response
	response1 := new(dns.Msg)
	response1.Answer = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   "example.com.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    60,
			},
			A: net.ParseIP("1.1.1.1"),
		},
	}
	transport.response = response1

	options := adapter.DNSQueryOptions{
		HoldValid:   2 * time.Second,
		HoldTimeout: 1 * time.Second,
	}

	question := dns.Question{
		Name:   "example.com.",
		Qtype:  dns.TypeA,
		Qclass: dns.ClassINET,
	}
	message := new(dns.Msg)
	message.SetQuestion(question.Name, question.Qtype)

	// 1. First query - cache the response
	resp, err := client.Exchange(context.Background(), transport, message, options, nil)
	assert.NoError(t, err)
	assert.Equal(t, "1.1.1.1", resp.Answer[0].(*dns.A).A.String())
	assert.Equal(t, 1, transport.Calls())

	// 2. Make transport fail
	transport.ResetCalls()
	transport.err = errors.New("DNS timeout")

	// 3. Query should fallback to stale cached response
	resp, err = client.Exchange(context.Background(), transport, message, options, nil)
	assert.NoError(t, err) // Should not error because of HoldTimeout fallback
	assert.Equal(t, "1.1.1.1", resp.Answer[0].(*dns.A).A.String())
	// Transport may have been attempted (failed), but cached response was returned
}

func TestClient_RetrySuccessOnSecondAttempt(t *testing.T) {
	attempts := 0
	transport := &MockTransport{
		exchangeFn: func(ctx context.Context, message *dns.Msg) (*dns.Msg, error) {
			attempts++
			if attempts == 1 {
				return nil, errors.New("temporary error")
			}
			// Success on second attempt
			response := new(dns.Msg)
			response.Answer = []dns.RR{
				&dns.A{
					Hdr: dns.RR_Header{
						Name:   "retry-success.com.",
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    60,
					},
					A: net.ParseIP("3.3.3.3"),
				},
			}
			response.SetReply(message)
			return response, nil
		},
	}

	client := NewClient(ClientOptions{})

	options := adapter.DNSQueryOptions{
		ResolveRetries: 3,
		ResolveTimeout: 50 * time.Millisecond,
	}

	question := dns.Question{
		Name:   "retry-success.com.",
		Qtype:  dns.TypeA,
		Qclass: dns.ClassINET,
	}
	message := new(dns.Msg)
	message.SetQuestion(question.Name, question.Qtype)

	// Should succeed on second attempt
	resp, err := client.Exchange(context.Background(), transport, message, options, nil)
	assert.NoError(t, err)
	assert.Equal(t, "3.3.3.3", resp.Answer[0].(*dns.A).A.String())
	assert.Equal(t, 2, attempts)
}

func TestClient_BackgroundRefreshDeduplication(t *testing.T) {
	transport := &MockTransport{
		delay: 100 * time.Millisecond,
	}

	client := NewClient(ClientOptions{})

	response1 := new(dns.Msg)
	response1.Answer = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   "dedup.com.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    60,
			},
			A: net.ParseIP("1.1.1.1"),
		},
	}
	transport.response = response1

	options := adapter.DNSQueryOptions{
		HoldValid: 1 * time.Second, // Changed from 500ms to avoid rounding ambiguity
	}

	question := dns.Question{
		Name:   "dedup.com.",
		Qtype:  dns.TypeA,
		Qclass: dns.ClassINET,
	}
	message := new(dns.Msg)
	message.SetQuestion(question.Name, question.Qtype)

	// First query
	_, err := client.Exchange(context.Background(), transport, message, options, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, transport.Calls())

	// Wait for hold to expire (past soft TTL of 1s)
	time.Sleep(1200 * time.Millisecond)

	// Launch multiple concurrent queries - should trigger only 1 background refresh
	var wg sync.WaitGroup
	results := make([]*dns.Msg, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			r, e := client.Exchange(context.Background(), transport, message, options, nil)
			assert.NoError(t, e)
			results[idx] = r
		}(i)
	}
	wg.Wait()

	// All should get stale data
	for _, r := range results {
		assert.Equal(t, "1.1.1.1", r.Answer[0].(*dns.A).A.String())
	}

	// Wait for background refresh to complete
	time.Sleep(200 * time.Millisecond)
	// Should be exactly 2 calls: initial + 1 background refresh (deduplicated)
	assert.Equal(t, 2, transport.Calls())
}

func TestClient_MixedHoldOptions(t *testing.T) {
	transport := &MockTransport{
		delay: 50 * time.Millisecond,
	}

	client := NewClient(ClientOptions{})

	options := adapter.DNSQueryOptions{
		HoldValid:   3 * time.Second,
		HoldNX:      1 * time.Second,
		HoldRefused: 1 * time.Second,
		HoldOther:   1 * time.Second,
	}

	t.Run("SUCCESS uses HoldValid", func(t *testing.T) {
		transport.ResetCalls()
		response := new(dns.Msg)
		response.Answer = []dns.RR{
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "success.com.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    60,
				},
				A: net.ParseIP("1.1.1.1"),
			},
		}
		transport.response = response

		question := dns.Question{
			Name:   "success.com.",
			Qtype:  dns.TypeA,
			Qclass: dns.ClassINET,
		}
		message := new(dns.Msg)
		message.SetQuestion(question.Name, question.Qtype)

		_, err := client.Exchange(context.Background(), transport, message, options, nil)
		assert.NoError(t, err)
		assert.Equal(t, 1, transport.Calls())

		// Should hit cache after 1s (HoldValid=3s)
		time.Sleep(500 * time.Millisecond)
		_, err = client.Exchange(context.Background(), transport, message, options, nil)
		assert.NoError(t, err)
		assert.Equal(t, 1, transport.Calls())
	})

	t.Run("NXDOMAIN uses HoldNX", func(t *testing.T) {
		transport.ResetCalls()
		response := new(dns.Msg)
		response.Rcode = dns.RcodeNameError
		transport.response = response

		question := dns.Question{
			Name:   "nxtest.com.",
			Qtype:  dns.TypeA,
			Qclass: dns.ClassINET,
		}
		message := new(dns.Msg)
		message.SetQuestion(question.Name, question.Qtype)

		resp, err := client.Exchange(context.Background(), transport, message, options, nil)
		assert.NoError(t, err)
		assert.Equal(t, dns.RcodeNameError, resp.Rcode)
		assert.Equal(t, 1, transport.Calls())

		// Should hit cache after 500ms (HoldNX=1s)
		time.Sleep(500 * time.Millisecond)
		_, err = client.Exchange(context.Background(), transport, message, options, nil)
		assert.NoError(t, err)
		assert.Equal(t, 1, transport.Calls())
	})
}

func TestClient_CacheDisabledWithHoldOptions(t *testing.T) {
	transport := &MockTransport{
		delay: 50 * time.Millisecond,
	}

	client := NewClient(ClientOptions{
		DisableCache: true,
	})

	response := new(dns.Msg)
	response.Answer = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   "nocache.com.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    60,
			},
			A: net.ParseIP("1.1.1.1"),
		},
	}
	transport.response = response

	options := adapter.DNSQueryOptions{
		HoldValid: 5 * time.Second,
	}

	question := dns.Question{
		Name:   "nocache.com.",
		Qtype:  dns.TypeA,
		Qclass: dns.ClassINET,
	}
	message := new(dns.Msg)
	message.SetQuestion(question.Name, question.Qtype)

	// First query
	_, err := client.Exchange(context.Background(), transport, message, options, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, transport.Calls())

	// Second query - should NOT hit cache (cache disabled)
	_, err = client.Exchange(context.Background(), transport, message, options, nil)
	assert.NoError(t, err)
	assert.Equal(t, 2, transport.Calls()) // Should call transport again
}

func TestClient_IndependentCacheWithHold(t *testing.T) {
	transport1 := &MockTransport{
		delay: 50 * time.Millisecond,
		tag:   "transport1",
	}
	transport2 := &MockTransport{
		delay: 50 * time.Millisecond,
		tag:   "transport2",
	}

	client := NewClient(ClientOptions{
		IndependentCache: true,
	})

	response1 := new(dns.Msg)
	response1.Answer = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   "independent.com.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    60,
			},
			A: net.ParseIP("1.1.1.1"),
		},
	}
	transport1.response = response1

	response2 := new(dns.Msg)
	response2.Answer = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   "independent.com.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    60,
			},
			A: net.ParseIP("2.2.2.2"),
		},
	}
	transport2.response = response2

	options := adapter.DNSQueryOptions{
		HoldValid: 2 * time.Second,
	}

	question := dns.Question{
		Name:   "independent.com.",
		Qtype:  dns.TypeA,
		Qclass: dns.ClassINET,
	}
	message := new(dns.Msg)
	message.SetQuestion(question.Name, question.Qtype)

	// Query transport1
	resp, err := client.Exchange(context.Background(), transport1, message, options, nil)
	assert.NoError(t, err)
	assert.Equal(t, "1.1.1.1", resp.Answer[0].(*dns.A).A.String())
	assert.Equal(t, 1, transport1.Calls())

	// Query transport2 - should have separate cache
	resp, err = client.Exchange(context.Background(), transport2, message, options, nil)
	assert.NoError(t, err)
	assert.Equal(t, "2.2.2.2", resp.Answer[0].(*dns.A).A.String())
	assert.Equal(t, 1, transport2.Calls())

	// Query transport1 again - should hit its cache
	resp, err = client.Exchange(context.Background(), transport1, message, options, nil)
	assert.NoError(t, err)
	assert.Equal(t, "1.1.1.1", resp.Answer[0].(*dns.A).A.String())
	assert.Equal(t, 1, transport1.Calls()) // No new call

	// Query transport2 again - should hit its cache
	resp, err = client.Exchange(context.Background(), transport2, message, options, nil)
	assert.NoError(t, err)
	assert.Equal(t, "2.2.2.2", resp.Answer[0].(*dns.A).A.String())
	assert.Equal(t, 1, transport2.Calls()) // No new call
}

func TestClient_TTLAdjustmentNoUnderflow(t *testing.T) {
	transport := &MockTransport{
		delay: 10 * time.Millisecond,
	}

	client := NewClient(ClientOptions{})

	response := new(dns.Msg)
	response.Answer = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   "ttl.com.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    5, // Very short TTL
			},
			A: net.ParseIP("1.1.1.1"),
		},
	}
	transport.response = response

	options := adapter.DNSQueryOptions{
		HoldValid: 1 * time.Second,
	}

	question := dns.Question{
		Name:   "ttl.com.",
		Qtype:  dns.TypeA,
		Qclass: dns.ClassINET,
	}
	message := new(dns.Msg)
	message.SetQuestion(question.Name, question.Qtype)

	// First query - TTL should be HoldValid (1s), not the original record TTL (5s)
	resp, err := client.Exchange(context.Background(), transport, message, options, nil)
	assert.NoError(t, err)
	assert.Equal(t, uint32(1), resp.Answer[0].(*dns.A).Hdr.Ttl)

	// Wait for record TTL to expire but still within hard TTL
	time.Sleep(200 * time.Millisecond)

	// Get cached response - TTL should not underflow
	resp, err = client.Exchange(context.Background(), transport, message, options, nil)
	assert.NoError(t, err)
	// TTL should be >= 0, not wrapped to a large value
	ttl := resp.Answer[0].(*dns.A).Hdr.Ttl
	assert.Greater(t, uint32(1000000), ttl) // Should be a reasonable value, not uint32 max
}

func TestClient_ComputeHoldTTL(t *testing.T) {
	tests := []struct {
		name          string
		rcode         int
		hasAnswer     bool
		holdValid     time.Duration
		holdNX        time.Duration
		holdRefused   time.Duration
		holdOther     time.Duration
		nativeTTL     uint32
		expectedTTL   uint32
	}{
		{
			name:        "SUCCESS with HoldValid",
			rcode:       dns.RcodeSuccess,
			hasAnswer:   true,
			holdValid:   10 * time.Second,
			nativeTTL:   300,
			expectedTTL: 10,
		},
		{
			name:        "SUCCESS without HoldValid uses native",
			rcode:       dns.RcodeSuccess,
			hasAnswer:   true,
			nativeTTL:   300,
			expectedTTL: 300,
		},
		{
			name:        "NXDOMAIN with HoldNX",
			rcode:       dns.RcodeNameError,
			holdNX:      5 * time.Second,
			nativeTTL:   300,
			expectedTTL: 5,
		},
		{
			name:        "NXDOMAIN without HoldNX uses native",
			rcode:       dns.RcodeNameError,
			nativeTTL:   300,
			expectedTTL: 300,
		},
		{
			name:        "REFUSED with HoldRefused",
			rcode:       dns.RcodeRefused,
			holdRefused: 5 * time.Second,
			expectedTTL: 5,
		},
		{
			name:        "REFUSED without HoldRefused returns 0",
			rcode:       dns.RcodeRefused,
			expectedTTL: 0,
		},
		{
			name:        "SERVFAIL with HoldOther",
			rcode:       dns.RcodeServerFailure,
			holdOther:   5 * time.Second,
			expectedTTL: 5,
		},
		{
			name:        "SERVFAIL without HoldOther returns 0",
			rcode:       dns.RcodeServerFailure,
			expectedTTL: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := new(dns.Msg)
			response.Rcode = tt.rcode
			if tt.hasAnswer {
				response.Answer = []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "test.com.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    tt.nativeTTL,
						},
					},
				}
			}

			options := adapter.DNSQueryOptions{
				HoldValid:   tt.holdValid,
				HoldNX:      tt.holdNX,
				HoldRefused: tt.holdRefused,
				HoldOther:   tt.holdOther,
			}

			ttl := computeHoldTTL(response, tt.nativeTTL, options)
			assert.Equal(t, tt.expectedTTL, ttl)
		})
	}
}

func TestClient_SubSecondHoldDuration(t *testing.T) {
	// Test that sub-second durations are rounded up to 1s
	tests := []struct {
		name     string
		duration time.Duration
		expected uint32
	}{
		{"500ms rounds to 1s", 500 * time.Millisecond, 1},
		{"100ms rounds to 1s", 100 * time.Millisecond, 1},
		{"1s stays 1s", 1 * time.Second, 1},
		{"5s stays 5s", 5 * time.Second, 5},
		{"0 stays 0", 0, 0},
		{"negative stays 0", -1 * time.Second, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := durationToSeconds(tt.duration)
			assert.Equal(t, tt.expected, result)
		})
	}

	// Integration test: sub-second HoldValid should still work
	transport := &MockTransport{
		delay: 50 * time.Millisecond,
	}

	client := NewClient(ClientOptions{})

	response := new(dns.Msg)
	response.Answer = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   "subsecond.com.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    60,
			},
			A: net.ParseIP("1.1.1.1"),
		},
	}
	transport.response = response

	options := adapter.DNSQueryOptions{
		HoldValid: 500 * time.Millisecond, // Sub-second
	}

	question := dns.Question{
		Name:   "subsecond.com.",
		Qtype:  dns.TypeA,
		Qclass: dns.ClassINET,
	}
	message := new(dns.Msg)
	message.SetQuestion(question.Name, question.Qtype)

	// First query
	_, err := client.Exchange(context.Background(), transport, message, options, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, transport.Calls())

	// Second query should still hit cache (because 500ms rounds to 1s)
	_, err = client.Exchange(context.Background(), transport, message, options, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, transport.Calls()) // No new call
}

func TestClient_HoldDurationForResponse(t *testing.T) {
	tests := []struct {
		name        string
		rcode       int
		hasAnswer   bool
		holdValid   time.Duration
		holdNX      time.Duration
		holdRefused time.Duration
		holdOther   time.Duration
		expected    time.Duration
	}{
		{
			name:      "SUCCESS with answer uses HoldValid",
			rcode:     dns.RcodeSuccess,
			hasAnswer: true,
			holdValid: 10 * time.Second,
			expected:  10 * time.Second,
		},
		{
			name:     "NXDOMAIN uses HoldNX",
			rcode:    dns.RcodeNameError,
			holdNX:   5 * time.Second,
			expected: 5 * time.Second,
		},
		{
			name:        "REFUSED uses HoldRefused",
			rcode:       dns.RcodeRefused,
			holdRefused: 5 * time.Second,
			expected:    5 * time.Second,
		},
		{
			name:      "SERVFAIL uses HoldOther",
			rcode:     dns.RcodeServerFailure,
			holdOther: 5 * time.Second,
			expected:  5 * time.Second,
		},
		{
			name:     "NOTIMP uses HoldOther",
			rcode:    dns.RcodeNotImplemented,
			holdOther: 3 * time.Second,
			expected: 3 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := new(dns.Msg)
			response.Rcode = tt.rcode
			if tt.hasAnswer {
				response.Answer = []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "test.com.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
						},
					},
				}
			}

			options := adapter.DNSQueryOptions{
				HoldValid:   tt.holdValid,
				HoldNX:      tt.holdNX,
				HoldRefused: tt.holdRefused,
				HoldOther:   tt.holdOther,
			}

			result := holdDurationForResponse(response, options)
			assert.Equal(t, tt.expected, result)
		})
	}
}
