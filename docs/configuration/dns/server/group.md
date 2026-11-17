---
icon: material/new-box
---

!!! question "Since sing-box 1.14.2.11"

# Group

Spreads queries over other DNS servers, and falls back to more when they all fail.

### Structure

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

### Fields

#### servers

==Required==

The tags of the DNS servers in the group.

Only these types are accepted: [`udp`](../udp/), [`tcp`](../tcp/), [`tls`](../tls/) and [`https`](../https/).
Each keeps its own options, such as `detour`, `domain_resolver` and TLS.

#### fallbacks

The tags of the DNS servers tried in order once every server in `servers` has failed.

The servers in `servers` get their share of the query's time and leave the rest to the fallbacks, so that servers which do not answer at all still leave time to try them.
Asking every server at once in `parallel` mode counts as one try.

The same types are accepted as in `servers`.

#### mode

How queries are spread over `servers`.

| Mode                     | Behavior |
|--------------------------|----------|
| `load_balance` (default) | Try the servers one after another in a random order that favours the faster ones, weighting each by the inverse of its recent round-trip time. A server that failed counts as slow until it answers again. A server not measured yet is tried early. |
| `parallel`               | Ask every server at once and take the first answer. |
| `sequential`             | Try the servers one after another in the order listed. |

Trying servers one after another gives each an equal share of the time left, so a server that does not answer cannot use up the whole query.

A server failure (`SERVFAIL`) or refusal (`REFUSED`) moves on to the next server like an error does, and counts as a failure for load balancing.
Other answers, such as `NXDOMAIN`, are final.

### Cache

The group is cached as one DNS server, under its own tag.
Queries to its members do not go through the DNS cache again.

Every query sent to the group shares that cache, whether it comes from DNS rules, from sing-box resolving a domain for itself, or from clients of an AnyTLS inbound whose [`dns.server`](/configuration/inbound/anytls/#dnsserver) is the group.
Like any query, one that sets the DNSSEC OK bit is not cached.

### Example

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
