---
icon: material/new-box
---

# Cloudflare WARP

!!! question "自 sing-box 1.14.0.18 起"

通过 MASQUE 连接 Cloudflare WARP：一条承载 IP 数据包的 HTTP/3 CONNECT-IP 隧道，经由用户态网络栈提供给 sing-box。支持 TCP 和 UDP。

!!! info "构建标签"

    需要 `with_quic` 和 `with_gvisor` 构建标签。

### 结构

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
  "api_detour": "",
  "network": "",
  "mtu": 1280,
  "tls": {},

  ... // QUIC 字段

  ... // 拨号字段
}
```

### 注册模式

WARP 设备必须先在 Cloudflare 注册才能连接。有两种模式。

**静态凭据。** 运行以下命令并将输出粘贴到 `outbounds`。它会注册一个新设备并输出 `private_key`、`address`、`endpoint_public_key`、`device_id` 和 `access_token`。

```shell
sing-box generate warp-registration
```

该命令支持 `--license`、`--access-jwt`、`--name`、`--model`、`--locale`、`--tag`、`--ipv6` 和 `--raw`。

**自动注册。** 留空 `private_key` 和 `address` 并启用 [`experimental.cache_file`](/zh/configuration/experimental/cache-file/)。首次启动时出站会注册设备，并以出站标签为键存入缓存文件。之后的启动会复用它。若 Cloudflare 拒绝已存储的设备，出站会重新注册一次。

!!! danger ""

    注册设备即表示接受 Cloudflare [服务条款](https://www.cloudflare.com/application/terms/)。私钥、访问令牌和许可证均为机密。

### 字段

#### server

隧道服务器地址。

默认使用注册时返回的 IPv4 端点，或 `162.159.198.1`。

#### server_port

隧道服务器端口。默认使用 `443`。

#### private_key

为设备注册的 ECDSA P-256 私钥，格式为 base64 SEC 1 或 PKCS #8 DER，或 PEM。可直接使用 usque `config.json` 中的 `private_key`。

静态凭据模式下须与 `address` 一同设置。

#### address

分配给设备的隧道地址，格式为 `/32` 和 `/128` 前缀。

静态凭据模式下须与 `private_key` 一同设置。

#### endpoint_public_key

服务器公钥，格式为 base64 PKIX DER 或 PEM。仅当服务器证书携带此公钥时才接受。

若为空，则按 `tls.server_name` 正常校验服务器证书，除非设置了 `tls.insecure`。

#### device_id

设备 ID。仅在静态模式下与 `license` 一同使用。

#### access_token

设备访问令牌。仅在静态模式下与 `license` 一同使用。

#### license

绑定到设备的 WARP+ 许可证。

自动模式下在注册时应用，或在与已存储许可证不同时应用。静态模式下需要 `device_id` 和 `access_token`。

#### access_jwt

自动注册时以 `CF-Access-Jwt-Assertion` 发送的 Zero Trust 团队令牌。

#### device_name

自动注册时设置的设备名称，显示在 Cloudflare 控制台中。

#### api_detour

注册 API 请求使用的出站标签。

若为空，API 请求使用本出站的拨号字段。

!!! warning ""

    API 主机 `api.cloudflareclient.com` 必须能在不经过本出站的情况下解析。若 DNS 服务器以本出站作为 `detour`，请为本出站设置直连的 [`domain_resolver`](/zh/configuration/shared/dial/#domain_resolver)，或设置拥有独立解析器的 `api_detour`。否则注册会等待一条尚无法连接的隧道。

#### network

启用的网络协议。`tcp` 或 `udp`。默认两者均启用。

#### mtu

隧道 MTU。默认使用 `1280`。

若日志报告丢弃了超过隧道限制的数据包（例如设置了 `initial_packet_size` 时），请调低该值。

#### tls

TLS 配置，参阅 [TLS](/zh/configuration/shared/tls/#outbound)。默认启用 `tls.enabled`。

仅 `server_name`、`insecure`、`certificate`、`certificate_path`、`min_version`、`max_version`、`cipher_suites`、`curve_preferences` 和 `handshake_timeout` 生效。`server_name` 默认为 `consumer-masque.cloudflareclient.com`，且不用于验证服务器身份。uTLS、Reality、ECH、ALPN 和客户端证书会被拒绝。

### QUIC 字段

参阅 [QUIC 字段](/zh/configuration/shared/quic/)。`keep_alive_period` 默认为 `30s`。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
