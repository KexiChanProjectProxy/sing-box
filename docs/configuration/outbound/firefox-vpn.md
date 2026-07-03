### Structure

```json
{
  "type": "firefox-vpn",
  "tag": "firefox-vpn-out",

  "server": "vpn.example.com",
  "server_port": 443,
  "email": "user@example.com",
  "password": "user-password",
  "api_detour": "direct",
  "tls": {
    "enabled": true,
    "server_name": "vpn.example.com"
  },

  ... // Dial Fields
}
```

!!! warning "V1 Limitations"

    This is the V1 implementation. It supports **TCP only**. UDP, MASQUE, disk cache, and serverlist auto-selection are **not supported** in this version.

### Fields

#### server

==Required==

The Firefox VPN server address.

#### server_port

==Required==

The Firefox VPN server port.

#### email

==Required==

Your Firefox account email address. Used to authenticate with the Firefox Accounts (FxA) service.

#### password

==Required==

Your Firefox account password. Used to authenticate with the Firefox Accounts (FxA) service.

!!! danger ""

    Credentials are sent to the Firefox Accounts and Guardian services over HTTPS. They are never stored on disk.

#### api_detour

The outbound tag used for API requests to Firefox Accounts and Guardian.

If omitted, API requests go through the default routing table.

#### tls

TLS configuration, see [TLS](/configuration/shared/tls/#outbound).

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
