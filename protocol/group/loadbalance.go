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
	"github.com/sagernet/sing/common/x/list"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/pause"

	"github.com/cespare/xxhash/v2"
)

const (
	strategyRandom         = "random"
	strategyConsistentHash = "consistent_hash"

	emptyPoolActionError       = "error"
	emptyPoolActionFallbackAll = "fallback_all"

	onEmptyKeyRandom    = "random"
	onEmptyKeyHashEmpty = "hash_empty"

	defaultInterval      = 3 * time.Minute
	defaultTimeout       = 5 * time.Second
	defaultIdleTimeout   = 30 * time.Minute
	defaultVirtualNodes  = 100
	defaultOnEmptyKey    = onEmptyKeyRandom
	defaultEmptyPoolAction = emptyPoolActionError

	// Hysteresis defaults
	defaultPrimaryFailures = 3
	defaultBackupHoldTime  = 30 * time.Second
)

// LoadBalance implements URL-test driven load balancing with HAProxy-style backup tiers
type LoadBalance struct {
	outbound.Adapter
	ctx        context.Context
	router     adapter.Router
	outbound   adapter.OutboundManager
	connection adapter.ConnectionManager
	logger     log.ContextLogger

	// Configuration
	primaryTags   []string
	backupTags    []string
	link          string
	interval      time.Duration
	timeout       time.Duration
	idleTimeout   time.Duration
	topNPrimary   int
	topNBackup    int
	strategy      string
	emptyPoolAction string

	// Hash configuration
	hashKeyParts     []string
	hashVirtualNodes int
	hashOnEmptyKey   string
	hashKeySalt      string

	// Hysteresis configuration
	hystPrimaryFailures uint32
	hystBackupHoldTime  time.Duration

	// Runtime state
	history                      adapter.URLTestHistoryStorage
	interruptGroup               *interrupt.Group
	interruptExternalConnections bool

	// Candidate pools and selection state
	candidateState atomic.Value // *candidateSnapshot
	tierState      atomic.Value // *tierStateSnapshot

	// Health check coordination
	checking atomic.Bool
	pauseManager pause.Manager
	pauseCallback *list.Element[pause.Callback]
	ticker       *time.Ticker
	tickerAccess sync.Mutex
	close        chan struct{}
}

// candidateSnapshot holds immutable snapshot of candidate pools
type candidateSnapshot struct {
	primaryCandidates []adapter.Outbound // Top-N primary nodes
	backupCandidates  []adapter.Outbound // Top-N backup nodes
	activeTier        string              // "primary" or "backup"

	// Consistent hash ring (only for consistent_hash strategy)
	hashRing *consistentHashRing
}

// tierStateSnapshot tracks hysteresis state
type tierStateSnapshot struct {
	activeTier          string    // Current active tier
	primaryFailureCount uint32    // Consecutive primary tier failures
	backupActivatedAt   time.Time // When backup tier was activated
}

// consistentHashRing implements NGINX-style consistent hashing
type consistentHashRing struct {
	points       []uint64           // Sorted hash points
	nodeMap      map[uint64]string  // Point -> node tag
	members      []string           // Current member tags (for rebuilding detection)
	virtualNodes int                // Virtual nodes per real node
}

// nodeStat holds health check result for a node
type nodeStat struct {
	tag     string
	delay   uint16
	failure bool
}

func RegisterLoadBalance(registry *outbound.Registry) {
	outbound.Register[option.LoadBalanceOutboundOptions](registry, C.TypeLoadBalance, NewLoadBalance)
}

