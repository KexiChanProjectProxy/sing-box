---
icon: material/new-box
---

!!! question "Since sing-box 1.12.0"

The `cloudflared` outbound tunnels TCP connections through a [Cloudflare Access](https://developers.cloudflare.com/cloudflare-one/applications/configure-apps/self-hosted-public-app/) hostname using the same WebSocket-based binary framing protocol as the official `cloudflared` client. No `cloudflared` subprocess is required.

!!! warning "TCP only"

    This outbound only supports TCP. UDP connections will be rejected.

### Structure

```json
{
  "type": "cloudflared",
  "tag": "cloudflared-out",

  "hostname": "tunnel.example.com",
  "cloudflared_version": "2026.2.0",

  ... // Dial Fields
}
```

### Fields

#### hostname

==Required==

The Cloudflare Access hostname to connect through (e.g. `tunnel.example.com`). The outbound dials `wss://<hostname>` using WebSocket over TLS.

#### cloudflared_version

The version string reported in the `User-Agent` header (`cloudflared/<version>`). This controls which cloudflared client version is impersonated.

Default: `2026.2.0`.

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.

### Configuration Example

```json
{
  "type": "cloudflared",
  "tag": "cloudflared-out",
  "hostname": "tunnel.example.com"
}
```

### How It Works

1. The outbound establishes a WebSocket connection to `wss://<hostname>` with the header `User-Agent: cloudflared/<version>`.
2. Data is framed as WebSocket binary messages, matching the wire protocol of the official cloudflared client.
3. The Cloudflare Access gateway decapsulates the tunnel and forwards traffic to the origin service.

This makes connections indistinguishable from a legitimate cloudflared client from Cloudflare's perspective.
