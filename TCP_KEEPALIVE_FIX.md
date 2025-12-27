# TCP Keep-Alive Fix for Go 1.23+

## Problem

TCP keep-alive was not being enabled for direct outbound (and other outbounds) when compiled with Go 1.23+ (including Go 1.24), even when properly configured in the JSON config:

```json
{
  "type": "direct",
  "tag": "direct",
  "tcp_keep_alive": "10s",
  "tcp_keep_alive_interval": "8s"
}
```

## Root Cause

Go 1.23 introduced a new `KeepAliveConfig` struct for `net.Dialer` that provides more precise control over TCP keep-alive settings:

```go
type KeepAliveConfig struct {
    Enable   bool
    Idle     time.Duration
    Interval time.Duration
}
```

When using Go 1.23+, this new field takes precedence over the legacy `KeepAlive` field. The sing-box dialer code was only using the legacy method:

```go
dialer.KeepAlive = keepAliveIdle
dialer.Control = control.Append(dialer.Control, control.SetKeepAlivePeriod(keepAliveIdle, keepAliveInterval))
```

This approach worked in Go 1.22 and earlier, but in Go 1.23+, the `KeepAliveConfig` field needs to be explicitly set with `Enable: true`.

## The Fix

Created build-tag specific implementations similar to the listener code:

### Files Created:

1. **common/dialer/default_go1.23.go** - For Go 1.23+
   - Uses the new `KeepAliveConfig` API
   - Explicitly sets `Enable: true`

2. **common/dialer/default_nogo1.23.go** - For Go 1.22 and earlier
   - Uses the legacy `KeepAlive` field
   - Uses Control function for socket options

### Modified:

**common/dialer/default.go:174-175**
- Changed from directly setting `dialer.KeepAlive` and `dialer.Control`
- Now calls `setKeepAliveConfig(&dialer, keepAliveIdle, keepAliveInterval)`
- This function is implemented differently based on Go version

## Technical Details

### Go 1.23+ Implementation (default_go1.23.go):
```go
func setKeepAliveConfig(dialer *net.Dialer, idle time.Duration, interval time.Duration) {
    dialer.KeepAliveConfig = net.KeepAliveConfig{
        Enable:   true,
        Idle:     idle,
        Interval: interval,
    }
}
```

### Pre-Go 1.23 Implementation (default_nogo1.23.go):
```go
func setKeepAliveConfig(dialer *net.Dialer, idle time.Duration, interval time.Duration) {
    dialer.KeepAlive = idle
    dialer.Control = control.Append(dialer.Control, control.SetKeepAlivePeriod(idle, interval))
}
```

## Verification

Build and test:
```bash
make build
./sing-box version
```

Expected output should show Go 1.24+ and the binary will now properly enable TCP keep-alive.

Test configuration:
```bash
./sing-box run -c test_keepalive_config.json
```

## Impact

This fix affects all outbound protocols that use the dialer:
- direct
- shadowsocks
- vmess
- trojan
- hysteria
- wireguard
- socks
- http
- and all other outbound types

TCP keep-alive settings will now work correctly when:
- Compiled with Go 1.23 or later
- Configuration includes `tcp_keep_alive` and/or `tcp_keep_alive_interval`
- `disable_tcp_keep_alive` is not set to true

## Backward Compatibility

✅ Fully backward compatible
- Go 1.22 and earlier: Uses legacy method (unchanged behavior)
- Go 1.23+: Uses new API (fixes the issue)
- Configuration syntax: Unchanged
- Default behavior: Unchanged (keep-alive enabled with system defaults if not configured)
