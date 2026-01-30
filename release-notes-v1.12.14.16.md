# sing-box v1.12.14.16

## 🐛 Bug Fix: Too Many Open Files

This release fixes the **"too many open files"** error when loading hash-only rulesets from directories.

### The Problem

```
ERROR router: watch rule-set file: fswatch: create fsnotify watcher: too many open files
```

When loading hash-only rulesets from a directory (e.g., sing-geoip, sing-geosite with hundreds or thousands of ruleset files), the router was creating a **file watcher for every single ruleset file**.

**Impact**:
- Hundreds/thousands of file descriptors consumed by fswatch
- Hits system limits (typically 1024 or 4096 open files)
- Service fails to start with "too many open files" error
- Even `LimitNOFILE=infinity` in systemd couldn't prevent it

### Root Cause

Hash-only rulesets are loaded once at startup and used exclusively for hash key generation in load balancers. They:
- **Don't need hot-reload** - they're static configuration data
- **Don't affect routing decisions** - only used to populate `metadata.MatchedRuleSet`
- **Don't change during runtime** - no point watching for file changes

But the code was creating file watchers (fswatch) for **every hash-only ruleset**, treating them the same as routing rulesets (which DO need hot-reload).

With typical geoip/geosite collections containing 200-2000+ files, this meant:
- 200-2000+ file watchers
- 200-2000+ file descriptors consumed
- **Immediate failure** on systems with default fd limits

### The Fix

Added a `DisableWatcher` flag to skip file watching for hash-only rulesets:

**1. Added internal flag to option.RuleSet** (`option/rule_set.go`):
```go
type _RuleSet struct {
    Type           string        `json:"type,omitempty"`
    Tag            string        `json:"tag"`
    Format         string        `json:"format,omitempty"`
    // ... other fields ...
    DisableWatcher bool          `json:"-"` // Internal: disable file watching for hash-only rulesets
}
```

**2. Modified ruleset loading to skip watcher creation** (`route/rule/rule_set_local.go`):
```go
// Only create file watcher if not disabled (hash-only rulesets disable watching)
if !options.DisableWatcher {
    watcher, err := fswatch.NewWatcher(fswatch.Options{
        Path: []string{filePath},
        Callback: func(path string) {
            uErr := ruleSet.reloadFile(path)
            if uErr != nil {
                logger.Error(E.Cause(uErr, "reload rule-set ", options.Tag))
            }
        },
    })
    if err != nil {
        return nil, err
    }
    ruleSet.watcher = watcher
}
```

**3. Set flag when loading hash rulesets** (`route/router.go`):
```go
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
```

### Impact

**Before this fix**:
- ❌ Loading 200 hash rulesets → 200 file watchers → "too many open files" error
- ❌ Service fails to start
- ❌ Unusable with large ruleset collections

**After this fix**:
- ✅ Loading 200 hash rulesets → **0 file watchers**
- ✅ Minimal file descriptor usage
- ✅ Works with any number of hash rulesets
- ✅ Service starts normally
- ✅ No performance impact

### Who Should Upgrade

⚠️ **EVERYONE using `hash_rule_set_directory` MUST upgrade**

If you're using the hash-only ruleset feature (introduced in v1.12.14.11) with a directory configuration, you're likely experiencing this issue and the service is failing to start.

If you're NOT using `hash_rule_set_directory`, this fix doesn't affect you.

### Verification

After upgrading, verify the fix worked:

```bash
# Restart service
sudo systemctl restart sing-box

# Check service status
sudo systemctl status sing-box

# Should see successful startup with hash rulesets loaded
journalctl -u sing-box -n 100 | grep -E "hash-only rulesets|too many open files"

# Should see log like:
# INFO loaded 200 hash-only rulesets from /etc/sing-box/ruleset (including subdirectories)

# Should NOT see:
# ERROR router: watch rule-set file: fswatch: create fsnotify watcher: too many open files
```

Check file descriptor usage (should be low):

```bash
# Get sing-box PID
PID=$(pgrep sing-box)

# Check open file descriptors
ls -l /proc/$PID/fd | wc -l

# Should be low (< 100) instead of hundreds/thousands
```

### Files Changed

1. **option/rule_set.go**:
   - Added `DisableWatcher bool` field to `_RuleSet` struct (internal flag, not serialized to JSON)

2. **route/rule/rule_set_local.go**:
   - Modified `NewLocalRuleSet()` to check `DisableWatcher` flag
   - Skip fswatch.NewWatcher() creation when flag is true

3. **route/router.go**:
   - Set `DisableWatcher = true` when creating hash-only ruleset options in `LoadHashRuleSetsFromDirectory()`

### Technical Details

**Why hash-only rulesets don't need file watching**:
- They're loaded once at startup
- Used only for hash key generation (load balancer consistent hashing)
- Don't affect routing decisions
- Static configuration data that doesn't change during runtime
- Hot-reload would provide no benefit (only hash keys, not routing behavior)

**File descriptor savings**:
- Before: N rulesets = N file watchers = N file descriptors
- After: N rulesets = 0 file watchers = 0 file descriptors
- Typical savings: 200-2000 file descriptors for geoip/geosite collections

**Routing rulesets still have hot-reload**:
- This change ONLY affects hash-only rulesets loaded from `hash_rule_set_directory`
- Regular routing rulesets (configured in `route.rule_set`) still have file watching enabled
- Hot-reload continues to work for routing configuration changes

## Build Information

- **Version**: 1.12.14.16
- **Build Date**: 2026-01-30
- **Build Tags**: `with_acme,with_clash_api,with_dhcp,with_embedded_tor,with_grpc,with_gvisor,with_low_memory,with_quic,with_shadowsocksr,with_utls,with_wireguard,with_tailscale`

## Download

All binaries built with full feature support.

### Platforms

- Linux: amd64, arm64
- Windows: amd64, arm64
- macOS: amd64 (Intel), arm64 (Apple Silicon)

---

**Full Changelog**: https://github.com/KexiChanProjectProxy/sing-box/compare/v1.12.14.15...v1.12.14.16
