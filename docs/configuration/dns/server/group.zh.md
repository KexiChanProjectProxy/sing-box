---
icon: material/new-box
---

!!! question "自 sing-box 1.14.2.11 起"

# 组

把查询分散到其他 DNS 服务器，全部失败时再尝试备用服务器。

### 结构

```json
{
  "dns": {
    "servers": [
      {
        "type": "group",
        "tag": "",

        "servers": [],
        "fallbacks": [],
        "mode": ""
      }
    ]
  }
}
```

### 字段

#### servers

==必填==

组内 DNS 服务器的标签。

只接受以下类型：[`udp`](../udp/)、[`tcp`](../tcp/)、[`tls`](../tls/) 和 [`https`](../https/)。
各服务器保留自己的选项，例如 `detour`、`domain_resolver` 和 TLS。

#### fallbacks

`servers` 中所有服务器都失败后，按顺序尝试的 DNS 服务器标签。

`servers` 中的服务器只使用查询时间中属于它们的份额，其余留给备用服务器，因此完全不应答的服务器也不会耗尽尝试备用服务器的时间。
`parallel` 模式下同时询问所有服务器计为一次尝试。

接受的类型与 `servers` 相同。

#### mode

查询在 `servers` 之间的分配方式。

| 模式                     | 行为 |
|--------------------------|------|
| `load_balance`（默认） | 按随机顺序依次尝试，较快的服务器更可能排在前面，权重为近期往返时间的倒数。失败的服务器在再次应答前按慢服务器计算。尚未测量的服务器会较早被尝试。 |
| `parallel`               | 同时询问所有服务器，采用最先到达的应答。 |
| `sequential`             | 按列出的顺序依次尝试。 |

依次尝试时，每个服务器分得剩余时间的相同份额，因此不应答的服务器不会耗尽整个查询的时间。

服务器失败（`SERVFAIL`）或拒绝（`REFUSED`）应答与错误一样会转到下一个服务器，并在负载均衡中计为失败。
其他应答（例如 `NXDOMAIN`）即为最终结果。

### 缓存

组作为一个 DNS 服务器，以自己的标签缓存。
发往成员的查询不会再次经过 DNS 缓存。

发往该组的所有查询共享此缓存，无论来自 DNS 规则、sing-box 自身的域名解析，还是 [`dns.server`](/zh/configuration/inbound/anytls/#dnsserver) 为该组的 AnyTLS 入站的客户端。
与其他查询一样，设置了 DNSSEC OK 位的查询不会被缓存。

### 示例

```json
{
  "dns": {
    "servers": [
      { "type": "tls", "tag": "cloudflare", "server": "1.1.1.1", "detour": "proxy" },
      { "type": "https", "tag": "google", "server": "8.8.8.8", "detour": "proxy" },
      { "type": "udp", "tag": "local-isp", "server": "223.5.5.5" },
      {
        "type": "group",
        "tag": "upstream",
        "servers": ["cloudflare", "google"],
        "fallbacks": ["local-isp"],
        "mode": "parallel"
      }
    ],
    "final": "upstream",
    "optimistic": { "enabled": true }
  },
  "inbounds": [
    {
      "type": "anytls",
      "tag": "anytls-in",
      ...
      "dns": { "enabled": true, "server": "upstream" }
    }
  ]
}
```
