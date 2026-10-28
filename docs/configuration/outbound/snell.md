---
icon: material/new-box
---

!!! question "Since sing-box 1.14.0"

### Structure

```json
{
  "type": "snell",
  "tag": "snell-out",

  "server": "127.0.0.1",
  "server_port": 1080,
  "server_unix": "",
  "version": 4,
  "psk": "password",
  "userkey": "",
  "reuse": false,
  "network": "tcp",
  "obfs_mode": "",
  "obfs_host": "",

  ... // Dial Fields
}
```

### Version 6 Structure

```json
{
  "type": "snell",
  "tag": "snell-out",

  "server": "127.0.0.1",
  "server_port": 1080,
  "version": 6,
  "psk": "password",
  "userkey": "",
  "reuse": false,
  "network": "tcp",
  "mode": "",

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

==Required==

The Snell protocol version, one of `4` `6`.

Version `4` supports HTTP obfuscation (`obfs_mode` / `obfs_host`); version `6`
replaces it with traffic shaping (`mode`) and requires a `psk` of 12 to 255
bytes.

!!! note

    Since we intentionally do not support the QUIC proxy mode of Snell v5, the v5 wire protocol
    is effectively identical to v4, so no separate v4 server or v5 client is provided.

#### psk

==Required==

The pre-shared key.

#### userkey

The user key, used to authenticate against a multi-user server.

#### reuse

Enable connection reuse (the Snell v2 `CONNECT` command).

#### network

Enabled network

One of `tcp` `udp`.

Both is enabled by default.

#### obfs_mode

==Version 4 only==

HTTP obfuscation mode, one of `none` `http`.

`none` is used by default.

#### obfs_host

==Version 4 only==

The HTTP `Host` header sent when `obfs_mode` is `http`.

`bing.com` is used by default.

#### mode

==Version 6 only==

Traffic shaping mode, one of `default` `unshaped` `unsafe-raw`.

`default` is used by default.

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
