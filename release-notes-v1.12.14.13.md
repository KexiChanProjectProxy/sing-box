# sing-box v1.12.14.13

## Critical Bug Fix: .srs Binary Format Parsing

This release fixes a **critical bug** that prevented `.srs` (binary ruleset) files from loading correctly.

### The Problem

In v1.12.14.12, when loading hash rulesets from directories containing `.srs` files, sing-box would crash with:

```
FATAL create service: load hash rulesets: walk hash ruleset directory: /etc/sing-box/ruleset:
load hash ruleset: sing-geoip/geoip-ad.srs: invalid character 'S' looking for beginning of value
```

**Root cause**: The code was not setting the file format, so it tried to parse binary `.srs` files as JSON text.

### The Fix

Now **auto-detects the format** based on file extension:
- `.srs` files → Binary format (`RuleSetFormatBinary`)
- `.json` files → JSON source format (`RuleSetFormatSource`)

### What This Means

Your directory structure now works correctly:

```
/etc/sing-box/ruleset/
  ├── sing-geoip/
  │   ├── cn.srs        ✅ Now loads correctly as binary
  │   ├── us.srs        ✅ Now loads correctly as binary
  │   └── private.json  ✅ Still loads as JSON
  └── sing-geosite/
      ├── google.srs    ✅ Now loads correctly as binary
      └── netflix.json  ✅ Still loads as JSON
```

### Testing

After upgrading, you should see successful loading:

```bash
journalctl -u sing-box | grep "hash-only rulesets"
# Should show: INFO loaded N hash-only rulesets from /etc/sing-box/ruleset (including subdirectories)
```

**No more FATAL errors!**

### Files Changed

- `route/router.go`: Added format auto-detection logic
  ```go
  if strings.HasSuffix(name, ".srs") {
      rulesetOptions.Format = C.RuleSetFormatBinary
  } else {
      rulesetOptions.Format = C.RuleSetFormatSource
  }
  ```

### Upgrade Path

**From v1.12.14.11 or earlier:**
- You had subdirectories being skipped entirely
- Upgrade to v1.12.14.13 (skip v1.12.14.12)

**From v1.12.14.12:**
- You had FATAL errors on .srs files
- Upgrade to v1.12.14.13 immediately

### Technical Details

The `.srs` format is sing-box's compiled rule-set binary format. It's created with:
```bash
sing-box rule-set compile source.json output.srs
```

Binary format benefits:
- Faster loading
- Smaller file size
- Pre-compiled for performance

This fix ensures both source JSON and compiled binary formats work seamlessly together.

## Build Information

- **Version**: 1.12.14.13
- **Build Date**: 2026-01-30
- **Build Tags**: `with_acme,with_clash_api,with_dhcp,with_embedded_tor,with_grpc,with_gvisor,with_low_memory,with_quic,with_shadowsocksr,with_utls,with_wireguard,with_tailscale`

## Download

All binaries built with full feature support.

### Platforms

- Linux: amd64, arm64
- Windows: amd64, arm64
- macOS: amd64 (Intel), arm64 (Apple Silicon)

---

**Full Changelog**: https://github.com/KexiChanProjectProxy/sing-box/compare/v1.12.14.12...v1.12.14.13
