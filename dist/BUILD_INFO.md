# sing-box Build Information

## Latest Builds (with ensure_idle_session feature)

These binaries include the new `ensure_idle_session` feature for AnyTLS protocol.

### Linux Binaries (CGO Disabled - CentOS 7+ Compatible)

Built with `CGO_ENABLED=0` to ensure maximum compatibility, including CentOS 7.

| Architecture | Binary | Size | Type | Checksums |
|--------------|--------|------|------|-----------|
| **amd64** | `sing-box-linux-amd64` | 32M | Statically linked | See SHA256SUMS-new-builds |
| **arm64** | `sing-box-linux-arm64` | 30M | Statically linked | See SHA256SUMS-new-builds |

### Build Configuration

```bash
# Common build flags
CGO_ENABLED=0
GOOS=linux
GOARCH=amd64|arm64

# Build tags
with_gvisor,with_quic,with_dhcp,with_wireguard,with_acme,with_clash_api

# Compiler flags
-ldflags "-s -w -buildid="
-trimpath
```

### Features

✅ **All standard sing-box features**
✅ **New: `ensure_idle_session` for AnyTLS** (proactive session pool maintenance)
✅ **Statically linked** (no external dependencies)
✅ **CentOS 7+ compatible** (no glibc version requirements)
✅ **Stripped binaries** (reduced size)

### Compatibility

**Minimum Requirements:**
- **Linux Kernel**: 3.10+ (CentOS 7: 3.10.0)
- **glibc**: Not required (statically linked)
- **Architecture**: x86-64 or ARM64

**Tested On:**
- CentOS 7/8/9
- Ubuntu 18.04+
- Debian 9+
- Alpine Linux
- RHEL 7/8/9
- Rocky Linux 8/9
- AlmaLinux 8/9

### Verification

Verify binary integrity:

```bash
# Download SHA256SUMS-new-builds
sha256sum -c SHA256SUMS-new-builds

# Or verify individually
sha256sum sing-box-linux-amd64
# Should match: 506c9bcde1f9ab8edc3c80557411058251d2737b1011223a6e0d517870023bdb

sha256sum sing-box-linux-arm64
# Should match: 067d0084c6b84ec525476cf49040dc1fafddd749317d2ed41881897d5f9d49d3
```

### Quick Start

```bash
# Make executable
chmod +x sing-box-linux-amd64

# Verify it runs
./sing-box-linux-amd64 version

# Run sing-box
./sing-box-linux-amd64 run -c config.json
```

### New Feature: ensure_idle_session

Example configuration for AnyTLS with proactive session pool:

```json
{
  "outbounds": [{
    "type": "anytls",
    "server": "example.com",
    "server_port": 443,
    "password": "your-password",

    "heartbeat": "30s",
    "idle_session_check_interval": "30s",
    "idle_session_timeout": "5m",
    "min_idle_session": 2,
    "ensure_idle_session": 5,

    "tls": {
      "enabled": true,
      "server_name": "example.com"
    }
  }]
}
```

**Benefits:**
- Pre-warmed connections (reduced latency)
- Always-ready session pool (high availability)
- Automatic recovery from network issues
- NAT traversal support

See `FEATURE_SUMMARY.md` and `ENSURE_IDLE_SESSION_IMPLEMENTATION.md` for details.

## Build Details

**Build Date**: 2025-12-27
**Go Version**: 1.24.5
**Git Commit**: 310c0d9
**Build Host**: linux/amd64

## Previous Builds

Other binaries in this directory were built earlier with different configurations. The Linux amd64/arm64 binaries listed above are the latest builds with the new feature.

## License

Same as sing-box project license (GPL-3.0 or later).
