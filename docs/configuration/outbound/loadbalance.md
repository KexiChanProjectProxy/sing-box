### Structure

```json
{
  "type": "loadbalance",
  "tag": "lb",

  "primary_outbounds": [
    "proxy-a",
    "proxy-b",
    "proxy-c"
  ],
  "backup_outbounds": [
    "proxy-backup-1",
    "proxy-backup-2"
  ],
  "url": "https://www.gstatic.com/generate_204",
  "interval": "3m",
  "timeout": "5s",
  "idle_timeout": "30m",
  "top_n": {
    "primary": 3,
    "backup": 1
  },
  "strategy": "consistent_hash",
  "hash": {
    "key_parts": ["src_ip", "matched_ruleset_or_etld"],
    "virtual_nodes": 100,
    "on_empty_key": "random",
    "key_salt": ""
  },
  "hysteresis": {
    "primary_failures": 3,
    "backup_hold_time": "5m"
  },
  "empty_pool_action": "reject",
  "interrupt_exist_connections": false
}
```

!!! info "LoadBalance Overview"

    The LoadBalance outbound provides intelligent load distribution across multiple proxy servers with health checking, failover, and flexible routing strategies. It supports URL-test driven Top-N selection (similar to HAProxy), consistent hashing, and automatic failover with hysteresis.

### Fields

#### primary_outbounds

==Required==

List of primary outbound tags to load balance across. These outbounds are continuously health-checked and ranked by latency.

#### backup_outbounds

List of backup outbound tags. Backup outbounds are only used when primary outbounds are unavailable or when hysteresis conditions trigger failover.

#### url

The URL to test for health checking. `https://www.gstatic.com/generate_204` will be used if empty.

#### interval

Health check interval. `3m` will be used if empty.

#### timeout

Health check timeout per outbound. `5s` will be used if empty.

#### idle_timeout

The idle timeout. `30m` will be used if empty.

#### top_n

URL-test driven Top-N selection configuration. This determines how many healthy outbounds from each pool are used for load balancing.

##### top_n.primary

Number of fastest primary outbounds to use. Default: use all healthy primary outbounds.

When set to a value like `3`, only the 3 fastest (lowest latency) primary outbounds will be included in the load balancing pool.

##### top_n.backup

Number of fastest backup outbounds to use. Default: use all healthy backup outbounds.

#### strategy

==Required==

Load balancing strategy. Available values:

- `random`: Random selection from healthy outbounds
- `consistent_hash`: Consistent hashing based on connection metadata

#### hash

Consistent hash configuration. Only used when `strategy` is `consistent_hash`.

##### hash.key_parts

==Required for consistent_hash==

Array of metadata fields to use for hash key construction. Parts are joined with `|` separator.

Available key parts:

| Key Part | Description | Example |
|----------|-------------|---------|
| `src_ip` | Source IP address | `192.168.1.100` |
| `dst_ip` | Destination IP address | `8.8.8.8` |
| `src_port` | Source port number | `12345` |
| `dst_port` | Destination port number | `443` |
| `network` | Network type | `tcp` or `udp` |
| `domain` | Full destination domain name | `api.example.com` |
| `inbound_tag` | Tag of the inbound that accepted the connection | `mixed-in` |
| `matched_ruleset` | Tag of the ruleset that matched this connection | `geosite-netflix` |
| `etld_plus_one` | eTLD+1 of the destination domain (PSL-based) | `example.com` |
| `matched_ruleset_or_etld` | **Smart fallback**: Use matched ruleset if available, otherwise eTLD+1 | See below |

**Common Configurations:**

- Traditional 5-tuple hashing: `["src_ip", "dst_ip", "dst_port"]`
- Per-source IP: `["src_ip"]`
- By ruleset category: `["src_ip", "matched_ruleset"]`
- By top domain: `["src_ip", "etld_plus_one"]`
- **Smart routing**: `["src_ip", "matched_ruleset_or_etld"]`

##### hash.virtual_nodes

Number of virtual nodes per real outbound in the consistent hash ring. Higher values provide better distribution but use more memory. Default: `100`.

Recommended values: 50-200

##### hash.on_empty_key

Behavior when hash key is empty. Available values:

- `random`: Select a random outbound (default)
- `hash_empty`: Hash the empty string (always selects the same outbound)

##### hash.key_salt

