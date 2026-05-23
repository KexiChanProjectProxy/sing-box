### 结构

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

启用会话复用。默认值为 `true`。

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

#### handshake

出站连接握手选项。

##### handshake.padding_min

出站握手的最小填充长度。默认值为 `0`。

##### handshake.padding_max

出站握手的最大填充长度。默认值为 `24`。

##### handshake.auth_timeout

握手认证超时时间。默认值为 `5s`。

#### multiplex

参阅 [多路复用](/zh/configuration/shared/multiplex#出站)。

#### transport

V2Ray 传输配置，参阅 [V2Ray 传输层](/zh/configuration/shared/v2ray-transport/)。

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
    "keepalive_interval": "30s"
  },
  "handshake": {
    "padding_min": 0,
    "padding_max": 24,
    "auth_timeout": "5s"
  }
}
```
