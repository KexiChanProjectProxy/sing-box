---
icon: material/alert-decagram
---

# 路由

!!! quote "sing-box 1.12.0 中的更改"

    :material-plus: [default_domain_resolver](#default_domain_resolver)  
    :material-note-remove: [geoip](#geoip)  
    :material-note-remove: [geosite](#geosite)

!!! quote "sing-box 1.11.0 中的更改"

    :material-plus: [default_network_strategy](#default_network_strategy)  
    :material-plus: [default_network_type](#default_network_type)  
    :material-plus: [default_fallback_network_type](#default_fallback_network_type)  
    :material-plus: [default_fallback_delay](#default_fallback_delay)

!!! quote "sing-box 1.8.0 中的更改"

    :material-plus: [rule_set](#rule_set)  
    :material-delete-clock: [geoip](#geoip)  
    :material-delete-clock: [geosite](#geosite)

### 结构

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
    "default_domain_resolver": "", // 或 {}
    "default_network_strategy": "",
    "default_network_type": [],
    "default_fallback_network_type": [],
    "default_fallback_delay": "",
    "default_tcp_keep_alive": "",
    "default_tcp_keep_alive_interval": "",
    "sniff_override_destination": false,
    "asn": {},

    // 已移除

    "geoip": {},
    "geosite": {}
  }
}
```

!!! note ""

    当内容只有一项时，可以忽略 JSON 数组 [] 标签

### 字段

| 键         | 格式                    |
|-----------|-----------------------|
| `geoip`   | [GeoIP](./geoip/)     |
| `geosite` | [Geosite](./geosite/) |

#### rule

一组 [路由规则](./rule/)    。

#### rule_set

!!! question "自 sing-box 1.8.0 起"

一组 [规则集](/zh/configuration/rule-set/)。

#### final

默认出站标签。如果为空，将使用第一个可用于对应协议的出站。

#### auto_detect_interface

!!! quote ""

    仅支持 Linux、Windows 和 macOS。

默认将出站连接绑定到默认网卡，以防止在 tun 下出现路由环路。

如果设置了 `outbound.bind_interface` 设置，则不生效。

#### override_android_vpn

!!! quote ""

    仅支持 Android。

启用 `auto_detect_interface` 时接受 Android VPN 作为上游网卡。

#### default_interface

!!! quote ""

    仅支持 Linux、Windows 和 macOS。

默认将出站连接绑定到指定网卡，以防止在 tun 下出现路由环路。

如果设置了 `auto_detect_interface` 设置，则不生效。

#### default_mark

!!! quote ""

    仅支持 Linux。

默认为出站连接设置路由标记。

如果设置了 `outbound.routing_mark` 设置，则不生效。

#### default_domain_resolver

!!! question "自 sing-box 1.12.0 起"

详情参阅 [拨号字段](/zh/configuration/shared/dial/#domain_resolver)。

可以被 `outbound.domain_resolver` 覆盖。

#### network_strategy

!!! question "自 sing-box 1.11.0 起"

详情参阅 [拨号字段](/zh/configuration/shared/dial/#network_strategy)。

当 `outbound.bind_interface`, `outbound.inet4_bind_address` 或 `outbound.inet6_bind_address` 已设置时不生效。

可以被 `outbound.network_strategy` 覆盖。

与 `default_interface` 冲突。

#### default_network_type

!!! question "自 sing-box 1.11.0 起"

详情参阅 [拨号字段](/zh/configuration/shared/dial/#default_network_type)。

#### default_fallback_network_type

!!! question "自 sing-box 1.11.0 起"

详情参阅 [拨号字段](/zh/configuration/shared/dial/#default_fallback_network_type)。

#### default_fallback_delay

!!! question "自 sing-box 1.11.0 起"

#### default_fallback_delay

!!! question "自 sing-box 1.11.0 起"

详情参阅 [拨号字段](/zh/configuration/shared/dial/#fallback_delay)。

#### default_tcp_keep_alive

出站连接的默认 TCP 保活初始周期。默认值：`5m`。

可被 `outbound` 拨号设置覆盖。

#### default_tcp_keep_alive_interval

出站连接的默认 TCP 保活间隔。默认值：`75s`。

可被 `outbound` 拨号设置覆盖。

#### sniff_override_destination

全局默认值：当 `sniff` 规则动作成功从连接中检测到域名时（例如 TLS SNI、HTTP `Host` 头），将连接的 IP 目标地址替换为嗅探到的域名，端口保持不变。

此功能使后续的 DNS 解析和路由决策使用域名而非原始 IP 地址，适用于以下场景：

- 在接收到 IP 连接后（例如来自透明代理）进行精确的基于域名的路由
- 由出站进行远端 DNS 解析，而非依赖客户端已解析的 IP

**推荐方式——按规则配置 `override_destination`：**

使用各 `sniff` 规则动作上的 `override_destination` 字段，可在不影响所有规则的情况下选择性地启用覆盖：

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

详情参阅 [`sniff` 动作](/zh/configuration/route/rule_action/#sniff)。

**全局回退方式：**

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

覆盖仅在成功嗅探到域名时生效。如果嗅探失败或连接中没有可识别的域名（例如纯 IP 流量），目标地址保持不变。

也可通过已废弃的 `inbound.sniff_override_destination` 选项在各入站中单独设置。

#### asn

ASN 数据库配置。[LoadBalance](/zh/configuration/outbound/loadbalance/) 哈希键部分的 `dst_asn` 及 `ip_asn` 路由规则需要此配置。

| 字段 | 描述 |
|------|------|
| `path` | ASN MMDB 文件路径 |
| `download_url` | 自动下载数据库的 URL |
| `download_detour` | 下载数据库所使用的出站标签 |

支持 MaxMind GeoLite2-ASN 格式（MMDB）。
