package group

import (
	"context"
	"math/rand"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/hash"
	"github.com/sagernet/sing-box/common/interrupt"
	"github.com/sagernet/sing-box/common/urltest"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/atomic"
	"github.com/sagernet/sing/common/batch"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/pause"
)

var (
	_ adapter.Outbound      = (*LoadBalance)(nil)
	_ adapter.OutboundGroup = (*LoadBalance)(nil)
	_ adapter.URLTestGroup  = (*LoadBalance)(nil)
)

func RegisterLoadBalance(registry *outbound.Registry) {
	outbound.Register[option.LoadBalanceOutboundOptions](registry, C.TypeLoadBalance, NewLoadBalance)
}

type LoadBalance struct {
	outbound.Adapter
	ctx                          context.Context
	router                       adapter.Router
	outbound                     adapter.OutboundManager
	connection                   adapter.ConnectionManager
	logger                       log.ContextLogger
	primaryTags                  []string
	backupTags                   []string
	link                         string
	interval                     time.Duration
	timeout                      time.Duration
	idleTimeout                  time.Duration
	strategy                     string
	topNPrimary                  int
	topNBackup                   int
	hashKeyParts                 []string
	virtualNodes                 int
	onEmptyKey                   string
	salt                         string
	primaryFailures              int
	backupHoldTime               time.Duration
	emptyPoolAction              string
	group                        *LoadBalanceGroup
	interruptExternalConnections bool
}

func NewLoadBalance(
	ctx context.Context,
	router adapter.Router,
	logger log.ContextLogger,
	tag string,
	options option.LoadBalanceOutboundOptions,
) (adapter.Outbound, error) {
	if len(options.PrimaryOutbounds) == 0 {
		return nil, E.New("missing primary outbounds")
	}

	strategy := options.Strategy
	if strategy == "" {
		strategy = "random" // default strategy
	}
	if strategy != "random" && strategy != "consistent_hash" {
		return nil, E.New("invalid strategy: ", strategy)
	}

	emptyPoolAction := options.EmptyPoolAction
	if emptyPoolAction == "" {
		emptyPoolAction = "error" // default: fail closed
	}
	if emptyPoolAction != "error" && emptyPoolAction != "fallback_all" {
		return nil, E.New("invalid empty_pool_action: ", emptyPoolAction)
	}

	// Default Top-N values (0 = all)
	topNPrimary := options.TopN.Primary
	topNBackup := options.TopN.Backup

	// Hash options
	var hashKeyParts []string
	virtualNodes := 10 // default
	onEmptyKey := "random"
	salt := ""

	if options.Hash != nil {
		hashKeyParts = options.Hash.KeyParts
		if options.Hash.VirtualNodes > 0 {
			virtualNodes = options.Hash.VirtualNodes
		}
		if options.Hash.OnEmptyKey != "" {
			onEmptyKey = options.Hash.OnEmptyKey
			if onEmptyKey != "random" && onEmptyKey != "hash_empty" {
				return nil, E.New("invalid on_empty_key: ", onEmptyKey)
			}
		}
		salt = options.Hash.Salt
	}

	// Hysteresis options
	primaryFailures := 3               // default: 3 consecutive failures
	backupHoldTime := 30 * time.Second // default: 30 seconds

	if options.Hysteresis != nil {
		if options.Hysteresis.PrimaryFailures > 0 {
			primaryFailures = options.Hysteresis.PrimaryFailures
		}
		if options.Hysteresis.BackupHoldTime > 0 {
			backupHoldTime = time.Duration(options.Hysteresis.BackupHoldTime)
		}
	}

	// URL test options
	link := options.URL
	if link == "" {
		link = "https://www.gstatic.com/generate_204"
	}
	interval := time.Duration(options.Interval)
	if interval == 0 {
		interval = 3 * time.Minute
	}
	timeout := time.Duration(options.Timeout)
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	idleTimeout := time.Duration(options.IdleTimeout)
	if idleTimeout == 0 {
		idleTimeout = 30 * time.Minute
	}

	allTags := append(options.PrimaryOutbounds, options.BackupOutbounds...)

	outboundAdapter := outbound.NewAdapter(
		C.TypeLoadBalance,
		tag,
		common.Map(allTags, func(it string) string {
			return N.NetworkTCP
		}),
		options.PrimaryOutbounds,
	)

	lb := &LoadBalance{
		Adapter:                      outboundAdapter,
		ctx:                          ctx,
		router:                       router,
		outbound:                     service.FromContext[adapter.OutboundManager](ctx),
		connection:                   service.FromContext[adapter.ConnectionManager](ctx),
		logger:                       logger,
		primaryTags:                  options.PrimaryOutbounds,
		backupTags:                   options.BackupOutbounds,
		link:                         link,
		interval:                     interval,
		timeout:                      timeout,
		idleTimeout:                  idleTimeout,
		strategy:                     strategy,
		topNPrimary:                  topNPrimary,
		topNBackup:                   topNBackup,
		hashKeyParts:                 hashKeyParts,
		virtualNodes:                 virtualNodes,
		onEmptyKey:                   onEmptyKey,
		salt:                         salt,
		primaryFailures:              primaryFailures,
		backupHoldTime:               backupHoldTime,
		emptyPoolAction:              emptyPoolAction,
		interruptExternalConnections: options.InterruptExistConnections,
	}

	lb.group = NewLoadBalanceGroup(
		ctx,
		router,
		lb.outbound,
		lb.connection,
		logger,
		allTags,
		options.PrimaryOutbounds,
		options.BackupOutbounds,
		link,
		interval,
		timeout,
		idleTimeout,
		strategy,
		topNPrimary,
		topNBackup,
		hashKeyParts,
		virtualNodes,
		onEmptyKey,
		salt,
		primaryFailures,
		backupHoldTime,
		emptyPoolAction,
	)

	return lb, nil
}

