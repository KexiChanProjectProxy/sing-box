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
  "ensure_idle_session": 0,
  "ensure_idle_session_create_rate": 0,
  "max_connection_lifetime": "",
  "connection_lifetime_jitter": "",
  "min_idle_session_for_age": 0,
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

!!! note "自 sing-box 1.12.15 起新增"

#### ensure_idle_session

主动维护池中目标数量的空闲会话。当池低于此阈值时，自动创建新会话。

与 `min_idle_session`（清理期间的被动保护）不同，这提供主动会话创建，以实现高可用性和低延迟连接建立。

默认值：`0`（禁用）

示例：`10` 维护至少 10 个预热连接可供使用。

#### ensure_idle_session_create_rate

限制池恢复期间的会话创建速率。每个清理周期创建的最大会话数。

这可以防止恢复会话池时的连接风暴，保护目标服务器免受突发峰值的影响。

- `0`：无限制（默认，立即创建所有需要的会话）
- `1-3`：慢速恢复（推荐用于有速率限制的目标）
- `3-5`：平衡恢复
- `5-10`：快速恢复

默认值：`0`（无限制）

#### max_connection_lifetime

会话自动关闭前的最大寿命。超过此生命周期的会话将被关闭，最旧的会话首先关闭。

这与 `ensure_idle_session` 配合使用以实现自动连接轮换。

优点：
- 安全性：限制连接暴露窗口
- 负载均衡：在动态后端之间分配
- NAT 弹性：定期会话更新

示例：`1h`（会话存活最多 1 小时）

默认值：无限制（无基于寿命的过期）

#### connection_lifetime_jitter

随机化连接生命周期以防止惊群效应。每个会话获得随机生命周期：`基础值 ± 抖动值`。

这将过期分布在时间窗口内，防止同时重新连接峰值，并确保随时间平滑的连接轮换。

使用 `max_connection_lifetime: 1h` 和 `connection_lifetime_jitter: 15m` 的示例：
- 会话将存活 45-75 分钟
- 过期在 30 分钟窗口内平滑分布

默认值：`0`（无随机化）

#### min_idle_session_for_age

保护免受基于寿命关闭的最小空闲会话数（与空闲超时保护分开）。

这允许对不同清理场景进行独立控制：
- `min_idle_session`：保护免受空闲超时关闭
- `min_idle_session_for_age`：保护免受基于寿命的关闭

用例：
- 激进的寿命轮换 + 宽松的空闲保护
- 保守的寿命轮换 + 激进的空闲清理
- 两种场景的平衡保护

默认值：`0`（使用与 `min_idle_session` 相同的值）

#### tls

==必填==

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#outbound)。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
