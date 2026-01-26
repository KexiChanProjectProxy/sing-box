package group

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/domain"
	"github.com/sagernet/sing-box/common/interrupt"
	"github.com/sagernet/sing-box/common/urltest"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/pause"
	"github.com/sagernet/sing/common/x/list"

	"github.com/cespare/xxhash/v2"
)

const (
	defaultFallbackHoldTime    = 30 * time.Second
	defaultMeasurementTimeout  = 10 * time.Second
	defaultTieredEmptyPoolAction = emptyPoolActionError
)

// TieredLoadBalance implements N-tier load balancing with real connection latency tracking
type TieredLoadBalance struct {
	outbound.Adapter

	ctx      context.Context
	router   adapter.Router
	outbound adapter.OutboundManager
	logger   log.ContextLogger

	// Configuration
	tiers           []TierConfig
	globalStrategy  string
	globalHash      *hashConfig
	emptyPoolAction string

	// Runtime components
	latencyTracker  *LatencyTracker
	tierState       atomic.Value // *TierStateSnapshot
	candidatePools  atomic.Value // *CandidatePoolSnapshot

	// Hash configuration (global)
	hashKeyParts     []string
	hashVirtualNodes int
	hashOnEmptyKey   string
	hashKeySalt      string

	// Hysteresis
	fallbackHoldTime time.Duration

	// Resources
	interruptGroup               *interrupt.Group
	interruptExternalConnections bool
	asnReader                    adapter.ASNReader
	geositeReader                adapter.GeositeReader

	// Periodic update
	updateMu sync.Mutex
	close    chan struct{}

	// URLTest support (optional for Clash API compatibility)
	link         string
	interval     time.Duration
	timeout      time.Duration
	idleTimeout  time.Duration
	history      adapter.URLTestHistoryStorage
	pauseManager pause.Manager
	pauseCallback *list.Element[pause.Callback]
	ticker       *time.Ticker
	tickerAccess sync.Mutex
	checking     atomic.Bool
}

// TierConfig holds configuration for a single tier
type TierConfig struct {
	level      int
	tags       []string
	topN       int
	strategy   string // Can override global strategy
	maxLatency time.Duration
}

// hashConfig holds hash configuration
type hashConfig struct {
	keyParts     []string
	virtualNodes int
	onEmptyKey   string
	keySalt      string
}

// TierStateSnapshot tracks current tier state
type TierStateSnapshot struct {
	activeTierLevel int
	tierActivatedAt time.Time
	previousTier    int
}

// CandidatePoolSnapshot holds immutable snapshot of candidate pools per tier
type CandidatePoolSnapshot struct {
	tierCandidates map[int][]adapter.Outbound
	tierHashRings  map[int]*consistentHashRing
}

// tierLatencyInfo holds latency information for sorting
type tierLatencyInfo struct {
	tag     string
	latency time.Duration
	healthy bool
}

func RegisterTieredLoadBalance(registry *outbound.Registry) {
	outbound.Register[option.TieredLoadBalanceOutboundOptions](registry, C.TypeTieredLoadBalance, NewTieredLoadBalance)
}

