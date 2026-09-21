package group

import (
	"context"
	"math/rand"
	"net"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/interrupt"
	"github.com/sagernet/sing-box/common/transportstats"
	"github.com/sagernet/sing-box/common/urltest"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/route"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

func RegisterLoadBalance(registry *outbound.Registry) {
	outbound.Register(registry, C.TypeLoadBalance, NewLoadBalance)
}

var (
	_ adapter.OutboundGroup            = (*LoadBalance)(nil)
	_ adapter.URLTestGroup             = (*LoadBalance)(nil)
	_ adapter.OutboundWithPreferDomain = (*LoadBalance)(nil)
)

type LoadBalance struct {
	outbound.Adapter
	ctx                          context.Context
	outbound                     adapter.OutboundManager
	connection                   adapter.ConnectionManager
	logger                       log.StructuredLogger
	tags                         []string
	primaryTags                  []string
	backupTags                   []string
	primaryOutbounds             map[string]adapter.Outbound
	backupOutbounds              map[string]adapter.Outbound
	url                          string
	interval                     time.Duration
	timeout                      time.Duration
	idleTimeout                  time.Duration
	tolerance                    uint16
	topNPrimary                  int
	strategy                     string
	hash                         *option.LoadBalanceHashOptions
	emptyPoolAction              string
	interruptGroup               *interrupt.Group
	interruptExternalConnections bool
	preferDomain                 bool
	overrideIP                   *option.OverrideIPOptions
	history                      *urltest.HistoryStorage
	group                        *URLTestGroup
	snapshot                     atomic.Pointer[CandidateSnapshot]
	// rebuildMu serialises snapshot rebuilds, which read the current snapshot
	// and store a successor derived from it.
	rebuildMu      sync.Mutex
	sorter         *sorter
	latencyStats   map[string]*transportstats.Recorder
	latencyStatsMu sync.Mutex
	// latencyRecorderFactory overrides how history recorders are built, so that
	// tests can drive the window with their own clock. nil uses the wall clock.
	latencyRecorderFactory func() *transportstats.Recorder
	// statsEnabled is set once in Start and read without locking afterwards.
	statsEnabled bool
}

func NewLoadBalance(ctx context.Context, router adapter.Router, logger log.StructuredLogger, tag string, options option.LoadBalanceOutboundOptions) (adapter.Outbound, error) {
	err := options.Check()
	if err != nil {
		return nil, err
	}
	allOutbounds := append(options.PrimaryOutbounds, options.BackupOutbounds...)
	if common.Contains(allOutbounds, tag) {
		return nil, E.New("loadbalance tag ", tag, " appears in primary_outbounds or backup_outbounds (self-reference)")
	}
	allTags := append(options.PrimaryOutbounds, options.BackupOutbounds...)
	lb := &LoadBalance{
		Adapter:                      outbound.NewAdapter(C.TypeLoadBalance, tag, []string{N.NetworkTCP, N.NetworkUDP}, allTags),
		ctx:                          ctx,
		outbound:                     service.FromContext[adapter.OutboundManager](ctx),
		connection:                   service.FromContext[adapter.ConnectionManager](ctx),
		logger:                       logger,
		tags:                         allTags,
		primaryTags:                  options.PrimaryOutbounds,
		backupTags:                   options.BackupOutbounds,
		primaryOutbounds:             make(map[string]adapter.Outbound),
		backupOutbounds:              make(map[string]adapter.Outbound),
		url:                          options.URL,
		interval:                     time.Duration(options.Interval),
		timeout:                      time.Duration(options.Timeout),
		idleTimeout:                  time.Duration(options.IdleTimeout),
		strategy:                     options.Strategy,
		hash:                         options.Hash,
		emptyPoolAction:              options.EmptyPoolAction,
		interruptGroup:               interrupt.NewGroup(),
		interruptExternalConnections: options.InterruptExistConnections,
		preferDomain:                 options.PreferDomain,
		overrideIP:                   options.OverrideIP,
	}
	if lb.timeout == 0 {
		lb.timeout = C.TCPTimeout
	}
	tolerance := options.Tolerance
	if tolerance == 0 {
		tolerance = defaultLoadBalanceTolerance
	}
	lb.tolerance = tolerance
	if options.TopN != nil {
		lb.topNPrimary = options.TopN.Primary
	}
	memberSorter, err := newSorter(options.SorterWeights())
	if err != nil {
		return nil, err
	}
	lb.sorter = memberSorter
	if memberSorter.usesLatencyWindow {
		lb.latencyStats = make(map[string]*transportstats.Recorder)
	}
	if options.WeightedDelay != nil {
		logger.WarnEvent("loadbalance.weighted_delay.deprecated",
			"weighted_delay is deprecated, use sorter instead",
			log.String("tag", tag))
	}
	lb.warnCoarseProbeInterval(logger, tag)
	return lb, nil
}

// warnCoarseProbeInterval reports a probe interval too coarse for the latency
// windows the sorter averages over. With an interval more than half the window,
// the window holds at most two samples, or none, in which case the latest delay
// is read instead, so the key behaves much like the latest delay.
func (l *LoadBalance) warnCoarseProbeInterval(logger log.StructuredLogger, tag string) {
	window := l.memberSorter().narrowestLatencyWindow
	if window == 0 {
		return
	}
	interval := l.interval
	if interval == 0 {
		interval = C.DefaultURLTestInterval
	}
	if interval*2 <= window {
		return
	}
	logger.WarnEvent("loadbalance.sorter.coarse_interval",
		"probe interval is too coarse for the sorter's latency window, which holds too few samples for a meaningful average",
		log.String("tag", tag),
		log.Duration("interval", interval),
		log.Duration("window", window))
}

func (l *LoadBalance) Start() error {
	outbounds := make([]adapter.Outbound, 0, len(l.tags))
	for i, tag := range l.primaryTags {
		detour, loaded := l.outbound.Outbound(tag)
		if !loaded {
			return E.New("primary outbound ", i, " not found: ", tag)
		}
		l.primaryOutbounds[tag] = detour
		outbounds = append(outbounds, detour)
	}
	for i, tag := range l.backupTags {
		detour, loaded := l.outbound.Outbound(tag)
		if !loaded {
			return E.New("backup outbound ", i, " not found: ", tag)
		}
		l.backupOutbounds[tag] = detour
		outbounds = append(outbounds, detour)
	}
	group, err := NewURLTestGroup(l.ctx, l.outbound, l.logger, outbounds, l.url, l.interval, 0, l.idleTimeout, false)
	if err != nil {
		return err
	}
	group.updateCallback = l.rebuildSnapshot
	group.dataPathLatency = true
	group.memberProbe = l.memberProbe
	group.afterProbe = func(tag string, delay uint16, ok bool) {
		if !ok {
			l.resetWindow(tag)
			return
		}
		l.observeDelay(tag, delay)
	}
	l.group = group
	l.history = group.history
	l.enableTransportStats()
	l.seedInitialSnapshot()
	return nil
}

// maxGroupDepth bounds how far the group walks into nested groups, so that a
// configuration cycle cannot hang startup or scoring.
const maxGroupDepth = 16

// enableTransportStats asks every outbound reachable from this group to start
// collecting the directions the sorter ranks on. It descends into nested groups,
// because a nested member carries traffic through one of its own members and it
// is that outbound which owns the connection being measured.
//
// Members are already started by the time a group starts, so enabling is
// expected to take effect on the connections an outbound opens from now on.
func (l *LoadBalance) enableTransportStats() {
	directions := l.memberSorter().directions
	if len(directions) == 0 {
		return
	}
	l.statsEnabled = true
	seen := make(map[string]int)
	var enabled int
	for _, outbounds := range []map[string]adapter.Outbound{l.primaryOutbounds, l.backupOutbounds} {
		for _, detour := range outbounds {
			enabled += l.enableTransportStatsOn(detour, directions, seen, 0)
		}
	}
	if enabled == 0 {
		l.logger.WarnEvent("loadbalance.sorter.unsupported",
			"sorter reads transport statistics but no member reports them",
			log.String("tag", l.Tag()))
	}
}

// enableTransportStatsOn enables collection on detour, or on every outbound it
// resolves to when it is a group, and returns how many were enabled.
//
// seen records the shallowest depth each outbound was visited at. An outbound
// reached again at a shallower depth is walked again, because the earlier visit
// may have been cut short by maxGroupDepth before reaching its members.
func (l *LoadBalance) enableTransportStatsOn(detour adapter.Outbound, directions []adapter.TransportStatsDirection, seen map[string]int, depth int) int {
	if detour == nil || depth >= maxGroupDepth {
		return 0
	}
	previousDepth, visited := seen[detour.Tag()]
	if visited && previousDepth <= depth {
		return 0
	}
	seen[detour.Tag()] = depth
	if member, isStatsMember := detour.(adapter.OutboundWithTransportStats); isStatsMember {
		if visited {
			// Already enabled on the deeper visit; enabling is idempotent but the
			// count must not include it twice.
			return 0
		}
		for _, direction := range directions {
			member.EnableTransportStats(direction)
		}
		return 1
	}
	group, isGroup := detour.(adapter.OutboundGroup)
	if !isGroup || l.outbound == nil {
		return 0
	}
	var enabled int
	for _, tag := range group.All() {
		if nested, loaded := l.outbound.Outbound(tag); loaded {
			enabled += l.enableTransportStatsOn(nested, directions, seen, depth+1)
		}
	}
	return enabled
}

// resolveStatsMember follows a member down to the outbound currently carrying
// its traffic and returns it when its protocol reports transport statistics.
//
// A nested group is resolved through its current selection. For a selector or
// urltest that is exactly the outbound the traffic uses. A nested loadbalance
// spreads its traffic over a whole pool, so its best ranked member stands in
// for the pool and the statistics describe only that part of it.
func (l *LoadBalance) resolveStatsMember(detour adapter.Outbound) adapter.OutboundWithTransportStats {
	for depth := 0; depth < maxGroupDepth; depth++ {
		if detour == nil {
			return nil
		}
		if member, isStatsMember := detour.(adapter.OutboundWithTransportStats); isStatsMember {
			return member
		}
		group, isGroup := detour.(adapter.OutboundGroup)
		if !isGroup || l.outbound == nil {
			return nil
		}
		next, loaded := l.outbound.Outbound(group.Now())
		if !loaded || next == detour {
			return nil
		}
		detour = next
	}
	return nil
}

func (l *LoadBalance) PostStart() error {
	l.group.PostStart()
	return nil
}

func (l *LoadBalance) Close() error {
	return common.Close(
		common.PtrOrNil(l.group),
	)
}

func (l *LoadBalance) Now() string {
	snapshot := l.snapshot.Load()
	if snapshot != nil && len(snapshot.Candidates) > 0 {
		return snapshot.Candidates[0].Tag
	}
	if len(l.primaryTags) > 0 {
		return l.primaryTags[0]
	}
	if len(l.backupTags) > 0 {
		return l.backupTags[0]
	}
	return ""
}

func (l *LoadBalance) All() []string {
	return l.tags
}

func (l *LoadBalance) PreferDomain() bool {
	return l.preferDomain
}

func (l *LoadBalance) OverrideIP() *option.OverrideIPOptions {
	return l.overrideIP
}

func (l *LoadBalance) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	networkName := N.NetworkName(network)
	if networkName != N.NetworkTCP && networkName != N.NetworkUDP {
		return nil, E.Extend(N.ErrUnknownNetwork, network)
	}
	l.group.Touch()
	metadata := loadBalanceMetadata(ctx)
	metadata.Network = networkName
	metadata.Destination = destination
	selected, found, err := l.pickCandidate(metadata)
	if err != nil {
		return nil, err
	}
	ctx = l.memberDialContext(ctx)
	var conn net.Conn
	_, err = l.walk(ctx, selected, found, networkName, func(ctx context.Context, detour adapter.Outbound) error {
		var dialErr error
		conn, dialErr = detour.DialContext(ctx, network, destination)
		return dialErr
	})
	if err != nil {
		return nil, err
	}
	return l.interruptGroup.NewConn(conn, interrupt.IsExternalConnectionFromContext(ctx)), nil
}

