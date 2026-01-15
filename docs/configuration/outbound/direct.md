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
  "xlat464_prefix": "64:ff9b::/96",

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

#### xlat464_prefix

IPv6 prefix for 464XLAT (CLAT) translation. Must be a /96 prefix per RFC 6052.

When configured, IPv4 addresses are translated to IPv6 by embedding the IPv4 address in the last 32 bits of the prefix.

**Requirements:**
- Your network must support 464XLAT (PLAT present)
- The prefix must be obtained from your network provider
- Recommended to use with `domain_strategy: ipv4_only` for proper operation

**Example translation:**
- IPv4 address: `104.16.184.241`
- Prefix: `64:ff9b::/96`
- Result: `64:ff9b::6810:b8f1`

**Example configuration:**

```json
{
  "type": "direct",
  "tag": "direct-464xlat",
  "domain_strategy": "ipv4_only",
  "xlat464_prefix": "64:ff9b::/96"
}
```

**Common prefixes:**
- Well-Known Prefix (RFC 6052): `64:ff9b::/96`
- Provider-specific prefixes vary by network operator

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
