# Release Notes - v1.12.14.24

**Release Date:** 2026-02-04

## New Features

### AnyTLS Redirect Masquerade Mode

Added a new `redirect` masquerade type for AnyTLS inbound that provides HTTP redirect responses for invalid connections.

#### Features:
- **Direct HTTP Redirects**: Return 302/301/307/308 redirects instead of proxying
- **Request URI Preservation**: Append original request path and query to redirect URL (like nginx `$request_uri`)
- **Custom Headers**: Support for custom headers in redirect responses
- **Configurable Status Codes**: Support for all HTTP redirect status codes (301, 302, 303, 307, 308)
- **TLS-less Mode**: Full support for operation without TLS (when terminated by upstream nginx)

#### Configuration Example:

```json
{
  "type": "anytls",
  "masquerade": {
    "type": "redirect",
    "url": "https://example.com",
    "status_code": 302,
    "append_request_uri": true,
    "headers": {
      "Cache-Control": ["no-cache"]
    }
  }
}
```

#### Request URI Preservation

With `append_request_uri: true`:
- Request: `GET /blog/article?id=123`
- Redirect: `https://example.com/blog/article?id=123`

Without (default):
- Request: `GET /blog/article?id=123`
- Redirect: `https://example.com`

## Files Changed

### Core Implementation
- `constant/anytls.go` - Added redirect masquerade type constant
- `option/anytls.go` - Added redirect configuration structure
- `protocol/anytls/inbound.go` - Implemented redirect handler with URI preservation

### Documentation
- `docs/configuration/inbound/anytls.md` - Comprehensive redirect mode documentation
  - Added HTTP Redirect Mode section
  - Added TLS-less Mode section with nginx examples
  - Updated masquerade examples

### Examples
- `examples/anytls/redirect-302.json` - Simple 302 redirect
- `examples/anytls/redirect-301-custom-headers.json` - 301 redirect with custom headers
- `examples/anytls/tls-less-nginx-upstream.json` - TLS-less mode configuration
- `examples/anytls/nginx-stream.conf` - nginx TLS termination example
- `examples/anytls/README.md` - Examples documentation

### Implementation Documentation
- `ANYTLS_REDIRECT_IMPLEMENTATION.md` - Detailed implementation notes
- `ANYTLS_APPEND_REQUEST_URI_FEATURE.md` - Request URI preservation feature documentation

## Benefits

### Improved Censorship Resistance
- Simpler than reverse proxy mode (no upstream connection needed)
- More realistic masquerade behavior
- Lower resource usage

### nginx-like Functionality
- Request URI preservation similar to nginx's `$request_uri` variable
- Compatible with existing nginx deployment patterns
- Easy migration from nginx redirect configurations

### Flexible Deployment Options
- Works with or without TLS
- Supports upstream TLS termination (nginx stream)
- Centralized certificate management possible

## Comparison with Other Masquerade Modes

| Mode | Description | Resource Usage | Use Case |
|------|-------------|----------------|----------|
| **proxy** | Reverse proxy to HTTP server | High | Full website masquerade |
| **file** | Static file server | Medium | Serve static content |
| **string** | Custom HTTP response | Low | Simple custom response |
| **redirect** (NEW) | HTTP redirect | Very Low | Site migration/forwarding |

## Backward Compatibility

✅ Fully backward compatible
- Existing masquerade modes unchanged
- New redirect type is opt-in
- Default behavior (no append_request_uri) maintains simplicity

## Testing

All changes have been verified:
- ✅ Build successful
- ✅ Configuration validation passed
- ✅ Documentation complete
- ✅ Example configurations provided

## Usage

### Basic Redirect
```json
{
  "masquerade": {
    "type": "redirect",
    "url": "https://example.com"
  }
}
```

### Dynamic Redirect (preserves path)
```json
{
  "masquerade": {
    "type": "redirect",
    "url": "https://example.com",
    "append_request_uri": true
  }
}
```

### TLS-less Mode (nginx upstream)
```json
{
  "type": "anytls",
  "listen": "127.0.0.1",
  "listen_port": 8080,
  "masquerade": {
    "type": "redirect",
    "url": "https://example.com",
    "append_request_uri": true
  }
}
```

## Migration Guide

### From nginx redirect:
**nginx:**
```nginx
location / {
    return 302 https://example.com$request_uri;
}
```

**AnyTLS:**
```json
{
  "masquerade": {
    "type": "redirect",
    "url": "https://example.com",
    "append_request_uri": true,
    "status_code": 302
  }
}
```

## Contributors

Implementation by Claude Code (AI Assistant)

## Links

- Documentation: `docs/configuration/inbound/anytls.md`
- Examples: `examples/anytls/`
- Implementation Details: `ANYTLS_REDIRECT_IMPLEMENTATION.md`
- Feature Documentation: `ANYTLS_APPEND_REQUEST_URI_FEATURE.md`
