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
  "min_idle_session_for_age": 1,
  "ensure_idle_session": 5,
  "ensure_idle_session_create_rate": 3,
  "heartbeat": "30s",
  "max_connection_lifetime": "1h",
  "connection_lifetime_jitter": "10m",
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

在空闲超时检查中，至少前 `n` 个空闲会话保持打开状态。默认值：`n`=0

**注意**：这是一个**被动保护**机制 - 它只防止现有会话因**空闲超时**而被关闭。

**用途**：保护会话免于 `idle_session_timeout` 清理。

#### min_idle_session_for_age

在基于年龄的清理中，至少保持 `n` 个空闲会话打开，无论其年龄如何。默认值：`n`=0

**注意**：这是基于年龄清理的**独立保护** - 独立于 `min_idle_session`。

**用途**：保护会话免于 `max_connection_lifetime` 清理。

**与 min_idle_session 的区别**：
- `min_idle_session`：保护免于空闲超时（5分钟不活动）
- `min_idle_session_for_age`：保护免于年龄过期（1小时连接生存时间）

**为何分开**：
- 不同场景需要不同的保护级别
- 您可能想要激进的年龄轮换（低最小值）但宽容的空闲保护（高最小值）
- 或相反：长期保持连接（高年龄最小值）但快速关闭不活动的连接（低空闲最小值）

**示例场景**：

场景 1：**激进轮换，宽容空闲保护**
```json
{
  "min_idle_session": 5,           // 保留 5 个会话免于空闲超时
  "min_idle_session_for_age": 1,   // 但允许激进的年龄轮换（只保留 1 个）
  "max_connection_lifetime": "30m",
  "idle_session_timeout": "10m"
}
```
使用场景：您希望为了安全快速轮换连接，但不希望仅因暂时空闲就关闭会话。

场景 2：**保守轮换，激进空闲清理**
```json
{
  "min_idle_session": 1,           // 激进地关闭空闲会话
  "min_idle_session_for_age": 5,   // 但长期保持连接活跃
  "max_connection_lifetime": "2h",
  "idle_session_timeout": "2m"
}
```
使用场景：您希望维持长期连接，但快速释放真正空闲会话的资源。

场景 3：**平衡**
```json
{
  "min_idle_session": 3,
  "min_idle_session_for_age": 2,   // 年龄上稍微更激进
  "max_connection_lifetime": "1h",
  "idle_session_timeout": "5m"
}
```

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

#### ensure_idle_session_create_rate

当 `ensure_idle_session` 激活时，限制每个清理周期创建的最大会话数。默认值：`0`（无限制 - 一次创建所有缺失的会话）

**解决的问题**：防止连接池需要大量会话时出现连接风暴。

**无速率限制时**（默认）：
- 连接池有 0 个会话，目标是 10 个
- 同时创建所有 10 个会话
- 可能使目标服务器过载、触发速率限制、资源激增

**有速率限制时**：
- 连接池有 0 个会话，目标是 10 个，速率限制为 3
- 周期 1：创建 3 个会话（缺口：10-0=10，限制为 3）
- 周期 2：创建 3 个会话（缺口：10-3=7，限制为 3）
- 周期 3：创建 3 个会话（缺口：10-6=4，限制为 3）
- 周期 4：创建 1 个会话（缺口：10-9=1，创建 1）
- 在 4 个周期内逐渐达到目标

**推荐值**：
```json
{
  "ensure_idle_session": 10,
  "ensure_idle_session_create_rate": 3,  // 每周期最多创建 3 个
  "idle_session_check_interval": "30s"   // 每 30 秒
}
```
结果：每 30 秒创建 3 个会话，直到达到 10 个（最多需要 2 分钟）

**使用场景**：

小速率限制 (1-3)：
- 敏感的目标服务器
- 服务器端严格的速率限制
- 资源受限的环境
- 需要渐进式启动

中等速率限制 (3-5)：
- 平衡的方法
- 大多数生产环境
- 在快速恢复的同时防止峰值

大速率限制 (5-10)：
- 需要快速恢复
- 稳定的目标服务器
- 高容量环境

无限制 (0 - 默认)：
- 测试环境
- 可信的本地网络
- 小池大小（ensure_idle_session < 5）

**配置示例**：

渐进恢复（敏感服务器）：
```json
{
  "ensure_idle_session": 20,
  "ensure_idle_session_create_rate": 2,
  "idle_session_check_interval": "30s"
}
```
每 30 秒创建 2 个会话（填充空池需要 5 分钟）

平衡恢复：
```json
{
  "ensure_idle_session": 10,
  "ensure_idle_session_create_rate": 3,
  "idle_session_check_interval": "30s"
}
```
每 30 秒创建 3 个会话（填充空池需要 2 分钟）

快速恢复：
```json
{
  "ensure_idle_session": 10,
  "ensure_idle_session_create_rate": 5,
  "idle_session_check_interval": "30s"
}
```
每 30 秒创建 5 个会话（填充空池需要 1 分钟）

**调试日志**：
```
[EnsureIdleSession] Current idle sessions: 0, target: 10, deficit=10, rate-limited to creating 3 sessions (will create 7 more in next cycle)
[EnsureIdleSession] Successfully created and pooled session #1 (seq=42)
[EnsureIdleSession] Successfully created and pooled session #2 (seq=43)
[EnsureIdleSession] Successfully created and pooled session #3 (seq=44)
```

**优势**：
- **防止连接风暴**：无大量同时连接
- **对服务器友好**：渐进式启动尊重目标限制
- **资源分布**：随时间分散 CPU/内存/网络负载
- **速率限制弹性**：不会触发服务器端速率限制
- **可预测的行为**：受控的创建速率

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

