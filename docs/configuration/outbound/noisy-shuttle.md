### Structure

```json
{
  "type": "noisy-shuttle",
  "tag": "ns-out",

  "server": "server.example",
  "server_port": 443,
  "password": "secret",
  "network": "tcp",
  "tls": {},
  "session": {
    "enabled": true,
    "max_streams": 16,
    "max_requests": 0,
    "idle_timeout": "5m",
    "max_age": "0s",
    "keepalive_interval": "30s"
  },
  "handshake": {
    "padding_min": 0,
    "padding_max": 24,
    "auth_timeout": "5s"
  },
  "multiplex": {},
  "transport": {},

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

Enable session multiplexing. Default is `true`.

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

#### handshake

Handshake options for outbound connections.

##### handshake.padding_min

Minimum padding length for outbound handshake. Default is `0`.

##### handshake.padding_max

Maximum padding length for outbound handshake. Default is `24`.

##### handshake.auth_timeout

Authentication timeout for handshake. Default is `5s`.

#### multiplex

See [Multiplex](/configuration/shared/multiplex#outbound) for details.

#### transport

V2Ray Transport configuration, see [V2Ray Transport](/configuration/shared/v2ray-transport/).

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
    "keepalive_interval": "30s"
  },
  "handshake": {
    "padding_min": 0,
    "padding_max": 24,
    "auth_timeout": "5s"
  }
}
```