func (l *LoadBalance) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	l.group.Touch()
	metadata := loadBalanceMetadata(ctx)
	metadata.Network = N.NetworkUDP
	metadata.Destination = destination
	selected, found, err := l.pickCandidate(metadata)
	if err != nil {
		return nil, err
	}
	ctx = l.memberDialContext(ctx)
	var conn net.PacketConn
	_, err = l.walk(ctx, selected, found, N.NetworkUDP, func(ctx context.Context, detour adapter.Outbound) error {
		var listenErr error
		conn, listenErr = detour.ListenPacket(ctx, destination)
		return listenErr
	})
	if err != nil {
		return nil, err
	}
	return l.interruptGroup.NewPacketConn(conn, interrupt.IsExternalConnectionFromContext(ctx)), nil
}

func (l *LoadBalance) memberDialContext(ctx context.Context) context.Context {
	if l.preferDomain || adapter.PreferDomainFromContext(ctx) {
		ctx = adapter.ContextWithPreferDomain(ctx, true)
	}
	return withOverrideIPContext(ctx, l.overrideIP)
}

// walk dials the strategy's first choice, then every remaining configured
// primary and backup ordered by last measured delay, until one succeeds.
// Global state (snapshot, hash ring, interrupt group) is left untouched.
func (l *LoadBalance) walk(ctx context.Context, selected Candidate, found bool, network string, dial func(context.Context, adapter.Outbound) error) (Candidate, error) {
	var selectedPtr *Candidate
	if found {
		selectedPtr = &selected
	}
	attempts := l.failoverAttempts(selectedPtr, network)
	used, tried, err := l.dialAttempts(ctx, attempts, dial)
	if err != nil {
		if len(tried) > 1 {
			l.logger.ErrorEventContext(ctx, "loadbalance.failover.exhausted", "loadbalance fail-over exhausted",
				log.String("selected", selected.Tag),
				log.String("tried", strings.Join(tried, ",")),
				log.Err(err))
		}
		return Candidate{}, err
	}
	if !found || used.Tag != selected.Tag {
		l.logger.InfoEventContext(ctx, "loadbalance.failover", "loadbalance fail-over",
			log.String("selected", selected.Tag),
			log.String("used", used.Tag),
			log.String("tried", strings.Join(tried, ",")))
	}
	return used, nil
}

