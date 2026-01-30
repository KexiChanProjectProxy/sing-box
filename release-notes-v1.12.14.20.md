# sing-box v1.12.14.20

## 🔍 Debug: Enhanced Diagnostic Logging for Hash Rulesets

This release adds detailed debug logging to help diagnose hash ruleset matching issues.

### New Debug Logs

When log level is set to `debug`, you'll now see:

**When matchers are nil**:
```
DEBUG matchHashRuleSets: matchers are nil, skipping
```

**When checking domains**:
```
DEBUG matchHashRuleSets: checking domain=www.google.com, regex count=454
```

**When extracting rules from rulesets**:
```
DEBUG extracted from geosite-google: 0 exact, 0 suffixes, 0 keywords, 3 regex
```

### How to Enable Debug Logging

Edit your `/etc/sing-box/config.json`:

```json
{
  "log": {
    "level": "debug"
  }
}
```

Then restart:
```bash
sudo systemctl restart sing-box
tail -f /var/log/sing-box.log | grep -E "matchHashRuleSets|extracted from|hash ruleset"
```

### What to Look For

**If hash rulesets are working**:
```
DEBUG extracted from geosite-google: 0 exact, 0 suffixes, 0 keywords, 3 regex
INFO router: built domain match index: 0 exact, 0 suffixes, 0 keywords, 454 regex
DEBUG matchHashRuleSets: checking domain=www.google.com, regex count=454
INFO hash ruleset matched (regex): domain=www.google.com → ruleset=geosite-google
```

**If matchers are nil** (hash rulesets not loaded):
```
DEBUG matchHashRuleSets: matchers are nil, skipping
```

**If domain doesn't match any regex**:
```
DEBUG matchHashRuleSets: checking domain=example.com, regex count=454
DEBUG hash ruleset no match: domain=example.com, ip=192.0.2.1
```

### Troubleshooting

1. **See "matchers are nil"** → Hash rulesets not loading, check `hash_rule_set_directory` config
2. **See "regex count=454" but no matches** → Regex patterns not matching domains
3. **Don't see any matchHashRuleSets logs** → Function not being called, check outbound configuration

### Also Includes

- Automated release workflow script (`release.sh`)
- Release process documentation (`RELEASE_WORKFLOW.md`)
- Ensures binaries always match committed code

## Build Information

- **Version**: 1.12.14.20
- **Build Date**: 2026-01-30
- **Build Tags**: `with_acme,with_clash_api,with_dhcp,with_embedded_tor,with_grpc,with_gvisor,with_low_memory,with_quic,with_shadowsocksr,with_utls,with_wireguard,with_tailscale`

## Download

All binaries built with full feature support.

### Platforms

- Linux: amd64, arm64
- Windows: amd64, arm64
- macOS: amd64 (Intel), arm64 (Apple Silicon)

---

**Full Changelog**: https://github.com/KexiChanProjectProxy/sing-box/compare/v1.12.14.19...v1.12.14.20
