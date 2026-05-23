### 结构

**注意：** 这是 sing-box 原生协议变体。它与 Rust noisy-shuttle 实现不兼容。

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

#### server_port

==必填==

服务器端口。

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
