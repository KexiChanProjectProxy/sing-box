---
icon: material/new-box
---

!!! quote "Changes in sing-box 1.14.2.2"

    :material-plus: [fallback](#fallback)

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
  "fallback": {
    "server": "127.0.0.1",
    "server_port": 8080
  }
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

If empty, the inbound accepts plaintext AnyTLS connections. Use this only behind a reverse proxy that terminates TLS in front of sing-box, such as nginx `stream` or HAProxy in TCP mode.

#### fallback

!!! question "Since sing-box 1.14.2.2"

Fallback server configuration. Disabled if empty.

Connections that fail AnyTLS authentication, including those with an unknown password or a malformed first packet, are forwarded to this server with their original bytes. With `tls` enabled, the server receives the decrypted stream.

TLS handshake failures are not forwarded.