func (lb *LoadBalance) Network() []string {
	// Dynamic network support based on available outbounds
	if lb.group != nil && lb.group.initialized.Load() {
		return lb.group.Network()
	}
	return []string{N.NetworkTCP, N.NetworkUDP}
}

func (lb *LoadBalance) Start() error {
	return lb.group.Start()
}

func (lb *LoadBalance) PostStart() error {
	return lb.group.PostStart()
}

func (lb *LoadBalance) Close() error {
	return lb.group.Close()
}

func (lb *LoadBalance) Now() string {
	return lb.group.Now()
}

func (lb *LoadBalance) All() []string {
	return lb.group.All()
}

func (lb *LoadBalance) URLTest(ctx context.Context) (map[string]uint16, error) {
	return lb.group.URLTest(ctx)
}

func (lb *LoadBalance) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	conn, err := lb.group.DialContext(ctx, network, destination)
	if err != nil {
		return nil, err
	}
	if lb.interruptExternalConnections {
		return lb.group.interruptGroup.NewConn(conn, interrupt.IsExternalConnectionFromContext(ctx)), nil
	}
	return conn, nil
}

func (lb *LoadBalance) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	conn, err := lb.group.ListenPacket(ctx, destination)
	if err != nil {
		return nil, err
	}
	if lb.interruptExternalConnections {
		return lb.group.interruptGroup.NewPacketConn(conn, interrupt.IsExternalConnectionFromContext(ctx)), nil
	}
	return conn, nil
}

func (lb *LoadBalance) NewConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	outbound, err := lb.group.Select(ctx, metadata, N.NetworkTCP)
	if err != nil {
		N.CloseOnHandshakeFailure(conn, onClose, err)
		lb.logger.ErrorContext(ctx, err)
		return
	}
	if connHandler, ok := outbound.(adapter.ConnectionHandlerEx); ok {
		connHandler.NewConnectionEx(ctx, conn, metadata, onClose)
	} else {
		N.CloseOnHandshakeFailure(conn, onClose, E.New("outbound ", outbound.Tag(), " does not support connection handling"))
		lb.logger.ErrorContext(ctx, "outbound ", outbound.Tag(), " does not support connection handling")
	}
}

func (lb *LoadBalance) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	outbound, err := lb.group.Select(ctx, metadata, N.NetworkUDP)
	if err != nil {
		N.CloseOnHandshakeFailure(conn, onClose, err)
		lb.logger.ErrorContext(ctx, err)
		return
	}
	if packetHandler, ok := outbound.(adapter.PacketConnectionHandlerEx); ok {
		packetHandler.NewPacketConnectionEx(ctx, conn, metadata, onClose)
	} else {
		N.CloseOnHandshakeFailure(conn, onClose, E.New("outbound ", outbound.Tag(), " does not support packet connection handling"))
		lb.logger.ErrorContext(ctx, "outbound ", outbound.Tag(), " does not support packet connection handling")
	}
}

