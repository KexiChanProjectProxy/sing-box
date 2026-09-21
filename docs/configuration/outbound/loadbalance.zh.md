`loadbalance` 出站是一个分组：对成员出站做健康检查，始终优先使用健康的主出站，并按一致性哈希或随机策略为每条连接选择成员。

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
  "timeout": "15s",
  "idle_timeout": "30m",
  "tolerance": 10,
  "sorter": {
    "latency": 1
  },
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

!!! quote ""

    该分组会显示在 [Clash API](/zh/configuration/experimental/clash-api/) 中（`now` / `all`）。`now` 是当前池中排序最优的候选；首次健康检查前则是第一个主出站。不能通过 API 手动选择成员。

### 字段

#### primary_outbounds

==必填==

主出站标签列表。至少需要一个标签。健康的主出站总是优先于备用出站。

不能包含本分组自己的标签。不能与 `backup_outbounds` 重复。允许嵌套分组（`selector`、`urltest`、`loadbalance`）。

#### backup_outbounds

==可选==

备用出站标签列表。备用出站仅在没有任何主候选出站健康时使用。所有健康的备用出站都会进入候选池，不受 `top_n` 限制。

不能包含本分组自己的标签。不能与 `primary_outbounds` 重复。

#### url

==可选==

健康检查测试 URL。默认使用 `https://www.gstatic.com/generate_204`。

#### interval

==可选==

健康检查间隔。默认使用 `3m`。

必须小于或等于 `idle_timeout`。

#### timeout

==可选==

不健康延迟阈值。没有已存储延迟、延迟为 0，或延迟大于等于该值的候选视为不健康。默认使用 `15s`。

HTTP 探测本身始终使用 15 秒截止时间，与该字段无关。

#### idle_timeout

==可选==

空闲超时时间。当检测到没有流量时，健康检查将停止，下一次连接时恢复。默认使用 `30m`。

#### top_n

==可选==

Top N 候选选择选项。仅使用 `primary`。

#### top_n.primary

==可选==

