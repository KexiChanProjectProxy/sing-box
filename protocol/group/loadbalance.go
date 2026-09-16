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
	delayWindow                  int
	windowWeight                 uint16
	lastWeight                   uint16
	windows                      map[string]*delayWindow
	windowsMu                    sync.Mutex
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
	if options.WeightedDelay != nil {
		lb.delayWindow = options.WeightedDelay.Window
		lb.windowWeight = options.WeightedDelay.WindowWeight
		lb.lastWeight = options.WeightedDelay.LastWeight
		lb.windows = make(map[string]*delayWindow)
	}
	return lb, nil
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
	l.seedInitialSnapshot()
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
	sortCandidates(measured)
	sortCandidates(unmeasured)
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

func (l *LoadBalance) rebuildSnapshot() {
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
			Latency:   l.rankedDelay(tag, delay),
			IsPrimary: isPrimary,
		})
	}
	sortCandidates(candidates)
	return candidates
}

const defaultLoadBalanceTolerance uint16 = 10

func sortCandidates(candidates []Candidate) {
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Latency != candidates[j].Latency {
			return candidates[i].Latency < candidates[j].Latency
		}
		return candidates[i].Tag < candidates[j].Tag
	})
}

func worstCandidateIndex(candidates []Candidate) int {
	worst := 0
	for i := 1; i < len(candidates); i++ {
		if candidates[i].Latency > candidates[worst].Latency ||
			(candidates[i].Latency == candidates[worst].Latency && candidates[i].Tag > candidates[worst].Tag) {
			worst = i
		}
	}
	return worst
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
		if uint32(selected[worst].Latency) > uint32(next.Latency)+uint32(tolerance) {
			delete(selectedSet, selected[worst].Tag)
			selected[worst] = next
			selectedSet[next.Tag] = struct{}{}
			continue
		}
		break
	}
	sortCandidates(selected)
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
