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
  "ensure_idle_session": 5,
  "heartbeat": "30s",
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

In the check, at least the first `n` idle sessions are kept open. Default value: `n`=0

**Note**: This is a **passive protection** mechanism - it only prevents existing sessions from being closed due to timeout.

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

#### tls

==Required==

TLS configuration, see [TLS](/configuration/shared/tls/#outbound).

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.

---

## Session Pool Management Guide

AnyTLS provides three complementary features for managing connection sessions:

| Feature | Type | Purpose |
|---------|------|---------|
| `min_idle_session` | **Passive Protection** | Prevents existing sessions from timeout closure |
| `ensure_idle_session` | **Active Creation** | Maintains minimum pool size by creating sessions |
| `heartbeat` | **Keepalive** | Keeps sessions alive through NAT and prevents timeouts |

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
