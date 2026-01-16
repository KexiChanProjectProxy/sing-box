### Structure

```json
{
  "type": "router",
  "tag": "https-router",

  ... // Listen Fields

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

### Listen Fields

See [Listen Fields](/configuration/shared/listen/) for details.

The `listen_port` field is optional. If not specified or set to 0, the router inbound will operate in **internal-only mode** and can only receive connections via detour from other inbounds.

### Fields

#### routes

==Required==

Array of routing rules. Routes are evaluated in priority order (higher priority first). The first matching route wins.

##### name

==Required==

Route name for logging and debugging.

##### match

==Required==

Match criteria for the route. All specified criteria must match (AND logic).

###### path_prefix

Match request path by prefix.

Example: `["/api/", "/v1/"]` matches `/api/users`, `/v1/data`, etc.

###### path_regex

Match request path by regular expression.

Example: `["^/api/v[0-9]+/.*$"]` matches `/api/v1/users`, `/api/v2/data`, etc.

###### host

Match request host/domain with wildcard support.

Example: `["api.example.com", "*.cdn.example.com"]`

Wildcards:
- `*.example.com` matches `api.example.com`, `cdn.example.com`
- `example.*` matches `example.com`, `example.org`

###### header

Match HTTP headers with wildcard support.

Example:
```json
{
  "Authorization": ["Bearer *"],
  "X-Token": ["secret-*", "admin-*"],
  "Upgrade": ["websocket"]
}
```

All specified headers must match. Multiple values are OR'd.

###### method

Match HTTP method.

Example: `["GET", "POST", "PUT"]`

##### target

==Required==

Target inbound tag to forward matched connections to.

!!! warning "Important"

    The target inbound **must be internal-only** (no `listen` or `listen_port` configured). The router will forward raw TCP connections to the target inbound, which then handles protocol processing.

##### strip_path_prefix

Optional path prefix to strip before forwarding to the target inbound.

Example: If `strip_path_prefix` is `/api` and request is `/api/users`, the target inbound receives `/users`.

##### priority

Route priority. Higher values are evaluated first.

Default: `0`

#### fallback

Fallback behavior when no route matches.

Default: Reject with 404 status code.

##### type

Fallback type. Available options:

- `static`: Serve static files from webroot
- `reject`: Return HTTP error status code
- `drop`: Drop connection silently
- `inbound`: Forward to another inbound

##### webroot

For `type: static`. Root directory for static files.

Example: `/var/www/html`

!!! warning "Security"

    Only files within the webroot directory can be served. Path traversal attempts are blocked.

##### index

For `type: static`. Default index filenames.

Default: `["index.html"]`

##### status_code

For `type: reject`. HTTP status code to return.

Default: `404`

Common values: `404` (Not Found), `403` (Forbidden), `500` (Internal Server Error)

##### target

For `type: inbound`. Target inbound tag to forward to.

#### timeout

HTTP server timeout configuration.

##### read

Read timeout for request reading.

Default: `30s`

##### write

Write timeout for response writing.

Default: `30s`

##### idle

Idle timeout for keep-alive connections.

Default: `120s`

#### max_request_body_size

Maximum request body size in bytes.

Default: `0` (unlimited)

Example: `10485760` (10MB)

#### tls

TLS configuration, see [TLS](/configuration/shared/tls/#inbound).

When TLS is enabled, the router operates as an HTTPS server.

### How It Works

The router inbound is an HTTP/HTTPS routing layer that forwards traffic to other **inbounds** based on HTTP request properties. This enables:

- **Protocol Multiplexing**: Run multiple proxy protocols (Trojan, VMess, VLESS) on one HTTP/HTTPS port
- **Path-Based Routing**: Different paths route to different protocols
- **Host-Based Routing**: Different domains route to different protocols
- **Camouflage**: Serve static website while hiding proxy traffic

#### Traffic Flow

```
Client → Router Inbound → Match Route → Target Inbound → Process Protocol → Route to Outbound
                          (HTTP match)   (trojan/vmess)   (trojan decode)   (proxy traffic)
```

The router inbound:
1. Accepts HTTP/HTTPS connections
2. Evaluates routes in priority order
3. Hijacks the connection when a route matches
4. Forwards raw TCP connection to target inbound
5. Target inbound handles protocol-specific processing
6. Traffic then flows through sing-box routing rules to outbound

!!! note "Target Inbound Configuration"

    Target inbounds **must not** have `listen` or `listen_port` configured. They work as internal-only inbounds that accept connections via `NewConnectionEx()`.

### Examples

#### Example 1: Multi-Protocol on One HTTPS Port

Run Trojan, VMess WebSocket, and VLESS on port 443:

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

#### Example 2: Internal Router with Detour

Use HTTP inbound as entry point, route internally:

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

#### Example 3: Domain-Based Routing

Route different domains to different protocols:

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

### Use Cases

1. **Protocol Multiplexing**: Multiple proxy protocols on one port
2. **CDN Camouflage**: Mix proxy traffic with legitimate static website
3. **Path-Based Distribution**: Different paths to different protocols
4. **Domain-Based Routing**: Different domains to different backends
5. **Header-Based Auth**: Route based on authentication tokens

### Performance

- Routes are pre-compiled at startup
- Fast evaluation order: method → path prefix → host → regex → headers
- Zero-copy forwarding where possible
- Connection pooling and reuse

### Troubleshooting

#### Issue: "target inbound not found"

**Solution**: Ensure the target inbound tag matches exactly and the inbound is defined.

#### Issue: "target inbound is not TCP injectable"

**Solution**: Only protocol inbounds (trojan, vmess, vless, etc.) can be targets, not tun or redirect inbounds.

#### Issue: WebSocket not working

**Solution**: Ensure route matches `Upgrade: websocket` header:

```json
{
  "match": {
    "path_prefix": ["/ws"],
    "header": {"Upgrade": ["websocket"]}
  }
}
```

#### Issue: Target inbound has its own listener

**Solution**: Remove `listen` and `listen_port` from target inbound configuration. They should be internal-only.
