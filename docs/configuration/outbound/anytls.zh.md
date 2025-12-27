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
  "idle_session_timeout": "5m",
  "min_idle_session": 2,
  "ensure_idle_session": 5,
  "heartbeat": "30s",
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

#### heartbeat

发送周期性心跳包以保持连接活跃并防止 NAT 会话超时。默认值：禁用 (0s)

心跳在**会话级别**运行（底层 TCP+TLS 连接），而非按流运行。所有心跳流量都在 TLS 内加密，无法与常规数据区分。

**工作原理**：
- 客户端按配置间隔发送 `HeartbeatRequest` 帧
- 服务器自动响应 `HeartbeatResponse`
- 通过发送周期性流量保持 NAT 映射活跃
- 对池中的空闲会话和活跃会话均有效

**推荐值**：
```json
{
  "heartbeat": "30s"   // 保守 - 适用于大多数 NAT（60秒以上超时）
  "heartbeat": "10s"   // 激进 - 适用于严格的 NAT（30秒超时）
  "heartbeat": "60s"   // 适中 - 适用于稳定连接和长超时
}
```

**最佳实践**：
- 设置心跳间隔**小于** NAT 超时时间（例如：60秒 NAT 超时设置 30秒心跳）
- 移动网络：使用 10-30秒（运营商通常有较短超时）
- 稳定连接：使用 30-60秒
- 如不需要可禁用（`"heartbeat": "0"`）以节省带宽

**开销**：
- 每个心跳帧约 7 字节
- CPU 影响极小（基于定时器）
- 示例：`30s` 间隔 = 每会话约 1.8 KB/小时

**调试日志**：
设置 `"log": {"level": "debug"}` 查看心跳活动：
```
[Heartbeat] Sending heartbeat request
[Heartbeat] Heartbeat sent successfully
[Heartbeat] Received heartbeat response
```

#### tls

==必填==

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#outbound)。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。

---

## 会话池管理指南

AnyTLS 提供三个互补功能来管理连接会话：

| 功能 | 类型 | 用途 |
|---------|------|---------|
| `min_idle_session` | **被动保护** | 防止现有会话因超时而关闭 |
| `ensure_idle_session` | **主动创建** | 通过创建会话维持最小池大小 |
| `heartbeat` | **保活** | 通过 NAT 保持会话活跃并防止超时 |

### 配置策略

#### 策略 1：高性能（低延迟）

适用于：游戏、实时交易、低延迟应用

```json
{
  "type": "anytls",
  "server": "example.com",
  "server_port": 443,
  "password": "your-password",

  "ensure_idle_session": 5,
  "min_idle_session": 3,
  "idle_session_timeout": "10m",
  "idle_session_check_interval": "30s",
  "heartbeat": "20s",

  "tls": {
    "enabled": true,
    "server_name": "example.com"
  }
}
```

**优势**：
- 始终有 5 个预热连接就绪
- 首次请求即时响应（无需 TLS 握手）
- 激进的心跳防止任何超时
- 受保护的最小值确保基线可用性

#### 策略 2：高可用性（生产环境）

适用于：生产服务、关键应用、24/7 运行时间

```json
{
  "type": "anytls",
  "server": "example.com",
  "server_port": 443,
  "password": "your-password",

  "ensure_idle_session": 10,
  "min_idle_session": 5,
  "idle_session_timeout": "15m",
  "idle_session_check_interval": "1m",
  "heartbeat": "30s",

  "tls": {
    "enabled": true,
    "server_name": "example.com"
  }
}
```

**优势**：
- 大型池处理突发流量
- 受保护的最小值防止过度清理
- 适中的心跳平衡保活与开销
- 从网络中断自动恢复

#### 策略 3：NAT 穿透（移动网络）

适用于：移动设备、不稳定网络、严格的 NAT

```json
{
  "type": "anytls",
  "server": "example.com",
  "server_port": 443,
  "password": "your-password",

  "ensure_idle_session": 3,
  "min_idle_session": 2,
  "idle_session_timeout": "5m",
  "idle_session_check_interval": "30s",
  "heartbeat": "10s",

  "tls": {
    "enabled": true,
    "server_name": "example.com"
  }
}
```

**优势**：
- 激进的心跳（10秒）保持 NAT 会话活跃
- 小池最小化移动数据使用
- 快速恢复（30秒检查间隔）
- 适用于 30秒 NAT 超时的运营商

