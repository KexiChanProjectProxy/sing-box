package direct

import (
	"context"
	"math"
	"math/rand/v2"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
)

const defaultSourceBindTTL = time.Hour

// addressPool picks random addresses from a set of prefixes, weighted by the
// number of usable host addresses in each prefix.
type addressPool struct {
	prefixes   []netip.Prefix
	cumulative []float64
}

func newAddressPool(addresses []*badoption.Prefixable) addressPool {
	var pool addressPool
	var total float64
	for _, address := range addresses {
		prefix := netip.Prefix(*address).Masked()
		if prefix.Addr().Is4In6() {
			continue
		}
		total += prefixWeight(prefix)
		pool.prefixes = append(pool.prefixes, prefix)
		pool.cumulative = append(pool.cumulative, total)
	}
	return pool
}

// prefixWeight is the number of addresses pickPrefix can return.
func prefixWeight(prefix netip.Prefix) float64 {
	hostBits := prefix.Addr().BitLen() - prefix.Bits()
	weight := math.Ldexp(1, hostBits)
	if hostBits >= 2 {
		if prefix.Addr().Is4() {
			weight -= 2
		} else {
			weight -= 1
		}
	}
	return weight
}

func (p addressPool) pick(random func() uint64) netip.Addr {
	if len(p.prefixes) == 0 {
		return netip.Addr{}
	}
	index := 0
	if len(p.prefixes) > 1 {
		total := p.cumulative[len(p.cumulative)-1]
		target := float64(random()>>11) / (1 << 53) * total
		for index < len(p.cumulative)-1 && target >= p.cumulative[index] {
			index++
		}
	}
	return pickPrefix(p.prefixes[index], random)
}

// pickPrefix returns a random address inside prefix. For IPv4 prefixes with
// at least two host bits the network and broadcast addresses are skipped; for
// IPv6 the all-zero Subnet-Router anycast address is skipped.
func pickPrefix(prefix netip.Prefix, random func() uint64) netip.Addr {
	base := prefix.Addr()
	hostBits := base.BitLen() - prefix.Bits()
	if hostBits == 0 {
		return base
	}
	for attempt := 0; attempt < 64; attempt++ {
		address, isReserved := randomHost(prefix, random)
		if !isReserved || hostBits < 2 {
			return address
		}
	}
	return base.Next()
}

func randomHost(prefix netip.Prefix, random func() uint64) (netip.Addr, bool) {
	base := prefix.Addr()
	bytes := base.As16()
	start := 0
	if base.Is4() {
		start = 12
	}
	var noise [16]byte
	for i := 0; i < 16; i += 8 {
		value := random()
		for j := 0; j < 8; j++ {
			noise[i+j] = byte(value >> (8 * j))
		}
	}
	prefixBits := prefix.Bits()
	allZero, allOnes := true, true
	for i := start; i < 16; i++ {
		bitOffset := (i - start) * 8
		var hostMask byte
		switch {
		case bitOffset+8 <= prefixBits:
			hostMask = 0
		case bitOffset >= prefixBits:
			hostMask = 0xff
		default:
			hostMask = 0xff >> (prefixBits - bitOffset)
		}
		host := noise[i] & hostMask
		bytes[i] |= host
		if host != 0 {
			allZero = false
		}
		if host != hostMask {
			allOnes = false
		}
	}
	var address netip.Addr
	if base.Is4() {
		address = netip.AddrFrom4([4]byte(bytes[12:]))
	} else {
		address = netip.AddrFrom16(bytes)
	}
	if base.Is4() {
		return address, allZero || allOnes
	}
	return address, allZero
}

type sourceBindRule struct {
	cidrs []netip.Prefix
	pool4 addressPool
	pool6 addressPool
}

func (r sourceBindRule) match(source netip.Addr) bool {
	for _, cidr := range r.cidrs {
		if cidr.Contains(source) {
			return true
		}
	}
	return false
}

type sourceBinding struct {
	inet4    netip.Addr
	inet6    netip.Addr
	dialer   dialer.ParallelInterfaceDialer
	lastSeen time.Time
}

// sourceBindMapper assigns bind addresses to client source addresses. The
// first matching rule wins; unmatched clients use the default pools. A
// client's assignment is kept while it opens new connections less than ttl
// apart, and is replaced by a fresh random pick after ttl of inactivity.
type sourceBindMapper struct {
	logger    log.ContextLogger
	rules     []sourceBindRule
	pool4     addressPool
	pool6     addressPool
	ttl       time.Duration
	now       func() time.Time
	random    func() uint64
	newDialer func(inet4 netip.Addr, inet6 netip.Addr) dialer.ParallelInterfaceDialer

	access    sync.Mutex
	entries   map[netip.Addr]*sourceBinding
	nextSweep time.Time
}

