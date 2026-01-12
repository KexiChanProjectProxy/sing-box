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
  "ensure_idle_session": 0,
  "ensure_idle_session_create_rate": 0,
  "max_connection_lifetime": "",
  "connection_lifetime_jitter": "",
  "min_idle_session_for_age": 0,
  "heartbeat": "11s",
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

!!! note "New in sing-box 1.12.15"

#### ensure_idle_session

Proactively maintain a target number of idle sessions in the pool. When the pool drops below this threshold, new sessions are automatically created.

Different from `min_idle_session` (passive protection during cleanup), this provides active session creation for high availability and low-latency connection establishment.

Default: `0` (disabled)

Example: `10` maintains at least 10 warm connections ready for use.

#### ensure_idle_session_create_rate

Limit the rate of session creation during pool recovery. Maximum number of sessions created per cleanup cycle.

This prevents connection storms when recovering the session pool, protecting destination servers from sudden spikes.

- `0`: Unlimited (default, create all needed sessions immediately)
- `1-3`: Slow recovery (recommended for rate-limited destinations)
- `3-5`: Balanced recovery
- `5-10`: Fast recovery

Default: `0` (unlimited)

#### max_connection_lifetime

Maximum age for a session before it's automatically closed. Sessions exceeding this lifetime are closed, with oldest sessions closed first.

This works together with `ensure_idle_session` for automatic connection rotation.

Benefits:
- Security: Limit connection exposure window
- Load balancing: Distribute across dynamic backends
- NAT resilience: Periodic session renewal

Example: `1h` (sessions live up to 1 hour)

Default: unlimited (no age-based expiration)

#### connection_lifetime_jitter

Randomize connection lifetimes to prevent thundering herd. Each session gets a random lifetime: `base ± jitter`.

This distributes expiration across a time window, preventing simultaneous reconnection spikes and ensuring smooth connection rotation over time.

Example with `max_connection_lifetime: 1h` and `connection_lifetime_jitter: 15m`:
- Sessions will live between 45-75 minutes
- Expirations spread smoothly over 30-minute window

Default: `0` (no randomization)

#### min_idle_session_for_age

Minimum number of idle sessions to protect from age-based closure (separate from idle timeout protection).

This allows independent control for different cleanup scenarios:
- `min_idle_session`: Protects from idle timeout closure
- `min_idle_session_for_age`: Protects from age-based closure

Use cases:
- Aggressive age rotation + generous idle protection
- Conservative age rotation + aggressive idle cleanup
- Balanced protection for both scenarios

Default: `0` (use same as `min_idle_session`)

!!! note "New in sing-box 1.12.15"

#### heartbeat

Interval for sending heartbeat packets to keep connections alive and detect connection failures early.

Heartbeat packets help maintain connection health and allow faster detection of dead connections.

Example: `11s` (send heartbeat every 11 seconds)

Default: disabled (no heartbeat)

!!! note "Requires anytls library support"

    This option will be passed to the anytls library once heartbeat support is added.

#### tls

==Required==

TLS configuration, see [TLS](/configuration/shared/tls/#outbound).

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
