---
icon: material/scale-balance
---

!!! question "自 sing-box 1.12.15 起"

### 结构

```json
{
  "type": "loadbalance",
  "tag": "load-balance",

  "primary_outbounds": [
    "proxy-a",
    "proxy-b",
    "proxy-c"
  ],
  "backup_outbounds": [
    "proxy-backup-1",
    "proxy-backup-2"
  ],
  "strategy": "random",
  "url": "",
  "interval": "",
  "timeout": "",
  "idle_timeout": "",
  "top_n": {
    "primary": 0,
    "backup": 0
  },
  "hash": {
    "key_parts": [],
    "virtual_nodes": 10,
    "on_empty_key": "random",
    "salt": ""
  },
  "hysteresis": {
    "primary_failures": 3,
    "backup_hold_time": "30s"
  },
  "empty_pool_action": "error",
  "interrupt_exist_connections": false
}
```

### 字段

#### primary_outbounds

==必填==

主层级出站标签列表。正常运行时使用这些出站。

#### backup_outbounds

备用层级出站标签列表。仅当所有主出站都失败时才使用这些出站。

遵循 HAProxy 备用语义：仅当所有主出站都不可用时才激活备用出站。

#### strategy

负载均衡策略。可用选项：
- `random`：均匀随机分布（默认）
- `consistent_hash`：基于哈希的路由，具有会话亲和性

默认值：`random`

#### url

用于健康检查的 URL。如果为空，将使用 `https://www.gstatic.com/generate_204`。

#### interval

健康检查间隔。如果为空，将使用 `3m`。

#### timeout

健康检查超时。如果为空，将使用 `5s`。

#### idle_timeout

连接的空闲超时。如果为空，将使用 `30m`。

#### top_n

每层级的 Top-N 选择配置。

##### top_n.primary

从主层级选择的最快出站数量。`0` 表示使用所有可用出站。默认值：`0`

##### top_n.backup

从备用层级选择的最快出站数量。`0` 表示使用所有可用出站。默认值：`0`

#### hash

一致性哈希配置。仅当 `strategy` 为 `consistent_hash` 时适用。

##### hash.key_parts

构建哈希键的元数据部分数组。可用部分：

- `src_ip`：源 IP 地址
- `dst_ip`：目标 IP 地址
- `dst_port`：目标端口号
- `network`：网络类型（tcp/udp）
- `domain`：完整目标域名
- `inbound_tag`：入站连接标签
- `matched_ruleset`：匹配的规则集标签（用于基于规则的路由）
- `etld_plus_one`：顶级域名（例如，从 "www.example.com" 提取 "example.com"）
- `matched_ruleset_or_etld`：智能回退（如果匹配规则集则使用规则集，否则使用 eTLD+1）
- `salt`：自定义盐值字符串

示例：`["src_ip", "dst_port"]` 创建类似 "192.168.1.100|443" 的键

##### hash.virtual_nodes

每个出站的虚拟节点数，用于更好的分布。默认值：`10`

更高的值提供更均匀的分布，但使用更多内存。

##### hash.on_empty_key

哈希键为空时的行为：
- `random`：随机选择（默认）
- `hash_empty`：使用空字符串作为键

默认值：`random`

##### hash.salt

添加到哈希键的自定义盐值字符串，用于额外的随机化。

#### hysteresis

滞后配置，以防止层级抖动。

##### hysteresis.primary_failures

切换到备用层级之前需要的连续失败次数。默认值：`3`

##### hysteresis.backup_hold_time

切换回主层级之前在备用层级停留的最短时间。默认值：`30s`

防止主出站间歇性可用时快速来回切换。

#### empty_pool_action

没有候选出站可用时的行为：
- `error`：返回错误（故障关闭，默认）
- `fallback_all`：使用所有配置的出站（故障开放）

默认值：`error`

#### interrupt_exist_connections

当选定的候选出站更改时中断现有连接。

仅入站连接受此设置影响，内部连接将始终被中断。

---

## 功能

### 双层级系统

LoadBalance 使用类似 HAProxy 的主/备用层级系统：

- **主层级**：正常运行时使用
- **备用层级**：仅当所有主出站都不可用时才激活

这提供了无需手动干预的自动故障转移。

### Top-N 选择

根据健康检查延迟从每个层级选择 N 个最快的出站：

```json
{
  "top_n": {
    "primary": 3,
    "backup": 2
  }
}
```

这会选择 3 个最快的主出站和 2 个最快的备用出站，通过仅使用性能最佳的出站来提高性能。

### 负载均衡策略

#### 随机策略

在可用候选出站之间均匀分布连接。

- 简单快速
- 无会话亲和性
- 适合无状态协议

#### 一致性哈希策略

提供会话亲和性 - 相同输入始终路由到相同出站：
- 使用虚拟节点实现均匀分布
- 池变化时重新映射最少（移除 25% 节点时重新映射约 25%）
- 可从连接元数据配置哈希键

### 基于哈希的路由

使用 `consistent_hash` 策略时，您可以根据各种元数据路由流量：

**按源 IP 路由：**
```json
"hash": {
  "key_parts": ["src_ip"]
}
```

**按目标路由：**
```json
"hash": {
  "key_parts": ["dst_ip", "dst_port"]
}
```

**按域名路由：**
```json
"hash": {
  "key_parts": ["etld_plus_one"]
}
```

**智能路由（规则集与域名回退）：**
```json
"hash": {
  "key_parts": ["src_ip", "matched_ruleset_or_etld"]
}
```