// LoadBalanceGroup manages the load balancing logic
type LoadBalanceGroup struct {
	ctx                  context.Context
	router               adapter.Router
	outbound             adapter.OutboundManager
	connection           adapter.ConnectionManager
	logger               log.Logger
	allTags              []string
	primaryTags          []string
	backupTags           []string
	allOutbounds         []adapter.Outbound
	primaryOutbounds     []adapter.Outbound
	backupOutbounds      []adapter.Outbound
	link                 string
	interval             time.Duration
	timeout              time.Duration
	idleTimeout          time.Duration
	strategy             string
	topNPrimary          int
	topNBackup           int
	hashKeyParts         []string
	hashRing             *hash.HashRing
	virtualNodes         int
	onEmptyKey           string
	salt                 string
	primaryFailures      int
	backupHoldTime       time.Duration
	emptyPoolAction      string
	history              *urltest.HistoryStorage
	checking             atomic.Bool
	pauseManager         pause.Manager
	selectedCandidates   atomic.TypedValue[[]*loadBalanceCandidate]
	usingBackupTier      atomic.Bool
	consecutiveFailures  atomic.Uint32
	lastBackupActivation sync.Mutex
	lastBackupTime       time.Time
	initialized          atomic.Bool
	interruptGroup       *interrupt.Group
	closeOnce            sync.Once
	closeChan            chan struct{}
}

type loadBalanceCandidate struct {
	outbound adapter.Outbound
	tag      string
}

func NewLoadBalanceGroup(
	ctx context.Context,
	router adapter.Router,
	outboundManager adapter.OutboundManager,
	connection adapter.ConnectionManager,
	logger log.Logger,
	allTags []string,
	primaryTags []string,
	backupTags []string,
	link string,
	interval time.Duration,
	timeout time.Duration,
	idleTimeout time.Duration,
	strategy string,
	topNPrimary int,
	topNBackup int,
	hashKeyParts []string,
	virtualNodes int,
	onEmptyKey string,
	salt string,
	primaryFailures int,
	backupHoldTime time.Duration,
	emptyPoolAction string,
) *LoadBalanceGroup {
	var hashRing *hash.HashRing
	if strategy == "consistent_hash" {
		hashRing = hash.NewHashRing(virtualNodes)
	}

	return &LoadBalanceGroup{
		ctx:             ctx,
		router:          router,
		outbound:        outboundManager,
		connection:      connection,
		logger:          logger,
		allTags:         allTags,
		primaryTags:     primaryTags,
		backupTags:      backupTags,
		link:            link,
		interval:        interval,
		timeout:         timeout,
		idleTimeout:     idleTimeout,
		strategy:        strategy,
		topNPrimary:     topNPrimary,
		topNBackup:      topNBackup,
		hashKeyParts:    hashKeyParts,
		hashRing:        hashRing,
		virtualNodes:    virtualNodes,
		onEmptyKey:      onEmptyKey,
		salt:            salt,
		primaryFailures: primaryFailures,
		backupHoldTime:  backupHoldTime,
		emptyPoolAction: emptyPoolAction,
		history:         urltest.NewHistoryStorage(),
		pauseManager:    service.FromContext[pause.Manager](ctx),
		interruptGroup:  interrupt.NewGroup(),
		closeChan:       make(chan struct{}),
	}
}

func (g *LoadBalanceGroup) Start() error {
	outbounds := make([]adapter.Outbound, 0, len(g.allTags))
	primaryOutbounds := make([]adapter.Outbound, 0, len(g.primaryTags))
	backupOutbounds := make([]adapter.Outbound, 0, len(g.backupTags))

	for _, tag := range g.allTags {
		outbound, loaded := g.outbound.Outbound(tag)
		if !loaded {
			return E.New("outbound not found: ", tag)
		}
		outbounds = append(outbounds, outbound)
	}

	for _, tag := range g.primaryTags {
		outbound, loaded := g.outbound.Outbound(tag)
		if !loaded {
			return E.New("primary outbound not found: ", tag)
		}
		primaryOutbounds = append(primaryOutbounds, outbound)
	}

	for _, tag := range g.backupTags {
		outbound, loaded := g.outbound.Outbound(tag)
		if !loaded {
			return E.New("backup outbound not found: ", tag)
		}
		backupOutbounds = append(backupOutbounds, outbound)
	}

	g.allOutbounds = outbounds
	g.primaryOutbounds = primaryOutbounds
	g.backupOutbounds = backupOutbounds

	// Bootstrap mode: Use all primary outbounds initially
	g.setBootstrapCandidates()

	return nil
}

