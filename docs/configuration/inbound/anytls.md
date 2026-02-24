---
icon: material/new-box
---

!!! question "Since sing-box 1.12.0"

### Structure

```json
{
  "type": "anytls",
  "tag": "anytls-in",

  ... // Listen Fields

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

### Listen Fields

See [Listen Fields](/configuration/shared/listen/) for details.

### Fields

#### users

==Required==

AnyTLS users.

#### padding_scheme

AnyTLS padding scheme line array.

Default padding scheme:

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

Optional masquerade configuration. When set, non-AnyTLS HTTP connections are handled by the masquerade handler instead of being rejected.

Can be specified as a URL string shorthand or as a structured object.

**URL shorthand:**

- `file:///path/to/dir` — Serve static files from a local directory
- `http://example.com` or `https://example.com` — Reverse proxy to an upstream HTTP/HTTPS server

**Structured object fields:**

##### masquerade.type

==Required==

Masquerade handler type. Available values:

- `file` — Serve static files from a local directory
- `proxy` — Reverse proxy to an upstream server
- `string` — Return a fixed HTTP response body
- `redirect` — Issue an HTTP redirect

##### masquerade (type: `file`)

```json
{
  "type": "file",
  "directory": "/var/www/html"
}
```

| Field | Description |
|-------|-------------|
| `directory` | ==Required== Local filesystem path to serve files from |

##### masquerade (type: `proxy`)

```json
{
  "type": "proxy",
  "url": "https://example.com",
  "rewrite_host": false
}
```

| Field | Description |
|-------|-------------|
| `url` | ==Required== Upstream HTTP/HTTPS URL to proxy requests to |
| `rewrite_host` | Replace the `Host` header with the upstream host. Default: `false` (preserve original `Host`) |

##### masquerade (type: `string`)

```json
{
  "type": "string",
  "status_code": 200,
  "headers": {},
  "content": "Hello, World!"
}
```

| Field | Description |
|-------|-------------|
| `content` | ==Required== Response body string |
| `status_code` | HTTP status code. Default: `200` |
| `headers` | Additional HTTP response headers |

##### masquerade (type: `redirect`)

```json
{
  "type": "redirect",
  "url": "https://example.com",
  "status_code": 302,
  "headers": {},
  "append_request_uri": false
}
```

| Field | Description |
|-------|-------------|
| `url` | ==Required== Redirect target URL |
| `status_code` | HTTP redirect status code. Default: `302` |
| `headers` | Additional HTTP response headers |
| `append_request_uri` | Append the original request path and query to the redirect URL. Default: `false` |

#### tls

TLS configuration, see [TLS](/configuration/shared/tls/#inbound).
