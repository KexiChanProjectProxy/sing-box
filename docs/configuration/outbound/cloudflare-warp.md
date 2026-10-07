---
icon: material/new-box
---

# Cloudflare WARP

!!! quote "Changes in sing-box 1.14.2.1"

    :material-plus: [ephemeral](#ephemeral)

!!! question "Since sing-box 1.14.0.18"

Connects to Cloudflare WARP over MASQUE: an HTTP/3 CONNECT-IP tunnel that carries IP packets, served to sing-box through a user-space network stack. TCP and UDP are supported.

!!! info "Build tags"

    Requires the `with_quic` and `with_gvisor` build tags.

### Structure

```json
{
  "type": "cloudflare-warp",
  "tag": "warp",

  "server": "",
  "server_port": 443,
  "private_key": "",
  "address": [],
  "endpoint_public_key": "",
  "device_id": "",
  "access_token": "",
  "license": "",
  "access_jwt": "",
  "device_name": "",
  "ephemeral": false,
  "api_detour": "",
  "network": "",
  "mtu": 1280,
  "tls": {},

  ... // QUIC Fields

  ... // Dial Fields
}
```

### Registration modes

A WARP device must be registered with Cloudflare before it can connect. Three modes are available.

**Static credentials.** Run the command below and paste its output into `outbounds`. It registers a new device and prints `private_key`, `address`, `endpoint_public_key`, `device_id` and `access_token`.

```shell
sing-box generate warp-registration
```

The command accepts `--license`, `--access-jwt`, `--name`, `--model`, `--locale`, `--tag`, `--ipv6` and `--raw`.

**Automatic registration.** Leave `private_key` and `address` empty and enable [`experimental.cache_file`](/configuration/experimental/cache-file/). On first start the outbound registers a device and stores it in the cache file under the outbound tag. Later starts reuse it. If Cloudflare rejects the stored device, the outbound registers a new one once.

**Ephemeral registration.** Leave `private_key` and `address` empty and set [`ephemeral`](#ephemeral). Every start registers a new device that is kept only in memory, so no cache file or other state is needed. This suits stateless nodes.

!!! danger ""

    Registering a device accepts Cloudflare's [Terms of Service](https://www.cloudflare.com/application/terms/). The private key, access token and license are secrets.

### Fields

#### server

The tunnel server address.

Defaults to the IPv4 endpoint returned at registration, or `162.159.198.1`.

#### server_port

The tunnel server port. `443` is used by default.

#### private_key

The ECDSA P-256 private key enrolled for the device, as base64 SEC 1 or PKCS #8 DER, or PEM. The `private_key` value in a usque `config.json` is accepted as is.

Required together with `address` for static credentials.

#### address

The tunnel addresses assigned to the device, as `/32` and `/128` prefixes.

Required together with `private_key` for static credentials.

#### endpoint_public_key

The server's public key, as base64 PKIX DER or PEM. The server certificate is accepted only when it carries this key.

If empty, the server certificate is verified normally against `tls.server_name`, unless `tls.insecure` is set.

#### device_id

The device ID. Only used together with `license` in static mode.

#### access_token

The device access token. Only used together with `license` in static mode.

#### license

A WARP+ license key to bind to the device.

In automatic mode it is applied at registration, or when it differs from the stored license. In static mode `device_id` and `access_token` are required.

#### access_jwt

A Zero Trust team token sent as `CF-Access-Jwt-Assertion` during automatic registration.

#### device_name

The device name shown in the Cloudflare dashboard, set during automatic registration.

#### ephemeral

!!! question "Since sing-box 1.14.2.1"

Register a new device at every start, keep it only in memory, and delete it from Cloudflare when sing-box shuts down. No cache file is needed.

Deletion is best effort. A device left behind by a crash or a forced kill stays registered, which matters when `license` is set, since a license allows a limited number of devices.

Only used for automatic registration.

#### api_detour

The outbound tag used for registration API requests.

If empty, API requests use this outbound's dial fields.

!!! warning ""

    The API host `api.cloudflareclient.com` must be resolvable without this outbound. If your DNS server uses this outbound as `detour`, set a direct [`domain_resolver`](/configuration/shared/dial/#domain_resolver) on this outbound, or an `api_detour` with its own resolver. Otherwise registration waits on a tunnel that cannot connect yet.

#### network

Enabled network. One of `tcp` `udp`. Both are enabled by default.

#### mtu

The tunnel MTU. `1280` is used by default.

Lower it if the log reports dropped packets larger than the tunnel allows, for example when `initial_packet_size` is set.

#### tls

TLS configuration, see [TLS](/configuration/shared/tls/#outbound). `tls.enabled` is implied.

Only `server_name`, `insecure`, `certificate`, `certificate_path`, `min_version`, `max_version`, `cipher_suites`, `curve_preferences` and `handshake_timeout` apply. `server_name` defaults to `consumer-masque.cloudflareclient.com` and is not used to authenticate the server. uTLS, Reality, ECH, ALPN and client certificates are rejected.

### QUIC Fields

See [QUIC Fields](/configuration/shared/quic/) for details. `keep_alive_period` defaults to `30s`.

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
