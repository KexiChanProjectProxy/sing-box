---
icon: material/new-box
---

!!! question "Since sing-box 1.12.0"

### Structure

```json
{
  "type": "anytls",
  "tag": "anytls-out",

  "server": "127.0.0.1",
  "server_port": 1080,
  "password": "8JCsPssfgS8tiRwiMlhARg==",
  "idle_session_check_interval": "30s",
  "idle_session_timeout": "5m",
  "min_idle_session": 2,
  "min_idle_session_for_age": 1,
  "ensure_idle_session": 5,
  "heartbeat": "30s",
  "max_connection_lifetime": "1h",
  "connection_lifetime_jitter": "10m",
  "tls": {},

  ... // Dial Fields
}
```

### Fields

#### server

==Required==

The server address.

#### server_port

==Required==

The server port.

#### password

==Required==

The AnyTLS password.

#### idle_session_check_interval

Interval checking for idle sessions. Default: 30s.

#### idle_session_timeout

In the check, close sessions that have been idle for longer than this. Default: 30s.

#### min_idle_session

In the idle timeout check, at least the first `n` idle sessions are kept open. Default value: `n`=0

**Note**: This is a **passive protection** mechanism - it only prevents existing sessions from being closed due to **idle timeout**.

**Purpose**: Protects sessions from `idle_session_timeout` cleanup.

#### min_idle_session_for_age

In the age-based cleanup, at least `n` idle sessions are kept open regardless of their age. Default value: `n`=0

**Note**: This is a **separate protection** for age-based cleanup - independent from `min_idle_session`.

**Purpose**: Protects sessions from `max_connection_lifetime` cleanup.

**Difference from min_idle_session**:
- `min_idle_session`: Protects from idle timeout (5m of inactivity)
- `min_idle_session_for_age`: Protects from age expiration (1h connection lifetime)

**Why separate**:
- Different scenarios require different protection levels
- You might want aggressive age rotation (low min) but generous idle protection (high min)
- Or vice versa: keep connections alive long-term (high age min) but quickly close inactive ones (low idle min)

**Example scenarios**:

Scenario 1: **Aggressive rotation, generous idle protection**
```json
{
  "min_idle_session": 5,           // Keep 5 sessions from idle timeout
  "min_idle_session_for_age": 1,   // But allow aggressive age rotation (only keep 1)
  "max_connection_lifetime": "30m",
  "idle_session_timeout": "10m"
}
```
Use case: You want to quickly rotate connections for security, but don't want to close sessions just because they're temporarily idle.

Scenario 2: **Conservative rotation, aggressive idle cleanup**
```json
{
  "min_idle_session": 1,           // Aggressively close idle sessions
  "min_idle_session_for_age": 5,   // But keep connections alive long-term
  "max_connection_lifetime": "2h",
  "idle_session_timeout": "2m"
}
```
Use case: You want to maintain long-lived connections but quickly free resources from truly idle sessions.

Scenario 3: **Balanced**
```json
{
  "min_idle_session": 3,
  "min_idle_session_for_age": 2,   // Slightly more aggressive on age
  "max_connection_lifetime": "1h",
  "idle_session_timeout": "5m"
}
```

#### ensure_idle_session

Proactively maintains at least `n` idle sessions in the pool. If the current idle session count is below this value, new sessions will be automatically created. Default value: `n`=0 (disabled)

**Note**: This is an **active maintenance** mechanism - it creates new sessions to maintain the pool size.

**Comparison**:
- `min_idle_session`: Protects existing sessions from timeout closure
- `ensure_idle_session`: Creates new sessions to reach target pool size

**Example**:
```json
{
  "min_idle_session": 3,
  "ensure_idle_session": 10,
  "idle_session_timeout": "5m"
}
```

With this configuration:
- The pool will always try to maintain 10 idle sessions
- Even if sessions are idle for >5 minutes, the first 3 won't be closed
- If pool drops below 10 (e.g., after cleanup or network issues), new sessions are created automatically

**Use cases**:
- **High-traffic scenarios**: Pre-warm connections to reduce first-request latency
- **NAT keepalive**: Maintain persistent connections through NAT devices
- **Reliability**: Always have ready-to-use sessions available

#### heartbeat

Sends periodic heartbeat packets to keep connections alive and prevent NAT session timeouts. Default value: disabled (0s)

