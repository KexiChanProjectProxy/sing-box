# sing-box v1.12.14.21

## 🐛 CRITICAL FIX: Extract IP Rules from IPSet Field

This release fixes a **critical bug** where geoip .srs files were NOT being parsed correctly, resulting in 0 IP rules being extracted.

### The Bug

When loading .srs geoip files (like geoip-cn.srs from SagerNet/sing-geoip), the IP extraction was completely broken:

**Root cause**:
- geoip .srs files store IPs in `IPSet` field (`*netipx.IPSet` object)
- But extraction code only looked at `IPCIDR` array (which is empty in geoip files)
- **Result**: 0 IPs extracted from geoip files

**Evidence**:
```go
// Before fix - only checked IPCIDR array:
result.IPCIDRs = append(result.IPCIDRs, ruleOpt.DefaultOptions.IPCIDR...)
// For geoip-cn.srs: IPCIDR = [] (empty!) → 0 IPs extracted
```

**What actually happened**:
```
Testing geoip-cn.srs:
  IPCIDR array: [] (empty)
  IPSet field: *netipx.IPSet with 7540 prefixes
  Extracted before fix: 0 IPs ❌
  Extracted after fix: 7540 IPs ✓
```

### The Fix

Now checks BOTH `IPCIDR` array AND `IPSet` object:

```go
// Extract from IPCIDR string array (for JSON rulesets)
result.IPCIDRs = append(result.IPCIDRs, ruleOpt.DefaultOptions.IPCIDR...)

// Extract from IPSet object (for .srs geoip files)  ← NEW!
if ruleOpt.DefaultOptions.IPSet != nil {
    for _, prefix := range ruleOpt.DefaultOptions.IPSet.Prefixes() {
        result.IPCIDRs = append(result.IPCIDRs, prefix.String())
    }
}
```

### Impact

**Before this fix**:
- ❌ geoip .srs files: 0 IPs extracted
- ❌ Hash-based IP matching: completely broken
- ❌ IP interval tree: empty (0 IPv4, 0 IPv6)
- ❌ Only domain regex matching worked

**After this fix**:
- ✅ geoip .srs files: correctly extracts all IP CIDRs
- ✅ Hash-based IP matching: fully functional
- ✅ IP interval tree: populated with thousands of ranges
- ✅ Both domain AND IP matching work

### Example

**geoip-cn.srs before**:
```
INFO router: built IP match index: 0 IPv4, 0 IPv6  ← Empty!
```

**geoip-cn.srs after**:
```
INFO router: built IP match index: 7540 IPv4, 0 IPv6  ← Populated!
INFO hash ruleset matched (ip): ip=1.0.1.1 → ruleset=geoip-cn
```

### Who Should Upgrade

⚠️ **EVERYONE using hash-based load balancing with geoip .srs files MUST upgrade**

If you're using geoip-cn.srs, geoip-us.srs, etc. for hash-based routing, the IP matching has been **completely non-functional** until now.

This also affects regular routing rulesets using geoip .srs files.

### Verification

After upgrading with debug logging enabled:

```bash
# You should now see:
DEBUG extracted from geoip-cn: 0 exact, 0 suffixes, 0 keywords, 0 regex
DEBUG extracted IP rules: 7540 CIDRs  ← NEW!
INFO router: built IP match index: 7540 IPv4, 0 IPv6  ← Populated!

# And when connections match:
INFO hash ruleset matched (ip): ip=1.0.1.1 → ruleset=geoip-cn
```

### Files Changed

1. **route/rule/rule_set_local.go**:
   - Modified `ExtractIPRules()` to check `IPSet` field
   - Convert IPSet prefixes to CIDR strings
   - Works for both routing and hash-only rulesets

### Technical Details

**.srs file structure**:

**Geosite files** (domain rules):
```json
{
  "domain_regex": ["^.*\\.google\\.com$"]
}
```

**Geoip files** (IP rules):
```json
{
  // IPCIDR array is EMPTY!
  // IP data is in IPSet object instead
}
```

The IPSet field is a `*netipx.IPSet` which efficiently stores IP ranges as prefixes. We extract them using:
```go
for _, prefix := range ruleOpt.DefaultOptions.IPSet.Prefixes() {
    result.IPCIDRs = append(result.IPCIDRs, prefix.String())
}
```

### Performance Impact

**Minimal**:
- IPSet.Prefixes() returns already-parsed prefix objects
- String conversion is O(1) per prefix
- One-time cost at startup
- No runtime performance impact

## Build Information

- **Version**: 1.12.14.21
- **Build Date**: 2026-01-30
- **Build Tags**: `with_acme,with_clash_api,with_dhcp,with_embedded_tor,with_grpc,with_gvisor,with_low_memory,with_quic,with_shadowsocksr,with_utls,with_wireguard,with_tailscale`

## Download

All binaries built with full feature support.

### Platforms

- Linux: amd64, arm64
- Windows: amd64, arm64
- macOS: amd64 (Intel), arm64 (Apple Silicon)

---

**Full Changelog**: https://github.com/KexiChanProjectProxy/sing-box/compare/v1.12.14.20...v1.12.14.21