func NewTieredLoadBalance(
	ctx context.Context,
	router adapter.Router,
	logger log.ContextLogger,
	tag string,
	options option.TieredLoadBalanceOutboundOptions,
) (adapter.Outbound, error) {
	outboundManager := service.FromContext[adapter.OutboundManager](ctx)
	if outboundManager == nil {
		return nil, E.New("missing outbound manager")
	}

	// Validate configuration
	if len(options.Tiers) == 0 {
		return nil, E.New("at least one tier is required")
	}

	// Validate strategy
	globalStrategy := options.Strategy
	if globalStrategy == "" {
		globalStrategy = strategyRandom
	}
	if globalStrategy != strategyRandom && globalStrategy != strategyConsistentHash {
		return nil, E.New("strategy must be 'random' or 'consistent_hash'")
	}

	if globalStrategy == strategyConsistentHash && options.Hash == nil {
		return nil, E.New("hash configuration required for consistent_hash strategy")
	}

	// Validate empty pool action
	emptyPoolAction := options.EmptyPoolAction
	if emptyPoolAction == "" {
		emptyPoolAction = defaultTieredEmptyPoolAction
	}
	if emptyPoolAction != emptyPoolActionError && emptyPoolAction != emptyPoolActionFallbackAll {
		return nil, E.New("empty_pool_action must be 'error' or 'fallback_all'")
	}

	// Validate tiers
	tiers := make([]TierConfig, len(options.Tiers))
	allTags := make(map[string]bool)

	for i, tierOpt := range options.Tiers {
		// Validate level sequence
		if tierOpt.Level != i+1 {
			return nil, E.New("tier levels must be sequential starting from 1, expected ", i+1, " got ", tierOpt.Level)
		}

		if len(tierOpt.Outbounds) == 0 {
			return nil, E.New("tier ", tierOpt.Level, " must have at least one outbound")
		}

		if tierOpt.TopN <= 0 {
			return nil, E.New("tier ", tierOpt.Level, " top_n must be > 0")
		}

		if time.Duration(tierOpt.MaxLatency) <= 0 {
			return nil, E.New("tier ", tierOpt.Level, " max_latency must be > 0")
		}

		// Tier strategy can override global
		tierStrategy := tierOpt.Strategy
		if tierStrategy == "" {
			tierStrategy = globalStrategy
		}
		if tierStrategy != strategyRandom && tierStrategy != strategyConsistentHash {
			return nil, E.New("tier ", tierOpt.Level, " strategy must be 'random' or 'consistent_hash'")
		}

		tiers[i] = TierConfig{
			level:      tierOpt.Level,
			tags:       append([]string{}, tierOpt.Outbounds...),
			topN:       tierOpt.TopN,
			strategy:   tierStrategy,
			maxLatency: time.Duration(tierOpt.MaxLatency),
		}

		// Collect all tags (duplicates allowed across tiers)
		for _, tag := range tierOpt.Outbounds {
			allTags[tag] = true
		}
	}

	// Collect unique dependencies
	dependencies := make([]string, 0, len(allTags))
	for tag := range allTags {
		dependencies = append(dependencies, tag)
	}

	lb := &TieredLoadBalance{
		Adapter:          outbound.NewAdapter(C.TypeTieredLoadBalance, tag, []string{N.NetworkTCP, N.NetworkUDP}, dependencies),
		ctx:              ctx,
		router:           router,
		outbound:         outboundManager,
		logger:           logger,
		tiers:            tiers,
		globalStrategy:   globalStrategy,
		emptyPoolAction:  emptyPoolAction,
		interruptExternalConnections: options.InterruptExistConnections,
		close:            make(chan struct{}),
	}

	// Parse latency monitoring options
	latencyOpt := options.LatencyMonitoring
	var failureThreshold uint32 = defaultFailureThreshold
	var recoveryThreshold uint32 = defaultRecoveryThreshold
	var historySize int = defaultHistorySize
	var samplingRate int = defaultSamplingRate

	if latencyOpt != nil {
		if latencyOpt.FailureThreshold > 0 {
			failureThreshold = latencyOpt.FailureThreshold
		}
		if latencyOpt.RecoveryThreshold > 0 {
			recoveryThreshold = latencyOpt.RecoveryThreshold
		}
		if latencyOpt.HistorySize > 0 {
			historySize = latencyOpt.HistorySize
		}
		if latencyOpt.SamplingRate > 0 {
			samplingRate = latencyOpt.SamplingRate
		}
		if latencyOpt.FallbackHoldTime > 0 {
			lb.fallbackHoldTime = time.Duration(latencyOpt.FallbackHoldTime)
		}
	}

	if lb.fallbackHoldTime == 0 {
		lb.fallbackHoldTime = defaultFallbackHoldTime
	}

	// Initialize latency tracker
	lb.latencyTracker = NewLatencyTracker(failureThreshold, recoveryThreshold, historySize, samplingRate)

	// Register all outbounds with their tier-specific thresholds
	for _, tier := range tiers {
		for _, tag := range tier.tags {
			lb.latencyTracker.RegisterOutbound(tag, tier.level, tier.maxLatency)
		}
	}

	// Hash configuration
	if options.Hash != nil {
		lb.globalHash = &hashConfig{
			keyParts:     options.Hash.KeyParts,
			virtualNodes: options.Hash.VirtualNodes,
			onEmptyKey:   options.Hash.OnEmptyKey,
			keySalt:      options.Hash.KeySalt,
		}

		lb.hashKeyParts = options.Hash.KeyParts
		lb.hashVirtualNodes = options.Hash.VirtualNodes
		lb.hashOnEmptyKey = options.Hash.OnEmptyKey
		lb.hashKeySalt = options.Hash.KeySalt

		if lb.hashVirtualNodes == 0 {
			lb.hashVirtualNodes = defaultVirtualNodes
		}
		if lb.hashOnEmptyKey == "" {
			lb.hashOnEmptyKey = defaultOnEmptyKey
		}
		if lb.hashOnEmptyKey != onEmptyKeyRandom && lb.hashOnEmptyKey != onEmptyKeyHashEmpty {
			return nil, E.New("hash.on_empty_key must be 'random' or 'hash_empty'")
		}
	}

	// Initialize tier state (start with tier 1)
	initialTierState := &TierStateSnapshot{
		activeTierLevel: 1,
		tierActivatedAt: time.Now(),
		previousTier:    0,
	}
	lb.tierState.Store(initialTierState)

	// Initialize empty candidate pools
	initialPools := &CandidatePoolSnapshot{
		tierCandidates: make(map[int][]adapter.Outbound),
		tierHashRings:  make(map[int]*consistentHashRing),
	}
	lb.candidatePools.Store(initialPools)

	if lb.interruptExternalConnections {
		lb.interruptGroup = interrupt.NewGroup()
	}

	// Get ASN reader from router if available
	if router != nil {
		lb.asnReader = router.ASNReader()
		lb.geositeReader = router.GeositeReader()
	}

	// URLTest configuration (optional, for Clash API compatibility)
	lb.link = options.URL
	lb.interval = time.Duration(options.Interval)
	lb.timeout = time.Duration(options.Timeout)
	lb.idleTimeout = time.Duration(options.IdleTimeout)

	// Set defaults for URLTest
	if lb.link == "" {
		lb.link = "https://www.gstatic.com/generate_204"
	}
	if lb.interval == 0 {
		lb.interval = defaultInterval
	}
	if lb.timeout == 0 {
		lb.timeout = defaultTimeout
	}
	if lb.idleTimeout == 0 {
		lb.idleTimeout = defaultIdleTimeout
	}

	return lb, nil
}

