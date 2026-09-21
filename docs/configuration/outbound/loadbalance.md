`loadbalance` outbound is a group that health-checks member outbounds, prefers healthy primaries over backups, and selects a member per connection with consistent hashing or random.

### Structure

```json
{
  "type": "loadbalance",
  "tag": "my-lb",

  "primary_outbounds": [
    "proxy-a",
    "proxy-b"
  ],
  "backup_outbounds": [
    "proxy-c"
  ],
  "url": "https://www.gstatic.com/generate_204",
  "interval": "3m",
  "timeout": "15s",
  "idle_timeout": "30m",
  "tolerance": 10,
  "sorter": {
    "latency": 1
  },
  "top_n": {
    "primary": 0
  },
  "strategy": "consistent_hash",
  "hash": {
    "key_parts": ["src_ip", "matched_ruleset_or_etld"],
    "virtual_nodes": 100,
    "on_empty_key": "random",
    "key_salt": ""
  },
  "empty_pool_action": "error",
  "interrupt_exist_connections": false,
  "prefer_domain": false,
  "override_ip": ""
}
```

!!! quote ""

    The group is visible in the [Clash API](/configuration/experimental/clash-api/) (`now` / `all`). `now` is the best-ranked candidate, or the first primary before the first health result. Members cannot be selected through the API.

### Fields

#### primary_outbounds

==Required==

List of primary outbound tags. At least one tag is required. Healthy primary outbounds are always preferred over backup outbounds.

The group's own tag cannot appear here. Tags cannot overlap with `backup_outbounds`. Nested groups (`selector`, `urltest`, `loadbalance`) are allowed.

#### backup_outbounds

==Optional==

List of backup outbound tags. Backup outbounds are only used when no primary candidate is healthy. All healthy backups enter the pool; `top_n` does not apply.

The group's own tag cannot appear here. Tags cannot overlap with `primary_outbounds`.

#### url

==Optional==

URL for health check testing. `https://www.gstatic.com/generate_204` will be used if empty.

#### interval

==Optional==

Health check interval. `3m` will be used if empty.

Must be less than or equal to `idle_timeout`.

#### timeout

==Optional==

Unhealthy latency threshold. A candidate is unhealthy when it has no stored latency, a zero latency, or a latency greater than or equal to this value. `15s` will be used if empty.

The HTTP probe itself always uses a 15s deadline, independent of this field.

#### idle_timeout

==Optional==

Idle timeout for periodic health checking. Health checks stop when no traffic is detected for this duration, and resume on the next connection. `30m` will be used if empty.

#### top_n

==Optional==

Top N candidate selection options. Only `primary` is used.

#### top_n.primary

==Optional==

