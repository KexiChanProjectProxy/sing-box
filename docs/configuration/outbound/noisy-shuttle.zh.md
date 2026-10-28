### 结构

**注意：** 这是 sing-box 原生协议变体。它与 Rust noisy-shuttle 实现不兼容。

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

  ... // 拨号字段
}
```

### 字段

#### server

==必填==

服务器地址。

设置 [server_unix](#server_unix) 时不需要。

#### server_port

==必填==

服务器端口。

设置 [server_unix](#server_unix) 时不需要。

#### server_unix

!!! quote ""

    仅支持 Linux。

要连接的 Unix 域套接字路径，用于替代 `server`:`server_port`，例如本地反向代理，或使用 [listen_unix](/zh/configuration/shared/listen/#listen_unix) 监听的另一个 sing-box。

设置后，`server` 和 `server_port` 不会被连接或解析，并变为可选。如果设置了它们，它们仍会作为 TLS `server_name` 的默认值，以及 V2Ray 传输层（如有）的默认 `Host`；否则请设置 `tls.server_name`。

套接字上只承载 TCP。如果协议在 TCP 连接内传输 UDP（例如 `udp_over_tcp`），UDP 仍可使用。

不能与 `detour`、`tcp_fast_open`、`tcp_multi_path`、`bind_interface`、`routing_mark` 或 `network_strategy` 同时使用。

#### password

==必填==

Noisy-Shuttle 密码。

#### network

启用的网络。

可选 `tcp` `udp`。

默认同时启用两者。

#### tls

TLS 配置，参阅 [TLS](/zh/configuration/shared/tls/#出站)。

#### session

会话复用选项。

##### session.enabled

启用会话复用。默认值为 `false`（禁用），必须显式启用。

##### session.max_streams

每个会话的最大并发流数。默认值为 `16`。

##### session.max_requests

每个流的最大并发请求数。`0` 表示无限制。默认值为 `0`。

##### session.idle_timeout

流的空闲超时时间。默认值为 `5m`。

##### session.max_age

会话的最大有效期。`0` 表示无过期时间。默认值为 `0s`。

##### session.keepalive_interval

发送 keepalive 数据包的间隔。默认值为 `30s`。

##### session.keepalive_timeout

keepalive 响应超时时间。默认值为 `2x keepalive_interval`（60s）。

#### handshake

出站连接握手选项。

##### handshake.padding_min

出站握手的最小填充长度。默认值为 `0`。

##### handshake.padding_max

出站握手的最大填充长度。默认值为 `24`。

##### handshake.auth_timeout

握手认证超时时间。默认值为 `5s`。

#### udp_timeout

UDP 会话超时时间。默认值为 `60s`。

#### udp_max_packet_size

UDP 数据包最大大小。默认值为 `1500`。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。

### 最小示例

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

### 高级示例（包含会话复用、keepalive 和 UDP）

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