func (lb *TieredLoadBalance) Network() []string {
	return []string{N.NetworkTCP, N.NetworkUDP}
}

func (lb *TieredLoadBalance) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}

	// Retrieve shared history storage for URLTest support
	var history adapter.URLTestHistoryStorage
	if historyFromCtx := service.PtrFromContext[urltest.HistoryStorage](lb.ctx); historyFromCtx != nil {
		history = historyFromCtx
	} else if clashServer := service.FromContext[adapter.ClashServer](lb.ctx); clashServer != nil {
		history = clashServer.HistoryStorage()
	} else {
		history = urltest.NewHistoryStorage()
	}
	lb.history = history

	// Initialize pause manager
	lb.pauseManager = service.FromContext[pause.Manager](lb.ctx)

	return nil
}

func (lb *TieredLoadBalance) PostStart() error {
	// Start with bootstrap pools (all tier 1 outbounds)
	lb.updateCandidates()
	return nil
}

func (lb *TieredLoadBalance) Close() error {
	close(lb.close)

	lb.tickerAccess.Lock()
	if lb.ticker != nil {
		lb.ticker.Stop()
		if lb.pauseManager != nil && lb.pauseCallback != nil {
			lb.pauseManager.UnregisterCallback(lb.pauseCallback)
		}
	}
	lb.tickerAccess.Unlock()

	return nil
}

// DialContext dials with latency measurement
func (lb *TieredLoadBalance) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	metadata := adapter.ContextFrom(ctx)
	if metadata == nil {
		metadata = &adapter.InboundContext{}
	}

	outbound, tierLevel, err := lb.selectOutbound(network, metadata)
	if err != nil {
		return nil, err
	}

	// Measure dial latency
	start := time.Now()
	conn, err := outbound.DialContext(ctx, network, destination)
	duration := time.Since(start)

	// Record latency asynchronously (if sampled)
	if lb.latencyTracker.ShouldSample() {
		go func() {
			success := err == nil
			lb.latencyTracker.RecordLatency(outbound.Tag(), tierLevel, duration, success)
			// Update candidates after recording
			lb.updateCandidates()
		}()
	}

	if err != nil {
		return nil, err
	}

	if lb.interruptGroup != nil {
		return lb.interruptGroup.NewConn(conn, interrupt.IsExternalConnectionFromContext(ctx)), nil
	}

	return conn, nil
}

