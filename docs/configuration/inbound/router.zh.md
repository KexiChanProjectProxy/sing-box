### 结构

```json
{
  "type": "router",
  "tag": "https-router",

  ... // 监听字段

  "routes": [
    {
      "name": "trojan-route",
      "match": {
        "path_prefix": ["/api/"],
        "host": ["api.example.com"],
        "header": {
          "X-Token": ["secret-*"]
        },
        "path_regex": ["^/v[0-9]+/.*$"],
        "method": ["GET", "POST"]
      },
      "target": "trojan-in",
      "strip_path_prefix": "/api",
      "priority": 100
    }
  ],
  "fallback": {
    "type": "static",
    "webroot": "/var/www/html",
    "index": ["index.html", "index.htm"],
    "status_code": 404
  },
  "timeout": {
    "read": "30s",
    "write": "30s",
    "idle": "120s"
  },
  "max_request_body_size": 10485760,
  "tls": {}
}
```

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/) 以了解详情。

`listen_port` 字段是可选的。如果未指定或设置为 0，路由入站将以**仅内部模式**运行，只能通过其他入站的 detour 接收连接。

### 字段

#### routes

==必填==

路由规则数组。路由按优先级顺序评估（优先级高的先评估）。第一个匹配的路由获胜。

##### name

==必填==

路由名称，用于日志和调试。

##### match

==必填==

路由的匹配条件。所有指定的条件必须匹配（AND 逻辑）。

###### path_prefix

按前缀匹配请求路径。

示例：`["/api/", "/v1/"]` 匹配 `/api/users`、`/v1/data` 等。

###### path_regex

按正则表达式匹配请求路径。

示例：`["^/api/v[0-9]+/.*$"]` 匹配 `/api/v1/users`、`/api/v2/data` 等。

###### host

使用通配符支持匹配请求主机/域名。

示例：`["api.example.com", "*.cdn.example.com"]`

通配符：
- `*.example.com` 匹配 `api.example.com`、`cdn.example.com`
- `example.*` 匹配 `example.com`、`example.org`

###### header

使用通配符支持匹配 HTTP 头。

示例：
```json
{
  "Authorization": ["Bearer *"],
  "X-Token": ["secret-*", "admin-*"],
  "Upgrade": ["websocket"]
}
```

所有指定的头必须匹配。多个值使用 OR 逻辑。

###### method

匹配 HTTP 方法。

示例：`["GET", "POST", "PUT"]`

##### target

==必填==

将匹配的连接转发到的目标入站标签。

!!! warning "重要"

    目标入站**必须是仅内部的**（未配置 `listen` 或 `listen_port`）。路由器将原始 TCP 连接转发到目标入站，然后目标入站处理协议解析。

##### strip_path_prefix

可选的路径前缀，在转发到目标入站之前剥离。

示例：如果 `strip_path_prefix` 是 `/api` 且请求是 `/api/users`，则目标入站接收 `/users`。

##### priority

路由优先级。值越高越先评估。

默认值：`0`

#### fallback

当没有路由匹配时的回退行为。

默认值：返回 404 状态码拒绝。

##### type

回退类型。可用选项：

- `static`：从 webroot 提供静态文件
- `reject`：返回 HTTP 错误状态码
- `drop`：静默断开连接
- `inbound`：转发到另一个入站

##### webroot

用于 `type: static`。静态文件的根目录。

示例：`/var/www/html`

!!! warning "安全"

    只能提供 webroot 目录内的文件。路径遍历尝试会被阻止。

##### index

用于 `type: static`。默认索引文件名。

默认值：`["index.html"]`

##### status_code

用于 `type: reject`。要返回的 HTTP 状态码。

默认值：`404`

常见值：`404`（未找到）、`403`（禁止）、`500`（内部服务器错误）

##### target

用于 `type: inbound`。要转发到的目标入站标签。

#### timeout

HTTP 服务器超时配置。

##### read

请求读取的读超时。

默认值：`30s`

##### write

响应写入的写超时。

默认值：`30s`

##### idle

保持活动连接的空闲超时。

默认值：`120s`

#### max_request_body_size

最大请求体大小（字节）。

默认值：`0`（无限制）

示例：`10485760`（10MB）

#### tls