// failoverAttempts builds the frozen per-connection attempt order: selected
// first, then untried primaries by last raw delay, then untried backups by last
// raw delay. Members that do not support network are skipped.
func (l *LoadBalance) failoverAttempts(selected *Candidate, network string) []Candidate {
	tried := make(map[string]struct{}, len(l.tags))
	attempts := make([]Candidate, 0, len(l.primaryTags)+len(l.backupTags))
	if selected != nil && selected.Outbound != nil && common.Contains(selected.Outbound.Network(), network) {
		attempts = append(attempts, *selected)
		tried[selected.Tag] = struct{}{}
	}
	attempts = append(attempts, l.poolByLastDelay(l.primaryTags, l.primaryOutbounds, true, network, tried)...)
	attempts = append(attempts, l.poolByLastDelay(l.backupTags, l.backupOutbounds, false, network, tried)...)
	return attempts
}

func (l *LoadBalance) poolByLastDelay(tags []string, outbounds map[string]adapter.Outbound, isPrimary bool, network string, tried map[string]struct{}) []Candidate {
	var measured, unmeasured []Candidate
	for _, tag := range tags {
		if _, done := tried[tag]; done {
			continue
		}
		detour := outbounds[tag]
		if detour == nil || !common.Contains(detour.Network(), network) {
			continue
		}
		tried[tag] = struct{}{}
		candidate := Candidate{Tag: tag, Outbound: detour, IsPrimary: isPrimary}
		if delay, ok := l.liveMemberDelay(tag, detour); ok && delay != 0 {
			candidate.Latency = delay
			measured = append(measured, candidate)
		} else {
			unmeasured = append(unmeasured, candidate)
		}
	}
	sortCandidatesByLatency(measured)
	sortCandidatesByLatency(unmeasured)
	return append(measured, unmeasured...)
}

