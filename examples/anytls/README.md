# AnyTLS Example Configurations

This directory contains example configurations for the AnyTLS inbound with masquerade feature.

## Examples

### 1. Simple 302 Redirect (`redirect-302.json`)

Basic configuration with HTTP 302 redirect for invalid connections.

**Features:**
- TLS enabled with certificate
- Single user authentication
- 302 redirect to example.com for failed auth

**Use case:** Simple masquerade as a regular HTTPS website.

### 2. 301 Redirect with Custom Headers (`redirect-301-custom-headers.json`)

Permanent redirect with custom HTTP headers.

**Features:**
- TLS enabled with certificate
- 301 permanent redirect
- Custom headers (Cache-Control, X-Redirect-Reason)

**Use case:** Masquerade as a moved website with caching hints.

### 3. TLS-less Mode with nginx Upstream (`tls-less-nginx-upstream.json` + `nginx-stream.conf`)

AnyTLS without TLS, relying on nginx for TLS termination.

**Features:**
- No TLS configuration in AnyTLS
- Listens on localhost only
- nginx handles TLS termination upstream

**Use case:** Centralized TLS management, certificate rotation handled by nginx.

**Setup:**
1. Configure nginx with `nginx-stream.conf` (adjust paths and ports as needed)
2. Start AnyTLS with `tls-less-nginx-upstream.json`
3. Clients connect to nginx on port 443 (HTTPS)
4. nginx forwards plain TCP to AnyTLS on 127.0.0.1:8080

## Redirect Features

### Request URI Preservation

The `append_request_uri` option allows you to preserve the original request path and query parameters:

- **Enabled (`true`)**: Redirects preserve the path - Request to `/blog/post?id=1` → `https://example.com/blog/post?id=1`
- **Disabled (`false`, default)**: All requests redirect to the exact URL - Request to `/blog/post?id=1` → `https://example.com`

This makes the redirect behavior more realistic and similar to nginx's redirect functionality.

### Redirect Status Codes

The redirect mode supports all HTTP redirect status codes:

- **301**: Moved Permanently - Use for permanent URL changes
- **302**: Found (default) - Temporary redirect, most common
- **303**: See Other - Redirect after POST to GET
- **307**: Temporary Redirect - Preserves HTTP method
- **308**: Permanent Redirect - Preserves HTTP method

## Security Notes

1. **Censorship Resistance**: Masquerade makes the server appear as a legitimate website to unauthorized users, improving resistance to active probing.

2. **Password Security**: The masquerade feature only protects unauthenticated connections. Keep user passwords secure.

3. **TLS Best Practices**:
   - Use strong certificates (Let's Encrypt, commercial CA, etc.)
   - Keep private keys secure
   - Use modern TLS protocols (1.2+)

4. **nginx TLS-less Mode**:
   - Bind AnyTLS to 127.0.0.1 only (not publicly accessible)
   - Ensure nginx is properly configured for TLS
   - Use firewall rules to block direct access to AnyTLS port

## Testing

### Test basic redirect response:
```bash
curl -i http://your-server:8443/
# or for nginx upstream:
curl -i -k https://your-server:443/
```

Expected response:
```
HTTP/1.1 302 Found
Location: https://example.com
```

### Test redirect with request URI preservation:
```bash
curl -i http://your-server:8443/blog/article?id=123
```

With `append_request_uri: true`, expected response:
```
HTTP/1.1 302 Found
Location: https://example.com/blog/article?id=123
```

With `append_request_uri: false` (or omitted), expected response:
```
HTTP/1.1 302 Found
Location: https://example.com
```

### Test with custom headers:
```bash
curl -i http://your-server:8443/
```

Expected response:
```
HTTP/1.1 301 Moved Permanently
Location: https://example.com/new-location
Cache-Control: public, max-age=31536000
X-Redirect-Reason: Site moved permanently
```

### Test redirect following (browser-like behavior):
```bash
curl -L http://your-server:8443/some/path
```

This will follow the redirect and show the content from the target URL.

### Test valid authentication:
```bash
# Use your AnyTLS client with correct credentials
# Connection should succeed and proxy traffic normally
```
