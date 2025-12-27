# sing-box Custom Build with Configurable TCP Keepalive

## Build Information

This is a custom build of sing-box 1.12.14 with enhanced TCP keepalive configuration support.

### Build Date
2025-12-27

### Platforms
- Linux AMD64 & ARM64
- Windows AMD64 & ARM64 
- Darwin/macOS AMD64 & ARM64 (Apple Silicon)

### Build Tags
- with_gvisor
- with_quic
- with_dhcp
- with_wireguard
- with_utls
- with_acme
- with_clash_api
- with_tailscale

## New Features: Configurable TCP Keepalive

### Per-Outbound Configuration
You can now configure TCP keepalive settings for each outbound:

```json
{
  "outbounds": [{
    "type": "direct",
    "tag": "direct",
    "disable_tcp_keep_alive": false,
    "tcp_keep_alive": "10m",
    "tcp_keep_alive_interval": "75s"
  }]
}
```

### System-Wide Defaults
Set default TCP keepalive values for all outbounds in the route section:

```json
{
  "route": {
    "default_tcp_keep_alive": "10m",
    "default_tcp_keep_alive_interval": "75s"
  }
}
```

### Configuration Priority
1. **Outbound-specific settings** (highest priority)
2. **System-wide route defaults** (medium priority)
3. **Hardcoded defaults** (10 minutes idle, 75 seconds interval)

### Options

- `disable_tcp_keep_alive`: Disable TCP keepalive entirely (default: false)
- `tcp_keep_alive`: Time before first keepalive probe (e.g., "10m", "5m30s")
- `tcp_keep_alive_interval`: Time between keepalive probes (e.g., "75s", "1m")
- `default_tcp_keep_alive`: System-wide default keepalive idle time
- `default_tcp_keep_alive_interval`: System-wide default keepalive interval

## File Checksums

See `SHA256SUMS` for archive checksums and `SHA256SUMS-binaries` for raw binary checksums.

## Installation

Extract the appropriate archive for your platform:

```bash
# Linux/macOS
tar xzf sing-box-<platform>-<arch>.tar.gz
chmod +x sing-box-<platform>-<arch>
./sing-box-<platform>-<arch> version

# Windows
tar xzf sing-box-windows-amd64.exe.tar.gz
.\sing-box-windows-amd64.exe version
```

## Modified Files

1. `option/outbound.go` - Added TCP keepalive fields to DialerOptions
2. `option/route.go` - Added system-wide default TCP keepalive settings
3. `adapter/network.go` - Added TCP keepalive to NetworkOptions
4. `route/network.go` - Initialize default TCP keepalive from route config
5. `common/dialer/default.go` - Implement configurable TCP keepalive logic

---
Built with ❤️ using sing-box 1.12.14
