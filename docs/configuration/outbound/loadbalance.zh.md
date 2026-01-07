### 结构

```json
{
  "type": "loadbalance",
  "tag": "lb",

  "primary_outbounds": [
    "proxy-a",
    "proxy-b",
    "proxy-c"
  ],
  "backup_outbounds": [
    "proxy-backup-1",
    "proxy-backup-2"
  ],
  "url": "https://www.gstatic.com/generate_204",
  "interval": "3m",
  "timeout": "5s",
  "idle_timeout": "30m",
  "top_n": {
    "primary": 3,
    "backup": 1
  },
  "strategy": "consistent_hash",
  "hash": {
    "key_parts": ["src_ip", "matched_ruleset_or_etld"],
    "virtual_nodes": 100,
    "on_empty_key": "random",
    "key_salt": ""
  },
  "hysteresis": {
    "primary_failures": 3,
    "backup_hold_time": "5m"
  },
  "empty_pool_action": "reject",
  "interrupt_exist_connections": false
}
```

!!! info "LoadBalance 概述"

    LoadBalance 出站提供智能负载均衡功能，支持健康检查、故障转移和灵活的路由策略。它支持 URL 测试驱动的 Top-N 选择（类似 HAProxy）、一致性哈希和带滞后的自动故障转移。

### 字段

#### primary_outbounds

==必填==

主要出站标签列表。这些出站会持续进行健康检查并按延迟排序。

#### backup_outbounds

备用出站标签列表。只有当主要出站不可用或触发滞后条件时才会使用备用出站。

#### url

健康检查使用的 URL。如果为空则使用 `https://www.gstatic.com/generate_204`。

#### interval

健康检查间隔。如果为空则使用 `3m`。

#### timeout

每个出站的健康检查超时时间。如果为空则使用 `5s`。

#### idle_timeout

空闲超时时间。如果为空则使用 `30m`。

#### top_n

URL 测试驱动的 Top-N 选择配置。这决定了每个池中有多少健康的出站用于负载均衡。

##### top_n.primary

使用的最快主要出站数量。默认：使用所有健康的主要出站。

当设置为 `3` 时，只有延迟最低的 3 个主要出站会被包含在负载均衡池中。

##### top_n.backup

使用的最快备用出站数量。默认：使用所有健康的备用出站。

#### strategy

==必填==

负载均衡策略。可用值：

- `random`：从健康出站中随机选择
- `consistent_hash`：基于连接元数据的一致性哈希

#### hash

一致性哈希配置。仅当 `strategy` 为 `consistent_hash` 时使用。

##### hash.key_parts

==consistent_hash 必填==

用于构建哈希键的元数据字段数组。各部分使用 `|` 分隔符连接。

可用的键部分：

| 键部分 | 描述 | 示例 |
|--------|------|------|
| `src_ip` | 源 IP 地址 | `192.168.1.100` |
| `dst_ip` | 目标 IP 地址 | `8.8.8.8` |
| `src_port` | 源端口号 | `12345` |
| `dst_port` | 目标端口号 | `443` |
| `network` | 网络类型 | `tcp` 或 `udp` |
| `domain` | 完整的目标域名 | `api.example.com` |
| `inbound_tag` | 接受连接的入站标签 | `mixed-in` |
| `matched_ruleset` | 匹配此连接的规则集标签 | `geosite-netflix` |
| `etld_plus_one` | 目标域名的 eTLD+1（基于 PSL） | `example.com` |
| `matched_ruleset_or_etld` | **智能回退**：优先使用匹配的规则集，否则使用 eTLD+1 | 见下文 |

**常见配置：**

- 传统五元组哈希：`["src_ip", "dst_ip", "dst_port"]`
- 按源 IP：`["src_ip"]`
- 按规则集分类：`["src_ip", "matched_ruleset"]`
- 按顶级域名：`["src_ip", "etld_plus_one"]`
- **智能路由**：`["src_ip", "matched_ruleset_or_etld"]`

##### hash.virtual_nodes

一致性哈希环中每个真实出站的虚拟节点数量。更高的值提供更好的分布但使用更多内存。默认：`100`。

推荐值：50-200

##### hash.on_empty_key

哈希键为空时的行为。可用值：

- `random`：选择随机出站（默认）
- `hash_empty`：哈希空字符串（总是选择相同的出站）

##### hash.key_salt

