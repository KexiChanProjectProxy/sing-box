### Structure

```json
{
  "type": "shadowtls",
  "tag": "st-out",
  
  "server": "127.0.0.1",
  "server_port": 1080,
  "server_unix": "",
  "version": 3,
  "password": "fuck me till the daylight",
  "tls": {},

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

#### version

ShadowTLS protocol version.

| Value         | Protocol Version                                                                        |
|---------------|-----------------------------------------------------------------------------------------|
| `1` (default) | [ShadowTLS v1](https://github.com/ihciah/shadow-tls/blob/master/docs/protocol-en.md#v1) |
| `2`           | [ShadowTLS v2](https://github.com/ihciah/shadow-tls/blob/master/docs/protocol-en.md#v2) |
| `3`           | [ShadowTLS v3](https://github.com/ihciah/shadow-tls/blob/master/docs/protocol-v3-en.md) |

#### password

Set password.

Only available in the ShadowTLS v2/v3 protocol.

#### tls

==Required==

TLS configuration, see [TLS](/configuration/shared/tls/#outbound).

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
