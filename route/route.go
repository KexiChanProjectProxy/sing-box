package route

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/conntrack"
	"github.com/sagernet/sing-box/common/process"
	"github.com/sagernet/sing-box/common/sniff"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	R "github.com/sagernet/sing-box/route/rule"
	"github.com/sagernet/sing-mux"
	"github.com/sagernet/sing-vmess"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/bufio/deadline"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/uot"

	"golang.org/x/exp/slices"
)

// Deprecated: use RouteConnectionEx instead.
func (r *Router) RouteConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	done := make(chan interface{})
	err := r.routeConnection(ctx, conn, metadata, N.OnceClose(func(it error) {
		close(done)
	}))
	if err != nil {
		return err
	}
	select {
	case <-done:
	case <-r.ctx.Done():
	}
	return nil
}

func (r *Router) RouteConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	err := r.routeConnection(ctx, conn, metadata, onClose)
	if err != nil {
		N.CloseOnHandshakeFailure(conn, onClose, err)
		if E.IsClosedOrCanceled(err) || R.IsRejected(err) {
			r.logger.DebugContext(ctx, "connection closed: ", err)
		} else {
			r.logger.ErrorContext(ctx, err)
		}
	}
}

func (r *Router) routeConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) error {
	//nolint:staticcheck
	if metadata.InboundDetour != "" {
		if metadata.LastInbound == metadata.InboundDetour {
			return E.New("routing loop on detour: ", metadata.InboundDetour)
		}
		detour, loaded := r.inbound.Get(metadata.InboundDetour)
		if !loaded {
			return E.New("inbound detour not found: ", metadata.InboundDetour)
		}
		injectable, isInjectable := detour.(adapter.TCPInjectableInbound)
		if !isInjectable {
			return E.New("inbound detour is not TCP injectable: ", metadata.InboundDetour)
		}
		metadata.LastInbound = metadata.Inbound
		metadata.Inbound = metadata.InboundDetour
		metadata.InboundDetour = ""
		injectable.NewConnectionEx(ctx, conn, metadata, onClose)
		return nil
	}
	conntrack.KillerCheck()
	metadata.Network = N.NetworkTCP
	switch metadata.Destination.Fqdn {
	case mux.Destination.Fqdn:
		return E.New("global multiplex is deprecated since sing-box v1.7.0, enable multiplex in Inbound fields instead.")
	case vmess.MuxDestination.Fqdn:
		return E.New("global multiplex (v2ray legacy) not supported since sing-box v1.7.0.")
	case uot.MagicAddress:
		return E.New("global UoT not supported since sing-box v1.7.0.")
	case uot.LegacyMagicAddress:
		return E.New("global UoT (legacy) not supported since sing-box v1.7.0.")
	}
	if deadline.NeedAdditionalReadDeadline(conn) {
		conn = deadline.NewConn(conn)
	}
	selectedRule, _, buffers, _, err := r.matchRule(ctx, &metadata, false, conn, nil)
	if err != nil {
		return err
	}
	r.matchHashRuleSets(&metadata)
	var selectedOutbound adapter.Outbound
	if selectedRule != nil {
		switch action := selectedRule.Action().(type) {
		case *R.RuleActionRoute:
			var loaded bool
			selectedOutbound, loaded = r.outbound.Outbound(action.Outbound)
			if !loaded {
				buf.ReleaseMulti(buffers)
				return E.New("outbound not found: ", action.Outbound)
			}
			if !common.Contains(selectedOutbound.Network(), N.NetworkTCP) {
				buf.ReleaseMulti(buffers)
				return E.New("TCP is not supported by outbound: ", selectedOutbound.Tag())
			}
		case *R.RuleActionReject:
			buf.ReleaseMulti(buffers)
			return action.Error(ctx)
		case *R.RuleActionHijackDNS:
			for _, buffer := range buffers {
				conn = bufio.NewCachedConn(conn, buffer)
			}
			N.CloseOnHandshakeFailure(conn, onClose, r.hijackDNSStream(ctx, conn, metadata))
			return nil
		}
	}
	if selectedRule == nil {
		defaultOutbound := r.outbound.Default()
		if !common.Contains(defaultOutbound.Network(), N.NetworkTCP) {
			buf.ReleaseMulti(buffers)
			return E.New("TCP is not supported by default outbound: ", defaultOutbound.Tag())
		}
		selectedOutbound = defaultOutbound
	}

	for _, buffer := range buffers {
		conn = bufio.NewCachedConn(conn, buffer)
	}
	for _, tracker := range r.trackers {
		conn = tracker.RoutedConnection(ctx, conn, metadata, selectedRule, selectedOutbound)
	}
	if outboundHandler, isHandler := selectedOutbound.(adapter.ConnectionHandlerEx); isHandler {
		outboundHandler.NewConnectionEx(ctx, conn, metadata, onClose)
	} else {
		r.connection.NewConnection(ctx, selectedOutbound, conn, metadata, onClose)
	}
	return nil
}

