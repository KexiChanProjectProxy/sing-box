!!! quote "Changes in sing-box 1.11.0"

    :material-plus: [server_ports](#server_ports)
    :material-plus: [hop_interval](#hop_interval)
    :material-plus: [realm](#realm)

### Structure

=== "Direct mode"

    ```json
    {
      "type": "hysteria2",
      "tag": "hy2-out",

      "server": "127.0.0.1",
      "server_port": 1080,
      "server_ports": [
        "2080:3000"
      ],
      "hop_interval": "",
      "up_mbps": 100,
      "down_mbps": 100,
      "obfs": {
        "type": "salamander",
        "password": "cry_me_a_r1ver"
      },
      "password": "goofy_ahh_password",
      "network": "tcp",
      "tls": {},
      "brutal_debug": false,

      ... // Dial Fields
    }
    ```

=== "Realm mode"

    ```json
    {
      "type": "hysteria2",
      "tag": "hy2-out",

      "realm": {
        "server_url": "hy2://relay.example.com:8443",
        "token": "your_relay_token",
        "realm_id": "my-realm",
        "stun_servers": ["stun.example.com:3478"],
        "prefer_ip_version": "prefer_ipv4",
        "fallback_timeout": "30s"
      },
      "up_mbps": 100,
      "down_mbps": 100,
      "obfs": {
        "type": "salamander",
        "password": "cry_me_a_r1ver"
      },
      "network": "tcp",
      "tls": {},
      "brutal_debug": false,

      ... // Dial Fields
    }
    ```

!!! note ""

    You can ignore the JSON Array [] tag when the content is only one item

!!! warning "Difference from official Hysteria2"

    The official Hysteria2 supports an authentication method called **userpass**,
    which essentially uses a combination of `<username>:<password>` as the actual password,
    while sing-box does not provide this alias.
    If you are planning to use sing-box with the official program,
    please note that you will need to fill the combination as the password.

### Fields

#### server

==Required==

The server address.

Ignored when `realm` is set.

#### server_port

==Required==

The server port.

Ignored if `server_ports` is set.

Ignored when `realm` is set.

#### server_ports

!!! question "Since sing-box 1.11.0"

Server port range list.

Conflicts with `server_port`.

Ignored when `realm` is set.

#### hop_interval

!!! question "Since sing-box 1.11.0"

Port hopping interval.

`30s` is used by default.

Ignored when `realm` is set.

#### up_mbps, down_mbps

Max bandwidth, in Mbps.

If empty, the BBR congestion control algorithm will be used instead of Hysteria CC.

#### obfs.type

QUIC traffic obfuscator type, only available with `salamander`.

Disabled if empty.

#### obfs.password

QUIC traffic obfuscator password.

#### password

Authentication password.

Ignored when `realm` is set.

#### network

Enabled network

One of `tcp` `udp`.

Both is enabled by default.

#### tls

==Required==

TLS configuration, see [TLS](/configuration/shared/tls/#outbound).

#### brutal_debug

Enable debug information logging for Hysteria Brutal CC.

### realm

Hysteria2 realm client configuration for relay connections.

When `realm` is set, `server`, `server_port`, `server_ports`, `hop_interval`, and `password` are ignored.

```json
{
  "server_url": "hy2://relay.example.com:8443",
  "token": "your_relay_token",
  "realm_id": "my-realm",
  "stun_servers": ["stun.example.com:3478"],
  "listen_ports": ["20000-30000", 40000],
  "prefer_ip_version": "prefer_ipv4",
  "fallback_timeout": "30s"
}
```

#### realm.server_url

==Required==

The relay server URL with format `hy2://[host]:[port]`.

#### realm.token

Realm authentication token, provided by the relay server administrator.

#### realm.realm_id

==Required==

The realm identifier, provided by the relay server administrator.

#### realm.stun_servers

STUN server list for NAT type detection.

#### realm.http_client

HTTP client options for relay connections.

#### realm.listen_ports

!!! warning "Realm-only option"

    This option is only available in `realm` mode and cannot be used with the `lport` query parameter in `server_url`.

Local port range for the relay connection.

Accepted formats:
- Array of integers: `[8080, 8081, 8082]`
- String range: `"20000-30000"` or `"20000-30000,40000"`
- Special values: `"all"` or `"*"` (use ephemeral port)

Port attempts are sequential, not randomized. First available port in the list is used.

Conflict: cannot be used when `server_url` contains `?lport=`.

#### realm.prefer_ip_version

IP version preference for relay connections.

| Value | Description |
|-------|-------------|
| `ipv4_only` | Use IPv4 only |
| `ipv6_only` | Use IPv6 only |
| `prefer_ipv4` | Prefer IPv4, fallback to IPv6 |
| `prefer_ipv6` | Prefer IPv6, fallback to IPv4 |

#### realm.fallback_timeout

Duration string for connection fallback timeout when primary path fails.

Accepts duration strings (`"30s"`, `"1m"`) or numeric seconds. Zero or empty disables fallback.

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
