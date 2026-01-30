# sing-box v1.12.14.12

## Bug Fix: Recursive Subdirectory Support for Hash Rulesets

This release fixes the hash ruleset loading feature to properly support **recursive subdirectory scanning**.

### What Was Fixed

In v1.12.14.11, the hash ruleset loader only looked for `.json` and `.srs` files directly in the configured directory and **skipped all subdirectories**. This meant if you had a structure like:

```
/etc/sing-box/ruleset/
  ├── sing-geoip/      <- SKIPPED
  │   ├── cn.json
  │   └── us.json
  └── sing-geosite/    <- SKIPPED
      ├── google.json
      └── netflix.srs
```

**None of these rulesets would be loaded!**

### Now Fixed

The loader now **recursively scans all subdirectories** and loads all `.json` and `.srs` files found anywhere in the tree.

### Tag Naming Convention

Tags are now generated from the **relative path** with directory separators replaced by hyphens:

```
/etc/sing-box/ruleset/
  ├── sing-geoip/
  │   ├── cn.json      → tag: "sing-geoip-cn"
  │   ├── us.json      → tag: "sing-geoip-us"
  │   └── private.srs  → tag: "sing-geoip-private"
  ├── sing-geosite/
  │   ├── google.json  → tag: "sing-geosite-google"
  │   └── netflix.srs  → tag: "sing-geosite-netflix"
  ├── gaming/
  │   ├── steam.json   → tag: "gaming-steam"
  │   └── epic.json    → tag: "gaming-epic"
  └── custom.json      → tag: "custom"
```

### Improved Logging

**New debug log for each loaded ruleset:**
```
DEBUG loaded hash ruleset: sing-geoip-cn from sing-geoip/cn.json
DEBUG loaded hash ruleset: sing-geoip-us from sing-geoip/us.json
DEBUG loaded hash ruleset: sing-geosite-google from sing-geosite/google.json
```

**Improved info log:**
```
INFO loaded 150 hash-only rulesets from /etc/sing-box/ruleset (including subdirectories)
```

**Better error messages:**
- Clear error if directory doesn't exist
- Warning if no `.json` or `.srs` files found anywhere in tree

### Example Configuration

```json
{
  "route": {
    "hash_rule_set_directory": "/etc/sing-box/ruleset"
  },
  "outbounds": [
    {
      "type": "load_balance",
      "tag": "lb",
      "strategy": "consistent_hash",
      "hash": {
        "key_parts": ["src_ip", "matched_ruleset"]
      },
      "outbounds": ["proxy1", "proxy2", "proxy3"]
    }
  ]
}
```

### Behavior

With the above configuration and directory structure:
- Connections to CN IPs → hash includes "sing-geoip-cn" → consistent routing
- Connections to Google domains → hash includes "sing-geosite-google" → different consistent route
- Connections to Steam domains → hash includes "gaming-steam" → another consistent route

## Technical Details

### Changes

- Replaced `os.ReadDir()` with `filepath.WalkDir()` for recursive scanning
- Added relative path calculation for tag generation
- Added directory existence validation
- Added debug logging per-ruleset
- Improved error messages

### Files Modified

- `route/router.go`: Complete rewrite of `LoadHashRuleSetsFromDirectory()`
- `constant/version.go`: Version bump to 1.12.14.12
- `build-releases.sh`: Version update

## Backward Compatibility

✅ **100% backward compatible**
- If you had files directly in the directory, they still work exactly the same
- Subdirectory scanning is automatic, no configuration changes needed
- Tag naming for top-level files unchanged

## Testing

To verify it's working:

1. **Check startup logs:**
```bash
journalctl -u sing-box | grep "hash-only rulesets"
# Should show: INFO loaded N hash-only rulesets from /path (including subdirectories)
```

2. **Check debug logs:**
```bash
journalctl -u sing-box | grep "loaded hash ruleset:"
# Should show each ruleset with its tag and relative path
```

3. **List your ruleset files:**
```bash
find /etc/sing-box/ruleset -name "*.json" -o -name "*.srs"
```

4. **Verify loading:**
The number of rulesets loaded should match the number of `.json` and `.srs` files found.

## Build Information

- **Version**: 1.12.14.12
- **Build Date**: 2026-01-30
- **Build Tags**: `with_acme,with_clash_api,with_dhcp,with_embedded_tor,with_grpc,with_gvisor,with_low_memory,with_quic,with_shadowsocksr,with_utls,with_wireguard,with_tailscale`

## Download

All binaries are built with full feature support.

### Platforms

- Linux: amd64, arm64
- Windows: amd64, arm64
- macOS: amd64 (Intel), arm64 (Apple Silicon)

---

**Full Changelog**: https://github.com/KexiChanProjectProxy/sing-box/compare/v1.12.14.11...v1.12.14.12
