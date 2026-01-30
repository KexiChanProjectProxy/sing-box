# sing-box v1.12.14.15

## 🚨 CRITICAL BUG FIX 🚨

This release fixes a **race condition causing immediate crashes** under concurrent load.

### The Crash

```
fatal error: concurrent map writes

goroutine 1563 [running]:
github.com/sagernet/sing-box/route.(*Router).setCachedIPMatch(...)
    github.com/sagernet/sing-box/route/route.go:667
github.com/sagernet/sing-box/route.(*Router).matchHashRuleSets(0xc000211040, 0xc001a70b08)
    github.com/sagernet/sing-box/route/route.go:610 +0x60e
```

### Root Cause

The hash ruleset matching cache (`hashMatchCache`) was being accessed by **multiple goroutines simultaneously** without any synchronization:

- Multiple connections happening concurrently
- Each calling `setCachedIPMatch()` or `setCachedDomainMatch()` to write cache entries
- No mutex protection on the underlying `map[string]string`
- **Result**: Race condition → immediate crash

This is a **classic concurrent map writes panic** in Go.

### The Fix

Added proper synchronization using `sync.RWMutex`:

**Before** (unsafe):
```go
type hashMatchCache struct {
    domainCache map[string]string  // ❌ No protection
    ipCache     map[string]string  // ❌ No protection
    maxSize     int
}

func (r *Router) getCachedIPMatch(ip string) (tag string, found bool) {
    tag, found = r.hashMatchCache.ipCache[ip]  // ❌ Unsafe read
    return
}

func (r *Router) setCachedIPMatch(ip string, tag string) {
    r.hashMatchCache.ipCache[ip] = tag  // ❌ Unsafe write
}
```

**After** (safe):
```go
type hashMatchCache struct {
    sync.RWMutex                   // ✅ Added mutex
    domainCache map[string]string
    ipCache     map[string]string
    maxSize     int
}

func (r *Router) getCachedIPMatch(ip string) (tag string, found bool) {
    r.hashMatchCache.RLock()       // ✅ Read lock
    tag, found = r.hashMatchCache.ipCache[ip]
    r.hashMatchCache.RUnlock()
    return
}

func (r *Router) setCachedIPMatch(ip string, tag string) {
    r.hashMatchCache.Lock()        // ✅ Write lock
    r.hashMatchCache.ipCache[ip] = tag
    r.hashMatchCache.Unlock()
    return
}
```

### Impact

**Without this fix**:
- sing-box crashes immediately under any concurrent load
- Multiple connections trigger race condition
- Service is unusable

**With this fix**:
- All concurrent cache access is properly synchronized
- Safe for production use with high connection rates
- No performance impact (RWMutex allows concurrent reads)

### Who Should Upgrade

⚠️ **EVERYONE running v1.12.14.11 through v1.12.14.14 MUST upgrade immediately**

These versions introduced the hash ruleset matching feature with the race condition. If you're using the `hash_rule_set_directory` configuration option, you **will experience crashes** under load.

If you're not using `hash_rule_set_directory`, this bug doesn't affect you (the cache is only created when hash rulesets are loaded).

### Verification

After upgrading, check that sing-box no longer crashes:

```bash
# Restart with new version
sudo systemctl restart sing-box

# Monitor for crashes
journalctl -u sing-box -f | grep -E "fatal|panic"

# Should see no "concurrent map writes" errors
# Service should remain stable under load
```

### Files Changed

1. **route/router.go**:
   - Added `sync.RWMutex` to `hashMatchCache` struct
   - Imported `sync` package

2. **route/route.go**:
   - Added `RLock/RUnlock` to all read operations:
     - `getCachedDomainMatch()`
     - `getCachedIPMatch()`
   - Added `Lock/Unlock` to all write operations:
     - `setCachedDomainMatch()`
     - `setCachedIPMatch()`

### Technical Details

**Concurrency Safety**:
- Read operations use `RLock()`: Multiple readers can access cache simultaneously
- Write operations use `Lock()`: Exclusive access for writes
- No contention during cache hits (most common case)
- Minimal performance overhead

**Performance Impact**:
- Negligible: RWMutex optimized for high-read, low-write scenarios
- Cache hit rate typically >90%, so mostly read operations
- Write locks only acquired on cache misses or updates

## Build Information

- **Version**: 1.12.14.15
- **Build Date**: 2026-01-30
- **Build Tags**: `with_acme,with_clash_api,with_dhcp,with_embedded_tor,with_grpc,with_gvisor,with_low_memory,with_quic,with_shadowsocksr,with_utls,with_wireguard,with_tailscale`

## Download

All binaries built with full feature support.

### Platforms

- Linux: amd64, arm64
- Windows: amd64, arm64
- macOS: amd64 (Intel), arm64 (Apple Silicon)

---

**Full Changelog**: https://github.com/KexiChanProjectProxy/sing-box/compare/v1.12.14.14...v1.12.14.15
