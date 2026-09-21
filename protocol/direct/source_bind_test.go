package direct

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/stretchr/testify/require"
)

// sourceBindTestDialer records which binding served a dial.
type sourceBindTestDialer struct {
	inet4, inet6 netip.Addr
	dials        *[]*sourceBindTestDialer
}

func (d *sourceBindTestDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	*d.dials = append(*d.dials, d)
	return nil, nil
}

func (d *sourceBindTestDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	*d.dials = append(*d.dials, d)
	return nil, nil
}

func (d *sourceBindTestDialer) DialParallelInterface(ctx context.Context, network string, destination M.Socksaddr, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.Conn, error) {
	return d.DialContext(ctx, network, destination)
}

func (d *sourceBindTestDialer) ListenSerialInterfacePacket(ctx context.Context, destination M.Socksaddr, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.PacketConn, error) {
	return d.ListenPacket(ctx, destination)
}

type sourceBindTestClock struct{ now time.Time }

func (c *sourceBindTestClock) Now() time.Time { return c.now }

// sequenceRandom yields a deterministic, well-spread sequence.
func sequenceRandom() func() uint64 {
	var state uint64 = 0x9e3779b97f4a7c15
	return func() uint64 {
		state ^= state << 13
		state ^= state >> 7
		state ^= state << 17
		return state
	}
}

func prefixes(values ...string) badoption.Listable[*badoption.Prefixable] {
	var result badoption.Listable[*badoption.Prefixable]
	for _, value := range values {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			addr := netip.MustParseAddr(value)
			prefix = netip.PrefixFrom(addr, addr.BitLen())
		}
		result = append(result, (*badoption.Prefixable)(&prefix))
	}
	return result
}

type sourceBindTestHarness struct {
	clock  *sourceBindTestClock
	mapper *sourceBindMapper
	dialer *sourceBindDialer
	base   *sourceBindTestDialer
	dials  []*sourceBindTestDialer
}

func newSourceBindTestHarness(t *testing.T, options option.SourceBindOptions) *sourceBindTestHarness {
	t.Helper()
	harness := &sourceBindTestHarness{clock: &sourceBindTestClock{now: time.Unix(1_700_000_000, 0)}}
	harness.base = &sourceBindTestDialer{dials: &harness.dials}
	mapper, err := newSourceBindMapper(nil, options, func(inet4 netip.Addr, inet6 netip.Addr) dialer.ParallelInterfaceDialer {
		return &sourceBindTestDialer{inet4: inet4, inet6: inet6, dials: &harness.dials}
	})
	require.NoError(t, err)
	mapper.now = harness.clock.Now
	mapper.random = sequenceRandom()
	harness.mapper = mapper
	harness.dialer = &sourceBindDialer{base: harness.base, mapper: mapper}
	return harness
}

// dial opens a TCP connection for source and returns the dialer that served it.
func (h *sourceBindTestHarness) dial(t *testing.T, source string) *sourceBindTestDialer {
	t.Helper()
	ctx := context.Background()
	if source != "" {
		ctx = adapter.WithContext(ctx, &adapter.InboundContext{Source: M.ParseSocksaddrHostPort(source, 40000)})
	}
	_, err := h.dialer.DialContext(ctx, N.NetworkTCP, M.ParseSocksaddrHostPort("198.18.0.1", 443))
	require.NoError(t, err)
	return h.dials[len(h.dials)-1]
}

func TestSourceBindRulePrecedence(t *testing.T) {
	t.Parallel()

	// Given a default pool and a rule with a fixed IPv4 and an IPv6 prefix
	harness := newSourceBindTestHarness(t, option.SourceBindOptions{
		Inet4Addresses: prefixes("203.0.113.0/28"),
		Inet6Addresses: prefixes("2001:db8:1::/64"),
		Rules: []option.SourceBindRule{{
			SourceIPCIDR:   prefixes("10.0.1.0/24", "10.0.2.5"),
			Inet4Addresses: prefixes("198.51.100.9"),
			Inet6Addresses: prefixes("2001:db8:2::/120"),
		}},
	})

	// When clients inside and outside the rule dial
	matched := harness.dial(t, "10.0.1.77")
	single := harness.dial(t, "10.0.2.5")
	unmatched := harness.dial(t, "10.0.3.1")

	// Then the rule wins for its sources and the pool serves everyone else
	require.Equal(t, netip.MustParseAddr("198.51.100.9"), matched.inet4)
	require.True(t, netip.MustParsePrefix("2001:db8:2::/120").Contains(matched.inet6))
	require.Equal(t, netip.MustParseAddr("198.51.100.9"), single.inet4)
	require.True(t, netip.MustParsePrefix("203.0.113.0/28").Contains(unmatched.inet4))
	require.True(t, netip.MustParsePrefix("2001:db8:1::/64").Contains(unmatched.inet6))
}

func TestSourceBindStablePerClient(t *testing.T) {
	t.Parallel()

	harness := newSourceBindTestHarness(t, option.SourceBindOptions{
		Inet6Addresses: prefixes("2001:db8:1::/64"),
	})
	first := harness.dial(t, "10.0.0.1")
	other := harness.dial(t, "10.0.0.2")
	for i := 0; i < 10; i++ {
		harness.clock.now = harness.clock.now.Add(time.Minute)
		require.Same(t, first, harness.dial(t, "10.0.0.1"))
		require.Same(t, other, harness.dial(t, "10.0.0.2"))
	}
	require.NotEqual(t, first.inet6, other.inet6)
	// IPv4-mapped IPv6 sources are the same client as the IPv4 source.
	require.Same(t, first, harness.dial(t, "::ffff:10.0.0.1"))
}

