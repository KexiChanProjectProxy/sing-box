---
icon: material/new-box
---

!!! question "自 sing-box 1.12.0 起"

### 结构

```json
{
  "type": "anytls",
  "tag": "anytls-out",

  "server": "127.0.0.1",
  "server_port": 1080,
  "password": "8JCsPssfgS8tiRwiMlhARg==",
  "idle_session_check_interval": "30s",
  "idle_session_timeout": "30s",
  "min_idle_session": 5,
  "min_idle_session_for_age": 0,
  "ensure_idle_session": 0,
  "ensure_idle_session_create_rate": 0,
  "heartbeat": "",
  "max_connection_lifetime": "",
  "connection_lifetime_jitter": "",
  "tls": {},
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

AnyTLS 密码。

#### idle_session_check_interval

检查空闲会话的时间间隔。默认值：`30s`。

#### idle_session_timeout

在检查中，关闭闲置时间超过此值的会话。默认值：`30s`。

#### min_idle_session

在检查中，至少保留前 `n` 个空闲会话不关闭。默认值：`0`。

#### min_idle_session_for_age

按生命周期清理会话时保留的最小空闲会话数。若关闭会话会导致空闲会话数低于此值，则不因 `max_connection_lifetime` 而关闭该会话。默认值：`0`。

#### ensure_idle_session

主动维持的目标空闲会话数。客户端将在后台创建新会话以达到此数量。默认值：`0`（禁用）。

#### ensure_idle_session_create_rate

在确保空闲会话时，创建新会话的最大速率（每秒会话数）。默认值：`0`（不限速）。

#### heartbeat

向空闲会话发送保活心跳的间隔，防止中间网络设备关闭空闲连接。默认：禁用。

#### max_connection_lifetime

会话在关闭并替换之前的最大存活时间，用于定期轮换会话。默认：禁用。

#### connection_lifetime_jitter

添加到 `max_connection_lifetime` 的随机抖动，用于分散会话轮换时间，避免惊群效应。默认：禁用。

#### tls

==必填==

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#outbound)。

#### transport

V2Ray 传输层配置，参阅 [V2Ray 传输层](/zh/configuration/shared/v2ray-transport/)。

当设置 `transport` 时，AnyTLS 握手将在传输层连接上进行。`cloudflared` 传输层类型尤其适合将 AnyTLS 流量通过 Cloudflare Access 隧道路由。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
