# Per-User IPv6 Implementation Summary

## Implementation Complete ✓

Successfully implemented per-user IPv6 address assignment via hash as specified in the plan.

## Files Modified

1. **option/outbound.go** - Added `Inet6BindPrefix *badoption.Prefix` field to `DialerOptions`
2. **option/rule_action.go** - Updated `DirectActionOptions.Descriptions()` to include inet6_bind_prefix
3. **common/dialer/default.go** - Added accessor methods for TCP/UDP dialers and config
4. **common/dialer/dialer.go** - Wired up the wrapper dialer in `NewWithOptions()` with validation
5. **common/dialer/xlat464_test.go** - Fixed test compatibility (added context parameter)

## Files Created

1. **common/dialer/per_user_ipv6.go** - Core wrapper dialer implementation
2. **common/dialer/per_user_ipv6_test.go** - Comprehensive unit tests
3. **PER_USER_IPV6.md** - Feature documentation
4. **examples/per_user_ipv6_config.json** - Example configuration

## Key Implementation Details

### Hash Algorithm
- Uses FNV-128a (stdlib `hash/fnv`)
- Input: `user_id` + `source_ip`
- Output: 128-bit hash (sufficient for any prefix length)

### Address Generation
- Preserves prefix bits exactly as configured
- Handles byte-aligned and non-byte-aligned prefixes correctly
- Hash fills the host portion of the address

### Dialer Integration
- Follows `xlat464Dialer` pattern (wrapper around `DefaultDialer`)
- Implements `ParallelInterfaceDialer` interface
- Falls back gracefully when no user info or IPv4 destination
- Creates per-connection dialer copies with computed `LocalAddr`

### Validation
- Mutual exclusion with `inet6_bind_address`
- Ensures prefix is IPv6
- Ensures prefix has host bits (length < 128)

## Test Results

All tests passing:
```
=== RUN   TestPerUserIPv6Hash
--- PASS: TestPerUserIPv6Hash (0.00s)
=== RUN   TestPerUserIPv6DifferentPrefixes
--- PASS: TestPerUserIPv6DifferentPrefixes (0.00s)
=== RUN   TestPerUserIPv6Uniqueness
--- PASS: TestPerUserIPv6Uniqueness (0.00s)
PASS
ok      github.com/sagernet/sing-box/common/dialer    0.006s
```

Tests verify:
- ✓ Determinism (same user+source → same IPv6)
- ✓ Uniqueness (different user/source → different IPv6)
- ✓ Prefix preservation across various lengths (/32, /48, /64, /80, /96)
- ✓ Edge cases (no user, no context)
- ✓ No hash collisions in 25 combinations

## Build Status

✓ Builds successfully: `sing-box version 1.12.14.25`
✓ Example config validates successfully
✓ All dialer tests pass (including xlat464)

## Configuration Examples

### Outbound-level:
```json
{
  "outbounds": [{
    "type": "direct",
    "inet6_bind_prefix": "2001:db8:abcd::/48"
  }]
}
```

### Rule action-level:
```json
{
  "route": {
    "rules": [{
      "auth_user": ["alice", "bob"],
      "action": "direct",
      "inet6_bind_prefix": "2001:db8:abcd::/48"
    }]
  }
}
```

## Data Flow

```
Client → Inbound (sets User + Source)
       → Router (rule matching)
       → perUserIPv6Dialer.DialContext(ctx)
           → adapter.ContextFrom(ctx) → get User + Source
           → fnv128a(user + source_ip) → hash
           → prefix | hash_host_bits → IPv6 address
           → copy inner dialer, set LocalAddr
           → dial with per-user IPv6
```

## Known Limitations

1. **TCP Fast Open**: The current implementation converts tcpDialer to net.Dialer, which preserves TFO settings but uses the standard net.Dialer flow
2. **Parallel Interface Dialing**: Falls back to inner dialer for parallel interface scenarios (interface binding may conflict with per-user address binding)
3. **IPv4 Only**: Feature only applies to IPv6 destinations; IPv4 traffic uses default dialer

## Future Enhancements (Optional)

- Integration with parallel interface dialing
- Support for IPv4 ranges (similar approach with smaller hash space)
- Configurable hash algorithm (SHA256, etc.)
- Address pool management with lease tracking

## Version Compatibility

- Requires Go 1.20+ (for existing codebase compatibility)
- No external dependencies added
- Uses only stdlib packages (`hash/fnv`, `net/netip`)
