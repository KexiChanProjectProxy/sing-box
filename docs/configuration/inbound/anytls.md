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
  "tls": {},
  "masquerade": {}
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

#### tls

TLS configuration, see [TLS](/configuration/shared/tls/#inbound).

TLS is optional. When not configured, AnyTLS operates in plain TCP mode, which is useful when TLS is terminated upstream by a reverse proxy like nginx.

#### masquerade

HTTP server behavior when a connection fails AnyTLS authentication.

When configured, invalid connections will be served as a normal HTTP server instead of being closed,
providing censorship resistance by making the server appear as a legitimate web service.

Conflict with `tls.acme`.

See [Masquerade](/configuration/shared/masquerade/) for details.

### Masquerade Examples

#### Simple URL Format

The simplest way to configure masquerade is to use a URL string:

```json
{
  "type": "anytls",
  "tls": {
    "enabled": true,
    "certificate_path": "/path/to/cert.pem",
    "key_path": "/path/to/key.pem"
  },
  "masquerade": "https://example.com"
}
```

This automatically creates a reverse proxy to the specified URL.

#### Reverse Proxy Mode

Forward invalid connections to another HTTP server:

```json
{
  "masquerade": {
    "type": "proxy",
    "url": "https://example.com",
    "rewrite_host": true
  }
}
```

The reverse proxy automatically adds the following headers:
- `X-Forwarded-For`: Original client IP (appends to existing chain if present)
- `X-Real-IP`: Direct client IP

HTTP redirects (301, 302, etc.) are automatically followed and passed through transparently.

#### Static File Server Mode

Serve static files from a directory:

```json
{
  "masquerade": {
    "type": "file",
    "directory": "/var/www/html"
  }
}
```

#### Custom HTTP Response Mode

Return a custom HTTP response:

```json
{
  "masquerade": {
    "type": "string",
    "status_code": 200,
    "headers": {
      "Content-Type": ["text/html; charset=utf-8"],
      "Server": ["nginx/1.20.0"]
    },
    "content": "<html><body><h1>Welcome</h1></body></html>"
  }
}
```

#### HTTP Redirect Mode

Return an HTTP redirect (302, 301, etc.) directly instead of proxying:

```json
{
  "masquerade": {
    "type": "redirect",
    "url": "https://example.com",
    "status_code": 302,
    "append_request_uri": true,
    "headers": {
      "Cache-Control": ["public, max-age=3600"],
      "X-Redirect-Reason": ["Maintenance"]
    }
  }
}
```

- `url`: Target URL to redirect to (required)
- `status_code`: HTTP status code (default: 302)
  - 301: Moved Permanently
  - 302: Found (temporary redirect)
  - 303: See Other
  - 307: Temporary Redirect (preserves method)
  - 308: Permanent Redirect (preserves method)
- `append_request_uri`: Append the request URI to the redirect URL (default: false)
  - When enabled, a request to `/path?query=value` redirects to `https://example.com/path?query=value`
  - When disabled, all requests redirect to the exact URL specified
- `headers`: Custom headers to include in the redirect response (optional)

The `Location` header is automatically set to the specified URL (with request URI appended if enabled).

**Examples:**

Static redirect (all requests go to the same URL):
```json
{
  "masquerade": {
    "type": "redirect",
    "url": "https://example.com"
  }
}
```

Dynamic redirect (preserves path and query):
```json
{
  "masquerade": {
    "type": "redirect",
    "url": "https://example.com",
    "append_request_uri": true
  }
}
```

With `append_request_uri: true`:
- Request: `GET /blog/article?id=123`
- Redirect: `https://example.com/blog/article?id=123`

Without `append_request_uri` (or set to false):
- Request: `GET /blog/article?id=123`
- Redirect: `https://example.com`

### Security Considerations

#### Censorship Resistance

The masquerade feature significantly improves censorship resistance:

- **Without masquerade**: Failed authentication attempts immediately close the connection, revealing that this is a proxy server to active probing by censors.
- **With masquerade**: Failed attempts receive a normal HTTP response, making the server indistinguishable from a legitimate HTTPS website.

This is particularly effective when combined with TLS, as the server appears to be a regular web server to anyone without valid credentials.

#### Active Probing Defense

Censorship systems often use active probing to detect proxy servers by attempting connections and analyzing the response.
With masquerade configured, these probes will see a normal website response rather than connection failures or
protocol-specific errors that might reveal the server's true purpose.

#### Best Practices

1. Use masquerade with TLS enabled for maximum effectiveness
2. Choose a realistic masquerade target that matches your server's apparent purpose
3. Consider using a popular website as the reverse proxy target to blend in with normal traffic
4. Keep your user passwords secure - masquerade only protects against unauthenticated connections

### TLS-less Mode (nginx Upstream TLS Termination)

AnyTLS can operate without TLS configuration when TLS is terminated by an upstream reverse proxy like nginx stream. In this mode:

- nginx handles TLS termination and forwards plain TCP to AnyTLS
- AnyTLS receives and processes plain HTTP/TCP connections
- Masquerade responses are served over HTTP (not HTTPS)

This is useful for:
- Centralized TLS certificate management in nginx
- TLS offloading to a dedicated proxy layer
- Simplified AnyTLS configuration

#### Example Configuration

**nginx stream configuration** (terminates TLS):

```nginx
stream {
    upstream anytls {
        server 127.0.0.1:8080;
    }

    server {
        listen 443 ssl;
        ssl_certificate /path/to/cert.pem;
        ssl_certificate_key /path/to/key.pem;
        ssl_protocols TLSv1.2 TLSv1.3;

        proxy_pass anytls;
        proxy_ssl off;  # Don't use TLS to upstream
    }
}
```

**AnyTLS configuration** (no TLS, receives plain TCP):

```json
{
  "type": "anytls",
  "tag": "anytls-in",
  "listen": "127.0.0.1",
  "listen_port": 8080,
  "users": [
    {
      "name": "user1",
      "password": "secret_password"
    }
  ],
  "masquerade": {
    "type": "redirect",
    "url": "https://example.com"
  }
}
```

**Traffic flow:**

```
Browser → HTTPS (TLS) → nginx stream (TLS termination)
                             ↓ Plain TCP/HTTP
                         AnyTLS server (no TLS)
                             ↓ If invalid auth
                         HTTP redirect response
```

In this setup:
- Clients connect to nginx with HTTPS
- nginx terminates TLS and forwards plain TCP to AnyTLS on 127.0.0.1:8080
- AnyTLS authenticates the connection
- Valid authentication: Traffic is proxied normally
- Invalid authentication: HTTP redirect is returned (served by nginx as HTTPS to client)
