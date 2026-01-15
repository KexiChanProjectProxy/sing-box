V2Ray Transport 是 v2ray 发明的一组私有协议，并污染了其他协议的名称，如 clash 中的 `trojan-grpc`。

### 结构

```json
{
  "type": ""
}
```

可用的传输协议：

* HTTP
* WebSocket
* QUIC
* gRPC
* HTTPUpgrade
* xhttp

!!! warning "与 v2ray-core 的区别"

    * 没有 TCP 传输层, 纯 HTTP 已合并到 HTTP 传输层。
    * 没有 mKCP 传输层。
    * 没有 DomainSocket 传输层。

!!! note ""

    当内容只有一项时，可以忽略 JSON 数组 [] 标签。

### HTTP

```json
{
  "type": "http",
  "host": [],
  "path": "",
  "method": "",
  "headers": {},
  "idle_timeout": "15s",
  "ping_timeout": "15s"
}
```

!!! warning "与 v2ray-core 的区别"

    不强制执行 TLS。如果未配置 TLS，将使用纯 HTTP 1.1。

#### host

主机域名列表。

如果设置，客户端将随机选择，服务器将验证。

#### path

!!! warning

    V2Ray 文档称服务端和客户端的路径必须一致，但实际代码允许客户端向路径添加任何后缀。
    sing-box 使用与 V2Ray 相同的行为，但请注意，该行为在 `WebSocket` 和 `HTTPUpgrade` 传输层中不存在。

HTTP 请求路径

服务器将验证。

#### method

HTTP 请求方法

如果设置，服务器将验证。

#### headers

HTTP 请求的额外标头

如果设置，服务器将写入响应。

#### idle_timeout

在 HTTP2 服务器中：

指定闲置客户端应在多长时间内使用 GOAWAY 帧关闭。PING 帧不被视为活动。

在 HTTP2 客户端中：

如果连接上没有收到任何帧，指定一段时间后将使用 PING 帧执行健康检查。需要注意的是，PING 响应被视为已接收的帧，因此如果连接上没有其他流量，则健康检查将在每个间隔执行一次。如果值为零，则不会执行健康检查。

默认使用零。

#### ping_timeout

在 HTTP2 客户端中：

指定发送 PING 帧后，在指定的超时时间内必须接收到响应。如果在指定的超时时间内没有收到 PING 帧的响应，则连接将关闭。默认超时持续时间为 15 秒。

### WebSocket

```json
{
  "type": "ws",
  "path": "",
  "headers": {},
  "max_early_data": 0,
  "early_data_header_name": ""
}
```

#### path

HTTP 请求路径

服务器将验证。

#### headers

HTTP 请求的额外标头

如果设置，服务器将写入响应。

#### max_early_data

请求中允许的最大有效负载大小。默认启用。

#### early_data_header_name

默认情况下，早期数据在路径而不是标头中发送。

要与 Xray-core 兼容，请将其设置为 `Sec-WebSocket-Protocol`。

它需要与服务器保持一致。

### QUIC

```json
{
  "type": "quic"
}
```

!!! warning "与 v2ray-core 的区别"

    没有额外的加密支持：
    它基本上是重复加密。 并且 Xray-core 在这里与 v2ray-core 不兼容。

### gRPC

