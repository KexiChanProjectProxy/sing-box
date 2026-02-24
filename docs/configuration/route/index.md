---
icon: material/alert-decagram
---

# Route

!!! quote "Changes in sing-box 1.12.0"

    :material-plus: [default_domain_resolver](#default_domain_resolver)  
    :material-note-remove: [geoip](#geoip)  
    :material-note-remove: [geosite](#geosite)

!!! quote "Changes in sing-box 1.11.0"

    :material-plus: [default_network_strategy](#default_network_strategy)  
    :material-plus: [default_network_type](#default_network_type)  
    :material-plus: [default_fallback_network_type](#default_fallback_network_type)  
    :material-plus: [default_fallback_delay](#default_fallback_delay)

!!! quote "Changes in sing-box 1.8.0"

    :material-plus: [rule_set](#rule_set)  
    :material-delete-clock: [geoip](#geoip)  
    :material-delete-clock: [geosite](#geosite)

### Structure

```json
{
  "route": {
    "rules": [],
    "rule_set": [],
    "final": "",
    "auto_detect_interface": false,
    "override_android_vpn": false,
    "default_interface": "",
    "default_mark": 0,
    "default_domain_resolver": "", // or {}
    "default_network_strategy": "",
    "default_network_type": [],
    "default_fallback_network_type": [],
    "default_fallback_delay": "",
    "default_tcp_keep_alive": "",
    "default_tcp_keep_alive_interval": "",
    "sniff_override_destination": false,
    "asn": {},

    // Removed

    "geoip": {},
    "geosite": {}
  }
}
```

!!! note ""

    You can ignore the JSON Array [] tag when the content is only one item

### Fields

#### rules

List of [Route Rule](./rule/)

#### rule_set

!!! question "Since sing-box 1.8.0"

List of [rule-set](/configuration/rule-set/)

#### final

Default outbound tag. the first outbound will be used if empty.

#### auto_detect_interface

!!! quote ""

    Only supported on Linux, Windows and macOS.

Bind outbound connections to the default NIC by default to prevent routing loops under tun.

Takes no effect if `outbound.bind_interface` is set.

#### override_android_vpn

!!! quote ""

    Only supported on Android.

Accept Android VPN as upstream NIC when `auto_detect_interface` enabled.

#### default_interface

!!! quote ""

    Only supported on Linux, Windows and macOS.

Bind outbound connections to the specified NIC by default to prevent routing loops under tun.

Takes no effect if `auto_detect_interface` is set.

#### default_mark

!!! quote ""

    Only supported on Linux.

Set routing mark by default.

Takes no effect if `outbound.routing_mark` is set.

#### default_domain_resolver

!!! question "Since sing-box 1.12.0"

See [Dial Fields](/configuration/shared/dial/#domain_resolver) for details.

Can be overrides by `outbound.domain_resolver`.

#### default_network_strategy

!!! question "Since sing-box 1.11.0"

See [Dial Fields](/configuration/shared/dial/#network_strategy) for details.

Takes no effect if `outbound.bind_interface`, `outbound.inet4_bind_address` or `outbound.inet6_bind_address` is set.

Can be overrides by `outbound.network_strategy`.

Conflicts with `default_interface`.

#### default_network_type

!!! question "Since sing-box 1.11.0"

See [Dial Fields](/configuration/shared/dial/#network_type) for details.

#### default_fallback_network_type

!!! question "Since sing-box 1.11.0"

See [Dial Fields](/configuration/shared/dial/#fallback_network_type) for details.

#### default_fallback_delay

!!! question "Since sing-box 1.11.0"

See [Dial Fields](/configuration/shared/dial/#fallback_delay) for details.

#### default_tcp_keep_alive

Default TCP keep-alive initial period for outbound connections. `5m` is used by default.

Can be overridden by `outbound` dial settings.

#### default_tcp_keep_alive_interval

Default TCP keep-alive interval for outbound connections. `75s` is used by default.

Can be overridden by `outbound` dial settings.

#### sniff_override_destination

Global default: when a `sniff` rule action successfully detects a domain name from the connection (e.g. TLS SNI, HTTP `Host` header), replace the connection's IP destination with the sniffed domain name, keeping the same port.

This causes subsequent DNS resolution and routing decisions to use the domain name rather than the original IP address, which is useful for:

- Accurate domain-based routing after receiving an IP connection (e.g. from a transparent proxy)
- Enabling remote DNS resolution by the outbound instead of relying on the client's resolved IP

**Preferred approach — per-rule `override_destination`:**

Use the `override_destination` field on individual `sniff` rule actions to enable the override selectively, without affecting all rules:

```json
{
  "route": {
    "rules": [
      {
        "action": "sniff",
        "override_destination": true
      }
    ]
  }
}
```

See [`sniff` action](/configuration/route/rule_action/#sniff) for details.

**Global fallback:**

```json
{
  "route": {
    "sniff_override_destination": true,
    "rules": [
      {
        "action": "sniff"
      }
    ]
  }
}
```

The override only applies when a domain name is successfully sniffed. If sniffing fails or the connection carries no recognisable domain (e.g. raw IP traffic), the destination is left unchanged.

Can also be set per-inbound via the deprecated `inbound.sniff_override_destination` option.

#### asn

ASN database configuration. Required for `dst_asn` in [LoadBalance](/configuration/outbound/loadbalance/) hash key parts and for `ip_asn` route rules.

| Field | Description |
|-------|-------------|
| `path` | Path to the ASN MMDB file |
| `download_url` | URL to automatically download the database from |
| `download_detour` | Outbound tag to use for downloading the database |

Supports MaxMind GeoLite2-ASN format (MMDB).
