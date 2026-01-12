---
icon: material/scale-balance
---

!!! question "Since sing-box 1.12.15"

### Structure

```json
{
  "type": "loadbalance",
  "tag": "load-balance",

  "primary_outbounds": [
    "proxy-a",
    "proxy-b",
    "proxy-c"
  ],
  "backup_outbounds": [
    "proxy-backup-1",
    "proxy-backup-2"
  ],
  "strategy": "random",
  "url": "",
  "interval": "",
  "timeout": "",
  "idle_timeout": "",
  "top_n": {
    "primary": 0,
    "backup": 0
  },
  "hash": {
    "key_parts": [],
    "virtual_nodes": 10,
    "on_empty_key": "random",
    "salt": ""
  },
  "hysteresis": {
    "primary_failures": 3,
    "backup_hold_time": "30s"
  },
  "empty_pool_action": "error",
  "interrupt_exist_connections": false
}
```

### Fields

#### primary_outbounds

==Required==

List of primary tier outbound tags. These outbounds are used during normal operation.

#### backup_outbounds

List of backup tier outbound tags. These outbounds are only used when all primary outbounds fail.

Follows HAProxy backup semantics: backups activate only when ALL primaries are unavailable.

#### strategy

Load balancing strategy. Available options:
- `random`: Uniform random distribution (default)
- `consistent_hash`: Hash-based routing with session affinity

Default: `random`

#### url

The URL to test for health checks. `https://www.gstatic.com/generate_204` will be used if empty.

#### interval

The health check interval. `3m` will be used if empty.

#### timeout

The health check timeout. `5s` will be used if empty.

#### idle_timeout

The idle timeout for connections. `30m` will be used if empty.

#### top_n

Top-N selection configuration for each tier.

##### top_n.primary

Number of fastest outbounds to select from primary tier. `0` means use all available. Default: `0`

##### top_n.backup

Number of fastest outbounds to select from backup tier. `0` means use all available. Default: `0`

#### hash

Consistent hash configuration. Only applies when `strategy` is `consistent_hash`.

##### hash.key_parts

Array of metadata parts to build the hash key. Available parts:

- `src_ip`: Source IP address
- `dst_ip`: Destination IP address
- `dst_port`: Destination port number
- `network`: Network type (tcp/udp)
- `domain`: Full destination domain
- `inbound_tag`: Inbound connection tag
- `matched_ruleset`: Matched ruleset tag (for rule-based routing)
- `etld_plus_one`: Top-level domain (e.g., "example.com" from "www.example.com")
- `matched_ruleset_or_etld`: Smart fallback (uses ruleset if matched, else eTLD+1)
- `salt`: Custom salt string

Example: `["src_ip", "dst_port"]` creates keys like "192.168.1.100|443"

##### hash.virtual_nodes

Number of virtual nodes per outbound for better distribution. Default: `10`

Higher values provide more even distribution but use more memory.

##### hash.on_empty_key

Behavior when hash key is empty:
- `random`: Select randomly (default)
- `hash_empty`: Use empty string as the key

Default: `random`

##### hash.salt

Custom salt string to add to hash keys for additional randomization.

#### hysteresis

Hysteresis configuration to prevent tier flapping.

##### hysteresis.primary_failures

Number of consecutive failures required before switching to backup tier. Default: `3`

##### hysteresis.backup_hold_time

Minimum time to stay in backup tier before switching back to primary. Default: `30s`

Prevents rapid back-and-forth switching when primaries become intermittently available.

#### empty_pool_action

Behavior when no candidates are available:
- `error`: Return error (fail closed, default)
- `fallback_all`: Use all configured outbounds (fail open)

Default: `error`

#### interrupt_exist_connections

Interrupt existing connections when the selected candidates change.

Only inbound connections are affected by this setting, internal connections will always be interrupted.

---

## Features

### Two-Tier System

LoadBalance uses a primary/backup tier system similar to HAProxy:

- **Primary tier**: Used during normal operation
- **Backup tier**: Activated only when ALL primary outbounds are unavailable

This provides automatic failover without manual intervention.

### Top-N Selection

Select the N fastest outbounds from each tier based on health check latency:

```json
{
  "top_n": {
    "primary": 3,
    "backup": 2
  }
}
```

This selects the 3 fastest primaries and 2 fastest backups, improving performance by using only the best-performing outbounds.

### Load Balancing Strategies

#### Random Strategy

Distributes connections uniformly across available candidates.

- Simple and fast
- No session affinity
- Good for stateless protocols

#### Consistent Hash Strategy

Provides session affinity - same input always routes to the same outbound:
- Uses virtual nodes for even distribution
- Minimal remapping when pool changes (~25% for 25% node removal)
- Configurable hash key from connection metadata

### Hash-Based Routing

With `consistent_hash` strategy, you can route traffic based on various metadata:

**Per-source IP routing:**
```json
"hash": {
  "key_parts": ["src_ip"]
}
```

**Per-destination routing:**
```json
"hash": {
  "key_parts": ["dst_ip", "dst_port"]
}
```

**Per-domain routing:**
```json
"hash": {
  "key_parts": ["etld_plus_one"]
}
```

**Smart routing (ruleset with domain fallback):**
```json
"hash": {
  "key_parts": ["src_ip", "matched_ruleset_or_etld"]
}
```