#### max_connection_lifetime

空闲连接的最大生存时间。启用后，超过此时长的空闲会话将自动关闭，优先关闭较旧的连接。默认值：禁用 (0s)

**工作原理**：
- 追踪每个会话的创建时间
- 定期检查空闲会话的年龄
- 关闭超过配置生存时间的空闲会话
- 优先使用较新的连接（FIFO - 先进先出）
- 优先关闭较旧的连接

**推荐值**：
```json
{
  "max_connection_lifetime": "1h"   // 保守 - 每小时轮换连接
  "max_connection_lifetime": "30m"  // 适中 - 每 30 分钟轮换
  "max_connection_lifetime": "2h"   // 长期 - 适用于稳定环境
}
```

**最佳实践**：
- 根据网络环境和安全要求设置
- 对于动态 IP 或安全关注的环境使用较低值（30分钟-1小时）
- 对于稳定、可信的网络使用较高值（2-4小时）
- 如不需要可禁用（`"max_connection_lifetime": "0"`）
- 与 `ensure_idle_session` 结合使用以在轮换后维持池大小

**使用场景**：
- **连接轮换**：定期刷新连接以防止过期会话
- **负载均衡**：随时间将连接分散到不同的服务器资源
- **安全性**：限制连接生存时间以减少暴露窗口
- **IP 轮换**：通过循环连接适应动态 IP 环境

**与池维护结合的示例**：
```json
{
  "max_connection_lifetime": "1h",
  "ensure_idle_session": 5,
  "min_idle_session": 2
}
```
此配置：
- 关闭超过 1 小时的空闲连接
- 自动创建新会话以维持 5 个空闲会话
- 保护至少 2 个会话免于基于超时的清理
- 结果是每小时自动轮换连接

**重要说明**：
- 基于年龄的清理会遵守 `min_idle_session_for_age` - 不会关闭会话如果这会使数量低于此最小值
- 与 `ensure_idle_session` 无缝协作实现自动轮换
- 清理时优先关闭较旧的连接
- 独立于 `min_idle_session`（后者保护免于空闲超时）

**调试日志**：
设置 `"log": {"level": "debug"}` 查看年龄清理活动：
```
[AgeCleanup] Found 5 expired sessions, closing 3 oldest (keeping 2 to maintain min_idle_session_for_age=2)
[AgeCleanup] Closing session #1 (seq=42, age=1h5m30s, maxLife=55m0s, created=2024-01-01 10:00:00)
```

#### connection_lifetime_jitter

连接生存时间的随机化范围。设置后，每个连接获得 `max_connection_lifetime ± jitter` 的随机生存时间。默认值：禁用 (0s)

**工作原理**：
- 防止"惊群问题"，即所有连接同时过期
- 每个会话在创建时获得唯一的随机化生存时间
- 生存时间 = `max_connection_lifetime` + random(-jitter, +jitter)
- 随时间分散连接关闭
- 减少大量重连造成的负载峰值

**推荐值**：
```json
{
  "max_connection_lifetime": "1h",
  "connection_lifetime_jitter": "10m"   // 连接在 50-70 分钟之间过期
}
```

```json
{
  "max_connection_lifetime": "30m",
  "connection_lifetime_jitter": "5m"    // 连接在 25-35 分钟之间过期
}
```

**最佳实践**：
- 将抖动设置为 max_connection_lifetime 的 10-30%
- 较大的抖动 = 更分散的过期时间
- 较小的抖动 = 更可预测的过期窗口
- 与 `ensure_idle_session` 一起使用以平滑维持池大小

**优势**：
- **避免惊群**：连接不会同时全部过期
- **平滑轮换**：随时间逐步替换连接
- **负载分布**：重连负载分散在时间窗口内
- **更好的稳定性**：没有新连接的突然峰值

**完整配置示例**：
```json
{
  "max_connection_lifetime": "1h",
  "connection_lifetime_jitter": "15m",
  "ensure_idle_session": 10,
  "min_idle_session": 5
}
```
此配置：
- 连接在 45-75 分钟之间过期（1小时 ± 15分钟）
- 通过轮换维持 10 个空闲会话
- 即使过期也始终保留至少 5 个会话
- 将重连分散在 30 分钟窗口内

**调试日志**：
```
[AgeCleanup] Closing session #1 (seq=42, age=67m12s, maxLife=62m30s, created=...)
[AgeCleanup] Closing session #2 (seq=38, age=71m45s, maxLife=68m15s, created=...)
```
注意每个会话由于抖动而有不同的 `maxLife` 值。

#### tls

==必填==

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#outbound)。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。

---

## 会话池管理指南

AnyTLS 提供七个互补功能来管理连接会话：

| 功能 | 类型 | 用途 |
|---------|------|---------|
| `min_idle_session` | **空闲超时保护** | 防止会话因空闲超时而关闭 |
| `min_idle_session_for_age` | **基于年龄保护** | 防止会话因年龄而关闭 |
| `ensure_idle_session` | **主动创建** | 通过创建会话维持最小池大小 |
| `ensure_idle_session_create_rate` | **创建速率限制** | 防止连接池恢复时的连接风暴 |
| `heartbeat` | **保活** | 通过 NAT 保持会话活跃并防止超时 |
| `max_connection_lifetime` | **基于年龄清理** | 限制连接生存时间并轮换旧连接 |
| `connection_lifetime_jitter` | **轮换平滑化** | 随机化生存时间以防止惊群 |

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
