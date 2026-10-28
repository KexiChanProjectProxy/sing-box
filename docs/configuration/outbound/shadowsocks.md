### Structure

```json
{
  "type": "shadowsocks",
  "tag": "ss-out",
  
  "server": "127.0.0.1",
  "server_port": 1080,
  "server_unix": "",
  "method": "2022-blake3-aes-128-gcm",
  "password": "8JCsPssfgS8tiRwiMlhARg==",
  "plugin": "",
  "plugin_opts": "",
  "network": "udp",
  "udp_over_tcp": false | {},
  "multiplex": {},

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

#### method

==Required==

Encryption methods:

* `2022-blake3-aes-128-gcm`
* `2022-blake3-aes-256-gcm`
* `2022-blake3-chacha20-poly1305`
* `none`
* `aes-128-gcm`
* `aes-192-gcm`
* `aes-256-gcm`
* `chacha20-ietf-poly1305`
* `xchacha20-ietf-poly1305`

Legacy encryption methods:

* `aes-128-ctr`
* `aes-192-ctr`
* `aes-256-ctr`
* `aes-128-cfb`
* `aes-192-cfb`
* `aes-256-cfb`
* `rc4-md5`
* `chacha20-ietf`
* `xchacha20`

#### password

==Required==

The shadowsocks password.

#### plugin

Shadowsocks SIP003 plugin, implemented in internal.

Only `obfs-local` and `v2ray-plugin` are supported.

#### plugin_opts

Shadowsocks SIP003 plugin options.

#### network

Enabled network

One of `tcp` `udp`.

Both is enabled by default.

#### udp_over_tcp

UDP over TCP configuration.

See [UDP Over TCP](/configuration/shared/udp-over-tcp/) for details.

Conflict with `multiplex`.

#### multiplex

See [Multiplex](/configuration/shared/multiplex#outbound) for details.

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