#### 策略 4：资源高效（物联网/边缘）

适用于：资源受限设备、最小开销、物联网

```json
{
  "type": "anytls",
  "server": "example.com",
  "server_port": 443,
  "password": "your-password",

  "ensure_idle_session": 1,
  "min_idle_session": 1,
  "idle_session_timeout": "2m",
  "idle_session_check_interval": "1m",
  "heartbeat": "45s",

  "tls": {
    "enabled": true,
    "server_name": "example.com"
  }
}
```

**优势**：
- 最小池（1 个会话）节省内存
- 激进的超时（2分钟）快速释放资源
- 仍然维持单个就绪连接
- 心跳防止连接丢失

#### 策略 5：平衡（通用）

适用于：通用用途、平衡性能

```json
{
  "type": "anytls",
  "server": "example.com",
  "server_port": 443,
  "password": "your-password",

  "ensure_idle_session": 5,
  "min_idle_session": 2,
  "idle_session_timeout": "5m",
  "idle_session_check_interval": "30s",
  "heartbeat": "30s",

  "tls": {
    "enabled": true,
    "server_name": "example.com"
  }
}
```

**优势**：
- 适中的池大小（5）处理正常流量
- 标准心跳（30秒）适用于大多数 NAT
- 合理的超时（5分钟）平衡资源
- 大多数部署的良好起点

### 功能交互

**功能如何协同工作**：

```
每 30秒（idle_session_check_interval）：

  1. 清理阶段：
     - 查找空闲 > 5分钟（idle_session_timeout）的会话
     - 关闭多余会话
     - 保留前 2 个（min_idle_session）即使空闲 > 5分钟

  2. 确保阶段：
     - 统计当前空闲会话
     - 如果数量 < 5（ensure_idle_session）：
       → 创建新会话达到 5 个

同时（持续）：
  - 心跳每 30秒（heartbeat）发送保活
  - 防止 NAT 超时
  - 作用于所有会话（空闲和活跃）
```

### 监控和调试

启用调试日志以监控会话池行为：

```json
{
  "log": {
    "level": "debug"
  }
}
```

**预期日志消息**：

```
// 会话池维护
[DEBUG] [EnsureIdleSession] Current idle sessions: 3, target: 5, creating 2 new sessions
[DEBUG] [EnsureIdleSession] Successfully created and pooled session #1 (seq=42)

// 心跳活动
[DEBUG] [Heartbeat] Sending heartbeat request
[DEBUG] [Heartbeat] Heartbeat sent successfully
[DEBUG] [Heartbeat] Received heartbeat response
```

### 故障排除

**问题**：会话不断被关闭

**解决方案**：增加 `min_idle_session` 或 `idle_session_timeout`

```json
{
  "min_idle_session": 5,        // 保护更多会话
  "idle_session_timeout": "10m"  // 更长超时
}
```

**问题**：流量突发时会话不足

**解决方案**：增加 `ensure_idle_session`

```json
{
  "ensure_idle_session": 10  // 维持更大的池
}
```

**问题**：连接通过 NAT 掉线

**解决方案**：减少 `heartbeat` 间隔

```json
{
  "heartbeat": "10s"  // 更激进的保活
}
```

**问题**：内存/带宽使用量高

**解决方案**：减少池大小和心跳频率

```json
{
  "ensure_idle_session": 2,
  "idle_session_timeout": "2m",
  "heartbeat": "60s"
}
```

### 性能指标

| 配置 | 内存 | 带宽 | 延迟 | 可用性 |
|--------------|--------|-----------|---------|--------------|
| 高性能 | ~50 KB | ~10 KB/小时 | 极低 | 很高 |
| 高可用性 | ~100 KB | ~20 KB/小时 | 极低 | 最高 |
| NAT 穿透 | ~30 KB | ~30 KB/小时 | 低 | 高 |
| 资源高效 | ~10 KB | ~8 KB/小时 | 低 | 中等 |
| 平衡 | ~50 KB | ~18 KB/小时 | 低 | 高 |

**说明**：
- 内存：每会话开销约 10 KB（TLS 状态 + 缓冲区）
- 带宽：仅心跳流量（每次 7 字节）
- 延迟：连接建立时间（TLS 握手）
- 可用性：网络问题期间的会话可用性
