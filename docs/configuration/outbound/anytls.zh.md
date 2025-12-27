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
  "ensure_idle_session": 10,
  "tls": {},

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

检查空闲会话的时间间隔。默认值：30秒。

#### idle_session_timeout

在检查中，关闭闲置时间超过此值的会话。默认值：30秒。

#### min_idle_session

在检查中，至少前 `n` 个空闲会话保持打开状态。默认值：`n`=0

**注意**：这是一个**被动保护**机制 - 它只防止现有会话因超时而被关闭。

#### ensure_idle_session

主动维护至少 `n` 个空闲会话在池中。如果当前空闲会话数低于此值，将自动创建新会话。默认值：`n`=0（禁用）

**注意**：这是一个**主动维护**机制 - 它会创建新会话以维持池大小。

**对比**：
- `min_idle_session`：保护现有会话不被超时关闭
- `ensure_idle_session`：创建新会话以达到目标池大小

**示例**：
```json
{
  "min_idle_session": 3,
  "ensure_idle_session": 10,
  "idle_session_timeout": "5m"
}
```

使用此配置：
- 连接池将始终尝试维持 10 个空闲会话
- 即使会话空闲超过 5 分钟，前 3 个也不会被关闭
- 如果池低于 10 个（例如清理后或网络问题），将自动创建新会话

**使用场景**：
- **高流量场景**：预热连接以减少首次请求延迟
- **NAT 保活**：通过 NAT 设备维持持久连接
- **可靠性**：始终有可用的会话

#### tls

==必填==

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#outbound)。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