The heartbeat operates at the **session level** (the underlying TCP+TLS connection), not per-stream. All heartbeat traffic is encrypted within TLS and indistinguishable from regular data.

**How it works**:
- Client sends `HeartbeatRequest` frames at the configured interval
- Server automatically responds with `HeartbeatResponse`
- Keeps NAT mappings alive by sending periodic traffic
- Works for both idle sessions (in pool) and active sessions

**Recommended values**:
```json
{
  "heartbeat": "30s"   // Conservative - works with most NATs (60s+ timeout)
  "heartbeat": "10s"   // Aggressive - for strict NATs (30s timeout)
  "heartbeat": "60s"   // Moderate - for stable connections with long timeouts
}
```

**Best practices**:
- Set heartbeat interval **less than** your NAT timeout (e.g., 30s for 60s NAT timeout)
- For mobile networks: use 10-30s (carriers often have short timeouts)
- For stable connections: use 30-60s
- Disable (`"heartbeat": "0"`) if not needed (saves bandwidth)

**Overhead**:
- ~7 bytes per heartbeat frame
- Minimal CPU impact (timer-based)
- Example: `30s` interval = ~1.8 KB/hour per session

**Debug logging**:
Set `"log": {"level": "debug"}` to see heartbeat activity:
```
[Heartbeat] Sending heartbeat request
[Heartbeat] Heartbeat sent successfully
[Heartbeat] Received heartbeat response
```

#### max_connection_lifetime

Maximum lifetime for idle connections. When enabled, idle sessions that exceed this age will be automatically closed, with older connections prioritized for closure. Default value: disabled (0s)

**How it works**:
- Tracks the creation time of each session
- Periodically checks idle sessions for age
- Closes idle sessions older than the configured lifetime
- Prefers newer connections for use (FIFO - First In, First Out)
- Prefers older connections for closure

**Recommended values**:
```json
{
  "max_connection_lifetime": "1h"   // Conservative - rotate connections hourly
  "max_connection_lifetime": "30m"  // Moderate - rotate every 30 minutes
  "max_connection_lifetime": "2h"   // Long-lived - for stable environments
}
```

**Best practices**:
- Set based on your network environment and security requirements
- Lower values (30m-1h) for environments with dynamic IPs or security concerns
- Higher values (2h-4h) for stable, trusted networks
- Disable (`"max_connection_lifetime": "0"`) if not needed
- Combine with `ensure_idle_session` to maintain pool size after rotation

**Use cases**:
- **Connection rotation**: Regularly refresh connections to prevent stale sessions
- **Load balancing**: Distribute connections across different server resources over time
- **Security**: Limit connection lifetime to reduce exposure window
- **IP rotation**: Work with dynamic IP environments by cycling connections

**Example with pool maintenance**:
```json
{
  "max_connection_lifetime": "1h",
  "ensure_idle_session": 5,
  "min_idle_session": 2
}
```
This configuration:
- Closes idle connections older than 1 hour
- Automatically creates new sessions to maintain 5 idle sessions
- Protects at least 2 sessions from timeout-based cleanup
- Results in automatic connection rotation every hour

**Important notes**:
- Age-based cleanup respects `min_idle_session_for_age` - will not close sessions if it would drop below this minimum
- Works seamlessly with `ensure_idle_session` for automatic rotation
- Oldest connections are closed first during cleanup
- Independent from `min_idle_session` (which protects from idle timeout)

**Debug logging**:
Set `"log": {"level": "debug"}` to see age cleanup activity:
```
[AgeCleanup] Found 5 expired sessions, closing 3 oldest (keeping 2 to maintain min_idle_session_for_age=2)
[AgeCleanup] Closing session #1 (seq=42, age=1h5m30s, maxLife=55m0s, created=2024-01-01 10:00:00)
```

#### connection_lifetime_jitter

Randomization range for connection lifetime. When set, each connection gets a random lifetime of `max_connection_lifetime ± jitter`. Default value: disabled (0s)

**How it works**:
- Prevents "thundering herd" problem where all connections expire simultaneously
- Each session gets a unique randomized lifetime on creation
- Lifetime = `max_connection_lifetime` + random(-jitter, +jitter)
- Distributes connection closures over time
- Reduces load spikes from mass reconnection

**Recommended values**:
```json
{
  "max_connection_lifetime": "1h",
  "connection_lifetime_jitter": "10m"   // Connections expire between 50m-70m
}
```

