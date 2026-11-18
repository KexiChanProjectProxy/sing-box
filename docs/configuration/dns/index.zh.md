---
icon: material/alert-decagram
---

!!! quote "sing-box 1.14.2.11 中的更改"

    :material-plus: [min_ttl](#min_ttl)  
    :material-plus: [max_ttl](#max_ttl)  
    :material-plus: [prefetch](#prefetch)  
    :material-alert: [optimistic](#optimistic)  
    :material-alert: [cache_capacity](#cache_capacity)

!!! quote "sing-box 1.14.0 中的更改"

    :material-delete-clock: [independent_cache](#independent_cache)  
    :material-plus: [optimistic](#optimistic)  
    :material-plus: [timeout](#timeout)

!!! quote "sing-box 1.12.0 中的更改"

    :material-decagram: [servers](#servers)

!!! quote "sing-box 1.11.0 中的更改"

    :material-plus: [cache_capacity](#cache_capacity)

# DNS

### 结构

```json
{
  "dns": {
    "servers": [],
    "rules": [],
    "final": "",
    "strategy": "",
    "disable_cache": false,
    "disable_expire": false,
    "independent_cache": false,
    "cache_capacity": 0,
    "min_ttl": 0,
    "max_ttl": 0,
    "optimistic": false, // or {}
    "prefetch": false,
    "timeout": "",
    "reverse_mapping": false,
    "client_subnet": "",
    "fakeip": {}
  }
}

```

### 字段

| 键        | 格式                      |
|----------|-------------------------|
| `server` | 一组 [DNS 服务器](./server/) |
| `rules`  | 一组 [DNS 规则](./rule/)    |

#### final

默认 DNS 服务器的标签。

默认使用第一个服务器。

#### strategy

默认解析域名策略。

可选值: `prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`。

#### disable_cache

禁用 DNS 缓存。

与 `optimistic` 和 `prefetch` 冲突。

#### disable_expire

禁用 DNS 缓存过期。

与 `optimistic` 和 `prefetch` 冲突。

#### independent_cache

!!! failure "已在 sing-box 1.14.0 废弃"

    `independent_cache` 已在 sing-box 1.14.0 废弃，且将在 sing-box 1.16.0 中被移除，参阅[迁移指南](/zh/migration/#迁移-independent-dns-cache)。

使每个 DNS 服务器的缓存独立，以满足特殊目的。如果启用，将轻微降低性能。

#### cache_capacity

!!! question "自 sing-box 1.11.0 起"

LRU 缓存容量。

为空或为 `0` 时使用 `1024`。

!!! note ""

    在 sing-box 1.14.2.11 之前，小于 1024 的值将被忽略。

#### min_ttl

!!! question "自 sing-box 1.14.2.11 起"

将低于此值的 DNS 响应 TTL 提高到此值，单位为秒。

返回给客户端的 TTL 与响应在缓存中保留的时间都使用提高后的值。
TTL 为 `0` 的响应保持不变，且不会被缓存。

对规则设置了 `rewrite_ttl` 的查询无效。

可以被 `rules.[].min_ttl` 覆盖。

#### max_ttl

!!! question "自 sing-box 1.14.2.11 起"

将高于此值的 DNS 响应 TTL 降低到此值，单位为秒。

不得小于 `min_ttl`。

对规则设置了 `rewrite_ttl` 的查询无效。

可以被 `rules.[].max_ttl` 覆盖。与规则设置的界限冲突的全局界限在该查询中被忽略。

#### optimistic

!!! question "自 sing-box 1.14.0 起"

启用乐观 DNS 缓存。当缓存的 DNS 条目已过期但仍在超时窗口内时，
立即返回过期的响应，同时在后台触发刷新。

与 `disable_cache` 和 `disable_expire` 冲突。

接受布尔值或对象。当设置为 `true` 时，使用默认超时 `3d`。

```json
{
  "enabled": true,
  "timeout": "3d",
  "client_timeout": ""
}
```

可以被 `rules.[].optimistic` 覆盖，被 `rules.[].disable_optimistic_cache` 禁用。
即使此处未启用 `optimistic`，规则仍可为其匹配的查询启用。

##### enabled

启用乐观 DNS 缓存。

##### timeout

过期缓存条目可被乐观提供的最长时间。

默认使用 `3d`。

##### client_timeout

!!! question "自 sing-box 1.14.2.11 起"

命中过期缓存条目的查询等待刷新的时间。

如果上游及时回应，则返回新的响应；否则，或者刷新失败时，返回过期的响应，
刷新在后台继续，与 [RFC 8767](https://www.rfc-editor.org/rfc/rfc8767) 一致。

默认立即返回过期的响应。

#### prefetch

!!! question "自 sing-box 1.14.2.11 起"

当命中的缓存条目剩余 TTL 不足其 TTL 的十分之一时，在后台刷新该条目，
使频繁查询的域名在过期前得到刷新。

与 `disable_cache` 和 `disable_expire` 冲突。

可以被 `rules.[].prefetch` 覆盖。

#### timeout

!!! question "自 sing-box 1.14.0 起"

每次 DNS 查询的默认超时时间。

默认使用 `10s`。

可被 `rules.[].timeout`（DNS 规则动作）或 `domain_resolver.timeout` 覆盖。

#### reverse_mapping

在响应 DNS 查询后存储 IP 地址的反向映射以为路由目的提供域名。

由于此过程依赖于应用程序在发出请求之前解析域名的行为，因此在 macOS 等 DNS 由系统代理和缓存的环境中可能会出现问题。

#### client_subnet

!!! question "自 sing-box 1.9.0 起"

默认情况下，将带有指定 IP 前缀的 `edns0-subnet` OPT 附加记录附加到每个查询。

如果值是 IP 地址而不是前缀，则会自动附加 `/32` 或 `/128`。

可以被 `servers.[].client_subnet` 或 `rules.[].client_subnet` 覆盖。

#### fakeip :material-note-remove:

[FakeIP](./fakeip/) 设置。
