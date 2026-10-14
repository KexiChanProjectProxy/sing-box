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
	"heartbeat": "30s",
	"min_idle_session_for_age": 2,
	"max_connection_lifetime": "300s",
	"connection_lifetime_jitter": "10s",
	"ensure_idle_session_create_rate": 5,
	"close_after_idle": "",
	"close_after_lifetime": "",
	"client_metadata": "",
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

#### ensure_idle_session

维护在连接池中的目标空闲会话数量。

#### heartbeat

发送心跳包以保持会话活跃的时间间隔。

每次心跳的往返也会更新会话的延迟，参阅 [会话选择](#会话选择)。

#### min_idle_session_for_age

受基于时间的清理保护的最小空闲会话数。

#### max_connection_lifetime

连接被回收前的最大生命周期。

#### connection_lifetime_jitter

添加到 max_connection_lifetime 的随机抖动，用于避免雷鸣般的群体效应。

#### ensure_idle_session_create_rate

每个时间间隔内创建新空闲会话的最大速率。

#### close_after_idle

关闭空闲时间超过此值的会话。为空时禁用。

与 `idle_session_timeout` 不同，它不受 `min_idle_session` 和 `ensure_idle_session` 保护的限制，因此新连接一直不选用的会话（例如延迟高的会话）总会被替换；`ensure_idle_session` 会补建新的会话。每隔 `idle_session_check_interval` 检查一次。

没有流量时，`ensure_idle_session` 保留的会话每隔 `close_after_idle` 轮换一次。

#### close_after_lifetime

会话存在时间超过此值后不再复用，并将其关闭。为空时禁用。

每个会话的期限在此值的 90% 到 100% 之间随机选取，避免同时建立的会话同时退役。到期后该会话不再分配新连接：空闲时立即关闭，否则在其承载的连接结束后立即关闭，不会打断任何连接。一直保持打开的连接会让其会话存活到连接结束。

与 `max_connection_lifetime` 不同，它不受 `min_idle_session_for_age` 保护的限制。

#### client_metadata

!!! question "自 sing-box 1.13.16 起"

参阅 [AnyTLS 客户端元数据](/zh/manual/misc/anytls-client-metadata/)。

#### tls

==必填==

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#出站)。

### 会话选择

客户端自行测量每个会话的端到端延迟，无需服务端支持。会话建立后立即发送设置帧，以服务端的回应计时；之后每次 [心跳](#heartbeat) 都会追加一个样本，计入平滑平均值。

* 新连接使用延迟最低的空闲会话。尚未测量的会话排在已测量的之后，其中较新的优先。
* 因 `max_connection_lifetime` 关闭会话时，先关闭延迟最高的，未测量的排在它们之前；`min_idle_session_for_age` 保留延迟最低的会话。
* `min_idle_session` 同样保留延迟最低的空闲会话。

`ensure_idle_session` 预建的会话在连接后立即测量，无需先承载流量。未设置 `heartbeat` 时，每个会话只测量一次。

日志级别为 `debug` 时，每个新连接都会记录一条 `anytls.session.selected` 事件，描述所用的会话。时长单位为毫秒，时间为 UTC。

| 字段 | 含义 |
|---|---|
| `session` | 会话编号，从 1 开始 |
| `source` | `pool` 表示复用的空闲会话，`new` 表示为此连接新建的会话 |
| `streams` | 该会话上已打开的连接数，包括本次 |
| `created_at`、`age` | 会话的创建时间和已存在时长 |
| `idle_for` | 复用的会话在此前空闲的时长 |
| `latency`、`latency_last`、`latency_samples` | 平滑延迟、最近一次延迟和样本数；未测量前省略前两项 |
| `max_lifetime`、`expires_at`、`lifetime_left` | 含抖动的会话生命周期、到期时间和剩余时间（到期后为负）；未设置 `max_connection_lifetime` 时省略 |
| `retire_at`、`retire_left` | 会话因 `close_after_lifetime` 停止复用的时间及剩余时间；未设置时省略 |
| `idle_sessions`、`total_sessions` | 选取后的空闲会话数和会话总数 |

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