// ListenPacket listens with latency measurement
func (lb *TieredLoadBalance) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	metadata := adapter.ContextFrom(ctx)
	if metadata == nil {
		metadata = &adapter.InboundContext{}
	}

	outbound, tierLevel, err := lb.selectOutbound(N.NetworkUDP, metadata)
	if err != nil {
		return nil, err
	}

	// Measure dial latency
	start := time.Now()
	conn, err := outbound.ListenPacket(ctx, destination)
	duration := time.Since(start)

	// Record latency asynchronously (if sampled)
	if lb.latencyTracker.ShouldSample() {
		go func() {
			success := err == nil
			lb.latencyTracker.RecordLatency(outbound.Tag(), tierLevel, duration, success)
			// Update candidates after recording
			lb.updateCandidates()
		}()
	}

	if err != nil {
		return nil, err
	}

	if lb.interruptGroup != nil {
		return lb.interruptGroup.NewPacketConn(conn, interrupt.IsExternalConnectionFromContext(ctx)), nil
	}

	return conn, nil
}

// selectOutbound selects an outbound from the active tier
func (lb *TieredLoadBalance) selectOutbound(network string, metadata *adapter.InboundContext) (adapter.Outbound, int, error) {
	// Touch to maintain activity
	lb.Touch()

	pools := lb.candidatePools.Load().(*CandidatePoolSnapshot)
	tierState := lb.tierState.Load().(*TierStateSnapshot)

	activeTier := tierState.activeTierLevel

	// Try tiers in order starting from active tier
	for tierLevel := activeTier; tierLevel <= len(lb.tiers); tierLevel++ {
		candidates, exists := pools.tierCandidates[tierLevel]
		if !exists || len(candidates) == 0 {
			continue
		}

		// Filter by network support
		networkCandidates := make([]adapter.Outbound, 0, len(candidates))
		for _, candidate := range candidates {
			if common.Contains(candidate.Network(), network) {
				networkCandidates = append(networkCandidates, candidate)
			}
		}

		if len(networkCandidates) == 0 {
			continue
		}

		// Select from this tier
		tier := lb.tiers[tierLevel-1]
		strategy := tier.strategy

		var selected adapter.Outbound
		var err error

		if strategy == strategyConsistentHash {
			selected, err = lb.selectConsistentHash(pools, tierLevel, networkCandidates, metadata)
		} else {
			selected = networkCandidates[rand.Intn(len(networkCandidates))]
		}

		if selected != nil {
			lb.logger.Debug(
				"selected outbound: tier=", tierLevel,
				", strategy=", strategy,
				", outbound=", selected.Tag(),
				", pool_size=", len(networkCandidates),
			)
			return selected, tierLevel, nil
		}

		if err != nil {
			lb.logger.Debug("tier ", tierLevel, " selection failed: ", err)
		}
	}

	// All tiers exhausted
	if lb.emptyPoolAction == emptyPoolActionFallbackAll {
		// Fallback to any configured outbound
		lb.logger.Warn("all tiers empty, falling back to any outbound")
		return lb.selectFallbackOutbound(network)
	}

	return nil, 0, E.New("no healthy candidates available in any tier")
}

// selectConsistentHash selects using consistent hash
func (lb *TieredLoadBalance) selectConsistentHash(
	pools *CandidatePoolSnapshot,
	tierLevel int,
	candidates []adapter.Outbound,
	metadata *adapter.InboundContext,
) (adapter.Outbound, error) {
	ring, exists := pools.tierHashRings[tierLevel]
	if !exists || ring == nil || len(ring.points) == 0 {
		// Fallback to random
		return candidates[rand.Intn(len(candidates))], nil
	}

	hashKey := lb.buildHashKey(metadata)

	if hashKey == "" {
		if lb.hashOnEmptyKey == onEmptyKeyRandom {
			return candidates[rand.Intn(len(candidates))], nil
		}
		// Hash empty string
		hashKey = ""
	}

	keyHash := xxhash.Sum64String(hashKey)
	nodeTag := lb.lookupHashRing(ring, keyHash)

	// Find outbound with this tag in candidates
	for _, candidate := range candidates {
		if candidate.Tag() == nodeTag {
			lb.logger.Debug(
				"consistent_hash: key=", hashKey,
				", hash=", keyHash,
				", selected=", nodeTag,
			)
			return candidate, nil
		}
	}

	// Tag not in candidates, fallback to random
	return candidates[rand.Intn(len(candidates))], nil
}

// selectFallbackOutbound selects any configured outbound ignoring health
func (lb *TieredLoadBalance) selectFallbackOutbound(network string) (adapter.Outbound, int, error) {
	// Try all configured outbounds from tier 1
	for _, tier := range lb.tiers {
		for _, tag := range tier.tags {
			detour, loaded := lb.outbound.Outbound(tag)
			if loaded && common.Contains(detour.Network(), network) {
				lb.logger.Warn("fallback selection: ", tag)
				return detour, tier.level, nil
			}
		}
	}

	return nil, 0, E.New("no outbounds available for network ", network)
}

