# Release Notes v1.12.14.28

## Release Date

2025-02-18

## Overview

This release completes the HAProxy-style DNS resolver implementation with lazy resolve and hold timers, fixing critical bugs and adding comprehensive test coverage.

---

## 🚀 New Features

### HAProxy-Style DNS Resolver

Implemented HAProxy-style DNS resolver with advanced caching capabilities:

#### Hold Options

| Option | Type | Description |
|--------|------|-------------|
| `hold_valid` | duration | Advertised TTL for successful responses with answers |
| `hold_nx` | duration | Advertised TTL for NXDOMAIN responses |
| `hold_refused` | duration | Advertised TTL for REFUSED responses |
| `hold_other` | duration | Advertised TTL for SERVFAIL and other error responses |
| `hold_timeout` | duration | Fallback to stale cache on DNS timeout |
| `resolve_retries` | number | Number of DNS query retry attempts |
| `resolve_timeout` | duration | Timeout for each retry attempt |

#### Key Features

- **Lazy Resolve (Stale-While-Revalidate)**: Serve stale data immediately while refreshing in background
- **Response-Type-Aware Caching**: Different hold durations for different response types
- **Background Refresh Deduplication**: Prevents duplicate refresh goroutines
- **Sub-Second Duration Rounding**: Sub-second values rounded up to 1s minimum
- **TTL Underflow Protection**: Prevents uint32 wrap-around in TTL calculations
- **Hold Timeout Fallback**: Graceful degradation during DNS outages

---

## 🐛 Bug Fixes

### DNS Client

- **Fixed sub-second duration truncation**: `durationToSeconds()` helper rounds sub-second durations to 1s minimum instead of truncating to 0
  - **Impact**: Sub-second hold durations (e.g., 500ms) now work correctly instead of silently disabling caching
- **Fixed response-type-aware soft TTL**: `holdDurationForResponse()` selects appropriate hold duration based on response type
  - **Impact**: NXDOMAIN, REFUSED, and SERVFAIL responses now use their respective hold options for soft TTL
- **Fixed uint32 underflow in TTL adjustment**: Added bounds checking to prevent wrap-around
  - **Impact**: Prevents corrupted TTL values when cached responses have been held for a long time
- **Implemented HoldTimeout fallback**: DNS timeouts now return last-known-good cached response
  - **Impact**: System remains functional during extended DNS outages

---

## ✨ Improvements

### Test Coverage

Added 13 comprehensive test cases for DNS client:

1. `TestClient_LazyRefresh` - Stale-while-revalidate with background refresh
2. `TestClient_Retry` - Retry exhaustion error handling
3. `TestClient_HoldNX` - NXDOMAIN caching with correct soft TTL
4. `TestClient_HoldRefused` - REFUSED response caching
5. `TestClient_HoldOther_SERVFAIL` - SERVFAIL response caching
6. `TestClient_HoldTimeout_FallbackToStale` - Timeout fallback to cached data
7. `TestClient_RetrySuccessOnSecondAttempt` - Retry success on attempt 2
8. `TestClient_BackgroundRefreshDeduplication` - Concurrent query deduplication
9. `TestClient_MixedHoldOptions` - Different hold durations per response type
10. `TestClient_CacheDisabledWithHoldOptions` - Hold options ignored when cache disabled
11. `TestClient_IndependentCacheWithHold` - Independent cache per transport
12. `TestClient_TTLAdjustmentNoUnderflow` - Record TTLs without underflow
13. `TestClient_SubSecondHoldDuration` - Sub-second duration rounding

All tests pass with race detector clean.

### Documentation

- Updated DNS rule action documentation with hold options
- Created comprehensive HAProxy DNS resolver guide
- Added configuration examples and best practices

---

## 📝 Configuration Examples

### Basic Configuration

```json
{
  "dns": {
    "servers": [
      {
        "type": "udp",
        "tag": "dns-remote",
        "server": "8.8.8.8",
        "server_port": 53,
        "hold_valid": "30m",
        "hold_nx": "10m",
        "hold_refused": "5m",
        "hold_other": "1m",
        "hold_timeout": "1h",
        "resolve_retries": 3,
        "resolve_timeout": "5s"
      }
    ]
  }
}
```

