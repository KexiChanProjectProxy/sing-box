---
icon: material/alert-decagram
---

!!! quote "sing-box 1.14.0.17 中的更改"

    :material-plus: [non_local_bind](#non_local_bind)  
    :material-plus: [source_bind](#source_bind)

!!! quote "sing-box 1.14.0 中的更改"

    :material-plus: [xlat464](#xlat464)

!!! quote "sing-box 1.11.0 中的更改"

    :material-delete-clock: [override_address](#override_address)  
    :material-delete-clock: [override_port](#override_port)

`direct` 出站直接发送请求。

### 结构

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

  ... // 拨号字段
}
```

### 字段

#### override_address

!!! failure "已在 sing-box 1.11.0 废弃"

    目标覆盖字段在 sing-box 1.11.0 中已废弃，并将在 sing-box 1.13.0 中被移除，参阅 [迁移指南](/zh/migration/#迁移-direct-出站中的目标地址覆盖字段到路由字段)。

覆盖连接目标地址。

#### override_port

!!! failure "已在 sing-box 1.11.0 废弃"

    目标覆盖字段在 sing-box 1.11.0 中已废弃，并将在 sing-box 1.13.0 中被移除，参阅 [迁移指南](/zh/migration/#迁移-direct-出站中的目标地址覆盖字段到路由字段)。

覆盖连接目标端口。

#### xlat464

XLAT464（NAT64）地址转换，用于 direct 出站。配置后，IPv4 字面目标地址和从域名解析获得的 IPv4 地址（A 记录）会被嵌入指定的 `/96` IPv6 前缀中，使得仅支持 IPv4 的目标地址可以通过纯 IPv6 网络路径访问。

```json
{
  "xlat464": {
    "prefix": "64:ff9b::/96",
    "allow_ipv6": false
  }
}
```

!!! info "契约"

    - `prefix` 必须是长度为 `/96` 的 IPv6 前缀。其他前缀长度将被拒绝。
    - IPv4 字面目标地址和 A 记录应答会被嵌入前缀（例如 `192.0.2.1` 变为 `64:ff9b::c000:201`）。
    - direct 出站的域名解析强制仅查询 A 记录（IPv4）。由前置路由 `resolve` 动作提供的 AAAA 应答会被丢弃。
    - 支持 TCP 和 UDP 协议。
    - 默认拒绝配置前缀之外的原生 IPv6 字面目标地址，防止其绕过 NAT64 路径。仅当显式设置 `allow_ipv6` 为 `true` 时才允许原生 IPv6 直连。
    - 不支持 ICMP、DNS64、自动前缀发现和非 `/96` 前缀长度。
    - 此选项仅适用于 direct 出站，不适用于其他出站类型。

#### non_local_bind

!!! question "自 sing-box 1.14.0.17 起"

!!! quote ""

    仅支持 Linux 和 FreeBSD。

允许套接字绑定到未分配给任何本地接口的地址。

在 Linux 上设置 `IP_FREEBIND`（以及 `IPV6_FREEBIND`）；在 FreeBSD 上设置 `IP_BINDANY` / `IPV6_BINDANY`，需要 root 或 `PRIV_NETINET_BINDANY` 权限。

作用于此出站的所有套接字，包括 [`inet4_bind_address`](/zh/configuration/shared/dial/#inet4_bind_address)、[`inet6_bind_address`](/zh/configuration/shared/dial/#inet6_bind_address) 以及 [`source_bind`](#source_bind) 的地址。

借此可以把整段路由前缀用作出口地址，而无需逐个把地址配置到接口上。上游网络需要把该前缀路由到本机，本机也需要把它视为本地地址：

```shell
ip -6 route add local 2001:db8:1::/64 dev lo
```

#### source_bind

!!! question "自 sing-box 1.14.0.17 起"

按客户端源 IP 选择每个连接的本地绑定地址。

存在此对象即启用。每个客户端源地址会被分配一个 IPv4 和一个 IPv6 绑定地址：

1. 使用第一条 `source_ip_cidr` 包含该客户端地址的规则。
2. 未匹配任何规则的客户端使用顶层的 `inet4_addresses` 与 `inet6_addresses`。
3. 未配置地址的地址族沿用 [拨号字段](/zh/configuration/shared/dial/) 的绑定，即 `inet4_bind_address` / `inet6_bind_address` 或系统默认。没有客户端来源的连接（例如 sing-box 自身发出的 DNS 查询）始终使用拨号字段的绑定。

单个地址即固定映射。配置前缀或多个条目时，为每个客户端随机选取一个地址，按前缀大小加权。`/30` 及更短的 IPv4 前缀不会选到网络地址和广播地址；`/126` 及更短的 IPv6 前缀不会选到全零地址。

只要客户端新建连接的间隔小于 [`ttl`](#source_bindttl)，就保持所选地址。超过 `ttl` 没有新连接后，该客户端的下一个连接会重新随机分配地址。已建立的连接保持原地址。分配结果仅保存在内存中，重启后不保留。

未分配到本地接口的地址需要启用 [`non_local_bind`](#non_local_bind)。

与 `network_strategy`、`network_type` 和 `fallback_network_type` 冲突。与 `inet4_bind_address` 相同，会为此出站禁用 `route.auto_detect_interface` 和 `route.default_interface`。

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

分配给未匹配任何规则的客户端的 IPv4 地址或前缀。

#### source_bind.inet6_addresses

分配给未匹配任何规则的客户端的 IPv6 地址或前缀。

#### source_bind.ttl

客户端最后一次新建连接后，其随机地址的保留时长。

默认使用 `1h`。

#### source_bind.rules

按顺序匹配的规则。每条规则需要 `source_ip_cidr`，以及 `inet4_addresses` 与 `inet6_addresses` 中的至少一项。

#### source_bind.rules.source_ip_cidr

此规则匹配的客户端源地址或前缀。IPv4 映射的 IPv6 客户端地址会匹配 IPv4 条目。

#### source_bind.rules.inet4_addresses

分配给匹配此规则的客户端的 IPv4 地址或前缀。

#### source_bind.rules.inet6_addresses

分配给匹配此规则的客户端的 IPv6 地址或前缀。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