// updateCandidates rebuilds candidate pools based on current latency data
func (lb *TieredLoadBalance) updateCandidates() {
	lb.updateMu.Lock()
	defer lb.updateMu.Unlock()

	newPools := &CandidatePoolSnapshot{
		tierCandidates: make(map[int][]adapter.Outbound),
		tierHashRings:  make(map[int]*consistentHashRing),
	}

	// Build candidate pool for each tier
	for _, tier := range lb.tiers {
		candidates := lb.selectTopNForTier(tier)
		newPools.tierCandidates[tier.level] = candidates

		// Log candidates for this tier
		lb.logCandidates(tier.level, candidates)

		// Build hash ring if needed
		if tier.strategy == strategyConsistentHash && len(candidates) > 0 {
			newPools.tierHashRings[tier.level] = lb.buildHashRing(candidates)
		}
	}

	// Update tier state
	oldTierState := lb.tierState.Load().(*TierStateSnapshot)
	lb.checkTierTransition(newPools)
	newTierState := lb.tierState.Load().(*TierStateSnapshot)

	// Log tier transition
	if oldTierState.activeTierLevel != newTierState.activeTierLevel {
		lb.logger.Warn(
			"tier transition: tier ", oldTierState.activeTierLevel,
			" -> tier ", newTierState.activeTierLevel,
		)
	}

	// Atomically update pools
	oldPools := lb.candidatePools.Swap(newPools)

	// Interrupt connections if tier changed
	if lb.interruptExternalConnections && oldPools != nil {
		oldState := lb.tierState.Load().(*TierStateSnapshot)
		if oldState.previousTier != 0 && oldState.previousTier != oldState.activeTierLevel {
			if lb.interruptGroup != nil {
				lb.interruptGroup.Interrupt(false)
			}
		}
	}
}

// selectTopNForTier selects top-N healthy outbounds for a tier
func (lb *TieredLoadBalance) selectTopNForTier(tier TierConfig) []adapter.Outbound {
	// Collect latency info for all outbounds in this tier
	infos := make([]tierLatencyInfo, 0, len(tier.tags))

	for _, tag := range tier.tags {
		_, loaded := lb.outbound.Outbound(tag)
		if !loaded {
			continue
		}

		healthy := lb.latencyTracker.IsHealthyForTier(tag, tier.level)
		latency := lb.latencyTracker.GetAverageLatency(tag)

		infos = append(infos, tierLatencyInfo{
			tag:     tag,
			latency: latency,
			healthy: healthy,
		})
	}

	// Filter to only healthy outbounds
	healthyInfos := make([]tierLatencyInfo, 0, len(infos))
	for _, info := range infos {
		if info.healthy {
			healthyInfos = append(healthyInfos, info)
		}
	}

	if len(healthyInfos) == 0 {
		lb.logger.Debug("tier ", tier.level, ": no healthy candidates")
		return nil
	}

	// Sort by latency (unknown latency = 0, sorts to beginning for cold start)
	sort.Slice(healthyInfos, func(i, j int) bool {
		// Put unknown (0) latencies first for cold start
		if healthyInfos[i].latency == 0 && healthyInfos[j].latency != 0 {
			return true
		}
		if healthyInfos[i].latency != 0 && healthyInfos[j].latency == 0 {
			return false
		}
		return healthyInfos[i].latency < healthyInfos[j].latency
	})

	// Select top-N
	topN := tier.topN
	if topN > len(healthyInfos) {
		topN = len(healthyInfos)
	}

	candidates := make([]adapter.Outbound, 0, topN)
	for i := 0; i < topN; i++ {
		detour, loaded := lb.outbound.Outbound(healthyInfos[i].tag)
		if loaded {
			candidates = append(candidates, detour)
		}
	}

	lb.logger.Debug(
		"tier ", tier.level, ": selected ", len(candidates), "/", topN,
		" candidates from ", len(healthyInfos), " healthy outbounds",
	)

	return candidates
}

