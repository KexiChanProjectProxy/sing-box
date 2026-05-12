!!! quote "sing-box 1.11.0 中的更改"

    :material-plus: [server_ports](#server_ports)
    :material-plus: [hop_interval](#hop_interval)
    :material-plus: [realm](#realm)

### 结构

=== "Direct 模式"

    ```json
    {
      "type": "hysteria2",
      "tag": "hy2-out",

      "server": "127.0.0.1",
      "server_port": 1080,
      "server_ports": [
        "2080:3000"
      ],
      "hop_interval": "",
      "up_mbps": 100,
      "down_mbps": 100,
      "obfs": {
        "type": "salamander",
        "password": "cry_me_a_r1ver"
      },
      "password": "goofy_ahh_password",
      "network": "tcp",
      "tls": {},
      "brutal_debug": false,

      ... // 拨号字段
    }
    ```

=== "Realm 模式"

    ```json
    {
      "type": "hysteria2",
      "tag": "hy2-out",

      "realm": {
        "server_url": "hy2://relay.example.com:8443",
        "token": "your_relay_token",
        "realm_id": "my-realm",
        "stun_servers": ["stun.example.com:3478"],
        "prefer_ip_version": "prefer_ipv4",
        "fallback_timeout": "30s"
      },
      "up_mbps": 100,
      "down_mbps": 100,
      "obfs": {
        "type": "salamander",
        "password": "cry_me_a_r1ver"
      },
      "network": "tcp",
      "tls": {},
      "brutal_debug": false,

      ... // 拨号字段
    }
    ```

!!! note ""

    当内容只有一项时，可以忽略 JSON 数组 [] 标签

!!! warning "与官方 Hysteria2 的区别"

    官方程序支持一种名为 **userpass** 的验证方式，
    本质上是将用户名与密码的组合 `<username>:<password>` 作为实际上的密码，而 sing-box 不提供此别名。
    要将 sing-box 与官方程序一起使用， 您需要填写该组合作为实际密码。

### 字段

#### server

==必填==

服务器地址。

当 `realm` 设置时忽略。

#### server_port

==必填==

服务器端口。

如果设置了 `server_ports`，则忽略此项。

当 `realm` 设置时忽略。

#### server_ports

!!! question "自 sing-box 1.11.0 起"

服务器端口范围列表。

与 `server_port` 冲突。

当 `realm` 设置时忽略。

#### hop_interval

!!! question "自 sing-box 1.11.0 起"

端口跳跃间隔。

默认使用 `30s`。

当 `realm` 设置时忽略。

#### up_mbps, down_mbps

最大带宽。

如果为空，将使用 BBR 拥塞控制算法而不是 Hysteria CC。

#### obfs.type

QUIC 流量混淆器类型，仅可设为 `salamander`。

如果为空则禁用。

#### obfs.password

QUIC 流量混淆器密码.

#### password

认证密码。

当 `realm` 设置时忽略。

#### network

启用的网络协议。

`tcp` 或 `udp`。

默认所有。

#### tls

==必填==

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#出站)。

#### brutal_debug

启用 Hysteria Brutal CC 的调试信息日志记录。

### realm

Hysteria2 realm 客户端配置，用于中继连接。

当 `realm` 设置时，`server`、`server_port`、`server_ports`、`hop_interval` 和 `password` 将被忽略。

```json
{
  "server_url": "hy2://relay.example.com:8443",
  "token": "your_relay_token",
  "realm_id": "my-realm",
  "stun_servers": ["stun.example.com:3478"],
  "listen_ports": ["20000-30000", 40000],
  "prefer_ip_version": "prefer_ipv4",
  "fallback_timeout": "30s"
}
```

#### realm.server_url

==必填==

中继服务器 URL，格式为 `hy2://[host]:[port]`。

#### realm.token

中继服务器管理员提供的 realm 认证令牌。

#### realm.realm_id

==必填==

realm 标识符，由中继服务器管理员提供。

#### realm.stun_servers

用于 NAT 类型检测的 STUN 服务器列表。

#### realm.http_client

中继连接的 HTTP 客户端选项。

#### realm.listen_ports

!!! warning "Realm 专属选项"

    此选项仅在 `realm` 模式下可用，不能与 `server_url` 中的 `lport` 查询参数同时使用。

中继连接的本地端口范围。

支持的格式：

- 整数数组：`[8080, 8081, 8082]`
- 字符串范围：`"20000-30000"` 或 `"20000-30000,40000"`
- 特殊值：`"all"` 或 `"*"`（使用临时端口）

端口尝试按顺序进行，而非随机。优先使用列表中第一个可用的端口。

冲突：当 `server_url` 包含 `?lport=` 时不可使用。

#### realm.prefer_ip_version

中继连接的 IP 版本偏好。

| 值             | 描述              |
|----------------|-----------------|
| `ipv4_only`    | 仅使用 IPv4        |
| `ipv6_only`    | 仅使用 IPv6        |
| `prefer_ipv4`  | 优先 IPv4， fallback 到 IPv6 |
| `prefer_ipv6`  | 优先 IPv6， fallback 到 IPv4 |

#### realm.fallback_timeout

当主路径失败时，连接 fallback 超时的持续时间字符串。

接受持续时间字符串（`"30s"`, `"1m"`）或数字秒。零或空将禁用 fallback。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