可选的哈希键盐值前缀，用于命名空间隔离。当多个 LoadBalance 实例需要不同的哈希环时很有用。

#### hysteresis

故障转移滞后配置。防止在主备池之间快速切换。

##### hysteresis.primary_failures

故障转移到备用池前的连续主池失败次数。默认：`1`（立即故障转移）。

##### hysteresis.backup_hold_time

返回主池前在备用池上停留的最短时间。默认：`0s`（主池恢复后立即返回）。

#### empty_pool_action

所有出站不可用时采取的操作。可用值：

- `reject`：拒绝连接（默认）
- `direct`：使用直连

#### interrupt_exist_connections

当活动出站池改变时中断现有连接。

只有入站连接受此设置影响，内部连接总是会被中断。

---

## 基于哈希的路由模式

### matched_ruleset_or_etld：智能回退模式

`matched_ruleset_or_etld` 键部分提供智能路由，根据连接是否匹配规则集自适应调整。

**优先级逻辑：**

1. **规则集优先**：如果连接匹配了规则集（例如 `geosite-netflix`、`geosite-openai`），使用规则集标签进行哈希
2. **域名回退**：如果没有匹配规则集，使用公共后缀列表提取 eTLD+1（顶级域名）

**行为示例：**

| 场景 | 输入 | 哈希键部分 | 描述 |
|------|------|-----------|------|
| 匹配规则集 | `api.netflix.com` 匹配 `geosite-netflix` | `geosite-netflix` | 使用规则集标签 |
| 无匹配，有效域名 | `cdn.example.com`（无匹配） | `example.com` | 提取 eTLD+1 |
| 无匹配，子域名 | `a.b.example.com`（无匹配） | `example.com` | 按顶级域名分组 |
| 多部分 TLD | `www.example.co.uk`（无匹配） | `example.co.uk` | PSL 感知提取 |
| IP 地址 | `8.8.8.8`（无匹配） | `-` | IP 占位符 |

**完整示例：**

配置 `["src_ip", "matched_ruleset_or_etld"]`：

- 从 `192.168.1.100` 到 `api.netflix.com` 的请求，规则集 `geosite-netflix` → 哈希键：`192.168.1.100|geosite-netflix`
- 从 `192.168.1.100` 到 `cdn1.example.com` 的请求（无规则集）→ 哈希键：`192.168.1.100|example.com`
- 从 `192.168.1.100` 到 `cdn2.example.com` 的请求（无规则集）→ 哈希键：`192.168.1.100|example.com`（与上面相同）

**优势：**

- **统一配置**：单一哈希模式同时处理基于规则和直连流量
- **内容分类路由**：规则匹配的流量按分类路由（例如所有 Netflix 流量到同一出站）
- **域名分组**：未匹配的流量按顶级域名路由（例如所有 example.com 子域名在一起）
- **会话粘性**：相同来源 + 分类/域名总是使用相同出站

**使用场景：**

- CDN 优化：将 CDN 的所有子域名路由到同一出站
- 服务隔离：为每个来源保持视频流、API 调用和 CDN 流量一致
- 混合路由：智能处理已分类（通过规则集）和未分类流量

---

## 配置示例

### 示例 1：带健康检查的简单轮询

```json
{
  "type": "loadbalance",
  "tag": "lb-simple",
  "primary_outbounds": [
    "proxy-1",
    "proxy-2",
    "proxy-3"
  ],
  "url": "https://www.gstatic.com/generate_204",
  "interval": "3m",
  "strategy": "random"
}
```

在所有健康代理之间随机分配连接。

### 示例 2：Top-N 选择与故障转移

```json
{
  "type": "loadbalance",
  "tag": "lb-topn",
  "primary_outbounds": [
    "proxy-1",
    "proxy-2",
    "proxy-3",
    "proxy-4",
    "proxy-5"
  ],
  "backup_outbounds": [
    "proxy-backup-1",
    "proxy-backup-2"
  ],
  "url": "https://www.gstatic.com/generate_204",
  "interval": "2m",
  "timeout": "5s",
  "top_n": {
    "primary": 3,
    "backup": 1
  },
  "strategy": "random",
  "hysteresis": {
    "primary_failures": 3,
    "backup_hold_time": "5m"
  }
}
```

使用最快的 3 个主代理。连续 3 次失败后回退到最快的备用代理，并在备用上至少停留 5 分钟。