func (g *LoadBalanceGroup) PostStart() error {
	// Start background health checking
	go g.urlTestLoop()
	return nil
}

func (g *LoadBalanceGroup) Close() error {
	g.closeOnce.Do(func() {
		close(g.closeChan)
	})
	return nil
}

func (g *LoadBalanceGroup) Network() []string {
	candidates := g.selectedCandidates.Load()
	if candidates == nil || len(candidates) == 0 {
		return []string{N.NetworkTCP, N.NetworkUDP}
	}

	supportTCP := false
	supportUDP := false

	for _, candidate := range candidates {
		networks := candidate.outbound.Network()
		for _, network := range networks {
			if network == N.NetworkTCP {
				supportTCP = true
			}
			if network == N.NetworkUDP {
				supportUDP = true
			}
		}
	}

	var result []string
	if supportTCP {
		result = append(result, N.NetworkTCP)
	}
	if supportUDP {
		result = append(result, N.NetworkUDP)
	}
	if len(result) == 0 {
		result = []string{N.NetworkTCP, N.NetworkUDP}
	}
	return result
}

func (g *LoadBalanceGroup) Now() string {
	candidates := g.selectedCandidates.Load()
	if candidates == nil || len(candidates) == 0 {
		return ""
	}
	return candidates[0].tag
}

func (g *LoadBalanceGroup) All() []string {
	return g.allTags
}

func (g *LoadBalanceGroup) URLTest(ctx context.Context) (map[string]uint16, error) {
	return g.urlTest(ctx, true)
}

func (g *LoadBalanceGroup) Select(ctx context.Context, metadata adapter.InboundContext, network string) (adapter.Outbound, error) {
	candidates := g.selectedCandidates.Load()

	// Filter by network capability
	networkCandidates := g.filterByNetwork(candidates, network)

	if len(networkCandidates) == 0 {
		// No candidates available
		if g.emptyPoolAction == "fallback_all" {
			// Fallback to all outbounds
			return g.selectFromAll(ctx, metadata, network)
		}
		return nil, E.New("no candidates available for network ", network)
	}

	// Select based on strategy
	switch g.strategy {
	case "random":
		return g.selectRandom(networkCandidates), nil
	case "consistent_hash":
		return g.selectConsistentHash(ctx, metadata, networkCandidates)
	default:
		return nil, E.New("unknown strategy: ", g.strategy)
	}
}

func (g *LoadBalanceGroup) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	metadata := adapter.InboundContext{
		Destination: destination,
		Network:     network,
	}
	if ctxMetadata := adapter.ContextFrom(ctx); ctxMetadata != nil {
		metadata = *ctxMetadata
	}

	outbound, err := g.Select(ctx, metadata, network)
	if err != nil {
		return nil, err
	}

	conn, err := outbound.DialContext(ctx, network, destination)
	if err != nil {
		// Track failure for hysteresis
		g.recordFailure()
	}
	return conn, err
}

func (g *LoadBalanceGroup) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	metadata := adapter.InboundContext{
		Destination: destination,
		Network:     N.NetworkUDP,
	}
	if ctxMetadata := adapter.ContextFrom(ctx); ctxMetadata != nil {
		metadata = *ctxMetadata
	}

	outbound, err := g.Select(ctx, metadata, N.NetworkUDP)
	if err != nil {
		return nil, err
	}

	conn, err := outbound.ListenPacket(ctx, destination)
	if err != nil {
		// Track failure for hysteresis
		g.recordFailure()
	}
	return conn, err
}

func (g *LoadBalanceGroup) setBootstrapCandidates() {
	// Bootstrap mode: use all primary outbounds before first health check
	candidates := make([]*loadBalanceCandidate, 0, len(g.primaryOutbounds))
	for i, outbound := range g.primaryOutbounds {
		candidates = append(candidates, &loadBalanceCandidate{
			outbound: outbound,
			tag:      g.primaryTags[i],
		})
	}

	// Initialize hash ring with all candidates for consistent hash
	if g.strategy == "consistent_hash" {
		for _, candidate := range candidates {
			g.hashRing.Add(candidate.tag)
		}
	}

	g.selectedCandidates.Store(candidates)
	g.initialized.Store(true)
}

