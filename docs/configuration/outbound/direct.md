---
icon: material/alert-decagram
---

!!! quote "Changes in sing-box 1.14.0.17"

    :material-plus: [non_local_bind](#non_local_bind)  
    :material-plus: [source_bind](#source_bind)

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

  "non_local_bind": false,
  "source_bind": {
    "inet4_addresses": [],
    "inet6_addresses": [],
    "ttl": "1h",
    "rules": [
      {
        "source_ip_cidr": [],
        "inet4_addresses": [],
        "inet6_addresses": []
      }
    ]
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

#### non_local_bind

!!! question "Since sing-box 1.14.0.17"

!!! quote ""

    Only supported on Linux and FreeBSD.

Allow sockets to bind to addresses that are not assigned to any local interface.

Sets `IP_FREEBIND` (and `IPV6_FREEBIND`) on Linux, and `IP_BINDANY` / `IPV6_BINDANY` on FreeBSD, which requires root or the `PRIV_NETINET_BINDANY` privilege.

It applies to every socket of this outbound, including [`inet4_bind_address`](/configuration/shared/dial/#inet4_bind_address), [`inet6_bind_address`](/configuration/shared/dial/#inet6_bind_address) and [`source_bind`](#source_bind) addresses.

This is how a whole routed prefix is used as egress addresses without assigning each address to an interface. The prefix must be routed to the host by the upstream network, and the host must accept it as local:

```shell
ip -6 route add local 2001:db8:1::/64 dev lo
```

#### source_bind

!!! question "Since sing-box 1.14.0.17"

Choose the local bind address of each connection by the client's source IP.

Enabled when this object is present. Each client source address is assigned one IPv4 and one IPv6 bind address:

1. The first rule whose `source_ip_cidr` contains the client address is used.
2. Clients matching no rule use the top-level `inet4_addresses` and `inet6_addresses`.
3. A family with no configured address keeps the [dial fields](/configuration/shared/dial/) binding, i.e. `inet4_bind_address` / `inet6_bind_address` or the system default. Connections without a client source, such as DNS queries made by sing-box itself, always use the dial fields binding.

A single address is a fixed mapping. When a prefix or several entries are given, a random address is picked for each client, weighted by prefix size. For IPv4 prefixes of `/30` or shorter, the network and broadcast addresses are never picked. For IPv6 prefixes of `/126` or shorter, the all-zero address is never picked.

The picked address is kept while the client keeps opening new connections less than [`ttl`](#source_bindttl) apart. After `ttl` without a new connection the client gets a fresh random address on its next connection. Established connections keep their address. Assignments are kept in memory and are not preserved across restarts.

Addresses that are not assigned to a local interface need [`non_local_bind`](#non_local_bind).

Conflicts with `network_strategy`, `network_type` and `fallback_network_type`. Like `inet4_bind_address`, it disables `route.auto_detect_interface` and `route.default_interface` for this outbound.

```json
{
  "non_local_bind": true,
  "source_bind": {
    "inet4_addresses": ["203.0.113.0/28", "198.51.100.7"],
    "inet6_addresses": "2001:db8:1::/64",
    "ttl": "1h",
    "rules": [
      {
        "source_ip_cidr": ["10.0.1.0/24", "10.0.2.5"],
        "inet4_addresses": "203.0.113.9",
        "inet6_addresses": "2001:db8:1::9/120"
      }
    ]
  }
}
```

#### source_bind.inet4_addresses

IPv4 addresses or prefixes assigned to clients that match no rule.

#### source_bind.inet6_addresses

IPv6 addresses or prefixes assigned to clients that match no rule.

#### source_bind.ttl

How long a client keeps its random address after its last new connection.

`1h` is used by default.

#### source_bind.rules

Rules tried in order. Each rule needs `source_ip_cidr` and at least one of `inet4_addresses` and `inet6_addresses`.

#### source_bind.rules.source_ip_cidr

Client source addresses or prefixes matched by this rule. IPv4-mapped IPv6 client addresses match IPv4 entries.

#### source_bind.rules.inet4_addresses

IPv4 addresses or prefixes assigned to clients matching this rule.

#### source_bind.rules.inet6_addresses

IPv6 addresses or prefixes assigned to clients matching this rule.

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
