### Structure

**Note:** This is a sing-box-native protocol variant. It is NOT wire compatible with the Rust noisy-shuttle implementation.

```json
{
  "type": "noisy-shuttle",
  "tag": "ns-in",

  ... // Listen Fields

  "users": [
    {
      "name": "alice",
      "password": "secret"
    }
  ],
  "tls": {},
  "fallback": {
    "server": "127.0.0.1",
    "server_port": 8080
  },
  "network": "tcp",
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
    "max_padding": 256,
    "auth_timeout": "5s"
  },
  "udp_timeout": "60s",
  "udp_max_packet_size": 1500
}
```

### Listen Fields

See [Listen Fields](/configuration/shared/listen/) for details.

### Fields

#### users

==Required==

Noisy-Shuttle users.

#### tls

TLS configuration, see [TLS](/configuration/shared/tls/#inbound).

#### fallback

Fallback server configuration. Disabled if `fallback` is empty.

When configured, unrecognizable traffic will be forwarded to the fallback destination.

#### network

Enabled network.

One of `tcp` `udp`.

Both is enabled by default.

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

Handshake options for inbound connections.

##### handshake.max_padding

Maximum padding length for inbound handshake. Default is `256`.

##### handshake.auth_timeout

Authentication timeout for handshake. Default is `5s`.

#### udp_timeout

UDP session timeout. Default is `60s`.

#### udp_max_packet_size

Maximum UDP packet size. Default is `1500`.

### Minimal Example

```json
{
  "type": "noisy-shuttle",
  "tag": "ns-in",
  "listen": "::",
  "listen_port": 443,
  "users": [
    {
      "name": "alice",
      "password": "secret"
    }
  ],
  "tls": {
    "enabled": true,
    "certificate_path": "/path/to/cert.pem",
    "key_path": "/path/to/key.pem"
  }
}
```

### Advanced Example (with session reuse, keepalive, and UDP)

```json
{
  "type": "noisy-shuttle",
  "tag": "ns-in",
  "listen": "::",
  "listen_port": 443,
  "users": [
    {
      "name": "alice",
      "password": "secret"
    }
  ],
  "network": "tcp,udp",
  "tls": {
    "enabled": true,
    "certificate_path": "/path/to/cert.pem",
    "key_path": "/path/to/key.pem"
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
    "max_padding": 256,
    "auth_timeout": "5s"
  },
  "udp_timeout": "60s",
  "udp_max_packet_size": 1500
}
```
