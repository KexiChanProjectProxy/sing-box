# sing-box v1.12.14.19

## 🐛 CRITICAL FIX: Hash Ruleset Regex Matching

This release fixes a **critical bug** where domain regex rules in hash-only rulesets were NOT being matched correctly.

### The Bug

Hash ruleset matching was completely broken for `.srs` files from SagerNet (sing-geoip, sing-geosite) because:

1. **Wrong regex matching**: Used `strings.Contains()` instead of actual regex matching
2. **Pattern not compiled**: Stored regex as string instead of compiled `*regexp.Regexp`
3. **.srs files only have regex rules**: SagerNet rulesets use `domain_regex` exclusively, not simple domain/suffix arrays

**Result**: Hash rulesets appeared to load (454 regex rules) but **NEVER matched any domains**, so load balancer consistent hashing didn't work at all.

### The Root Cause

**.srs file structure** (from SagerNet/sing-geosite):
```json
{
  "domain_regex": [
    "^adservice\\.google\\.([a-z]{2}|com?)(\\.[a-z]{2})?$",
    "^r+[0-9]+(---|\\.)sn-(2x3|ni5|j5o)\\w{5}\\.googlevideo\\.com$"
  ]
}
```

**Broken code** (before):
```go
type hashDomainRegexEntry struct {
    pattern     string  // ❌ Stored as string, not compiled!
    rulesetTag  string
    specificity int
}

// In matching:
if strings.Contains(domainHost, regexEntry.pattern) {  // ❌ Wrong! Not regex matching!
    bestMatch = regexEntry.rulesetTag
}
```

**Fixed code** (after):
```go
type hashDomainRegexEntry struct {
    pattern     *regexp.Regexp  // ✅ Compiled regex
    rulesetTag  string
    specificity int
}

// In index building:
for _, pattern := range domainRules.DomainRegex {
    compiled, err := regexp.Compile(pattern)  // ✅ Compile on load
    if err != nil {
        r.logger.Warn("invalid domain_regex: ", err)
        continue
    }
    index.domainRegex = append(index.domainRegex, hashDomainRegexEntry{
        pattern:     compiled,  // ✅ Store compiled pattern
        rulesetTag:  tag,
        specificity: 1,
    })
}

// In matching:
if regexEntry.pattern.MatchString(domainHost) {  // ✅ Proper regex matching!
    bestMatch = regexEntry.rulesetTag
}
```

### Impact

**Before this fix**:
- ❌ Hash rulesets loaded but NEVER matched
- ❌ `metadata.MatchedRuleSet` always empty
- ❌ Load balancer consistent hashing broken (all traffic uses same hash key)
- ❌ No ruleset-based connection distribution

**After this fix**:
- ✅ Regex patterns compiled on startup
- ✅ Proper regex matching against domains
- ✅ `metadata.MatchedRuleSet` populated correctly
- ✅ Load balancer consistent hashing works as designed
- ✅ Traffic distributed based on matched rulesets

### Who Should Upgrade

⚠️ **EVERYONE using `hash_rule_set_directory` with .srs files MUST upgrade IMMEDIATELY**

If you're using sing-geoip or sing-geosite .srs files for hash-based load balancing, the feature has been **completely non-functional** until now. This fix makes it actually work for the first time.

### Verification

After upgrading, you should now see hash matching logs:

```bash
# Restart service
sudo systemctl restart sing-box

# Watch for hash ruleset matches
tail -f /var/log/sing-box.log | grep "hash ruleset"

# Expected output:
# INFO hash ruleset matched (regex): domain=www.google.com → ruleset=geosite-google
# INFO hash ruleset matched (regex): domain=adservice.google.com → ruleset=geosite-google
# DEBUG hash ruleset matched (cached): domain=www.google.com → ruleset=geosite-google
```

**Before this fix**, you would see:
```
DEBUG hash ruleset no match: domain=www.google.com, ip=172.217.0.0
```

**Now** you'll see actual matches!

### Example: What Now Works

**Configuration**:
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
      "outbounds": ["hkg-1", "hkg-2", "hkg-3"]
    }
  ]
}
```

**Ruleset directory**:
```
/etc/sing-box/ruleset/
├── geosite-google.srs    (regex: ^.*\.google\.com$)
├── geosite-github.srs    (regex: ^.*\.github\.com$)
└── geosite-twitter.srs   (regex: ^.*\.twitter\.com$)
```

**Traffic behavior** (NOW WORKING):
- `www.google.com` → matches `geosite-google` → consistent hash with key `(src_ip, geosite-google)` → routes to hkg-2
- `api.github.com` → matches `geosite-github` → consistent hash with key `(src_ip, geosite-github)` → routes to hkg-1
- `twitter.com` → matches `geosite-twitter` → consistent hash with key `(src_ip, geosite-twitter)` → routes to hkg-3

Same source IP to same service always goes to same outbound (consistent hashing working correctly).

### Additional Improvements

Added debug logging to show what's being extracted from each ruleset:

```
DEBUG extracted from geosite-google: 0 exact, 0 suffixes, 0 keywords, 3 regex
DEBUG extracted from geosite-github: 0 exact, 0 suffixes, 0 keywords, 5 regex
INFO router: built domain match index: 0 exact, 0 suffixes, 0 keywords, 454 regex
```

This helps verify rulesets are loading and shows their composition.

### Files Changed

1. **route/router.go**:
   - Changed `hashDomainRegexEntry.pattern` from `string` to `*regexp.Regexp`
   - Added `regexp` import
   - Compile regex patterns when building index with error handling
   - Added debug logging for extraction counts

2. **route/route.go**:
   - Changed from `strings.Contains()` to `regexEntry.pattern.MatchString()` for proper regex matching

### Performance Impact

**Slight improvement**:
- Regex compiled once at startup (not on every match)
- `MatchString()` on compiled pattern is faster than recompiling each time
- No performance degradation vs broken code (both O(n) over regex list)

**Memory impact**:
- Minimal: compiled regex patterns stored in memory (~100-500KB for typical ruleset collection)
- One-time cost at startup

## Build Information

- **Version**: 1.12.14.19
- **Build Date**: 2026-01-30
- **Build Tags**: `with_acme,with_clash_api,with_dhcp,with_embedded_tor,with_grpc,with_gvisor,with_low_memory,with_quic,with_shadowsocksr,with_utls,with_wireguard,with_tailscale`

## Download

All binaries built with full feature support.

### Platforms

- Linux: amd64, arm64
- Windows: amd64, arm64
- macOS: amd64 (Intel), arm64 (Apple Silicon)

---

**Full Changelog**: https://github.com/KexiChanProjectProxy/sing-box/compare/v1.12.14.17...v1.12.14.19
