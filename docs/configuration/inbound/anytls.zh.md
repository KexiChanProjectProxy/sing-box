---
icon: material/new-box
---

!!! quote "sing-box 1.14.2.11 中的更改"

    :material-plus: [dns](#dns)  
    :material-plus: [probe](#probe)

!!! quote "sing-box 1.14.2.2 中的更改"

    :material-plus: [fallback](#fallback)

!!! question "自 sing-box 1.12.0 起"

### 结构

```json
{
  "type": "anytls",
  "tag": "anytls-in",

  ... // 监听字段

  "users": [
    {
      "name": "sekai",
      "password": "8JCsPssfgS8tiRwiMlhARg=="
    }
  ],
  "padding_scheme": [],
  "tls": {},
  "fallback": {
    "server": "127.0.0.1",
    "server_port": 8080
  },
  "dns": {
    "enabled": false,
    "server": ""
  },
  "probe": {
    "disabled": false,
    "max_bytes": 33554432,
    "max_duration": "5s",
    "cooldown": "60s",
    "max_concurrent": 4
  }
}
```

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/)。

### 字段

#### users

==必填==

AnyTLS 用户。

#### padding_scheme

AnyTLS 填充方案行数组。

默认填充方案:

```json
[
  "stop=8",
  "0=30-30",
  "1=100-400",
  "2=400-500,c,500-1000,c,500-1000,c,500-1000,c,500-1000",
  "3=9-9,500-1000",
  "4=500-1000",
  "5=500-1000",
  "6=500-1000",
  "7=500-1000"
]
```

#### tls

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#入站)。

如果为空，入站接受明文 AnyTLS 连接。仅应在 sing-box 前置了终止 TLS 的反向代理时使用，例如 nginx `stream` 或 TCP 模式的 HAProxy。

#### fallback

!!! question "自 sing-box 1.14.2.2 起"

回退服务器配置。如果为空，则禁用回退。

AnyTLS 认证失败的连接（包括未知密码或首包格式错误）将携带原始数据转发到此服务器。启用 `tls` 时，服务器收到解密后的数据流。

TLS 握手失败的连接不会被转发。

#### dns

!!! question "自 sing-box 1.14.2.11 起"

应答客户端在 AnyTLS 会话内发送的 DNS 查询。

客户端把每个查询作为原始 DNS 报文放进独立的帧发送，不打开流。
服务器在会话设置中声明支持此功能，因此只有支持它的客户端会发送查询，其他 AnyTLS 客户端不受影响。
客户端请使用 [AnyTLS DNS 服务器](/zh/configuration/dns/server/anytls/)。

##### dns.enabled

应答会话内的 DNS 查询。

##### dns.server

应答查询的 [DNS 服务器](/zh/configuration/dns/server/) 标签。

查询直接发往此服务器，不经过 DNS 规则，但与发往该服务器的其他所有查询共享缓存，包括 sing-box 自身发出的查询。
为此会先移除仅与客户端链路有关的 EDNS 选项（例如 cookie 和 padding），保留客户端子网。
通常选择 [组 DNS 服务器](/zh/configuration/dns/server/group/)。

如果为空，查询经过 [DNS 规则](/zh/configuration/dns/rule/)。
规则可以用 `inbound`、`auth_user` 和 `source_ip_cidr` 匹配它们，这些字段分别为此入站、AnyTLS 用户和客户端地址。

#### probe

!!! question "自 sing-box 1.14.2.11 起"

客户端请求的吞吐探测的限制。除非禁用，探测默认开启。

客户端测量到此服务器的路径时，请求服务器在一个会话中持续发送几秒填充数据（下行），或自己发送填充数据（上行）。
此时发送端要发的数据多于路径能承载的量，它的 TCP_INFO 反映的就是路径本身，而不是应用。
探测期间，该会话会像被一个流占用一样从客户端的连接池中取出。

下行探测在内核的投递速率（只计未受应用限制的样本）连续三个往返增长不到 25% 时提前结束，这与 BBR 退出启动阶段的判断相同。
上行探测在客户端发完允许的数据量时结束，服务器在计完全部数据后应答。
随后发送端等待填充数据被确认，再报告测量结果：

| 键 | 含义 |
|---|---|
| `goodput_bps`、`goodput_us` | 在启动阶段之后、只受网络限制的往返中，每秒被确认的字节数，以及这些往返的总时长。用于比较路径的主要指标。 |
| `avg_rate_bps` | `acked_bytes` 除以 `busy_us`：发送端有数据待发时的速率，无论受什么限制。 |
| `acked_bytes`、`busy_us` | 被确认的字节数，以及发送端有数据待发的时长。 |
| `rwnd_limited_us`、`sndbuf_limited_us`、`net_limited_us` | `busy_us` 中分别受接收端窗口、发送端缓冲区和网络限制的时间。 |
| `peak_bw_bps` | 未受应用限制时采样到的最高投递速率。 |
| `bbr_bw_bps`、`bbr_min_rtt_us` | 发送端使用 BBR 时，结束时 BBR 对路径的估计。其带宽在突发投递的路径上会偏高。 |
| `srtt_us`、`min_rtt_us` | 结束时（负载下）的平滑 RTT，以及无排队时的最低 RTT。 |
| `sent_bytes`、`retrans_bytes`、`dsack_dups`、`rtos` | 发送与重传的字节数、接收端其实已收到的重传，以及重传超时次数。 |
| `cc`、`drained` | 拥塞控制算法，以及填充数据是否在最后一次写入后 3 秒内被确认。 |
| `received_bytes`、`rcv_rtt_us` | 接收端收到的数据及其 RTT 估计。 |

只受网络限制的时间不足 100 毫秒时（例如接收端窗口始终限制着发送端），不报告 `goodput_bps`；此时 `avg_rate_bps` 和各项受限时间说明连接实际达到的速率及原因。
这些时间以内核时钟节拍计，精度为 1 到 10 毫秒。
`srtt_us` 与 `min_rtt_us` 之差是探测造成的排队延迟。

速率单位为字节每秒，时长单位为微秒。服务器内核不报告的键会被省略。

探测的填充数据计入用户流量：它作为一条发往 `probe.anytls.arpa`、经由出站 `anytls-probe`（并不存在）的连接交给流量统计和连接日志。
客户端在上行探测之外发送的填充数据会在会话关闭时以同样方式计入；超过一次探测允许量的会话会被关闭。
超出时长 10 秒仍未结束的探测（例如客户端停止读取），其会话会被关闭。

服务器在会话设置中声明支持探测，因此其他 AnyTLS 客户端不受影响。

##### probe.disabled

不提供探测。

##### probe.max_bytes

单次探测传输的最多填充字节数。

默认使用 `33554432`（32 MiB）。

##### probe.max_duration

单次探测的最长时长。

默认使用 `5s`。

##### probe.cooldown

同一用户在同一方向上两次探测之间的间隔，跨该用户的所有会话计算。

默认使用 `60s`。

##### probe.max_concurrent

此入站同时运行的最多探测数。超出的请求被以繁忙拒绝。

默认使用 `4`。
