package group

import (
	"context"
	"math/rand"
	"net"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	F "github.com/sagernet/sing/common/format"

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
	history                      *urltest.HistoryStorage
	group                        *URLTestGroup
	snapshot                     atomic.Pointer[CandidateSnapshot]
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
	l.group = group
	l.history = group.history
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

func (l *LoadBalance) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if networkName := N.NetworkName(network); networkName != N.NetworkTCP && networkName != N.NetworkUDP {
		return nil, E.Extend(N.ErrUnknownNetwork, network)
	}
	l.group.Touch()
	metadata := loadBalanceMetadata(ctx)
	metadata.Network = N.NetworkName(network)
	metadata.Destination = destination
	candidate, err := l.selectCandidate(metadata)
	if err != nil {
		return nil, err
	}
	if l.preferDomain || adapter.PreferDomainFromContext(ctx) {
		ctx = adapter.ContextWithPreferDomain(ctx, true)
	}
	conn, err := candidate.Outbound.DialContext(ctx, network, destination)
	if err != nil {
		l.logger.ErrorEventContext(ctx, "protocol.message", F.ToString(err))

		l.history.DeleteURLTestHistory(RealTag(candidate.Outbound))
		return nil, err
	}
	return l.interruptGroup.NewConn(conn, interrupt.IsExternalConnectionFromContext(ctx)), nil
}

func (l *LoadBalance) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	l.group.Touch()
	metadata := loadBalanceMetadata(ctx)
	metadata.Network = N.NetworkUDP
	metadata.Destination = destination
	candidate, err := l.selectCandidate(metadata)
	if err != nil {
		return nil, err
	}
	if l.preferDomain || adapter.PreferDomainFromContext(ctx) {
		ctx = adapter.ContextWithPreferDomain(ctx, true)
	}
	conn, err := candidate.Outbound.ListenPacket(ctx, destination)
	if err != nil {
		l.logger.ErrorEventContext(ctx, "protocol.message", F.ToString(err))

		l.history.DeleteURLTestHistory(RealTag(candidate.Outbound))
		return nil, err
	}
	return l.interruptGroup.NewPacketConn(conn, interrupt.IsExternalConnectionFromContext(ctx)), nil
}

func (l *LoadBalance) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	ctx = interrupt.ContextWithIsExternalConnection(ctx)
	if l.preferDomain || adapter.PreferDomainFromContext(ctx) {
		ctx = route.ApplyPreferDomain(ctx, &metadata, l)
	}
	l.connection.NewConnection(ctx, l, conn, metadata, onClose)
}

func (l *LoadBalance) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	ctx = interrupt.ContextWithIsExternalConnection(ctx)
	if l.preferDomain || adapter.PreferDomainFromContext(ctx) {
		ctx = route.ApplyPreferDomain(ctx, &metadata, l)
	}
	l.connection.NewPacketConnection(ctx, l, conn, metadata, onClose)
}

func (l *LoadBalance) URLTest(ctx context.Context) (map[string]uint16, error) {
	return l.group.URLTest(ctx)
}

func (l *LoadBalance) CheckOutbounds() {
	l.group.CheckOutbounds(true)
}

func (l *LoadBalance) selectCandidate(metadata adapter.InboundContext) (Candidate, error) {
	snapshot := l.snapshot.Load()
	if snapshot == nil || len(snapshot.Candidates) == 0 {
		return l.selectEmptyPoolFallback()
	}
	if l.strategy == "random" {
		candidate, err := SelectRandomFromSnapshot(snapshot)
		if err != nil {
			return Candidate{}, E.Cause(err, "loadbalance")
		}
		return candidate, nil
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
		return Candidate{}, E.Cause(err, "loadbalance")
	}
	return candidate, nil
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

func (l *LoadBalance) healthyCandidates(tags []string, outbounds map[string]adapter.Outbound, isPrimary bool) []Candidate {
	candidates := make([]Candidate, 0, len(tags))
	for _, tag := range tags {
		detour := outbounds[tag]
		if detour == nil {
			continue
		}
		history := l.history.LoadURLTestHistory(RealTag(detour))
		if history == nil || history.Delay == 0 || (l.timeout > 0 && time.Duration(history.Delay)*time.Millisecond >= l.timeout) {
			continue
		}
		candidates = append(candidates, Candidate{
			Tag:       tag,
			Outbound:  detour,
			Latency:   history.Delay,
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
