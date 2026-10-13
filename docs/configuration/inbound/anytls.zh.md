---
icon: material/new-box
---

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
