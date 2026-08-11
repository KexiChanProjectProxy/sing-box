`http-dynamic` outbound is an HTTP CONNECT proxy client with a fixed Basic authorization username and a password derived for each connection from the routed inbound metadata.

### Structure

```json
{
  "type": "http-dynamic",
  "tag": "http-dynamic-out",

  "server": "127.0.0.1",
  "server_port": 1080,
  "username": "fixed-user",
  "path": "",
  "headers": {},
  "tls": {},

  ... // Dial Fields
}
```

### Fields

#### server

==Required==

The server address.

#### server_port

==Required==

The server port.

#### username

==Required==

The fixed HTTP Basic authorization username.

#### Password derivation

For every connection, the Basic authorization password is the first 16 lowercase hexadecimal characters of:

```text
sha256(inbound_username + client_source_ip)
```

`client_source_ip` is the inbound client's IP address without its port. The outbound rejects a connection when the routed inbound has no authenticated username or valid source IP.

#### path

Path of HTTP request.

#### headers

Extra headers of HTTP request.

#### tls

TLS configuration, see [TLS](/configuration/shared/tls/#outbound).

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
