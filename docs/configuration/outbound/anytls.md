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

**Note**: This is a **passive protection** mechanism - it only prevents existing sessions from being closed due to timeout.

#### ensure_idle_session

Proactively maintains at least `n` idle sessions in the pool. If the current idle session count is below this value, new sessions will be automatically created. Default value: `n`=0 (disabled)

**Note**: This is an **active maintenance** mechanism - it creates new sessions to maintain the pool size.

**Comparison**:
- `min_idle_session`: Protects existing sessions from timeout closure
- `ensure_idle_session`: Creates new sessions to reach target pool size

**Example**:
```json
{
  "min_idle_session": 3,
  "ensure_idle_session": 10,
  "idle_session_timeout": "5m"
}
```

With this configuration:
- The pool will always try to maintain 10 idle sessions
- Even if sessions are idle for >5 minutes, the first 3 won't be closed
- If pool drops below 10 (e.g., after cleanup or network issues), new sessions are created automatically

**Use cases**:
- **High-traffic scenarios**: Pre-warm connections to reduce first-request latency
- **NAT keepalive**: Maintain persistent connections through NAT devices
- **Reliability**: Always have ready-to-use sessions available

#### tls

==Required==

TLS configuration, see [TLS](/configuration/shared/tls/#outbound).

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
