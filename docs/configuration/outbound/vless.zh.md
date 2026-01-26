### 结构

```json
{
  "type": "vless",
  "tag": "vless-out",

  "server": "127.0.0.1",
  "server_port": 1080,
  "uuid": "bf000d23-0752-40b4-affe-68f7707a9661",
  "flow": "xtls-rprx-vision",
  "network": "tcp",
  "tls": {},
  "packet_encoding": "",
  "multiplex": {},
  "transport": {},
  "connection_pool": {},

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

#### uuid

==必填==

VLESS 用户 ID。

#### flow

VLESS 子协议。

可用值：

* `xtls-rprx-vision`

#### network

启用的网络协议。

`tcp` 或 `udp`。

默认所有。

#### tls

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#outbound)。

#### packet_encoding

UDP 包编码，默认使用 xudp。

| 编码         | 描述            |
|------------|---------------|
| (空)        | 禁用            |
| packetaddr | 由 v2ray 5+ 支持 |
| xudp       | 由 xray 支持     |

#### multiplex

参阅 [多路复用](/zh/configuration/shared/multiplex#outbound)。

#### transport

V2Ray 传输配置，参阅 [V2Ray 传输层](/zh/configuration/shared/v2ray-transport/)。

#### connection_pool

连接池配置，用于改善延迟和可靠性。

!!! warning "兼容性"

    连接池与 `tcp_fast_open` 选项不兼容。如果同时启用两者，出站将无法初始化。

##### 结构

```json
{
  "connection_pool": {
    "ensure_idle_session": 3,
    "ensure_idle_session_create_rate": 2,
    "min_idle_session": 2,
    "min_idle_session_for_age": 0,
    "idle_session_check_interval": "30s",
    "idle_session_timeout": "5m",
    "max_connection_lifetime": "1h",
    "connection_lifetime_jitter": "10m",
    "heartbeat": "30s"
  }
}
```

##### 字段

###### ensure_idle_session

在池中维护的空闲连接数。设置为 `0` 禁用连接池。

默认值：`0`（禁用）

###### ensure_idle_session_create_rate

每个维护周期最多创建的连接数。这可以防止池预热期间的连接风暴。

默认值：`1`

###### min_idle_session

池中保持的最小空闲连接数。这可以防止在 `idle_session_timeout` 触发时过度清理。

默认值：`0`

###### min_idle_session_for_age

执行基于年龄的清理时保持的最小空闲连接数（当连接超过 `max_connection_lifetime` 时）。

如果未设置，使用 `min_idle_session` 的值。

默认值：`0`

###### idle_session_check_interval

运行维护周期的频率，该周期执行清理并确保空闲连接。

默认值：`30s`

###### idle_session_timeout

关闭空闲时间超过此持续时间的空闲连接。遵守 `min_idle_session`。

默认值：`5m`

###### max_connection_lifetime

连接轮换前的最大年龄。设置为 `0` 禁用基于年龄的轮换。

这有助于分配负载并防止长期连接变得陈旧。

默认值：`0`（禁用）

###### connection_lifetime_jitter

添加到 `max_connection_lifetime` 的随机抖动。这可以防止所有连接同时轮换（惊群问题）。

实际生命周期将是：`max_connection_lifetime ± connection_lifetime_jitter`

默认值：`0`

###### heartbeat

TCP 级保活检查的间隔。请注意，VLESS 协议没有内置的心跳机制，因此这使用 TCP 读取截止时间来检测死连接。

设置为 `0` 禁用心跳。

默认值：`0`（禁用）

##### 使用示例

**低延迟（游戏、交易）**

维护一个小型的预热连接池以获得最小延迟。

```json
{
  "connection_pool": {
    "ensure_idle_session": 3,
    "min_idle_session": 2,
    "idle_session_timeout": "10m"
  }
}
```

**高可用性（生产环境）**

包括连接轮换和速率限制的生产环境配置。

```json
{
  "connection_pool": {
    "ensure_idle_session": 5,
    "ensure_idle_session_create_rate": 2,
    "min_idle_session": 3,
    "idle_session_timeout": "5m",
    "max_connection_lifetime": "1h",
    "connection_lifetime_jitter": "10m"
  }
}
```

**资源高效（移动设备、IoT）**

资源受限设备的最小配置。

```json
{
  "connection_pool": {
    "ensure_idle_session": 1,
    "min_idle_session": 1,
    "idle_session_timeout": "2m"
  }
}
```

**与多路复用一起使用**

连接池和多路复用可以一起使用。连接池维护基础 TCP+TLS 连接，而多路复用在它们之上创建多个流。

```json
{
  "multiplex": {
    "enabled": true,
    "max_connections": 2
  },
  "connection_pool": {
    "ensure_idle_session": 3,
    "min_idle_session": 2
  }
}
```

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