func (g *LoadBalanceGroup) urlTestLoop() {
	if g.interval == 0 {
		return
	}

	ticker := time.NewTicker(g.interval)
	defer ticker.Stop()

	// Run first test immediately
	ctx, cancel := context.WithTimeout(g.ctx, g.timeout*time.Duration(len(g.allOutbounds)))
	_, _ = g.urlTest(ctx, false)
	cancel()

	for {
		g.pauseManager.WaitActive()
		select {
		case <-g.closeChan:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(g.ctx, g.timeout*time.Duration(len(g.allOutbounds)))
			_, _ = g.urlTest(ctx, false)
			cancel()
		}
	}
}

func (g *LoadBalanceGroup) urlTest(ctx context.Context, force bool) (map[string]uint16, error) {
	if !force && g.checking.Swap(true) {
		return nil, E.New("already checking")
	}
	defer g.checking.Store(false)

	result := make(map[string]uint16)
	b, _ := batch.New(ctx, batch.WithConcurrencyNum[any](10))

	for _, tag := range g.allTags {
		tag := tag
		outbound, loaded := g.outbound.Outbound(tag)
		if !loaded {
			continue
		}

		b.Go(tag, func() (any, error) {
			t, err := urltest.URLTest(ctx, g.link, outbound)
			if err != nil {
				g.logger.Debug("outbound ", tag, " URL test failed: ", err)
				return nil, nil
			}
			g.logger.Debug("outbound ", tag, " URL test: ", t, "ms")
			g.history.StoreURLTestHistory(tag, &adapter.URLTestHistory{
				Time:  time.Now(),
				Delay: t,
			})
			result[tag] = t
			return nil, nil
		})
	}

	b.Wait()

	// Update candidates based on test results
	g.updateCandidates(result)

	return result, nil
}

func (g *LoadBalanceGroup) updateCandidates(latencies map[string]uint16) {
	// Separate primaries and backups
	primaryLatencies := make(map[string]uint16)
	backupLatencies := make(map[string]uint16)

	for _, tag := range g.primaryTags {
		if latency, ok := latencies[tag]; ok {
			primaryLatencies[tag] = latency
		}
	}

	for _, tag := range g.backupTags {
		if latency, ok := latencies[tag]; ok {
			backupLatencies[tag] = latency
		}
	}

	// Select top-N from each tier
	primaryCandidates := g.selectTopN(g.primaryTags, primaryLatencies, g.topNPrimary)
	backupCandidates := g.selectTopN(g.backupTags, backupLatencies, g.topNBackup)

	// Determine which tier to use based on hysteresis
	var finalCandidates []string
	usingBackup := g.usingBackupTier.Load()

	if !usingBackup {
		// Currently using primary tier
		if len(primaryCandidates) == 0 {
			// No primary candidates available
			failures := g.consecutiveFailures.Add(1)
			if int(failures) >= g.primaryFailures {
				// Switch to backup tier
				g.logger.Warn("all primary outbounds unavailable, switching to backup tier")
				g.usingBackupTier.Store(true)
				g.lastBackupActivation.Lock()
				g.lastBackupTime = time.Now()
				g.lastBackupActivation.Unlock()
				finalCandidates = backupCandidates
			} else {
				// Not enough failures yet, stay with empty primaries
				finalCandidates = []string{}
			}
		} else {
			// Primary candidates available
			g.consecutiveFailures.Store(0)
			finalCandidates = primaryCandidates
		}
	} else {
		// Currently using backup tier
		if len(primaryCandidates) > 0 {
			// Primary candidates available again
			g.lastBackupActivation.Lock()
			lastActivation := g.lastBackupTime
			g.lastBackupActivation.Unlock()
			if time.Since(lastActivation) >= g.backupHoldTime {
				// Hold time elapsed, switch back to primary
				g.logger.Info("primary outbounds available again, switching back from backup tier")
				g.usingBackupTier.Store(false)
				g.consecutiveFailures.Store(0)
				finalCandidates = primaryCandidates
			} else {
				// Still in hold time, stay with backup
				finalCandidates = backupCandidates
			}
		} else {
			// Still no primary candidates
			finalCandidates = backupCandidates
		}
	}

	// Convert tags to candidates
	candidates := make([]*loadBalanceCandidate, 0, len(finalCandidates))
	for _, tag := range finalCandidates {
		outbound, loaded := g.outbound.Outbound(tag)
		if loaded {
			candidates = append(candidates, &loadBalanceCandidate{
				outbound: outbound,
				tag:      tag,
			})
		}
	}

	// Update hash ring if using consistent hash
	if g.strategy == "consistent_hash" {
		// Clear and rebuild hash ring
		existingNodes := g.hashRing.Nodes()
		for _, node := range existingNodes {
			g.hashRing.Remove(node)
		}
		for _, candidate := range candidates {
			g.hashRing.Add(candidate.tag)
		}
	}

	// Store new candidates
	g.selectedCandidates.Store(candidates)

	// Interrupt existing connections if tier changed
	if g.selectedCandidates.Load() != nil && len(candidates) > 0 {
		g.interruptGroup.Interrupt(false)
	}
}