// checkTierTransition checks if tier should transition
func (lb *TieredLoadBalance) checkTierTransition(pools *CandidatePoolSnapshot) {
	currentState := lb.tierState.Load().(*TierStateSnapshot)
	activeTier := currentState.activeTierLevel

	// Try to recover to lower (better) tier
	for tierLevel := 1; tierLevel < activeTier; tierLevel++ {
		candidates, exists := pools.tierCandidates[tierLevel]
		if exists && len(candidates) > 0 {
			// Check hold time
			if time.Since(currentState.tierActivatedAt) >= lb.fallbackHoldTime {
				lb.logger.Info(
					"tier recovery: tier ", activeTier, " -> tier ", tierLevel,
					" (hold time elapsed: ", time.Since(currentState.tierActivatedAt), ")",
				)

				newState := &TierStateSnapshot{
					activeTierLevel: tierLevel,
					tierActivatedAt: time.Now(),
					previousTier:    activeTier,
				}
				lb.tierState.Store(newState)
				return
			}
		}
	}

	// Try to fallback to higher (worse) tier
	candidates, exists := pools.tierCandidates[activeTier]
	if !exists || len(candidates) == 0 {
		// Current tier empty, try next tier
		for tierLevel := activeTier + 1; tierLevel <= len(lb.tiers); tierLevel++ {
			candidates, exists := pools.tierCandidates[tierLevel]
			if exists && len(candidates) > 0 {
				lb.logger.Warn(
					"tier fallback: tier ", activeTier, " -> tier ", tierLevel,
					" (current tier empty)",
				)

				newState := &TierStateSnapshot{
					activeTierLevel: tierLevel,
					tierActivatedAt: time.Now(),
					previousTier:    activeTier,
				}
				lb.tierState.Store(newState)
				return
			}
		}
	}
}

// buildHashRing constructs a consistent hash ring (reused from LoadBalance)
func (lb *TieredLoadBalance) buildHashRing(members []adapter.Outbound) *consistentHashRing {
	ring := &consistentHashRing{
		points:       make([]uint64, 0, len(members)*lb.hashVirtualNodes),
		nodeMap:      make(map[uint64]string),
		members:      make([]string, len(members)),
		virtualNodes: lb.hashVirtualNodes,
	}

	for i, member := range members {
		ring.members[i] = member.Tag()

		// Create virtual nodes
		for j := 0; j < lb.hashVirtualNodes; j++ {
			virtualKey := fmt.Sprintf("%s:%d", member.Tag(), j)
			hash := xxhash.Sum64String(virtualKey)
			ring.points = append(ring.points, hash)
			ring.nodeMap[hash] = member.Tag()
		}
	}

	// Sort points for binary search
	sort.Slice(ring.points, func(i, j int) bool {
		return ring.points[i] < ring.points[j]
	})

	return ring
}

// lookupHashRing finds the node for a given key hash (reused from LoadBalance)
func (lb *TieredLoadBalance) lookupHashRing(ring *consistentHashRing, keyHash uint64) string {
	if len(ring.points) == 0 {
		return ""
	}

	// Binary search for first point >= keyHash
	idx := sort.Search(len(ring.points), func(i int) bool {
		return ring.points[i] >= keyHash
	})

	// Wrap around if necessary
	if idx >= len(ring.points) {
		idx = 0
	}

	return ring.nodeMap[ring.points[idx]]
}

