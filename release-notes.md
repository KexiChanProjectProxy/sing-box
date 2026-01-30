## Bug Fixes

### XLAT464 Translation
- Fixed XLAT464 IPv4 address detection to properly check the `Addr` field directly
- This resolves issues where destinations with DNS-resolved IPv4 addresses were not being translated to IPv6
- The fix ensures that when using `domain_strategy: ipv4_only` with `xlat464_prefix`, domain names are properly resolved to IPv4 and then translated to IPv6

### Debug Logging
- Added comprehensive debug logging for XLAT464 translation process
- Added debug logging for sniffing actions to help troubleshoot protocol detection issues
- Debug logs show destination address details, resolution status, and translation steps

## Technical Details

The XLAT464 fix changes the detection logic from:
```go
if !destination.IsIP() || !destination.Addr.Is4()
```

To:
```go
if destination.Addr.IsValid() && destination.Addr.Is4()
```

This ensures that resolved IPv4 addresses are properly detected and translated, even when the `Socksaddr` structure contains additional fields.

## Configuration Example

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

## Flow
Domain → DNS (ipv4_only) → IPv4 → XLAT464 → IPv6 → Connection
