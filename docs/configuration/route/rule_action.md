---
icon: material/new-box
---

!!! quote "Changes in sing-box 1.12.0"

    :material-plus: [tls_fragment](#tls_fragment)  
    :material-plus: [tls_fragment_fallback_delay](#tls_fragment_fallback_delay)  
    :material-plus: [tls_record_fragment](#tls_record_fragment)  
    :material-plus: [resolve.disable_cache](#disable_cache)  
    :material-plus: [resolve.rewrite_ttl](#rewrite_ttl)  
    :material-plus: [resolve.client_subnet](#client_subnet)

## Final actions

### route

```json
{
  "action": "route", // default
  "outbound": "",
 
  ... // route-options Fields
}
```

!!! note ""

    You can ignore the JSON Array [] tag when the content is only one item

`route` inherits the classic rule behavior of routing connection to the specified outbound.

#### outbound

==Required==

Tag of target outbound.

#### route-options Fields

See `route-options` fields below.

### reject

```json
{
  "action": "reject",
  "method": "default", // default
  "no_drop": false
}
```

`reject` reject connections

The specified method is used for reject tun connections if `sniff` action has not been performed yet.

For non-tun connections and already established connections, will just be closed.

#### method

- `default`: Reply with TCP RST for TCP connections, and ICMP port unreachable for UDP packets.
- `drop`: Drop packets.

#### no_drop

If not enabled, `method` will be temporarily overwritten to `drop` after 50 triggers in 30s.

Not available when `method` is set to drop.

### hijack-dns

```json
{
  "action": "hijack-dns"
}
```

`hijack-dns` hijack DNS requests to the sing-box DNS module.

## Non-final actions

### route-options

```json
{
  "action": "route-options",
  "override_address": "",
  "override_port": 0,
  "network_strategy": "",
  "fallback_delay": "",
  "udp_disable_domain_unmapping": false,
  "udp_connect": false,
  "udp_timeout": "",
  "tls_fragment": false,
  "tls_fragment_fallback_delay": "",
  "tls_record_fragment": ""
}
```

`route-options` set options for routing.

#### override_address

Override the connection destination address.

#### override_port

Override the connection destination port.

#### network_strategy

See [Dial Fields](/configuration/shared/dial/#network_strategy) for details.

Only take effect if outbound is direct without `outbound.bind_interface`,
`outbound.inet4_bind_address` and `outbound.inet6_bind_address` set.

#### network_type

See [Dial Fields](/configuration/shared/dial/#network_type) for details.

#### fallback_network_type

See [Dial Fields](/configuration/shared/dial/#fallback_network_type) for details.

#### fallback_delay

See [Dial Fields](/configuration/shared/dial/#fallback_delay) for details.

#### udp_disable_domain_unmapping

If enabled, for UDP proxy requests addressed to a domain,
the original packet address will be sent in the response instead of the mapped domain.

This option is used for compatibility with clients that
do not support receiving UDP packets with domain addresses, such as Surge.

#### udp_connect

If enabled, attempts to connect UDP connection to the destination instead of listen.

#### udp_timeout

Timeout for UDP connections.

Setting a larger value than the UDP timeout in inbounds will have no effect.

Default value for protocol sniffed connections:

| Timeout | Protocol             |
|---------|----------------------|
| `10s`   | `dns`, `ntp`, `stun` |
| `30s`   | `quic`, `dtls`       |

If no protocol is sniffed, the following ports will be recognized as protocols by default:

| Port | Protocol |
|------|----------|
| 53   | `dns`    |
| 123  | `ntp`    |
| 443  | `quic`   |
| 3478 | `stun`   |

#### tls_fragment

!!! question "Since sing-box 1.12.0"

Fragment TLS handshakes to bypass firewalls.

This feature is intended to circumvent simple firewalls based on **plaintext packet matching**,
and should not be used to circumvent real censorship.

Due to poor performance, try `tls_record_fragment` first, and only apply to server names known to be blocked.

On Linux, Apple platforms, (administrator privileges required) Windows,
the wait time can be automatically detected. Otherwise, it will fall back to
waiting for a fixed time specified by `tls_fragment_fallback_delay`.

In addition, if the actual wait time is less than 20ms, it will also fall back to waiting for a fixed time,
because the target is considered to be local or behind a transparent proxy.

#### tls_fragment_fallback_delay

!!! question "Since sing-box 1.12.0"

The fallback value used when TLS segmentation cannot automatically determine the wait time.

`500ms` is used by default.

#### tls_record_fragment

!!! question "Since sing-box 1.12.0"

Fragment TLS handshake into multiple TLS records to bypass firewalls.

### sniff

!!! quote "Changes in sing-box 1.12.14.10"

    :material-plus: [protocols](#protocols)
    :material-plus: [skip_domain](#skip_domain)
    :material-plus: [skip_domain_suffix](#skip_domain_suffix)
    :material-plus: [skip_src_address](#skip_src_address)
    :material-plus: [skip_dst_address](#skip_dst_address)

**Legacy Mode (Simple):**

```json
{
  "action": "sniff",
  "sniffer": [],
  "timeout": "",
  "override_destination": false
}
```

**Advanced Mode:**

```json
{
  "action": "sniff",
  "timeout": "300ms",
  "override_destination": false,
  "protocols": {
    "http": {
      "ports": [80],
      "port_ranges": ["8080-8880", "3000:3100"],
      "override_destination": true
    },
    "tls": {
      "ports": [443, 8443],
      "override_destination": true
    },
    "quic": {
      "ports": [443]
    }
  },
  "skip_domain": ["example.com"],
  "skip_domain_suffix": ["local", "lan"],
  "skip_src_address": ["192.168.0.3/32"],
  "skip_dst_address": ["127.0.0.1"]
}
```

`sniff` performs protocol sniffing on connections.

For deprecated `inbound.sniff` options, it is considered to `sniff()` performed before routing.

!!! note "Mode Detection"

    - If `protocols` field is present → **Advanced Mode**
    - If only `sniffer` field is present → **Legacy Mode**
    - If both `protocols` and `sniffer` are present → **Error** (mutually exclusive)

#### sniffer

Enabled sniffers.

All sniffers enabled by default.

Available protocol values can be found on in [Protocol Sniff](../sniff/)

#### timeout

Timeout for sniffing.

`300ms` is used by default.

#### override_destination

!!! question "Since sing-box 1.12.14.10"

Global default for whether to override the connection destination with the sniffed domain.

Can be overridden per-protocol in advanced mode.

Default: `false`

#### protocols

!!! question "Since sing-box 1.12.14.10"

Per-protocol configuration for advanced sniffing control.

When using `protocols`, the `sniffer` field cannot be used.

**Available protocols:** `http`, `tls`, `quic`, `dns`, `stun`, `bittorrent`, `dtls`, `ssh`, `rdp`, `ntp`

Each protocol configuration supports:

##### ports

List of specific ports to sniff for this protocol.

Example: `[80, 8080]`

##### port_ranges

List of port ranges to sniff for this protocol.

Supports both dash (`8080-8880`) and colon (`8080:8880`) formats.

Example: `["8080-8880", "3000:3100"]`

##### override_destination

Per-protocol override destination setting.

- If `null` or not specified: Uses global `override_destination` setting
- If `true`: Override destination for this protocol
- If `false`: Don't override destination for this protocol

#### skip_domain

!!! question "Since sing-box 1.12.14.10"

List of exact domain names that should NOT have their destination overridden.

Sniffing still occurs, but the destination remains as the original IP address.

Example: `["Mijia Cloud", "api.example.com"]`

#### skip_domain_suffix

!!! question "Since sing-box 1.12.14.10"

List of domain suffixes that should NOT have their destination overridden.

Uses standard domain suffix matching (e.g., `"local"` matches `*.local`).

Example: `["local", "lan", "internal"]`

#### skip_src_address

!!! question "Since sing-box 1.12.14.10"

List of source IP addresses or CIDR ranges to skip sniffing entirely.

When a connection's source IP matches, no sniffing is performed.

Accepts both single IPs and CIDR notation.

Example: `["192.168.0.3/32", "10.0.0.0/8"]`

#### skip_dst_address

!!! question "Since sing-box 1.12.14.10"

List of destination IP addresses or CIDR ranges to skip sniffing entirely.

When a connection's destination IP matches, no sniffing is performed.

Example: `["127.0.0.1", "192.168.0.0/16"]`

## Configuration Examples

### Example 1: HTTP on Non-Standard Ports

Sniff HTTP traffic on port 80 and range 8080-8880, override destination:

```json
{
  "action": "sniff",
  "protocols": {
    "http": {
      "ports": [80],
      "port_ranges": ["8080-8880"],
      "override_destination": true
    }
  }
}
```

### Example 2: Skip Override for Local Domains

Sniff TLS and HTTP but don't override .local or .lan domains:

```json
{
  "action": "sniff",
  "sniffer": ["tls", "http"],
  "override_destination": true,
  "skip_domain_suffix": ["local", "lan"]
}
```

### Example 3: Skip Private Network Sniffing

Don't sniff internal network traffic:

```json
{
  "action": "sniff",
  "sniffer": ["tls", "http"],
  "skip_src_address": ["10.0.0.0/8", "192.168.0.0/16"],
  "skip_dst_address": ["127.0.0.0/8"]
}
```

### Example 4: Per-Protocol Override Control

Override for HTTP and TLS, but not QUIC:

```json
{
  "action": "sniff",
  "override_destination": false,
  "protocols": {
    "http": {
      "ports": [80],
      "override_destination": true
    },
    "tls": {
      "ports": [443],
      "override_destination": true
    },
    "quic": {
      "ports": [443]
    }
  }
}
```

### resolve

```json
{
  "action": "resolve",
  "server": "",
  "strategy": "",
  "disable_cache": false,
  "rewrite_ttl": null,
  "client_subnet": null
}
```

`resolve` resolve request destination from domain to IP addresses.

#### server

Specifies DNS server tag to use instead of selecting through DNS routing.

#### strategy

DNS resolution strategy, available values are: `prefer_ipv4`, `prefer_ipv6`, `ipv4_only`, `ipv6_only`.

`dns.strategy` will be used by default.

#### disable_cache

!!! question "Since sing-box 1.12.0"

Disable cache and save cache in this query.

#### rewrite_ttl

!!! question "Since sing-box 1.12.0"

Rewrite TTL in DNS responses.

#### client_subnet

!!! question "Since sing-box 1.12.0"

Append a `edns0-subnet` OPT extra record with the specified IP prefix to every query by default.

If value is an IP address instead of prefix, `/32` or `/128` will be appended automatically.

Will overrides `dns.client_subnet`.