func TestSourceBindSlidingExpiry(t *testing.T) {
	t.Parallel()

	// Given the default one hour TTL
	harness := newSourceBindTestHarness(t, option.SourceBindOptions{
		Inet6Addresses: prefixes("2001:db8:1::/64"),
	})
	start := harness.clock.now
	first := harness.dial(t, "10.0.0.1")

	// When the client keeps opening connections less than an hour apart,
	// even though more than an hour passes in total
	for _, offset := range []time.Duration{50 * time.Minute, 100 * time.Minute, 150 * time.Minute} {
		harness.clock.now = start.Add(offset)
		require.Same(t, first, harness.dial(t, "10.0.0.1"), "offset %s", offset)
	}

	// Then after an hour of inactivity it is reassigned
	harness.clock.now = start.Add(150*time.Minute + time.Hour)
	second := harness.dial(t, "10.0.0.1")
	require.NotSame(t, first, second)
	require.NotEqual(t, first.inet6, second.inet6)
}

func TestSourceBindSweepsIdleClients(t *testing.T) {
	t.Parallel()

	harness := newSourceBindTestHarness(t, option.SourceBindOptions{
		Inet4Addresses: prefixes("203.0.113.0/24"),
		TTL:            badoption.Duration(10 * time.Minute),
	})
	for i := 1; i <= 50; i++ {
		harness.dial(t, netip.AddrFrom4([4]byte{10, 0, 0, byte(i)}).String())
	}
	require.Len(t, harness.mapper.entries, 50)
	harness.clock.now = harness.clock.now.Add(11 * time.Minute)
	harness.dial(t, "10.0.1.1")
	require.Len(t, harness.mapper.entries, 1)
}

func TestSourceBindFallsBackToBase(t *testing.T) {
	t.Parallel()

	// Given a rule that only binds IPv6 and no default pool
	harness := newSourceBindTestHarness(t, option.SourceBindOptions{
		Rules: []option.SourceBindRule{{
			SourceIPCIDR:   prefixes("10.0.0.0/8"),
			Inet6Addresses: prefixes("2001:db8::5"),
		}},
	})

	// Then a matched client binds IPv6 only and leaves IPv4 to the base
	matched := harness.dial(t, "10.1.2.3")
	require.False(t, matched.inet4.IsValid())
	require.Equal(t, netip.MustParseAddr("2001:db8::5"), matched.inet6)

	// And unmatched clients and dials without a client use the base dialer
	require.Same(t, harness.base, harness.dial(t, "192.168.1.1"))
	require.Same(t, harness.base, harness.dial(t, "192.168.1.1"))
	require.Same(t, harness.base, harness.dial(t, ""))
}

func TestSourceBindListenPacket(t *testing.T) {
	t.Parallel()

	harness := newSourceBindTestHarness(t, option.SourceBindOptions{Inet4Addresses: prefixes("192.0.2.7")})
	tcp := harness.dial(t, "10.0.0.1")
	ctx := adapter.WithContext(context.Background(), &adapter.InboundContext{Source: M.ParseSocksaddrHostPort("10.0.0.1", 1)})
	_, err := harness.dialer.ListenPacket(ctx, M.ParseSocksaddrHostPort("198.18.0.1", 53))
	require.NoError(t, err)
	require.Same(t, tcp, harness.dials[len(harness.dials)-1])
}

func TestAddressPoolSkipsReservedHosts(t *testing.T) {
	t.Parallel()

	random := sequenceRandom()
	for _, testCase := range []struct {
		prefix   string
		reserved []string
	}{
		{"192.0.2.0/30", []string{"192.0.2.0", "192.0.2.3"}},
		{"2001:db8::/126", []string{"2001:db8::"}},
	} {
		prefix := netip.MustParsePrefix(testCase.prefix)
		seen := make(map[netip.Addr]bool)
		for i := 0; i < 500; i++ {
			address := pickPrefix(prefix, random)
			require.True(t, prefix.Contains(address), address)
			seen[address] = true
		}
		for _, reserved := range testCase.reserved {
			require.False(t, seen[netip.MustParseAddr(reserved)], "%s picked %s", testCase.prefix, reserved)
		}
		require.Len(t, seen, int(prefixWeight(prefix)), testCase.prefix)
	}
	// /31 and /127 have no reserved host part.
	require.Equal(t, float64(2), prefixWeight(netip.MustParsePrefix("192.0.2.0/31")))
	require.Equal(t, float64(2), prefixWeight(netip.MustParsePrefix("2001:db8::/127")))
}

func TestAddressPoolWeightedByPrefixSize(t *testing.T) {
	t.Parallel()

	pool := newAddressPool(prefixes("192.0.2.1", "198.51.100.0/24"))
	random := sequenceRandom()
	single := 0
	for i := 0; i < 10000; i++ {
		if pool.pick(random) == netip.MustParseAddr("192.0.2.1") {
			single++
		}
	}
	// One address out of 255 usable: expect about 39 hits.
	require.Less(t, single, 120)
	require.Greater(t, single, 5)
}