// dialAttempts tries each candidate once, in order. It returns the successful
// candidate and the tags attempted (including the successful one).
func (l *LoadBalance) dialAttempts(ctx context.Context, attempts []Candidate, dial func(context.Context, adapter.Outbound) error) (Candidate, []string, error) {
	if len(attempts) == 0 {
		return Candidate{}, nil, l.emptyPoolError()
	}
	tried := make([]string, 0, len(attempts))
	var lastErr error
	for _, candidate := range attempts {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Candidate{}, tried, ctxErr
		}
		tried = append(tried, candidate.Tag)
		err := dial(ctx, candidate.Outbound)
		if err == nil {
			return candidate, tried, nil
		}
		realTag := RealTag(candidate.Outbound)
		l.logger.ErrorEventContext(ctx, "urltest.error", "urltest error", log.Err(err), log.String("tag", realTag))
		l.history.DeleteURLTestHistory(realTag)
		l.resetWindow(candidate.Tag)
		lastErr = err
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return Candidate{}, tried, ctxErr
	}
	return Candidate{}, tried, lastErr
}

func (l *LoadBalance) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	ctx = interrupt.ContextWithIsExternalConnection(ctx)
	if l.preferDomain || adapter.PreferDomainFromContext(ctx) {
		ctx = route.ApplyPreferDomain(ctx, &metadata, l)
	}
	var err error
	ctx, err = applyGroupOverrideIP(ctx, &metadata, l, l.overrideIP, l.ctx)
	if err != nil {
		N.CloseOnHandshakeFailure(conn, onClose, err)
		l.logger.ErrorEventContext(ctx, "outbound.override_ip.error", "override_ip failed", log.Err(err))
		return
	}
	l.connection.NewConnection(ctx, l, conn, metadata, onClose)
}

