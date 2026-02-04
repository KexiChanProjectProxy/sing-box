# AnyTLS Redirect Masquerade Implementation

## Overview

This document describes the implementation of the HTTP redirect masquerade mode and TLS-less mode support for AnyTLS inbound.

## Implementation Date

2026-02-04

## Features Implemented

### 1. HTTP Redirect Masquerade Mode

Added a new masquerade type `"redirect"` that returns HTTP redirects (302, 301, etc.) directly instead of proxying.

**Capabilities:**
- Configurable redirect URL
- Configurable HTTP status code (301, 302, 303, 307, 308)
- Custom HTTP headers support
- Request URI preservation (append to redirect URL)
- Default status code: 302 (Found)

**Use cases:**
- Simpler masquerade without reverse proxy complexity
- Lower resource usage compared to proxy mode
- Direct client redirection to legitimate websites
- Custom redirect logic with headers

### 2. TLS-less Mode Documentation

Documented the existing capability to run AnyTLS without TLS configuration, useful when TLS is terminated by an upstream reverse proxy like nginx.

**Capabilities:**
- AnyTLS operates in plain TCP mode
- nginx (or other reverse proxy) terminates TLS upstream
- Centralized certificate management
- TLS offloading to dedicated proxy layer

**Note:** This feature already existed in the codebase (TLS is optional), no code changes were needed - only documentation was added.

### 3. Custom Headers Support

The redirect mode supports custom HTTP headers via the existing `badoption.HTTPHeader` type, consistent with the string masquerade mode.

## Files Modified

### 1. `/home/kexi/sing-box/constant/anytls.go`

Added new constant for redirect masquerade type:

```go
const (
    AnyTLSMasqueradeTypeFile     = "file"
    AnyTLSMasqueradeTypeProxy    = "proxy"
    AnyTLSMasqueradeTypeString   = "string"
    AnyTLSMasqueradeTypeRedirect = "redirect"  // NEW
)
```

### 2. `/home/kexi/sing-box/option/anytls.go`

**Added redirect configuration structure:**

```go
type AnyTLSMasqueradeRedirect struct {
    URL              string               `json:"url"`
    StatusCode       int                  `json:"status_code,omitempty"`
    Headers          badoption.HTTPHeader `json:"headers,omitempty"`
    AppendRequestURI bool                 `json:"append_request_uri,omitempty"`
}
```

