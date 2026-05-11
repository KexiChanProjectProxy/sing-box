---
icon: material/alert-decagram
---

!!! quote "sing-box 1.11.0 中的更改"

    :material-delete-clock: [override_address](#override_address)  
    :material-delete-clock: [override_port](#override_port)

`direct` 出站直接发送请求。

### 结构

```json
{
  "type": "direct",
  "tag": "direct-out",

  "override_address": "1.0.0.1",
  "override_port": 53,
  "use_origin_dst": false,
  "xlat464_prefix": "",

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

#### use_origin_dst

使用通过 `SO_ORIGINAL_DST`（Linux 透明代理）获取的原始目标地址，而非连接的表观目标地址。

当连接被 iptables/nftables REDIRECT 或 TPROXY 重定向时使用，使 direct 出站转发至重定向前的原始目标，而非本地监听地址。

#### xlat464_prefix

464XLAT（CLAT）前缀，用于合成 IPv4 映射的 IPv6 地址（RFC 6052）。必须为 `/96` 前缀。

设置后，IPv4 目标地址将通过嵌入此前缀转换为 IPv6 地址，从而在纯 IPv6 网络上实现 IPv4 连接。

示例：`"64:ff9b::/96"`（众所周知的 NAT64 前缀）

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