func (r *Router) RoutePacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	done := make(chan interface{})
	err := r.routePacketConnection(ctx, conn, metadata, N.OnceClose(func(it error) {
		close(done)
	}))
	if err != nil {
		conn.Close()
		if E.IsClosedOrCanceled(err) || R.IsRejected(err) {
			r.logger.DebugContext(ctx, "connection closed: ", err)
		} else {
			r.logger.ErrorContext(ctx, err)
		}
	}
	select {
	case <-done:
	case <-r.ctx.Done():
	}
	return nil
}

func (r *Router) RoutePacketConnectionEx(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	err := r.routePacketConnection(ctx, conn, metadata, onClose)
	if err != nil {
		N.CloseOnHandshakeFailure(conn, onClose, err)
		if E.IsClosedOrCanceled(err) || R.IsRejected(err) {
			r.logger.DebugContext(ctx, "connection closed: ", err)
		} else {
			r.logger.ErrorContext(ctx, err)
		}
	}
}

func (r *Router) routePacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) error {
	//nolint:staticcheck
	if metadata.InboundDetour != "" {
		if metadata.LastInbound == metadata.InboundDetour {
			return E.New("routing loop on detour: ", metadata.InboundDetour)
		}
		detour, loaded := r.inbound.Get(metadata.InboundDetour)
		if !loaded {
			return E.New("inbound detour not found: ", metadata.InboundDetour)
		}
		injectable, isInjectable := detour.(adapter.UDPInjectableInbound)
		if !isInjectable {
			return E.New("inbound detour is not UDP injectable: ", metadata.InboundDetour)
		}
		metadata.LastInbound = metadata.Inbound
		metadata.Inbound = metadata.InboundDetour
		metadata.InboundDetour = ""
		injectable.NewPacketConnectionEx(ctx, conn, metadata, onClose)
		return nil
	}
	conntrack.KillerCheck()

	// TODO: move to UoT
	metadata.Network = N.NetworkUDP

	// Currently we don't have deadline usages for UDP connections
	/*if deadline.NeedAdditionalReadDeadline(conn) {
		conn = deadline.NewPacketConn(bufio.NewNetPacketConn(conn))
	}*/

	selectedRule, _, _, packetBuffers, err := r.matchRule(ctx, &metadata, false, nil, conn)
	if err != nil {
		return err
	}
	r.matchHashRuleSets(&metadata)
	var selectedOutbound adapter.Outbound
	var selectReturn bool
	if selectedRule != nil {
		switch action := selectedRule.Action().(type) {
		case *R.RuleActionRoute:
			var loaded bool
			selectedOutbound, loaded = r.outbound.Outbound(action.Outbound)
			if !loaded {
				N.ReleaseMultiPacketBuffer(packetBuffers)
				return E.New("outbound not found: ", action.Outbound)
			}
			if !common.Contains(selectedOutbound.Network(), N.NetworkUDP) {
				N.ReleaseMultiPacketBuffer(packetBuffers)
				return E.New("UDP is not supported by outbound: ", selectedOutbound.Tag())
			}
		case *R.RuleActionReject:
			N.ReleaseMultiPacketBuffer(packetBuffers)
			return action.Error(ctx)
		case *R.RuleActionHijackDNS:
			return r.hijackDNSPacket(ctx, conn, packetBuffers, metadata, onClose)
		}
	}
	if selectedRule == nil || selectReturn {
		defaultOutbound := r.outbound.Default()
		if !common.Contains(defaultOutbound.Network(), N.NetworkUDP) {
			N.ReleaseMultiPacketBuffer(packetBuffers)
			return E.New("UDP is not supported by outbound: ", defaultOutbound.Tag())
		}
		selectedOutbound = defaultOutbound
	}
	for _, buffer := range packetBuffers {
		conn = bufio.NewCachedPacketConn(conn, buffer.Buffer, buffer.Destination)
		N.PutPacketBuffer(buffer)
	}
	for _, tracker := range r.trackers {
		conn = tracker.RoutedPacketConnection(ctx, conn, metadata, selectedRule, selectedOutbound)
	}
	if metadata.FakeIP {
		conn = bufio.NewNATPacketConn(bufio.NewNetPacketConn(conn), metadata.OriginDestination, metadata.Destination)
	}
	if outboundHandler, isHandler := selectedOutbound.(adapter.PacketConnectionHandlerEx); isHandler {
		outboundHandler.NewPacketConnectionEx(ctx, conn, metadata, onClose)
	} else {
		r.connection.NewPacketConnection(ctx, selectedOutbound, conn, metadata, onClose)
	}
	return nil
}