// buildHashKey constructs hash key from connection metadata (reused from LoadBalance)
func (lb *TieredLoadBalance) buildHashKey(metadata *adapter.InboundContext) string {
	if len(lb.hashKeyParts) == 0 {
		return ""
	}

	parts := make([]string, 0, len(lb.hashKeyParts))

	for _, part := range lb.hashKeyParts {
		switch part {
		case "src_ip":
			if metadata.Source.IsValid() {
				parts = append(parts, metadata.Source.Addr.String())
			} else {
				parts = append(parts, "-")
			}
		case "dst_ip":
			if metadata.Destination.IsValid() && !metadata.Destination.IsFqdn() {
				parts = append(parts, metadata.Destination.Addr.String())
			} else {
				parts = append(parts, "-")
			}
		case "src_port":
			if metadata.Source.IsValid() {
				parts = append(parts, fmt.Sprintf("%d", metadata.Source.Port))
			} else {
				parts = append(parts, "-")
			}
		case "dst_port":
			if metadata.Destination.IsValid() {
				parts = append(parts, fmt.Sprintf("%d", metadata.Destination.Port))
			} else {
				parts = append(parts, "-")
			}
		case "network":
			if metadata.Network != "" {
				parts = append(parts, metadata.Network)
			} else {
				parts = append(parts, "-")
			}
		case "domain":
			if metadata.Destination.IsFqdn() {
				parts = append(parts, metadata.Destination.Fqdn)
			} else {
				parts = append(parts, "-")
			}
		case "inbound_tag":
			if metadata.Inbound != "" {
				parts = append(parts, metadata.Inbound)
			} else {
				parts = append(parts, "-")
			}
		case "matched_ruleset":
			if metadata.MatchedRuleSet != "" {
				parts = append(parts, metadata.MatchedRuleSet)
			} else {
				parts = append(parts, "-")
			}
		case "etld_plus_one":
			if metadata.Destination.IsFqdn() {
				etld := domain.ExtractETLDPlusOne(metadata.Destination.Fqdn)
				parts = append(parts, etld)
			} else {
				parts = append(parts, "-")
			}
		case "matched_ruleset_or_etld":
			if metadata.MatchedRuleSet != "" {
				parts = append(parts, metadata.MatchedRuleSet)
			} else if metadata.Destination.IsFqdn() {
				etld := domain.ExtractETLDPlusOne(metadata.Destination.Fqdn)
				parts = append(parts, etld)
			} else {
				parts = append(parts, "-")
			}
		case "dst_asn":
			// Requires ASN reader
			if lb.asnReader != nil && metadata.Destination.IsValid() && !metadata.Destination.IsFqdn() {
				asn := lb.asnReader.Lookup(metadata.Destination.Addr)
				if asn != 0 {
					parts = append(parts, fmt.Sprintf("AS%d", asn))
				} else {
					parts = append(parts, "-")
				}
			} else {
				parts = append(parts, "-")
			}
		case "dst_geosite":
			// Requires geosite reader
			if lb.geositeReader != nil && metadata.Destination.IsFqdn() {
				code := lb.geositeReader.Lookup(metadata.Destination.Fqdn)
				if code != "" {
					parts = append(parts, fmt.Sprintf("geosite:%s", code))
				} else {
					parts = append(parts, "-")
				}
			} else {
				parts = append(parts, "-")
			}
		default:
			parts = append(parts, "-")
		}
	}

	key := strings.Join(parts, "|")
	if lb.hashKeySalt != "" {
		key = lb.hashKeySalt + key
	}

	return key
}

// OutboundGroup interface implementation
func (lb *TieredLoadBalance) Now() string {
	pools := lb.candidatePools.Load().(*CandidatePoolSnapshot)
	tierState := lb.tierState.Load().(*TierStateSnapshot)

	candidates, exists := pools.tierCandidates[tierState.activeTierLevel]
	if !exists || len(candidates) == 0 {
		return ""
	}

	return candidates[0].Tag()
}

func (lb *TieredLoadBalance) All() []string {
	allTags := make(map[string]bool)
	for _, tier := range lb.tiers {
		for _, tag := range tier.tags {
			allTags[tag] = true
		}
	}

	result := make([]string, 0, len(allTags))
	for tag := range allTags {
		result = append(result, tag)
	}
	return result
}

// Touch starts or resets the idle timeout for periodic URLTest checks
func (lb *TieredLoadBalance) Touch() {
	if lb.idleTimeout == 0 {
		return
	}

	lb.tickerAccess.Lock()
	defer lb.tickerAccess.Unlock()

	if lb.ticker != nil {
		return
	}

	lb.ticker = time.NewTicker(lb.interval)
	if lb.pauseManager != nil {
		lb.pauseCallback = pause.RegisterTicker(lb.pauseManager, lb.ticker, lb.interval, nil)
	}
	go lb.loopCheck()
}

func (lb *TieredLoadBalance) loopCheck() {
	if lb.idleTimeout == 0 {
		select {}
	}

	idleTimer := time.NewTimer(lb.idleTimeout)
	defer idleTimer.Stop()

	for {
		select {
		case <-lb.close:
			return
		case <-idleTimer.C:
			lb.tickerAccess.Lock()
			lb.ticker.Stop()
			lb.ticker = nil
			lb.tickerAccess.Unlock()
			return
		case <-lb.ticker.C:
			// Perform URL test to gather additional health data
			go lb.performURLTest(lb.ctx)
			idleTimer.Reset(lb.idleTimeout)
		}
	}
}

