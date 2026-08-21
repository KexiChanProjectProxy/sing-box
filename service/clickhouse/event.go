package clickhouse

import (
	"net"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	R "github.com/sagernet/sing-box/route/rule"
	tun "github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	M "github.com/sagernet/sing/common/metadata"
)

const (
	actionAllow  = "allow"
	actionReject = "reject"
	closeFin     = "fin"
	closeRst     = "rst"
	closeTimeout = "timeout"
	closeDrop    = "drop"
	closeReject  = "reject"
)

type sessionEvent struct {
	Node         string
	ID           string
	Start        time.Time
	End          time.Time
	DurationMs   int64
	Action       string
	Network      string
	Protocol     string
	User         string
	Source       sessionAddr
	Destination  sessionDest
	Inbound      string
	InboundType  string
	Outbound     string
	OutboundType string
	Chain        []string
	Rule         string
	Upload       int64
	Download     int64
	Close        string
	Process      string
}

type sessionAddr struct {
	IP   string
	Port uint16
	MAC  string
}

type sessionDest struct {
	Domain string
	IP     string
	Port   uint16
}

type sessionSnapshot struct {
	metadata     adapter.InboundContext
	rule         adapter.Rule
	outboundTag  string
	outboundType string
	chain        []string
	start        time.Time
	end          time.Time
	upload       int64
	download     int64
	action       string
	close        string
}

func skipSession(metadata adapter.InboundContext, outbound adapter.Outbound) bool {
	if metadata.Protocol == C.ProtocolDNS {
		return true
	}
	return outbound != nil && outbound.Type() == C.TypeDNS
}

func buildSessionEvent(id string, node string, snapshot sessionSnapshot) sessionEvent {
	start := snapshot.start
	end := snapshot.end
	if start.IsZero() {
		start = end
	}
	if end.IsZero() {
		end = start
	}
	if end.Before(start) {
		end = start
	}
	event := sessionEvent{
		Node:         node,
		ID:           id,
		Start:        start,
		End:          end,
		DurationMs:   end.Sub(start).Milliseconds(),
		Action:       snapshot.action,
		Network:      snapshot.metadata.Network,
		Protocol:     snapshot.metadata.Protocol,
		User:         snapshot.metadata.User,
		Source:       sessionAddrFrom(snapshot.metadata.Source, snapshot.metadata.SourceMACAddress),
		Destination:  sessionDestFrom(snapshot.metadata),
		Inbound:      snapshot.metadata.Inbound,
		InboundType:  snapshot.metadata.InboundType,
		Outbound:     snapshot.outboundTag,
		OutboundType: snapshot.outboundType,
		Rule:         ruleName(snapshot.rule),
		Upload:       snapshot.upload,
		Download:     snapshot.download,
		Close:        snapshot.close,
		Process:      processName(snapshot.metadata.ProcessInfo),
	}
	if len(snapshot.chain) > 1 {
		event.Chain = snapshot.chain
	}
	return event
}

func (e sessionEvent) appendArgs() []any {
	chain := e.Chain
	if chain == nil {
		chain = []string{}
	}
	return []any{
		e.Node,
		e.ID,
		e.Start,
		e.End,
		e.DurationMs,
		e.Action,
		e.Network,
		e.Protocol,
		e.User,
		e.Source.IP,
		e.Source.Port,
		e.Source.MAC,
		e.Destination.Domain,
		e.Destination.IP,
		e.Destination.Port,
		e.Inbound,
		e.InboundType,
		e.Outbound,
		e.OutboundType,
		chain,
		e.Rule,
		e.Upload,
		e.Download,
		e.Close,
		e.Process,
	}
}

func sessionAddrFrom(addr M.Socksaddr, mac net.HardwareAddr) sessionAddr {
	result := sessionAddr{}
	if addr.Addr.IsValid() {
		result.IP = addr.Addr.Unmap().String()
	}
	if addr.Port != 0 {
		result.Port = addr.Port
	}
	if len(mac) > 0 {
		result.MAC = mac.String()
	}
	return result
}

func sessionDestFrom(metadata adapter.InboundContext) sessionDest {
	result := sessionDest{
		Domain: destinationDomain(metadata),
	}
	if metadata.Destination.IsIP() {
		result.IP = metadata.Destination.Addr.Unmap().String()
	} else if len(metadata.DestinationAddresses) > 0 {
		result.IP = metadata.DestinationAddresses[0].Unmap().String()
	}
	if metadata.Destination.Port != 0 {
		result.Port = metadata.Destination.Port
	}
	return result
}

func destinationDomain(metadata adapter.InboundContext) string {
	if metadata.Domain != "" {
		return metadata.Domain
	}
	return metadata.Destination.Fqdn
}

func ruleName(rule adapter.Rule) string {
	if rule == nil {
		return "final"
	}
	description := rule.String()
	if description != "" {
		return description
	}
	return rule.Action().String()
}

func processName(info *adapter.ConnectionOwner) string {
	if info == nil {
		return ""
	}
	if info.ProcessPath != "" {
		return info.ProcessPath
	}
	if len(info.AndroidPackageNames) > 0 {
		return info.AndroidPackageNames[0]
	}
	return ""
}

func flowCloseReason(reason tun.FlowCloseReason) string {
	switch reason {
	case tun.FlowCloseFinished:
		return closeFin
	case tun.FlowCloseTimeout:
		return closeTimeout
	default:
		return closeRst
	}
}

func rejectCloseReason(rule adapter.Rule) string {
	if rule == nil {
		return closeReject
	}
	action, isReject := rule.Action().(*R.RuleActionReject)
	if !isReject {
		return closeReject
	}
	if action.Method == C.RuleActionRejectMethodDrop {
		return closeDrop
	}
	return closeReject
}

func resolveChain(outboundManager adapter.OutboundManager, outbound adapter.Outbound) (tag string, outboundType string, chain []string) {
	if outbound == nil {
		return
	}
	next := outbound.Tag()
	for range 8 {
		detour, loaded := outboundManager.Outbound(next)
		if !loaded {
			break
		}
		chain = append(chain, next)
		tag = detour.Tag()
		outboundType = detour.Type()
		group, isGroup := detour.(adapter.OutboundGroup)
		if !isGroup {
			break
		}
		next = group.Now()
	}
	return tag, outboundType, common.Reverse(chain)
}