```json
{
  "max_connection_lifetime": "30m",
  "connection_lifetime_jitter": "5m"    // Connections expire between 25m-35m
}
```

**Best practices**:
- Set jitter to 10-30% of max_connection_lifetime
- Larger jitter = more spread out expiration times
- Smaller jitter = more predictable expiration windows
- Use with `ensure_idle_session` to maintain pool size smoothly

**Benefits**:
- **Avoid thundering herd**: Connections don't all expire at once
- **Smooth rotation**: Gradual connection replacement over time
- **Load distribution**: Reconnection load spread across time window
- **Better stability**: No sudden spikes in new connections

**Example with full configuration**:
```json
{
  "max_connection_lifetime": "1h",
  "connection_lifetime_jitter": "15m",
  "ensure_idle_session": 10,
  "min_idle_session": 5
}
```
This configuration:
- Connections expire between 45-75 minutes (1h ± 15m)
- Maintains 10 idle sessions through rotation
- Always keeps at least 5 sessions even if expired
- Spreads reconnections across 30-minute window

**Debug logging**:
```
[AgeCleanup] Closing session #1 (seq=42, age=67m12s, maxLife=62m30s, created=...)
[AgeCleanup] Closing session #2 (seq=38, age=71m45s, maxLife=68m15s, created=...)
```
Notice each session has a different `maxLife` value due to jitter.

#### tls

==Required==

