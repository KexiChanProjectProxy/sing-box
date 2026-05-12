---
icon: material/alert-decagram
---

!!! quote "sing-box 1.11.0 中的更改"

    :material-alert: [masquerade](#masquerade)  
    :material-alert: [ignore_client_bandwidth](#ignore_client_bandwidth)

### 结构

```json
{
  "type": "hysteria2",
  "tag": "hy2-in",
  
  ... // 监听字段

  "up_mbps": 100,
  "down_mbps": 100,
  "obfs": {
    "type": "salamander",
    "password": "cry_me_a_r1ver"
  },
  "users": [
    {
      "name": "tobyxdd",
      "password": "goofy_ahh_password"
    }
  ],
  "ignore_client_bandwidth": false,
  "tls": {},
  "masquerade": "", // 或 {}
  "brutal_debug": false
}
```

!!! warning "与官方 Hysteria2 的区别"

    官方程序支持一种名为 **userpass** 的验证方式，
    本质上是将用户名与密码的组合 `<username>:<password>` 作为实际上的密码，而 sing-box 不提供此别名。
    要将 sing-box 与官方程序一起使用， 您需要填写该组合作为实际密码。

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/)。

### 字段

#### up_mbps, down_mbps

支持的速率，默认不限制。

与 `ignore_client_bandwidth` 冲突。

#### obfs.type

QUIC 流量混淆器类型，仅可设为 `salamander`。

如果为空则禁用。

#### obfs.password

QUIC 流量混淆器密码.

#### users

Hysteria 用户

#### users.password

认证密码。

#### ignore_client_bandwidth

*当 `up_mbps` 和 `down_mbps` 未设定时*:

命令客户端使用 BBR 拥塞控制算法而不是 Hysteria CC。

*当 `up_mbps` 和 `down_mbps` 已设定时*:

禁止客户端使用 BBR 拥塞控制算法。

#### tls

==必填==

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#入站)。

#### masquerade

HTTP3 服务器认证失败时的行为 （URL 字符串配置）。

| Scheme       | 示例                      | 描述      |
|--------------|-------------------------|---------|
| `file`       | `file:///var/www`       | 作为文件服务器 |
| `http/https` | `http://127.0.0.1:8080` | 作为反向代理  |

如果 masquerade 未配置，则返回 404 页。

与 `masquerade.type` 冲突。

#### masquerade.type

HTTP3 服务器认证失败时的行为 （对象配置）。

| Type     | 描述      | 字段                                  |
|----------|---------|-------------------------------------|
| `file`   | 作为文件服务器 | `directory`                         |
| `proxy`  | 作为反向代理  | `url`, `rewrite_host`               |
| `string` | 返回固定响应  | `status_code`, `headers`, `content` |

如果 masquerade 未配置，则返回 404 页。

与 `masquerade` 冲突。

#### masquerade.directory

文件服务器根目录。

#### masquerade.url

反向代理目标 URL。

#### masquerade.rewrite_host

重写请求头中的 Host 字段到目标 URL。

#### masquerade.status_code

固定响应状态码。

#### masquerade.headers

固定响应头。

#### masquerade.content

固定响应内容。

#### brutal_debug

启用 Hysteria Brutal CC 的调试信息日志记录。

### realm

==当使用 Hysteria2 realm 模式时必填==

Hysteria2 realm 服务配置，用于多用户中继和带宽控制。

```json
{
  "server_url": "hy2://example.com:8443",
  "token": "your_relay_token",
  "realm_id": "my-realm",
  "stun_servers": ["stun.example.com:3478"],
  "listen_ports": ["20000-30000", 40000],
  "prefer_ip_version": "prefer_ipv4",
  "fallback_timeout": "30s",
  "stun_domain_resolver": {}
}
```

#### realm.server_url

==必填==

中继服务器 URL，格式为 `hy2://[host]:[port]`。

查询参数：

| 参数   | 描述                            |
|--------|-------------------------------|
| `lport` | 本地端口（与 `listen_ports` 冲突） |

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

中继监听器的本地端口范围。

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

#### realm.stun_domain_resolver

STUN 服务器 DNS 解析的域名解析器选项。
