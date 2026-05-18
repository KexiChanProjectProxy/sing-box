package route

import (
	"context"
	"net/netip"
	"slices"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/process"
	"github.com/sagernet/sing-box/log"
)

type processCacheKey struct {
	Network     string
	Source      netip.AddrPort
	Destination netip.AddrPort
}

type processCacheEntry struct {
	result *adapter.ConnectionOwner
	err    error
}

func (r *Router) findProcessInfoCached(ctx context.Context, network string, source netip.AddrPort, destination netip.AddrPort) (*adapter.ConnectionOwner, error) {
	key := processCacheKey{
		Network:     network,
		Source:      source,
		Destination: destination,
	}
	if entry, ok := r.processCache.Get(key); ok {
		return entry.result, entry.err
	}
	result, err := process.FindProcessInfo(r.processSearcher, ctx, network, source, destination)
	r.processCache.Add(key, processCacheEntry{result: result, err: err})
	return result, err
}

func (r *Router) searchProcessInfo(ctx context.Context, metadata *adapter.InboundContext) {
	if r.processSearcher == nil || metadata.ProcessInfo != nil || !r.isLocalSource(metadata.Source.Addr) {
		return
	}
	var originDestination netip.AddrPort
	if metadata.OriginDestination.IsValid() {
		originDestination = metadata.OriginDestination.AddrPort()
	} else if metadata.Destination.IsIP() {
		originDestination = metadata.Destination.AddrPort()
	}
	processInfo, err := r.findProcessInfoCached(ctx, metadata.Network, metadata.Source.AddrPort(), originDestination)
	if err != nil {
		r.logger.InfoEventContext(ctx, "route.process.search_failed", "failed to search process", log.Err(err))
		return
	}
	metadata.ProcessInfo = processInfo
	if processInfo.ProcessPath != "" {
		if processInfo.UserName != "" {
			r.logger.InfoEventContext(ctx, "route.process.found", "found process path", log.String("process_path", processInfo.ProcessPath), log.String("user", processInfo.UserName))
		} else if processInfo.UserId != -1 {
			r.logger.InfoEventContext(ctx, "route.process.found", "found process path", log.String("process_path", processInfo.ProcessPath), log.Int("user_id", int(processInfo.UserId)))
		} else {
			r.logger.InfoEventContext(ctx, "route.process.found", "found process path", log.String("process_path", processInfo.ProcessPath))
		}
		return
	}
	if len(processInfo.AndroidPackageNames) > 0 {
		r.logger.InfoEventContext(ctx, "route.process.found", "found package name", log.String("package_name", strings.Join(processInfo.AndroidPackageNames, ", ")))
		return
	}
	if processInfo.UserId != -1 {
		if processInfo.UserName != "" {
			r.logger.InfoEventContext(ctx, "route.process.found", "found user", log.String("user", processInfo.UserName))
		} else {
			r.logger.InfoEventContext(ctx, "route.process.found", "found user id", log.Int("user_id", int(processInfo.UserId)))
		}
	}
}

func (r *Router) isLocalSource(source netip.Addr) bool {
	if source.IsLoopback() {
		return true
	}
	if r.platformInterface != nil {
		if slices.Contains(r.platformInterface.MyInterfaceAddress(), source) {
			return true
		}
	}
	for _, netInterface := range r.network.InterfaceFinder().Interfaces() {
		for _, prefix := range netInterface.Addresses {
			if prefix.Addr() == source {
				return true
			}
		}
	}
	return false
}
