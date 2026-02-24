---
icon: material/new-box
---

!!! question "自 sing-box 1.12.0 起"

`cloudflared` 出站通过 [Cloudflare Access](https://developers.cloudflare.com/cloudflare-one/applications/configure-apps/self-hosted-public-app/) 主机名，使用与官方 `cloudflared` 客户端相同的基于 WebSocket 的二进制帧协议，隧道传输 TCP 连接。无需运行 `cloudflared` 子进程。

!!! warning "仅支持 TCP"

    此出站仅支持 TCP，UDP 连接将被拒绝。

### 结构

```json
{
  "type": "cloudflared",
  "tag": "cloudflared-out",

  "hostname": "tunnel.example.com",
  "cloudflared_version": "2026.2.0",

  ... // 拨号字段
}
```

### 字段

#### hostname

==必填==

要连接的 Cloudflare Access 主机名（例如 `tunnel.example.com`）。出站通过 WebSocket over TLS 拨号到 `wss://<hostname>`。

#### cloudflared_version

在 `User-Agent` 头中报告的版本字符串（`cloudflared/<version>`）。此字段控制所模拟的 cloudflared 客户端版本。

默认值：`2026.2.0`。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。

### 配置示例

```json
{
  "type": "cloudflared",
  "tag": "cloudflared-out",
  "hostname": "tunnel.example.com"
}
```

### 工作原理

1. 出站与 `wss://<hostname>` 建立 WebSocket 连接，并携带请求头 `User-Agent: cloudflared/<version>`。
2. 数据以 WebSocket 二进制消息帧格式传输，与官方 cloudflared 客户端的线级协议一致。
3. Cloudflare Access 网关解封装隧道，并将流量转发到源服务。

从 Cloudflare 的角度来看，此连接与合法的 cloudflared 客户端无法区分。
