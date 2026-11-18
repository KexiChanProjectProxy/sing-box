---
icon: material/alert-decagram
---

!!! quote "Changes in sing-box 1.14.2.11"

    :material-plus: [min_ttl](#min_ttl)  
    :material-plus: [max_ttl](#max_ttl)  
    :material-plus: [prefetch](#prefetch)  
    :material-alert: [optimistic](#optimistic)  
    :material-alert: [cache_capacity](#cache_capacity)

!!! quote "Changes in sing-box 1.14.0"

    :material-delete-clock: [independent_cache](#independent_cache)  
    :material-plus: [optimistic](#optimistic)  
    :material-plus: [timeout](#timeout)

!!! quote "Changes in sing-box 1.12.0"

    :material-decagram: [servers](#servers)

!!! quote "Changes in sing-box 1.11.0"

    :material-plus: [cache_capacity](#cache_capacity)

# DNS

### Structure

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

### Fields

| Key      | Format                          |
|----------|---------------------------------|
| `server` | List of [DNS Server](./server/) |
| `rules`  | List of [DNS Rule](./rule/)     |
| `fakeip` | :material-note-remove: [FakeIP](./fakeip/) |

#### final

Default dns server tag.

The first server will be used if empty.

#### strategy

Default domain strategy for resolving the domain names.

One of `prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`.

#### disable_cache

Disable dns cache.

Conflict with `optimistic` and `prefetch`.

#### disable_expire

Disable dns cache expire.

Conflict with `optimistic` and `prefetch`.

#### independent_cache

!!! failure "Deprecated in sing-box 1.14.0"

    `independent_cache` is deprecated and will be removed in sing-box 1.14.0, check [Migration](/migration/#migrate-independent-dns-cache).

Make each DNS server's cache independent for special purposes. If enabled, will slightly degrade performance.

#### cache_capacity

!!! question "Since sing-box 1.11.0"

LRU cache capacity.

`1024` is used when empty or `0`.

!!! note ""

    Before sing-box 1.14.2.11, values less than 1024 were ignored.

#### min_ttl

!!! question "Since sing-box 1.14.2.11"

Raise the TTL of DNS responses to this value when it is lower, in seconds.

The TTL returned to clients and the time the response stays in the cache both use the raised value.
Responses with a TTL of `0` are left as they are and are not cached.

Has no effect on queries whose rule sets `rewrite_ttl`.

Can be overridden by `rules.[].min_ttl`.

#### max_ttl

!!! question "Since sing-box 1.14.2.11"

Lower the TTL of DNS responses to this value when it is higher, in seconds.

Must not be less than `min_ttl`.

Has no effect on queries whose rule sets `rewrite_ttl`.

Can be overridden by `rules.[].max_ttl`. A global bound that conflicts with a bound set by the rule is ignored for that query.

#### optimistic

!!! question "Since sing-box 1.14.0"

Enable optimistic DNS caching. When a cached DNS entry has expired but is still within the timeout window,
the stale response is returned immediately while a background refresh is triggered.

Conflict with `disable_cache` and `disable_expire`.

Accepts a boolean or an object. When set to `true`, the default timeout of `3d` is used.

```json
{
  "enabled": true,
  "timeout": "3d",
  "client_timeout": ""
}
```

Can be overridden by `rules.[].optimistic` and disabled by `rules.[].disable_optimistic_cache`.
When `optimistic` is not enabled here, rules can still enable it for the queries they match.

##### enabled

Enable optimistic DNS caching.

##### timeout

The maximum time an expired cache entry can be served optimistically.

`3d` is used by default.

##### client_timeout

!!! question "Since sing-box 1.14.2.11"

How long a query that hits an expired cache entry waits for the refresh.

If the upstream answers in time, the fresh response is returned. Otherwise, or if the refresh fails,
the expired response is returned and the refresh continues in the background, as in
[RFC 8767](https://www.rfc-editor.org/rfc/rfc8767).

The expired response is returned immediately by default.

#### prefetch

!!! question "Since sing-box 1.14.2.11"

Refresh a cached entry in the background when it is hit with less than a tenth of its TTL left,
so that frequently queried domains are refreshed before they expire.

Conflict with `disable_cache` and `disable_expire`.

Can be overridden by `rules.[].prefetch`.

#### timeout

!!! question "Since sing-box 1.14.0"

Default timeout for each DNS query.

`10s` is used by default.

Can be overridden by `rules.[].timeout` (DNS rule action) or `domain_resolver.timeout`.

#### reverse_mapping

Stores a reverse mapping of IP addresses after responding to a DNS query in order to provide domain names when routing.

Since this process relies on the act of resolving domain names by an application before making a request, it can be
problematic in environments such as macOS, where DNS is proxied and cached by the system.

#### client_subnet

!!! question "Since sing-box 1.9.0"

Append a `edns0-subnet` OPT extra record with the specified IP prefix to every query by default.

If value is an IP address instead of prefix, `/32` or `/128` will be appended automatically.

Can be overridden by `servers.[].client_subnet` or `rules.[].client_subnet`.
