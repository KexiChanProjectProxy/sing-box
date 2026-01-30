package route

import (
	"context"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/asn"
	"github.com/sagernet/sing-box/common/geosite"
	"github.com/sagernet/sing-box/common/process"
	"github.com/sagernet/sing-box/common/taskmonitor"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental/libbox/platform"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	R "github.com/sagernet/sing-box/route/rule"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/task"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/pause"
)

var _ adapter.Router = (*Router)(nil)

type Router struct {
	ctx                context.Context
	logger             log.ContextLogger
	inbound            adapter.InboundManager
	outbound           adapter.OutboundManager
	dns                adapter.DNSRouter
	dnsTransport       adapter.DNSTransportManager
	connection         adapter.ConnectionManager
	network            adapter.NetworkManager
	rules              []adapter.Rule
	needFindProcess    bool
	ruleSets           []adapter.RuleSet
	ruleSetMap         map[string]adapter.RuleSet
	hashRuleSets       []adapter.RuleSet
	hashRuleSetMap     map[string]adapter.RuleSet
	hashDomainMatcher  *hashDomainMatchIndex
	hashIPMatcher      *hashIPMatchIndex
	hashMatchCache     *hashMatchCache
	processSearcher    process.Searcher
	pauseManager       pause.Manager
	trackers           []adapter.ConnectionTracker
	platformInterface  platform.Interface
	needWIFIState      bool
	started            bool
	asnReader          adapter.ASNReader
	asnPath            string
	geositeReader      adapter.GeositeReader
	geositePath        string
}

type hashDomainMatchEntry struct {
	rulesetTag  string
	specificity int
}

type hashDomainMatchIndex struct {
	exactDomains   map[string]hashDomainMatchEntry
	domainSuffixes map[string]hashDomainMatchEntry
	domainKeywords []hashDomainKeywordEntry
	domainRegex    []hashDomainRegexEntry
}

type hashDomainKeywordEntry struct {
	keyword     string
	rulesetTag  string
	specificity int
}

type hashDomainRegexEntry struct {
	pattern     string
	rulesetTag  string
	specificity int
}

type hashIPMatchIndex struct {
	ipv4Tree *ipIntervalTree
	ipv6Tree *ipIntervalTree
}

type ipIntervalTree struct {
	intervals []ipInterval
}

type ipInterval struct {
	start      string
	end        string
	rulesetTag string
	prefixBits int
}

type hashMatchCache struct {
	sync.RWMutex
	domainCache map[string]string
	ipCache     map[string]string
	maxSize     int
}

func newHashMatchCache(maxSize int) *hashMatchCache {
	return &hashMatchCache{
		domainCache: make(map[string]string),
		ipCache:     make(map[string]string),
		maxSize:     maxSize,
	}
}

func NewRouter(ctx context.Context, logFactory log.Factory, options option.RouteOptions, dnsOptions option.DNSOptions) *Router {
	router := &Router{
		ctx:               ctx,
		logger:            logFactory.NewLogger("router"),
		inbound:           service.FromContext[adapter.InboundManager](ctx),
		outbound:          service.FromContext[adapter.OutboundManager](ctx),
		dns:               service.FromContext[adapter.DNSRouter](ctx),
		dnsTransport:      service.FromContext[adapter.DNSTransportManager](ctx),
		connection:        service.FromContext[adapter.ConnectionManager](ctx),
		network:           service.FromContext[adapter.NetworkManager](ctx),
		rules:             make([]adapter.Rule, 0, len(options.Rules)),
		ruleSetMap:        make(map[string]adapter.RuleSet),
		hashRuleSetMap:    make(map[string]adapter.RuleSet),
		hashMatchCache:    newHashMatchCache(10000),
		needFindProcess:   hasRule(options.Rules, isProcessRule) || hasDNSRule(dnsOptions.Rules, isProcessDNSRule) || options.FindProcess,
		pauseManager:      service.FromContext[pause.Manager](ctx),
		platformInterface: service.FromContext[platform.Interface](ctx),
		needWIFIState:     hasRule(options.Rules, isWIFIRule) || hasDNSRule(dnsOptions.Rules, isWIFIDNSRule),
	}

	// Initialize ASN database path if configured
	if options.ASN != nil && options.ASN.Path != "" {
		router.asnPath = options.ASN.Path
	}

	// Initialize Geosite database path if configured
	if options.Geosite != nil && options.Geosite.Path != "" {
		router.geositePath = options.Geosite.Path
	}

	return router
}