Select top N healthy primary outbounds by score (see [sorter](#sorter)). `0` means all healthy primary outbounds are used. Default: `0`.

#### top_n.backup

Not supported. Must be omitted or `0`.

#### tolerance

==Optional==

Score tolerance when choosing the top-N candidate set, in milliseconds.

On the first snapshot, the N best-scoring healthy primaries are taken as-is.
Afterwards, a better outbound replaces an incumbent only if it is better by more than this value. An equal delta keeps the incumbent.
`10` will be used if empty.

Scores are millisecond equivalent (see [sorter](#sorter)), so this value keeps its meaning whatever the sorter ranks on.

#### sorter

==Optional==

Ranking weights, as a map of metric keyword to weight. Ranking (`top_n`, sort order, `now`, `tolerance`) uses the resulting score, lowest first. Health checking and the timeout still use the raw measured delay, and so does connection fail-over.

`{"latency": 1}` will be used if omitted, which ranks on the last measured delay alone. An empty object is a configuration error.

A member's score is the weighted sum of its metrics:

```
score = Σ weight × metric
```

Every weight is positive, and says **how many milliseconds one unit of that metric is worth**. Metrics where a higher value is better are subtracted rather than added, so a lower score is always better and scores stay comparable with the raw delay they replace.

```json
{
  "sorter": {
    "latency_avg_1m": 1,
    "client_rtt": 0.5,
    "server_loss_rate_1m": 20,
    "server_delivery_rate": 0.1
  }
}
```

Read as: rank mostly on the one minute average delay, count half a millisecond for each millisecond of transport RTT, treat one percent of downstream loss as 20 ms, and credit 0.1 ms for each Mbps of measured throughput.

Supported keywords:

| Keyword                | Unit | Window     | Better |
|------------------------|------|------------|--------|
| `latency`              | ms   | latest     | lower  |
| `latency_avg_1m`       | ms   | 1m mean    | lower  |
| `latency_avg_5m`       | ms   | 5m mean    | lower  |
| `client_rtt`           | ms   | 1m mean    | lower  |
| `client_rttvar`        | ms   | 1m mean    | lower  |
| `client_loss_rate_30s` | %    | 30s totals | lower  |
| `client_loss_rate_1m`  | %    | 1m totals  | lower  |
| `client_loss_rate_5m`  | %    | 5m totals  | lower  |
| `client_delivery_rate` | Mbps | 1m peak    | higher |
| `server_rtt`           | ms   | 1m mean    | lower  |
| `server_rttvar`        | ms   | 1m mean    | lower  |
| `server_loss_rate_30s` | %    | 30s totals | lower  |
| `server_loss_rate_1m`  | %    | 1m totals  | lower  |
| `server_loss_rate_5m`  | %    | 5m totals  | lower  |
| `server_delivery_rate` | Mbps | 1m peak    | higher |

Windows are accumulated in ten second buckets, so a window covers between one bucket less than its name and its name. A loss rate is lost packets over sent packets across the window, an RTT keyword is the mean of the samples in it, and a delivery rate is its highest bucket, because a proxy connection is usually limited by what the application asks for rather than by the path.

An unknown keyword or a negative weight is a configuration error, as is a set of weights that are all zero. A single keyword weighted zero is simply dropped.

!!! note "Which members report transport statistics"

    Only [`hysteria2`](/configuration/outbound/hysteria2/) and [`anytls`](/configuration/outbound/anytls/) members report `client_*` and `server_*`. Other members are scored on the pool average for them, and a nested group is measured through the member it currently selects.

    | Keyword          | `hysteria2`                                   | `anytls`                                       |
    |------------------|-----------------------------------------------|------------------------------------------------|
    | `*_rtt`          | QUIC smoothed RTT                             | `TCP_INFO` RTT                                 |
    | `*_rttvar`       | QUIC RTT mean deviation                       | `TCP_INFO` RTT variance                        |
    | `*_loss_rate_*`  | packets declared lost over packets sent        | retransmitted segments over data segments sent |
    | `*_delivery_rate`| measured throughput                           | kernel delivery rate estimate                  |

    `client_*` is read locally: always for `hysteria2`, and on Linux for `anytls`. `server_*` is reported by the server over the protocol and needs a sing-box server that supports it; for `anytls` the server must also run on Linux. The request is negotiated when a connection is set up, so it covers connections opened after the group starts. A server that does not support it ignores the request, and the member is then scored on the pool average for `server_*`.

    Limits worth knowing:

    - An `anytls` member with a `detour` does not report `client_*`, since the socket under it belongs to another transport.
    - A connection that sent fewer than 32 packets since the last sample — for example only keepalives, heartbeats or the statistics exchange — contributes nothing to that sample unless it lost at least half of what it sent, and at least two packets. A member carrying little traffic is therefore scored on the pool average rather than on that noise once its earlier samples age out of the window, while an `anytls` member whose retransmissions start failing shows its losses for a while, until its backed-off retransmissions become too sparse and it too returns to the pool average. A `hysteria2` path that stops delivering anything shows no loss here, since QUIC finds such losses by its probe timer and does not report them; health checks catch that case instead.
    - For `hysteria2`, if one request for the server's statistics fails, the server direction stops reporting for the rest of that connection and resumes on the next one.

    When no member of the group reports at all, `loadbalance.sorter.unsupported` is logged at startup.

!!! note "Latency keywords"

    `latency` is the last health check result. `latency_avg_1m` and `latency_avg_5m` average the results measured within that window, and fall back to the last result when the window holds no sample.

    An average is only meaningful when the window holds several probes, so lower `interval` to around `15s`–`30s` before ranking on one. A `loadbalance.sorter.coarse_interval` warning is logged at startup when `interval` is more than half the narrowest window configured, which the default `interval` of three minutes is for both of them.

!!! note "Transport keywords"

    `client_*` is measured locally and describes the client to server direction. `server_*` is measured by the proxy server and reported back over the protocol, and describes the server to client direction — downstream loss is not visible to the client on its own.

    Both cover only the hop between this machine and the proxy server. A member that cannot report a metric is scored on the average of the members that can, so it is neither favoured nor penalised for it, and a keyword no member can report has no effect on the order.

    Members that are not in the current candidate set carry no user traffic, so their transport metrics are often missing. A member that is itself a group is measured through the outbound it currently selects; for a nested `loadbalance` that is its best-ranked member, which stands in for its whole pool.

    Members with equal scores are ordered by raw delay, then by tag. This matters whenever a sorter cannot tell members apart — for example when no member reports its keywords, or when several members report none of them and all take the same pool average — so latency still decides rather than tag names. The `loadbalance.sorter.unsupported` warning is logged once at startup, and only when no member implements the reporting interface at all.

!!! note "Reading the scores"

    At `debug` level the group logs each member's score and the contribution of each keyword after every health check round, which is how a weight is tuned.

#### weighted_delay

==Deprecated==

Use [sorter](#sorter) instead. It is translated to an equivalent sorter, and configuring both is an error.

When this object is present, ranking latency blends the window average with the last sample. The blend is preserved by the translation, the window is not: `weighted_delay` counted samples, while the sorter's `latency_avg_5m` covers a fixed five minutes. Because the translation always produces `latency_avg_5m`, the default `interval` of three minutes raises the `loadbalance.sorter.coarse_interval` warning described under [sorter](#sorter).

```json
{"weighted_delay": {"window_weight": 7, "last_weight": 3}}
```

is equivalent to

```json
{"sorter": {"latency_avg_5m": 0.7, "latency": 0.3}}
```

#### weighted_delay.window

==Deprecated==

Number of samples in the window. `5` will be used if empty. Must be between `1` and `64`. Ignored by the translation, which always uses a five minute window.

#### weighted_delay.window_weight

==Deprecated==

Weight of the sum of samples in the window. `1` will be used if empty.

#### weighted_delay.last_weight

==Deprecated==

Weight of the newest sample. That sample is also included in the window sum. `1` will be used if empty.

#### strategy

==Optional==

Selection strategy. Supported values: `consistent_hash`, `random`. Default: `consistent_hash`.

`hash` is ignored when `strategy` is `random`.

When `strategy` is `consistent_hash` (the default) but `hash.key_parts` is empty, the hash key is empty and selection follows `hash.on_empty_key` (default `random`). Set `hash.key_parts` to get session affinity.

#### hash

==Optional==

Consistent hash options. Ignored when `strategy` is `random`.

#### hash.key_parts

==Optional==

Parts used to construct the hash key, joined with `|`. Empty parts are skipped.

Supported values:

* `src_ip`: client source IP
* `matched_ruleset_or_etld`: the first matched [rule set](/configuration/rule-set/) tag, or the eTLD+1 of the sniffed domain / destination FQDN if no rule set matched

No default. An empty list (including when `hash` is omitted) produces an empty key.

#### hash.virtual_nodes

==Optional==

Number of virtual nodes per candidate in the hash ring. Higher values improve distribution uniformity. Default: `100`.

#### hash.on_empty_key

==Optional==

Behavior when the hash key is empty. `random` selects a random candidate; `error` returns a dial error. Default: `random`.

#### hash.key_salt

==Optional==

Salt prepended to the hash key and to virtual-node names. Default: `""`.

#### empty_pool_action

==Optional==

Action when no healthy candidate exists. Supported values: `error`, `random`. `error` picks no first choice and the connection goes straight to [connection fail-over](#connection-fail-over), walking every configured primary and then every configured backup; the dial fails only when all of them fail, or immediately when no outbound is configured. `random` picks a random first choice from all configured primary and backup outbounds without health filtering, then fails over through the rest. Default: `error`.

#### interrupt_exist_connections

==Optional==

Interrupt existing connections when the healthy candidate **set** changes (membership), not when a later connection hashes to a different member of the same set.

Only inbound connections are affected by this setting, internal connections will always be interrupted.

#### prefer_domain

==Optional==

See [Dial Fields](/configuration/shared/dial/#prefer_domain).

Applied at this group before the connection is delegated to the selected member.

#### override_ip

==Optional==

See [Dial Fields](/configuration/shared/dial/#override_ip).

`prefer_domain` and `override_ip` are mutually exclusive.

### Startup Behavior

The outbound starts immediately and seeds the candidate pool with all primary outbounds. A background health check then replaces that seed with the healthy top-N set. Nested `loadbalance` / `urltest` members that have not produced a delay yet keep the seed. `empty_pool_action` applies only after health results exist and no candidate remains healthy. Leaf members that fail HTTP still empty the pool, but with the default `error` action a connection still [fails over](#connection-fail-over) through every configured member before it fails.

### Health Check

Members are probed in the background. Unlike [`urltest`](/configuration/outbound/urltest/), loadbalance measures HTTP RTT after dial, proxy handshake, and destination TLS have finished.

A member is healthy only when a stored latency exists, is non-zero, and is strictly below `timeout`. Health never depends on the [sorter](#sorter): a member is filtered on its raw delay, then the survivors are scored and ranked. Failed probes and failed dials delete that member's stored latency, and any member filtered out as unhealthy — including one whose delay merely reached `timeout` — has its latency history reset, so it is not ranked on averages from before it degraded. A failed dial also moves the current connection to the next member (see [Connection Fail-over](#connection-fail-over)). The candidate pool is rebuilt after each health-check round, not after a failed dial, so transport metrics reach the ranking on the next round.

Nested `loadbalance` and `urltest` members are never HTTP-probed by the parent. The parent reuses the child's current delay (the lowest raw delay in its snapshot, or last urltest history). For transport keywords a nested group is instead measured through its current selection, so for a nested `loadbalance` the two can describe different members. A nested `selector` is treated as a leaf: reuse a fresh history of the selected outbound, otherwise HTTP-probe that outbound.

### Connection Fail-over

When the chosen member fails before any payload is forwarded, the same connection tries the next member instead of failing. Failures that trigger this include TCP connect errors, TLS or proxy handshake errors, authentication failures, rejections, and timeouts returned by the member's dial (or UDP listen).

Attempt order is fixed when the connection starts:

1. The first choice from `strategy` and the current candidate pool, exactly as without fail-over.
2. Every other configured `primary_outbounds` member.
3. Every other configured `backup_outbounds` member, only after all primaries have failed.

Within steps 2 and 3, members are ordered by their last measured raw latency, lowest first, with ties broken by tag. Members without a measurement come last, ordered by tag. `sorter`, `top_n`, `tolerance`, and health filtering do not apply here: a dial that is already failing falls back on the simplest signal available, and unhealthy members are still tried as long as they are configured. A backup is never tried before an untried primary, even when it is faster.

Each member is tried at most once per connection. Members that do not support the connection's network are skipped. A nested `loadbalance`, `urltest`, or `selector` counts as one member and handles its own members. When every member fails, the error of the last attempt is returned.

Fail-over stops as soon as a member succeeds. Errors after the tunnel is established, such as resets, empty responses, or destination HTTP errors, never trigger fail-over. Cancelling the connection stops the walk.

A failed connection does not rebuild the candidate pool, change consistent-hash affinity, or interrupt other connections. The next connection still picks its first choice from the current pool; failed members leave the pool only through their deleted latency and the next health-check round.

There is no overall fail-over timeout. In the worst case a connection waits for the connect timeout of every member in turn, bounded by the caller's own deadline.

Each failed attempt logs a `urltest.error` event with the member tag. A connection that succeeds on a member other than the first choice logs an info `loadbalance.failover` event with `selected` (empty when the pool was empty), `used`, and `tried`. A connection that fails after more than one attempt logs an error `loadbalance.failover.exhausted` event with `selected`, `tried`, and the last error.

!!! warning "Behaviour change in 1.14.0.15"

    With `empty_pool_action: error`, an empty candidate pool no longer fails the dial immediately. Deployments that relied on fast failure when every member is unhealthy now wait for real dial attempts.

### Primary/Backup Semantics

Healthy primary outbounds are always preferred over backup outbounds. Backup outbounds are only used when no primary candidate is healthy.

When `top_n.primary` is `0` or at least as large as the healthy primary set, every healthy primary is used. Otherwise the pool is the N best-scoring healthy primaries (see [sorter](#sorter)), with `tolerance` hysteresis after the first snapshot.

Backup outbounds skip top-N: if no primary is healthy, every healthy backup is used.

### Consistent Hash

With the `consistent_hash` strategy, the same hash key consistently selects the same candidate as long as the candidate set does not change. When a candidate is removed, only keys that mapped to that candidate are remapped.

Without `hash.key_parts`, this degenerates to `hash.on_empty_key` (default random).

### Random Strategy

With the `random` strategy, each connection request selects a candidate randomly from the current healthy candidate pool. No hash key computation is performed, and no session affinity is provided. Every selection is independent.