!!! note ""

    默认安装不包含标准 gRPC (兼容性好，但性能较差), 参阅 [安装](/zh/installation/build-from-source/#_5)。

```json
{
  "type": "grpc",
  "service_name": "TunService",
  "idle_timeout": "15s",
  "ping_timeout": "15s",
  "permit_without_stream": false
}
```

#### service_name

gRPC 服务名称。

#### idle_timeout

在标准 gRPC 服务器/客户端：

如果传输在此时间段后没有看到任何活动，它会向客户端发送 ping 请求以检查连接是否仍然活动。

在默认 gRPC 服务器/客户端：

它的行为与 HTTP 传输层中的相应设置相同。

#### ping_timeout

在标准 gRPC 服务器/客户端：

经过一段时间之后，客户端将执行 keepalive 检查并等待活动。如果没有检测到任何活动，则会关闭连接。

在默认 gRPC 服务器/客户端：

它的行为与 HTTP 传输层中的相应设置相同。

#### permit_without_stream

在标准 gRPC 客户端：

如果启用，客户端传输即使没有活动连接也会发送 keepalive ping。如果禁用，则在没有活动连接时，将忽略 `idle_timeout` 和 `ping_timeout`，并且不会发送 keepalive ping。

默认禁用。

### HTTPUpgrade

```json
{
  "type": "httpupgrade",
  "host": "",
  "path": "",
  "headers": {}
}
```

#### host

主机域名。

服务器将验证。

#### path

HTTP 请求路径

服务器将验证。

#### headers

HTTP 请求的额外标头。

如果设置，服务器将写入响应。
### xhttp

```json
{
  "type": "xhttp",
  "host": "",
  "path": "/",
  "mode": "auto",
  "headers": {},
  "x_padding_bytes": {
    "from": 100,
    "to": 1000
  },
  "sc_max_each_post_bytes": {
    "from": 1000000,
    "to": 1000000
  },
  "sc_min_posts_interval_ms": {
    "from": 30,
    "to": 30
  },
  "sc_max_buffered_posts": 30,
  "no_grpc_header": false,
  "xmux": {
    "max_concurrency": {
      "from": 0,
      "to": 0
    },
    "max_connections": {
      "from": 0,
      "to": 0
    },
    "c_max_reuse_times": {
      "from": 0,
      "to": 0
    },
    "h_max_request_times": {
      "from": 0,
      "to": 0
    },
    "h_max_reusable_secs": {
      "from": 0,
      "to": 0
    },
    "h_keep_alive_period": 0
  }
}
```

!!! note ""

    xhttp 传输层在 Xray-core 中也称为 splithttp。
    它提供基于 HTTP 的隧道传输，具有高级流量混淆和连接优化功能。

!!! warning ""

    此传输层与 Xray-core 的 xhttp/splithttp 传输层兼容。

#### host

HTTP 请求的主机域名。

如果不为空，服务端将验证。

#### path

HTTP 请求的路径。

客户端会自动在路径后添加会话 ID。

默认为 `/`。

#### mode

xhttp 传输的操作模式。

当前支持：

- `auto`：自动选择模式（默认为 `packet-up`）
- `packet-up`：通过多个 POST 请求以序列化数据包发送数据

默认值：`auto`

!!! note ""

    未来版本将支持更多模式（`stream-up`、`stream-down`、`stream-one`）。

#### headers

HTTP 请求的额外头部。

如果不为空，服务端将在响应中写入。

#### x_padding_bytes

用于流量混淆的随机填充范围。

每个请求将添加一个长度在此范围内的随机 `x_padding` 查询参数。

默认值：`{"from": 100, "to": 1000}`

#### sc_max_each_post_bytes

每个 POST 请求的最大负载大小（字节）。

默认值：`{"from": 1000000, "to": 1000000}`（1MB）

#### sc_min_posts_interval_ms

POST 请求之间的最小间隔（毫秒）。

这有助于控制请求速率，避免服务器过载。

默认值：`{"from": 30, "to": 30}`

#### sc_max_buffered_posts

上传队列中缓冲的最大数据包数量。

如果队列超过此大小，连接将被断开以防止内存耗尽。

默认值：`30`

#### no_grpc_header

如果启用，GET 响应中将不发送 `Content-Type: text/event-stream` 头部。

默认值：`false`

#### xmux

用于优化连接复用的连接多路复用（Xmux）配置。

##### max_concurrency

每个池化连接允许的最大并发连接数。

范围值，其中 `0` 表示无限制。

默认值：`{"from": 0, "to": 0}`（无限制）

##### max_connections

连接池中的最大总连接数。

范围值，其中 `0` 表示无限制。

默认值：`{"from": 0, "to": 0}`（无限制）

##### c_max_reuse_times

连接可以被重用的最大次数。

范围值，其中 `0` 表示无限制。

默认值：`{"from": 0, "to": 0}`（无限制）

##### h_max_request_times

每个连接允许的最大 HTTP 请求数。

范围值，其中 `0` 表示无限制。

默认值：`{"from": 0, "to": 0}`（无限制）

##### h_max_reusable_secs

连接的最大生存时间（秒）（TTL）。

范围值，其中 `0` 表示无限制。

默认值：`{"from": 0, "to": 0}`（无限制）

##### h_keep_alive_period

空闲连接的保活超时时间（秒）。

`0` 表示禁用。

默认值：`0`

!!! tip "范围配置"

    许多 xhttp 参数使用带有 `from` 和 `to` 字段的范围配置。
    实际使用的值将在此范围内随机选择，提供额外的混淆效果。
    如果 `from` 等于 `to`，则使用该精确值。