func (r *Router) Initialize(rules []option.Rule, ruleSets []option.RuleSet) error {
	for i, options := range rules {
		rule, err := R.NewRule(r.ctx, r.logger, options, false)
		if err != nil {
			return E.Cause(err, "parse rule[", i, "]")
		}
		r.rules = append(r.rules, rule)
	}
	for i, options := range ruleSets {
		if _, exists := r.ruleSetMap[options.Tag]; exists {
			return E.New("duplicate rule-set tag: ", options.Tag)
		}
		ruleSet, err := R.NewRuleSet(r.ctx, r.logger, options)
		if err != nil {
			return E.Cause(err, "parse rule-set[", i, "]")
		}
		r.ruleSets = append(r.ruleSets, ruleSet)
		r.ruleSetMap[options.Tag] = ruleSet
	}
	return nil
}

func (r *Router) LoadHashRuleSetsFromDirectory(ctx context.Context, dirPath string) error {
	if dirPath == "" {
		r.logger.Debug("hash_rule_set_directory not configured, skipping")
		return nil
	}

	r.logger.Info("loading hash-only rulesets from directory: ", dirPath)

	// Make sure the directory exists
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return E.New("hash ruleset directory does not exist: ", dirPath)
	}

	var loadedCount int

	// Recursively walk the directory tree
	err := filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories themselves
		if d.IsDir() {
			return nil
		}

		// Only process .json and .srs files
		name := d.Name()
		if !strings.HasSuffix(name, ".json") && !strings.HasSuffix(name, ".srs") {
			return nil
		}

		// Calculate tag from relative path
		// Example: /etc/sing-box/ruleset/sing-geoip/cn.json -> "sing-geoip-cn"
		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			return E.Cause(err, "calculate relative path for: ", path)
		}

		// Replace path separators with hyphens and remove extension
		// sing-geoip/cn.json -> sing-geoip-cn
		tag := strings.ReplaceAll(relPath, string(filepath.Separator), "-")
		tag = strings.TrimSuffix(tag, filepath.Ext(tag))

		// Check for duplicate tags
		if _, exists := r.ruleSetMap[tag]; exists {
			return E.New("duplicate ruleset tag between routing and hash rulesets: ", tag)
		}
		if _, exists := r.hashRuleSetMap[tag]; exists {
			return E.New("duplicate hash ruleset tag: ", tag)
		}

		// Create ruleset options
		rulesetOptions := option.RuleSet{
			Type: C.RuleSetTypeLocal,
			Tag:  tag,
		}
		rulesetOptions.LocalOptions.Path = path

		// Set format based on file extension
		if strings.HasSuffix(name, ".srs") {
			rulesetOptions.Format = C.RuleSetFormatBinary
		} else {
			rulesetOptions.Format = C.RuleSetFormatSource
		}

		// Disable file watching for hash-only rulesets to prevent "too many open files"
		rulesetOptions.DisableWatcher = true

		// Load the ruleset
		ruleSet, err := R.NewRuleSet(ctx, r.logger, rulesetOptions)
		if err != nil {
			return E.Cause(err, "load hash ruleset: ", relPath)
		}

		r.hashRuleSets = append(r.hashRuleSets, ruleSet)
		r.hashRuleSetMap[tag] = ruleSet
		loadedCount++

		r.logger.Debug("loaded hash ruleset: ", tag, " from ", relPath)
		return nil
	})

	if err != nil {
		return E.Cause(err, "walk hash ruleset directory: ", dirPath)
	}

	if loadedCount == 0 {
		r.logger.Warn("hash_rule_set_directory configured but no .json or .srs files found in: ", dirPath)
	} else {
		r.logger.Info("loaded ", loadedCount, " hash-only rulesets from ", dirPath, " (including subdirectories)")
	}

	return nil
}