TLS 配置，参阅 [TLS](/zh/configuration/shared/tls/#inbound)。

启用 TLS 时，路由器作为 HTTPS 服务器运行。

### 工作原理

路由入站是一个 HTTP/HTTPS 路由层，根据 HTTP 请求属性将流量转发到其他**入站**。这实现了：

- **协议复用**：在一个 HTTP/HTTPS 端口上运行多个代理协议（Trojan、VMess、VLESS）
- **基于路径的路由**：不同路径路由到不同协议
- **基于主机的路由**：不同域名路由到不同协议
- **伪装**：在隐藏代理流量的同时提供静态网站

#### 流量流向

```
客户端 → 路由入站 → 匹配路由 → 目标入站 → 处理协议 → 路由到出站
          (HTTP匹配)   (trojan/vmess)  (trojan解码)  (代理流量)
```

路由入站：
1. 接受 HTTP/HTTPS 连接
2. 按优先级顺序评估路由
3. 当路由匹配时劫持连接
4. 将原始 TCP 连接转发到目标入站
5. 目标入站处理特定协议的解析
6. 流量然后通过 sing-box 路由规则流向出站

!!! note "目标入站配置"

    目标入站**不得**配置 `listen` 或 `listen_port`。它们作为仅内部的入站工作，通过 `NewConnectionEx()` 接受连接。

### 示例

#### 示例 1：一个 HTTPS 端口上的多协议

在 443 端口上运行 Trojan、VMess WebSocket 和 VLESS：

```json
{
  "inbounds": [
    {
      "type": "router",
      "tag": "https-router",
      "listen": "0.0.0.0",
      "listen_port": 443,
      "tls": {
        "enabled": true,
        "server_name": "example.com",
        "certificate_path": "/path/to/cert.pem",
        "key_path": "/path/to/key.pem"
      },
      "routes": [
        {
          "name": "trojan-route",
          "match": {
            "path_prefix": ["/trojan"]
          },
          "target": "trojan-in",
          "priority": 100
        },
        {
          "name": "vmess-ws",
          "match": {
            "path_prefix": ["/vmess"],
            "header": {
              "Upgrade": ["websocket"]
            }
          },
          "target": "vmess-in",
          "priority": 90
        },
        {
          "name": "vless-route",
          "match": {
            "path_prefix": ["/vless"]
          },
          "target": "vless-in",
          "priority": 80
        }
      ],
      "fallback": {
        "type": "static",
        "webroot": "/var/www/html",
        "index": ["index.html"]
      }
    },
    {
      "type": "trojan",
      "tag": "trojan-in",
      "users": [
        {"password": "your-password"}
      ]
    },
    {
      "type": "vmess",
      "tag": "vmess-in",
      "users": [
        {"uuid": "your-uuid"}
      ]
    },
    {
      "type": "vless",
      "tag": "vless-in",
      "users": [
        {"uuid": "your-uuid"}
      ]
    }
  ]
}
```

#### 示例 2：使用 Detour 的内部路由器

使用 HTTP 入站作为入口点，内部路由：

```json
{
  "inbounds": [
    {
      "type": "http",
      "tag": "http-entry",
      "listen": "0.0.0.0",
      "listen_port": 8080,
      "detour": "internal-router"
    },
    {
      "type": "router",
      "tag": "internal-router",
      "routes": [
        {
          "name": "api-trojan",
          "match": {
            "path_prefix": ["/api/"]
          },
          "target": "trojan-in",
          "priority": 100
        }
      ],
      "fallback": {
        "type": "reject",
        "status_code": 404
      }
    },
    {
      "type": "trojan",
      "tag": "trojan-in",
      "users": [{"password": "password"}]
    }
  ]
}
```

#### 示例 3：基于域名的路由

将不同域名路由到不同协议：

```json
{
  "inbounds": [
    {
      "type": "router",
      "tag": "https-router",
      "listen": "0.0.0.0",
      "listen_port": 443,
      "tls": {
        "enabled": true,
        "server_name": "*.example.com"
      },
      "routes": [
        {
          "name": "api-domain",
          "match": {
            "host": ["api.example.com"]
          },
          "target": "trojan-in",
          "priority": 100
        },
        {
          "name": "cdn-domain",
          "match": {
            "host": ["cdn.example.com"]
          },
          "target": "vless-in",
          "priority": 100
        },
        {
          "name": "wildcard",
          "match": {
            "host": ["*.example.com"]
          },
          "target": "vmess-in",
          "priority": 50
        }
      ]
    },
    {
      "type": "trojan",
      "tag": "trojan-in",
      "users": [{"password": "password"}]
    },
    {
      "type": "vless",
      "tag": "vless-in",
      "users": [{"uuid": "uuid"}]
    },
    {
      "type": "vmess",
      "tag": "vmess-in",
      "users": [{"uuid": "uuid"}]
    }
  ]
}
```

### 使用场景

1. **协议复用**：在一个端口上运行多个代理协议
2. **CDN 伪装**：将代理流量与合法静态网站混合
3. **基于路径的分发**：不同路径到不同协议
4. **基于域名的路由**：不同域名到不同后端
5. **基于头的认证**：根据认证令牌路由

### 性能

- 路由在启动时预编译
- 快速评估顺序：method → path prefix → host → regex → headers
- 尽可能使用零拷贝转发
- 连接池和重用

### 故障排除

#### 问题："target inbound not found"

**解决方案**：确保目标入站标签完全匹配且入站已定义。

#### 问题："target inbound is not TCP injectable"

**解决方案**：只有协议入站（trojan、vmess、vless 等）可以作为目标，而不是 tun 或 redirect 入站。

#### 问题：WebSocket 不工作

**解决方案**：确保路由匹配 `Upgrade: websocket` 头：

```json
{
  "match": {
    "path_prefix": ["/ws"],
    "header": {"Upgrade": ["websocket"]}
  }
}
```

#### 问题：目标入站有自己的监听器

**解决方案**：从目标入站配置中删除 `listen` 和 `listen_port`。它们应该是仅内部的。
