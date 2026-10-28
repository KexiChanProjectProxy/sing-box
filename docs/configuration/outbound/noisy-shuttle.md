### Structure

**Note:** This is a sing-box-native protocol variant. It is NOT wire compatible with the Rust noisy-shuttle implementation.

```json
{
  "type": "noisy-shuttle",
  "tag": "ns-out",

  "server": "server.example",
  "server_port": 443,
  "server_unix": "",
  "password": "secret",
  "network": "tcp",
  "tls": {},
  "session": {
    "enabled": true,
    "max_streams": 16,
    "max_requests": 0,
    "idle_timeout": "5m",
    "max_age": "0s",
    "keepalive_interval": "30s",
    "keepalive_timeout": "60s"
  },
  "handshake": {
    "padding_min": 0,
    "padding_max": 24,
    "auth_timeout": "5s"
  },
  "udp_timeout": "60s",
  "udp_max_packet_size": 1500,

  ... // Dial Fields
}
```

### Fields

#### server

==Required==

The server address.

Not required when [server_unix](#server_unix) is set.

#### server_port

==Required==

The server port.

Not required when [server_unix](#server_unix) is set.

#### server_unix

!!! quote ""

    Only supported on Linux.

Path of a Unix domain socket to connect to instead of `server`:`server_port`, such as a local reverse proxy or another sing-box listening with [listen_unix](/configuration/shared/listen/#listen_unix).

When set, `server` and `server_port` are never dialed or resolved and become optional. If they are set, they are still used as the default TLS `server_name` and as the default `Host` of the V2Ray transport, if any. Otherwise set `tls.server_name`.

Only TCP is carried over the socket. UDP keeps working where the protocol carries it inside the TCP connection, for example with `udp_over_tcp`.

Cannot be used with `detour`, `tcp_fast_open`, `tcp_multi_path`, `bind_interface`, `routing_mark` or `network_strategy`.

#### password

==Required==

The Noisy-Shuttle password.

#### network

Enabled network.

One of `tcp` `udp`.

Both is enabled by default.

#### tls

TLS configuration, see [TLS](/configuration/shared/tls/#outbound).

#### session

Session multiplex options.

##### session.enabled

Enable session multiplexing. Default is `false` (disabled). Must be explicitly enabled.

##### session.max_streams

Maximum concurrent streams per session. Default is `16`.

##### session.max_requests

Maximum concurrent requests per stream. `0` means unlimited. Default is `0`.

##### session.idle_timeout

Idle timeout for streams. Default is `5m`.

##### session.max_age

Maximum age for sessions. `0` means no expiry. Default is `0s`.

##### session.keepalive_interval

Interval for sending keepalive packets. Default is `30s`.

##### session.keepalive_timeout

Timeout for keepalive response. Default is `2x keepalive_interval` (60s).

#### handshake

Handshake options for outbound connections.

##### handshake.padding_min

Minimum padding length for outbound handshake. Default is `0`.

##### handshake.padding_max

Maximum padding length for outbound handshake. Default is `24`.

##### handshake.auth_timeout

Authentication timeout for handshake. Default is `5s`.

#### udp_timeout

UDP session timeout. Default is `60s`.

#### udp_max_packet_size

Maximum UDP packet size. Default is `1500`.

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.

### Minimal Example

```json
{
  "type": "noisy-shuttle",
  "tag": "ns-out",
  "server": "server.example",
  "server_port": 443,
  "password": "secret",
  "network": "tcp",
  "tls": {
    "enabled": true,
    "server_name": "server.example"
  }
}
```

### Advanced Example (with session reuse, keepalive, and UDP)

```json
{
  "type": "noisy-shuttle",
  "tag": "ns-out",
  "server": "server.example",
  "server_port": 443,
  "password": "secret",
  "network": "tcp,udp",
  "tls": {
    "enabled": true,
    "server_name": "server.example"
  },
  "session": {
    "enabled": true,
    "max_streams": 16,
    "max_requests": 0,
    "idle_timeout": "5m",
    "max_age": "0s",
    "keepalive_interval": "30s",
    "keepalive_timeout": "60s"
  },
  "handshake": {
    "padding_min": 0,
    "padding_max": 24,
    "auth_timeout": "5s"
  },
  "udp_timeout": "60s",
  "udp_max_packet_size": 1500
}
```
