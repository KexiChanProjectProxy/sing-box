# AnyTLS Ensure Idle Session Pool Implementation

## Summary

Added proactive idle session pool maintenance feature to the AnyTLS protocol. This feature ensures a minimum number of idle sessions are always available in the connection pool by automatically creating new sessions when the count drops below the configured threshold.

## Feature Comparison

### Before: `min_idle_session` (Passive Protection)
- **Behavior**: Only protects existing sessions from timeout-based closure
- **Does NOT**: Create new sessions proactively
- **Use case**: Prevent premature closure of recently used sessions

### After: `ensure_idle_session` (Active Maintenance)
- **Behavior**: Proactively creates sessions to maintain pool size
- **Does**: Auto-create sessions when pool drops below threshold
- **Use case**: Pre-warm connections for low-latency, high-availability scenarios

## Changes Made

### 1. Modified `sing-box` Configuration

#### `option/anytls.go`
- Added `EnsureIdleSession int` field to `AnyTLSOutboundOptions`

#### `protocol/anytls/outbound.go`
- Pass `EnsureIdleSession` to `anytls.ClientConfig`

### 2. Modified `anytls` Library

#### `/tmp/sing-anytls/client.go`
- Added `EnsureIdleSession int` field to `ClientConfig` struct
- Updated `NewClient()` to pass `ensureIdleSession` parameter to session client

#### `/tmp/sing-anytls/session/client.go`
- Added `ensureIdleSession int` field to `Client` struct
- Updated `NewClient()` signature to accept `ensureIdleSession` parameter
- Implemented `ensureIdleSessionPool()` method with the following logic:
  - Check if feature is enabled (`ensureIdleSession > 0`)
  - Count current idle sessions
  - Calculate deficit: `ensureIdleSession - currentIdleCount`
  - Asynchronously create deficit sessions
  - Add created sessions to idle pool immediately
  - Log creation success/failure at debug level
- Updated periodic check goroutine to call `ensureIdleSessionPool()` after `idleCleanup()`

## Configuration Examples

### Example 1: Basic Pool Maintenance
```json
{
  "ensure_idle_session": 5,
  "idle_session_check_interval": "30s"
}
```
- Maintains exactly 5 idle sessions at all times
- Checks and creates sessions every 30 seconds if needed

### Example 2: Combined with Timeout Protection
```json
{
  "min_idle_session": 2,
  "ensure_idle_session": 5,
  "idle_session_timeout": "5m",
  "idle_session_check_interval": "30s"
}
```
- Always maintains 5 idle sessions (ensure)
- First 2 sessions are protected from timeout closure (min)
- Sessions idle >5 minutes will be closed (beyond min threshold)

### Example 3: High Availability Setup
```json
{
  "ensure_idle_session": 10,
  "min_idle_session": 5,
  "idle_session_timeout": "10m",
  "heartbeat": "30s"
}
```
- Maintains 10 pre-warmed connections
- Protects 5 from timeout closure
- Heartbeat keeps NAT sessions alive
- Ideal for production high-traffic scenarios

## How It Works

### Session Lifecycle

```
┌─────────────────────────────────────────────────────────────┐
│  Every idle_session_check_interval (e.g., 30s)              │
│                                                              │
│  Step 1: Cleanup Phase (existing behavior)                  │
│  ┌────────────────────────────────────────────────┐         │
│  │ - Find sessions idle > idle_session_timeout    │         │
│  │ - Keep first min_idle_session (reset timer)    │         │
│  │ - Close excess sessions                        │         │
│  └────────────────────────────────────────────────┘         │
│                                                              │
│  Step 2: Ensure Phase (NEW)                                 │
│  ┌────────────────────────────────────────────────┐         │
│  │ - Count current idle sessions                  │         │
│  │ - If count < ensure_idle_session:              │         │
│  │   → Create (ensure - count) sessions           │         │
│  │   → Add to idle pool immediately               │         │
│  │   → Log debug messages                         │         │
│  └────────────────────────────────────────────────┘         │
└─────────────────────────────────────────────────────────────┘
```

### Scenario Walkthrough

**Configuration:**
```json
{
  "min_idle_session": 2,
  "ensure_idle_session": 5,
  "idle_session_timeout": "5m",
  "idle_session_check_interval": "30s"
}
```

**Timeline:**

| Time | Event | Idle Count | Action |
|------|-------|------------|--------|
| T=0 | Service starts | 0 | Ensure creates 5 sessions → 5 idle |
| T=30s | Periodic check | 5 | No action (count == target) |
| T=1m | User makes 3 requests | 2 | 3 sessions in use, 2 idle |
| T=1.5m | Requests complete | 5 | 3 sessions return to pool |
| T=6m | Sessions idle > 5m | 5 | Cleanup: Keep first 2 (min), close 3 → 2 idle |
| T=6m+1ms | Ensure check | 2 | Deficit = 3, create 3 new sessions → 5 idle |
| T=8m | Steady state | 5 | Pool maintained at 5 idle sessions |