func (r *Router) PreMatch(metadata adapter.InboundContext) error {
	selectedRule, _, _, _, err := r.matchRule(r.ctx, &metadata, true, nil, nil)
	if err != nil {
		return err
	}
	if selectedRule == nil {
		return nil
	}
	rejectAction, isReject := selectedRule.Action().(*R.RuleActionReject)
	if !isReject {
		return nil
	}
	return rejectAction.Error(context.Background())
}

func (r *Router) matchRule(
	ctx context.Context, metadata *adapter.InboundContext, preMatch bool,
	inputConn net.Conn, inputPacketConn N.PacketConn,
) (
	selectedRule adapter.Rule, selectedRuleIndex int,
	buffers []*buf.Buffer, packetBuffers []*N.PacketBuffer, fatalErr error,
) {
	if r.processSearcher != nil && metadata.ProcessInfo == nil {
		var originDestination netip.AddrPort
		if metadata.OriginDestination.IsValid() {
			originDestination = metadata.OriginDestination.AddrPort()
		} else if metadata.Destination.IsIP() {
			originDestination = metadata.Destination.AddrPort()
		}
		processInfo, fErr := process.FindProcessInfo(r.processSearcher, ctx, metadata.Network, metadata.Source.AddrPort(), originDestination)
		if fErr != nil {
			r.logger.InfoContext(ctx, "failed to search process: ", fErr)
		} else {
			if processInfo.ProcessPath != "" {
				if processInfo.User != "" {
					r.logger.InfoContext(ctx, "found process path: ", processInfo.ProcessPath, ", user: ", processInfo.User)
				} else if processInfo.UserId != -1 {
					r.logger.InfoContext(ctx, "found process path: ", processInfo.ProcessPath, ", user id: ", processInfo.UserId)
				} else {
					r.logger.InfoContext(ctx, "found process path: ", processInfo.ProcessPath)
				}
			} else if processInfo.PackageName != "" {
				r.logger.InfoContext(ctx, "found package name: ", processInfo.PackageName)
			} else if processInfo.UserId != -1 {
				if processInfo.User != "" {
					r.logger.InfoContext(ctx, "found user: ", processInfo.User)
				} else {
					r.logger.InfoContext(ctx, "found user id: ", processInfo.UserId)
				}
			}
			metadata.ProcessInfo = processInfo
		}
	}
	if metadata.Destination.Addr.IsValid() && r.dnsTransport.FakeIP() != nil && r.dnsTransport.FakeIP().Store().Contains(metadata.Destination.Addr) {
		domain, loaded := r.dnsTransport.FakeIP().Store().Lookup(metadata.Destination.Addr)
		if !loaded {
			fatalErr = E.New("missing fakeip record, try enable `experimental.cache_file`")
			return
		}
		if domain != "" {
			metadata.OriginDestination = metadata.Destination
			metadata.Destination = M.Socksaddr{
				Fqdn: domain,
				Port: metadata.Destination.Port,
			}
			metadata.FakeIP = true
			r.logger.DebugContext(ctx, "found fakeip domain: ", domain)
		}
	} else if metadata.Domain == "" {
		domain, loaded := r.dns.LookupReverseMapping(metadata.Destination.Addr)
		if loaded {
			metadata.Domain = domain
			r.logger.DebugContext(ctx, "found reserve mapped domain: ", metadata.Domain)
		}
	}
	if metadata.Destination.IsIPv4() {
		metadata.IPVersion = 4
	} else if metadata.Destination.IsIPv6() {
		metadata.IPVersion = 6
	}

	//nolint:staticcheck
	if metadata.InboundOptions != common.DefaultValue[option.InboundOptions]() {
		if !preMatch && metadata.InboundOptions.SniffEnabled {
			r.logger.WarnContext(ctx, "inbound-level sniff options are deprecated; please migrate to route rule actions")
			r.logger.DebugContext(ctx, "legacy sniff action triggered for inbound ", metadata.Inbound)
			newBuffer, newPackerBuffers, newErr := r.actionSniff(ctx, metadata, &R.RuleActionSniff{
				OverrideDestination: metadata.InboundOptions.SniffOverrideDestination,
				Timeout:             time.Duration(metadata.InboundOptions.SniffTimeout),
			}, inputConn, inputPacketConn, nil, nil)
			if newBuffer != nil {
				buffers = []*buf.Buffer{newBuffer}
			} else if len(newPackerBuffers) > 0 {
				packetBuffers = newPackerBuffers
			}
			if newErr != nil {
				fatalErr = newErr
				return
			}
		}
		if C.DomainStrategy(metadata.InboundOptions.DomainStrategy) != C.DomainStrategyAsIS {
			fatalErr = r.actionResolve(ctx, metadata, &R.RuleActionResolve{
				Strategy: C.DomainStrategy(metadata.InboundOptions.DomainStrategy),
			})
			if fatalErr != nil {
				return
			}
		}
		if metadata.InboundOptions.UDPDisableDomainUnmapping {
			metadata.UDPDisableDomainUnmapping = true
		}
		metadata.InboundOptions = option.InboundOptions{}
	}

match:
	for currentRuleIndex, currentRule := range r.rules {
		metadata.ResetRuleCache()
		if !currentRule.Match(metadata) {
			continue
		}
		if !preMatch {
			ruleDescription := currentRule.String()
			if ruleDescription != "" {
				r.logger.DebugContext(ctx, "match[", currentRuleIndex, "] ", currentRule, " => ", currentRule.Action())
			} else {
				r.logger.DebugContext(ctx, "match[", currentRuleIndex, "] => ", currentRule.Action())
			}
		} else {
			switch currentRule.Action().Type() {
			case C.RuleActionTypeReject:
				ruleDescription := currentRule.String()
				if ruleDescription != "" {
					r.logger.DebugContext(ctx, "pre-match[", currentRuleIndex, "] ", currentRule, " => ", currentRule.Action())
				} else {
					r.logger.DebugContext(ctx, "pre-match[", currentRuleIndex, "] => ", currentRule.Action())
				}
			}
		}
		var routeOptions *R.RuleActionRouteOptions
		switch action := currentRule.Action().(type) {
		case *R.RuleActionRoute:
			routeOptions = &action.RuleActionRouteOptions
		case *R.RuleActionRouteOptions:
			routeOptions = action
		}
		if routeOptions != nil {
			// TODO: add nat
			if (routeOptions.OverrideAddress.IsValid() || routeOptions.OverridePort > 0) && !metadata.RouteOriginalDestination.IsValid() {
				metadata.RouteOriginalDestination = metadata.Destination
			}
			if routeOptions.OverrideAddress.IsValid() {
				metadata.Destination = M.Socksaddr{
					Addr: routeOptions.OverrideAddress.Addr,
					Port: metadata.Destination.Port,
					Fqdn: routeOptions.OverrideAddress.Fqdn,
				}
				metadata.DestinationAddresses = nil
			}
			if routeOptions.OverridePort > 0 {
				metadata.Destination = M.Socksaddr{
					Addr: metadata.Destination.Addr,
					Port: routeOptions.OverridePort,
					Fqdn: metadata.Destination.Fqdn,
				}
			}
			if routeOptions.NetworkStrategy != nil {
				metadata.NetworkStrategy = routeOptions.NetworkStrategy
			}
			if len(routeOptions.NetworkType) > 0 {
				metadata.NetworkType = routeOptions.NetworkType
			}
			if len(routeOptions.FallbackNetworkType) > 0 {
				metadata.FallbackNetworkType = routeOptions.FallbackNetworkType
			}
			if routeOptions.FallbackDelay != 0 {
				metadata.FallbackDelay = routeOptions.FallbackDelay
			}
			if routeOptions.UDPDisableDomainUnmapping {
				metadata.UDPDisableDomainUnmapping = true
			}
			if routeOptions.UDPConnect {
				metadata.UDPConnect = true
			}
			if routeOptions.UDPTimeout > 0 {
				metadata.UDPTimeout = routeOptions.UDPTimeout
			}
			if routeOptions.TLSFragment {
				metadata.TLSFragment = true
				metadata.TLSFragmentFallbackDelay = routeOptions.TLSFragmentFallbackDelay
			}
			if routeOptions.TLSRecordFragment {
				metadata.TLSRecordFragment = true
			}
		}
		switch action := currentRule.Action().(type) {
		case *R.RuleActionSniff:
			if !preMatch {
				newBuffer, newPacketBuffers, newErr := r.actionSniff(ctx, metadata, action, inputConn, inputPacketConn, buffers, packetBuffers)
				if newBuffer != nil {
					buffers = append(buffers, newBuffer)
				} else if len(newPacketBuffers) > 0 {
					packetBuffers = append(packetBuffers, newPacketBuffers...)
				}
				if newErr != nil {
					fatalErr = newErr
					return
				}
			} else {
				selectedRule = currentRule
				selectedRuleIndex = currentRuleIndex
				break match
			}
		case *R.RuleActionResolve:
			fatalErr = r.actionResolve(ctx, metadata, action)
			if fatalErr != nil {
				return
			}
		}
		actionType := currentRule.Action().Type()
		if actionType == C.RuleActionTypeRoute ||
			actionType == C.RuleActionTypeReject ||
			actionType == C.RuleActionTypeHijackDNS ||
			(actionType == C.RuleActionTypeSniff && preMatch) {
			selectedRule = currentRule
			selectedRuleIndex = currentRuleIndex
			break match
		}
	}
	return
}