func (r *Router) buildHashDomainMatchIndex() error {
	if len(r.hashRuleSets) == 0 {
		return nil
	}

	index := &hashDomainMatchIndex{
		exactDomains:   make(map[string]hashDomainMatchEntry),
		domainSuffixes: make(map[string]hashDomainMatchEntry),
		domainKeywords: make([]hashDomainKeywordEntry, 0),
		domainRegex:    make([]hashDomainRegexEntry, 0),
	}

	for _, ruleSet := range r.hashRuleSets {
		tag := ruleSet.Name()

		domainRules, err := ruleSet.ExtractDomainRules()
		if err != nil {
			return E.Cause(err, "extract domain rules from ", tag)
		}

		for _, domain := range domainRules.ExactDomains {
			domain = strings.ToLower(domain)
			if existing, exists := index.exactDomains[domain]; exists {
				r.logger.Warn("duplicate domain rule: ", domain, " in ", existing.rulesetTag, " and ", tag)
				continue
			}
			index.exactDomains[domain] = hashDomainMatchEntry{
				rulesetTag:  tag,
				specificity: 1000,
			}
		}

		for _, suffix := range domainRules.DomainSuffixes {
			suffix = strings.ToLower(suffix)
			if existing, exists := index.domainSuffixes[suffix]; exists {
				r.logger.Warn("duplicate domain_suffix rule: ", suffix, " in ", existing.rulesetTag, " and ", tag)
				continue
			}
			index.domainSuffixes[suffix] = hashDomainMatchEntry{
				rulesetTag:  tag,
				specificity: 100,
			}
		}

		for _, keyword := range domainRules.DomainKeywords {
			index.domainKeywords = append(index.domainKeywords, hashDomainKeywordEntry{
				keyword:     strings.ToLower(keyword),
				rulesetTag:  tag,
				specificity: 10,
			})
		}

		for _, pattern := range domainRules.DomainRegex {
			index.domainRegex = append(index.domainRegex, hashDomainRegexEntry{
				pattern:     pattern,
				rulesetTag:  tag,
				specificity: 1,
			})
		}
	}

	r.hashDomainMatcher = index
	r.logger.Info("built domain match index: ", len(index.exactDomains), " exact, ",
		len(index.domainSuffixes), " suffixes, ", len(index.domainKeywords), " keywords, ",
		len(index.domainRegex), " regex")

	return nil
}

func (r *Router) buildHashIPMatchIndex() error {
	if len(r.hashRuleSets) == 0 {
		return nil
	}

	var ipv4Intervals []ipInterval
	var ipv6Intervals []ipInterval

	for _, ruleSet := range r.hashRuleSets {
		tag := ruleSet.Name()

		ipRules, err := ruleSet.ExtractIPRules()
		if err != nil {
			return E.Cause(err, "extract IP rules from ", tag)
		}

		for _, cidrStr := range ipRules.IPCIDRs {
			start, end, prefixBits, err := parseCIDRRange(cidrStr)
			if err != nil {
				r.logger.Warn("invalid ip_cidr in ", tag, ": ", err)
				continue
			}

			interval := ipInterval{
				start:      start,
				end:        end,
				rulesetTag: tag,
				prefixBits: prefixBits,
			}

			if strings.Contains(cidrStr, ".") {
				ipv4Intervals = append(ipv4Intervals, interval)
			} else {
				ipv6Intervals = append(ipv6Intervals, interval)
			}
		}
	}

	r.hashIPMatcher = &hashIPMatchIndex{
		ipv4Tree: buildIPIntervalTree(ipv4Intervals),
		ipv6Tree: buildIPIntervalTree(ipv6Intervals),
	}

	r.logger.Info("built IP match index: ", len(ipv4Intervals), " IPv4, ", len(ipv6Intervals), " IPv6")
	return nil
}

func parseCIDRRange(cidrStr string) (start, end string, prefixBits int, err error) {
	prefix, parseErr := netip.ParsePrefix(cidrStr)
	if parseErr != nil {
		err = parseErr
		return
	}

	prefixBits = prefix.Bits()
	startAddr := prefix.Addr()
	start = startAddr.String()

	if prefix.IsSingleIP() {
		end = start
		return
	}

	lastAddr := lastAddrInPrefix(prefix)
	end = lastAddr.String()
	return
}

