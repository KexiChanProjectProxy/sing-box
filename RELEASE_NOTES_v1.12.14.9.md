# sing-box v1.12.14.9

## Release Date
January 24, 2026

## Overview
This release introduces VLESS connection pool management for improved latency and reliability.

## New Features

### VLESS Connection Pool 🎯
Implements AnyTLS-style connection pool management for VLESS protocol (client-side only).

**Key Features:**
- **Pre-connections**: Maintain warm TCP+TLS connections to reduce first-request latency
- **Rate limiting**: Prevent connection storms during pool warmup
- **Idle management**: Automatic cleanup with configurable minimum protection
- **Age rotation**: Connection lifetime limits with jitter to prevent thundering herd
- **TCP keepalive**: Optional heartbeat to detect dead connections
- **Network-aware**: Automatic reset on interface changes

**Configuration Example:**
```json
{
  "type": "vless",
  "server": "example.com",
  "server_port": 443,
  "uuid": "your-uuid",
  "tls": {
    "enabled": true,
    "server_name": "example.com"
  },
  "connection_pool": {
    "ensure_idle_session": 3,
    "min_idle_session": 2,
    "idle_session_timeout": "5m",
    "max_connection_lifetime": "1h",
    "connection_lifetime_jitter": "10m"
  }
}
```

**Available Options:**
- `ensure_idle_session`: Number of idle connections to maintain (0 = disabled)
- `ensure_idle_session_create_rate`: Max connections to create per cycle (default: 1)
- `min_idle_session`: Minimum idle connections to keep (default: 0)
- `min_idle_session_for_age`: Minimum idle when doing age-based cleanup
- `idle_session_check_interval`: Maintenance interval (default: 30s)
- `idle_session_timeout`: Close idle connections after duration (default: 5m)
- `max_connection_lifetime`: Maximum connection age before rotation (0 = disabled)
- `connection_lifetime_jitter`: Random jitter for lifetime (prevents thundering herd)
- `heartbeat`: TCP-level keepalive interval (0 = disabled)

**Compatibility:**
- ✅ Works with multiplex (pool provides base connections, multiplex streams over them)
- ✅ Works with TLS and all transport types
- ✅ Non-breaking (disabled by default)
- ❌ Incompatible with TCP Fast Open (validation added)

**Use Cases:**
- **Low Latency**: Gaming, trading applications
- **High Availability**: Production services with connection rotation
- **Resource Efficient**: Mobile/IoT with minimal connections

## Technical Details

### Implementation
- **Files Added**:
  - `protocol/vless/pool.go` - Core connection pool implementation (431 lines)
  - `protocol/vless/pool_test.go` - Comprehensive unit tests (656 lines)
  - `protocol/vless/integration_test.go` - Integration tests (214 lines)

- **Files Modified**:
  - `option/vless.go` - Added connection pool configuration options
  - `protocol/vless/outbound.go` - Integrated pool into VLESS outbound

### Testing
- 18 comprehensive tests covering:
  - Basic functionality (connection creation, reuse)
  - Concurrency and thread safety
  - Idle timeout cleanup
  - Age-based rotation with jitter
  - Minimum idle protection
  - Network interface reset
  - Graceful shutdown
  - Configuration parsing and validation

- ✅ All tests pass with race detector (`-race` flag)
- ✅ No data races detected
- ✅ No goroutine leaks
- ✅ Full build verification

### Architecture
```
User Request → Outbound.DialContext → connPool.GetConn
    → [Reuse Idle] or [Create New] → TCP+TLS Connection
    → VLESS Handshake → Application Data
    → Close (returns to pool)
```

### Maintenance Algorithm
Every `idle_session_check_interval`:
1. **Phase 1: Cleanup**
   - Close idle connections older than `idle_session_timeout` (respects `min_idle_session`)
   - Close connections exceeding `max_connection_lifetime` (respects `min_idle_session_for_age`)

2. **Phase 2: Ensure**
   - Count current idle connections
   - Create needed connections up to `ensure_idle_session`
   - Limit creation rate to `ensure_idle_session_create_rate` per cycle

## Performance Impact

### Benefits
- Reduced latency on subsequent requests (no TCP+TLS handshake overhead)
- Improved reliability with connection rotation
- Better resource utilization with idle management

### Costs
- Memory overhead: ~10KB per pooled connection
- Background maintenance goroutine
- Slightly increased complexity

## Known Limitations
1. **No Protocol-Level Heartbeat**: VLESS has no PING/PONG mechanism, only TCP keepalive available
2. **No Pre-Handshake Pool**: VLESS handshake requires destination, happens on-demand
3. **TCP Fast Open Incompatible**: TFO lazy connections conflict with eager pooling
4. **Memory Overhead**: Each pooled connection consumes ~10KB even when idle

## Documentation
- Complete implementation guide: `VLESS_CONNECTION_POOL_IMPLEMENTATION.md`
- Example configurations: `vless-connection-pool-examples.json`

## Breaking Changes
None. The connection pool is completely optional and disabled by default.

## Upgrade Notes
Existing configurations continue to work without modification. To enable connection pooling, add the `connection_pool` section to your VLESS outbound configuration.

## Contributors
- Implementation and testing by Claude Code

---

**Full Changelog**: https://github.com/sagernet/sing-box/compare/v1.12.14.8...v1.12.14.9
