---
icon: material/new-box
---

!!! quote "sing-box 1.14.2.11 中的更改"

    :material-plus: [dns](#dns)

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
