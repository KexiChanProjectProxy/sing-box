# sing-box v1.12.14.14

## Critical Bug Fixes

This release contains **two critical bug fixes**:

### 1. Fix Vless Connection Pool Nil Pointer Crash

**Problem**: Shutdown crashes with segmentation violation:
```
panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x50 pc=0x1844db4]
github.com/sagernet/sing-box/protocol/vless.(*ConnectionPool).Close(0x0)
```

**Root Cause**: The connection pool is **optional** (only created when connection pooling is enabled in config). However, the `Close()` method tried to call methods on the pool without checking if it was nil.

**Fix**: Added `common.PtrOrNil()` wrapper to safely handle nil pool pointer.

**Impact**: No more crashes during sing-box shutdown when vless connection pooling is disabled.

### 2. Improved Hash Ruleset Loading Diagnostics

**Problem**: Users experiencing issues with hash rulesets not loading had **no diagnostic information** in logs.

**Improvements**:
- Added `INFO` log at function start: `"loading hash-only rulesets from directory: /path"`
- Added `DEBUG` log when config is empty: `"hash_rule_set_directory not configured, skipping"`
- Existing debug logs for each loaded ruleset remain

**Now you can diagnose issues**:

```bash
# Check if feature is configured
journalctl -u sing-box | grep "hash_rule_set_directory"

# Check if loading started
journalctl -u sing-box | grep "loading hash-only rulesets"

# Check results
journalctl -u sing-box | grep "loaded .* hash-only rulesets"
```

## Diagnostic Steps for Missing Hash Rulesets

If you're not seeing hash rulesets load:

### Step 1: Check if configured
```bash
grep -i hash_rule_set_directory /etc/sing-box/config.json
```

Should show:
```json
"hash_rule_set_directory": "/etc/sing-box/ruleset"
```

### Step 2: Check startup logs
```bash
journalctl -u sing-box --since "5 minutes ago" | grep "hash"
```

Expected output:
```
INFO loading hash-only rulesets from directory: /etc/sing-box/ruleset
DEBUG loaded hash ruleset: sing-geoip-cn from sing-geoip/cn.json
DEBUG loaded hash ruleset: sing-geoip-us from sing-geoip/us.json
INFO loaded 150 hash-only rulesets from /etc/sing-box/ruleset (including subdirectories)
```

If you see **"hash_rule_set_directory not configured"** → Check your config JSON

If you see **"loading hash-only rulesets"** but no "loaded N rulesets" → Check directory contents:
```bash
find /etc/sing-box/ruleset -name "*.json" -o -name "*.srs"
```

If no files found → Verify your ruleset files are in the right place with correct extensions

### Step 3: Check for errors
```bash
journalctl -u sing-box --since "5 minutes ago" | grep -i "error\|fatal"
```

## Files Changed

1. **protocol/vless/outbound.go**:
   ```go
   func (h *Outbound) Close() error {
       // Before: return common.Close(..., h.connPool)
       // After:  return common.Close(..., common.PtrOrNil(h.connPool))
   }
   ```

2. **route/router.go**:
   ```go
   func (r *Router) LoadHashRuleSetsFromDirectory(...) error {
       if dirPath == "" {
           r.logger.Debug("hash_rule_set_directory not configured, skipping")
           return nil
       }
       r.logger.Info("loading hash-only rulesets from directory: ", dirPath)
       // ...
   }
   ```

## Build Information

- **Version**: 1.12.14.14
- **Build Date**: 2026-01-30
- **Build Tags**: `with_acme,with_clash_api,with_dhcp,with_embedded_tor,with_grpc,with_gvisor,with_low_memory,with_quic,with_shadowsocksr,with_utls,with_wireguard,with_tailscale`

## Download

All binaries built with full feature support.

### Platforms

- Linux: amd64, arm64
- Windows: amd64, arm64
- macOS: amd64 (Intel), arm64 (Apple Silicon)

---

**Full Changelog**: https://github.com/KexiChanProjectProxy/sing-box/compare/v1.12.14.13...v1.12.14.14