func (l *LoadBalance) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	ctx = interrupt.ContextWithIsExternalConnection(ctx)
	if l.preferDomain || adapter.PreferDomainFromContext(ctx) {
		ctx = route.ApplyPreferDomain(ctx, &metadata, l)
	}
	var err error
	ctx, err = applyGroupOverrideIP(ctx, &metadata, l, l.overrideIP, l.ctx)
	if err != nil {
		_ = conn.Close()
		if onClose != nil {
			onClose(err)
		}
		l.logger.ErrorEventContext(ctx, "outbound.override_ip.error", "override_ip failed", log.Err(err))
		return
	}
	l.connection.NewPacketConnection(ctx, l, conn, metadata, onClose)
}

func (l *LoadBalance) URLTest(ctx context.Context) (map[string]uint16, error) {
	return l.group.URLTest(ctx)
}

func (l *LoadBalance) CheckOutbounds() {
	l.group.CheckOutbounds(l.ctx, true)
}

func (l *LoadBalance) PerformUpdateCheck() {
	l.rebuildSnapshot()
}

func (l *LoadBalance) selectCandidate(metadata adapter.InboundContext) (Candidate, error) {
	candidate, found, err := l.pickCandidate(metadata)
	if err != nil {
		return Candidate{}, err
	}
	if !found {
		return Candidate{}, l.emptyPoolError()
	}
	return candidate, nil
}

// pickCandidate returns the strategy's first choice. found is false (with a nil
// error) when the snapshot is empty and empty_pool_action is error; strategy
// errors such as an empty hash key are returned as errors.
func (l *LoadBalance) pickCandidate(metadata adapter.InboundContext) (Candidate, bool, error) {
	snapshot := l.snapshot.Load()
	if snapshot == nil || len(snapshot.Candidates) == 0 {
		if l.emptyPoolAction == "random" {
			candidate, err := l.selectEmptyPoolFallback()
			if err != nil {
				return Candidate{}, false, nil
			}
			return candidate, true, nil
		}
		return Candidate{}, false, nil
	}
	if l.strategy == "random" {
		candidate, err := SelectRandomFromSnapshot(snapshot)
		if err != nil {
			return Candidate{}, false, E.Cause(err, "loadbalance")
		}
		return candidate, true, nil
	}
	key := l.computeHashKey(metadata)
	onEmptyKey := "random"
	virtualNodes := 0
	keySalt := ""
	if l.hash != nil {
		if l.hash.OnEmptyKey != "" {
			onEmptyKey = l.hash.OnEmptyKey
		}
		virtualNodes = l.hash.VirtualNodes
		keySalt = l.hash.KeySalt
	}
	candidate, err := SelectFromSnapshot(snapshot, key, onEmptyKey, virtualNodes, keySalt)
	if err != nil {
		return Candidate{}, false, E.Cause(err, "loadbalance")
	}
	return candidate, true, nil
}

func (l *LoadBalance) computeHashKey(metadata adapter.InboundContext) string {
	if l.hash == nil || len(l.hash.KeyParts) == 0 {
		return ""
	}
	parts := make([]string, 0, len(l.hash.KeyParts))
	for _, part := range l.hash.KeyParts {
		derived := route.DeriveHashKeyPart(metadata, part)
		if derived != "" {
			parts = append(parts, derived)
		}
	}
	return strings.Join(parts, "|")
}

