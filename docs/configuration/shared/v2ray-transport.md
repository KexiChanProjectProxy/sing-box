V2Ray Transport is a set of private protocols invented by v2ray, and has contaminated the names of other protocols, such
as `trojan-grpc` in clash.

### Structure

```json
{
  "type": ""
}
```

Available transports:

* HTTP
* WebSocket
* QUIC
* gRPC
* HTTPUpgrade
* xhttp

!!! warning "Difference from v2ray-core"

    * No TCP transport, plain HTTP is merged into the HTTP transport.
    * No mKCP transport.
    * No DomainSocket transport.

!!! note ""

    You can ignore the JSON Array [] tag when the content is only one item

### HTTP

```json
{
  "type": "http",
  "host": [],
  "path": "",
  "method": "",
  "headers": {},
  "idle_timeout": "15s",
  "ping_timeout": "15s"
}
```

!!! warning "Difference from v2ray-core"

    TLS is not enforced. If TLS is not configured, plain HTTP 1.1 is used.

#### host

List of host domain.

The client will choose randomly and the server will verify if not empty.

#### path

!!! warning

    V2Ray's documentation says that the path between the server and the client must be consistent, 
    but the actual code allows the client to add any suffix to the path.
    sing-box uses the same behavior as V2Ray, but note that the behavior does not exist in `WebSocket` and `HTTPUpgrade` transport.

Path of HTTP request.

The server will verify.

#### method

Method of HTTP request.

The server will verify if not empty.

#### headers

Extra headers of HTTP request.

The server will write in response if not empty.

#### idle_timeout

In HTTP2 server:

Specifies the time until idle clients should be closed with a GOAWAY frame. PING frames are not considered as activity.

In HTTP2 client:

Specifies the period of time after which a health check will be performed using a ping frame if no frames have been
received on the connection.Please note that a ping response is considered a received frame, so if there is no other
traffic on the connection, the health check will be executed every interval. If the value is zero, no health check will
be performed.

Zero is used by default.

#### ping_timeout

In HTTP2 client:

Specifies the timeout duration after sending a PING frame, within which a response must be received.
If a response to the PING frame is not received within the specified timeout duration, the connection will be closed.
The default timeout duration is 15 seconds.

### WebSocket

```json
{
  "type": "ws",
  "path": "",
  "headers": {},
  "max_early_data": 0,
  "early_data_header_name": ""
}
```

#### path

Path of HTTP request.

The server will verify.

#### headers

Extra headers of HTTP request.

The server will write in response if not empty.

#### max_early_data

Allowed payload size is in the request. Enabled if not zero.

#### early_data_header_name

Early data is sent in path instead of header by default.

To be compatible with Xray-core, set this to `Sec-WebSocket-Protocol`.

It needs to be consistent with the server.

### QUIC

```json
{
  "type": "quic"
}
```

!!! warning "Difference from v2ray-core"

    No additional encryption support:
    It's basically duplicate encryption. And Xray-core is not compatible with v2ray-core in here.

### gRPC

