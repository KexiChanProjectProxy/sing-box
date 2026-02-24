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
  "min_idle_session_for_age": 0,
  "ensure_idle_session": 0,
  "ensure_idle_session_create_rate": 0,
  "heartbeat": "",
  "max_connection_lifetime": "",
  "connection_lifetime_jitter": "",
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

Interval for checking idle sessions. Default: `30s`.

#### idle_session_timeout

In the check, close sessions that have been idle longer than this duration. Default: `30s`.

#### min_idle_session

In the check, keep at least the first `n` idle sessions open. Default: `0`.

#### min_idle_session_for_age

Minimum number of idle sessions to retain when cleaning up sessions by age. Sessions will not be closed due to `max_connection_lifetime` if the idle session count would fall below this value. Default: `0`.

#### ensure_idle_session

Target number of idle sessions to proactively maintain. The client will create new sessions in the background to reach this count. Default: `0` (disabled).

#### ensure_idle_session_create_rate

Maximum rate (sessions per second) at which new sessions are created when ensuring idle sessions. Default: `0` (unlimited).

#### heartbeat

Interval for sending keep-alive heartbeats on idle sessions to prevent them from being closed by intermediate network devices. Default: disabled.

#### max_connection_lifetime

Maximum lifetime of a session before it is closed and replaced. Useful for rotating sessions periodically. Default: disabled.

#### connection_lifetime_jitter

Random jitter added to `max_connection_lifetime` to spread out session rotation and avoid thundering herd. Default: disabled.

#### tls

==Required==

TLS configuration, see [TLS](/configuration/shared/tls/#outbound).

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