func NewLoadBalance(
	ctx context.Context,
	router adapter.Router,
	logger log.ContextLogger,
	tag string,
	options option.LoadBalanceOutboundOptions,
) (adapter.Outbound, error) {
	outboundManager := service.FromContext[adapter.OutboundManager](ctx)
	if outboundManager == nil {
		return nil, E.New("missing outbound manager")
	}

	if len(options.PrimaryOutbounds) == 0 {
		return nil, E.New("primary_outbounds is required")
	}

	if options.TopN.Primary <= 0 {
		return nil, E.New("top_n.primary must be > 0")
	}

	if options.Strategy != strategyRandom && options.Strategy != strategyConsistentHash {
		return nil, E.New("strategy must be 'random' or 'consistent_hash'")
	}

	if options.Strategy == strategyConsistentHash && options.Hash == nil {
		return nil, E.New("hash configuration required for consistent_hash strategy")
	}

	if options.EmptyPoolAction != "" &&
	   options.EmptyPoolAction != emptyPoolActionError &&
	   options.EmptyPoolAction != emptyPoolActionFallbackAll {
		return nil, E.New("empty_pool_action must be 'error' or 'fallback_all'")
	}

	// Collect all tags for dependencies
	allTags := append([]string{}, options.PrimaryOutbounds...)
	allTags = append(allTags, options.BackupOutbounds...)

	lb := &LoadBalance{
		Adapter:    outbound.NewAdapter(C.TypeLoadBalance, tag, []string{N.NetworkTCP, N.NetworkUDP}, allTags),
		ctx:        ctx,
		router:     router,
		outbound:   outboundManager,
		logger:     logger,
		primaryTags: options.PrimaryOutbounds,
		backupTags:  options.BackupOutbounds,
		link:        options.URL,
		interval:    time.Duration(options.Interval),
		timeout:     time.Duration(options.Timeout),
		idleTimeout: time.Duration(options.IdleTimeout),
		topNPrimary: options.TopN.Primary,
		topNBackup:  options.TopN.Backup,
		strategy:    options.Strategy,
		emptyPoolAction: options.EmptyPoolAction,
		interruptExternalConnections: options.InterruptExistConnections,
		close:       make(chan struct{}),
	}

	// Set defaults
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
	if lb.topNBackup == 0 {
		if len(lb.backupTags) > 0 {
			lb.topNBackup = len(lb.backupTags)
		}
	}
	if lb.emptyPoolAction == "" {
		lb.emptyPoolAction = defaultEmptyPoolAction
	}

	// Hash configuration
	if options.Hash != nil {
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

	// Hysteresis configuration
	if options.Hysteresis != nil {
		lb.hystPrimaryFailures = options.Hysteresis.PrimaryFailures
		lb.hystBackupHoldTime = time.Duration(options.Hysteresis.BackupHoldTime)
	}
	if lb.hystPrimaryFailures == 0 {
		lb.hystPrimaryFailures = defaultPrimaryFailures
	}
	if lb.hystBackupHoldTime == 0 {
		lb.hystBackupHoldTime = defaultBackupHoldTime
	}

	// Initialize tier state
	initialTierState := &tierStateSnapshot{
		activeTier: "primary",
	}
	lb.tierState.Store(initialTierState)

	// Check for duplicates
	tagSet := make(map[string]bool)
	for _, tag := range allTags {
		if tagSet[tag] {
			return nil, E.New("duplicate outbound tag: ", tag)
		}
		tagSet[tag] = true
	}

	if lb.interruptExternalConnections {
		lb.interruptGroup = interrupt.NewGroup()
	}

	connectionManager := service.FromContext[adapter.ConnectionManager](ctx)
	if connectionManager != nil {
		lb.connection = connectionManager
	}

	return lb, nil
}

func (lb *LoadBalance) Network() []string {
	if lb.candidateState.Load() == nil {
		return []string{N.NetworkTCP, N.NetworkUDP}
	}

	snapshot := lb.candidateState.Load().(*candidateSnapshot)

	// Determine which candidates to check based on active tier
	var candidates []adapter.Outbound
	if snapshot.activeTier == "primary" && len(snapshot.primaryCandidates) > 0 {
		candidates = snapshot.primaryCandidates
	} else if len(snapshot.backupCandidates) > 0 {
		candidates = snapshot.backupCandidates
	}

	if len(candidates) == 0 {
		return []string{N.NetworkTCP, N.NetworkUDP}
	}

	// Return intersection of all candidate networks
	networks := make(map[string]bool)
	for _, network := range candidates[0].Network() {
		networks[network] = true
	}

	for _, candidate := range candidates[1:] {
		candidateNetworks := make(map[string]bool)
		for _, network := range candidate.Network() {
			candidateNetworks[network] = true
		}
		// Keep only common networks
		for network := range networks {
			if !candidateNetworks[network] {
				delete(networks, network)
			}
		}
	}

	result := make([]string, 0, len(networks))
	for network := range networks {
		result = append(result, network)
	}
	return result
}

func (lb *LoadBalance) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}

	// Retrieve shared history storage
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