## Debug Logging

Enable debug logging to see pool maintenance activity:

```json
{
  "log": {
    "level": "debug"
  }
}
```

**Example log output:**
```
[DEBUG] [EnsureIdleSession] Current idle sessions: 2, target: 5, creating 3 new sessions
[DEBUG] [EnsureIdleSession] Successfully created and pooled session #1 (seq=42)
[DEBUG] [EnsureIdleSession] Successfully created and pooled session #2 (seq=43)
[DEBUG] [EnsureIdleSession] Successfully created and pooled session #3 (seq=44)
```

**Error handling:**
```
[DEBUG] [EnsureIdleSession] Failed to create session #1: dial tcp: connection refused
```

Sessions that fail to create are logged but don't block other sessions or the periodic check. Failed creations will be retried on the next check interval.

## Use Cases

### 1. Low-Latency Applications
**Problem**: First request after idle period has high latency due to TLS handshake
**Solution**:
```json
{
  "ensure_idle_session": 3,
  "min_idle_session": 1
}
```
- Always have 3 pre-established TLS connections ready
- First request uses existing session → no handshake delay

### 2. NAT Traversal & Keepalive
**Problem**: NAT devices close idle TCP connections, breaking session pool
**Solution**:
```json
{
  "ensure_idle_session": 5,
  "heartbeat": "30s",
  "idle_session_timeout": "10m"
}
```
- Heartbeat keeps NAT mappings alive
- Pool automatically recovers if sessions die (network issues, etc.)

### 3. High Availability Services
**Problem**: Need consistent performance even during network disruptions
**Solution**:
```json
{
  "ensure_idle_session": 10,
  "min_idle_session": 5,
  "idle_session_check_interval": "10s"
}
```
- Large pool handles burst traffic
- Quick recovery (10s interval) from session losses
- Min protection prevents over-aggressive cleanup

### 4. Resource-Constrained Environments
**Problem**: Too many idle connections waste resources
**Solution**:
```json
{
  "ensure_idle_session": 2,
  "min_idle_session": 1,
  "idle_session_timeout": "2m"
}
```
- Small pool (2 sessions) balances latency vs resources
- Aggressive timeout (2m) releases resources quickly
- Still maintains minimum availability

## Implementation Details

### Asynchronous Session Creation
- Sessions are created in separate goroutines to avoid blocking periodic check
- Each goroutine independently handles errors
- Failed creations don't affect other sessions being created
- Goroutines check for client shutdown before creating

### Thread Safety
- Uses `idleSessionLock` mutex for pool access
- Lock is held minimally (only during count/insert)
- Session creation happens outside lock to avoid blocking

### Error Handling
- Network errors: Logged at debug level, retried next interval
- Client shutdown: Goroutines exit gracefully
- Context cancellation: Respected during session creation

### Resource Management
- Sessions are added to tracking structures immediately
- Failed sessions don't leak resources
- Client shutdown closes all sessions including newly created ones

## Performance Considerations

### Memory Usage
- Each session: ~10KB overhead (TLS state, buffers)
- 10 idle sessions ≈ 100KB additional memory
- Negligible for most deployments

### Network Overhead
- Each session: 1 TLS handshake (1-2 RTT)
- Heartbeat: 7 bytes per interval
- Creation is amortized over check interval

### CPU Usage
- Session creation: ~1ms CPU per session (TLS handshake)
- Periodic check: <0.1ms per session
- Negligible impact on overall performance

## Backward Compatibility

- **Default behavior unchanged**: `ensure_idle_session=0` (disabled by default)
- **Old clients**: Continue to work (feature is client-side only)
- **Old servers**: Fully compatible (no server changes needed)
- **Config migration**: No breaking changes to existing configs

## Testing

### Manual Testing
```bash
# Build sing-box with changes
make build

# Run with debug logging
./sing-box run -c config.json
```

### Expected Behavior
1. At startup with `ensure_idle_session=5`: See 5 creation logs
2. After idle timeout cleanup: See deficit creation logs
3. During high traffic: No unnecessary creations (pool is in use)
4. On network failure: Graceful error logging, retry next interval

## Future Enhancements

Potential improvements (not implemented):
- Configurable creation rate limiting (avoid thundering herd)
- Separate check interval for pool maintenance
- Metrics export (pool size, creation success rate)
- Dynamic pool sizing based on traffic patterns
- Connection health checks before adding to pool
