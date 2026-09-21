!!! quote "Changes in sing-box 1.13.0"

    :material-plus: [quic_congestion_control](#quic_congestion_control)

### Structure

```json
{
"type": "naive",
"tag": "naive-in",
"network": "udp",
...
// Listen Fields

"users": [
{
"username": "sekai",
"password": "password"
}
],
"quic_congestion_control": "",
"tls": {}
}
```

### Listen Fields

See [Listen Fields](/configuration/shared/listen/) for details.

### Fields

#### network

Listen network, one of `tcp` `udp`.

Both if empty.

#### users

==Required==

Naive users.

#### quic_congestion_control

!!! question "Since sing-box 1.13.0"

QUIC congestion control algorithm.

| Algorithm      | Description                     |
|----------------|---------------------------------|
| `bbr`          | BBR                             |
| `bbr_standard` | BBR (Standard version)         |
| `bbr2`         | BBRv2                           |
| `bbr2_variant` | BBRv2 (An experimental variant) |
| `cubic`        | CUBIC                           |
| `reno`         | New Reno                        |

`bbr` is used by default (the default of QUICHE, used by Chromium which NaiveProxy is based on).

!!! note ""

    Since sing-box 1.14.0.16 the BBR variants share one BBR implementation: `bbr`, `bbr_standard` and `bbr2` use its standard profile, and `bbr2_variant` uses its aggressive profile. The values remain valid, so existing configurations keep working.

#### tls

TLS configuration, see [TLS](/configuration/shared/tls/#inbound).