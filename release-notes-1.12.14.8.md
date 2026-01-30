## Debug Logging Enhancement

### XLAT464 and DNS Resolution Debugging
- Added comprehensive debug logging in XLAT464 translation layer
- Added debug logging in DNS ResolveDialer to track domain resolution
- Logs now show the complete flow: Domain → DNS Resolution → XLAT464 Translation → Connection

### What's New in Debug Logs

**ResolveDialer logs:**
- `resolve: destination is already IP, passing through` - When destination is already an IP
- `resolve: resolving domain X with strategy Y` - DNS resolution start
- `resolve: resolved X to N addresses: [...]` - DNS resolution results

**XLAT464 logs:**
- `xlat464: received destination: X, Addr.IsValid=Y, Addr.Is4=Z, Fqdn=...` - What XLAT464 receives
- `xlat464: translated A to B` - Successful IPv4 to IPv6 translation
- `xlat464: skipping translation (not IPv4)` - When translation is skipped

### Use Case

These logs help diagnose XLAT464 translation issues by showing:
1. Whether DNS resolution is working correctly
2. What destination format XLAT464 receives (IP vs domain)
3. Whether IPv4 addresses are being properly detected and translated
4. The complete IPv4-to-IPv6 translation process

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

### Expected Log Flow

For a domain like `file.caixin.com:443` with XLAT464 enabled:

```
DEBUG resolve: resolving domain file.caixin.com with strategy ipv4_only
DEBUG resolve: resolved file.caixin.com to 3 addresses: [61.156.246.8 61.156.246.6 61.156.246.9]
DEBUG xlat464: received destination: 61.156.246.8:443, Addr.IsValid=true, Addr.Is4=true, Fqdn=
DEBUG xlat464: translated 61.156.246.8 to 2a0c:b641:69c:b4c4:0:4:3d9c:f608
```

## Previous Changes (from v1.12.14.7)

### XLAT464 Translation Fix
- Fixed IPv4 detection logic to check `Addr.IsValid() && Addr.Is4()` directly
- Ensures DNS-resolved IPv4 addresses are properly translated to IPv6

### Sniffing Debug Logs
- Added debug logging for inbound sniffing actions
- Shows when sniffing is triggered and results