func (g *LoadBalanceGroup) selectTopN(tags []string, latencies map[string]uint16, topN int) []string {
	// If topN is 0, use all
	if topN <= 0 {
		topN = len(tags)
	}

	// Filter tags that have latency results
	type tagLatency struct {
		tag     string
		latency uint16
	}

	available := make([]tagLatency, 0, len(tags))
	for _, tag := range tags {
		if latency, ok := latencies[tag]; ok {
			available = append(available, tagLatency{tag: tag, latency: latency})
		}
	}

	// Sort by latency (ascending)
	sort.Slice(available, func(i, j int) bool {
		return available[i].latency < available[j].latency
	})

	// Take top N
	limit := topN
	if limit > len(available) {
		limit = len(available)
	}

	result := make([]string, limit)
	for i := 0; i < limit; i++ {
		result[i] = available[i].tag
	}

	return result
}

func (g *LoadBalanceGroup) filterByNetwork(candidates []*loadBalanceCandidate, network string) []*loadBalanceCandidate {
	if candidates == nil {
		return nil
	}

	filtered := make([]*loadBalanceCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		networks := candidate.outbound.Network()
		for _, n := range networks {
			if n == network {
				filtered = append(filtered, candidate)
				break
			}
		}
	}
	return filtered
}

func (g *LoadBalanceGroup) selectRandom(candidates []*loadBalanceCandidate) adapter.Outbound {
	if len(candidates) == 0 {
		return nil
	}
	return candidates[rand.Intn(len(candidates))].outbound
}

func (g *LoadBalanceGroup) selectConsistentHash(ctx context.Context, metadata adapter.InboundContext, candidates []*loadBalanceCandidate) (adapter.Outbound, error) {
	// Build hash key from metadata
	key := hash.BuildHashKey(&metadata, g.hashKeyParts, g.salt)

	// Handle empty key
	if key == "" {
		if g.onEmptyKey == "random" {
			return g.selectRandom(candidates), nil
		}
		// hash_empty: use empty string as key
		key = ""
	}

	// Get from hash ring
	tag, ok := g.hashRing.Get(key)
	if !ok {
		// Hash ring empty, fall back to random
		return g.selectRandom(candidates), nil
	}

	// Find outbound by tag
	for _, candidate := range candidates {
		if candidate.tag == tag {
			return candidate.outbound, nil
		}
	}

	// Tag not in current candidates, fall back to random
	return g.selectRandom(candidates), nil
}

func (g *LoadBalanceGroup) selectFromAll(ctx context.Context, metadata adapter.InboundContext, network string) (adapter.Outbound, error) {
	// Fallback to all outbounds when no candidates available
	allCandidates := make([]*loadBalanceCandidate, 0, len(g.allOutbounds))
	for i, outbound := range g.allOutbounds {
		allCandidates = append(allCandidates, &loadBalanceCandidate{
			outbound: outbound,
			tag:      g.allTags[i],
		})
	}

	// Filter by network
	networkCandidates := g.filterByNetwork(allCandidates, network)
	if len(networkCandidates) == 0 {
		return nil, E.New("no outbounds support network ", network)
	}

	// Select randomly from all
	return g.selectRandom(networkCandidates), nil
}

func (g *LoadBalanceGroup) recordFailure() {
	// This could be enhanced to track per-outbound failures
	// For now, just increment the global failure counter
	if !g.usingBackupTier.Load() {
		g.consecutiveFailures.Add(1)
	}
}