func newSourceBindMapper(logger log.ContextLogger, options option.SourceBindOptions, newDialer func(inet4 netip.Addr, inet6 netip.Addr) dialer.ParallelInterfaceDialer) (*sourceBindMapper, error) {
	err := options.Check()
	if err != nil {
		return nil, err
	}
	ttl := time.Duration(options.TTL)
	if ttl == 0 {
		ttl = defaultSourceBindTTL
	}
	mapper := &sourceBindMapper{
		logger:    logger,
		pool4:     newAddressPool(options.Inet4Addresses),
		pool6:     newAddressPool(options.Inet6Addresses),
		ttl:       ttl,
		now:       time.Now,
		random:    rand.Uint64,
		newDialer: newDialer,
		entries:   make(map[netip.Addr]*sourceBinding),
	}
	for _, ruleOptions := range options.Rules {
		rule := sourceBindRule{
			pool4: newAddressPool(ruleOptions.Inet4Addresses),
			pool6: newAddressPool(ruleOptions.Inet6Addresses),
		}
		for _, cidr := range ruleOptions.SourceIPCIDR {
			prefix := netip.Prefix(*cidr)
			if prefix.Addr().Is4In6() {
				prefix = netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-96)
			}
			rule.cidrs = append(rule.cidrs, prefix.Masked())
		}
		mapper.rules = append(mapper.rules, rule)
	}
	return mapper, nil
}

// bindingFor returns the binding for source, or nil when neither family has
// an address to bind to.
func (m *sourceBindMapper) bindingFor(ctx context.Context, source netip.Addr) *sourceBinding {
	source = source.Unmap()
	now := m.now()
	m.access.Lock()
	defer m.access.Unlock()
	if !now.Before(m.nextSweep) {
		for address, binding := range m.entries {
			if now.Sub(binding.lastSeen) >= m.ttl {
				delete(m.entries, address)
			}
		}
		m.nextSweep = now.Add(m.ttl / 2)
	}
	binding, loaded := m.entries[source]
	if loaded && now.Sub(binding.lastSeen) < m.ttl {
		binding.lastSeen = now
		if binding.dialer == nil {
			return nil
		}
		return binding
	}
	pool4, pool6 := m.pool4, m.pool6
	for _, rule := range m.rules {
		if rule.match(source) {
			pool4, pool6 = rule.pool4, rule.pool6
			break
		}
	}
	binding = &sourceBinding{
		inet4:    pool4.pick(m.random),
		inet6:    pool6.pick(m.random),
		lastSeen: now,
	}
	if binding.inet4.IsValid() || binding.inet6.IsValid() {
		binding.dialer = m.newDialer(binding.inet4, binding.inet6)
	}
	m.entries[source] = binding
	if m.logger != nil {
		m.logger.DebugEventContext(ctx, "direct.source_bind", "assign bind address",
			log.String("source", source.String()),
			log.String("inet4", addrString(binding.inet4)),
			log.String("inet6", addrString(binding.inet6)),
		)
	}
	if binding.dialer == nil {
		return nil
	}
	return binding
}

func addrString(address netip.Addr) string {
	if !address.IsValid() {
		return "default"
	}
	return address.String()
}

// sourceBindDialer dials through a dialer bound to the address assigned to
// the connection's client source address, or through base when the
// connection has no client (internal dials) or no address is assigned.
type sourceBindDialer struct {
	base   dialer.ParallelInterfaceDialer
	mapper *sourceBindMapper
}

func (d *sourceBindDialer) dialerFor(ctx context.Context) dialer.ParallelInterfaceDialer {
	metadata := adapter.ContextFrom(ctx)
	if metadata == nil || !metadata.Source.Addr.IsValid() {
		return d.base
	}
	binding := d.mapper.bindingFor(ctx, metadata.Source.Addr)
	if binding == nil {
		return d.base
	}
	return binding.dialer
}

func (d *sourceBindDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return d.dialerFor(ctx).DialContext(ctx, network, destination)
}

func (d *sourceBindDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return d.dialerFor(ctx).ListenPacket(ctx, destination)
}

func (d *sourceBindDialer) DialParallelInterface(ctx context.Context, network string, destination M.Socksaddr, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.Conn, error) {
	return d.dialerFor(ctx).DialParallelInterface(ctx, network, destination, strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
}

func (d *sourceBindDialer) ListenSerialInterfacePacket(ctx context.Context, destination M.Socksaddr, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.PacketConn, error) {
	return d.dialerFor(ctx).ListenSerialInterfacePacket(ctx, destination, strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
}

func (d *sourceBindDialer) Upstream() any {
	return d.base
}

var _ dialer.ParallelInterfaceDialer = (*sourceBindDialer)(nil)
