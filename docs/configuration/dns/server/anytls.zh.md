---
icon: material/new-box
---

!!! question "自 sing-box 1.14.2.11 起"

# AnyTLS

通过 [AnyTLS 出站](/zh/configuration/outbound/anytls/) 的会话把查询发往其服务器。
服务器用自己的 DNS 应答。

### 结构

```json
{
  "dns": {
    "servers": [
      {
        "type": "anytls",
        "tag": "",

        "outbound": "anytls-out"
      }
    ]
  }
}
```

### 字段

#### outbound

==必填==

[AnyTLS 出站](/zh/configuration/outbound/anytls/) 的标签。

不接受选择器或其他出站组。

### 工作方式

每个查询作为原始 DNS 报文放进 AnyTLS 会话内的独立帧发送，不打开流。
查询走延迟最低的已打开会话，没有会话时会新建一个。
会话在查询途中关闭时，会在另一个会话上重试一次。

服务器必须在其 AnyTLS 入站上启用 [`dns`](/zh/configuration/inbound/anytls/#dns)。
其他服务器不提供此功能，查询会立即失败并给出说明，代理连接不受影响。

!!! warning

    不要通过 `domain_resolver` 或 `route.default_domain_resolver` 用此 DNS 服务器解析该 AnyTLS 出站的服务器地址。
    否则出站需要此服务器才能连接，而此服务器又需要该出站。