按评分选择前 N 个健康主出站（见 [sorter](#sorter)）。`0` 表示使用所有健康主出站。默认：`0`。

#### top_n.backup

不支持。必须省略或为 `0`。

#### tolerance

==可选==

选择 Top N 候选集合时的评分容差，单位为毫秒。

首次快照直接取评分最优的 N 个健康主出站。
之后，更优的出站只有在比当前候选好出超过该值时才会将其替换；差值等于该值时保留现有候选。
默认使用 `10`。

评分以毫秒为等价单位（见 [sorter](#sorter)），因此无论 sorter 依据哪些指标排序，该值的含义都保持不变。

#### sorter

==可选==

排序权重，格式为「指标关键词 → 权重」的映射。排序（`top_n`、排序顺序、`now`、`tolerance`）使用由此得到的评分，评分越低越优先。健康检查、超时判断以及连接级故障转移仍使用原始实测延迟。

省略时默认使用 `{"latency": 1}`，即仅按最近一次实测延迟排序。配置为空对象会报错。

成员的评分是各项指标的加权和：

```
score = Σ 权重 × 指标
```

所有权重均为正数，其含义是**该指标每 1 个单位折合多少毫秒**。数值越高越好的指标会被减去而不是加上，因此评分始终越低越优，并且与它所替代的原始延迟保持在同一量纲上。

```json
{
  "sorter": {
    "latency_avg_1m": 1,
    "client_rtt": 0.5,
    "server_loss_rate_1m": 20,
    "server_delivery_rate": 0.1
  }
}
```

可读作：主要按 1 分钟平均延迟排序；传输层 RTT 每 1 毫秒折合 0.5 毫秒；下行丢包每 1% 折合 20 毫秒；实测吞吐每 1 Mbps 抵扣 0.1 毫秒。

支持的关键词：

| 关键词                  | 单位 | 窗口        | 优劣方向 |
|------------------------|------|------------|----------|
| `latency`              | ms   | 最近一次     | 越低越优 |
| `latency_avg_1m`       | ms   | 1m 平均     | 越低越优 |
| `latency_avg_5m`       | ms   | 5m 平均     | 越低越优 |
| `client_rtt`           | ms   | 1m 平均     | 越低越优 |
| `client_rttvar`        | ms   | 1m 平均     | 越低越优 |
| `client_loss_rate_30s` | %    | 30s 累计    | 越低越优 |
| `client_loss_rate_1m`  | %    | 1m 累计     | 越低越优 |
| `client_loss_rate_5m`  | %    | 5m 累计     | 越低越优 |
| `client_delivery_rate` | Mbps | 1m 峰值     | 越高越优 |
| `server_rtt`           | ms   | 1m 平均     | 越低越优 |
| `server_rttvar`        | ms   | 1m 平均     | 越低越优 |
| `server_loss_rate_30s` | %    | 30s 累计    | 越低越优 |
| `server_loss_rate_1m`  | %    | 1m 累计     | 越低越优 |
| `server_loss_rate_5m`  | %    | 5m 累计     | 越低越优 |
| `server_delivery_rate` | Mbps | 1m 峰值     | 越高越优 |

窗口按 10 秒一桶累计，因此实际覆盖范围介于「名义时长减去一桶」与名义时长之间。丢包率为窗口内丢失包数除以发送包数；RTT 类关键词取窗口内样本的平均值；吞吐取窗口内最高的那一桶，因为代理连接通常受限于应用的实际需求而非链路本身。

未知关键词或负权重会导致配置错误，权重全为零同样如此。单个关键词权重为零时直接忽略该关键词。

!!! note "哪些成员会上报传输层统计"

    只有 [`hysteria2`](/zh/configuration/outbound/hysteria2/) 与 [`anytls`](/zh/configuration/outbound/anytls/) 成员会上报 `client_*` 与 `server_*`。其他成员在这些关键词上按池内平均值计分；嵌套分组按其当前选中的成员测量。

    | 关键词            | `hysteria2`              | `anytls`                   |
    |------------------|--------------------------|----------------------------|
    | `*_rtt`          | QUIC 平滑 RTT              | `TCP_INFO` RTT             |
    | `*_rttvar`       | QUIC RTT 平均偏差           | `TCP_INFO` RTT 抖动          |
    | `*_loss_rate_*`  | 判定丢失的包数 / 发送包数       | 重传分段数 / 发送的数据分段数        |
    | `*_delivery_rate`| 实测吞吐                     | 内核投递速率估计                  |

    `client_*` 在本机读取：`hysteria2` 在所有平台可用，`anytls` 需要 Linux。`server_*` 由服务端经协议回传，需要支持该功能的 sing-box 服务端；`anytls` 还要求服务端运行在 Linux 上。该请求在建立连接时协商，因此只覆盖分组启动之后新建的连接。不支持的服务端会忽略该请求，此时该成员在 `server_*` 上按池内平均值计分。

    需要注意的限制：

    - 配置了 `detour` 的 `anytls` 成员不上报 `client_*`，因为其底层套接字属于另一个传输。
    - 自上次采样以来发送少于 32 个包的连接（例如只有保活、心跳或统计交换本身）不计入该次采样，除非它发出的包至少一半丢失且至少丢失两个。因此流量很少的成员在其早先样本移出窗口后按池内平均值计分，而不是按这些噪声计分；而重传开始失败的 `anytls` 成员会在一段时间内显示其丢包，直到退避后的重传过于稀疏，它同样回到池内平均值。完全不再投递的 `hysteria2` 链路在这里不会显示丢包，因为 QUIC 通过探测定时器发现这类丢包且不上报；这种情况由健康检查发现。
    - 对于 `hysteria2`，若某次获取服务端统计的请求失败，该连接剩余时间内服务端方向不再上报，下一条连接时恢复。

    当分组中没有任何成员能上报时，启动时会记录 `loadbalance.sorter.unsupported`。

!!! note "延迟类关键词"

    `latency` 是最近一次健康检查结果。`latency_avg_1m` 与 `latency_avg_5m` 取对应时间窗内实测结果的平均值；窗口内没有样本时回退到最近一次结果。

    只有当窗口内包含多次探测时平均值才有意义，因此在按平均值排序前应把 `interval` 调低到 `15s`–`30s` 左右。当 `interval` 超过所配置的最窄窗口的一半时，启动时会记录 `loadbalance.sorter.coarse_interval` 警告；`interval` 默认为 3 分钟，对上述两个窗口都会触发该警告。

!!! note "传输层关键词"

    `client_*` 在本机测量，反映客户端到服务端方向；`server_*` 由代理服务端测量并经协议回传，反映服务端到客户端方向——下行丢包仅靠客户端自身是看不到的。

    两者都只覆盖本机与代理服务器之间的这一跳。某成员无法上报某项指标时，该项按其余可上报成员的平均值计分，既不会因此占优也不会被惩罚；所有成员都无法上报的关键词不会影响排序结果。

    不在当前候选集合中的成员没有用户流量，其传输层指标经常是缺失的。成员本身是分组时，按其当前选中的出站来测量；对于嵌套的 `loadbalance`，即其排序最优的成员，以它代表整个候选池。

    评分相同的成员先按原始延迟、再按 tag 排序。当 sorter 无法区分成员时这一点很重要——例如没有成员上报相应关键词，或多个成员都未上报而取得相同的池内平均值——此时仍由延迟而不是 tag 名称决定顺序。`loadbalance.sorter.unsupported` 警告只在启动时记录一次，且仅当没有任何成员实现上报接口时才会出现。

!!! note "查看评分"

    在 `debug` 日志级别下，每轮健康检查后分组都会输出各成员的评分及每个关键词的贡献值，可据此调整权重。

#### weighted_delay

==已弃用==

请改用 [sorter](#sorter)。该配置会被转换为等价的 sorter；同时配置两者会报错。

存在该对象时，排序延迟会把窗口平均值与最近一次采样按比例混合。转换保留了混合比例，但不保留窗口定义：`weighted_delay` 按采样个数计窗口，而 sorter 的 `latency_avg_5m` 是固定的 5 分钟时间窗。由于转换结果固定为 `latency_avg_5m`，`interval` 默认的 3 分钟会触发 [sorter](#sorter) 一节所述的 `loadbalance.sorter.coarse_interval` 警告。

```json
{"weighted_delay": {"window_weight": 7, "last_weight": 3}}
```

等价于

```json
{"sorter": {"latency_avg_5m": 0.7, "latency": 0.3}}
```

#### weighted_delay.window

==已弃用==

窗口内的采样数量。默认使用 `5`。必须在 `1` 到 `64` 之间。转换时忽略该值，固定使用 5 分钟时间窗。

#### weighted_delay.window_weight

==已弃用==

窗口内采样之和的权重。默认使用 `1`。

#### weighted_delay.last_weight

==已弃用==

最新一次采样的权重。该采样同时计入窗口之和。默认使用 `1`。

#### strategy

==可选==

选择策略。支持的值：`consistent_hash`、`random`。默认：`consistent_hash`。

`strategy` 为 `random` 时忽略 `hash`。

当 `strategy` 为 `consistent_hash`（默认）但 `hash.key_parts` 为空时，哈希键为空，选择行为遵循 `hash.on_empty_key`（默认 `random`）。需要会话亲和性时必须设置 `hash.key_parts`。

#### hash

==可选==

一致性哈希选项。`strategy` 为 `random` 时忽略。

#### hash.key_parts

==可选==

用于构建哈希键的部分，以 `|` 连接。空的部分会被跳过。

支持的值：

* `src_ip`：客户端源 IP
* `matched_ruleset_or_etld`：第一个匹配的 [规则集](/zh/configuration/rule-set/) 标签；若没有规则集匹配，则为探测到的域名 / 目标 FQDN 的 eTLD+1

无默认值。空列表（包括省略 `hash`）会产生空键。

#### hash.virtual_nodes

==可选==

哈希环中每个候选的虚拟节点数量。较高的值可以提高分布均匀性。默认：`100`。

#### hash.on_empty_key

==可选==

哈希键为空时的行为。`random` 随机选择一个候选；`error` 返回拨号错误。默认：`random`。

#### hash.key_salt

==可选==

添加到哈希键和虚拟节点名称前面的盐。默认：`""`。

#### empty_pool_action

==可选==

没有健康候选时的行为。支持的值：`error`、`random`。`error` 不产生首选，连接直接进入[连接级故障转移](#连接级故障转移)，依次尝试全部已配置的主出站、再尝试全部已配置的备用出站；只有全部失败才报错，未配置任何出站时立即报错。`random` 从所有已配置的主出站和备用出站中随机选一个作为首选（不进行健康过滤），失败后再按故障转移尝试其余成员。默认：`error`。

#### interrupt_exist_connections

==可选==

当健康候选**集合**（成员）发生变化时，中断现有连接；同一集合内后续连接哈希到不同成员时不会中断。

仅入站连接受此设置影响，内部连接将始终被中断。

#### prefer_domain

==可选==

参见 [Dial 字段](/zh/configuration/shared/dial/#prefer_domain)。

在连接委派给所选成员之前，于本分组生效。

#### override_ip

==可选==

参见 [Dial 字段](/zh/configuration/shared/dial/#override_ip)。

`prefer_domain` 与 `override_ip` 互斥。

### 启动行为

出站立即启动，并用全部主出站预填充候选池。后台健康检查随后用健康的 Top-N 集合替换该预填充。尚未产生延迟的嵌套 `loadbalance` / `urltest` 成员会保留预填充。仅在已有健康结果且没有任何候选健康时，才使用 `empty_pool_action`。叶子成员 HTTP 探测失败仍会清空候选池，但在默认的 `error` 下，连接仍会先按[连接级故障转移](#连接级故障转移)尝试全部已配置成员，全部失败才报错。

### 健康检查

成员探测在后台进行。与 [`urltest`](/zh/configuration/outbound/urltest/) 不同，loadbalance 在拨号、代理握手和目标 TLS 完成之后才测量 HTTP RTT。

仅当已存储延迟存在、不为 0，且严格小于 `timeout` 时，该成员才视为健康。健康判断从不依赖 [sorter](#sorter)：先按原始延迟过滤，再对通过的成员计分排序。探测失败和拨号失败会删除该成员已存储的延迟；任何被判定为不健康而被过滤掉的成员——包括延迟只是达到 `timeout` 的成员——其延迟历史都会被重置，以免在其质量下降后仍按之前的平均值排序。拨号失败还会让当前连接改试下一个成员（见[连接级故障转移](#连接级故障转移)）。候选池在每一轮健康检查之后重建，拨号失败不会触发重建，因此传输层指标会在下一轮才进入排序。

嵌套的 `loadbalance` 和 `urltest` 成员不会被父分组 HTTP 探测。父分组复用子分组当前的延迟（其快照中最低的原始延迟，或 urltest 最近一次历史）。而传输层关键词是通过嵌套分组当前选中的出站来测量的，因此对于嵌套的 `loadbalance`，两者可能对应不同的成员。嵌套 `selector` 视为叶子：在新鲜历史可用时复用当前选中出站的历史，否则对该出站做 HTTP 探测。

### 连接级故障转移

选中的成员在转发任何业务数据之前失败时，同一条连接会改试下一个成员，而不是直接失败。触发条件包括成员拨号（或 UDP 监听）返回的错误：TCP 连不上、TLS 或代理握手失败、认证失败、被拒绝、超时。

尝试顺序在连接开始时确定：

1. 按 `strategy` 和当前候选池选出的首选，与没有故障转移时完全相同。
2. 其余全部已配置的 `primary_outbounds` 成员。
3. 其余全部已配置的 `backup_outbounds` 成员，仅在所有主出站都失败之后。

第 2、3 步内部按最近一次测得的原始延迟升序排列，延迟相同按 tag 排序；没有测量值的成员排在最后，按 tag 排序。这里不使用 `sorter`、`top_n`、`tolerance` 和健康过滤：拨号本身已经在失败，因此回退到最简单的信号；只要仍在配置中，不健康的成员也会被尝试。即使备用出站更快，只要还有未尝试的主出站，就不会先试备用出站。

每个成员在一条连接上最多尝试一次。不支持当前网络的成员会被跳过。嵌套的 `loadbalance`、`urltest` 或 `selector` 算作一个成员，由其自身处理内部成员。全部失败时返回最后一次尝试的错误。

一旦某个成员成功，故障转移即结束。隧道建立之后的错误（连接重置、空响应、目标站 HTTP 错误等）不会触发故障转移。连接被取消时停止后续尝试。

单条连接的失败不会重建候选池、不会改变一致性哈希亲和，也不会打断其他连接。下一条连接仍从当前候选池选首选；失败的成员只会因延迟被删除，并在下一轮健康检查之后离开候选池。

故障转移没有总超时。最坏情况下，一条连接会依次等待每个成员的连接超时，上限为调用方自身的截止时间。

每次尝试失败都会记录带成员 tag 的 `urltest.error` 事件。连接最终由首选以外的成员建立时，记录 info 级 `loadbalance.failover` 事件，包含 `selected`（候选池为空时为空字符串）、`used` 和 `tried`。连接在多次尝试后仍失败时，记录 error 级 `loadbalance.failover.exhausted` 事件，包含 `selected`、`tried` 和最后一次错误。

!!! warning "1.14.0.15 行为变更"

    `empty_pool_action: error` 时，候选池为空不再立即让拨号失败。依赖「全员不健康即快速失败」的部署，现在会等待真实的拨号尝试。

### 主备语义

健康的主出站总是优先于备用出站。备用出站仅在没有任何主候选出站健康时使用。

当 `top_n.primary` 为 `0` 或不小于健康主出站数量时，使用全部健康主出站。否则候选池为评分最优的 N 个健康主出站（见 [sorter](#sorter)），首次快照之后受 `tolerance` 迟滞约束。

备用出站不做 Top-N：若没有健康的主出站，则使用全部健康备用出站。

### 一致性哈希

使用 `consistent_hash` 策略时，只要候选集合不变，相同的哈希键始终选择相同的候选。当某个候选被移除时，只有映射到该候选的键会被重新映射。

未设置 `hash.key_parts` 时，退化为 `hash.on_empty_key`（默认随机）。

### 随机策略

使用 `random` 策略时，每次连接请求从当前健康候选池中随机选择一个候选。不进行哈希键计算，不提供会话亲和性。每次选择相互独立。
