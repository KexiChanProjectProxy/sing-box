---
icon: material/alert-decagram
---

!!! quote "sing-box 1.11.0 中的更改"

    :material-alert-decagram: [override_address](#override_address)  
    :material-alert-decagram: [override_port](#override_port)

`direct` 出站直接发送请求。

### 结构

```json
{
  "type": "direct",
  "tag": "direct-out",

  "override_address": "1.0.0.1",
  "override_port": 53,
  "xlat464_prefix": "64:ff9b::/96",

  ... // 拨号字段
}
```

### 字段

#### override_address

!!! failure "已在 sing-box 1.11.0 废弃"

    目标覆盖字段在 sing-box 1.11.0 中已废弃，并将在 sing-box 1.13.0 中被移除，参阅 [迁移指南](/migration/#migrate-destination-override-fields-to-route-options)。

覆盖连接目标地址。

#### override_port

!!! failure "已在 sing-box 1.11.0 废弃"

    目标覆盖字段在 sing-box 1.11.0 中已废弃，并将在 sing-box 1.13.0 中被移除，参阅 [迁移指南](/migration/#migrate-destination-override-fields-to-route-options)。

覆盖连接目标端口。

#### xlat464_prefix

用于 464XLAT（CLAT）转换的 IPv6 前缀。根据 RFC 6052 规范，必须是 /96 前缀。

配置后，IPv4 地址将通过在前缀的最后 32 位嵌入 IPv4 地址来转换为 IPv6。

**要求：**
- 您的网络必须支持 464XLAT（存在 PLAT）
- 前缀必须从网络运营商获取
- 建议配合 `domain_strategy: ipv4_only` 使用以确保正常工作

**转换示例：**
- IPv4 地址：`104.16.184.241`
- 前缀：`64:ff9b::/96`
- 结果：`64:ff9b::6810:b8f1`

**配置示例：**

```json
{
  "type": "direct",
  "tag": "direct-464xlat",
  "domain_strategy": "ipv4_only",
  "xlat464_prefix": "64:ff9b::/96"
}
```

**常用前缀：**
- 知名前缀（RFC 6052）：`64:ff9b::/96`
- 运营商特定前缀因网络运营商而异

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
