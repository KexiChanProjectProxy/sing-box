---
icon: material/alert-decagram
---

!!! quote "Changes in sing-box 1.14.0"

    :material-plus: [xlat464](#xlat464)

!!! quote "Changes in sing-box 1.11.0"

    :material-delete-clock: [override_address](#override_address)  
    :material-delete-clock: [override_port](#override_port)

`direct` outbound send requests directly.

### Structure

```json
{
  "type": "direct",
  "tag": "direct-out",

  "xlat464": {
    "prefix": "64:ff9b::/96",
    "allow_ipv6": false
  },
  
  "override_address": "1.0.0.1",
  "override_port": 53,
  
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

#### xlat464

XLAT464 (NAT64) address translation for the direct outbound. When configured, IPv4 literal destinations and IPv4 addresses obtained from domain resolution (A records) are embedded into the specified `/96` IPv6 prefix, allowing IPv4-only destinations to be reached over an IPv6-only network path.

```json
{
  "xlat464": {
    "prefix": "64:ff9b::/96",
    "allow_ipv6": false
  }
}
```

!!! info "Contract"

    - The `prefix` must be an IPv6 prefix with a `/96` length. Any other prefix length is rejected.
    - IPv4 literal destinations and A-record answers are embedded into the prefix (e.g. `192.0.2.1` becomes `64:ff9b::c000:201`).
    - Domain resolution for direct-owned destinations is forced to A-only (IPv4). AAAA answers supplied by a preceding route `resolve` action are dropped.
    - TCP and UDP protocols are supported.
    - Native IPv6 literal destinations outside the configured prefix are rejected by default, preventing them from bypassing the NAT64 path. Set `allow_ipv6` to `true` to explicitly allow direct native IPv6 connections.
    - ICMP, DNS64, automatic prefix discovery, and non-`/96` prefix lengths are not supported.
    - This option is exclusive to the direct outbound and does not apply to other outbound types.

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
