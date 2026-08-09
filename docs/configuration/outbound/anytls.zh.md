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

#### min_idle_session_for_age

受基于时间的清理保护的最小空闲会话数。

#### max_connection_lifetime

连接被回收前的最大生命周期。

#### connection_lifetime_jitter

添加到 max_connection_lifetime 的随机抖动，用于避免雷鸣般的群体效应。

#### ensure_idle_session_create_rate

每个时间间隔内创建新空闲会话的最大速率。

#### client_metadata

!!! question "自 sing-box 1.13.16 起"

参阅 [AnyTLS 客户端元数据](/zh/manual/misc/anytls-client-metadata/)。

#### tls

==必填==

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#出站)。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