Optional salt prefix for hash key namespace isolation. Useful when multiple LoadBalance instances need different hash rings.

#### hysteresis

Hysteresis configuration for failover damping. Prevents rapid switching between primary and backup pools.

##### hysteresis.primary_failures

Number of consecutive primary pool failures before failing over to backup pool. Default: `1` (immediate failover).

##### hysteresis.backup_hold_time

Minimum time to stay on backup pool before returning to primary pool. Default: `0s` (return immediately when primary recovers).

#### empty_pool_action

Action to take when all outbounds are unavailable. Available values:

- `reject`: Reject the connection (default)
- `direct`: Use direct connection

#### interrupt_exist_connections

Interrupt existing connections when the active outbound pool changes.

Only inbound connections are affected by this setting, internal connections will always be interrupted.

---

## Hash-Based Routing Modes

### matched_ruleset_or_etld: Smart Fallback Mode

The `matched_ruleset_or_etld` key part provides intelligent routing that adapts based on whether the connection matched a rule set.

**Priority Logic:**

1. **Ruleset Priority**: If the connection matched a ruleset (e.g., `geosite-netflix`, `geosite-openai`), use the ruleset tag for hashing
2. **Domain Fallback**: If no ruleset matched, extract the eTLD+1 (top domain) using Public Suffix List

**Behavior Examples:**

| Scenario | Input | Hash Key Part | Description |
|----------|-------|---------------|-------------|
| Ruleset matched | `api.netflix.com` matched `geosite-netflix` | `geosite-netflix` | Uses ruleset tag |
| No match, valid domain | `cdn.example.com` (no match) | `example.com` | Extracts eTLD+1 |
| No match, subdomain | `a.b.example.com` (no match) | `example.com` | Groups by top domain |
| Multi-part TLD | `www.example.co.uk` (no match) | `example.co.uk` | PSL-aware extraction |
| IP address | `8.8.8.8` (no match) | `-` | Placeholder for IPs |

**Full Example:**

With configuration `["src_ip", "matched_ruleset_or_etld"]`:

- Request from `192.168.1.100` to `api.netflix.com` with ruleset `geosite-netflix` → hash key: `192.168.1.100|geosite-netflix`
- Request from `192.168.1.100` to `cdn1.example.com` (no ruleset) → hash key: `192.168.1.100|example.com`
- Request from `192.168.1.100` to `cdn2.example.com` (no ruleset) → hash key: `192.168.1.100|example.com` (same as above)

**Benefits:**

- **Unified configuration**: Single hash mode handles both rule-based and direct traffic
- **Content category routing**: Rule-matched traffic routes by category (e.g., all Netflix traffic to same outbound)
- **Domain grouping**: Non-matched traffic routes by top domain (e.g., all example.com subdomains together)
- **Sticky sessions**: Same source + category/domain always uses same outbound

**Use Cases:**

- CDN optimization: Route all subdomains of a CDN to the same outbound
- Service isolation: Keep video streaming, API calls, and CDN traffic consistent per source
- Mixed routing: Handle both categorized (via rulesets) and uncategorized traffic intelligently

---

## Configuration Examples

### Example 1: Simple Round-Robin with Health Checking

```json
{
  "type": "loadbalance",
  "tag": "lb-simple",
  "primary_outbounds": [
    "proxy-1",
    "proxy-2",
    "proxy-3"
  ],
  "url": "https://www.gstatic.com/generate_204",
  "interval": "3m",
  "strategy": "random"
}
```

Randomly distributes connections across all healthy proxies.

### Example 2: Top-N Selection with Failover

```json
{
  "type": "loadbalance",
  "tag": "lb-topn",
  "primary_outbounds": [
    "proxy-1",
    "proxy-2",
    "proxy-3",
    "proxy-4",
    "proxy-5"
  ],
  "backup_outbounds": [
    "proxy-backup-1",
    "proxy-backup-2"
  ],
  "url": "https://www.gstatic.com/generate_204",
  "interval": "2m",
  "timeout": "5s",
  "top_n": {
    "primary": 3,
    "backup": 1
  },
  "strategy": "random",
  "hysteresis": {
    "primary_failures": 3,
    "backup_hold_time": "5m"
  }
}
```

Uses the 3 fastest primary proxies. Falls back to the fastest backup proxy after 3 consecutive failures, and stays on backup for at least 5 minutes.

