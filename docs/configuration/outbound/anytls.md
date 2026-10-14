---
icon: material/new-box
---

!!! question "Since sing-box 1.12.0"

### Structure

```json
{
  "type": "anytls",
  "tag": "anytls-out",

  "server": "127.0.0.1",
  "server_port": 1080,
  "password": "8JCsPssfgS8tiRwiMlhARg==",
  "idle_session_check_interval": "30s",
  "idle_session_timeout": "30s",
  "min_idle_session": 5,
	"ensure_idle_session": 10,
	"heartbeat": "30s",
	"min_idle_session_for_age": 2,
	"max_connection_lifetime": "300s",
	"connection_lifetime_jitter": "10s",
	"ensure_idle_session_create_rate": 5,
	"client_metadata": "",
  "tls": {},

  ... // Dial Fields
}
```

### Fields

#### server

==Required==

The server address.

#### server_port

==Required==

The server port.

#### password

==Required==

The AnyTLS password.

#### idle_session_check_interval

Interval checking for idle sessions. Default: 30s.

#### idle_session_timeout

In the check, close sessions that have been idle for longer than this. Default: 30s.

#### min_idle_session

In the check, at least the first `n` idle sessions are kept open. Default value: `n`=0

#### ensure_idle_session

Target number of idle sessions to maintain in the pool.

#### heartbeat

Interval for sending heartbeat packets to keep sessions alive.

Each heartbeat round trip also refreshes the session's latency, see [Session selection](#session-selection).

#### min_idle_session_for_age

Minimum idle sessions protected from age-based cleanup.

#### max_connection_lifetime

Maximum lifetime of a connection before it is recycled.

#### connection_lifetime_jitter

Random jitter added to max_connection_lifetime to avoid thundering herd.

#### ensure_idle_session_create_rate

Maximum rate at which new idle sessions are created per interval.

#### client_metadata

!!! question "Since sing-box 1.13.16"

Check [AnyTLS client metadata](/manual/misc/anytls-client-metadata/).

#### tls

==Required==

TLS configuration, see [TLS](/configuration/shared/tls/#outbound).

### Session selection

The client measures the end-to-end latency of every session itself, so no server support is needed. A session sends its settings as soon as it is opened and is timed by the server's reply, and every [heartbeat](#heartbeat) adds another sample to a smoothed average.

* A new connection uses the idle session with the lowest latency. Sessions not yet measured come after measured ones, newest first.
* Sessions closed by `max_connection_lifetime` are the highest-latency ones first, with unmeasured ones before them; `min_idle_session_for_age` keeps the lowest-latency ones.
* `min_idle_session` likewise keeps the lowest-latency idle sessions open.

Pre-created sessions from `ensure_idle_session` are measured right after they connect, before they carry any traffic. Without `heartbeat`, a session's latency is only measured once.

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
