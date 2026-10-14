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
  "latency_mode": ""
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

#### prefer_domain

==Optional==

See [Dial Fields](/configuration/shared/dial/#prefer_domain).

#### override_ip

==Optional==

See [Dial Fields](/configuration/shared/dial/#override_ip).
