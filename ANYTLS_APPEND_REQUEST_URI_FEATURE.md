# AnyTLS Redirect: Request URI Preservation Feature

## Overview

The `append_request_uri` option allows the redirect masquerade mode to preserve the original request path and query parameters when redirecting, similar to nginx's `$request_uri` variable functionality.

## Implementation Date

2026-02-04 (Added to initial redirect implementation)

## Feature Description

When `append_request_uri` is enabled, the redirect URL dynamically includes the original request's:
- **Path**: The URL path from the original request
- **Query parameters**: Any query string from the original request
- **Fragment**: URL fragment (hash) from the original request

This makes the masquerade behavior more realistic by appearing to redirect users to the same resource on a different domain, rather than always redirecting to a fixed URL.

## Configuration

### Syntax

```json
{
  "masquerade": {
    "type": "redirect",
    "url": "https://example.com",
    "append_request_uri": true
  }
}
```

### Parameters

- **`append_request_uri`** (boolean, optional)
  - Default: `false`
  - When `true`: Appends request path and query to redirect URL
  - When `false`: Uses exact redirect URL for all requests

## Behavior Examples

### Example 1: Static Redirect (default behavior)

**Configuration:**
```json
{
  "masquerade": {
    "type": "redirect",
    "url": "https://example.com"
  }
}
```

**Behavior:**
| Request | Redirect Location |
|---------|------------------|
| `GET /` | `https://example.com` |
| `GET /blog` | `https://example.com` |
| `GET /blog/article?id=123` | `https://example.com` |
| `GET /api/users?page=2` | `https://example.com` |

All requests redirect to the exact same URL.

### Example 2: Dynamic Redirect (with append_request_uri)

**Configuration:**
```json
{
  "masquerade": {
    "type": "redirect",
    "url": "https://example.com",
    "append_request_uri": true
  }
}
```

**Behavior:**
| Request | Redirect Location |
|---------|------------------|
| `GET /` | `https://example.com/` |
| `GET /blog` | `https://example.com/blog` |
| `GET /blog/article?id=123` | `https://example.com/blog/article?id=123` |
| `GET /api/users?page=2` | `https://example.com/api/users?page=2` |

Each request preserves its path and query in the redirect.

### Example 3: Redirect with Base Path

**Configuration:**
```json
{
  "masquerade": {
    "type": "redirect",
    "url": "https://example.com/newsite",
    "append_request_uri": true
  }
}
```

**Behavior:**
| Request | Redirect Location |
|---------|------------------|
| `GET /` | `https://example.com/newsite/` |
| `GET /blog` | `https://example.com/newsite/blog` |
| `GET /blog/article?id=123` | `https://example.com/newsite/blog/article?id=123` |

The base URL path is preserved and the request path is appended.

## Use Cases

### 1. Mimicking Site Migration

Make the server appear as if the site has moved to a new domain:

```json
{
  "masquerade": {
    "type": "redirect",
    "url": "https://newdomain.com",
    "append_request_uri": true,
    "status_code": 301
  }
}
```

Unauthorized users see realistic 301 redirects to the "new location" while maintaining the URL structure.

### 2. Domain Forwarding Simulation

Simulate a domain forwarding service:

```json
{
  "masquerade": {
    "type": "redirect",
    "url": "https://maindomain.com",
    "append_request_uri": true,
    "status_code": 302
  }
}
```

Makes the server appear to be forwarding all requests to another domain.

### 3. CDN/Reverse Proxy Simulation

Simulate a CDN or reverse proxy redirect:

```json
{
  "masquerade": {
    "type": "redirect",
    "url": "https://cdn.example.com",
    "append_request_uri": true,
    "headers": {
      "X-CDN-Provider": ["CloudFlare"],
      "Cache-Control": ["public, max-age=3600"]
    }
  }
}
```

Appears as a CDN redirecting to cached content.

## Implementation Details

### URL Construction Algorithm

When `append_request_uri` is enabled:

1. **Parse base URL** at server initialization (performance optimization)
2. **For each redirect request:**
   - Start with parsed base URL
   - Trim trailing slash from base path
   - Append request path to base path
   - Add request query parameters
   - Add request fragment (if present)
   - Serialize to final redirect URL

### Code Flow

```go
// Parse base URL once (initialization)
redirectBaseURL, err := url.Parse(baseURL)

// For each request (handler)
if appendRequestURI {
    targetURL := *redirectBaseURL
    targetURL.Path = strings.TrimRight(targetURL.Path, "/") + r.URL.Path
    targetURL.RawQuery = r.URL.RawQuery
    targetURL.Fragment = r.URL.Fragment
    redirectURL = targetURL.String()
}
```

### Performance Characteristics

- **Initialization**: One-time URL parsing overhead
- **Per-request**: String concatenation and serialization
- **Memory**: Minimal (URL struct copy per request)
- **CPU**: Very low overhead (~microseconds per redirect)

## Comparison with nginx

This feature is inspired by nginx's redirect functionality:

### nginx equivalent:

```nginx
location / {
    return 302 https://example.com$request_uri;
}
```

### AnyTLS equivalent:

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

Both achieve the same result: redirecting while preserving the request path and query.

## Security Considerations

### Information Disclosure

- **Static redirect** (default): Minimal information - all requests go to same URL
- **Dynamic redirect** (append enabled): Reveals URL structure through redirects

For maximum censorship resistance, consider which approach fits your threat model:
- High security: Use static redirect (less predictable)
- High realism: Use dynamic redirect (more believable as legitimate site)

### Open Redirect Prevention

The implementation does NOT allow user-controlled redirect URLs - the base URL is configured server-side and cannot be influenced by request parameters. This prevents open redirect vulnerabilities.

## Testing

### Test Setup

Start AnyTLS with append_request_uri enabled:

```bash
sing-box run -c config.json
```

### Test Commands

**Test root path:**
```bash
curl -I http://127.0.0.1:8080/
```

**Test nested path:**
```bash
curl -I http://127.0.0.1:8080/blog/article
```

**Test with query parameters:**
```bash
curl -I http://127.0.0.1:8080/api/users?page=2&limit=10
```

**Test with special characters:**
```bash
curl -I http://127.0.0.1:8080/search?q=hello%20world
```

### Expected Results

With `append_request_uri: true`:
```
Location: https://example.com/
Location: https://example.com/blog/article
Location: https://example.com/api/users?page=2&limit=10
Location: https://example.com/search?q=hello%20world
```

With `append_request_uri: false`:
```
Location: https://example.com
Location: https://example.com
Location: https://example.com
Location: https://example.com
```

## Backward Compatibility

- **Default behavior**: `append_request_uri: false` (static redirect)
- **Existing configs**: Continue to work unchanged
- **New feature**: Opt-in via explicit configuration

This ensures no breaking changes to existing deployments.

## Future Enhancements

Possible improvements:
1. **URL rewriting patterns**: Advanced path transformations
2. **Conditional append**: Based on path patterns or headers
3. **Variable substitution**: Support for more nginx-style variables ($host, $scheme, etc.)
4. **Query parameter filtering**: Include/exclude specific parameters

## Summary

The `append_request_uri` feature provides flexible redirect behavior:

✅ **Simple to configure**: Single boolean option
✅ **Performance efficient**: Minimal overhead
✅ **nginx-compatible**: Similar to nginx redirect functionality
✅ **Backward compatible**: Defaults to static redirect
✅ **Realistic masquerade**: Makes redirects appear more legitimate

This enhancement significantly improves the realism of the redirect masquerade mode, making AnyTLS servers harder to distinguish from legitimate web services.