func (lb *LoadBalance) PostStart() error {
	// Perform initial synchronous health check to ensure candidateState is initialized
	// before any connections are attempted
	lb.performHealthCheck(lb.ctx)
	return nil
}

func (lb *LoadBalance) Close() error {
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

// Touch starts or resets the idle timeout for health checking
func (lb *LoadBalance) Touch() {
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

func (lb *LoadBalance) loopCheck() {
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
			// Perform health check
			go lb.performHealthCheck(lb.ctx)
			idleTimer.Reset(lb.idleTimeout)
		}
	}
}

// performHealthCheck executes URLTest for all configured outbounds
func (lb *LoadBalance) performHealthCheck(ctx context.Context) {
	if lb.checking.Swap(true) {
		return
	}
	defer lb.checking.Store(false)

	if lb.pauseManager != nil && lb.pauseManager.IsPaused() {
		return
	}

	allTags := append([]string{}, lb.primaryTags...)
	allTags = append(allTags, lb.backupTags...)

	// Collect outbounds
	outbounds := make([]adapter.Outbound, 0, len(allTags))
	for _, tag := range allTags {
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

	// Perform health checks in batches
	batchSize := 10
	resultChan := make(chan nodeStat, len(outbounds))

	for i := 0; i < len(outbounds); i += batchSize {
		end := i + batchSize
		if end > len(outbounds) {
			end = len(outbounds)
		}

		var wg sync.WaitGroup
		for _, detour := range outbounds[i:end] {
			wg.Add(1)
			go func(d adapter.Outbound) {
				defer wg.Done()

				// Create context with timeout
				testCtx, cancel := context.WithTimeout(ctx, lb.timeout)
				defer cancel()

				t, err := urltest.URLTest(testCtx, lb.link, d)
				if err != nil {
					lb.logger.Debug("health check failed for ", d.Tag(), ": ", err)
					resultChan <- nodeStat{tag: d.Tag(), failure: true}
				} else {
					lb.history.StoreURLTestHistory(RealTag(d), &adapter.URLTestHistory{
						Time:  time.Now(),
						Delay: t,
					})
					resultChan <- nodeStat{tag: d.Tag(), delay: t}
				}
			}(detour)
		}
		wg.Wait()
	}
	close(resultChan)

	// Update candidate pools
	lb.updateCandidates()
}

// updateCandidates rebuilds candidate pools based on current health check results
func (lb *LoadBalance) updateCandidates() {
	// Collect health stats for primary tier
	primaryStats := lb.collectTierStats(lb.primaryTags)

	// Collect health stats for backup tier
	backupStats := lb.collectTierStats(lb.backupTags)

	// Select Top-N candidates per tier
	primaryCandidates := lb.selectTopN(primaryStats, lb.topNPrimary)
	backupCandidates := lb.selectTopN(backupStats, lb.topNBackup)

	// Apply hysteresis to determine active tier
	currentTierState := lb.tierState.Load().(*tierStateSnapshot)
	newTierState := lb.applyHysteresis(currentTierState, primaryCandidates, backupCandidates)
	lb.tierState.Store(newTierState)

	// Build new snapshot
	newSnapshot := &candidateSnapshot{
		primaryCandidates: primaryCandidates,
		backupCandidates:  backupCandidates,
		activeTier:        newTierState.activeTier,
	}

	// Build consistent hash ring if needed
	if lb.strategy == strategyConsistentHash {
		var ringMembers []adapter.Outbound
		if newTierState.activeTier == "primary" && len(primaryCandidates) > 0 {
			ringMembers = primaryCandidates
		} else if len(backupCandidates) > 0 {
			ringMembers = backupCandidates
		}

		if len(ringMembers) > 0 {
			// Check if ring needs rebuilding
			oldSnapshot := lb.candidateState.Load()
			var needRebuild bool
			if oldSnapshot == nil {
				needRebuild = true
			} else {
				oldRing := oldSnapshot.(*candidateSnapshot).hashRing
				if oldRing == nil {
					needRebuild = true
				} else {
					// Check membership change
					needRebuild = !lb.sameMembership(oldRing.members, ringMembers)
				}
			}

			if needRebuild {
				newSnapshot.hashRing = lb.buildHashRing(ringMembers)
			} else {
				// Reuse existing ring
				newSnapshot.hashRing = oldSnapshot.(*candidateSnapshot).hashRing
			}
		}
	}

	// Atomically update snapshot
	oldSnapshot := lb.candidateState.Swap(newSnapshot)

	// Log tier changes
	if oldSnapshot != nil {
		oldTier := oldSnapshot.(*candidateSnapshot).activeTier
		if oldTier != newTierState.activeTier {
			lb.logger.Info(
				"tier switch: ", oldTier, " -> ", newTierState.activeTier,
				", primary candidates: ", len(primaryCandidates),
				", backup candidates: ", len(backupCandidates),
			)
		}
	}

	// Log candidate details
	lb.logCandidates("primary", primaryCandidates, primaryStats)
	lb.logCandidates("backup", backupCandidates, backupStats)

	// Interrupt existing connections if configured
	if lb.interruptExternalConnections && oldSnapshot != nil {
		oldActive := oldSnapshot.(*candidateSnapshot).activeTier
		if oldActive != newTierState.activeTier {
			if lb.interruptGroup != nil {
				lb.interruptGroup.Interrupt(false)
			}
		}
	}
}

// collectTierStats retrieves health stats for a tier's outbounds
func (lb *LoadBalance) collectTierStats(tags []string) []nodeStat {
	stats := make([]nodeStat, 0, len(tags))

	for _, tag := range tags {
		detour, loaded := lb.outbound.Outbound(tag)
		if !loaded {
			continue
		}

		history := lb.history.LoadURLTestHistory(RealTag(detour))
		if history == nil {
			// No history = not yet probed, treat as failure
			stats = append(stats, nodeStat{tag: tag, failure: true})
			continue
		}

		// Check if history is stale (older than interval * 2)
		if time.Since(history.Time) > lb.interval*2 {
			stats = append(stats, nodeStat{tag: tag, failure: true})
			continue
		}

		stats = append(stats, nodeStat{tag: tag, delay: history.Delay})
	}

	return stats
}

// selectTopN selects the lowest-latency N successful nodes from stats
func (lb *LoadBalance) selectTopN(stats []nodeStat, n int) []adapter.Outbound {
	// Filter successful nodes
	successNodes := make([]nodeStat, 0, len(stats))
	for _, stat := range stats {
		if !stat.failure {
			successNodes = append(successNodes, stat)
		}
	}

	if len(successNodes) == 0 {
		return nil
	}

	// Sort by delay
	sort.Slice(successNodes, func(i, j int) bool {
		return successNodes[i].delay < successNodes[j].delay
	})

	// Take Top-N
	topN := n
	if topN > len(successNodes) {
		topN = len(successNodes)
	}

	result := make([]adapter.Outbound, 0, topN)
	for i := 0; i < topN; i++ {
		detour, loaded := lb.outbound.Outbound(successNodes[i].tag)
		if loaded {
			result = append(result, detour)
		}
	}

	return result
}

// applyHysteresis applies hysteresis logic to tier selection
func (lb *LoadBalance) applyHysteresis(
	current *tierStateSnapshot,
	primaryCandidates, backupCandidates []adapter.Outbound,
) *tierStateSnapshot {
	newState := &tierStateSnapshot{
		activeTier:          current.activeTier,
		primaryFailureCount: current.primaryFailureCount,
		backupActivatedAt:   current.backupActivatedAt,
	}

	primaryAvailable := len(primaryCandidates) > 0
	backupAvailable := len(backupCandidates) > 0

	switch current.activeTier {
	case "primary":
		if !primaryAvailable {
			// Primary tier failed
			newState.primaryFailureCount++
			lb.logger.Debug(
				"primary tier failure ", newState.primaryFailureCount, "/", lb.hystPrimaryFailures,
			)

			if newState.primaryFailureCount >= lb.hystPrimaryFailures {
				// Switch to backup tier
				if backupAvailable {
					newState.activeTier = "backup"
					newState.backupActivatedAt = time.Now()
					newState.primaryFailureCount = 0
					lb.logger.Warn("switching to backup tier after ", lb.hystPrimaryFailures, " failures")
				} else {
					lb.logger.Error("primary tier failed but no backup candidates available")
				}
			}
		} else {
			// Primary tier recovered
			newState.primaryFailureCount = 0
		}

	case "backup":
		if primaryAvailable {
			// Check if backup hold time has elapsed
			if time.Since(current.backupActivatedAt) >= lb.hystBackupHoldTime {
				newState.activeTier = "primary"
				newState.primaryFailureCount = 0
				lb.logger.Info("switching back to primary tier after hold time")
			} else {
				lb.logger.Debug(
					"primary tier available but backup hold time not elapsed: ",
					time.Since(current.backupActivatedAt), "/", lb.hystBackupHoldTime,
				)
			}
		} else if !backupAvailable {
			// Both tiers failed
			lb.logger.Error("backup tier failed and primary tier still unavailable")
		}
	}

	return newState
}

// sameMembership checks if two outbound lists have the same members (order-independent)
func (lb *LoadBalance) sameMembership(oldMembers []string, newMembers []adapter.Outbound) bool {
	if len(oldMembers) != len(newMembers) {
		return false
	}

	oldSet := make(map[string]bool)
	for _, tag := range oldMembers {
		oldSet[tag] = true
	}

	for _, outbound := range newMembers {
		if !oldSet[outbound.Tag()] {
			return false
		}
	}

	return true
}

// buildHashRing constructs a consistent hash ring for the given members
func (lb *LoadBalance) buildHashRing(members []adapter.Outbound) *consistentHashRing {
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

// lookupHashRing finds the node for a given key hash using consistent hashing
func (lb *LoadBalance) lookupHashRing(ring *consistentHashRing, keyHash uint64) string {
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

// buildHashKey constructs hash key from connection metadata
func (lb *LoadBalance) buildHashKey(metadata *adapter.InboundContext) string {
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
			} else if metadata.Domain != "" {
				parts = append(parts, metadata.Domain)
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
			// Use the ruleset tag that matched this connection (if any)
			if metadata.MatchedRuleSet != "" {
				parts = append(parts, metadata.MatchedRuleSet)
			} else {
				parts = append(parts, "-")
			}
		case "etld_plus_one":
			// Extract eTLD+1 (effective TLD + 1) from the domain
			// e.g., a.b.example.com -> example.com, a.b.example.co.uk -> example.co.uk
			var rawDomain string
			if metadata.Destination.IsFqdn() {
				rawDomain = metadata.Destination.Fqdn
			} else if metadata.Domain != "" {
				rawDomain = metadata.Domain
			}
			etldPlusOne := domain.ExtractETLDPlusOne(rawDomain)
			parts = append(parts, etldPlusOne)
		case "matched_ruleset_or_etld":
			// Priority: use matched ruleset if available, otherwise fall back to eTLD+1
			// This enables unified hashing for both ruleset-matched and direct domain connections
			// Use case: route by content category (ruleset) or domain grouping (eTLD+1)
			if metadata.MatchedRuleSet != "" {
				// Ruleset matched - use it for hashing (same behavior as matched_ruleset)
				parts = append(parts, metadata.MatchedRuleSet)
			} else {
				// No ruleset match - fall back to domain-based hashing
				// Extract eTLD+1 from domain (same behavior as etld_plus_one)
				var rawDomain string
				if metadata.Destination.IsFqdn() {
					rawDomain = metadata.Destination.Fqdn
				} else if metadata.Domain != "" {
					rawDomain = metadata.Domain
				}
				etldPlusOne := domain.ExtractETLDPlusOne(rawDomain)
				parts = append(parts, etldPlusOne)
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

// selectOutbound selects an outbound from the current candidate pool
func (lb *LoadBalance) selectOutbound(network string, metadata *adapter.InboundContext) (adapter.Outbound, error) {
	// Touch to maintain activity
	lb.Touch()

	snapshot := lb.candidateState.Load()
	if snapshot == nil {
		return nil, E.New("no candidates available (not initialized)")
	}

	cs := snapshot.(*candidateSnapshot)

	// Determine active candidates based on tier
	var candidates []adapter.Outbound
	if cs.activeTier == "primary" && len(cs.primaryCandidates) > 0 {
		candidates = cs.primaryCandidates
	} else if len(cs.backupCandidates) > 0 {
		candidates = cs.backupCandidates
	}

	// Handle empty pool
	if len(candidates) == 0 {
		if lb.emptyPoolAction == emptyPoolActionFallbackAll {
			// Fallback to all configured outbounds (ignore health)
			lb.logger.Warn("both tiers empty, falling back to all outbounds")
			allTags := append([]string{}, lb.primaryTags...)
			allTags = append(allTags, lb.backupTags...)

			for _, tag := range allTags {
				detour, loaded := lb.outbound.Outbound(tag)
				if loaded && common.Contains(detour.Network(), network) {
					candidates = append(candidates, detour)
				}
			}

			if len(candidates) == 0 {
				return nil, E.New("no outbounds available for network ", network)
			}
		} else {
			return nil, E.New("no healthy candidates available in any tier")
		}
	}

	// Filter by network support
	networkCandidates := make([]adapter.Outbound, 0, len(candidates))
	for _, candidate := range candidates {
		if common.Contains(candidate.Network(), network) {
			networkCandidates = append(networkCandidates, candidate)
		}
	}

	if len(networkCandidates) == 0 {
		return nil, E.New("no candidates support network ", network)
	}

	// Select based on strategy
	var selected adapter.Outbound

	switch lb.strategy {
	case strategyRandom:
		selected = networkCandidates[rand.Intn(len(networkCandidates))]
		lb.logger.Debug(
			"random selection: tier=", cs.activeTier,
			", selected=", selected.Tag(),
			", pool_size=", len(networkCandidates),
		)

	case strategyConsistentHash:
		if cs.hashRing == nil || len(cs.hashRing.points) == 0 {
			// Fallback to random if ring not built
			selected = networkCandidates[rand.Intn(len(networkCandidates))]
			lb.logger.Warn("hash ring not available, using random selection")
		} else {
			// Build hash key
			hashKey := lb.buildHashKey(metadata)

			if hashKey == "" {
				// Handle empty key based on configuration
				if lb.hashOnEmptyKey == onEmptyKeyRandom {
					selected = networkCandidates[rand.Intn(len(networkCandidates))]
					lb.logger.Debug("empty hash key, using random selection")
				} else {
					// Hash empty string
					keyHash := xxhash.Sum64String("")
					nodeTag := lb.lookupHashRing(cs.hashRing, keyHash)

					// Find outbound with this tag in network candidates
					for _, candidate := range networkCandidates {
						if candidate.Tag() == nodeTag {
							selected = candidate
							break
						}
					}

					if selected == nil {
						// Tag not in network candidates, fallback to random
						selected = networkCandidates[rand.Intn(len(networkCandidates))]
						lb.logger.Debug("hash target not in network candidates, using random")
					}
				}
			} else {
				// Hash the key and lookup
				keyHash := xxhash.Sum64String(hashKey)
				nodeTag := lb.lookupHashRing(cs.hashRing, keyHash)

				// Find outbound with this tag in network candidates
				for _, candidate := range networkCandidates {
					if candidate.Tag() == nodeTag {
						selected = candidate
						break
					}
				}

				if selected == nil {
					// Tag not in network candidates, fallback to random
					selected = networkCandidates[rand.Intn(len(networkCandidates))]
					lb.logger.Debug("hash target not in network candidates, using random")
				} else {
					lb.logger.Debug(
						"consistent_hash selection: tier=", cs.activeTier,
						", key=", hashKey,
						", key_hash=", keyHash,
						", selected=", selected.Tag(),
					)
				}
			}
		}
	}

	if selected == nil {
		return nil, E.New("selection algorithm failed")
	}

	return selected, nil
}

func (lb *LoadBalance) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	metadata := adapter.ContextFrom(ctx)
	if metadata == nil {
		metadata = &adapter.InboundContext{}
	}

	selected, err := lb.selectOutbound(network, metadata)
	if err != nil {
		return nil, err
	}

	conn, err := selected.DialContext(ctx, network, destination)
	if err != nil {
		return nil, err
	}

	if lb.interruptGroup != nil {
		return lb.interruptGroup.NewConn(conn, interrupt.IsExternalConnectionFromContext(ctx)), nil
	}

	return conn, nil
}

func (lb *LoadBalance) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	metadata := adapter.ContextFrom(ctx)
	if metadata == nil {
		metadata = &adapter.InboundContext{}
	}

	selected, err := lb.selectOutbound(N.NetworkUDP, metadata)
	if err != nil {
		return nil, err
	}

	conn, err := selected.ListenPacket(ctx, destination)
	if err != nil {
		return nil, err
	}

	if lb.interruptGroup != nil {
		return lb.interruptGroup.NewPacketConn(conn, interrupt.IsExternalConnectionFromContext(ctx)), nil
	}

	return conn, nil
}

// OutboundGroup interface implementation
func (lb *LoadBalance) Now() string {
	snapshot := lb.candidateState.Load()
	if snapshot == nil {
		return ""
	}

	cs := snapshot.(*candidateSnapshot)

	var candidates []adapter.Outbound
	if cs.activeTier == "primary" && len(cs.primaryCandidates) > 0 {
		candidates = cs.primaryCandidates
	} else if len(cs.backupCandidates) > 0 {
		candidates = cs.backupCandidates
	}

	if len(candidates) == 0 {
		return ""
	}

	// Return first candidate as representative
	return candidates[0].Tag()
}

func (lb *LoadBalance) All() []string {
	allTags := append([]string{}, lb.primaryTags...)
	allTags = append(allTags, lb.backupTags...)
	return allTags
}

// logCandidates logs detailed candidate information
func (lb *LoadBalance) logCandidates(tierName string, candidates []adapter.Outbound, stats []nodeStat) {
	if len(candidates) == 0 {
		lb.logger.Debug(tierName, " tier: 0 candidates")
		return
	}

	// Build delay map
	delayMap := make(map[string]uint16)
	for _, stat := range stats {
		if !stat.failure {
			delayMap[stat.tag] = stat.delay
		}
	}

	tags := make([]string, len(candidates))
	for i, c := range candidates {
		delay := delayMap[c.Tag()]
		tags[i] = fmt.Sprintf("%s(%dms)", c.Tag(), delay)
	}

	lb.logger.Info(
		tierName, " tier: ", len(candidates), " candidates: [",
		strings.Join(tags, ", "),
		"]",
	)
}