当规则匹配时按规则集路由流量，但对于不匹配的流量回退到按域名路由。

### 启动模式

LoadBalance 立即从所有主出站开始，防止启动期间出现"无可用候选出站"错误。第一次健康检查完成后，它切换到基于健康的选择。

### 网络过滤

根据网络能力自动过滤候选出站：
- TCP 连接仅使用支持 TCP 的出站
- UDP 连接仅使用支持 UDP 的出站

### Clash API 兼容性

实现 `URLTest` 方法以实现 Clash API 兼容性，启用 `/proxies/:name/delay` 端点进行手动健康检查。

---

## 配置示例

### 示例 1：简单随机负载均衡

```json
{
  "outbounds": [
    {
      "type": "loadbalance",
      "tag": "load-balance",
      "primary_outbounds": ["proxy-1", "proxy-2", "proxy-3"],
      "strategy": "random"
    }
  ]
}
```

### 示例 2：基于源 IP 的一致性哈希

确保相同的源 IP 始终使用相同的出站。

```json
{
  "type": "loadbalance",
  "tag": "load-balance",
  "primary_outbounds": ["proxy-a", "proxy-b", "proxy-c"],
  "strategy": "consistent_hash",
  "hash": {
    "key_parts": ["src_ip"],
    "virtual_nodes": 50
  }
}
```

### 示例 3：主/备用与 Top-N

```json
{
  "type": "loadbalance",
  "tag": "load-balance",
  "primary_outbounds": ["proxy-1", "proxy-2", "proxy-3", "proxy-4", "proxy-5"],
  "backup_outbounds": ["backup-1", "backup-2"],
  "strategy": "random",
  "top_n": {
    "primary": 3,
    "backup": 1
  },
  "hysteresis": {
    "primary_failures": 3,
    "backup_hold_time": "30s"
  }
}
```

此配置：
- 从主层级选择 3 个最快的出站
- 当所有主出站连续失败 3 次时回退到最快的备用出站
- 在切换回主出站之前至少在备用模式下停留 30 秒

### 示例 4：一致性哈希路由

```json
{
  "type": "loadbalance",
  "tag": "hash-balance",

  "primary_outbounds": ["proxy-a", "proxy-b", "proxy-c"],
  "strategy": "consistent_hash",
  "hash": {
    "key_parts": ["src_ip", "dst_port"],
    "virtual_nodes": 50,
    "on_empty_key": "random"
  }
}
```

这提供会话亲和性：来自相同源 IP 到相同目标端口的连接将始终使用相同的出站（除非它变得不可用）。

### 示例 5：基于规则的哈希路由

```json
{
  "type": "loadbalance",
  "tag": "rule-hash",

  "primary_outbounds": ["proxy-streaming", "proxy-gaming", "proxy-general"],
  "strategy": "consistent_hash",
  "hash": {
    "key_parts": ["src_ip", "matched_ruleset"],
    "virtual_nodes": 30
  }
}
```

结合路由规则，这允许不同的服务类型为每个用户粘附到特定的出站。

### 示例 6：智能回退路由

```json
{
  "type": "loadbalance",
  "tag": "smart-balance",

  "primary_outbounds": ["proxy-1", "proxy-2", "proxy-3"],
  "strategy": "consistent_hash",
  "hash": {
    "key_parts": ["src_ip", "matched_ruleset_or_etld"],
    "virtual_nodes": 40
  }
}
```

`matched_ruleset_or_etld` 部分提供智能回退：
- 对于匹配规则集的连接：使用规则集标签进行哈希
- 对于其他连接：使用 eTLD+1 域名进行哈希

这为基于规则的连接和直接连接提供统一配置，同时保持会话粘性。

### 示例 7：基于域名的路由

```json
{
  "type": "loadbalance",
  "tag": "domain-balance",

  "primary_outbounds": ["proxy-a", "proxy-b", "proxy-c"],
  "strategy": "consistent_hash",
  "hash": {
    "key_parts": ["src_ip", "etld_plus_one"],
    "virtual_nodes": 50
  }
}
```

来自同一用户到相同顶级域名（例如，所有 *.example.com）的连接将使用相同的出站。这对于维护与 CDN 和多域名服务的会话状态很有用。

---

## 高级主题

### 启动模式

LoadBalance 支持在第一次健康检查完成之前立即处理连接。启动时：
1. 所有主出站都可用于选择
2. 选择使用配置的策略（random 或 consistent_hash）
3. 第一次健康检查后，选择切换到基于健康的候选出站

这可以防止启动期间出现"无可用候选出站"错误。

### 网络过滤

LoadBalance 根据连接类型自动过滤候选出站：
- TCP 连接：仅使用支持 TCP 的出站
- UDP 连接：仅使用支持 UDP 的出站

### Clash API 兼容性

LoadBalance 实现 `URLTest` 方法以实现 Clash API 兼容性，支持：
- 通过 `/proxies/:name/delay` 端点进行手动健康检查
- 实时延迟监控
- 与 Clash 兼容的仪表板集成

### 性能注意事项

- **虚拟节点**：更多虚拟节点（50-100）提供更好的分布，但使用更多内存
- **Top-N 选择**：限制为前 3-5 个出站可减少开销，同时保持良好性能
- **检查间隔**：更长的间隔（5m-10m）减少目标服务器的负载
- **滞后**：防止过度切换；根据您的网络稳定性进行调整