### 示例 3：按源 IP 的一致性哈希

```json
{
  "type": "loadbalance",
  "tag": "lb-per-ip",
  "primary_outbounds": [
    "proxy-1",
    "proxy-2",
    "proxy-3",
    "proxy-4"
  ],
  "url": "https://www.gstatic.com/generate_204",
  "interval": "3m",
  "strategy": "consistent_hash",
  "hash": {
    "key_parts": ["src_ip"],
    "virtual_nodes": 100
  }
}
```

每个源 IP 总是路由到相同的代理（每个客户端的会话粘性）。

### 示例 4：基于规则集的路由

```json
{
  "type": "loadbalance",
  "tag": "lb-by-category",
  "primary_outbounds": [
    "proxy-1",
    "proxy-2",
    "proxy-3",
    "proxy-4"
  ],
  "url": "https://www.gstatic.com/generate_204",
  "interval": "3m",
  "strategy": "consistent_hash",
  "hash": {
    "key_parts": ["src_ip", "matched_ruleset"],
    "virtual_nodes": 100
  }
}
```

按源 IP + 规则集分类路由。相同来源访问相同分类（例如 Netflix）总是使用相同代理。

**需要路由配置：**

```json
{
  "route": {
    "rule_set": [
      {
        "tag": "geosite-netflix",
        "type": "remote",
        "format": "binary",
        "url": "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-netflix.srs"
      }
    ],
    "rules": [
      {
        "rule_set": ["geosite-netflix"],
        "outbound": "lb-by-category"
      }
    ]
  }
}
```

### 示例 5：顶级域名路由

```json
{
  "type": "loadbalance",
  "tag": "lb-by-domain",
  "primary_outbounds": [
    "proxy-1",
    "proxy-2",
    "proxy-3",
    "proxy-4"
  ],
  "url": "https://www.gstatic.com/generate_204",
  "interval": "3m",
  "strategy": "consistent_hash",
  "hash": {
    "key_parts": ["src_ip", "etld_plus_one"],
    "virtual_nodes": 100
  }
}
```

按源 IP + 顶级域名路由。来自同一来源的 example.com 所有子域名使用相同代理。

**示例：**
- `192.168.1.100` → `api.example.com` 和 `cdn.example.com` → 相同代理
- `192.168.1.100` → `example.com` 和 `example.co.uk` → 不同代理

### 示例 6：带规则集回退的智能路由

```json
{
  "type": "loadbalance",
  "tag": "lb-smart",
  "primary_outbounds": [
    "proxy-1",
    "proxy-2",
    "proxy-3",
    "proxy-4"
  ],
  "url": "https://www.gstatic.com/generate_204",
  "interval": "3m",
  "top_n": {
    "primary": 3
  },
  "strategy": "consistent_hash",
  "hash": {
    "key_parts": ["src_ip", "matched_ruleset_or_etld"],
    "virtual_nodes": 100,
    "on_empty_key": "random"
  }
}
```

**智能路由行为：**

- **已分类流量**（通过规则集匹配）：按分类路由
  - Netflix 流量 → `192.168.1.100|geosite-netflix`
  - Google 流量 → `192.168.1.100|geosite-google`

- **未分类流量**（无规则集匹配）：按顶级域名路由
  - `cdn1.example.com` → `192.168.1.100|example.com`
  - `cdn2.example.com` → `192.168.1.100|example.com`（与上面相同）
  - `api.other.com` → `192.168.1.100|other.com`（不同）

**优势：**
- 智能处理基于规则和直连流量
- 无需单独的 LoadBalance 实例
- 已分类和未分类内容的一致路由

---

## 最佳实践

### 健康检查

- **间隔**：大多数场景使用 `3m`，高可用设置使用 `1m`
- **超时**：设置为 `5s` 或更短以快速检测故障
- **URL**：使用轻量级端点如 `/generate_204` 或健康检查端点

### Top-N 选择

- **主要数量**：选择总主要出站的 50-70% 以平衡冗余和性能
- **备用数量**：通常 1-2 个备用就足够了
- **示例**：有 6 个主代理时，使用 `top_n.primary: 4`

### 一致性哈希

- **虚拟节点**：100 是一个好的默认值。如果有很多出站，增加到 200 以获得更好的分布
- **键部分**：根据路由需求选择：
  - 每个客户端的会话持久性：`["src_ip"]`
  - 应用级路由：`["src_ip", "matched_ruleset"]`
  - 域名分组：`["src_ip", "etld_plus_one"]`
  - 智能混合：`["src_ip", "matched_ruleset_or_etld"]`

