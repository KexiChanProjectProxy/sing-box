### Structure

```json
{
  "type": "vless",
  "tag": "vless-out",
  
  "server": "127.0.0.1",
  "server_port": 1080,
  "server_unix": "",
  "uuid": "bf000d23-0752-40b4-affe-68f7707a9661",
  "flow": "xtls-rprx-vision",
  "network": "tcp",
  "tls": {},
  "packet_encoding": "",
  "multiplex": {},
  "transport": {},

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

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
