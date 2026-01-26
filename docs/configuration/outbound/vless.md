### Structure

```json
{
  "type": "vless",
  "tag": "vless-out",

  "server": "127.0.0.1",
  "server_port": 1080,
  "uuid": "bf000d23-0752-40b4-affe-68f7707a9661",
  "flow": "xtls-rprx-vision",
  "network": "tcp",
  "tls": {},
  "packet_encoding": "",
  "multiplex": {},
  "transport": {},
  "connection_pool": {},

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

#### uuid

==Required==

VLESS user id.

#### flow

VLESS Sub-protocol.

Available values:

* `xtls-rprx-vision`

#### network

Enabled network

One of `tcp` `udp`.

Both is enabled by default.

#### tls

TLS configuration, see [TLS](/configuration/shared/tls/#outbound).

#### packet_encoding

UDP packet encoding, xudp is used by default.

| Encoding   | Description           |
|------------|-----------------------|
| (none)     | Disabled              |
| packetaddr | Supported by v2ray 5+ |
| xudp       | Supported by xray     |

#### multiplex

See [Multiplex](/configuration/shared/multiplex#outbound) for details.

#### transport

V2Ray Transport configuration, see [V2Ray Transport](/configuration/shared/v2ray-transport/).

#### connection_pool

Connection pool configuration for improved latency and reliability.

!!! warning "Compatibility"

    Connection pool is not compatible with `tcp_fast_open` option. The outbound will fail to initialize if both are enabled.

##### Structure

```json
{
  "connection_pool": {
    "ensure_idle_session": 3,
    "ensure_idle_session_create_rate": 2,
    "min_idle_session": 2,
    "min_idle_session_for_age": 0,
    "idle_session_check_interval": "30s",
    "idle_session_timeout": "5m",
    "max_connection_lifetime": "1h",
    "connection_lifetime_jitter": "10m",
    "heartbeat": "30s"
  }
}
```

##### Fields

###### ensure_idle_session

Number of idle connections to maintain in the pool. Set to `0` to disable connection pooling.

Default: `0` (disabled)

###### ensure_idle_session_create_rate

Maximum number of connections to create per maintenance cycle. This prevents connection storms during pool warmup.

Default: `1`

###### min_idle_session

Minimum number of idle connections to keep in the pool. This prevents aggressive cleanup when `idle_session_timeout` triggers.

Default: `0`

###### min_idle_session_for_age

Minimum number of idle connections to keep when performing age-based cleanup (when connections exceed `max_connection_lifetime`).

If not set, uses the value of `min_idle_session`.

Default: `0`

###### idle_session_check_interval

How often to run the maintenance cycle that performs cleanup and ensures idle connections.

Default: `30s`

###### idle_session_timeout

Close idle connections that have been inactive for longer than this duration. Respects `min_idle_session`.

Default: `5m`

###### max_connection_lifetime

Maximum age of a connection before it's rotated. Set to `0` to disable age-based rotation.

This helps distribute load and prevents long-lived connections from becoming stale.

Default: `0` (disabled)

###### connection_lifetime_jitter

Random jitter to add to `max_connection_lifetime`. This prevents all connections from being rotated at the same time (thundering herd problem).

The actual lifetime will be: `max_connection_lifetime ± connection_lifetime_jitter`

Default: `0`

###### heartbeat

Interval for TCP-level keepalive checks. Note that VLESS protocol has no built-in heartbeat mechanism, so this uses TCP read deadlines to detect dead connections.

Set to `0` to disable heartbeat.

Default: `0` (disabled)

##### Usage Examples

**Low Latency (Gaming, Trading)**

Maintains a small pool of warm connections for minimal latency.

```json
{
  "connection_pool": {
    "ensure_idle_session": 3,
    "min_idle_session": 2,
    "idle_session_timeout": "10m"
  }
}
```

**High Availability (Production)**

Includes connection rotation and rate limiting for production environments.

```json
{
  "connection_pool": {
    "ensure_idle_session": 5,
    "ensure_idle_session_create_rate": 2,
    "min_idle_session": 3,
    "idle_session_timeout": "5m",
    "max_connection_lifetime": "1h",
    "connection_lifetime_jitter": "10m"
  }
}
```

**Resource Efficient (Mobile, IoT)**

Minimal configuration for resource-constrained devices.

```json
{
  "connection_pool": {
    "ensure_idle_session": 1,
    "min_idle_session": 1,
    "idle_session_timeout": "2m"
  }
}
```

**With Multiplex**

Connection pool and multiplex can be used together. The pool maintains base TCP+TLS connections, while multiplex creates multiple streams over them.

```json
{
  "multiplex": {
    "enabled": true,
    "max_connections": 2
  },
  "connection_pool": {
    "ensure_idle_session": 3,
    "min_idle_session": 2
  }
}
```

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