### 滞后配置

- **主要失败次数**：设置为 `2-3` 以避免短暂故障时的抖动
- **备用保持时间**：设置为 `3m-10m` 以给主要时间恢复
- **示例**：`primary_failures: 3` + `backup_hold_time: 5m` 对大多数情况都有效

### 故障转移策略

- **有备用**：使用 `top_n` 保持最佳主要活跃，需要时回退到备用
- **无备用**：使用 `empty_pool_action: "direct"` 实现优雅降级
- **高可用**：结合 `top_n`、`hysteresis` 和备用池

---

## 技术细节

### 哈希键构建

哈希键通过 `|` 分隔符连接键部分构建：

- 缺失或空值使用 `-` 占位符
- 可选盐值前缀：`{key_salt}{part1}|{part2}|{part3}`
- 哈希函数：xxHash（快速、高质量分布）

**示例：**

| 配置 | 元数据 | 哈希键 |
|------|--------|--------|
| `["src_ip", "dst_port"]` | src=192.168.1.100, dst_port=443 | `192.168.1.100\|443` |
| `["src_ip", "matched_ruleset"]` | src=10.0.0.1, ruleset=geosite-google | `10.0.0.1\|geosite-google` |
| `["src_ip", "etld_plus_one"]` | src=10.0.0.1, domain=api.example.com | `10.0.0.1\|example.com` |
| 带盐值 `"prod-"` | 同上 | `prod-10.0.0.1\|example.com` |

### eTLD+1 提取

`etld_plus_one` 和 `matched_ruleset_or_etld` 键部分使用公共后缀列表（PSL）进行准确的域名提取：

- **基于 PSL**：正确处理多部分 TLD 如 `.co.uk`、`.com.au`、`.gov.uk`
- **规范化**：小写、去除端口、去除尾部点
- **IP 处理**：IPv4 和 IPv6 地址返回 `-` 占位符
- **回退**：如果 PSL 提取失败，返回规范化的完整域名

**提取示例：**

| 输入 | eTLD+1 | 说明 |
|------|--------|------|
| `www.example.com` | `example.com` | 标准域名 |
| `api.v2.example.com` | `example.com` | 深层子域名 |
| `example.co.uk` | `example.co.uk` | 多部分 TLD（PSL） |
| `www.example.co.uk` | `example.co.uk` | 英国子域名 |
| `example.com:443` | `example.com` | 去除端口 |
| `EXAMPLE.COM` | `example.com` | 规范化小写 |
| `192.168.1.1` | `-` | IPv4 占位符 |
| `2001:db8::1` | `-` | IPv6 占位符 |

### 一致性哈希环

- **算法**：NGINX 风格的一致性哈希与虚拟节点
- **虚拟节点**：每个出站在哈希环上获得 N 个位置
- **最小重映射**：当出站改变时，只有 K/N 的连接重新映射（K=总连接数，N=出站数量）
- **稳定性**：相同哈希键 + 相同出站集 → 总是相同出站

### 健康检查状态

出站根据健康检查在状态之间转换：

1. **健康**：在超时时间内成功响应
2. **不健康**：健康检查失败或超时
3. **恢复中**：曾经不健康，现在响应（如果配置了滞后则使用）

Top-N 选择只考虑健康的出站，按响应延迟排序。

---

## 故障排除

### 所有连接都到一个出站

**原因**：哈希键没有变化（例如只使用 `dst_port`，而所有流量都到 443 端口）

**解决方案**：添加更多动态部分如 `src_ip` 或 `domain`

### 连接频繁切换出站

**原因 1**：使用 `random` 策略
**解决方案**：使用 `consistent_hash` 实现会话粘性

**原因 2**：健康检查抖动
**解决方案**：配置 `hysteresis` 以抑制快速切换

### 主要出站失败时没有流量

**原因**：没有配置备用出站且 `empty_pool_action` 为 `reject`

**解决方案**：添加 `backup_outbounds` 或设置 `empty_pool_action: "direct"`

### 哈希结果与预期不同

**原因**：缺少元数据字段（例如 `matched_ruleset` 为空）

**解决方案**：检查路由规则是否设置了预期的元数据。启用调试日志以检查哈希键。
