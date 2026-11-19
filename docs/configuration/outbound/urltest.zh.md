### 结构

```json
{
  "type": "urltest",
  "tag": "auto",
  
  "outbounds": [
    "proxy-a",
    "proxy-b",
    "proxy-c"
  ],
  "url": "",
  "interval": "",
  "tolerance": 50,
  "idle_timeout": "",
  "interrupt_exist_connections": false,
  "prefer_domain": false,
  "override_ip": "",
  "latency_mode": "",
  "latency_offset": {}
}
```

### 字段

#### outbounds

==必填==

用于测试的出站标签列表。

#### url

用于测试的链接。默认使用 `https://www.gstatic.com/generate_204`。

#### interval

测试间隔。 默认使用 `3m`。

#### tolerance

以毫秒为单位的测试容差。 默认使用 `50`。

#### idle_timeout

空闲超时。默认使用 `30m`。

#### interrupt_exist_connections

当选定的出站发生更改时，中断现有连接。

仅入站连接受此设置影响，内部连接将始终被中断。

#### latency_mode

==可选==

成员延迟的来源。默认值：`url_test`。

| 值 | 行为 |
|---|---|
| `url_test` | 按周期性 URL 测试为成员排序，在检查后切换成员。 |
| `session` | 能测量自身连接的成员（目前为 [`anytls`](/zh/configuration/outbound/anytls/#会话选择)）按新连接将要使用的那条连接的延迟排序，即延迟最低的空闲会话。每个连接都重新选择成员，因此组会在两次检查之间跟随这些延迟变化。不使用 `tolerance`。 |

`session` 模式下，没有已测量空闲会话的成员，或不测量自身连接的成员，按其 URL 测试结果排序。两种模式下 URL 测试都会继续运行，用于回退和发现失效成员。

会话延迟只覆盖到代理服务器这一段，URL 测试覆盖到 `url` 的完整路径，因此混合两种成员的组比较的是不同的量。建议为 `anytls` 成员设置 [`heartbeat`](/zh/configuration/outbound/anytls/#heartbeat)，使其延迟保持最新。

日志级别为 `debug` 时，每个连接都会记录一条 `urltest.select` 事件，包含所选的 `tag`、`latency_ms` 以及来源 `source`（`session` 或 `url_test`）。

#### latency_offset

==可选==

为每个成员人工调整延迟，格式为「成员 tag → 毫秒数」的映射。
正数让该成员更不优先，相当于它的延迟高了这么多；负数让它更优先。

```json
{
  "outbounds": ["hk-1", "jp-1"],
  "latency_offset": {
    "hk-1": 30,
    "jp-1": -20
  }
}
```

两种 `latency_mode` 下，成员都按延迟加偏移量比较，`tolerance` 作用于平移后的值。
只改变成员的选择：已存储的 URL 测试结果和 Clash API 显示的延迟都不受偏移影响。
平移后的值小于零没有问题，不会回绕。

键必须是 `outbounds` 中的 tag，其他键视为配置错误。
没有配置键的成员偏移量为 0。
所选成员配置了偏移量时，`urltest.select` 调试事件还会以 `latency_offset_ms` 记录它。

#### prefer_domain

==可选==

参见 [Dial 字段](/zh/configuration/shared/dial/#prefer_domain)。

#### override_ip

==可选==

参见 [Dial 字段](/zh/configuration/shared/dial/#override_ip)。