!!! note ""

    standard gRPC has good compatibility but poor performance and is not included by default, see [Installation](/installation/build-from-source/#build-tags).

```json
{
  "type": "grpc",
  "service_name": "TunService",
  "idle_timeout": "15s",
  "ping_timeout": "15s",
  "permit_without_stream": false
}
```

#### service_name

Service name of gRPC.

#### idle_timeout

In standard gRPC server/client:

If the transport doesn't see any activity after a duration of this time,
it pings the client to check if the connection is still active.

In default gRPC server/client:

It has the same behavior as the corresponding setting in HTTP transport.

#### ping_timeout

In standard gRPC server/client:

The timeout that after performing a keepalive check, the client will wait for activity.
If no activity is detected, the connection will be closed.

In default gRPC server/client:

It has the same behavior as the corresponding setting in HTTP transport.

#### permit_without_stream

In standard gRPC client:

If enabled, the client transport sends keepalive pings even with no active connections.
If disabled, when there are no active connections, `idle_timeout` and `ping_timeout` will be ignored and no keepalive
pings will be sent.

Disabled by default.

### HTTPUpgrade

```json
{
  "type": "httpupgrade",
  "host": "",
  "path": "",
  "headers": {}
}
```

#### host

Host domain.

The server will verify if not empty.

#### path

Path of HTTP request.

The server will verify.

#### headers

Extra headers of HTTP request.

The server will write in response if not empty.
### xhttp

```json
{
  "type": "xhttp",
  "host": "",
  "path": "/",
  "mode": "auto",
  "headers": {},
  "x_padding_bytes": {
    "from": 100,
    "to": 1000
  },
  "sc_max_each_post_bytes": {
    "from": 1000000,
    "to": 1000000
  },
  "sc_min_posts_interval_ms": {
    "from": 30,
    "to": 30
  },
  "sc_max_buffered_posts": 30,
  "no_grpc_header": false,
  "xmux": {
    "max_concurrency": {
      "from": 0,
      "to": 0
    },
    "max_connections": {
      "from": 0,
      "to": 0
    },
    "c_max_reuse_times": {
      "from": 0,
      "to": 0
    },
    "h_max_request_times": {
      "from": 0,
      "to": 0
    },
    "h_max_reusable_secs": {
      "from": 0,
      "to": 0
    },
    "h_keep_alive_period": 0
  }
}
```

!!! note ""

    xhttp transport is also known as splithttp in Xray-core.
    It provides HTTP-based tunneling with advanced traffic obfuscation and connection optimization.

!!! warning ""

    This transport is compatible with Xray-core's xhttp/splithttp transport.

#### host

Host domain for HTTP requests.

The server will verify if not empty.

#### path

Path of HTTP request.

The path is automatically suffixed with a session ID on the client side.

Defaults to `/`.

#### mode

Operation mode for xhttp transport.

Currently supported:

- `auto`: Automatically select mode (default: `packet-up`)
- `packet-up`: Send data as sequenced packets via multiple POST requests

Default: `auto`

!!! note ""

    Additional modes (`stream-up`, `stream-down`, `stream-one`) will be supported in future versions.

#### headers

Extra headers of HTTP request.

The server will write in response if not empty.

#### x_padding_bytes

Random padding range for traffic obfuscation.

A random `x_padding` query parameter with length in this range will be added to each request.

Default: `{"from": 100, "to": 1000}`

#### sc_max_each_post_bytes

Maximum payload size for each POST request in bytes.

Default: `{"from": 1000000, "to": 1000000}` (1MB)

#### sc_min_posts_interval_ms

Minimum interval between POST requests in milliseconds.

This helps control the request rate and avoid overwhelming the server.

Default: `{"from": 30, "to": 30}`

#### sc_max_buffered_posts

Maximum number of packets to buffer in the upload queue.

If the queue exceeds this size, the connection will be torn down to prevent memory exhaustion.

Default: `30`

#### no_grpc_header

If enabled, the `Content-Type: text/event-stream` header will not be sent in GET responses.

Default: `false`

#### xmux

Connection multiplexing (Xmux) configuration for optimizing connection reuse.

##### max_concurrency

Maximum concurrent connections allowed per pooled connection.

Range value where `0` means unlimited.

Default: `{"from": 0, "to": 0}` (unlimited)

##### max_connections

Maximum total connections in the connection pool.

Range value where `0` means unlimited.

Default: `{"from": 0, "to": 0}` (unlimited)

##### c_max_reuse_times

Maximum number of times a connection can be reused.

Range value where `0` means unlimited.

Default: `{"from": 0, "to": 0}` (unlimited)

##### h_max_request_times

Maximum number of HTTP requests allowed per connection.

Range value where `0` means unlimited.

Default: `{"from": 0, "to": 0}` (unlimited)

##### h_max_reusable_secs

Maximum lifetime of a connection in seconds (TTL).

Range value where `0` means unlimited.

Default: `{"from": 0, "to": 0}` (unlimited)

##### h_keep_alive_period

Keep-alive timeout in seconds for idle connections.

`0` means disabled.

Default: `0`

!!! tip "Range Configuration"

    Many xhttp parameters use a range configuration with `from` and `to` fields.
    The actual value used will be randomly selected within this range, providing additional obfuscation.
    If `from` equals `to`, that exact value is used.