// performURLTest performs URL testing for all configured outbounds
func (lb *TieredLoadBalance) performURLTest(ctx context.Context) {
	if lb.checking.Swap(true) {
		return
	}
	defer lb.checking.Store(false)

	if lb.pauseManager != nil && lb.pauseManager.IsPaused() {
		return
	}

	// Collect all unique tags
	allTags := make(map[string]bool)
	for _, tier := range lb.tiers {
		for _, tag := range tier.tags {
			allTags[tag] = true
		}
	}

	// Collect outbounds
	outbounds := make([]adapter.Outbound, 0, len(allTags))
	for tag := range allTags {
		detour, loaded := lb.outbound.Outbound(tag)
		if !loaded {
			lb.logger.Error("outbound not found: ", tag)
			continue
		}
		outbounds = append(outbounds, detour)
	}

	if len(outbounds) == 0 {
		return
	}

	// Perform URL tests with controlled concurrency
	maxConcurrent := 10
	semaphore := make(chan struct{}, maxConcurrent)

	var wg sync.WaitGroup
	for _, detour := range outbounds {
		wg.Add(1)
		go func(d adapter.Outbound) {
			defer wg.Done()

			// Acquire semaphore slot
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Create context with timeout
			testCtx, cancel := context.WithTimeout(ctx, lb.timeout)
			defer cancel()

			t, err := urltest.URLTest(testCtx, lb.link, d)
			if err != nil {
				lb.logger.Debug("URL test failed for ", d.Tag(), ": ", err)
				lb.history.DeleteURLTestHistory(RealTag(d))
			} else {
				lb.logger.Debug("URL test succeeded for ", d.Tag(), ": ", t, "ms")
				lb.history.StoreURLTestHistory(RealTag(d), &adapter.URLTestHistory{
					Time:  time.Now(),
					Delay: t,
				})
			}
		}(detour)
	}
	wg.Wait()

	// Note: We don't update candidates here since real latency tracking is primary
	// URL test is only for Clash API compatibility and monitoring
}

// URLTest performs on-demand URL testing and returns latency results
func (lb *TieredLoadBalance) URLTest(ctx context.Context) (map[string]uint16, error) {
	result := make(map[string]uint16)

	// Prevent concurrent health checks
	if lb.checking.Swap(true) {
		return result, nil
	}
	defer lb.checking.Store(false)

	// Check if paused
	if lb.pauseManager != nil && lb.pauseManager.IsPaused() {
		return result, nil
	}

	// Collect all unique tags
	allTags := make(map[string]bool)
	for _, tier := range lb.tiers {
		for _, tag := range tier.tags {
			allTags[tag] = true
		}
	}

	// Collect outbounds to test
	outbounds := make([]adapter.Outbound, 0, len(allTags))
	for tag := range allTags {
		detour, loaded := lb.outbound.Outbound(tag)
		if !loaded {
			lb.logger.Error("outbound not found: ", tag)
			continue
		}
		outbounds = append(outbounds, detour)
	}

	if len(outbounds) == 0 {
		return result, nil
	}

	// Perform health checks with controlled concurrency
	maxConcurrent := 10
	semaphore := make(chan struct{}, maxConcurrent)
	var resultAccess sync.Mutex

	var wg sync.WaitGroup
	for _, detour := range outbounds {
		wg.Add(1)
		go func(d adapter.Outbound) {
			defer wg.Done()

			// Acquire semaphore slot
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Create context with timeout
			testCtx, cancel := context.WithTimeout(ctx, lb.timeout)
			defer cancel()

			t, err := urltest.URLTest(testCtx, lb.link, d)
			if err != nil {
				lb.logger.Debug("URL test failed for ", d.Tag(), ": ", err)
				lb.history.DeleteURLTestHistory(RealTag(d))
			} else {
				lb.logger.Debug("URL test succeeded for ", d.Tag(), ": ", t, "ms")
				lb.history.StoreURLTestHistory(RealTag(d), &adapter.URLTestHistory{
					Time:  time.Now(),
					Delay: t,
				})

				resultAccess.Lock()
				result[d.Tag()] = t
				resultAccess.Unlock()
			}
		}(detour)
	}
	wg.Wait()

	return result, nil
}

// logCandidates logs detailed candidate information for a tier
func (lb *TieredLoadBalance) logCandidates(tierLevel int, candidates []adapter.Outbound) {
	if len(candidates) == 0 {
		lb.logger.Debug("tier ", tierLevel, ": 0 candidates")
		return
	}

	// Build latency info
	tags := make([]string, len(candidates))
	for i, c := range candidates {
		latency := lb.latencyTracker.GetAverageLatency(c.Tag())
		if latency > 0 {
			tags[i] = fmt.Sprintf("%s(%dms)", c.Tag(), latency.Milliseconds())
		} else {
			tags[i] = fmt.Sprintf("%s(?)", c.Tag())
		}
	}

	lb.logger.Info(
		"tier ", tierLevel, ": ", len(candidates), " candidates: [",
		strings.Join(tags, ", "),
		"]",
	)
}