TLS configuration, see [TLS](/configuration/shared/tls/#outbound).

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.

---

## Session Pool Management Guide

AnyTLS provides six complementary features for managing connection sessions:

| Feature | Type | Purpose |
|---------|------|---------|
| `min_idle_session` | **Idle Timeout Protection** | Prevents sessions from idle timeout closure |
| `min_idle_session_for_age` | **Age-based Protection** | Prevents sessions from age-based closure |
| `ensure_idle_session` | **Active Creation** | Maintains minimum pool size by creating sessions |
| `heartbeat` | **Keepalive** | Keeps sessions alive through NAT and prevents timeouts |
| `max_connection_lifetime` | **Age-based Cleanup** | Limits connection lifetime and rotates old connections |
| `connection_lifetime_jitter` | **Rotation Smoothing** | Randomizes lifetime to prevent thundering herd |

### Configuration Strategies

#### Strategy 1: High Performance (Low Latency)

Best for: Gaming, real-time trading, low-latency applications

```json
{
  "type": "anytls",
  "server": "example.com",
  "server_port": 443,
  "password": "your-password",

  "ensure_idle_session": 5,
  "min_idle_session": 3,
  "idle_session_timeout": "10m",
  "idle_session_check_interval": "30s",
  "heartbeat": "20s",

  "tls": {
    "enabled": true,
    "server_name": "example.com"
  }
}
```

**Benefits**:
- Always have 5 pre-warmed connections ready
- First request is instant (no TLS handshake)
- Aggressive heartbeat prevents any timeouts
- Protected minimum ensures baseline availability

#### Strategy 2: High Availability (Production)

Best for: Production services, critical applications, 24/7 uptime

```json
{
  "type": "anytls",
  "server": "example.com",
  "server_port": 443,
  "password": "your-password",

  "ensure_idle_session": 10,
  "min_idle_session": 5,
  "idle_session_timeout": "15m",
  "idle_session_check_interval": "1m",
  "heartbeat": "30s",

  "tls": {
    "enabled": true,
    "server_name": "example.com"
  }
}
```

**Benefits**:
- Large pool handles burst traffic
- Protected minimum prevents over-aggressive cleanup
- Moderate heartbeat balances keepalive vs overhead
- Auto-recovery from network disruptions

#### Strategy 3: NAT Traversal (Mobile Networks)

Best for: Mobile devices, unstable networks, strict NATs

```json
{
  "type": "anytls",
  "server": "example.com",
  "server_port": 443,
  "password": "your-password",

  "ensure_idle_session": 3,
  "min_idle_session": 2,
  "idle_session_timeout": "5m",
  "idle_session_check_interval": "30s",
  "heartbeat": "10s",

  "tls": {
    "enabled": true,
    "server_name": "example.com"
  }
}
```

**Benefits**:
- Aggressive heartbeat (10s) keeps NAT sessions alive
- Small pool minimizes mobile data usage
- Quick recovery (30s check interval)
- Works with carriers that have 30s NAT timeout

#### Strategy 4: Resource Efficient (IoT/Edge)

Best for: Resource-constrained devices, minimal overhead, IoT

```json
{
  "type": "anytls",
  "server": "example.com",
  "server_port": 443,
  "password": "your-password",

  "ensure_idle_session": 1,
  "min_idle_session": 1,
  "idle_session_timeout": "2m",
  "idle_session_check_interval": "1m",
  "heartbeat": "45s",

  "tls": {
    "enabled": true,
    "server_name": "example.com"
  }
}
```

**Benefits**:
- Minimal pool (1 session) saves memory
- Aggressive timeout (2m) releases resources quickly
- Still maintains single ready connection
- Heartbeat prevents connection loss

#### Strategy 5: Balanced (General Use)

Best for: General-purpose usage, balanced performance

```json
{
  "type": "anytls",
  "server": "example.com",
  "server_port": 443,
  "password": "your-password",

  "ensure_idle_session": 5,
  "min_idle_session": 2,
  "idle_session_timeout": "5m",
  "idle_session_check_interval": "30s",
  "heartbeat": "30s",

  "tls": {
    "enabled": true,
    "server_name": "example.com"
  }
}
```

**Benefits**:
- Moderate pool size (5) handles normal traffic
- Standard heartbeat (30s) works with most NATs
- Reasonable timeout (5m) balances resources
- Good starting point for most deployments

### Feature Interaction

**How the features work together**:

```
Every 30s (idle_session_check_interval):

  1. Cleanup Phase:
     - Find sessions idle > 5m (idle_session_timeout)
     - Close excess sessions
     - Keep first 2 (min_idle_session) even if idle > 5m

  2. Ensure Phase:
     - Count current idle sessions
     - If count < 5 (ensure_idle_session):
       → Create new sessions to reach 5

Meanwhile (continuous):
  - Heartbeat sends keepalive every 30s (heartbeat)
  - Prevents NAT timeout
  - Works on all sessions (idle and active)
```

### Monitoring and Debugging

Enable debug logging to monitor session pool behavior:

```json
{
  "log": {
    "level": "debug"
  }
}
```

**Expected log messages**:

```
// Session pool maintenance
[DEBUG] [EnsureIdleSession] Current idle sessions: 3, target: 5, creating 2 new sessions
[DEBUG] [EnsureIdleSession] Successfully created and pooled session #1 (seq=42)

// Heartbeat activity
[DEBUG] [Heartbeat] Sending heartbeat request
[DEBUG] [Heartbeat] Heartbeat sent successfully
[DEBUG] [Heartbeat] Received heartbeat response
```

### Troubleshooting

**Problem**: Sessions keep getting closed

**Solution**: Increase `min_idle_session` or `idle_session_timeout`

```json
{
  "min_idle_session": 5,        // Protect more sessions
  "idle_session_timeout": "10m"  // Longer timeout
}
```

**Problem**: Not enough sessions during traffic bursts

**Solution**: Increase `ensure_idle_session`

```json
{
  "ensure_idle_session": 10  // Maintain larger pool
}
```

**Problem**: Connections drop through NAT

**Solution**: Reduce `heartbeat` interval

```json
{
  "heartbeat": "10s"  // More aggressive keepalive
}
```

**Problem**: High memory/bandwidth usage

**Solution**: Reduce pool size and heartbeat frequency

```json
{
  "ensure_idle_session": 2,
  "idle_session_timeout": "2m",
  "heartbeat": "60s"
}
```

### Performance Metrics

| Configuration | Memory | Bandwidth | Latency | Availability |
|--------------|--------|-----------|---------|--------------|
| High Performance | ~50 KB | ~10 KB/h | Minimal | Very High |
| High Availability | ~100 KB | ~20 KB/h | Minimal | Maximum |
| NAT Traversal | ~30 KB | ~30 KB/h | Low | High |
| Resource Efficient | ~10 KB | ~8 KB/h | Low | Medium |
| Balanced | ~50 KB | ~18 KB/h | Low | High |

**Notes**:
- Memory: Per-session overhead ~10 KB (TLS state + buffers)
- Bandwidth: Heartbeat traffic only (7 bytes per beat)
- Latency: Connection establishment time (TLS handshake)
- Availability: Session availability during network issues