// rebuildSnapshot re-ranks the members and republishes the candidate pool.
//
// It reads the previous snapshot, derives the next one from it and stores the
// result, so concurrent rebuilds are serialised: the Clash API can call it
// through PerformUpdateCheck while a health check round is calling it, and
// without this they could duplicate a generation, clobber each other's pool, or
// interrupt live connections twice.
func (l *LoadBalance) rebuildSnapshot() {
	l.rebuildMu.Lock()
	defer l.rebuildMu.Unlock()
	primaryCandidates := l.healthyCandidates(l.primaryTags, l.primaryOutbounds, true)
	snap := l.snapshot.Load()
	var previous []Candidate
	if snap != nil {
		previous = snap.Candidates
	}
	var candidates []Candidate
	if len(primaryCandidates) > 0 {
		candidates = selectTopNWithTolerance(primaryCandidates, l.topNPrimary, l.tolerance, previous)
	} else {
		candidates = l.healthyCandidates(l.backupTags, l.backupOutbounds, false)
	}
	if len(candidates) == 0 && snap != nil && snapshotAllZeroLatency(snap) && l.hasPendingSelfTestingMember() {
		return
	}
	if snap != nil && sameCandidateSet(snap.Candidates, candidates) {
		l.snapshot.Store(&CandidateSnapshot{
			Candidates: candidates,
			Generation: snap.Generation,
		})
		return
	}
	generation := uint64(1)
	if snap != nil {
		generation = snap.Generation + 1
	}
	l.snapshot.Store(&CandidateSnapshot{
		Candidates: candidates,
		Generation: generation,
	})
	if snap != nil {
		l.interruptGroup.Interrupt(l.interruptExternalConnections)
	}
}

func (l *LoadBalance) seedInitialSnapshot() {
	candidates := make([]Candidate, 0, len(l.primaryTags))
	for _, tag := range l.primaryTags {
		if detour := l.primaryOutbounds[tag]; detour != nil {
			candidates = append(candidates, Candidate{
				Tag:       tag,
				Outbound:  detour,
				IsPrimary: true,
			})
		}
	}
	if len(candidates) == 0 {
		for _, tag := range l.backupTags {
			if detour := l.backupOutbounds[tag]; detour != nil {
				candidates = append(candidates, Candidate{
					Tag:      tag,
					Outbound: detour,
				})
			}
		}
	}
	if len(candidates) == 0 {
		return
	}
	l.snapshot.Store(&CandidateSnapshot{
		Candidates: candidates,
		Generation: 1,
	})
}

func (l *LoadBalance) healthyCandidates(tags []string, outbounds map[string]adapter.Outbound, isPrimary bool) []Candidate {
	candidates := make([]Candidate, 0, len(tags))
	for _, tag := range tags {
		detour := outbounds[tag]
		if detour == nil {
			continue
		}
		delay, ok := l.liveMemberDelay(tag, detour)
		if !ok || delay == 0 || (l.timeout > 0 && time.Duration(delay)*time.Millisecond >= l.timeout) {
			l.resetWindow(tag)
			continue
		}
		candidates = append(candidates, Candidate{
			Tag:       tag,
			Outbound:  detour,
			Latency:   delay,
			IsPrimary: isPrimary,
		})
	}
	l.scoreCandidates(candidates)
	sortCandidatesByScore(candidates)
	return candidates
}

const defaultLoadBalanceTolerance uint16 = 10

// memberSorter returns the group's sorter, falling back to the latency only
// ranking so that a zero valued LoadBalance behaves like an unconfigured one.
func (l *LoadBalance) memberSorter() *sorter {
	if l.sorter == nil {
		return latencyOnlySorter
	}
	return l.sorter
}

// scoreCandidates fills in Score on every candidate from the configured sorter.
func (l *LoadBalance) scoreCandidates(candidates []Candidate) {
	breakdowns := l.memberSorter().score(candidates, func(candidate Candidate) metricSource {
		source := metricSource{
			latency:       candidate.Latency,
			latencyWindow: l.latencyRecorderIfExists(candidate.Tag),
		}
		if l.statsEnabled {
			if member := l.resolveStatsMember(candidate.Outbound); member != nil {
				source.client = member.TransportStats(adapter.TransportStatsClient)
				source.server = member.TransportStats(adapter.TransportStatsServer)
			}
		}
		return source
	})
	l.logScores(breakdowns)
}

// logScores explains each member's score, which is how a weight is tuned. The
// default sorter scores the raw delay, so it has nothing to explain.
func (l *LoadBalance) logScores(breakdowns []breakdown) {
	if l.logger == nil || l.memberSorter().isDefault || len(breakdowns) == 0 {
		return
	}
	for _, member := range breakdowns {
		fields := make([]log.Field, 0, len(member.Contributions)*2+2)
		fields = append(fields, log.String("tag", member.Tag), log.Float64("score", member.Score))
		for _, item := range member.Contributions {
			fields = append(fields, log.Float64(item.Key, item.Weighted))
			if !item.Measured {
				fields = append(fields, log.Bool(item.Key+".estimated", true))
			}
		}
		l.logger.DebugEvent("loadbalance.score", "loadbalance member score", fields...)
	}
}

