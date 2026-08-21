### 结构

```json
{
  "type": "loadbalance",
  "tag": "my-lb",

  "primary_outbounds": [
    "proxy-a",
    "proxy-b"
  ],
  "backup_outbounds": [
    "proxy-c"
  ],
  "url": "https://www.gstatic.com/generate_204",
  "interval": "3m",
  "timeout": "5s",
  "idle_timeout": "30m",
  "tolerance": 10,
  "top_n": {
    "primary": 0
  },
  "strategy": "consistent_hash",
  "hash": {
    "key_parts": ["src_ip", "matched_ruleset_or_etld"],
    "virtual_nodes": 100,
    "on_empty_key": "random",
    "key_salt": ""
  },
  "empty_pool_action": "error",
  "interrupt_exist_connections": false,
  "prefer_domain": false,
  "override_ip": ""
}
```

### 字段

#### primary_outbounds

==必填==

主出站标签列表。健康的主出站总是优先于备用出站。

#### backup_outbounds

==可选==

备用出站标签列表。备用出站仅在没有任何主候选出站健康时使用。

#### url

==可选==

健康检查测试 URL。默认使用 `https://www.gstatic.com/generate_204`。

#### interval

==可选==

健康检查间隔。默认使用 `3m`。

#### timeout

==可选==

健康检查超时时间。如果候选出站延迟超过此值，则视为不健康。默认使用 `5s`。

#### idle_timeout

==可选==

空闲超时时间。当检测到没有流量时，健康检查将停止。默认使用 `30m`。

#### top_n

==可选==

Top N 候选选择选项。

#### top_n.primary

==可选==

按延迟选择前 N 个健康主出站。`0` 表示使用所有健康主出站。默认：`0`。

#### tolerance

==可选==

选择 Top N 候选集合时的延迟容差，单位为毫秒。
更快的出站只有在比当前候选快出超过该值时才会将其替换。
默认使用 `10`。

#### strategy

==可选==

选择策略。支持的值：`consistent_hash`、`random`。默认：`consistent_hash`。

#### hash

==可选==

一致性哈希选项。

#### hash.key_parts

==可选==

用于构建哈希键的部分。支持的值：`src_ip`、`matched_ruleset_or_etld`。默认：`["src_ip", "matched_ruleset_or_etld"]`。

#### hash.virtual_nodes

==可选==

哈希环中每个候选的虚拟节点数量。较高的值可以提高分布均匀性。默认：`100`。

#### hash.on_empty_key

==可选==

哈希键为空时的行为。`random` 随机选择一个候选；`error` 返回拨号错误。默认：`random`。

#### hash.key_salt

==可选==

添加到哈希键输入前面的盐，用于额外随机化。默认：`""`。

#### empty_pool_action

==可选==

没有健康候选时的行为。支持的值：`error`、`random`。`error` 拨号失败并返回错误；`random` 从所有已配置的主出站和备用出站中随机选择，不进行健康过滤。默认：`error`。

#### interrupt_exist_connections

==可选==

当选定的出站发生更改时，中断现有连接。

仅入站连接受此设置影响，内部连接将始终被中断。

#### prefer_domain

==可选==

通过选定的出站优先进行域名解析。默认：`false`。

#### override_ip

==可选==

参见 [Dial 字段](/zh/configuration/shared/dial/#override_ip)。

### 启动行为

出站立即启动，并用全部主出站预填充候选池。后台健康检查随后用健康的 Top-N 集合替换该预填充。仅在已有健康结果且没有任何候选健康时，才使用 `empty_pool_action`。

### 主备语义

健康的主出站总是优先于备用出站。备用出站仅在没有任何主候选出站健康时使用。

### 一致性哈希

使用 `consistent_hash` 策略时，只要候选集合不变，相同的哈希键始终选择相同的候选。当某个候选被移除时，只有映射到该候选的键会被重新映射。

### 随机策略

使用 `random` 策略时，每次连接请求从当前健康候选池中随机选择一个候选。不进行哈希键计算，不提供会话亲和性。每次选择相互独立。