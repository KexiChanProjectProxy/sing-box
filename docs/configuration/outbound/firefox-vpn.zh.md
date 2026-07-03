### 结构

```json
{
  "type": "firefox-vpn",
  "tag": "firefox-vpn-out",

  "server": "vpn.example.com",
  "server_port": 443,
  "email": "user@example.com",
  "password": "user-password",
  "api_detour": "direct",
  "tls": {
    "enabled": true,
    "server_name": "vpn.example.com"
  },

  ... // 拨号字段
}
```

!!! warning "V1 限制"

    这是 V1 实现。仅支持 **TCP**。UDP、MASQUE、磁盘缓存和服务器列表自动选择在此版本中**不受支持**。

### 字段

#### server

==必填==

Firefox VPN 服务器地址。

#### server_port

==必填==

Firefox VPN 服务器端口。

#### email

==必填==

你的 Firefox 账户邮箱地址。用于向 Firefox Accounts（FxA）服务进行身份验证。

#### password

==必填==

你的 Firefox 账户密码。用于向 Firefox Accounts（FxA）服务进行身份验证。

!!! danger ""

    凭据通过 HTTPS 发送到 Firefox Accounts 和 Guardian 服务。它们永远不会存储在磁盘上。

#### api_detour

用于 Firefox Accounts 和 Guardian API 请求的出站标签。

如果省略，API 请求将通过默认路由表。

#### tls

TLS 配置，参阅 [TLS](/zh/configuration/shared/tls/#outbound)。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
