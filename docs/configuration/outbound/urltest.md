### Structure

```json
{
  "type": "urltest",
  "tag": "auto",
  
  "outbounds": [
    "proxy-a",
    "proxy-b",
    "proxy-c"
  ],
  "url": "",
  "interval": "",
  "tolerance": 0,
  "idle_timeout": "",
  "interrupt_exist_connections": false,
  "prefer_domain": false,
  "override_ip": "",
  "latency_mode": "",
  "latency_offset": {}
}
```

### Fields

#### outbounds

==Required==

List of outbound tags to test.

#### url

The URL to test. `https://www.gstatic.com/generate_204` will be used if empty.

#### interval

The test interval. `3m` will be used if empty.

#### tolerance

The test tolerance in milliseconds. `50` will be used if empty.

#### idle_timeout

The idle timeout. `30m` will be used if empty.

#### interrupt_exist_connections

Interrupt existing connections when the selected outbound has changed.

Only inbound connections are affected by this setting, internal connections will always be interrupted.

#### latency_mode

==Optional==

Where member latency comes from. Default: `url_test`.

| Value | Behaviour |
|---|---|
| `url_test` | Members are ranked on the periodic URL test, and the group switches member after a check. |
| `session` | Members that measure their connections, currently [`anytls`](/configuration/outbound/anytls/#session-selection), are ranked on the latency of the connection a new dial would use: the lowest-latency idle session. The member is chosen again for every connection, so the group follows those latencies between checks. `tolerance` does not apply. |

In `session` mode a member with no measured idle session, or one that does not measure its connections, is ranked on its URL test result. URL tests keep running in both modes for that fallback and for detecting failed members.

A session latency covers only the hop to the proxy server, while a URL test covers the whole path to `url`, so a group that mixes both compares different measures. Set [`heartbeat`](/configuration/outbound/anytls/#heartbeat) on `anytls` members to keep their latency current.

At `debug` level each connection logs a `urltest.select` event with the chosen `tag`, its `latency_ms`, and its `source`, `session` or `url_test`.

#### latency_offset

==Optional==

A manual latency adjustment per member, as a map of member tag to milliseconds.
A positive value makes the member less preferred, as if its latency were that much higher; a negative value makes it more preferred.

```json
{
  "outbounds": ["hk-1", "jp-1"],
  "latency_offset": {
    "hk-1": 30,
    "jp-1": -20
  }
}
```

Members are compared on their latency plus their offset, in both `latency_mode`s, and `tolerance` applies to the shifted values.
Only the choice of member changes: stored URL test results and the delays shown in the Clash API are unshifted.
A shifted value below zero is fine and does not wrap.

Keys must be tags from `outbounds`; any other key is a configuration error.
Members without a key have no offset.
When the chosen member has an offset, the `urltest.select` debug event also carries it as `latency_offset_ms`.

#### prefer_domain

==Optional==

See [Dial Fields](/configuration/shared/dial/#prefer_domain).

#### override_ip

==Optional==

See [Dial Fields](/configuration/shared/dial/#override_ip).
