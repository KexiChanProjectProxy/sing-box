---
icon: material/alert-decagram
---

!!! quote "Changes in sing-box 1.11.0"

    :material-delete-clock: [override_address](#override_address)  
    :material-delete-clock: [override_port](#override_port)

`direct` outbound send requests directly.

### Structure

```json
{
  "type": "direct",
  "tag": "direct-out",

  "override_address": "1.0.0.1",
  "override_port": 53,
  "use_origin_dst": false,
  "xlat464_prefix": "",

  ... // Dial Fields
}
```

### Fields

#### override_address

!!! failure "Deprecated in sing-box 1.11.0"

    Destination override fields are deprecated in sing-box 1.11.0 and will be removed in sing-box 1.13.0, see [Migration](/migration/#migrate-destination-override-fields-to-route-options).

Override the connection destination address.

#### override_port

!!! failure "Deprecated in sing-box 1.11.0"

    Destination override fields are deprecated in sing-box 1.11.0 and will be removed in sing-box 1.13.0, see [Migration](/migration/#migrate-destination-override-fields-to-route-options).

Override the connection destination port.

Protocol value can be `1` or `2`.

#### use_origin_dst

Use the original destination address obtained via `SO_ORIGINAL_DST` (Linux transparent proxy) instead of the connection's apparent destination.

Use this when the connection was redirected by iptables/nftables REDIRECT or TPROXY, and the direct outbound should forward to the pre-redirect destination rather than the local listener address.

#### xlat464_prefix

464XLAT (CLAT) prefix for synthesizing an IPv4-mapped IPv6 address (RFC 6052). Must be a `/96` prefix.

When set, IPv4 destination addresses are translated to IPv6 by embedding them in this prefix, enabling IPv4 connectivity over IPv6-only networks.

Example: `"64:ff9b::/96"` (the well-known NAT64 prefix)

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
