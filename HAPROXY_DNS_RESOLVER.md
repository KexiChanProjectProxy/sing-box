# HAProxy-Style DNS Resolver

## Overview

The HAProxy-style DNS resolver is an advanced DNS caching mechanism that implements lazy resolve with hold timers, similar to HAProxy's DNS resolver behavior. It provides fine-grained control over DNS response caching with different TTL policies based on response types.

## Key Concepts

### Soft TTL vs Hard TTL

- **Soft TTL**: The advertised TTL for responses. After this duration, the cached response becomes "stale" but is still returned immediately to the client. A background refresh is triggered asynchronously.
- **Hard TTL**: The maximum lifetime of a cached entry. After this duration, the cached entry is discarded and a blocking refresh is required.

**Relationship**: `hard_ttl = max(soft_ttl * 2, soft_ttl + 30s)`

### Lazy Resolve (Stale-While-Revalidate)

When a cached response passes its soft TTL but is still within the hard TTL:
- The stale cached response is returned **immediately** to the client (no waiting)
- A background refresh is triggered **asynchronously** to update the cache
- Subsequent queries get the fresh data once the background refresh completes

This provides **zero-latency responses** even when cache entries expire.

### Response-Type-Aware Caching

Different DNS response types can have different hold durations:

| Response Type | Hold Option | Description |
|--------------|-------------|-------------|
| SUCCESS (with answers) | `hold_valid` | Successful DNS responses with answer records |
| NXDOMAIN | `hold_nx` | Non-existent domain responses |
| REFUSED | `hold_refused` | Query refused by DNS server |
| SERVFAIL/Other | `hold_other` | Server failure and other error responses |

## Configuration

### DNS Server Configuration

```json
{
  "dns": {
    "servers": [
      {
        "type": "udp",
        "tag": "dns-remote",
        "server": "8.8.8.8",
        "server_port": 53,
        
        "resolve_retries": 3,
        "resolve_timeout": "5s",
        
        "hold_valid": "30m",
        "hold_nx": "10m",
        "hold_refused": "5m",
        "hold_other": "1m",
        "hold_timeout": "1h"
      }
    ]
  }
}
```

### DNS Rule Configuration

You can also apply hold options per DNS rule:

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

## Options

### Hold Options

#### `hold_valid` <duration>

Advertised TTL for successful DNS responses with answers.

**Example**: `"30m"` (30 minutes)

**Behavior**:
- Responses are served immediately from cache for 30 minutes
- After 30 minutes, stale responses are still served but background refresh is triggered
- Hard TTL: 60 minutes (30m × 2)
- Minimum hard TTL: 30m + 30s = 30m 30s

#### `hold_nx` <duration>

Advertised TTL for NXDOMAIN (non-existent domain) responses.

**Example**: `"10m"` (10 minutes)

**Use case**: Cache negative responses to avoid repeated queries for non-existent domains.

#### `hold_refused` <duration>

Advertised TTL for REFUSED DNS responses.

**Example**: `"5m"` (5 minutes)

**Use case**: Handle DNS servers that refuse queries (e.g., due to rate limiting).

#### `hold_other` <duration>

Advertised TTL for other DNS error responses (SERVFAIL, NOTIMP, etc.).

**Example**: `"1m"` (1 minute)

**Use case**: Short cache for transient server errors.

#### `hold_timeout` <duration>

Fallback to stale cached response when DNS queries time out.

**Example**: `"1h"` (1 hour)

**Behavior**: When DNS queries fail (timeout, network error), return the last successfully cached response if available and within the hold_timeout period.

### Retry Options

#### `resolve_retries` <number>

Number of DNS query retry attempts.

**Example**: `3`

**Behavior**: Retry failed DNS queries up to the specified number of times.

#### `resolve_timeout` <duration>

Timeout for each DNS query retry attempt.

**Example**: `"5s"` (5 seconds)

**Behavior**: Each retry attempt has its own timeout. Total timeout = `resolve_timeout × resolve_retries`.

## Usage Examples

### High-Performance Caching

```json
{
  "dns": {
    "servers": [
      {
        "type": "udp",
        "tag": "dns-fast",
        "server": "1.1.1.1",
        "hold_valid": "1h",
        "hold_nx": "30m",
        "hold_timeout": "2h"
      }
    ]
  }
}
```

**Configuration goals**:
- Long cache time for successful responses (1 hour)
- Shorter cache for negative responses (30 minutes)
- Fallback to stale data on extended outages (2 hours)

### Aggressive Refresh

```json
{
  "dns": {
    "servers": [
      {
        "type": "https",
        "tag": "dns-doh",
        "server": "https://dns.cloudflare.com/dns-query",
        "hold_valid": "5m",
        "hold_nx": "1m",
        "hold_other": "30s",
        "resolve_retries": 5,
        "resolve_timeout": "10s"
      }
    ]
  }
}
```

**Configuration goals**:
- Short cache times (5 minutes) for fresh data
- Quick refresh for errors (30 seconds to 1 minute)
- Aggressive retry on failures (5 retries × 10 seconds)

### Conservative Caching

```json
{
  "dns": {
    "servers": [
      {
        "type": "udp",
        "tag": "dns-conservative",
        "server": "8.8.8.8",
        "hold_valid": "10m",
        "hold_nx": "5m",
        "hold_refused": "2m",
        "hold_other": "1m",
        "hold_timeout": "30m",
        "resolve_retries": 2,
        "resolve_timeout": "3s"
      }
    ]
  }
}
```

**Configuration goals**:
- Moderate cache times (10 minutes)
- Shorter cache for errors (1-5 minutes)
- Limited fallback (30 minutes)
- Conservative retry (2 retries × 3 seconds)