### Example 3: Per-Source IP Consistent Hashing

```json
{
  "type": "loadbalance",
  "tag": "lb-per-ip",
  "primary_outbounds": [
    "proxy-1",
    "proxy-2",
    "proxy-3",
    "proxy-4"
  ],
  "url": "https://www.gstatic.com/generate_204",
  "interval": "3m",
  "strategy": "consistent_hash",
  "hash": {
    "key_parts": ["src_ip"],
    "virtual_nodes": 100
  }
}
```

Each source IP always routes to the same proxy (sticky sessions per client).

### Example 4: Ruleset-Based Routing

```json
{
  "type": "loadbalance",
  "tag": "lb-by-category",
  "primary_outbounds": [
    "proxy-1",
    "proxy-2",
    "proxy-3",
    "proxy-4"
  ],
  "url": "https://www.gstatic.com/generate_204",
  "interval": "3m",
  "strategy": "consistent_hash",
  "hash": {
    "key_parts": ["src_ip", "matched_ruleset"],
    "virtual_nodes": 100
  }
}
```

Routes by source IP + ruleset category. Same source accessing same category (e.g., Netflix) always uses same proxy.

**Requires route configuration:**

```json
{
  "route": {
    "rule_set": [
      {
        "tag": "geosite-netflix",
        "type": "remote",
        "format": "binary",
        "url": "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-netflix.srs"
      }
    ],
    "rules": [
      {
        "rule_set": ["geosite-netflix"],
        "outbound": "lb-by-category"
      }
    ]
  }
}
```

### Example 5: Top Domain Routing

```json
{
  "type": "loadbalance",
  "tag": "lb-by-domain",
  "primary_outbounds": [
    "proxy-1",
    "proxy-2",
    "proxy-3",
    "proxy-4"
  ],
  "url": "https://www.gstatic.com/generate_204",
  "interval": "3m",
  "strategy": "consistent_hash",
  "hash": {
    "key_parts": ["src_ip", "etld_plus_one"],
    "virtual_nodes": 100
  }
}
```

Routes by source IP + top domain. All subdomains of example.com from the same source use the same proxy.

**Examples:**
- `192.168.1.100` → `api.example.com` and `cdn.example.com` → same proxy
- `192.168.1.100` → `example.com` and `example.co.uk` → different proxies

### Example 6: Smart Routing with Ruleset Fallback

```json
{
  "type": "loadbalance",
  "tag": "lb-smart",
  "primary_outbounds": [
    "proxy-1",
    "proxy-2",
    "proxy-3",
    "proxy-4"
  ],
  "url": "https://www.gstatic.com/generate_204",
  "interval": "3m",
  "top_n": {
    "primary": 3
  },
  "strategy": "consistent_hash",
  "hash": {
    "key_parts": ["src_ip", "matched_ruleset_or_etld"],
    "virtual_nodes": 100,
    "on_empty_key": "random"
  }
}
```

**Smart routing behavior:**

- **Categorized traffic** (matched by rulesets): Routes by category
  - Netflix traffic → `192.168.1.100|geosite-netflix`
  - Google traffic → `192.168.1.100|geosite-google`

- **Uncategorized traffic** (no ruleset match): Routes by top domain
  - `cdn1.example.com` → `192.168.1.100|example.com`
  - `cdn2.example.com` → `192.168.1.100|example.com` (same as above)
  - `api.other.com` → `192.168.1.100|other.com` (different)

**Benefits:**
- Handles both rule-based and direct traffic intelligently
- No need for separate LoadBalance instances
- Consistent routing for both categorized and uncategorized content

---

## Best Practices

### Health Checking

- **Interval**: Use `3m` for most scenarios, `1m` for high-availability setups
- **Timeout**: Set to `5s` or less to detect failures quickly
- **URL**: Use a lightweight endpoint like `/generate_204` or health check endpoints

### Top-N Selection

- **Primary count**: Choose 50-70% of total primary outbounds for balance between redundancy and performance
- **Backup count**: Usually 1-2 backups are sufficient
- **Example**: With 6 primary proxies, use `top_n.primary: 4`

### Consistent Hashing

