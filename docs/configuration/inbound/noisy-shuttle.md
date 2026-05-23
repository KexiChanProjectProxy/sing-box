### Structure

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
    "keepalive_interval": "30s"
  },
  "handshake": {
    "max_padding": 256,
    "auth_timeout": "5s"
  },
  "multiplex": {},
  "transport": {}
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

Handshake options for inbound connections.

##### handshake.max_padding

Maximum padding length for inbound handshake. Default is `256`.

##### handshake.auth_timeout

Authentication timeout for handshake. Default is `5s`.

#### multiplex

See [Multiplex](/configuration/shared/multiplex#inbound) for details.

#### transport

V2Ray Transport configuration, see [V2Ray Transport](/configuration/shared/v2ray-transport/).

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
    "keepalive_interval": "30s"
  },
  "handshake": {
    "max_padding": 256,
    "auth_timeout": "5s"
  }
}
```
