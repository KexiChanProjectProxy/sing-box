# sing-box v1.12.14.17

## ✨ Enhancement: Hash Ruleset Matching Logs

This release adds comprehensive logging for hash-only ruleset matching to help verify and debug load balancer consistent hashing behavior.

### What's New

Added detailed logging to show when hash rulesets are matched and which ruleset tag is being used for load balancer hashing.

### Log Output Examples

**INFO Level - Hash Ruleset Matches**:
```
INFO hash ruleset matched (exact): domain=www.google.com → ruleset=sing-geosite-google
INFO hash ruleset matched (suffix): domain=api.github.com → ruleset=sing-geosite-github
INFO hash ruleset matched (keyword): domain=cdn.example.com → ruleset=sing-geosite-cdn
INFO hash ruleset matched (ip): ip=1.1.1.1 → ruleset=sing-geoip-cloudflare
```

**DEBUG Level - Cache Hits** (when same domain/IP is matched again):
```
DEBUG hash ruleset matched (cached): domain=www.google.com → ruleset=sing-geosite-google
DEBUG hash ruleset matched (cached): ip=1.1.1.1 → ruleset=sing-geoip-cloudflare
```

**DEBUG Level - No Match**:
```
DEBUG hash ruleset no match: domain=unknown.example.com, ip=192.0.2.1
```

### Match Types Shown in Logs

The logs now indicate HOW the domain was matched:
- **`(exact)`** - Exact domain match (highest specificity)
- **`(suffix)`** - Domain suffix match
- **`(keyword)`** - Domain keyword match
- **`(regex)`** - Domain regex pattern match
- **`(ip)`** - IP CIDR match
- **`(cached)`** - Previously matched result from cache (fast path)

### Why This Matters

Hash-only rulesets are used for **consistent hashing in load balancers**. Previously, there was no visibility into:
- Whether hash rulesets were actually matching connections
- Which ruleset tag was being used for the hash key
- Cache hit rates for performance monitoring

With these logs, you can now:
1. **Verify** hash rulesets are working correctly
2. **Debug** load balancer routing decisions
3. **Monitor** cache effectiveness (cache hits vs misses)
4. **Troubleshoot** why connections are routing to specific outbounds

### Example Configuration & Expected Logs

**Configuration**:
```json
{
  "route": {
    "hash_rule_set_directory": "/etc/sing-box/ruleset",
    "rules": [
      {
        "outbound": "load-balancer"
      }
    ]
  },
  "outbounds": [
    {
      "type": "load_balance",
      "tag": "load-balancer",
      "strategy": "consistent_hash",
      "hash": {
        "key_parts": ["src_ip", "matched_ruleset"]
      },
      "outbounds": ["hkg-1", "hkg-2", "hkg-3"]
    }
  ]
}
```

**With directory structure**:
```
/etc/sing-box/ruleset/
├── sing-geoip/
│   ├── cn.srs
│   ├── us.srs
│   └── jp.srs
└── sing-geosite/
    ├── google.srs
    ├── github.srs
    └── cloudflare.srs
```

**Expected logs when connecting**:
```
INFO inbound/redirect[redirect]: inbound connection to www.google.com:443
INFO hash ruleset matched (suffix): domain=www.google.com → ruleset=sing-geosite-google
INFO outbound/anytls[hkg-2]: outbound connection to www.google.com:443

INFO inbound/redirect[redirect]: inbound connection to api.github.com:443
INFO hash ruleset matched (suffix): domain=api.github.com → ruleset=sing-geosite-github
INFO outbound/anytls[hkg-1]: outbound connection to api.github.com:443

INFO inbound/redirect[redirect]: inbound connection to www.google.com:443
DEBUG hash ruleset matched (cached): domain=www.google.com → ruleset=sing-geosite-google
INFO outbound/anytls[hkg-2]: outbound connection to www.google.com:443
```

Notice:
- First connection to google.com: **INFO level** (new match, added to cache)
- Second connection to google.com: **DEBUG level** (cache hit, fast path)
- **Same outbound** (hkg-2) is used for google.com due to consistent hashing with `matched_ruleset=sing-geosite-google`
- **Different outbound** (hkg-1) is used for github.com due to different hash key `matched_ruleset=sing-geosite-github`

### Performance Impact

The logging has **minimal performance impact**:
- Only logs when a match occurs (not every connection attempt)
- Uses efficient string concatenation
- DEBUG level cache hits can be disabled in production if needed
- INFO level matches provide valuable insights without spam

### Log Level Recommendations

**Development/Testing**:
```json
{
  "log": {
    "level": "debug"
  }
}
```
Shows all matches including cache hits, no matches, and full debugging information.

**Production**:
```json
{
  "log": {
    "level": "info"
  }
}
```
Shows only new matches (not cached), which is useful for monitoring without excessive logs.

### Files Changed

1. **route/route.go**:
   - Added `matchType` variable to track match method (exact/suffix/keyword/regex/ip)
   - Added INFO level logging for new hash ruleset matches with match type
   - Added DEBUG level logging for cache hits
   - Added DEBUG level logging when no hash ruleset matches

### Verification

After upgrading, you should see logs like this:

```bash
# Restart service
sudo systemctl restart sing-box

# Watch for hash ruleset matching
journalctl -u sing-box -f | grep "hash ruleset"

# Expected output:
# INFO hash ruleset matched (suffix): domain=example.com → ruleset=sing-geosite-example
# DEBUG hash ruleset matched (cached): domain=example.com → ruleset=sing-geosite-example
```

If you see **no logs at all**, it means:
1. No hash rulesets are loaded (check for "loaded N hash-only rulesets" at startup)
2. No connections are matching hash rulesets (check your ruleset files)
3. Hash ruleset matching is not being called (check outbound configuration)

### Troubleshooting

**No "hash ruleset matched" logs appear**:
- Check startup logs for "loaded N hash-only rulesets from ..."
- Verify ruleset files exist and are valid (.srs or .json)
- Confirm connections are actually happening (check inbound logs)
- Make sure outbound is configured (hash matching only happens during routing)

**Only seeing DEBUG level logs**:
- This is normal! DEBUG logs show cache hits (very frequent)
- Set `log.level: "debug"` to see cache hits and misses
- First connection to a domain/IP will show INFO level (new match)
- Subsequent connections show DEBUG level (cache hit)

**No matches but rulesets loaded**:
- Check ruleset content matches actual traffic
- Verify domain/IP rules are correct in ruleset files
- Use DEBUG level to see "no match" logs with domain/IP details

## Build Information

- **Version**: 1.12.14.17
- **Build Date**: 2026-01-30
- **Build Tags**: `with_acme,with_clash_api,with_dhcp,with_embedded_tor,with_grpc,with_gvisor,with_low_memory,with_quic,with_shadowsocksr,with_utls,with_wireguard,with_tailscale`

## Download

All binaries built with full feature support.

### Platforms

- Linux: amd64, arm64
- Windows: amd64, arm64
- macOS: amd64 (Intel), arm64 (Apple Silicon)

---

**Full Changelog**: https://github.com/KexiChanProjectProxy/sing-box/compare/v1.12.14.16...v1.12.14.17
