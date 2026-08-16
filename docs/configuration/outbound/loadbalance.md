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
  "timeout": "5s",
  "idle_timeout": "30m",
  "tolerance": 10,
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
  "prefer_domain": false
}
```

### Fields

#### primary_outbounds

==Required==

List of primary outbound tags. Healthy primary outbounds are always preferred over backup outbounds.

#### backup_outbounds

==Optional==

List of backup outbound tags. Backup outbounds are only used when no primary candidate is healthy.

#### url

==Optional==

URL for health check testing. `https://www.gstatic.com/generate_204` will be used if empty.

#### interval

==Optional==

Health check interval. `3m` will be used if empty.

#### timeout

==Optional==

Health check timeout. A candidate is considered unhealthy if its latency exceeds this value. `5s` will be used if empty.

#### idle_timeout

==Optional==

Idle timeout for periodic health checking. Health checks stop when no traffic is detected for this duration. `30m` will be used if empty.

#### top_n

==Optional==

Top N candidate selection options.

#### top_n.primary

==Optional==

Select top N healthy primary outbounds by latency. `0` means all healthy primary outbounds are used. Default: `0`.

#### tolerance

==Optional==

Latency tolerance in milliseconds when choosing the top-N candidate set.
A faster outbound replaces a current candidate only if it is better by more than this value.
`10` will be used if empty.

#### strategy

==Optional==

Selection strategy. Supported values: `consistent_hash`, `random`. Default: `consistent_hash`.

#### hash

==Optional==

Consistent hash options.

#### hash.key_parts

==Optional==

Parts used to construct the hash key. Supported values: `src_ip`, `matched_ruleset_or_etld`. Default: `["src_ip", "matched_ruleset_or_etld"]`.

#### hash.virtual_nodes

==Optional==

Number of virtual nodes per candidate in the hash ring. Higher values improve distribution uniformity. Default: `100`.

#### hash.on_empty_key

==Optional==

Behavior when the hash key is empty. `random` selects a random candidate; `error` returns a dial error. Default: `random`.

#### hash.key_salt

==Optional==

Salt prepended to hash key input for additional randomization. Default: `""`.

#### empty_pool_action

==Optional==

Action when no healthy candidate exists. Supported values: `error`, `random`. `error` causes dials to fail; `random` selects randomly from all configured primary and backup outbounds without health filtering. Default: `error`.

#### interrupt_exist_connections

==Optional==

Interrupt existing connections when the selected outbound has changed.

Only inbound connections are affected by this setting, internal connections will always be interrupted.

#### prefer_domain

==Optional==

Prefer domain resolution through the selected outbound. Default: `false`.

### Startup Behavior

The outbound starts immediately and triggers an initial health check in the background. Before any health results exist, the candidate pool is empty and `empty_pool_action` determines behavior.

### Primary/Backup Semantics

Healthy primary outbounds are always preferred over backup outbounds. Backup outbounds are only used when no primary candidate is healthy.

### Consistent Hash

With the `consistent_hash` strategy, the same hash key consistently selects the same candidate as long as the candidate set does not change. When a candidate is removed, only keys that mapped to that candidate are remapped.

### Random Strategy

With the `random` strategy, each connection request selects a candidate randomly from the current healthy candidate pool. No hash key computation is performed, and no session affinity is provided. Every selection is independent.