func (r *Router) matchHashRuleSets(metadata *adapter.InboundContext) bool {
	if r.hashDomainMatcher == nil && r.hashIPMatcher == nil {
		return false
	}

	var bestMatch string
	var bestSpecificity int
	var domainHost string
	var matchType string // For logging: "exact", "suffix", "keyword", "regex", "ip"

	if r.hashDomainMatcher != nil {
		domainHost = metadata.Domain
		if domainHost == "" {
			domainHost = metadata.Destination.Fqdn
		}

		if domainHost != "" {
			domainHost = strings.ToLower(domainHost)

			if cachedTag, found := r.getCachedDomainMatch(domainHost); found {
				if cachedTag != "" {
					metadata.MatchedRuleSet = cachedTag
					r.logger.Debug("hash ruleset matched (cached): domain=", domainHost, " → ruleset=", cachedTag)
					return true
				}
				goto ipMatching
			}

			if entry, exists := r.hashDomainMatcher.exactDomains[domainHost]; exists {
				if entry.specificity > bestSpecificity {
					bestSpecificity = entry.specificity
					bestMatch = entry.rulesetTag
					matchType = "exact"
				}
			}

			if bestSpecificity < 100 {
				parts := strings.Split(domainHost, ".")
				for i := 0; i < len(parts); i++ {
					suffix := strings.Join(parts[i:], ".")
					if entry, exists := r.hashDomainMatcher.domainSuffixes[suffix]; exists {
						if entry.specificity > bestSpecificity {
							bestSpecificity = entry.specificity
							bestMatch = entry.rulesetTag
							matchType = "suffix"
						}
					}
				}
			}

			if bestSpecificity < 10 {
				for _, keywordEntry := range r.hashDomainMatcher.domainKeywords {
					if strings.Contains(domainHost, keywordEntry.keyword) {
						if keywordEntry.specificity > bestSpecificity {
							bestSpecificity = keywordEntry.specificity
							bestMatch = keywordEntry.rulesetTag
							matchType = "keyword"
						}
					}
				}
			}

			if bestSpecificity < 1 {
				for _, regexEntry := range r.hashDomainMatcher.domainRegex {
					if strings.Contains(domainHost, regexEntry.pattern) {
						if regexEntry.specificity > bestSpecificity {
							bestSpecificity = regexEntry.specificity
							bestMatch = regexEntry.rulesetTag
							matchType = "regex"
						}
					}
				}
			}
		}
	}

	if bestMatch != "" {
		r.setCachedDomainMatch(domainHost, bestMatch)
		metadata.MatchedRuleSet = bestMatch
		r.logger.Info("hash ruleset matched (", matchType, "): domain=", domainHost, " → ruleset=", bestMatch)
		return true
	}

	if domainHost != "" {
		r.setCachedDomainMatch(domainHost, "")
	}

ipMatching:
	if r.hashIPMatcher != nil {
		destIP := metadata.Destination.Addr
		if !destIP.IsValid() && len(metadata.DestinationAddresses) > 0 {
			destIP = metadata.DestinationAddresses[0]
		}

		if destIP.IsValid() {
			ipStr := destIP.String()

			if cachedTag, found := r.getCachedIPMatch(ipStr); found {
				if cachedTag != "" {
					metadata.MatchedRuleSet = cachedTag
					r.logger.Debug("hash ruleset matched (cached): ip=", ipStr, " → ruleset=", cachedTag)
					return true
				}
				return false
			}

			var tree *ipIntervalTree
			if destIP.Is4() {
				tree = r.hashIPMatcher.ipv4Tree
			} else {
				tree = r.hashIPMatcher.ipv6Tree
			}

			if tree != nil {
				for _, interval := range tree.intervals {
					if ipInRange(ipStr, interval.start, interval.end) {
						r.setCachedIPMatch(ipStr, interval.rulesetTag)
						metadata.MatchedRuleSet = interval.rulesetTag
						r.logger.Info("hash ruleset matched (ip): ip=", ipStr, " → ruleset=", interval.rulesetTag)
						return true
					}
				}
			}

			r.setCachedIPMatch(ipStr, "")
		}
	}

	r.logger.Debug("hash ruleset no match: domain=", domainHost, ", ip=", metadata.Destination.Addr)
	return false
}