This routes traffic by ruleset when rules match, but falls back to per-domain routing for unmatched traffic.

### Bootstrap Mode

LoadBalance starts immediately with all primary outbounds, preventing "no candidates available" errors during startup. After the first health check completes, it switches to health-based selection.

### Network Filtering

Automatically filters candidates by network capability:
- TCP connections only use TCP-capable outbounds
- UDP connections only use UDP-capable outbounds

### Clash API Compatibility

Implements the `URLTest` method for Clash API compatibility, enabling the `/proxies/:name/delay` endpoint for manual health checks.

---

## Configuration Examples

### Example 1: Simple Random Load Balancing

```json
{
  "outbounds": [
    {
      "type": "loadbalance",
      "tag": "load-balance",
      "primary_outbounds": ["proxy-1", "proxy-2", "proxy-3"],
      "strategy": "random"
    }
  ]
}
```

### Example 2: Consistent Hash with Source IP

Ensures the same source IP always uses the same outbound.

```json
{
  "type": "loadbalance",
  "tag": "load-balance",
  "primary_outbounds": ["proxy-a", "proxy-b", "proxy-c"],
  "strategy": "consistent_hash",
  "hash": {
    "key_parts": ["src_ip"],
    "virtual_nodes": 50
  }
}
```

### Example 3: Primary/Backup with Top-N

```json
{
  "type": "loadbalance",
  "tag": "load-balance",
  "primary_outbounds": ["proxy-1", "proxy-2", "proxy-3", "proxy-4", "proxy-5"],
  "backup_outbounds": ["backup-1", "backup-2"],
  "strategy": "random",
  "top_n": {
    "primary": 3,
    "backup": 1
  },
  "hysteresis": {
    "primary_failures": 3,
    "backup_hold_time": "30s"
  }
}
```

This configuration:
- Selects the 3 fastest outbounds from the primary tier
- Falls back to the fastest backup when all primaries fail 3 consecutive times
- Stays in backup mode for at least 30 seconds before switching back

### Consistent Hash Routing

```json
{
  "type": "loadbalance",
  "tag": "hash-balance",

  "primary_outbounds": ["proxy-a", "proxy-b", "proxy-c"],
  "strategy": "consistent_hash",
  "hash": {
    "key_parts": ["src_ip", "dst_port"],
    "virtual_nodes": 50,
    "on_empty_key": "random"
  }
}
```

This provides session affinity: connections from the same source IP to the same destination port will always use the same outbound (unless it becomes unavailable).

### Rule-Based Hash Routing

```json
{
  "type": "loadbalance",
  "tag": "rule-hash",

  "primary_outbounds": ["proxy-streaming", "proxy-gaming", "proxy-general"],
  "strategy": "consistent_hash",
  "hash": {
    "key_parts": ["src_ip", "matched_ruleset"],
    "virtual_nodes": 30
  }
}
```

Combined with routing rules, this allows different service types to stick to specific outbounds per user.

### Smart Fallback Routing

```json
{
  "type": "loadbalance",
  "tag": "smart-balance",

  "primary_outbounds": ["proxy-1", "proxy-2", "proxy-3"],
  "strategy": "consistent_hash",
  "hash": {
    "key_parts": ["src_ip", "matched_ruleset_or_etld"],
    "virtual_nodes": 40
  }
}
```

The `matched_ruleset_or_etld` part provides smart fallback:
- For connections matching a ruleset: use the ruleset tag for hashing
- For other connections: use the eTLD+1 domain for hashing

This provides unified configuration for both rule-based and direct connections while maintaining session stickiness.

### Domain-Based Routing

```json
{
  "type": "loadbalance",
  "tag": "domain-balance",

  "primary_outbounds": ["proxy-a", "proxy-b", "proxy-c"],
  "strategy": "consistent_hash",
  "hash": {
    "key_parts": ["src_ip", "etld_plus_one"],
    "virtual_nodes": 50
  }
}
```

Connections to the same top-level domain (e.g., all *.example.com) from the same user will use the same outbound. This is useful for maintaining session state with CDNs and multi-domain services.

---

## Advanced Topics

### Bootstrap Mode

LoadBalance supports immediate connection handling before the first health check completes. On startup:
1. All primary outbounds are available for selection
2. Selection uses the configured strategy (random or consistent_hash)
3. After the first health check, selection switches to health-based candidates

This prevents "no candidates available" errors during startup.

### Network Filtering

LoadBalance automatically filters candidates based on connection type:
- TCP connections: only uses outbounds that support TCP
- UDP connections: only uses outbounds that support UDP

### Clash API Compatibility

LoadBalance implements the `URLTest` method for Clash API compatibility, enabling:
- Manual health checks via `/proxies/:name/delay` endpoint
- Real-time latency monitoring
- Integration with Clash-compatible dashboards

### Performance Considerations

- **Virtual Nodes**: More virtual nodes (50-100) provide better distribution but use more memory
- **Top-N Selection**: Limiting to top 3-5 outbounds reduces overhead while maintaining good performance
- **Check Interval**: Longer intervals (5m-10m) reduce load on destination servers
- **Hysteresis**: Prevents excessive switching; adjust based on your network stability
