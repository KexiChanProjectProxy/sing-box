`socks` 入站是一个 socks4, socks4a 和 socks5 服务器.

### 结构

```json
{
  "type": "socks",
  "tag": "socks-in",

  ... // 监听字段

  "users": [
    {
      "username": "admin",
      "password": "admin"
    }
  ],
  "auth_bypass": [
    "127.0.0.1/32",
    "192.168.0.0/16"
  ]
}
```

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/)。

### 字段

#### users

SOCKS 用户

如果为空则不需要验证。

#### auth_bypass

跳过认证的来源 IP 地址或 CIDR 前缀列表。

来自匹配地址的连接即使设置了 `users` 也无需凭据即可接入。IPv4 映射的 IPv6 地址按 IPv4 匹配。