## Technical Details

### Sub-Second Durations

Sub-second hold durations are rounded up to 1 second:

| Input Duration | Stored Duration |
|----------------|-----------------|
| `500ms` | `1s` |
| `100ms` | `1s` |
| `1s` | `1s` |
| `5s` | `5s` |
| `0` | `0` (disabled) |

**Why**: The DNS TTL field is a 32-bit integer in seconds. Sub-second values would truncate to 0, disabling caching.

### TTL Adjustment

When a cached response is returned, its TTL is adjusted to reflect remaining cache lifetime:

```
response_ttl = original_ttl - elapsed_time
```

This prevents TTL underflow and ensures clients see accurate TTL values.

**Underflow protection**: When `elapsed_time > original_ttl`, the TTL is set to 0 instead of wrapping around to a large value.

### Background Refresh Deduplication

Multiple concurrent queries for the same expired entry trigger **only one** background refresh. This prevents:
- Duplicate network traffic
- Server overload
- Race conditions

### Independent Cache Per Transport

When `independent_cache: true` is enabled, each DNS transport has its own cache entry. This allows:
- Different hold options per transport
- Transport-specific caching strategies
- Isolation between transports

## Behavior Comparison

### Traditional DNS Caching

```
Query 1 (t=0s)      → Cache Miss → Fetch from DNS server → Cache (TTL: 60s)
Query 2 (t=30s)     → Cache Hit → Return cached
Query 3 (t=61s)     → Cache Expired → Fetch from DNS server → Wait...
```

### HAProxy-Style Caching (hold_valid=30s)

```
Query 1 (t=0s)      → Cache Miss → Fetch from DNS server → Cache (soft: 30s, hard: 60s)
Query 2 (t=30s)     → Soft Expired → Return stale + Trigger background refresh
Query 3 (t=31s)     → Cache Hit (fresh data from background refresh)
Query 4 (t=61s)     → Hard Expired → Cache Miss → Fetch from DNS server
```

**Key difference**: At t=30s, HAProxy-style returns **immediately** with stale data, while traditional caching would **block** waiting for a fresh response.

## Performance Benefits

### Reduced Latency

- **Stale-while-revalidate**: Zero wait time for expired cache entries
- **Background refresh**: Asynchronous updates don't block queries
- **Hold timeout**: Graceful degradation during outages

### Reduced Server Load

- **Longer cache lifetimes**: Fewer queries to upstream DNS servers
- **Deduplication**: One background refresh per expired entry
- **Negative caching**: Avoid repeated queries for non-existent domains

### Improved Reliability

- **Hold timeout**: Serve last-known-good during outages
- **Retry logic**: Automatic retry on transient failures
- **Response-type caching**: Different policies for different response types

## Troubleshooting

### DNS Responses Have TTL=0

**Problem**: Cached responses show TTL of 0.

**Possible causes**:
1. Hold duration is sub-second (e.g., `500ms`) - use ≥ 1 second
2. `disable_cache` is enabled
3. Cache entry has expired (past hard TTL)

### No Background Refresh

**Problem**: Cache entries are not being refreshed in background.

**Possible causes**:
1. `hold_*` options are set to 0 (disabled)
2. Queries are not reaching soft TTL expiration
3. `independent_cache` is causing cache isolation

Check logs for: `background DNS refresh` messages.

### High Memory Usage

**Problem**: DNS cache consuming too much memory.

**Solutions**:
1. Reduce `hold_*` durations
2. Set `cache_capacity` to limit cache size
3. Enable `disable_expire` only if needed (keeps entries forever)

### Stale Data Being Served Too Long

**Problem**: Clients are getting very old DNS data.

**Solutions**:
1. Reduce `hold_*` durations
2. Check system clock accuracy
3. Verify background refresh is completing successfully

## Best Practices

### 1. Match Hold Durations to Use Case

- **Frequently changing records**: Short `hold_valid` (5-15 minutes)
- **Stable records**: Long `hold_valid` (1-24 hours)
- **Non-existent domains**: Moderate `hold_nx` (10-30 minutes)
- **Transient errors**: Short `hold_other` (1-5 minutes)

### 2. Set Appropriate Retry Values

- **Fast network**: Fewer retries, shorter timeout (`retries: 2, timeout: 3s`)
- **Unreliable network**: More retries, longer timeout (`retries: 5, timeout: 10s`)

### 3. Use Hold Timeout for Critical Services

For critical services, set a long `hold_timeout` (1-24 hours) to allow the system to continue functioning during extended DNS outages.

### 4. Monitor Cache Effectiveness

Check logs for:
- `cached` responses (cache hits)
- `exchanged` responses (cache misses)
- `background DNS refresh` (async updates)

High cache hit ratios = better performance.

### 5. Test Before Production

Use the test utilities to verify:
```bash
sing-box dns check --config config.json
```

## Migration from Traditional Caching

### Before (Traditional)

```json
{
  "dns": {
    "servers": [
      {
        "type": "udp",
        "tag": "dns-remote",
        "server": "8.8.8.8",
        "server_port": 53
      }
    ]
  }
}
```

### After (HAProxy-Style)

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

**Benefits**:
- Faster responses (lazy resolve)
- Better resilience (hold timeout)
- More control (response-type caching)

## References

- [HAProxy DNS Resolver Documentation](https://cbonte.github.io/haproxy-dconv/2.4/configuration.html#5.2-resolvers)
- [DNS Rule Actions](/configuration/dns/rule_action/)
- [DNS Server Options](/configuration/dns/server/)
