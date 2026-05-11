---
icon: material/new-box
---

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
  "masquerade": {},
  "tls": {}
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

#### masquerade

可选的伪装配置。设置后，非 AnyTLS 的 HTTP 连接将由伪装处理器处理，而不是直接拒绝。

可以指定为 URL 字符串简写或结构化对象。

**URL 简写：**

- `file:///path/to/dir` — 从本地目录提供静态文件
- `http://example.com` 或 `https://example.com` — 反向代理到上游 HTTP/HTTPS 服务器

**结构化对象字段：**

##### masquerade.type

==必填==

伪装处理器类型。可用值：

- `file` — 从本地目录提供静态文件
- `proxy` — 反向代理到上游服务器
- `string` — 返回固定的 HTTP 响应体
- `redirect` — 发送 HTTP 重定向

##### masquerade（类型：`file`）

```json
{
  "type": "file",
  "directory": "/var/www/html"
}
```

| 字段 | 描述 |
|------|------|
| `directory` | ==必填== 提供文件服务的本地文件系统路径 |

##### masquerade（类型：`proxy`）

```json
{
  "type": "proxy",
  "url": "https://example.com",
  "rewrite_host": false
}
```

| 字段 | 描述 |
|------|------|
| `url` | ==必填== 代理请求的上游 HTTP/HTTPS URL |
| `rewrite_host` | 将 `Host` 头替换为上游主机。默认：`false`（保留原始 `Host`） |

##### masquerade（类型：`string`）

```json
{
  "type": "string",
  "status_code": 200,
  "headers": {},
  "content": "Hello, World!"
}
```

| 字段 | 描述 |
|------|------|
| `content` | ==必填== 响应体字符串 |
| `status_code` | HTTP 状态码。默认：`200` |
| `headers` | 附加的 HTTP 响应头 |

##### masquerade（类型：`redirect`）

```json
{
  "type": "redirect",
  "url": "https://example.com",
  "status_code": 302,
  "headers": {},
  "append_request_uri": false
}
```

| 字段 | 描述 |
|------|------|
| `url` | ==必填== 重定向目标 URL |
| `status_code` | HTTP 重定向状态码。默认：`302` |
| `headers` | 附加的 HTTP 响应头 |
| `append_request_uri` | 将原始请求的路径和查询参数追加到重定向 URL。默认：`false` |

#### tls

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#入站)。