func lastAddrInPrefix(prefix netip.Prefix) netip.Addr {
	addr := prefix.Addr()
	bits := prefix.Bits()

	if addr.Is4() {
		a := addr.As4()
		val := uint32(a[0])<<24 | uint32(a[1])<<16 | uint32(a[2])<<8 | uint32(a[3])
		mask := ^uint32(0) << (32 - bits)
		val |= ^mask
		return netip.AddrFrom4([4]byte{byte(val >> 24), byte(val >> 16), byte(val >> 8), byte(val)})
	} else {
		a := addr.As16()
		val := make([]byte, 16)
		copy(val, a[:])

		remainingBits := 128 - bits
		byteIndex := 15

		for remainingBits > 0 {
			if remainingBits >= 8 {
				val[byteIndex] = 0xFF
				remainingBits -= 8
			} else {
				val[byteIndex] |= byte(0xFF >> (8 - remainingBits))
				remainingBits = 0
			}
			byteIndex--
		}

		return netip.AddrFrom16([16]byte(val))
	}
}

func buildIPIntervalTree(intervals []ipInterval) *ipIntervalTree {
	sort.Slice(intervals, func(i, j int) bool {
		return intervals[i].prefixBits > intervals[j].prefixBits
	})

	return &ipIntervalTree{intervals: intervals}
}

func (r *Router) Start(stage adapter.StartStage) error {
	monitor := taskmonitor.New(r.logger, C.StartTimeout)
	switch stage {
	case adapter.StartStateStart:
		var cacheContext *adapter.HTTPStartContext
		if len(r.ruleSets) > 0 {
			monitor.Start("initialize rule-set")
			cacheContext = adapter.NewHTTPStartContext(r.ctx)
			var ruleSetStartGroup task.Group
			for i, ruleSet := range r.ruleSets {
				ruleSetInPlace := ruleSet
				ruleSetStartGroup.Append0(func(ctx context.Context) error {
					err := ruleSetInPlace.StartContext(ctx, cacheContext)
					if err != nil {
						return E.Cause(err, "initialize rule-set[", i, "]")
					}
					return nil
				})
			}
			ruleSetStartGroup.Concurrency(5)
			ruleSetStartGroup.FastFail()
			err := ruleSetStartGroup.Run(r.ctx)
			monitor.Finish()
			if err != nil {
				return err
			}
		}
		if cacheContext != nil {
			cacheContext.Close()
		}
		// Start hash-only rulesets
		if len(r.hashRuleSets) > 0 {
			monitor.Start("initialize hash-only rule-sets")
			if cacheContext == nil {
				cacheContext = adapter.NewHTTPStartContext(r.ctx)
			}
			var hashRuleSetStartGroup task.Group
			for i, ruleSet := range r.hashRuleSets {
				ruleSetInPlace := ruleSet
				ruleSetIndex := i
				hashRuleSetStartGroup.Append0(func(ctx context.Context) error {
					err := ruleSetInPlace.StartContext(ctx, cacheContext)
					if err != nil {
						return E.Cause(err, "initialize hash rule-set[", ruleSetIndex, "]")
					}
					return nil
				})
			}
			hashRuleSetStartGroup.Concurrency(5)
			hashRuleSetStartGroup.FastFail()
			err := hashRuleSetStartGroup.Run(r.ctx)
			if err != nil {
				if cacheContext != nil {
					cacheContext.Close()
				}
				return err
			}
			monitor.Finish()
		}
		// Close cache context if it was created for hash rulesets
		if cacheContext != nil && len(r.ruleSets) == 0 {
			cacheContext.Close()
		}
		// Build hash match indices
		if len(r.hashRuleSets) > 0 {
			monitor.Start("build hash match indices")
			err := r.buildHashDomainMatchIndex()
			if err != nil {
				return err
			}
			err = r.buildHashIPMatchIndex()
			if err != nil {
				return err
			}
			monitor.Finish()
		}
		// Initialize ASN reader if path is configured
		if r.asnPath != "" {
			monitor.Start("initialize ASN database")
			asnReader, err := asn.Open(r.asnPath)
			monitor.Finish()
			if err != nil {
				if !os.IsNotExist(err) {
					r.logger.Warn(E.Cause(err, "open ASN database"))
				} else {
					r.logger.Debug("ASN database not found: ", r.asnPath)
				}
			} else {
				r.asnReader = asnReader
				r.logger.Info("ASN database loaded from ", r.asnPath)
			}
		}
		// Initialize Geosite reader if path is configured
		if r.geositePath != "" {
			monitor.Start("initialize geosite database")
			geositeReader, codes, err := geosite.Open(r.geositePath)
			monitor.Finish()
			if err != nil {
				if !os.IsNotExist(err) {
					r.logger.Warn(E.Cause(err, "open geosite database"))
				} else {
					r.logger.Debug("geosite database not found: ", r.geositePath)
				}
			} else {
				// Build matcher for domain lookup
				matcher, err := geosite.NewMatcher(geositeReader, codes)
				if err != nil {
					r.logger.Warn(E.Cause(err, "create geosite matcher"))
				} else {
					r.geositeReader = matcher
					r.logger.Info("geosite database loaded from ", r.geositePath, " with ", len(codes), " codes")
				}
			}
		}
		needFindProcess := r.needFindProcess
		for _, ruleSet := range r.ruleSets {
			metadata := ruleSet.Metadata()
			if metadata.ContainsProcessRule {
				needFindProcess = true
			}
			if metadata.ContainsWIFIRule {
				r.needWIFIState = true
			}
		}
		if needFindProcess {
			if r.platformInterface != nil {
				r.processSearcher = r.platformInterface
			} else {
				monitor.Start("initialize process searcher")
				searcher, err := process.NewSearcher(process.Config{
					Logger:         r.logger,
					PackageManager: r.network.PackageManager(),
				})
				monitor.Finish()
				if err != nil {
					if err != os.ErrInvalid {
						r.logger.Warn(E.Cause(err, "create process searcher"))
					}
				} else {
					r.processSearcher = searcher
				}
			}
		}
	case adapter.StartStatePostStart:
		for i, rule := range r.rules {
			monitor.Start("initialize rule[", i, "]")
			err := rule.Start()
			monitor.Finish()
			if err != nil {
				return E.Cause(err, "initialize rule[", i, "]")
			}
		}
		for _, ruleSet := range r.ruleSets {
			monitor.Start("post start rule_set[", ruleSet.Name(), "]")
			err := ruleSet.PostStart()
			monitor.Finish()
			if err != nil {
				return E.Cause(err, "post start rule_set[", ruleSet.Name(), "]")
			}
		}
		r.started = true
		return nil
	case adapter.StartStateStarted:
		for _, ruleSet := range r.ruleSets {
			ruleSet.Cleanup()
		}
		runtime.GC()
	}
	return nil
}