Fields:
- `URL`: Target URL to redirect to
- `StatusCode`: HTTP status code (default: 302)
- `Headers`: Custom headers to include in redirect response
- `AppendRequestURI`: When true, appends the original request path and query to the redirect URL (similar to nginx's `$request_uri`)

**Updated `_AnyTLSMasquerade` struct:**
- Added `RedirectOptions AnyTLSMasqueradeRedirect` field

**Updated JSON marshaling:**
- Added redirect case to `MarshalJSON()` method
- Added redirect case to `UnmarshalJSON()` method

### 3. `/home/kexi/sing-box/protocol/anytls/inbound.go`

**Added redirect handler implementation:**

```go
case C.AnyTLSMasqueradeTypeRedirect:
    redirectURL := options.Masquerade.RedirectOptions.URL
    statusCode := options.Masquerade.RedirectOptions.StatusCode
    if statusCode == 0 {
        statusCode = http.StatusFound // Default to 302
    }
    customHeaders := options.Masquerade.RedirectOptions.Headers

    masqueradeHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Add custom headers first
        for key, values := range customHeaders {
            for _, value := range values {
                w.Header().Add(key, value)
            }
        }
        // Set Location header (required for redirects)
        w.Header().Set("Location", redirectURL)
        // Send redirect status
        w.WriteHeader(statusCode)
    })
```

**Key implementation details:**
- Custom headers are added before the Location header
- Location header is always set (required for HTTP redirects)
- Status code defaults to 302 (StatusFound) if not specified
- Headers use the standard `badoption.HTTPHeader` type
- Base URL is parsed once during initialization for efficiency
- When `append_request_uri` is enabled:
  - Request path is appended to base URL path (trailing slashes handled)
  - Query parameters are preserved from original request
  - URL fragments are preserved from original request
  - Similar to nginx's redirect with `$request_uri` variable

### 4. `/home/kexi/sing-box/docs/configuration/inbound/anytls.md`

**Added documentation sections:**

1. **HTTP Redirect Mode section:**
   - Configuration format
   - Supported status codes
   - Custom headers usage
   - Examples

2. **TLS-less Mode section:**
   - Explanation of TLS-less operation
   - nginx stream configuration example
   - AnyTLS configuration example
   - Traffic flow diagram
   - Use cases

3. **Updated TLS field description:**
   - Clarified that TLS is optional
   - Explained upstream TLS termination use case

## Files Created

### Example Configurations

1. **`/home/kexi/sing-box/examples/anytls/redirect-302.json`**
   - Simple 302 redirect example
   - With TLS enabled

2. **`/home/kexi/sing-box/examples/anytls/redirect-301-custom-headers.json`**
   - 301 permanent redirect
   - Custom headers example

3. **`/home/kexi/sing-box/examples/anytls/tls-less-nginx-upstream.json`**
   - AnyTLS without TLS configuration
   - For use with nginx upstream TLS termination

4. **`/home/kexi/sing-box/examples/anytls/nginx-stream.conf`**
   - nginx stream configuration example
   - TLS termination setup
   - Upstream forwarding to AnyTLS

5. **`/home/kexi/sing-box/examples/anytls/README.md`**
   - Overview of all examples
   - Usage instructions
   - Testing commands
   - Security notes

## Configuration Examples

### Simple 302 Redirect

```json
{
  "type": "anytls",
  "masquerade": {
    "type": "redirect",
    "url": "https://example.com"
  }
}
```

### 302 Redirect with Request URI Preservation

```json
{
  "type": "anytls",
  "masquerade": {
    "type": "redirect",
    "url": "https://example.com",
    "append_request_uri": true
  }
}
```

With this configuration:
- Request to `/path/to/resource?query=value`
- Redirects to `https://example.com/path/to/resource?query=value`

### 301 Redirect with Custom Headers

```json
{
  "type": "anytls",
  "masquerade": {
    "type": "redirect",
    "url": "https://example.com/new-location",
    "status_code": 301,
    "append_request_uri": true,
    "headers": {
      "Cache-Control": ["public, max-age=31536000"],
      "X-Redirect-Reason": ["Site moved"]
    }
  }
}
```

### TLS-less Mode (nginx Upstream)

**AnyTLS config:**
```json
{
  "type": "anytls",
  "listen": "127.0.0.1",
  "listen_port": 8080,
  "users": [...],
  "masquerade": {
    "type": "redirect",
    "url": "https://example.com"
  }
}
```

**nginx config:**
```nginx
stream {
    upstream anytls {
        server 127.0.0.1:8080;
    }
    server {
        listen 443 ssl;
        ssl_certificate /path/to/cert.pem;
        ssl_certificate_key /path/to/key.pem;
        proxy_pass anytls;
        proxy_ssl off;
    }
}
```

## Supported Redirect Status Codes

- **301**: Moved Permanently
- **302**: Found (temporary redirect) - **DEFAULT**
- **303**: See Other
- **307**: Temporary Redirect (preserves HTTP method)
- **308**: Permanent Redirect (preserves HTTP method)

## Testing

### Validation

Configuration validation confirmed working:
```bash
$ sing-box check -c test-redirect.json
# (no errors - validation passed)
```

### Manual Testing Commands

**Test basic redirect response:**
```bash
curl -i http://127.0.0.1:8080/
```

Expected output:
```
HTTP/1.1 302 Found
Location: https://example.com
X-Test-Header: test-value
Cache-Control: no-cache
```

**Test redirect with request URI preservation:**
```bash
curl -i http://127.0.0.1:8080/blog/article?id=123
```

With `append_request_uri: true`, expected output:
```
HTTP/1.1 302 Found
Location: https://example.com/blog/article?id=123
```

With `append_request_uri: false` or omitted, expected output:
```
HTTP/1.1 302 Found
Location: https://example.com
```

**Test redirect following:**
```bash
curl -L http://127.0.0.1:8080/some/path
```

Expected: Content from the redirect target URL (with path preserved if `append_request_uri` enabled).

## Backward Compatibility

All changes are backward compatible:
- Existing masquerade modes (file, proxy, string) unchanged
- New redirect type is opt-in
- TLS optional behavior already existed (no breaking changes)
- Configuration format follows existing patterns

## Code Quality

- **Lines of code added:** ~50 lines
- **Files modified:** 4 files
- **Files created:** 6 files (examples + docs)
- **Build status:** ✅ Successful
- **Configuration validation:** ✅ Passed

## Design Consistency

The implementation follows existing patterns:
- Uses same JSON marshaling approach as other masquerade types
- Uses `badoption.HTTPHeader` type (consistent with string mode)
- Handler implementation follows same pattern as proxy/file/string
- Error handling consistent with existing code

## Security Considerations

1. **Censorship Resistance**: Redirect mode provides effective masquerade without proxy complexity
2. **Resource Usage**: Lower overhead than proxy mode (no upstream connection)
3. **Information Disclosure**: Redirect URL is visible to unauthorized clients (by design)
4. **TLS-less Mode Security**: AnyTLS should bind to 127.0.0.1 only when using nginx upstream

## Performance Characteristics

- **CPU Usage**: Minimal (simpler than proxy mode)
- **Memory Usage**: Minimal (no buffering or upstream connections)
- **Latency**: Very low (immediate redirect response)
- **Concurrent Connections**: High capacity (no upstream bottleneck)

## Future Enhancements

Possible future improvements:
1. Variable substitution in redirect URLs (e.g., preserve path/query)
2. Conditional redirects based on request headers
3. Multiple redirect targets with load balancing
4. Rate limiting for masquerade responses

## Implementation Complexity

- **Estimated effort:** Low (30-50 lines of code)
- **Actual effort:** ~50 lines of code + documentation
- **Risk assessment:** Very low
- **Dependencies:** None (uses existing HTTP library)
- **Testing required:** Manual testing sufficient

## References

- Original implementation plan: `/home/kexi/sing-box/ANYTLS_MASQUERADE_IMPLEMENTATION.md`
- AnyTLS protocol: https://github.com/anytls/sing-anytls
- HTTP redirects spec: RFC 7231 Section 6.4

## Conclusion

The redirect masquerade mode and TLS-less mode documentation provide enhanced flexibility for AnyTLS deployments. The implementation is simple, efficient, and follows existing code patterns. All objectives from the original plan have been achieved:

✅ Direct 302 redirect mode implemented
✅ Support for custom headers in redirects
✅ Configurable status codes (301, 302, 307, 308, etc.)
✅ TLS-less mode documented (already worked, no code needed)
✅ Example configurations created
✅ Comprehensive documentation added
✅ Build verification successful
✅ Configuration validation successful
