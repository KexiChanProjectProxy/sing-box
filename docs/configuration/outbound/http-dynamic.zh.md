`http-dynamic` 出站是一个 HTTP CONNECT 代理客户端。它使用固定的 Basic 认证用户名，并为每个连接根据路由入站元数据生成密码。

### 结构

```json
{
  "type": "http-dynamic",
  "tag": "http-dynamic-out",

  "server": "127.0.0.1",
  "server_port": 1080,
  "username": "fixed-user",
  "path": "",
  "headers": {},
  "tls": {},

  ... // 拨号字段
}
```

### 字段

#### server

==必填==

服务器地址。

#### server_port

==必填==

服务器端口。

#### username

==必填==

固定的 HTTP Basic 认证用户名。

#### 密码派生

每个连接的 Basic 认证密码为以下值的前 16 个小写十六进制字符：

```text
sha256(入站用户名 + 客户端源 IP)
```

客户端源 IP 不包含端口。路由入站没有已认证用户名或有效源 IP 时，出站会拒绝该连接。

#### path

HTTP 请求路径。

#### headers

HTTP 请求的额外标头。默认情况下，CONNECT 请求的 `Host` 标头为配置的代理服务器地址；可通过 `headers.Host` 覆盖。

#### tls

TLS 配置，参阅 [TLS](/zh/configuration/shared/tls/#出站)。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