// sortCandidatesByScore orders candidates for ranking: best score first, then
// lowest raw delay, then by tag so that the order is stable across rebuilds.
//
// The delay tie-break matters whenever a sorter cannot tell two members apart.
// A metric no member reports contributes the same pool average to every score,
// so a sorter built only from unreportable keys would otherwise leave the tag
// deciding the order and ignore latency entirely.
func sortCandidatesByScore(candidates []Candidate) {
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score < candidates[j].Score
		}
		if candidates[i].Latency != candidates[j].Latency {
			return candidates[i].Latency < candidates[j].Latency
		}
		return candidates[i].Tag < candidates[j].Tag
	})
}

// sortCandidatesByLatency orders candidates by raw measured delay. Fail-over
// uses it rather than the sorter, so that a dial that is already failing falls
// back on the simplest signal available.
func sortCandidatesByLatency(candidates []Candidate) {
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Latency != candidates[j].Latency {
			return candidates[i].Latency < candidates[j].Latency
		}
		return candidates[i].Tag < candidates[j].Tag
	})
}

// worstCandidateIndex returns the candidate an incumbent set would give up
// first, using the same ordering as sortCandidatesByScore.
func worstCandidateIndex(candidates []Candidate) int {
	worst := 0
	for i := 1; i < len(candidates); i++ {
		if rankAfter(candidates[i], candidates[worst]) {
			worst = i
		}
	}
	return worst
}

// outranksBeyondTolerance reports whether next is better than incumbent by more
// than tolerance, which is what it takes to displace an incumbent.
//
// Scores are compared first. When they tie, which happens whenever the sorter
// cannot tell the two apart, the raw delay decides with the same tolerance, so
// that a slow incumbent cannot hold its place just because nothing the sorter
// reads distinguishes it. With the default sorter scores equal the delay, so
// the second branch never changes the outcome there.
func outranksBeyondTolerance(next, incumbent Candidate, tolerance uint16) bool {
	if incumbent.Score != next.Score {
		return incumbent.Score > next.Score+float64(tolerance)
	}
	return float64(incumbent.Latency) > float64(next.Latency)+float64(tolerance)
}

// rankAfter reports whether candidate ranks worse than other.
func rankAfter(candidate, other Candidate) bool {
	if candidate.Score != other.Score {
		return candidate.Score > other.Score
	}
	if candidate.Latency != other.Latency {
		return candidate.Latency > other.Latency
	}
	return candidate.Tag > other.Tag
}

func selectTopNWithTolerance(healthy []Candidate, n int, tolerance uint16, previous []Candidate) []Candidate {
	if len(healthy) == 0 {
		return nil
	}
	if n <= 0 || n >= len(healthy) {
		return healthy
	}
	if len(previous) == 0 {
		return healthy[:n]
	}
	previousSet := make(map[string]struct{}, len(previous))
	for _, candidate := range previous {
		previousSet[candidate.Tag] = struct{}{}
	}
	selected := make([]Candidate, 0, n)
	selectedSet := make(map[string]struct{}, n)
	for _, candidate := range healthy {
		if _, ok := previousSet[candidate.Tag]; !ok {
			continue
		}
		selected = append(selected, candidate)
		selectedSet[candidate.Tag] = struct{}{}
		if len(selected) == n {
			break
		}
	}
	for _, next := range healthy {
		if _, ok := selectedSet[next.Tag]; ok {
			continue
		}
		if len(selected) < n {
			selected = append(selected, next)
			selectedSet[next.Tag] = struct{}{}
			continue
		}
		worst := worstCandidateIndex(selected)
		if outranksBeyondTolerance(next, selected[worst], tolerance) {
			delete(selectedSet, selected[worst].Tag)
			selected[worst] = next
			selectedSet[next.Tag] = struct{}{}
			continue
		}
		break
	}
	sortCandidatesByScore(selected)
	return selected
}

func (l *LoadBalance) emptyPoolError() error {
	return E.New("loadbalance: empty candidate pool")
}

func (l *LoadBalance) selectEmptyPoolFallback() (Candidate, error) {
	if l.emptyPoolAction == "random" {
		var candidates []Candidate
		for _, tag := range l.primaryTags {
			if detour, ok := l.primaryOutbounds[tag]; ok {
				candidates = append(candidates, Candidate{
					Tag:       tag,
					Outbound:  detour,
					IsPrimary: true,
				})
			}
		}
		for _, tag := range l.backupTags {
			if detour, ok := l.backupOutbounds[tag]; ok {
				candidates = append(candidates, Candidate{
					Tag:       tag,
					Outbound:  detour,
					IsPrimary: false,
				})
			}
		}
		if len(candidates) > 0 {
			return candidates[rand.Intn(len(candidates))], nil
		}
	}
	return Candidate{}, l.emptyPoolError()
}

