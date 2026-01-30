## Bug Fixes

### XLAT464 Translation Fix
- **Fixed XLAT464 IPv4 address detection** to properly check `Addr.IsValid() && Addr.Is4()` directly
- This resolves issues where destinations with DNS-resolved IPv4 addresses were not being translated to IPv6
- The fix ensures that when using `domain_strategy: ipv4_only` with `xlat464_prefix`, domain names are properly resolved to IPv4 and then translated to IPv6

### Technical Details

**The XLAT464 fix changes the detection logic from:**
```go
if !destination.IsIP() || !destination.Addr.Is4()
```

**To:**
```go
if destination.Addr.IsValid() && destination.Addr.Is4()
```

This ensures that resolved IPv4 addresses are properly detected and translated, even when the `Socksaddr` structure contains additional fields.

### How XLAT464 Works

The correct dialer chain flow is:
```
Domain → ResolveDialer (DNS with ipv4_only) → IPv4 Address → XLAT464Dialer → IPv6 Address → Connection
```

**Example:**
1. Domain: `file.caixin.com:443`
2. DNS resolves to: `61.156.246.8`
3. XLAT464 translates to: `2a0c:b641:69c:b4c4:0:4:3d9c:f608:443`
4. Connection made to IPv6 address

### Configuration Example

```json
{
  "outbounds": [
    {
      "type": "direct",
      "domain_strategy": "ipv4_only",
      "xlat464_prefix": "2a0c:b641:69c:b4c4:0:4::/96"
    }
  ]
}
```

**Important:** The prefix must be a `/96` prefix per RFC 6052.

### Previous Changes (from v1.12.14.7)

- Added debug logging for inbound sniffing actions
- Shows when sniffing is triggered and results

---

**Note:** Initial v1.12.14.8 binaries had debug logging that caused crashes. These have been replaced with stable binaries containing only the core XLAT464 fix.