### Per-Rule Configuration

```json
{
  "dns": {
    "rules": [
      {
        "rule_set": "geosite-openai",
        "server": "dns-openai",
        "hold_valid": "1h",
        "hold_nx": "30m",
        "hold_timeout": "2h"
      }
    ]
  }
}
```

---

## 🔧 Technical Details

### Cache Behavior

- **Soft TTL**: Response served immediately + background refresh triggered
- **Hard TTL**: `soft_ttl * 2` (minimum `soft_ttl + 30s`)
- **Sub-second rounding**: 500ms → 1s, 100ms → 1s

### Implementation Files

| File | Changes |
|------|---------|
| `dns/client.go` | Added `durationToSeconds()`, `holdDurationForResponse()`, fixed `loadResponseWithHold()`, implemented `HoldTimeout` fallback |
| `dns/client_test.go` | Added 13 comprehensive test cases |
| `adapter/dns.go` | Added HAProxy-style hold options to `DNSQueryOptions` |
| `docs/configuration/dns/rule_action.md` | Added hold options documentation |

---

## 🔄 Breaking Changes

None. This is a pure feature addition with backward compatibility maintained.

---

## ⚠️ Known Issues

None.

---

## 📊 Performance Impact

### Benefits

- **Reduced latency**: Stale-while-revalidate provides zero-wait responses
- **Reduced server load**: Longer cache lifetimes reduce upstream DNS queries
- **Improved reliability**: Hold timeout provides graceful degradation

### Considerations

- **Memory usage**: Longer cache lifetimes may increase memory consumption
- **Staleness**: Stale data may be served for up to the hold duration

---

## 📚 Documentation

- [HAProxy DNS Resolver Guide](./HAPROXY_DNS_RESOLVER.md)
- [DNS Rule Actions](./docs/configuration/dns/rule_action.md)
- [DNS Server Options](./docs/configuration/dns/server/)

---

## 🙏 Acknowledgments

This implementation is inspired by HAProxy's DNS resolver functionality:
https://cbonte.github.io/haproxy-dconv/2.4/configuration.html#5.2-resolvers

---

## 📦 Checksums

```
d6e4d391d61615253f9ecb091119a62b2e64c7a498726b314cfd149200de1c6d  sing-box-1.12.14.28-darwin-amd64
5c2cd405263fa71e13798f399b29b6b9412d67414d34194e6a2822a759315302  sing-box-1.12.14.28-darwin-arm64
c46c7323be27417a43335aaf536b57ada650a199ca36c0b87433fc503701eacd  sing-box-1.12.14.28-linux-amd64
4c36784b497536a039186545635a3fca4cd848be22698c0aac7e5fab51ff5322  sing-box-1.12.14.28-linux-arm64
03246997b96a24507c8036b4e28c70e15044c774d8d5f62984f90fee76c3bf2e  sing-box-1.12.14.28-windows-amd64.exe
```

---

## 📥 Download

- [Linux amd64](https://github.com/KexiChanProjectProxy/sing-box/releases/download/v1.12.14.28/sing-box-1.12.14.28-linux-amd64)
- [Linux arm64](https://github.com/KexiChanProjectProxy/sing-box/releases/download/v1.12.14.28/sing-box-1.12.14.28-linux-arm64)
- [macOS amd64](https://github.com/KexiChanProjectProxy/sing-box/releases/download/v1.12.14.28/sing-box-1.12.14.28-darwin-amd64)
- [macOS arm64](https://github.com/KexiChanProjectProxy/sing-box/releases/download/v1.12.14.28/sing-box-1.12.14.28-darwin-arm64)
- [Windows amd64](https://github.com/KexiChanProjectProxy/sing-box/releases/download/v1.12.14.28/sing-box-1.12.14.28-windows-amd64.exe)

---

## 🔗 Links

- **GitHub**: https://github.com/KexiChanProjectProxy/sing-box
- **Release**: https://github.com/KexiChanProjectProxy/sing-box/releases/tag/v1.12.14.28
- **Documentation**: https://github.com/KexiChanProjectProxy/sing-box/wiki

---

**Full Changelog**: https://github.com/KexiChanProjectProxy/sing-box/compare/v1.12.14.27...v1.12.14.28