func sameCandidateSet(previous []Candidate, next []Candidate) bool {
	if len(previous) != len(next) {
		return false
	}
	previousSet := make(map[string]bool, len(previous))
	for _, candidate := range previous {
		previousSet[candidate.Tag] = candidate.IsPrimary
	}
	for _, candidate := range next {
		isPrimary, exists := previousSet[candidate.Tag]
		if !exists || isPrimary != candidate.IsPrimary {
			return false
		}
	}
	return true
}

func loadBalanceMetadata(ctx context.Context) adapter.InboundContext {
	if metadata := adapter.ContextFrom(ctx); metadata != nil {
		return *metadata
	}
	return adapter.InboundContext{}
}

func (l *LoadBalance) probeInterval() time.Duration {
	if l.interval > 0 {
		return l.interval
	}
	return C.DefaultURLTestInterval
}

func (l *LoadBalance) memberProbe(detour adapter.Outbound) (uint16, probeAction) {
	switch member := detour.(type) {
	case *LoadBalance:
		delay, ok := nestedMinDelay(member, l.history)
		if ok {
			return delay, probeUseCache
		}
		return 0, probeSkip
	case *URLTest:
		history := l.history.LoadURLTestHistory(RealTag(member))
		if history != nil && history.Delay != 0 {
			return history.Delay, probeUseCache
		}
		return 0, probeSkip
	default:
		history := l.history.LoadURLTestHistory(RealTag(detour))
		if history != nil && history.Delay != 0 && time.Since(history.Time) < l.probeInterval() {
			return history.Delay, probeUseCache
		}
		return 0, probeHTTP
	}
}

func nestedMinDelay(nested *LoadBalance, history *urltest.HistoryStorage) (uint16, bool) {
	snap := nested.snapshot.Load()
	if snap != nil {
		var min uint16
		found := false
		for _, candidate := range snap.Candidates {
			if candidate.Latency == 0 {
				continue
			}
			if !found || candidate.Latency < min {
				min = candidate.Latency
				found = true
			}
		}
		if found {
			return min, true
		}
	}
	var min uint16
	found := false
	for _, tag := range nested.tags {
		detour := nested.primaryOutbounds[tag]
		if detour == nil {
			detour = nested.backupOutbounds[tag]
		}
		if detour == nil {
			continue
		}
		stored := history.LoadURLTestHistory(RealTag(detour))
		if stored == nil || stored.Delay == 0 {
			continue
		}
		if !found || stored.Delay < min {
			min = stored.Delay
			found = true
		}
	}
	return min, found
}

func (l *LoadBalance) liveMemberDelay(tag string, detour adapter.Outbound) (uint16, bool) {
	switch member := detour.(type) {
	case *LoadBalance:
		return nestedMinDelay(member, l.history)
	case *URLTest:
		history := l.history.LoadURLTestHistory(RealTag(member))
		if history == nil || history.Delay == 0 {
			return 0, false
		}
		return history.Delay, true
	default:
		history := l.history.LoadURLTestHistory(RealTag(detour))
		if history == nil || history.Delay == 0 {
			return 0, false
		}
		return history.Delay, true
	}
}

func (l *LoadBalance) hasPendingSelfTestingMember() bool {
	tags := l.primaryTags
	outbounds := l.primaryOutbounds
	hasPrimary := false
	for _, tag := range tags {
		if outbounds[tag] != nil {
			hasPrimary = true
			break
		}
	}
	if !hasPrimary {
		tags = l.backupTags
		outbounds = l.backupOutbounds
	}
	for _, tag := range tags {
		detour := outbounds[tag]
		if detour == nil {
			continue
		}
		switch detour.(type) {
		case *LoadBalance, *URLTest:
			if _, ok := l.liveMemberDelay(tag, detour); !ok {
				return true
			}
		}
	}
	return false
}

func snapshotAllZeroLatency(snap *CandidateSnapshot) bool {
	if snap == nil || len(snap.Candidates) == 0 {
		return false
	}
	for _, candidate := range snap.Candidates {
		if candidate.Latency != 0 {
			return false
		}
	}
	return true
}