func ipInRange(ip, start, end string) bool {
	ipAddr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	startAddr, err := netip.ParseAddr(start)
	if err != nil {
		return false
	}
	endAddr, err := netip.ParseAddr(end)
	if err != nil {
		return false
	}

	return !ipAddr.Less(startAddr) && !endAddr.Less(ipAddr)
}

func (r *Router) getCachedDomainMatch(domain string) (tag string, found bool) {
	if r.hashMatchCache == nil {
		return "", false
	}
	r.hashMatchCache.RLock()
	tag, found = r.hashMatchCache.domainCache[domain]
	r.hashMatchCache.RUnlock()
	return
}

func (r *Router) setCachedDomainMatch(domain string, tag string) {
	if r.hashMatchCache == nil {
		return
	}
	r.hashMatchCache.Lock()
	if len(r.hashMatchCache.domainCache) >= r.hashMatchCache.maxSize {
		r.hashMatchCache.domainCache = make(map[string]string)
	}
	r.hashMatchCache.domainCache[domain] = tag
	r.hashMatchCache.Unlock()
}

func (r *Router) getCachedIPMatch(ip string) (tag string, found bool) {
	if r.hashMatchCache == nil {
		return "", false
	}
	r.hashMatchCache.RLock()
	tag, found = r.hashMatchCache.ipCache[ip]
	r.hashMatchCache.RUnlock()
	return
}

