# Debug Analysis for XLAT464 and Sniffing Issues

## Changes Made

### 1. XLAT464 Translation Fix (`common/dialer/xlat464.go`)

**Problem:** The original logic checked `!destination.IsIP()` which might fail if the destination has both an `Addr` field and an `Fqdn` field set, even after DNS resolution.

**Fix:** Changed the logic to:
```go
if destination.Addr.IsValid() && destination.Addr.Is4() {
    // Translate IPv4 to IPv6
}
```

This now explicitly checks if the `Addr` field contains a valid IPv4 address, regardless of whether `Fqdn` is also set.

### 2. Added Debug Logging

Added extensive debug logging to track both issues:

#### Sniffing Debug Logs:
- `route/route.go:346`: "legacy sniff action triggered for inbound X"
- `route/route.go:496`: Details about sniff action parameters
- `route/route.go:527`: "attempting stream sniff with N sniffers"
- `route/route.go:555`: "stream sniff failed: ERROR"
- Existing logs at 543-548 for successful sniffs

#### XLAT464 Debug Logs:
- `protocol/direct/outbound.go:184-188`: Shows original destination and address details when XLAT464 is enabled

## Expected Behavior After Fix

### XLAT464:
When a connection is made:
1. Domain name is resolved to IPv4 (e.g., www.caixin.com → 118.180.6.144)
2. XLAT464 dialer receives the IPv4 address
3. Translates it to IPv6 using the prefix: `2a0c:b641:69c:b4c4:0:4::/96`
   - Example: 118.180.6.144 → 2a0c:b641:69c:b4c4:0:4:7680:690
4. Connection is made to the IPv6 address

You should see DEBUG logs showing:
```
DEBUG xlat464 enabled, original destination: ..., Addr.IsValid=true, Addr.Is4=true, Addr=118.180.6.144
```

### Sniffing:
For shadowsocks inbound with `sniff: true`:
1. Router triggers sniffing action
2. Attempts to detect protocol (TLS, HTTP, etc.)
3. If successful, extracts domain name and optionally overrides destination

You should see DEBUG logs showing:
```
DEBUG legacy sniff action triggered for inbound ss-in
DEBUG actionSniff called: inputConn=true, inputPacketConn=false, ...
DEBUG attempting stream sniff with 6 sniffers
DEBUG sniffed protocol: tls, domain: www.caixin.com
```

## Why Sniffing Might Not Work for Shadowsocks

**Important Note:** Shadowsocks already provides the destination address in its protocol. The destination "www.caixin.com:443" in the logs comes from the Shadowsocks protocol itself, not from sniffing.

Sniffing would only extract additional information like:
- Protocol detection (TLS, HTTP)
- For HTTPS connections to 443, it can read the TLS ClientHello SNI
- But this happens AFTER the Shadowsocks decryption

The debug logs will show if sniffing is:
1. Not being triggered at all
2. Being triggered but failing
3. Being triggered successfully but not finding useful information

## Testing Instructions

1. Copy `sing-box-arm64` to your NanoPi R6S:
   ```bash
   scp sing-box-arm64 nanopi-r6s:/usr/local/bin/sing-box-debug
   ```

2. Stop the existing service:
   ```bash
   ssh nanopi-r6s "systemctl stop sing-box"
   ```

3. Run with debug logging:
   ```bash
   ssh nanopi-r6s "/usr/local/bin/sing-box-debug run -c /path/to/config.json"
   ```

4. Make a test connection through the shadowsocks proxy

5. Look for the debug messages in the output:
   - Search for "xlat464 enabled" to see XLAT464 translation details
   - Search for "sniff" to see sniffing activity
   - Search for "legacy sniff action triggered" to confirm sniffing is activated

## What to Check

### For XLAT464:
- Is `Addr.IsValid=true`?
- Is `Addr.Is4=true`?
- Is `Addr` showing an IPv4 address?
- If all yes, translation should happen. If no IPv6 in connection, check network routing.

### For Sniffing:
- Is "legacy sniff action triggered" appearing?
- Is "actionSniff called" appearing?
- If yes, look for either:
  - "sniffed protocol: ..." (success)
  - "stream sniff failed: ..." (failure with reason)
- If no, the sniff action is not being triggered properly.

## Potential Root Causes

### XLAT464 Not Working:
1. **Addr field not set**: ResolveDialer might be passing domain instead of IP
2. **Network routing**: Translation works but system doesn't route IPv6
3. **DNS not resolving**: Domain strategy not forcing IPv4

### Sniffing Not Working:
1. **Not triggered**: Inbound options not properly set
2. **Already has protocol**: Another component already set metadata.Protocol
3. **Connection type**: Shadowsocks already provides destination, no need to sniff
4. **Timeout**: Sniffing timeout too short to read ClientHello

## Next Steps

Run the debug build and share the logs. The new debug messages will reveal exactly where the issues are occurring.