- **Virtual nodes**: 100 is a good default. Increase to 200 for better distribution if you have many outbounds
- **Key parts**: Choose based on your routing needs:
  - Session persistence per client: `["src_ip"]`
  - Application-level routing: `["src_ip", "matched_ruleset"]`
  - Domain grouping: `["src_ip", "etld_plus_one"]`
  - Smart hybrid: `["src_ip", "matched_ruleset_or_etld"]`

### Hysteresis Configuration

- **Primary failures**: Set to `2-3` to avoid flapping during transient failures
- **Backup hold time**: Set to `3m-10m` to give primary time to recover
- **Example**: `primary_failures: 3` + `backup_hold_time: 5m` works well for most cases

### Failover Strategy

- **With backups**: Use `top_n` to keep best primaries active, fall back to backups when needed
- **Without backups**: Use `empty_pool_action: "direct"` for graceful degradation
- **High availability**: Combine `top_n`, `hysteresis`, and backup pools

---

## Technical Details

### Hash Key Construction

Hash keys are constructed by joining key parts with `|` separator:

- Missing or empty values use `-` placeholder
- Optional salt prefix: `{key_salt}{part1}|{part2}|{part3}`
- Hash function: xxHash (fast, high-quality distribution)

**Examples:**

| Configuration | Metadata | Hash Key |
|---------------|----------|----------|
| `["src_ip", "dst_port"]` | src=192.168.1.100, dst_port=443 | `192.168.1.100\|443` |
| `["src_ip", "matched_ruleset"]` | src=10.0.0.1, ruleset=geosite-google | `10.0.0.1\|geosite-google` |
| `["src_ip", "etld_plus_one"]` | src=10.0.0.1, domain=api.example.com | `10.0.0.1\|example.com` |
| With salt `"prod-"` | Same as above | `prod-10.0.0.1\|example.com` |

### eTLD+1 Extraction

The `etld_plus_one` and `matched_ruleset_or_etld` key parts use the Public Suffix List (PSL) for accurate domain extraction:

- **PSL-based**: Correctly handles multi-part TLDs like `.co.uk`, `.com.au`, `.gov.uk`
- **Normalization**: Lowercase, strip ports, strip trailing dots
- **IP handling**: IPv4 and IPv6 addresses return `-` placeholder
- **Fallback**: If PSL extraction fails, returns the normalized full domain

**Extraction Examples:**

| Input | eTLD+1 | Notes |
|-------|--------|-------|
| `www.example.com` | `example.com` | Standard domain |
| `api.v2.example.com` | `example.com` | Deep subdomain |
| `example.co.uk` | `example.co.uk` | Multi-part TLD (PSL) |
| `www.example.co.uk` | `example.co.uk` | UK subdomain |
| `example.com:443` | `example.com` | Port stripped |
| `EXAMPLE.COM` | `example.com` | Normalized lowercase |
| `192.168.1.1` | `-` | IPv4 placeholder |
| `2001:db8::1` | `-` | IPv6 placeholder |

### Consistent Hash Ring

- **Algorithm**: NGINX-style consistent hashing with virtual nodes
- **Virtual nodes**: Each outbound gets N positions on the hash ring
- **Minimal remapping**: When outbounds change, only K/N connections remap (K=total connections, N=outbound count)
- **Stability**: Same hash key + same outbound set → always same outbound

### Health Check States

Outbounds transition through states based on health checks:

1. **Healthy**: Responds successfully within timeout
2. **Unhealthy**: Failed health check or timeout exceeded
3. **Recovering**: Was unhealthy, now responding (uses hysteresis if configured)

Top-N selection only considers healthy outbounds, ranked by response latency.

---

## Troubleshooting

### All connections go to one outbound

**Cause**: Hash key has no variance (e.g., only using `dst_port` when all traffic goes to port 443)

**Solution**: Add more dynamic parts like `src_ip` or `domain`

### Connections switch outbounds frequently

**Cause 1**: Using `random` strategy
**Solution**: Use `consistent_hash` for sticky sessions

**Cause 2**: Health check flapping
**Solution**: Configure `hysteresis` to dampen rapid switching

### No traffic when primary outbounds fail

**Cause**: No backup outbounds configured and `empty_pool_action` is `reject`

**Solution**: Add `backup_outbounds` or set `empty_pool_action: "direct"`

### Different hash results than expected

**Cause**: Missing metadata fields (e.g., `matched_ruleset` is empty)

**Solution**: Check that routing rules are setting the expected metadata. Enable debug logging to inspect hash keys.