func (r *Router) Close() error {
	monitor := taskmonitor.New(r.logger, C.StopTimeout)
	var err error
	for i, rule := range r.rules {
		monitor.Start("close rule[", i, "]")
		err = E.Append(err, rule.Close(), func(err error) error {
			return E.Cause(err, "close rule[", i, "]")
		})
		monitor.Finish()
	}
	for i, ruleSet := range r.ruleSets {
		monitor.Start("close rule-set[", i, "]")
		err = E.Append(err, ruleSet.Close(), func(err error) error {
			return E.Cause(err, "close rule-set[", i, "]")
		})
		monitor.Finish()
	}
	for i, ruleSet := range r.hashRuleSets {
		monitor.Start("close hash rule-set[", i, "]")
		err = E.Append(err, ruleSet.Close(), func(err error) error {
			return E.Cause(err, "close hash rule-set[", i, "]")
		})
		monitor.Finish()
	}
	if r.asnReader != nil {
		monitor.Start("close ASN database")
		if asnCloser, ok := r.asnReader.(interface{ Close() error }); ok {
			err = E.Append(err, asnCloser.Close(), func(err error) error {
				return E.Cause(err, "close ASN database")
			})
		}
		monitor.Finish()
	}
	if r.geositeReader != nil {
		monitor.Start("close geosite database")
		if geositeCloser, ok := r.geositeReader.(interface{ Close() error }); ok {
			err = E.Append(err, geositeCloser.Close(), func(err error) error {
				return E.Cause(err, "close geosite database")
			})
		}
		monitor.Finish()
	}
	return err
}

func (r *Router) RuleSet(tag string) (adapter.RuleSet, bool) {
	ruleSet, loaded := r.ruleSetMap[tag]
	return ruleSet, loaded
}

func (r *Router) NeedWIFIState() bool {
	return r.needWIFIState
}

func (r *Router) Rules() []adapter.Rule {
	return r.rules
}

func (r *Router) AppendTracker(tracker adapter.ConnectionTracker) {
	r.trackers = append(r.trackers, tracker)
}

func (r *Router) ResetNetwork() {
	r.network.ResetNetwork()
	r.dns.ResetNetwork()
}

func (r *Router) ASNReader() adapter.ASNReader {
	return r.asnReader
}

func (r *Router) GeositeReader() adapter.GeositeReader {
	return r.geositeReader
}
