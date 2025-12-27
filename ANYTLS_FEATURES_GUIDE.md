# AnyTLS Advanced Features Guide

This document provides a comprehensive guide to the three advanced features for AnyTLS protocol session management.

## Table of Contents

- [Features Overview](#features-overview)
- [Feature 1: min_idle_session (Passive Protection)](#feature-1-min_idle_session-passive-protection)
- [Feature 2: ensure_idle_session (Active Maintenance)](#feature-2-ensure_idle_session-active-maintenance)
- [Feature 3: heartbeat (Connection Keepalive)](#feature-3-heartbeat-connection-keepalive)
- [How They Work Together](#how-they-work-together)
- [Configuration Examples](#configuration-examples)
- [Troubleshooting](#troubleshooting)

---

## Features Overview

| Feature | Type | Purpose | Default |
|---------|------|---------|---------|
| `min_idle_session` | **Passive Protection** | Prevents existing sessions from being closed due to timeout | 0 (disabled) |
| `ensure_idle_session` | **Active Creation** | Automatically creates sessions to maintain minimum pool size | 0 (disabled) |
| `heartbeat` | **Connection Keepalive** | Sends periodic packets to prevent NAT timeout | 0s (disabled) |

### Quick Comparison

```
min_idle_session:      [Protection]  "Don't close my sessions!"
ensure_idle_session:   [Creation]    "Always have N sessions ready!"
heartbeat:             [Keepalive]   "Keep my connections alive!"
```

---

## Feature 1: min_idle_session (Passive Protection)

### What It Does

Protects the first `N` idle sessions from being closed due to `idle_session_timeout`.

### How It Works

```
Every idle_session_check_interval:
  1. Find sessions idle > idle_session_timeout
  2. Skip first N sessions (even if they exceed timeout)
  3. Close only the excess sessions
```

### Example

```json
{
  "idle_session_timeout": "5m",
  "min_idle_session": 3
}
```

**Scenario**:
- You have 5 sessions, all idle for 6 minutes
- Result: First 3 are **kept** (timer reset), last 2 are **closed**

### Use Cases

- Prevent premature closure of recently-used sessions
- Maintain a baseline pool after traffic dies down
- Protect session pool from aggressive cleanup

### Configuration Tips

```json
{
  "min_idle_session": 0,   // No protection (default)
  "min_idle_session": 2,   // Protect 2 sessions
  "min_idle_session": 5    // Protect 5 sessions
}
```

---

## Feature 2: ensure_idle_session (Active Maintenance)

### What It Does

Automatically creates new sessions when the idle pool drops below `N` sessions.

### How It Works

```
Every idle_session_check_interval:
  1. Count current idle sessions
  2. If count < ensure_idle_session:
     → Create (ensure_idle_session - count) new sessions
  3. Add created sessions to idle pool
```

### Example

```json
{
  "ensure_idle_session": 5
}
```

**Scenario**:
- Current idle sessions: 2
- Target: 5
- Result: Automatically creates **3 new sessions**

### Use Cases

- **Pre-warm connections** for low-latency applications
- **High availability** - always have ready sessions
- **Auto-recovery** from network disruptions
- **NAT traversal** - maintain persistent connections

### Configuration Tips

```json
{
  "ensure_idle_session": 0,    // Disabled (default)
  "ensure_idle_session": 3,    // Low-latency mode
  "ensure_idle_session": 5,    // Balanced mode
  "ensure_idle_session": 10    // High-availability mode
}
```

### Performance Impact

| Pool Size | Memory | Creation Time |
|-----------|--------|---------------|
| 3 sessions | ~30 KB | ~300ms |
| 5 sessions | ~50 KB | ~500ms |
| 10 sessions | ~100 KB | ~1s |

---

## Feature 3: heartbeat (Connection Keepalive)

### What It Does

Sends periodic heartbeat packets to keep sessions alive and prevent NAT timeout.

### How It Works

```
Continuously (for each session):
  Every <heartbeat> interval:
    1. Client sends HeartbeatRequest
    2. Server responds with HeartbeatResponse
    3. NAT sees traffic → resets timeout
```

### Example

```json
{
  "heartbeat": "30s"
}
```

**Result**:
- Every 30 seconds, each session sends a 7-byte heartbeat
- NAT sees activity and keeps the connection alive
- Works for both idle and active sessions

### Use Cases

- **NAT traversal** - keep mappings alive
- **Mobile networks** - prevent carrier timeout (often 30-60s)
- **Firewall persistence** - maintain stateful firewall entries
- **Connection stability** - detect broken connections early

### Configuration Tips

```json
{
  "heartbeat": "0",     // Disabled (default)
  "heartbeat": "10s",   // Aggressive (strict NAT, 30s timeout)
  "heartbeat": "30s",   // Conservative (most NATs, 60s+ timeout)
  "heartbeat": "60s"    // Moderate (stable connections)
}
```

**Best practice**: Set heartbeat to **50-75%** of your NAT timeout

### Overhead

| Interval | Bandwidth per Session | CPU Impact |
|----------|----------------------|------------|
| 10s | ~2.5 KB/hour | Minimal |
| 30s | ~0.8 KB/hour | Negligible |
| 60s | ~0.4 KB/hour | Negligible |

---

## How They Work Together

### Combined Operation Flow

```
┌────────────────────────────────────────────────┐
│  Every 30s (idle_session_check_interval)       │
│                                                 │
│  Phase 1: Cleanup (min_idle_session)           │
│  ┌────────────────────────────────────────┐   │
│  │ • Find sessions idle > 5m              │   │
│  │ • Keep first 2 (min_idle_session)      │   │
│  │ • Close excess                         │   │
│  └────────────────────────────────────────┘   │
│                                                 │
│  Phase 2: Ensure (ensure_idle_session)         │
│  ┌────────────────────────────────────────┐   │
│  │ • Count idle sessions                  │   │
│  │ • If < 5: create new sessions          │   │
│  └────────────────────────────────────────┘   │
└────────────────────────────────────────────────┘

┌────────────────────────────────────────────────┐
│  Continuous (heartbeat)                         │
│                                                 │
│  Every 30s (on each session):                  │
│  • Send HeartbeatRequest                       │
│  • Receive HeartbeatResponse                   │
│  • Keep NAT alive                              │
└────────────────────────────────────────────────┘
```

### Example Timeline

```
Time  | Idle Count | Action
------|------------|----------------------------------
0s    | 0          | ensure: create 5 sessions → 5
30s   | 5          | No action (at target)
60s   | 5          | heartbeat sends on all 5 sessions
2m    | 2          | (3 sessions in use)
5m    | 5          | (sessions returned to pool)
6m    | 5          | cleanup: all > 5m idle
      |            | min: keep first 2
      |            | close: 3 sessions
      |            | Result: 2 idle sessions
6m+1  | 2          | ensure: create 3 new → 5
      | 5          | Pool restored!
```

---

## Configuration Examples

### Example 1: Balanced (Recommended Starting Point)

```json
{
  "idle_session_check_interval": "30s",
  "idle_session_timeout": "5m",
  "min_idle_session": 2,
  "ensure_idle_session": 5,
  "heartbeat": "30s"
}
```

**Good for**: General-purpose usage, moderate traffic

**Benefits**:
- Maintains 5 pre-warmed connections
- Protects 2 from cleanup
- Standard heartbeat works with most NATs
- Balanced resource usage

### Example 2: High Performance (Low Latency)

```json
{
  "idle_session_check_interval": "30s",
  "idle_session_timeout": "10m",
  "min_idle_session": 3,
  "ensure_idle_session": 5,
  "heartbeat": "20s"
}
```

**Good for**: Gaming, real-time trading, low-latency apps

**Benefits**:
- Always have 5 instant connections
- Aggressive heartbeat (20s) prevents any timeout
- Long timeout (10m) avoids cleanup during short idle periods

### Example 3: Mobile Network (NAT Traversal)

```json
{
  "idle_session_check_interval": "30s",
  "idle_session_timeout": "5m",
  "min_idle_session": 2,
  "ensure_idle_session": 3,
  "heartbeat": "10s"
}
```

**Good for**: Mobile devices, unstable networks, strict NATs

**Benefits**:
- Small pool (3) minimizes mobile data
- Aggressive heartbeat (10s) defeats 30s NAT timeout
- Works with carriers having short timeouts

### Example 4: High Availability (Production)

```json
{
  "idle_session_check_interval": "1m",
  "idle_session_timeout": "15m",
  "min_idle_session": 5,
  "ensure_idle_session": 10,
  "heartbeat": "30s"
}
```

**Good for**: Production services, 24/7 uptime, critical apps

**Benefits**:
- Large pool (10) handles burst traffic
- Protected minimum (5) ensures baseline availability
- Long timeout prevents premature cleanup
- Moderate heartbeat balances keepalive vs overhead

### Example 5: Resource Efficient (IoT)

```json
{
  "idle_session_check_interval": "1m",
  "idle_session_timeout": "2m",
  "min_idle_session": 1,
  "ensure_idle_session": 1,
  "heartbeat": "45s"
}
```

**Good for**: Resource-constrained devices, IoT, edge computing

**Benefits**:
- Minimal pool (1 session) saves memory
- Aggressive timeout (2m) releases resources quickly
- Still maintains single ready connection

---

## Troubleshooting

### Problem 1: Sessions Keep Getting Closed

**Symptoms**:
- Frequent TLS handshakes
- High latency on first request
- Log shows many session closures

**Solutions**:

```json
// Increase protected sessions
{
  "min_idle_session": 5
}

// OR increase timeout
{
  "idle_session_timeout": "10m"
}

// OR maintain larger pool
{
  "ensure_idle_session": 10
}
```

### Problem 2: Not Enough Sessions During Bursts

**Symptoms**:
- Slow response during traffic spikes
- Multiple TLS handshakes simultaneously
- Pool exhausted logs

**Solution**:

```json
{
  "ensure_idle_session": 10  // Larger pool
}
```

### Problem 3: Connections Drop Through NAT

**Symptoms**:
- Periodic connection failures
- "Connection reset by peer" errors
- Sessions work initially but fail after idle period

**Solutions**:

```json
// Reduce heartbeat interval
{
  "heartbeat": "10s"  // More aggressive
}

// OR ensure pool maintenance
{
  "ensure_idle_session": 5  // Auto-recreate dead sessions
}
```

### Problem 4: High Memory Usage

**Symptoms**:
- Memory consumption grows over time
- OOM errors on resource-constrained devices

**Solution**:

```json
{
  "ensure_idle_session": 2,      // Smaller pool
  "idle_session_timeout": "2m",  // Aggressive cleanup
  "heartbeat": "60s"             // Less frequent
}
```

### Problem 5: High Bandwidth Usage

**Symptoms**:
- Unexpected data usage
- Bandwidth monitoring shows constant traffic

**Solution**:

```json
{
  "heartbeat": "60s"  // OR "0" to disable
}
```

**Note**: Heartbeat uses ~7 bytes per beat. Even at 10s interval, that's only ~2.5 KB/hour per session.

---

## Debug Logging

Enable debug logging to monitor all three features:

```json
{
  "log": {
    "level": "debug"
  }
}
```

**Expected logs**:

```
// Heartbeat
[DEBUG] [Heartbeat] Sending heartbeat request
[DEBUG] [Heartbeat] Heartbeat sent successfully
[DEBUG] [Heartbeat] Received heartbeat response

// Pool Maintenance
[DEBUG] [EnsureIdleSession] Current idle sessions: 3, target: 5, creating 2 new sessions
[DEBUG] [EnsureIdleSession] Successfully created and pooled session #1 (seq=42)
[DEBUG] [EnsureIdleSession] Successfully created and pooled session #2 (seq=43)
```

---

## Summary

### Feature Relationship

```
┌──────────────────────────────────────────────┐
│             Session Lifecycle                 │
│                                               │
│  Creation ──► ensure_idle_session            │
│     │                                         │
│     ▼                                         │
│  Active   ──► heartbeat (keepalive)          │
│     │                                         │
│     ▼                                         │
│  Idle     ──► heartbeat (keepalive)          │
│     │                                         │
│     ▼                                         │
│  Timeout  ──► min_idle_session (protection)  │
│     │                                         │
│     ▼                                         │
│  Cleanup  ──► ensure_idle_session (recreate) │
│                                               │
└──────────────────────────────────────────────┘
```

### When to Use Each Feature

| Scenario | min_idle | ensure_idle | heartbeat |
|----------|----------|-------------|-----------|
| General use | ✅ 2-3 | ✅ 5 | ✅ 30s |
| Low latency | ✅ 3-5 | ✅ 5-10 | ✅ 10-20s |
| Mobile/NAT | ✅ 1-2 | ✅ 2-3 | ✅ 10s |
| Production | ✅ 5-10 | ✅ 10-20 | ✅ 30s |
| IoT/Edge | ✅ 1 | ✅ 1-2 | ✅ 45-60s |
| No resources | ❌ 0 | ❌ 0 | ❌ 0 or 60s |

### Key Takeaways

1. **min_idle_session** = Safety net (prevents loss)
2. **ensure_idle_session** = Performance boost (pre-warming)
3. **heartbeat** = Reliability (keeps alive)

4. All three work together for optimal performance
5. Start with balanced config, then tune based on needs
6. Monitor with debug logs during initial deployment
7. Adjust based on your specific NAT/network conditions

---

For more information, see:
- [HEARTBEAT_IMPLEMENTATION.md](HEARTBEAT_IMPLEMENTATION.md) - Heartbeat details
- [ENSURE_IDLE_SESSION_IMPLEMENTATION.md](ENSURE_IDLE_SESSION_IMPLEMENTATION.md) - Ensure pool details
- [docs/configuration/outbound/anytls.md](docs/configuration/outbound/anytls.md) - Full documentation
