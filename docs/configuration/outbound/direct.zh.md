---
icon: material/alert-decagram
---

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
    "prefix": "64:ff9b::/96"
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
    "prefix": "64:ff9b::/96"
  }
}
```

!!! info "契约"

    - `prefix` 必须是长度为 `/96` 的 IPv6 前缀。其他前缀长度将被拒绝。
    - IPv4 字面目标地址和 A 记录应答会被嵌入前缀（例如 `192.0.2.1` 变为 `64:ff9b::c000:201`）。
    - direct 出站的域名解析强制仅查询 A 记录（IPv4）。由前置路由 `resolve` 动作提供的 AAAA 应答会被丢弃。
    - 支持 TCP 和 UDP 协议。
    - 原生 IPv6 字面目标地址（非映射地址）保持不变。
    - 不支持 ICMP、DNS64、自动前缀发现和非 `/96` 前缀长度。
    - 此选项仅适用于 direct 出站，不适用于其他出站类型。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
