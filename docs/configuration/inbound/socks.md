`socks` inbound is a socks4, socks4a, socks5 server.

### Structure

```json
{
  "type": "socks",
  "tag": "socks-in",

  ... // Listen Fields

  "users": [
    {
      "username": "admin",
      "password": "admin"
    }
  ],
  "auth_bypass": [
    "127.0.0.1/32",
    "192.168.0.0/16"
  ]
}
```

### Listen Fields

See [Listen Fields](/configuration/shared/listen/) for details.

### Fields

#### users

SOCKS users.

No authentication required if empty.

#### auth_bypass

List of source IP addresses or CIDR prefixes that skip authentication.

Connections from a matching address are accepted without credentials even when `users` is set. IPv4-mapped IPv6 addresses are matched as IPv4.