func (r *Router) setCachedIPMatch(ip string, tag string) {
	if r.hashMatchCache == nil {
		return
	}
	r.hashMatchCache.Lock()
	if len(r.hashMatchCache.ipCache) >= r.hashMatchCache.maxSize {
		r.hashMatchCache.ipCache = make(map[string]string)
	}
	r.hashMatchCache.ipCache[ip] = tag
	r.hashMatchCache.Unlock()
}

// shouldSkipForDomain checks if a domain matches skip filters (static or ruleset)
func (r *Router) shouldSkipForDomain(action *R.RuleActionSniff, metadata *adapter.InboundContext, domain string) bool {
	if domain == "" {
		return false
	}

	domainLower := strings.ToLower(domain)

	// Check static domain matcher
	if action.SkipDomainMatcher != nil {
		if (*action.SkipDomainMatcher).Match(domainLower) {
			return true
		}
	}

	// Check ruleset matcher
	if action.SkipDomainRuleSetItem != nil {
		tempMetadata := *metadata
		tempMetadata.Domain = domain
		if action.SkipDomainRuleSetItem.Match(&tempMetadata) {
			return true
		}
	}

	return false
}

func (r *Router) actionSniff(
	ctx context.Context, metadata *adapter.InboundContext, action *R.RuleActionSniff,
	inputConn net.Conn, inputPacketConn N.PacketConn, inputBuffers []*buf.Buffer, inputPacketBuffers []*N.PacketBuffer,
) (buffer *buf.Buffer, packetBuffers []*N.PacketBuffer, fatalErr error) {
	r.logger.DebugContext(ctx, "actionSniff called: inputConn=", inputConn != nil, ", inputPacketConn=", inputPacketConn != nil,
		", destination=", metadata.Destination, ", action.StreamSniffers=", len(action.StreamSniffers),
		", action.PacketSniffers=", len(action.PacketSniffers))

	// IP-based skip filtering (before any sniffing attempt)
	if action.SkipSrcIPSet != nil && metadata.Source.Addr.IsValid() {
		if action.SkipSrcIPSet.Contains(metadata.Source.Addr) {
			r.logger.DebugContext(ctx, "sniff skipped: source IP ", metadata.Source.Addr, " in skip list")
			return
		}
	}

	if action.SkipDstIPSet != nil && metadata.Destination.Addr.IsValid() {
		if action.SkipDstIPSet.Contains(metadata.Destination.Addr) {
			r.logger.DebugContext(ctx, "sniff skipped: destination IP ", metadata.Destination.Addr, " in skip list")
			return
		}
	}

	// Domain-based skip filtering BEFORE sniffing (if skip_sniffing enabled)
	if action.SkipSniffing && metadata.Domain != "" {
		if r.shouldSkipForDomain(action, metadata, metadata.Domain) {
			r.logger.DebugContext(ctx, "sniff skipped: pre-sniff domain ", metadata.Domain, " in skip list (skip_sniffing=true)")
			return
		}
	}

	if sniff.Skip(metadata) {
		r.logger.DebugContext(ctx, "sniff skipped due to port considered as server-first")
		return
	} else if metadata.Protocol != "" {
		r.logger.DebugContext(ctx, "duplicate sniff skipped")
		return
	}
	if inputConn != nil {
		if len(action.StreamSniffers) == 0 && len(action.PacketSniffers) > 0 {
			return
		} else if slices.Equal(metadata.SnifferNames, action.SnifferNames) && metadata.SniffError != nil && !errors.Is(metadata.SniffError, sniff.ErrNeedMoreData) {
			r.logger.DebugContext(ctx, "packet sniff skipped due to previous error: ", metadata.SniffError)
			return
		}
		var streamSniffers []sniff.StreamSniffer

		if len(action.ProtocolConfigs) > 0 {
			// Advanced mode: filter sniffers by port
			for _, config := range action.ProtocolConfigs {
				// Skip if port doesn't match
				if config.PortMatcher != nil && !config.PortMatcher.Match(metadata) {
					continue
				}
				// Add this protocol's sniffers
				streamSniffers = append(streamSniffers, config.StreamSniffers...)
			}

			if len(streamSniffers) == 0 {
				r.logger.DebugContext(ctx, "no stream sniffers match port filter for port ", metadata.Destination.Port)
				return
			}
		} else if len(action.StreamSniffers) > 0 {
			// Legacy mode: use pre-configured sniffers
			streamSniffers = action.StreamSniffers
		} else {
			// Default: all stream sniffers
			streamSniffers = []sniff.StreamSniffer{
				sniff.TLSClientHello,
				sniff.HTTPHost,
				sniff.StreamDomainNameQuery,
				sniff.BitTorrent,
				sniff.SSH,
				sniff.RDP,
			}
		}
		sniffBuffer := buf.NewPacket()
		r.logger.DebugContext(ctx, "attempting stream sniff with ", len(streamSniffers), " sniffers")
		err := sniff.PeekStream(
			ctx,
			metadata,
			inputConn,
			inputBuffers,
			sniffBuffer,
			action.Timeout,
			streamSniffers...,
		)
		metadata.SnifferNames = action.SnifferNames
		metadata.SniffError = err
		if err == nil {
			// Determine if we should override destination
			shouldOverride := action.OverrideDestination // Start with global default

			// Check per-protocol override setting (advanced mode)
			if len(action.ProtocolConfigs) > 0 && metadata.Protocol != "" {
				if config, ok := action.ProtocolConfigs[metadata.Protocol]; ok {
					if config.OverrideDestination != nil {
						shouldOverride = *config.OverrideDestination
					}
				}
			}

			// Apply skip_domain filter (prevents override even if enabled)
			if shouldOverride && metadata.Domain != "" {
				if r.shouldSkipForDomain(action, metadata, metadata.Domain) {
					shouldOverride = false
					r.logger.DebugContext(ctx, "skip override for domain: ", metadata.Domain)
				}
			}

			// Perform override if allowed
			//goland:noinspection GoDeprecation
			if shouldOverride && M.IsDomainName(metadata.Domain) {
				metadata.Destination = M.Socksaddr{
					Fqdn: metadata.Domain,
					Port: metadata.Destination.Port,
				}
			}
			if metadata.Domain != "" && metadata.Client != "" {
				r.logger.DebugContext(ctx, "sniffed protocol: ", metadata.Protocol, ", domain: ", metadata.Domain, ", client: ", metadata.Client)
			} else if metadata.Domain != "" {
				r.logger.DebugContext(ctx, "sniffed protocol: ", metadata.Protocol, ", domain: ", metadata.Domain)
			} else {
				r.logger.DebugContext(ctx, "sniffed protocol: ", metadata.Protocol)
			}
		} else if err != nil {
			r.logger.DebugContext(ctx, "stream sniff failed: ", err)
		}
		if !sniffBuffer.IsEmpty() {
			buffer = sniffBuffer
		} else {
			sniffBuffer.Release()
		}
	} else if inputPacketConn != nil {
		if len(action.PacketSniffers) == 0 && len(action.StreamSniffers) > 0 {
			return
		} else if slices.Equal(metadata.SnifferNames, action.SnifferNames) && metadata.SniffError != nil && !errors.Is(metadata.SniffError, sniff.ErrNeedMoreData) {
			r.logger.DebugContext(ctx, "packet sniff skipped due to previous error: ", metadata.SniffError)
			return
		}
		quicMoreData := func() bool {
			return slices.Equal(metadata.SnifferNames, action.SnifferNames) && errors.Is(metadata.SniffError, sniff.ErrNeedMoreData)
		}
		var packetSniffers []sniff.PacketSniffer

		if len(action.ProtocolConfigs) > 0 {
			// Advanced mode: filter sniffers by port
			for _, config := range action.ProtocolConfigs {
				// Skip if port doesn't match
				if config.PortMatcher != nil && !config.PortMatcher.Match(metadata) {
					continue
				}
				// Add this protocol's sniffers
				packetSniffers = append(packetSniffers, config.PacketSniffers...)
			}

			if len(packetSniffers) == 0 {
				r.logger.DebugContext(ctx, "no packet sniffers match port filter for port ", metadata.Destination.Port)
				return
			}
		} else if len(action.PacketSniffers) > 0 {
			// Legacy mode: use pre-configured sniffers
			packetSniffers = action.PacketSniffers
		} else {
			// Default: all packet sniffers
			packetSniffers = []sniff.PacketSniffer{
				sniff.DomainNameQuery,
				sniff.QUICClientHello,
				sniff.STUNMessage,
				sniff.UTP,
				sniff.UDPTracker,
				sniff.DTLSRecord,
				sniff.NTP,
			}
		}
		var err error
		for _, packetBuffer := range inputPacketBuffers {
			if quicMoreData() {
				err = sniff.PeekPacket(
					ctx,
					metadata,
					packetBuffer.Buffer.Bytes(),
					sniff.QUICClientHello,
				)
			} else {
				err = sniff.PeekPacket(
					ctx, metadata,
					packetBuffer.Buffer.Bytes(),
					packetSniffers...,
				)
			}
			metadata.SnifferNames = action.SnifferNames
			metadata.SniffError = err
			if errors.Is(err, sniff.ErrNeedMoreData) {
				// TODO: replace with generic message when there are more multi-packet protocols
				r.logger.DebugContext(ctx, "attempt to sniff fragmented QUIC client hello")
				continue
			}
			goto finally
		}
		packetBuffers = inputPacketBuffers
		for {
			var (
				sniffBuffer = buf.NewPacket()
				destination M.Socksaddr
				done        = make(chan struct{})
			)
			go func() {
				sniffTimeout := C.ReadPayloadTimeout
				if action.Timeout > 0 {
					sniffTimeout = action.Timeout
				}
				inputPacketConn.SetReadDeadline(time.Now().Add(sniffTimeout))
				destination, err = inputPacketConn.ReadPacket(sniffBuffer)
				inputPacketConn.SetReadDeadline(time.Time{})
				close(done)
			}()
			select {
			case <-done:
			case <-ctx.Done():
				inputPacketConn.Close()
				fatalErr = ctx.Err()
				return
			}
			if err != nil {
				sniffBuffer.Release()
				if !errors.Is(err, context.DeadlineExceeded) {
					fatalErr = err
					return
				}
			} else {
				if quicMoreData() {
					err = sniff.PeekPacket(
						ctx,
						metadata,
						sniffBuffer.Bytes(),
						sniff.QUICClientHello,
					)
				} else {
					err = sniff.PeekPacket(
						ctx, metadata,
						sniffBuffer.Bytes(),
						packetSniffers...,
					)
				}
				packetBuffer := N.NewPacketBuffer()
				*packetBuffer = N.PacketBuffer{
					Buffer:      sniffBuffer,
					Destination: destination,
				}
				packetBuffers = append(packetBuffers, packetBuffer)
				metadata.SnifferNames = action.SnifferNames
				metadata.SniffError = err
				if errors.Is(err, sniff.ErrNeedMoreData) {
					// TODO: replace with generic message when there are more multi-packet protocols
					r.logger.DebugContext(ctx, "attempt to sniff fragmented QUIC client hello")
					continue
				}
			}
			goto finally
		}
	finally:
		if err == nil {
			// Determine if we should override destination
			shouldOverride := action.OverrideDestination // Start with global default

			// Check per-protocol override setting (advanced mode)
			if len(action.ProtocolConfigs) > 0 && metadata.Protocol != "" {
				if config, ok := action.ProtocolConfigs[metadata.Protocol]; ok {
					if config.OverrideDestination != nil {
						shouldOverride = *config.OverrideDestination
					}
				}
			}

			// Apply skip_domain filter (prevents override even if enabled)
			if shouldOverride && metadata.Domain != "" {
				if r.shouldSkipForDomain(action, metadata, metadata.Domain) {
					shouldOverride = false
					r.logger.DebugContext(ctx, "skip override for domain: ", metadata.Domain)
				}
			}

			// Perform override if allowed
			//goland:noinspection GoDeprecation
			if shouldOverride && M.IsDomainName(metadata.Domain) {
				metadata.Destination = M.Socksaddr{
					Fqdn: metadata.Domain,
					Port: metadata.Destination.Port,
				}
			}
			if metadata.Domain != "" && metadata.Client != "" {
				r.logger.DebugContext(ctx, "sniffed packet protocol: ", metadata.Protocol, ", domain: ", metadata.Domain, ", client: ", metadata.Client)
			} else if metadata.Domain != "" {
				r.logger.DebugContext(ctx, "sniffed packet protocol: ", metadata.Protocol, ", domain: ", metadata.Domain)
			} else if metadata.Client != "" {
				r.logger.DebugContext(ctx, "sniffed packet protocol: ", metadata.Protocol, ", client: ", metadata.Client)
			} else {
				r.logger.DebugContext(ctx, "sniffed packet protocol: ", metadata.Protocol)
			}
		}
	}
	return
}

func (r *Router) actionResolve(ctx context.Context, metadata *adapter.InboundContext, action *R.RuleActionResolve) error {
	if metadata.Destination.IsFqdn() {
		var transport adapter.DNSTransport
		if action.Server != "" {
			var loaded bool
			transport, loaded = r.dnsTransport.Transport(action.Server)
			if !loaded {
				return E.New("DNS server not found: ", action.Server)
			}
		}
		addresses, err := r.dns.Lookup(adapter.WithContext(ctx, metadata), metadata.Destination.Fqdn, adapter.DNSQueryOptions{
			Transport:    transport,
			Strategy:     action.Strategy,
			DisableCache: action.DisableCache,
			RewriteTTL:   action.RewriteTTL,
			ClientSubnet: action.ClientSubnet,
		})
		if err != nil {
			return err
		}
		metadata.DestinationAddresses = addresses
		r.logger.DebugContext(ctx, "resolved [", strings.Join(F.MapToString(metadata.DestinationAddresses), " "), "]")
	}
	return nil
}
