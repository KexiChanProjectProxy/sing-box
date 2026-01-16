package route

import (
	"context"
	"os"
	"runtime"

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
	ctx               context.Context
	logger            log.ContextLogger
	inbound           adapter.InboundManager
	outbound          adapter.OutboundManager
	dns               adapter.DNSRouter
	dnsTransport      adapter.DNSTransportManager
	connection        adapter.ConnectionManager
	network           adapter.NetworkManager
	rules             []adapter.Rule
	needFindProcess   bool
	ruleSets          []adapter.RuleSet
	ruleSetMap        map[string]adapter.RuleSet
	processSearcher   process.Searcher
	pauseManager      pause.Manager
	trackers          []adapter.ConnectionTracker
	platformInterface platform.Interface
	needWIFIState     bool
	started           bool
	asnReader         adapter.ASNReader
	asnPath           string
	geositeReader     adapter.GeositeReader
	geositePath       string
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
