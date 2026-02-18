---
icon: material/new-box
---

!!! quote "Changes in sing-box 1.12.0"

    :material-plus: [strategy](#strategy)  
    :material-plus: [predefined](#predefined)

!!! question "Since sing-box 1.11.0"

### route

```json
{
  "action": "route",  // default
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

`route` inherits the classic rule behavior of routing DNS requests to the specified server.

#### server

==Required==

Tag of target server.

#### strategy

!!! question "Since sing-box 1.12.0"

Set domain strategy for this query.

One of `prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`.

#### disable_cache

Disable cache and save cache in this query.

#### rewrite_ttl

Rewrite TTL in DNS responses.

#### client_subnet

Append a `edns0-subnet` OPT extra record with the specified IP prefix to every query by default.

If value is an IP address instead of prefix, `/32` or `/128` will be appended automatically.

Will overrides `dns.client_subnet`.

#### resolve_retries

Number of DNS query retry attempts.

#### resolve_timeout

Timeout for each DNS query retry attempt.

#### hold_valid

!!! question "HAProxy-style resolver"

Advertised TTL for successful DNS responses with answers.

The response will be returned immediately (stale-while-revalidate) after this duration, with a background refresh triggered.

Cache lifetime is `hold_valid * 2` (minimum `hold_valid + 30s`).

#### hold_nx

!!! question "HAProxy-style resolver"

Advertised TTL for NXDOMAIN (non-existent domain) responses.

#### hold_refused

!!! question "HAProxy-style resolver"

Advertised TTL for REFUSED DNS responses.

#### hold_other

!!! question "HAProxy-style resolver"

Advertised TTL for other DNS error responses (e.g., SERVFAIL).

#### hold_timeout

!!! question "HAProxy-style resolver"

Fallback to stale cached response when DNS queries time out.

If set, the last successful cached response will be returned instead of failing on timeout.

### route-options

```json
{
  "action": "route-options",
  "disable_cache": false,
  "rewrite_ttl": null,
  "client_subnet": null
}
```

`route-options` set options for routing.

### reject

```json
{
  "action": "reject",
  "method": "",
  "no_drop": false
}
```

`reject` reject DNS requests.

#### method

- `default`: Reply with REFUSED.
- `drop`: Drop the request.

`default` will be used by default.

#### no_drop

If not enabled, `method` will be temporarily overwritten to `drop` after 50 triggers in 30s.

Not available when `method` is set to drop.

### predefined

!!! question "Since sing-box 1.12.0"

```json
{
  "action": "predefined",
  "rcode": "",
  "answer": [],
  "ns": [],
  "extra": []
}
```

`predefined` responds with predefined DNS records.

#### rcode

The response code.

| Value      | Value in the legacy rcode server | Description     |
|------------|----------------------------------|-----------------|
| `NOERROR`  | `success`                        | Ok              |
| `FORMERR`  | `format_error`                   | Bad request     |
| `SERVFAIL` | `server_failure`                 | Server failure  |
| `NXDOMAIN` | `name_error`                     | Not found       |
| `NOTIMP`   | `not_implemented`                | Not implemented |
| `REFUSED`  | `refused`                        | Refused         |

`NOERROR` will be used by default.

#### answer

List of text DNS record to respond as answers.

Examples:

| Record Type | Example                       |
|-------------|-------------------------------|
| `A`         | `localhost. IN A 127.0.0.1`   |
| `AAAA`      | `localhost. IN AAAA ::1`      |
| `TXT`       | `localhost. IN TXT \"Hello\"` |

#### ns

List of text DNS record to respond as name servers.

#### extra

List of text DNS record to respond as extra records.
