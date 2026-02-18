---
icon: material/new-box
---

!!! quote "sing-box 1.12.0 中的更改"

    :material-plus: [strategy](#strategy)  
    :material-plus: [predefined](#predefined)

!!! question "自 sing-box 1.11.0 起"

### route

```json
{
  "action": "route", // 默认
  "server": "",
  "strategy": "",
  "disable_cache": false,
  "rewrite_ttl": null,
  "client_subnet": null,
  "resolve_retries": 0,
  "resolve_timeout": "",
  "hold_valid": "",
  "hold_nx": "",
  "hold_refused": "",
  "hold_other": "",
  "hold_timeout": ""
}
```

`route` 继承了将 DNS 请求 路由到指定服务器的经典规则动作。

#### server

==必填==

目标 DNS 服务器的标签。

#### strategy

!!! question "自 sing-box 1.12.0 起"

为此查询设置域名策略。

可选项：`prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`。

#### disable_cache

在此查询中禁用缓存。

#### rewrite_ttl

重写 DNS 回应中的 TTL。

#### client_subnet

默认情况下，将带有指定 IP 前缀的 `edns0-subnet` OPT 附加记录附加到每个查询。

如果值是 IP 地址而不是前缀，则会自动附加 `/32` 或 `/128`。

将覆盖 `dns.client_subnet`.

#### resolve_retries

DNS 查询重试次数。

#### resolve_timeout

每次 DNS 查询重试的超时时间。

#### hold_valid

!!! question "HAProxy 风格解析器"

成功 DNS 回应（包含答案）的 advertised TTL。

在此持续时间后，响应将立即返回（stale-while-revalidate），同时触发后台刷新。

缓存生命周期为 `hold_valid * 2`（至少 `hold_valid + 30s`）。

#### hold_nx

!!! question "HAProxy 风格解析器"

NXDOMAIN（域名不存在）响应的 advertised TTL。

#### hold_refused

!!! question "HAProxy 风格解析器"

REFUSED DNS 响应的 advertised TTL。

#### hold_other

!!! question "HAProxy 风格解析器"

其他 DNS 错误响应（例如 SERVFAIL）的 advertised TTL。

#### hold_timeout

!!! question "HAProxy 风格解析器"

当 DNS 查询超时时，回退到过时的缓存响应。

如果设置，将在超时时返回最后成功的缓存响应，而不是直接失败。

### route-options

```json
{
  "action": "route-options",
  "disable_cache": false,
  "rewrite_ttl": null,
  "client_subnet": null
}
```

`route-options` 为路由设置选项。

### reject

```json
{
  "action": "reject",
  "method": "",
  "no_drop": false
}
```

`reject` 拒绝 DNS 请求。

#### method

- `default`: 返回 REFUSED。
- `drop`: 丢弃请求。

默认使用 `defualt`。

#### no_drop

如果未启用，则 30 秒内触发 50 次后，`method` 将被暂时覆盖为 `drop`。

当 `method` 设为 `drop` 时不可用。

### predefined

!!! question "自 sing-box 1.12.0 起"

```json
{
  "action": "predefined",
  "rcode": "",
  "answer": [],
  "ns": [],
  "extra": []
}
```

`predefined` 以预定义的 DNS 记录响应。

#### rcode

响应码。

| 值          | 旧 rcode DNS 服务器中的值 | 描述              |
|------------|--------------------|-----------------|
| `NOERROR`  | `success`          | Ok              |
| `FORMERR`  | `format_error`     | Bad request     |
| `SERVFAIL` | `server_failure`   | Server failure  |
| `NXDOMAIN` | `name_error`       | Not found       |
| `NOTIMP`   | `not_implemented`  | Not implemented |
| `REFUSED`  | `refused`          | Refused         |

默认使用 `NOERROR`。

#### answer

用于作为回答响应的文本 DNS 记录列表。

例子:

| 记录类型   | 例子                            |
|--------|-------------------------------|
| `A`    | `localhost. IN A 127.0.0.1`   |
| `AAAA` | `localhost. IN AAAA ::1`      |
| `TXT`  | `localhost. IN TXT \"Hello\"` |

#### ns

用于作为名称服务器响应的文本 DNS 记录列表。

#### extra

用于作为额外记录响应的文本 DNS 记录列表。
