---
icon: material/alert-decagram
---

#### 1.13.11

* 修复在 1.13.9 中引入的进程搜索器故障
* 修复与改进

#### 1.13.10

* 修复在 1.13.9 中引入的进程搜索器故障

#### 1.13.9

* 修复与改进

#### 1.13.8

* 更新 naiveproxy 至 v147.0.7727.49-1
* 修复 fake-ip DNS 服务器在未配置地址类型时返回 SUCCESS 的问题
* 修复与改进

#### 1.13.7

* 修复与改进

#### 1.13.6

* 修复与改进

#### 1.13.5

* 修复与改进

#### 1.13.4

* 修复与改进

#### 1.13.3

* 添加 OpenWrt 和 Alpine APK 包到发布版本 **1**
* 回退支持 macOS 10.13 High Sierra **2**
* OCM 服务：为 Responses API 添加 WebSocket 支持 **3**
* 修复与改进

**1**：

Alpine APK 文件在文件名中使用 `linux` 以区别使用 `openwrt` 前缀的 OpenWrt APK：

- OpenWrt: `sing-box_{version}_openwrt_{architecture}.apk`
- Alpine: `sing-box_{version}_linux_{architecture}.apk`

**2**：

旧版 macOS 二进制文件（带 `-legacy-macos-10.13` 后缀）现支持
macOS 10.13 High Sierra，使用 Go 1.25 补丁构建
来自 [SagerNet/go](https://github.com/SagerNet/go)。

**3**：

参阅 [OCM](/zh/configuration/service/ocm)。

#### 1.13.2

* 修复与改进

#### 1.13.1

* 修复与改进

#### 1.12.14

* 回退修复

#### 1.13.0

自 1.12 以来的重要变化：

* 添加 NaiveProxy 出站 **1**
* 添加对 `auto_redirect` 的预匹配支持 **2**
* 改进 `auto_redirect` **3**
* 添加 Chrome Root Store 证书选项 **4**
* 为 ACME DNS-01 挑战提供商添加新选项 **5**
* 为 Linux 和 Windows 添加 Wi-Fi 状态支持 **6**
* 为 TLS 选项添加曲线偏好、固定公钥 SHA256、mTLS 和 ECH `query_server_name` **7**
* 添加 kTLS 支持 **8**
* 添加 ICMP echo (ping) 代理支持 **9**
* 添加 `interface_address`、`network_interface_address` 和 `default_interface_address` 规则项 **10**
* 添加 `preferred_by` 路由规则项 **11**
* 改进 `local` DNS 服务器 **12**
* 为 listen 和 dial 字段添加 `disable_tcp_keep_alive`、`tcp_keep_alive` 和 `tcp_keep_alive_interval` 选项 **13**
* 为 dial 字段添加 `bind_address_no_port` 选项 **14**
* 为 Tailscale 端点添加系统接口、中继服务器和广告标签选项 **15**
* 添加 Claude Code 复用器服务 **16**
* 添加 OpenAI Codex 复用器服务 **17**
* Apple/Android：重构 GUI
* Apple/Android：添加通过 [QRS](https://github.com/qifi-dev/qrs) 分享配置的支持
* Android：添加通过 Xposed 抵抗 VPN 检测的支持
* 放弃对 go1.23 的支持 **18**
* 放弃对 Android 5.0 的支持 **19**
* 更新 uTLS 至 v1.8.2 **20**
* 更新 quic-go 至 v0.59.0
* 更新 gVisor 至 v20250811
* 更新 Tailscale 至 v1.92.4

**1**：

NaiveProxy 出站现支持 QUIC、ECH、UDP over TCP 和可配置的 QUIC 拥塞控制。

仅在 Apple 平台、Android、Windows 和部分 Linux 架构上可用。
每个 Windows 发布版本都包含 `libcronet.dll` ——
请确保此文件与 `sing-box.exe` 在同一目录，或在 `PATH` 列出的目录中。

参阅 [NaiveProxy 出站](/zh/configuration/outbound/naive/)。

**2**：

`auto_redirect` 现允许您根据路由规则绕过 sing-box 进行连接。

引入新的规则动作 `bypass` 以支持此功能：当在预匹配中匹配时，连接将绕过 sing-box 直接连接。

此功能需要启用 `auto_redirect` 的 Linux。

参阅 [预匹配](/zh/configuration/shared/pre-match/) 和 [规则动作](/zh/configuration/route/rule_action/#bypass)。

**3**：

`auto_redirect` 现默认拒绝 MPTCP 连接以修复兼容性问题。
您可通过新的 `exclude_mptcp` 选项将其更改为绕过 sing-box。

添加备用 iproute2 规则，在系统默认规则（32766: main, 32767: default）之后检查，
确保当系统表中未找到路由时，流量被路由到 sing-box 表。
规则索引可通过 `auto_redirect_iproute2_fallback_rule_index` 自定义（默认：32768）。

参阅 [TUN](/zh/configuration/inbound/tun/#exclude_mptcp)。

**4**：

添加 `chrome` 作为新的证书存储选项，与 `mozilla` 并列。
两个存储都会过滤掉中国 CA 证书。

参阅 [证书](/zh/configuration/certificate/#store)。

**5**：

参阅 [DNS-01 挑战](/zh/configuration/shared/dns01_challenge/)。

**6**：

sing-box 现可以在 Linux 和 Windows 上监控 Wi-Fi 状态，以启用基于 `wifi_ssid` 和 `wifi_bssid` 的路由规则。

参阅 [Wi-Fi 状态](/zh/configuration/shared/wifi-state/)。

**7**：

参阅 [TLS](/zh/configuration/shared/tls/)。

**8**：

为 TLS 入站添加 `kernel_tx` 和 `kernel_rx` 选项。
在 Linux 5.1+ 上通过 `splice(2)` 实现 TLS 1.3 内核级 TLS 卸载。

参阅 [TLS](/zh/configuration/shared/tls/)。

**9**：

sing-box 现可以代理 ICMP echo (ping) 请求。
路由规则中有新的 `icmp` 网络类型可用。
支持从 TUN、WireGuard 和 Tailscale 入站到 Direct、WireGuard 和 Tailscale 出站。
`reject` 动作也可以回复 ICMP echo 请求。

**10**：

New rule items for matching based on interface IP addresses, available in route rules, DNS rules and rule-sets.

**11**：

Matches outbounds' preferred routes.
对于 Tailscale：MagicDNS 域名和同级的允许 IP。对于 WireGuard：同级的允许 IP。

**12**：

The `local` DNS server now uses platform-native resolution:
`getaddrinfo`/libresolv on Apple platforms, systemd-resolved DBus on Linux.
A new `prefer_go` option is available to opt out.

参阅 [Local DNS](/zh/configuration/dns/server/local/).

**13**：

The default TCP keep-alive initial period has been updated from 10 minutes to 5 minutes.

参阅 [Dial Fields](/zh/configuration/shared/dial/#tcp_keep_alive).

**14**：

Adds the Linux socket option `IP_BIND_ADDRESS_NO_PORT` support when explicitly binding to a source address.

This allows reusing the same source port for multiple connections, improving scalability for high-concurrency proxy scenarios.

参阅 [Dial Fields](/zh/configuration/shared/dial/#bind_address_no_port).

**15**：

Tailscale endpoint can now create a system TUN interface to handle traffic directly.
New `relay_server_port` and `relay_server_static_endpoints` options for incoming relay connections.
New `advertise_tags` option for ACL tag advertisement.

参阅 [Tailscale endpoint](/zh/configuration/endpoint/tailscale/).

**16**：

CCM (Claude Code Multiplexer) service allows you to access your local Claude Code subscription remotely through custom tokens, eliminating the need for OAuth authentication on remote clients.

参阅 [CCM](/zh/configuration/service/ccm).

**17**：

参阅 [OCM](/zh/configuration/service/ocm)。

**18**：

Due to maintenance difficulties, sing-box 1.13.0 requires at least Go 1.24 to compile.

**19**：

Due to maintenance difficulties, sing-box 1.13.0 will be the last version to support Android 5.0,
and only through a separate legacy build (with `-legacy-android-5` suffix).

对于独立二进制文件，最低 Android 版本已提高到 Android 6.0，
since Termux requires Android 7.0 or later.

**20**：

This update fixes missing padding extension for Chrome 120+ fingerprints.

Also, documentation has been updated with a warning about uTLS fingerprinting vulnerabilities.
uTLS is not recommended for censorship circumvention due to fundamental architectural limitations;
use NaiveProxy instead for TLS fingerprint resistance.

#### 1.12.23

* 修复与改进

#### 1.13.0-rc.5

* 为 NaiveProxy 出站添加 `mipsle`、`mips64le`、`riscv64` 和 `loong64` 支持

#### 1.12.22

* 修复与改进

#### 1.13.0-rc.3

* 修复与改进

#### 1.12.21

* 修复与改进

#### 1.13.0-rc.2

* 修复与改进

#### 1.12.20

* 修复与改进

#### 1.13.0-rc.1

* 修复与改进

#### 1.12.19

* 修复与改进

#### 1.13.0-beta.8

* 为 `auto_redirect` 添加备用路由规则 **1**
* 修复与改进

**1**：

添加备用 iproute2 规则，在系统默认规则（32766: main, 32767: default）之后检查，
确保当系统表中未找到路由时，流量被路由到 sing-box 表。

规则索引可通过 `auto_redirect_iproute2_fallback_rule_index` 自定义（默认：32768）。

#### 1.12.18

* 为 `auto_redirect` 添加备用路由规则 **1**
* 修复与改进

**1**：

添加备用 iproute2 规则，在系统默认规则（32766: main, 32767: default）之后检查，
确保当系统表中未找到路由时，流量被路由到 sing-box 表。

规则索引可通过 `auto_redirect_iproute2_fallback_rule_index` 自定义（默认：32768）。

#### 1.13.0-beta.6

* 更新 uTLS 至 v1.8.2 **1**
* 修复与改进

**1**：

This update fixes missing padding extension for Chrome 120+ fingerprints.

Also, documentation has been updated with a warning about uTLS fingerprinting vulnerabilities.
uTLS is not recommended for censorship circumvention due to fundamental architectural limitations;
use NaiveProxy instead for TLS fingerprint resistance.

#### 1.12.17

* 更新 uTLS 至 v1.8.2 **1**
* 修复与改进

**1**：

This update fixes missing padding extension for Chrome 120+ fingerprints.

Also, documentation has been updated with a warning about uTLS fingerprinting vulnerabilities.
uTLS is not recommended for censorship circumvention due to fundamental architectural limitations;
use NaiveProxy instead for TLS fingerprint resistance.

#### 1.13.0-beta.5

* 修复与改进

#### 1.12.16

* 修复与改进

#### 1.13.0-beta.4

* Apple/Android：添加通过 [QRS](https://github.com/qifi-dev/qrs) 分享配置的支持
* Android：添加通过 Xposed 抵抗 VPN 检测的支持
* 更新 quic-go 至 v0.59.0
* 修复与改进

#### 1.13.0-beta.2

* 为 dial 字段添加 `bind_address_no_port` 选项 **1**
* 修复与改进

**1**：

Adds the Linux socket option `IP_BIND_ADDRESS_NO_PORT` support when explicitly binding to a source address.

This allows reusing the same source port for multiple connections, improving scalability for high-concurrency proxy scenarios.

参阅 [Dial Fields](/zh/configuration/shared/dial/#bind_address_no_port).

#### 1.13.0-beta.1

* 为 Tailscale 端点添加系统接口支持 **1**
* 修复与改进

**1**：

Tailscale endpoint can now create a system TUN interface to handle traffic directly.

参阅 [Tailscale endpoint](/zh/configuration/endpoint/tailscale/#system_interface).

#### 1.12.15

* 修复与改进

#### 1.13.0-alpha.36

* 降级 quic-go 至 v0.57.1
* 修复与改进

#### 1.13.0-alpha.35

* 添加对 `auto_redirect` 的预匹配支持 **1**
* 修复与改进

**1**：

`auto_redirect` 现允许您根据路由规则绕过 sing-box 进行连接。

引入新的规则动作 `bypass` 以支持此功能：当在预匹配中匹配时，连接将绕过 sing-box 直接连接。

此功能需要启用 `auto_redirect` 的 Linux。

参阅 [预匹配](/zh/configuration/shared/pre-match/) 和 [规则动作](/zh/configuration/route/rule_action/#bypass)。

#### 1.13.0-alpha.34

* 添加 Chrome Root Store 证书选项 **1**
* 为 ACME DNS-01 挑战提供商添加新选项 **2**
* 为 Linux 和 Windows 添加 Wi-Fi 状态支持 **3**
* 更新 naiveproxy 至 143.0.7499.109
* 更新 quic-go 至 v0.58.0
* 更新 tailscale 至 v1.92.4
* 放弃对 go1.23 的支持 **4**
* 放弃对 Android 5.0 的支持 **5**

**1**：

添加 `chrome` 作为新的证书存储选项，与 `mozilla` 并列。
两个存储都会过滤掉中国 CA 证书。

参阅 [证书](/zh/configuration/certificate/#store)。

**2**：

参阅 [DNS-01 挑战](/zh/configuration/shared/dns01_challenge/)。

**3**：

sing-box 现可以在 Linux 和 Windows 上监控 Wi-Fi 状态，以启用基于 `wifi_ssid` 和 `wifi_bssid` 的路由规则。

参阅 [Wi-Fi 状态](/zh/configuration/shared/wifi-state/)。

**4**：

Due to maintenance difficulties, sing-box 1.13.0 requires at least Go 1.24 to compile.

**5**：

Due to maintenance difficulties, sing-box 1.13.0 will be the last version to support Android 5.0,
and only through a separate legacy build (with `-legacy-android-5` suffix).

对于独立二进制文件，最低 Android 版本已提高到 Android 6.0，
since Termux requires Android 7.0 or later.

#### 1.12.14

* 修复与改进

#### 1.13.0-alpha.33

* 修复与改进

#### 1.13.0-alpha.32

* 移除 `certificate_public_key_sha256` option for NaiveProxy outbound **1**
* 修复与改进

**1**：

Self-signed certificates change traffic behavior significantly, which defeats the purpose of NaiveProxy's design to resist traffic analysis.
由于这个原因以及维护成本，不再有理由继续支持 `certificate_public_key_sha256`，它原本是为了简化自签名证书的使用而设计的。

#### 1.13.0-alpha.31

* 为 NaiveProxy 出站添加 QUIC 支持 **1**
* 为 NaiveProxy 添加 QUIC 拥塞控制选项 **2**
* 修复与改进

**1**：

NaiveProxy outbound now supports QUIC.

参阅 [NaiveProxy outbound](/zh/configuration/outbound/naive/#quic).

**2**：

NaiveProxy inbound and outbound now supports configurable QUIC congestion control algorithms, including BBR and BBRv2.

参阅 [NaiveProxy inbound](/zh/configuration/inbound/naive/#quic_congestion_control) and [NaiveProxy outbound](/zh/configuration/outbound/naive/#quic_congestion_control).

#### 1.13.0-alpha.30

* 为 NaiveProxy 出站添加 ECH 支持 **1**
* 添加 `tls.ech.query_server_name` 选项 **2**
* 修复 Windows 上的 NaiveProxy 出站 **3**
* 添加 OpenAI Codex 复用器服务 **4**
* 修复与改进

**1**：

参阅 [NaiveProxy outbound](/zh/configuration/outbound/naive/#tls).

**2**：

参阅 [TLS](/zh/configuration/shared/tls/#query_server_name).

**3**：

每个 Windows 发布版本现在都包含 `libcronet.dll`。
Ensure this file is in the same directory as `sing-box.exe` or in a directory listed in `PATH`.

**4**：

参阅 [OCM](/zh/configuration/service/ocm)。

#### 1.13.0-alpha.29

* 为 naiveproxy 出站添加 UDP over TCP 支持 **1**
* 修复与改进

**1**：

参阅 [NaiveProxy outbound](/zh/configuration/outbound/naive/#udp_over_tcp).

#### 1.13.0-alpha.28

* 添加 naiveproxy 出站 **1**
* 为 dial 字段添加 `disable_tcp_keep_alive`、`tcp_keep_alive` 和 `tcp_keep_alive_interval` 选项 **2**
* 将默认 TCP keep-alive 初始周期从 10 分钟改为 5 分钟
* 更新 quic-go 至 v0.57.1
* 修复与改进

**1**：

仅在 Apple 平台、Android、Windows 和部分 Linux 架构上可用。

参阅 [NaiveProxy 出站](/zh/configuration/outbound/naive/)。

**2**：

参阅 [Dial Fields](/zh/configuration/shared/dial/#tcp_keep_alive).

* __Unfortunately, for non-technical reasons, we are currently unable to notarize the standalone version of the macOS client:
because system extensions require signatures to function, we have had to temporarily halt its release.__

__We plan to fix the App Store release issue and launch a new standalone desktop client, but until then,
only clients on TestFlight will be available (unless you have an Apple Developer Program and compile from source code).__


#### 1.12.13

* 修复 naive 入站
* 修复与改进

__Unfortunately, for non-technical reasons, we are currently unable to notarize the standalone version of the macOS client:
because system extensions require signatures to function, we have had to temporarily halt its release.__

__We plan to fix the App Store release issue and launch a new standalone desktop client, but until then,
only clients on TestFlight will be available (unless you have an Apple Developer Program and compile from source code).__

#### 1.12.12

* 修复与改进

#### 1.13.0-alpha.26

* 更新 quic-go 至 v0.55.0
* 修复 hysteria2 中的内存泄漏
* 修复与改进

#### 1.12.11

* 修复与改进

#### 1.13.0-alpha.24

* 添加 Claude Code 复用器服务 **1**
* 修复与改进

**1**：

CCM (Claude Code Multiplexer) service allows you to access your local Claude Code subscription remotely through custom tokens, eliminating the need for OAuth authentication on remote clients.

参阅 [CCM](/zh/configuration/service/ccm).

#### 1.13.0-alpha.23

* 修复与 MPTCP 的兼容性 **1**
* 修复与改进

**1**：

`auto_redirect` now rejects MPTCP connections by default to fix compatibility issues,
but you can change it to bypass the sing-box via the new `exclude_mptcp` option.

参阅 [TUN](/zh/configuration/inbound/tun/#exclude_mptcp)。

#### 1.13.0-alpha.22

* 更新 uTLS 至 v1.8.1 **1**
* 修复与改进

**1**：

This update fixes an critical issue that could cause simulated Chrome fingerprints to be detected,
see https://github.com/refraction-networking/utls/pull/375.

#### 1.12.10

* 更新 uTLS 至 v1.8.1 **1**
* 修复与改进

**1**：

This update fixes an critical issue that could cause simulated Chrome fingerprints to be detected,
see https://github.com/refraction-networking/utls/pull/375.

#### 1.13.0-alpha.21

* 修复客户端选项中缺失的 mTLS 支持 **1**
* 修复与改进

参阅 [TLS](/zh/configuration/shared/tls/)。

#### 1.12.9

* 修复与改进

#### 1.13.0-alpha.16

* 为 TLS 选项添加曲线偏好、固定公钥 SHA256 和 mTLS **1**
* 修复与改进

参阅 [TLS](/zh/configuration/shared/tls/)。

#### 1.13.0-alpha.15

* 更新 quic-go 至 v0.54.0
* 更新 gVisor 至 v20250811
* 更新 Tailscale 至 v1.86.5
* 修复与改进

#### 1.12.8

* 修复与改进

#### 1.13.0-alpha.11

* 修复与改进

#### 1.12.5

* 修复与改进

#### 1.13.0-alpha.10

* 改进 kTLS 支持 **1**
* 修复与改进

**1**：

kTLS is now compatible with custom TLS implementations other than uTLS.

#### 1.12.4

* 修复与改进

#### 1.12.3

* 修复与改进

#### 1.12.2

* 修复与改进

#### 1.12.1

* 修复与改进

#### 1.12.0

* 重构 DNS servers **1**
* 添加域名解析器选项 **2**
* 为路由选项和出站 TLS 选项添加 TLS 片段/记录片段支持 **3**
* 添加证书选项 **4**
* 添加 Tailscale 端点和 DNS 服务器 **5**
* 放弃对 go1.22 的支持 **6**
* 添加 AnyTLS 协议 **7**
* 迁移 to stdlib ECH implementation **8**
* 添加 NTP 嗅探器 **9**
* 为 ShadowTLS 入站添加通配符 SNI 支持 **10**
* 改进 `auto_redirect` **11**
* 为监听器添加控制选项 **12**
* 添加 DERP 服务 **13**
* 添加 Resolved 服务和 DNS 服务器 **14**
* 添加 SSM API 服务 **15**
* 为 tun 添加回环地址支持 **16**
* 改进 Apple 平台上的 tun 性能 **17**
* 更新 quic-go 至 v0.52.0
* 更新 gVisor 至 20250319.0
* 更新应用商店中图形客户端的状态 **18**

**1**：

DNS servers are refactored for better performance and scalability.

参阅 [DNS server](/zh/configuration/dns/server/).

有关迁移，请参阅 [迁移到新 DNS 服务器格式](/zh/migration/#migrate-to-new-dns-servers)。

Compatibility for old formats will be removed in sing-box 1.14.0.

**2**：

Legacy `outbound` DNS rules are deprecated
and can be replaced by the new `domain_resolver` option.

参阅 [Dial Fields](/zh/configuration/shared/dial/#domain_resolver) and
[Route](/zh/configuration/route/#default_domain_resolver).

有关迁移，
see [Migrate outbound DNS rule items to domain resolver](/zh/migration/#migrate-outbound-dns-rule-items-to-domain-resolver).

**3**：

参阅 [Route Action](/zh/configuration/route/rule_action/#tls_fragment) and [TLS](/zh/configuration/shared/tls/).

**4**：

New certificate options allow you to manage the default list of trusted X509 CA certificates.

对于系统证书列表，修复了 Go 无法正确读取 Android 受信证书的问题。

You can also use the Mozilla Included List instead, or add trusted certificates yourself.

参阅 [Certificate](/zh/configuration/certificate/).

**5**：

参阅 [Tailscale](/zh/configuration/endpoint/tailscale/).

**6**：

Due to maintenance difficulties, sing-box 1.12.0 requires at least Go 1.23 to compile.

对于 Windows 7 用户，旧版二进制文件现继续使用 Go 1.23 和补丁编译
from [MetaCubeX/go](https://github.com/MetaCubeX/go).

**7**：

The new AnyTLS protocol claims to mitigate TLS proxy traffic characteristics and comes with a new multiplexing scheme.

参阅 [AnyTLS Inbound](/zh/configuration/inbound/anytls/) and [AnyTLS Outbound](/zh/configuration/outbound/anytls/).

**8**：

参阅 [TLS](/zh/configuration/shared/tls).

The build tag `with_ech` is no longer needed and has been removed.

**9**：

参阅 [Protocol Sniff](/zh/configuration/route/sniff/).

**10**：

参阅 [ShadowTLS](/zh/configuration/inbound/shadowtls/#wildcard_sni).

**11**：

Now `auto_redirect` fixes compatibility issues between tun and Docker bridge networks,
see [Tun](/zh/configuration/inbound/tun/#auto_redirect).

**12**：

You can now set `bind_interface`, `routing_mark` and `reuse_addr` in Listen Fields.

参阅 [Listen Fields](/zh/configuration/shared/listen/).

**13**：

DERP service is a Tailscale DERP server, similar to [derper](https://pkg.go.dev/tailscale.com/cmd/derper).

参阅 [DERP Service](/zh/configuration/service/derp/).

**14**：

Resolved service is a fake systemd-resolved DBUS service to receive DNS settings from other programs
(e.g. NetworkManager) and provide DNS resolution.

参阅 [Resolved Service](/zh/configuration/service/resolved/) and [Resolved DNS Server](/zh/configuration/dns/server/resolved/).

**15**：

SSM API service is a RESTful API server for managing Shadowsocks servers.

参阅 [SSM API Service](/zh/configuration/service/ssm-api/).

**16**：

TUN now implements SideStore's StosVPN.

参阅 [Tun](/zh/configuration/inbound/tun/#loopback_address).

**17**：

We have significantly improved the performance of tun inbound on Apple platforms, especially in the gVisor stack.

The following data was tested
using [tun_bench](https://github.com/SagerNet/sing-box/blob/dev-next/cmd/internal/tun_bench/main.go) on M4 MacBook pro.

| Version     | Stack  | MTU   | Upload | Download |
|-------------|--------|-------|--------|----------|
| 1.11.15     | gvisor | 1500  | 852M   | 2.57G    |
| 1.12.0-rc.4 | gvisor | 1500  | 2.90G  | 4.68G    |
| 1.11.15     | gvisor | 4064  | 2.31G  | 6.34G    |
| 1.12.0-rc.4 | gvisor | 4064  | 7.54G  | 12.2G    |
| 1.11.15     | gvisor | 65535 | 27.6G  | 18.1G    |
| 1.12.0-rc.4 | gvisor | 65535 | 39.8G  | 34.7G    |
| 1.11.15     | system | 1500  | 664M   | 706M     |
| 1.12.0-rc.4 | system | 1500  | 2.44G  | 2.51G    |
| 1.11.15     | system | 4064  | 1.88G  | 1.94G    |
| 1.12.0-rc.4 | system | 4064  | 6.45G  | 6.27G    |
| 1.11.15     | system | 65535 | 26.2G  | 17.4G    |
| 1.12.0-rc.4 | system | 65535 | 17.6G  | 21.0G    |

**18**：

We continue to experience issues updating our sing-box apps on the App Store and Play Store.
Until we rewrite and resubmit the apps, they are considered irrecoverable.
Therefore, after this release, we will not be repeating this notice unless there is new information.

### 1.11.15

* 修复与改进

_We are temporarily unable to update sing-box apps on the App Store because the reviewer mistakenly found that we
violated the rules (TestFlight users are not affected)._

#### 1.12.0-beta.32

* 改进 tun performance on Apple platforms **1**
* 修复与改进

**1**：

We have significantly improved the performance of tun inbound on Apple platforms, especially in the gVisor stack.

### 1.11.14

* 修复与改进

_We are temporarily unable to update sing-box apps on the App Store because the reviewer mistakenly found that we
violated the rules (TestFlight users are not affected)._

#### 1.12.0-beta.24

* 允许 `tls_fragment` and `tls_record_fragment` to be enabled together **1**
* 同时添加 fragment options for TLS client configuration **2**
* 修复与改进

**1**：

仅用于调试，如果记录分片正常工作，建议禁用。

参阅 [Route Action](/zh/configuration/route/rule_action/#tls_fragment).

**2**：

参阅 [TLS](/zh/configuration/shared/tls/)。

#### 1.12.0-beta.23

* 添加 loopback address support for tun **1**
* 添加 cache support for ssm-api **2**
* 修复与改进

**1**：

TUN now implements SideStore's StosVPN.

参阅 [Tun](/zh/configuration/inbound/tun/#loopback_address).

**2**：

参阅 [SSM API Service](/zh/configuration/service/ssm-api/#cache_path).

#### 1.12.0-beta.21

* 修复 missing `home` option for DERP service **1**
* 修复与改进

**1**：

You can now choose what the DERP home page shows, just like with derper's `-home` flag.

参阅 [DERP](/zh/configuration/service/derp/#home).

### 1.11.13

* 修复与改进

_We are temporarily unable to update sing-box apps on the App Store because the reviewer mistakenly found that we
violated the rules (TestFlight users are not affected)._

#### 1.12.0-beta.17

* 更新 quic-go 至 v0.52.0
* 修复与改进

#### 1.12.0-beta.15

* 添加 DERP service **1**
* 添加 Resolved service and DNS server **2**
* 添加 SSM API service **3**
* 修复与改进

**1**：

DERP service is a Tailscale DERP server, similar to [derper](https://pkg.go.dev/tailscale.com/cmd/derper).

参阅 [DERP Service](/zh/configuration/service/derp/).

**2**：

Resolved service is a fake systemd-resolved DBUS service to receive DNS settings from other programs
(e.g. NetworkManager) and provide DNS resolution.

参阅 [Resolved Service](/zh/configuration/service/resolved/) and [Resolved DNS Server](/zh/configuration/dns/server/resolved/).

**3**：

SSM API service is a RESTful API server for managing Shadowsocks servers.

参阅 [SSM API Service](/zh/configuration/service/ssm-api/).

### 1.11.11

* 修复与改进

_We are temporarily unable to update sing-box apps on the App Store because the reviewer mistakenly found that we
violated the rules (TestFlight users are not affected)._

#### 1.12.0-beta.13

* 添加 TLS record fragment route options **1**
* 添加 missing `accept_routes` option for Tailscale **2**
* 修复与改进

**1**：

参阅 [Route Action](/zh/configuration/route/rule_action/#tls_record_fragment).

**2**：

参阅 [Tailscale](/zh/configuration/endpoint/tailscale/#accept_routes).

#### 1.12.0-beta.10

* 添加 control options for listeners **1**
* 修复与改进

**1**：

You can now set `bind_interface`, `routing_mark` and `reuse_addr` in Listen Fields.

参阅 [Listen Fields](/zh/configuration/shared/listen/).

### 1.11.10

* 取消弃用 `block` 出站 **1**
* 修复与改进

**1**：

Since we don’t have a replacement for using the `block` outbound in selectors yet,
we decided to temporarily undeprecate the `block` outbound until a replacement is available in the future.

_We are temporarily unable to update sing-box apps on the App Store because the reviewer mistakenly found that we
violated the rules (TestFlight users are not affected)._

#### 1.12.0-beta.9

* 更新 quic-go 至 v0.51.0
* 修复与改进

### 1.11.9

* 修复与改进

_We are temporarily unable to update sing-box apps on the App Store because the reviewer mistakenly found that we
violated the rules (TestFlight users are not affected)._

#### 1.12.0-beta.5

* 修复与改进

### 1.11.8

* 改进 `auto_redirect` **1**
* 修复与改进

**1**：

Now `auto_redirect` fixes compatibility issues between TUN and Docker bridge networks,
see [Tun](/zh/configuration/inbound/tun/#auto_redirect).

_We are temporarily unable to update sing-box apps on the App Store because the reviewer mistakenly found that we
violated the rules (TestFlight users are not affected)._

#### 1.12.0-beta.3

* 修复与改进

### 1.11.7

* 修复与改进

_We are temporarily unable to update sing-box apps on the App Store because the reviewer mistakenly found that we
violated the rules (TestFlight users are not affected)._

#### 1.12.0-beta.1

* 修复与改进

**1**：

Now `auto_redirect` fixes compatibility issues between tun and Docker bridge networks,
see [Tun](/zh/configuration/inbound/tun/#auto_redirect).

### 1.11.6

* 修复与改进

_We are temporarily unable to update sing-box apps on the App Store because the reviewer mistakenly found that we
violated the rules (TestFlight users are not affected)._

#### 1.12.0-alpha.19

* 更新 gVisor 至 20250319.0
* 修复与改进

#### 1.12.0-alpha.18

* 添加 wildcard SNI support for ShadowTLS inbound **1**
* 修复与改进

**1**：

参阅 [ShadowTLS](/zh/configuration/inbound/shadowtls/#wildcard_sni).

#### 1.12.0-alpha.17

* 添加 NTP sniffer **1**
* 修复与改进

**1**：

参阅 [Protocol Sniff](/zh/configuration/route/sniff/).

#### 1.12.0-alpha.16

* 更新 `domain_resolver` 行为 **1**
* 修复与改进

**1**：

`route.default_domain_resolver` or `outbound.domain_resolver` is now optional when only one DNS server is configured.

参阅 [Dial Fields](/zh/configuration/shared/dial/#domain_resolver).

### 1.11.5

* 修复与改进

_We are temporarily unable to update sing-box apps on the App Store because the reviewer mistakenly found that we
violated the rules (TestFlight users are not affected)._

#### 1.12.0-alpha.13

* 移动 `predefined` DNS server to DNS rule action **1**
* 修复与改进

**1**：

参阅 [DNS Rule Action](/zh/configuration/dns/rule_action/#predefined).

### 1.11.4

* 修复与改进

#### 1.12.0-alpha.11

* 修复与改进

#### 1.12.0-alpha.10

* 添加 AnyTLS protocol **1**
* 改进 `resolve` route action **2**
* 迁移 to stdlib ECH implementation **3**
* 修复与改进

**1**：

The new AnyTLS protocol claims to mitigate TLS proxy traffic characteristics and comes with a new multiplexing scheme.

参阅 [AnyTLS Inbound](/zh/configuration/inbound/anytls/) and [AnyTLS Outbound](/zh/configuration/outbound/anytls/).

**2**：

`resolve` route action now accepts `disable_cache` and other options like in DNS route actions,
see [Route Action](/zh/configuration/route/rule_action).

**3**：

参阅 [TLS](/zh/configuration/shared/tls).

The build tag `with_ech` is no longer needed and has been removed.

#### 1.12.0-alpha.7

* 添加 Tailscale DNS server **1**
* 修复与改进

**1**：

参阅 [Tailscale](/zh/configuration/dns/server/tailscale/).

#### 1.12.0-alpha.6

* 添加 Tailscale endpoint **1**
* 放弃对 go1.22 的支持 **2**
* 修复与改进

**1**：

参阅 [Tailscale](/zh/configuration/endpoint/tailscale/).

**2**：

Due to maintenance difficulties, sing-box 1.12.0 requires at least Go 1.23 to compile.

对于 Windows 7 用户，旧版二进制文件现继续使用 Go 1.23 和补丁编译
from [MetaCubeX/go](https://github.com/MetaCubeX/go).

### 1.11.3

* 修复与改进

_This version overwrites 1.11.2, as incorrect binaries were released due to a bug in the continuous integration
process._

#### 1.12.0-alpha.5

* 修复与改进

### 1.11.1

* 修复与改进

#### 1.12.0-alpha.2

* 更新 quic-go 至 v0.49.0
* 修复与改进

#### 1.12.0-alpha.1

* 重构 DNS servers **1**
* 添加域名解析器选项 **2**
* 添加 TLS fragment route options **3**
* 添加证书选项 **4**

**1**：

DNS servers are refactored for better performance and scalability.

参阅 [DNS server](/zh/configuration/dns/server/).

有关迁移，请参阅 [迁移到新 DNS 服务器格式](/zh/migration/#migrate-to-new-dns-servers)。

Compatibility for old formats will be removed in sing-box 1.14.0.

**2**：

Legacy `outbound` DNS rules are deprecated
and can be replaced by the new `domain_resolver` option.

参阅 [Dial Fields](/zh/configuration/shared/dial/#domain_resolver) and
[Route](/zh/configuration/route/#default_domain_resolver).

有关迁移，
see [Migrate outbound DNS rule items to domain resolver](/zh/migration/#migrate-outbound-dns-rule-items-to-domain-resolver).

**3**：

The new TLS fragment route options allow you to fragment TLS handshakes to bypass firewalls.

This feature is intended to circumvent simple firewalls based on **plaintext packet matching**, and should not be used
to circumvent real censorship.

Since it is not designed for performance, it should not be applied to all connections, but only to server names that are
known to be blocked.

参阅 [Route Action](/zh/configuration/route/rule_action/#tls_fragment).

**4**：

New certificate options allow you to manage the default list of trusted X509 CA certificates.

对于系统证书列表，修复了 Go 无法正确读取 Android 受信证书的问题。

You can also use the Mozilla Included List instead, or add trusted certificates yourself.

参阅 [Certificate](/zh/configuration/certificate/).

### 1.11.0

Important changes since 1.10:

* 引入 rule actions **1**
* 改进 tun compatibility **3**
* 将路由选项合并到路由动作 **4**
* 添加 `network_type`, `network_is_expensive` and `network_is_constrainted` rule items **5**
* 添加 multi network dialing **6**
* 添加 `cache_capacity` DNS option **7**
* 添加 `override_address` and `override_port` route options **8**
* 升级 WireGuard outbound to endpoint **9**
* 添加 UDP GSO support for WireGuard
* 使 GSO adaptive **10**
* 添加 UDP timeout route option **11**
* 添加 more masquerade options for hysteria2 **12**
* 添加 `rule-set merge` command
* 添加 port hopping support for Hysteria2 **13**
* Hysteria2 `ignore_client_bandwidth` behavior update **14**

**1**：

New rule actions replace legacy inbound fields and special outbound fields,
and can be used for pre-matching **2**.

参阅 [Rule](/zh/configuration/route/rule/),
[Rule Action](/zh/configuration/route/rule_action/),
[DNS Rule](/zh/configuration/dns/rule/) and
[DNS Rule Action](/zh/configuration/dns/rule_action/).

有关迁移， see
[Migrate legacy special outbounds to rule actions](/zh/migration/#migrate-legacy-special-outbounds-to-rule-actions),
[Migrate legacy inbound fields to rule actions](/zh/migration/#migrate-legacy-inbound-fields-to-rule-actions)
and [Migrate legacy DNS route options to rule actions](/zh/migration/#migrate-legacy-dns-route-options-to-rule-actions).

**2**：

Similar to Surge's pre-matching.

Specifically, new rule actions allow you to reject connections with
TCP RST (for TCP connections) and ICMP port unreachable (for UDP packets)
before connection established to improve tun's compatibility.

参阅 [Rule Action](/zh/configuration/route/rule_action/).

**3**：

When `gvisor` tun stack is enabled, even if the request passes routing,
if the outbound connection establishment fails,
the connection still does not need to be established and a TCP RST is replied.

**4**：

Route options in DNS route actions will no longer be considered deprecated,
see [DNS Route Action](/zh/configuration/dns/rule_action/).

Also, now `udp_disable_domain_unmapping` and `udp_connect` can also be configured in route action,
see [Route Action](/zh/configuration/route/rule_action/).

**5**：

When using in graphical clients, new routing rule items allow you to match on
network type (WIFI, cellular, etc.), whether the network is expensive, and whether Low Data Mode is enabled.

参阅 [Route Rule](/zh/configuration/route/rule/), [DNS Route Rule](/zh/configuration/dns/rule/)
and [Headless Rule](/zh/configuration/rule-set/headless-rule/).

**6**：

Similar to Surge's strategy.

New options allow you to connect using multiple network interfaces,
prefer or only use one type of interface,
and configure a timeout to fallback to other interfaces.

参阅 [Dial Fields](/zh/configuration/shared/dial/#network_strategy),
[Rule Action](/zh/configuration/route/rule_action/#network_strategy)
and [Route](/zh/configuration/route/#default_network_strategy).

**7**：

参阅 [DNS](/zh/configuration/dns/#cache_capacity).

**8**：

参阅 [Rule Action](/zh/configuration/route/#override_address) and
[Migrate destination override fields to route options](/zh/migration/#migrate-destination-override-fields-to-route-options).

**9**：

The new WireGuard endpoint combines inbound and outbound capabilities,
and the old outbound will be removed in sing-box 1.13.0.

参阅 [Endpoint](/zh/configuration/endpoint/), [WireGuard Endpoint](/zh/configuration/endpoint/wireguard/)
and [Migrate WireGuard outbound fields to route options](/zh/migration/#migrate-wireguard-outbound-to-endpoint).

**10**：

对于 WireGuard 出站和端点，GSO 将在可用时自动启用，
see [WireGuard Outbound](/zh/configuration/outbound/wireguard/#gso).

对于 TUN，GSO 已被移除，
see [Deprecated](/zh/deprecated/#gso-option-in-tun).

**11**：

参阅 [Rule Action](/zh/configuration/route/rule_action/#udp_timeout).

**12**：

参阅 [Hysteria2](/zh/configuration/inbound/hysteria2/#masquerade).

**13**：

参阅 [Hysteria2](/zh/configuration/outbound/hysteria2/).

**14**：

When `up_mbps` and `down_mbps` are set, `ignore_client_bandwidth` instead denies clients from using BBR CC.

### 1.10.7

* 修复与改进

#### 1.11.0-beta.20

* Hysteria2 `ignore_client_bandwidth` behavior update **1**
* 修复与改进

**1**：

When `up_mbps` and `down_mbps` are set, `ignore_client_bandwidth` instead denies clients from using BBR CC.

参阅 [Hysteria2](/zh/configuration/inbound/hysteria2/#ignore_client_bandwidth).

#### 1.11.0-beta.17

* 添加 port hopping support for Hysteria2 **1**
* 修复与改进

**1**：

参阅 [Hysteria2](/zh/configuration/outbound/hysteria2/).

#### 1.11.0-beta.14

* 允许 adding route (exclude) address sets to routes **1**
* 修复与改进

**1**：

When `auto_redirect` is not enabled, directly add `route[_exclude]_address_set`
to tun routes (equivalent to `route[_exclude]_address`).

Note that it **doesn't work on the Android graphical client** due to
the Android VpnService not being able to handle a large number of routes (DeadSystemException),
but otherwise it works fine on all command line clients and Apple platforms.

参阅 [route_address_set](/zh/configuration/inbound/tun/#route_address_set) and
[route_exclude_address_set](/zh/configuration/inbound/tun/#route_exclude_address_set).

#### 1.11.0-beta.12

* 添加 `rule-set merge` command
* 修复与改进

#### 1.11.0-beta.3

* 添加 more masquerade options for hysteria2 **1**
* 修复与改进

**1**：

参阅 [Hysteria2](/zh/configuration/inbound/hysteria2/#masquerade).

#### 1.11.0-alpha.25

* 更新 quic-go 至 v0.48.2
* 修复与改进

#### 1.11.0-alpha.22

* 添加 UDP timeout route option **1**
* 修复与改进

**1**：

参阅 [Rule Action](/zh/configuration/route/rule_action/#udp_timeout).

#### 1.11.0-alpha.20

* 添加 UDP GSO support for WireGuard
* 使 GSO adaptive **1**

**1**：

对于 WireGuard 出站和端点，GSO 将在可用时自动启用，
see [WireGuard Outbound](/zh/configuration/outbound/wireguard/#gso).

对于 TUN，GSO 已被移除，
see [Deprecated](/zh/deprecated/#gso-option-in-tun).

#### 1.11.0-alpha.19

* 升级 WireGuard outbound to endpoint **1**
* 修复与改进

**1**：

The new WireGuard endpoint combines inbound and outbound capabilities,
and the old outbound will be removed in sing-box 1.13.0.

参阅 [Endpoint](/zh/configuration/endpoint/), [WireGuard Endpoint](/zh/configuration/endpoint/wireguard/)
and [Migrate WireGuard outbound fields to route options](/zh/migration/#migrate-wireguard-outbound-to-endpoint).

### 1.10.2

* 添加 deprecated warnings
* 修复 proxying websocket connections in HTTP/mixed inbounds
* 修复与改进

#### 1.11.0-alpha.18

* 修复与改进

#### 1.11.0-alpha.16

* 添加 `cache_capacity` DNS option **1**
* 添加 `override_address` and `override_port` route options **2**
* 修复与改进

**1**：

参阅 [DNS](/zh/configuration/dns/#cache_capacity).

**2**：

参阅 [Rule Action](/zh/configuration/route/#override_address) and
[Migrate destination override fields to route options](/zh/migration/#migrate-destination-override-fields-to-route-options).

#### 1.11.0-alpha.15

* 改进 multi network dialing **1**
* 修复与改进

**1**：

New options allow you to configure the network strategy flexibly.

参阅 [Dial Fields](/zh/configuration/shared/dial/#network_strategy),
[Rule Action](/zh/configuration/route/rule_action/#network_strategy)
and [Route](/zh/configuration/route/#default_network_strategy).

#### 1.11.0-alpha.14

* 添加 multi network dialing **1**
* 修复与改进

**1**：

Similar to Surge's strategy.

New options allow you to connect using multiple network interfaces,
prefer or only use one type of interface,
and configure a timeout to fallback to other interfaces.

参阅 [Dial Fields](/zh/configuration/shared/dial/#network_strategy),
[Rule Action](/zh/configuration/route/rule_action/#network_strategy)
and [Route](/zh/configuration/route/#default_network_strategy).

#### 1.11.0-alpha.13

* 修复与改进

#### 1.11.0-alpha.12

* 将路由选项合并到路由动作 **1**
* 添加 `network_type`, `network_is_expensive` and `network_is_constrainted` rule items **2**
* 修复与改进

**1**：

Route options in DNS route actions will no longer be considered deprecated,
see [DNS Route Action](/zh/configuration/dns/rule_action/).

Also, now `udp_disable_domain_unmapping` and `udp_connect` can also be configured in route action,
see [Route Action](/zh/configuration/route/rule_action/).

**2**：

When using in graphical clients, new routing rule items allow you to match on
network type (WIFI, cellular, etc.), whether the network is expensive, and whether Low Data Mode is enabled.

参阅 [Route Rule](/zh/configuration/route/rule/), [DNS Route Rule](/zh/configuration/dns/rule/)
and [Headless Rule](/zh/configuration/rule-set/headless-rule/).

#### 1.11.0-alpha.9

* 改进 tun compatibility **1**
* 修复与改进

**1**：

When `gvisor` tun stack is enabled, even if the request passes routing,
if the outbound connection establishment fails,
the connection still does not need to be established and a TCP RST is replied.

#### 1.11.0-alpha.7

* 引入 rule actions **1**

**1**：

New rule actions replace legacy inbound fields and special outbound fields,
and can be used for pre-matching **2**.

参阅 [Rule](/zh/configuration/route/rule/),
[Rule Action](/zh/configuration/route/rule_action/),
[DNS Rule](/zh/configuration/dns/rule/) and
[DNS Rule Action](/zh/configuration/dns/rule_action/).

有关迁移， see
[Migrate legacy special outbounds to rule actions](/zh/migration/#migrate-legacy-special-outbounds-to-rule-actions),
[Migrate legacy inbound fields to rule actions](/zh/migration/#migrate-legacy-inbound-fields-to-rule-actions)
and [Migrate legacy DNS route options to rule actions](/zh/migration/#migrate-legacy-dns-route-options-to-rule-actions).

**2**：

Similar to Surge's pre-matching.

Specifically, new rule actions allow you to reject connections with
TCP RST (for TCP connections) and ICMP port unreachable (for UDP packets)
before connection established to improve tun's compatibility.

参阅 [Rule Action](/zh/configuration/route/rule_action/).

#### 1.11.0-alpha.6

* 更新 quic-go 至 v0.48.1
* 设置 gateway for tun correctly
* 修复与改进

#### 1.11.0-alpha.2

* 添加 warnings for usage of deprecated features
* 修复与改进

#### 1.11.0-alpha.1

* 更新 quic-go 至 v0.48.0
* 修复与改进

### 1.10.1

* 修复与改进

### 1.10.0

Important changes since 1.9:

* 引入 auto-redirect **1**
* 添加 AdGuard DNS Filter support **2**
* TUN address fields are merged **3**
* 添加 custom options for `auto-route` and `auto-redirect` **4**
* 放弃对 go1.18 和 go1.19 的支持 **5**
* 添加 tailing comma support in JSON configuration
* 改进 sniffers **6**
* 添加 new `inline` rule-set type **7**
* 添加 access control options for Clash API **8**
* 添加 `rule_set_ip_cidr_accept_empty` DNS address filter rule item **9**
* 添加 auto reload support for local rule-set
* 更新 fsnotify 用法 **10**
* 添加 IP address support for `rule-set match` command
* 添加 `rule-set decompile` command
* 添加 `process_path_regex` rule item
* 更新 uTLS 至 v1.6.7 **11**
* 优化规则集的内存使用 **12**

**1**：

The new auto-redirect feature allows TUN to automatically
configure connection redirection to improve proxy performance.

When auto-redirect is enabled, new route address set options will allow you to
automatically configure destination IP CIDR rules from a specified rule set to the firewall.

Specified or unspecified destinations will bypass the sing-box routes to get better performance
(for example, keep hardware offloading of direct traffics on the router).

参阅 [TUN](/zh/configuration/inbound/tun).

**2**：

The new feature allows you to use AdGuard DNS Filter lists in a sing-box without AdGuard Home.

参阅 [AdGuard DNS Filter](/zh/configuration/rule-set/adguard/).

**3**：

参阅 [Migration](/zh/migration/#tun-address-fields-are-merged).

**4**：

参阅 [iproute2_table_index](/zh/configuration/inbound/tun/#iproute2_table_index),
[iproute2_rule_index](/zh/configuration/inbound/tun/#iproute2_rule_index),
[auto_redirect_input_mark](/zh/configuration/inbound/tun/#auto_redirect_input_mark) and
[auto_redirect_output_mark](/zh/configuration/inbound/tun/#auto_redirect_output_mark).

**5**：

Due to maintenance difficulties, sing-box 1.10.0 requires at least Go 1.20 to compile.

**6**：

BitTorrent, DTLS, RDP, SSH sniffers are added.

Now the QUIC sniffer can correctly extract the server name from Chromium requests and
can identify common QUIC clients, including
Chromium, Safari, Firefox, quic-go (including uquic disguised as Chrome).

**7**：

The new [rule-set](/zh/configuration/rule-set/) type inline (which also becomes the default type)
allows you to write headless rules directly without creating a rule-set file.

**8**：

With new access control options, not only can you allow Clash dashboards
to access the Clash API on your local network,
you can also manually limit the websites that can access the API instead of allowing everyone.

参阅 [Clash API](/zh/configuration/experimental/clash-api/).

**9**：

参阅 [DNS Rule](/zh/configuration/dns/rule/#rule_set_ip_cidr_accept_empty).

**10**：

sing-box now uses fsnotify correctly and will not cancel watching
if the target file is deleted or recreated via rename (e.g. `mv`).

This affects all path options that support reload, including
`tls.certificate_path`, `tls.key_path`, `tls.ech.key_path` and `rule_set.path`.

**11**：

Some legacy chrome fingerprints have been removed and will fallback to chrome,
see [utls](/zh/configuration/shared/tls#utls).

**12**：

参阅 [Source Format](/zh/configuration/rule-set/source-format/#version).

### 1.9.7

* 修复与改进

#### 1.10.0-beta.11

* 更新 uTLS 至 v1.6.7 **1**

**1**：

Some legacy chrome fingerprints have been removed and will fallback to chrome,
see [utls](/zh/configuration/shared/tls#utls).

#### 1.10.0-beta.10

* 添加 `process_path_regex` rule item
* 修复与改进

_The macOS standalone versions of sing-box (>=1.9.5/<1.10.0-beta.11) now silently fail and require manual granting of
the **Full Disk Access** permission to system extension to start, probably due to Apple's changed security policy. We
will prompt users about this in feature versions._

### 1.9.6

* 修复与改进

### 1.9.5

* 更新 quic-go 至 v0.47.0
* 修复 direct dialer not resolving domain
* 修复 no error return when empty DNS cache retrieved
* 修复 build with go1.23
* 修复 stream sniffer
* 修复 bad redirect in clash-api
* 修复 wireguard events chan leak
* 修复 cached conn eats up read deadlines
* 修复 disconnected interface selected as default in windows
* 更新 Apple 平台客户端的 Bundle Identifier **1**

**1**：

参阅 [Migration](/zh/migration/#bundle-identifier-updates-in-apple-platform-clients).

We are still working on getting all sing-box apps back on the App Store, which should be completed within a week
(SFI on the App Store and others on TestFlight are already available).

#### 1.10.0-beta.8

* 修复与改进

_With the help of a netizen, we are in the process of getting sing-box apps back on the App Store, which should be
completed within a month (TestFlight is already available)._

#### 1.10.0-beta.7

* 更新 quic-go 至 v0.47.0
* 修复与改进

#### 1.10.0-beta.6

* 添加 RDP sniffer
* 修复与改进

#### 1.10.0-beta.5

* 添加 PNA support for [Clash API](/zh/configuration/experimental/clash-api/)
* 修复与改进

#### 1.10.0-beta.3

* 添加 SSH sniffer
* 修复与改进

#### 1.10.0-beta.2

* 使用 go1.23 构建
* 修复与改进

### 1.9.4

* 更新 quic-go 至 v0.46.0
* 更新 Hysteria2 BBR 拥塞控制
* Filter HTTPS ipv4hint/ipv6hint with domain strategy
* 修复 crash on Android when using process rules
* 修复 non-IP queries accepted by address filter rules
* 修复 UDP server for shadowsocks AEAD multi-user inbounds
* 修复 default next protos for v2ray QUIC transport
* 修复 default end value of port range configuration options
* 修复 reset v2ray transports
* 修复 panic caused by rule-set generation of duplicate keys for `domain_suffix`
* 修复 UDP connnection leak when sniffing
* 修复与改进

_Due to problems with our Apple developer account,
sing-box apps on Apple platforms are temporarily unavailable for download or update.
If your company or organization is willing to help us return to the App Store,
please [contact us](mailto:contact@sagernet.org)._

#### 1.10.0-alpha.29

* 更新 quic-go 至 v0.46.0
* 修复与改进

#### 1.10.0-alpha.25

* 添加 AdGuard DNS Filter support **1**

**1**：

The new feature allows you to use AdGuard DNS Filter lists in a sing-box without AdGuard Home.

参阅 [AdGuard DNS Filter](/zh/configuration/rule-set/adguard/).

#### 1.10.0-alpha.23

* 添加 Chromium support for QUIC sniffer
* 添加 client type detect support for QUIC sniffer **1**
* 修复与改进

**1**：

Now the QUIC sniffer can correctly extract the server name from Chromium requests and
can identify common QUIC clients, including
Chromium, Safari, Firefox, quic-go (including uquic disguised as Chrome).

参阅 [Protocol Sniff](/zh/configuration/route/sniff/) and [Route Rule](/zh/configuration/route/rule/#client).

#### 1.10.0-alpha.22

* 优化规则集的内存使用 **1**
* 修复与改进

**1**：

参阅 [Source Format](/zh/configuration/rule-set/source-format/#version).

#### 1.10.0-alpha.20

* 添加 DTLS sniffer
* 修复与改进

#### 1.10.0-alpha.19

* 添加 `rule-set decompile` command
* 添加 IP address support for `rule-set match` command
* 修复与改进

#### 1.10.0-alpha.18

* 添加 new `inline` rule-set type **1**
* 添加 auto reload support for local rule-set
* 更新 fsnotify 用法 **2**
* 修复与改进

**1**：

The new [rule-set](/zh/configuration/rule-set/) type inline (which also becomes the default type)
allows you to write headless rules directly without creating a rule-set file.

**2**：

sing-box now uses fsnotify correctly and will not cancel watching
if the target file is deleted or recreated via rename (e.g. `mv`).

This affects all path options that support reload, including
`tls.certificate_path`, `tls.key_path`, `tls.ech.key_path` and `rule_set.path`.

#### 1.10.0-alpha.17

* 一些混乱的更改 **1**
* `rule_set_ipcidr_match_source` rule items are renamed **2**
* 添加 `rule_set_ip_cidr_accept_empty` DNS address filter rule item **3**
* 更新 quic-go 至 v0.45.1
* 修复与改进

**1**：

Something may be broken, please actively report problems with this version.

**2**：

`rule_set_ipcidr_match_source` route and DNS rule items are renamed to
`rule_set_ip_cidr_match_source` and will be remove in sing-box 1.11.0.

**3**：

参阅 [DNS Rule](/zh/configuration/dns/rule/#rule_set_ip_cidr_accept_empty).

#### 1.10.0-alpha.16

* 添加 custom options for `auto-route` and `auto-redirect` **1**
* 修复与改进

**1**：

参阅 [iproute2_table_index](/zh/configuration/inbound/tun/#iproute2_table_index),
[iproute2_rule_index](/zh/configuration/inbound/tun/#iproute2_rule_index),
[auto_redirect_input_mark](/zh/configuration/inbound/tun/#auto_redirect_input_mark) and
[auto_redirect_output_mark](/zh/configuration/inbound/tun/#auto_redirect_output_mark).

#### 1.10.0-alpha.13

* TUN address fields are merged **1**
* 添加 route address set support for auto-redirect **2**

**1**：

参阅 [Migration](/zh/migration/#tun-address-fields-are-merged).

**2**：

The new feature will allow you to configure the destination IP CIDR rules
in the specified rule-sets to the firewall automatically.

Specified or unspecified destinations will bypass the sing-box routes to get better performance
(for example, keep hardware offloading of direct traffics on the router).

参阅 [route_address_set](/zh/configuration/inbound/tun/#route_address_set)
and [route_exclude_address_set](/zh/configuration/inbound/tun/#route_exclude_address_set).

#### 1.10.0-alpha.12

* 修复 auto-redirect not configuring nftables forward chain correctly
* 修复与改进

### 1.9.3

* 修复与改进

#### 1.10.0-alpha.10

* 修复与改进

### 1.9.2

* 修复与改进

#### 1.10.0-alpha.8

* 放弃对 go1.18 和 go1.19 的支持 **1**
* 更新 quic-go 至 v0.45.0
* 更新 Hysteria2 BBR 拥塞控制
* 修复与改进

**1**：

Due to maintenance difficulties, sing-box 1.10.0 requires at least Go 1.20 to compile.

### 1.9.1

* 修复与改进

#### 1.10.0-alpha.7

* 修复与改进

#### 1.10.0-alpha.5

* 改进 auto-redirect **1**

**1**：

nftables support and DNS hijacking has been added.

Tun inbounds with `auto_route` and `auto_redirect` now works as expected on routers **without intervention**.

#### 1.10.0-alpha.4

* 修复 auto-redirect **1**
* 改进 auto-route on linux **2**

**1**：

Tun inbounds with `auto_route` and `auto_redirect` now works as expected on routers.

**2**：

Tun inbounds with `auto_route` and `strict_route` now works as expected on routers and servers,
but the usages of [exclude_interface](/zh/configuration/inbound/tun/#exclude_interface) need to be updated.

#### 1.10.0-alpha.2

* 将 auto-redirect 移动到 Tun **1**
* 修复与改进

**1**：

Linux support are added.

参阅 [Tun](/zh/configuration/inbound/tun/#auto_redirect).

#### 1.10.0-alpha.1

* 添加 tailing comma support in JSON configuration
* 添加 simple auto-redirect for Android **1**
* 添加 BitTorrent sniffer **2**

**1**：

It allows you to use redirect inbound in the sing-box Android client
and automatically configures IPv4 TCP redirection via su.

This may alleviate the symptoms of some OCD patients who think that
redirect can effectively save power compared to the system HTTP Proxy.

参阅 [Redirect](/zh/configuration/inbound/redirect/).

**2**：

参阅 [Protocol Sniff](/zh/configuration/route/sniff/).

### 1.9.0

* 修复与改进

Important changes since 1.8:

* `domain_suffix` behavior update **1**
* `process_path` format update on Windows **2**
* 添加 address filter DNS rule items **3**
* 添加 support for `client-subnet` DNS options **4**
* 添加 rejected DNS response cache support **5**
* 添加 `bypass_domain` and `search_domain` platform HTTP proxy options **6**
* 修复 missing `rule_set_ipcidr_match_source` item in DNS rules **7**
* Handle Windows power events
* 如果 `dns.independent_cache` 被禁用，始终禁用 fake-ip DNS 传输的缓存
* 改进 DNS truncate behavior
* 更新 Hysteria 协议
* 更新 quic-go 至 v0.43.1
* 更新 gVisor 至 20240422.0
* Mitigating TunnelVision attacks **8**

**1**：

参阅 [Migration](/zh/migration/#domain_suffix-behavior-update).

**2**：

参阅 [Migration](/zh/migration/#process_path-format-update-on-windows).

**3**：

The new DNS feature allows you to more precisely bypass Chinese websites via **DNS leaks**. Do not use plain local DNS
if using this method.

参阅 [Address Filter Fields](/zh/configuration/dns/rule#address-filter-fields).

[Client example](/zh/manual/proxy/client#traffic-bypass-usage-for-chinese-users) updated.

**4**：

参阅 [DNS](/zh/configuration/dns), [DNS Server](/zh/configuration/dns/server) and [DNS Rules](/zh/configuration/dns/rule).

Since this feature makes the scenario mentioned in `alpha.1` no longer leak DNS requests,
the [Client example](/zh/manual/proxy/client#traffic-bypass-usage-for-chinese-users) has been updated.

**5**：

The new feature allows you to cache the check results of
[Address filter DNS rule items](/zh/configuration/dns/rule/#address-filter-fields) until expiration.

**6**：

参阅 [TUN](/zh/configuration/inbound/tun) inbound.

**7**：

参阅 [DNS Rule](/zh/configuration/dns/rule/).

**8**：

参阅 [TunnelVision](/zh/manual/misc/tunnelvision).

#### 1.9.0-rc.22

* 修复与改进

#### 1.9.0-rc.20

* Prioritize `*_route_address` in linux auto-route
* 修复 `*_route_address` in darwin auto-route

#### 1.8.14

* 修复 hysteria2 panic
* 修复与改进

#### 1.9.0-rc.18

* 添加 custom prefix support in EDNS0 client subnet options
* 修复 hysteria2 crash
* 修复 `store_rdrc` corrupted
* 更新 quic-go 至 v0.43.1
* 修复与改进

#### 1.9.0-rc.16

* Mitigating TunnelVision attacks **1**
* 修复与改进

**1**：

参阅 [TunnelVision](/zh/manual/misc/tunnelvision).

#### 1.9.0-rc.15

* 修复与改进

#### 1.8.13

* 修复 fake-ip mapping
* 修复与改进

#### 1.9.0-rc.14

* 修复与改进

#### 1.9.0-rc.13

* 更新 Hysteria 协议
* 更新 quic-go 至 v0.43.0
* 更新 gVisor 至 20240422.0
* 修复与改进

#### 1.8.12

* 我们现在有了官方的 APT 和 DNF 仓库 **1**
* 修复 packet MTU for QUIC protocols
* 修复与改进

**1**：

Including stable and beta versions, see https://sing-box.sagernet.org/installation/package-manager/

#### 1.9.0-rc.11

* 修复与改进

#### 1.8.11

* 修复与改进

#### 1.8.10

* 修复与改进

#### 1.9.0-beta.17

* 更新 `quic-go` 至 v0.42.0
* 修复与改进

#### 1.9.0-beta.16

* 修复与改进

_Our Testflight distribution has been temporarily blocked by Apple (possibly due to too many beta versions)
and you cannot join the test, install or update the sing-box beta app right now.
请耐心等待处理。_

#### 1.9.0-beta.14

* 更新 gVisor 至 20240212.0-65-g71212d503
* 修复与改进

#### 1.8.9

* 修复与改进

#### 1.8.8

* 修复与改进

#### 1.9.0-beta.7

* 修复与改进

#### 1.9.0-beta.6

* 修复 address filter DNS rule items **1**
* 修复 DNS outbound responding with wrong data
* 修复与改进

**1**：

Fixed an issue where address filter DNS rule was incorrectly rejected under certain circumstances.
If you have enabled `store_rdrc` to save results, consider clearing the cache file.

#### 1.8.7

* 修复与改进

#### 1.9.0-alpha.15

* 修复与改进

#### 1.9.0-alpha.14

* 改进 DNS truncate behavior
* 修复与改进

#### 1.9.0-alpha.13

* 修复与改进

#### 1.8.6

* 修复与改进

#### 1.9.0-alpha.12

* Handle Windows power events
* 如果 `dns.independent_cache` 被禁用，始终禁用 fake-ip DNS 传输的缓存
* 修复与改进

#### 1.9.0-alpha.11

* 修复 missing `rule_set_ipcidr_match_source` item in DNS rules **1**
* 修复与改进

**1**：

参阅 [DNS Rule](/zh/configuration/dns/rule/).

#### 1.9.0-alpha.10

* 添加 `bypass_domain` and `search_domain` platform HTTP proxy options **1**
* 修复与改进

**1**：

参阅 [TUN](/zh/configuration/inbound/tun) inbound.

#### 1.9.0-alpha.8

* 添加 rejected DNS response cache support **1**
* 修复与改进

**1**：

The new feature allows you to cache the check results of
[Address filter DNS rule items](/zh/configuration/dns/rule/#address-filter-fields) until expiration.

#### 1.9.0-alpha.7

* 更新 gVisor 至 20240206.0
* 修复与改进

#### 1.9.0-alpha.6

* 修复与改进

#### 1.9.0-alpha.3

* 更新 `quic-go` 至 v0.41.0
* 修复与改进

#### 1.9.0-alpha.2

* 添加 support for `client-subnet` DNS options **1**
* 修复与改进

**1**：

参阅 [DNS](/zh/configuration/dns), [DNS Server](/zh/configuration/dns/server) and [DNS Rules](/zh/configuration/dns/rule).

Since this feature makes the scenario mentioned in `alpha.1` no longer leak DNS requests,
the [Client example](/zh/manual/proxy/client#traffic-bypass-usage-for-chinese-users) has been updated.

#### 1.9.0-alpha.1

* `domain_suffix` behavior update **1**
* `process_path` format update on Windows **2**
* 添加 address filter DNS rule items **3**

**1**：

参阅 [Migration](/zh/migration/#domain_suffix-behavior-update).

**2**：

参阅 [Migration](/zh/migration/#process_path-format-update-on-windows).

**3**：

The new DNS feature allows you to more precisely bypass Chinese websites via **DNS leaks**. Do not use plain local DNS
if using this method.

参阅 [Address Filter Fields](/zh/configuration/dns/rule#address-filter-fields).

[Client example](/zh/manual/proxy/client#traffic-bypass-usage-for-chinese-users) updated.

#### 1.8.5

* 修复与改进

#### 1.8.4

* 修复与改进

#### 1.8.2

* 修复与改进

#### 1.8.1

* 修复与改进

### 1.8.0

* 修复与改进

Important changes since 1.7:

* 迁移 cache file from Clash API to independent options **1**
* 引入 [rule-set](/zh/configuration/rule-set/) **2**
* 添加 `sing-box geoip`, `sing-box geosite` and `sing-box rule-set` commands **3**
* 允许 nested logical rules **4**
* Independent `source_ip_is_private` and `ip_is_private` rules **5**
* 在 JSON 解码错误消息中添加上下文 **6**
* 拒绝内部 fake-ip 查询
* 添加 GSO support for TUN and WireGuard system interface **8**
* 为 URLTest 出站添加 `idle_timeout` **9**
* 添加 simple loopback detect
* 优化 memory usage of idle connections
* 更新 uTLS 至 1.5.4 **10**
* 更新依赖项 **11**

**1**：

参阅 [Cache File](/zh/configuration/experimental/cache-file/) and
[Migration](/zh/migration/#migrate-cache-file-from-clash-api-to-independent-options).

**2**：

rule-set is independent collections of rules that can be compiled into binaries to improve performance.
Compared to legacy GeoIP and Geosite resources,
it can include more types of rules, load faster,
use less memory, and update automatically.

参阅 [Route#rule_set](/zh/configuration/route/#rule_set),
[Route Rule](/zh/configuration/route/rule/),
[DNS Rule](/zh/configuration/dns/rule/),
[rule-set](/zh/configuration/rule-set/),
[Source Format](/zh/configuration/rule-set/source-format/) and
[Headless Rule](/zh/configuration/rule-set/headless-rule/).

有关 GEO 资源迁移，请参阅 [将 GeoIP 迁移到规则集](/zh/migration/#migrate-geoip-to-rule-sets) 和
[Migrate Geosite to rule-sets](/zh/migration/#migrate-geosite-to-rule-sets).

**3**：

New commands manage GeoIP, Geosite and rule-set resources, and help you migrate GEO resources to rule-sets.

**4**：

Logical rules in route rules, DNS rules, and the new headless rule now allow nesting of logical rules.

**5**：

The `private` GeoIP country never existed and was actually implemented inside V2Ray.
Since GeoIP was deprecated, we made this rule independent, see [Migration](/zh/migration/#migrate-geoip-to-rule-sets).

**6**：

JSON parse errors will now include the current key path.
Only takes effect when compiled with Go 1.21+.

**7**：

All internal DNS queries now skip DNS rules with `server` type `fakeip`,
and the default DNS server can no longer be `fakeip`.

This change is intended to break incorrect usage and essentially requires no action.

**8**：

参阅 [TUN](/zh/configuration/inbound/tun/) inbound and [WireGuard](/zh/configuration/outbound/wireguard/) outbound.

**9**：

When URLTest is idle for a certain period of time, the scheduled delay test will be paused.

**10**：

Added some new [fingerprints](/zh/configuration/shared/tls#utls).
Also, starting with this release, uTLS requires at least Go 1.20.

**11**：

Updated `cloudflare-tls`, `gomobile`, `smux`, `tfo-go` and `wireguard-go` to latest, `quic-go` to `0.40.1` and  `gvisor`
to `20231204.0`

#### 1.8.0-rc.11

* 修复与改进

#### 1.7.8

* 修复与改进

#### 1.8.0-rc.10

* 修复与改进

#### 1.7.7

* 修复 V2Ray transport `path` validation behavior **1**
* 修复与改进

**1**：

参阅 [V2Ray transport](/zh/configuration/shared/v2ray-transport/).

#### 1.8.0-rc.7

* 修复与改进

#### 1.8.0-rc.3

* 修复 V2Ray transport `path` validation behavior **1**
* 修复与改进

**1**：

参阅 [V2Ray transport](/zh/configuration/shared/v2ray-transport/).

#### 1.7.6

* 修复与改进

#### 1.8.0-rc.1

* 修复与改进

#### 1.8.0-beta.9

* 添加 simple loopback detect
* 修复与改进

#### 1.7.5

* 修复与改进

#### 1.8.0-alpha.17

* 添加 GSO support for TUN and WireGuard system interface **1**
* 更新 uTLS 至 1.5.4 **2**
* 更新依赖项 **3**
* 修复与改进

**1**：

参阅 [TUN](/zh/configuration/inbound/tun/) inbound and [WireGuard](/zh/configuration/outbound/wireguard/) outbound.

**2**：

Added some new [fingerprints](/zh/configuration/shared/tls#utls).
Also, starting with this release, uTLS requires at least Go 1.20.

**3**：

Updated `cloudflare-tls`, `gomobile`, `smux`, `tfo-go` and `wireguard-go` to latest, and `gvisor` to `20231204.0`

This may break something, good luck!

#### 1.7.4

* 修复与改进

_Due to the long waiting time, this version is no longer waiting for approval
by the Apple App Store, so updates to Apple Platforms will be delayed._

#### 1.8.0-alpha.16

* 修复与改进

#### 1.8.0-alpha.15

* 一些混乱的更改 **1**
* 修复与改进

**1**：

Designed to optimize memory usage of idle connections, may take effect on the following protocols:

| Protocol                                             | TCP              | UDP              |
|------------------------------------------------------|------------------|------------------|
| HTTP proxy server                                    | :material-check: | /                |
| SOCKS5                                               | :material-close: | :material-check: |
| Shadowsocks none/AEAD/AEAD2022                       | :material-check: | :material-check: |
| Trojan                                               | /                | :material-check: |
| TUIC/Hysteria/Hysteria2                              | :material-close: | :material-check: |
| Multiplex                                            | :material-close: | :material-check: |
| Plain TLS (Trojan/VLESS without extra sub-protocols) | :material-check: | /                |
| Other protocols                                      | :material-close: | :material-close: |

At the same time, everything existing may be broken, please actively report problems with this version.

#### 1.8.0-alpha.13

* 修复与改进

#### 1.8.0-alpha.10

* 为 URLTest 出站添加 `idle_timeout` **1**
* 修复与改进

**1**：

When URLTest is idle for a certain period of time, the scheduled delay test will be paused.

#### 1.7.2

* 修复与改进

#### 1.8.0-alpha.8

* 在 JSON 解码错误消息中添加上下文 **1**
* 拒绝内部 fake-ip 查询
* 修复与改进

**1**：

JSON parse errors will now include the current key path.
Only takes effect when compiled with Go 1.21+.

**2**：

All internal DNS queries now skip DNS rules with `server` type `fakeip`,
and the default DNS server can no longer be `fakeip`.

This change is intended to break incorrect usage and essentially requires no action.

#### 1.8.0-alpha.7

* 修复与改进

#### 1.7.1

* 修复与改进

#### 1.8.0-alpha.6

* 修复 rule-set matching logic **1**
* 修复与改进

**1**：

Now the rules in the `rule_set` rule item can be logically considered to be merged into the rule using rule-sets,
rather than completely following the AND logic.

#### 1.8.0-alpha.5

* 并行规则集初始化
* Independent `source_ip_is_private` and `ip_is_private` rules **1**

**1**：

The `private` GeoIP country never existed and was actually implemented inside V2Ray.
Since GeoIP was deprecated, we made this rule independent, see [Migration](/zh/migration/#migrate-geoip-to-rule-sets).

#### 1.8.0-alpha.1

* 迁移 cache file from Clash API to independent options **1**
* 引入 [rule-set](/zh/configuration/rule-set/) **2**
* 添加 `sing-box geoip`, `sing-box geosite` and `sing-box rule-set` commands **3**
* 允许 nested logical rules **4**

**1**：

参阅 [Cache File](/zh/configuration/experimental/cache-file/) and
[Migration](/zh/migration/#migrate-cache-file-from-clash-api-to-independent-options).

**2**：

rule-set is independent collections of rules that can be compiled into binaries to improve performance.
Compared to legacy GeoIP and Geosite resources,
it can include more types of rules, load faster,
use less memory, and update automatically.

参阅 [Route#rule_set](/zh/configuration/route/#rule_set),
[Route Rule](/zh/configuration/route/rule/),
[DNS Rule](/zh/configuration/dns/rule/),
[rule-set](/zh/configuration/rule-set/),
[Source Format](/zh/configuration/rule-set/source-format/) and
[Headless Rule](/zh/configuration/rule-set/headless-rule/).

有关 GEO 资源迁移，请参阅 [将 GeoIP 迁移到规则集](/zh/migration/#migrate-geoip-to-rule-sets) 和
[Migrate Geosite to rule-sets](/zh/migration/#migrate-geosite-to-rule-sets).

**3**：

New commands manage GeoIP, Geosite and rule-set resources, and help you migrate GEO resources to rule-sets.

**4**：

Logical rules in route rules, DNS rules, and the new headless rule now allow nesting of logical rules.

### 1.7.0

* 修复与改进

Important changes since 1.6:

* 添加 [exclude route support](/zh/configuration/inbound/tun/) for TUN inbound
* 添加 `udp_disable_domain_unmapping` [inbound listen option](/zh/configuration/shared/listen/) **1**
* 添加 [HTTPUpgrade V2Ray transport](/zh/configuration/shared/v2ray-transport#HTTPUpgrade) support **2**
* 迁移 multiplex and UoT server to inbound **3**
* 添加 TCP Brutal support for multiplex **4**
* 添加 `wifi_ssid` and `wifi_bssid` route and DNS rules **5**
* 更新 quic-go 至 v0.40.0
* 更新 gVisor 至 20231113.0

**1**：

If enabled, for UDP proxy requests addressed to a domain,
the original packet address will be sent in the response instead of the mapped domain.

This option is used for compatibility with clients that
do not support receiving UDP packets with domain addresses, such as Surge.

**2**：

Introduced in V2Ray 5.10.0.

The new HTTPUpgrade transport has better performance than WebSocket and is better suited for CDN abuse.

**3**：

Starting in 1.7.0, multiplexing support is no longer enabled by default
and needs to be turned on explicitly in inbound
options.

**4**

Hysteria Brutal Congestion Control Algorithm in TCP. A kernel module needs to be installed on the Linux server,
see [TCP Brutal](/zh/configuration/shared/tcp-brutal/) for details.

**5**：

Only supported in graphical clients on Android and Apple platforms.

#### 1.7.0-rc.3

* 修复与改进

#### 1.6.7

* macOS: Add button for uninstall SystemExtension in the standalone graphical client
* 修复 missing UDP user context on TUIC/Hysteria2 inbounds
* 修复与改进

#### 1.7.0-rc.2

* 修复 missing UDP user context on TUIC/Hysteria2 inbounds
* macOS: Add button for uninstall SystemExtension in the standalone graphical client

#### 1.6.6

* 修复与改进

#### 1.7.0-rc.1

* 修复与改进

#### 1.7.0-beta.5

* 更新 gVisor 至 20231113.0
* 修复与改进

#### 1.7.0-beta.4

* 添加 `wifi_ssid` and `wifi_bssid` route and DNS rules **1**
* 修复与改进

**1**：

Only supported in graphical clients on Android and Apple platforms.

#### 1.7.0-beta.3

* 修复 zero TTL was incorrectly reset
* 修复与改进

#### 1.6.5

* 修复 crash if TUIC inbound authentication failed
* 修复与改进

#### 1.7.0-beta.2

* 修复 crash if TUIC inbound authentication failed
* 更新 quic-go 至 v0.40.0
* 修复与改进

#### 1.6.4

* 修复与改进

#### 1.7.0-beta.1

* 修复与改进

#### 1.6.3

* iOS/Android: Fix profile auto update
* 修复与改进

#### 1.7.0-alpha.11

* iOS/Android: Fix profile auto update
* 修复与改进

#### 1.7.0-alpha.10

* 修复 tcp-brutal not working with TLS
* 修复 Android client not closing in some cases
* 修复与改进

#### 1.6.2

* 修复与改进

#### 1.6.1

* Our [Android client](/zh/installation/clients/sfa/) is now available in the Google Play Store ▶️
* 修复与改进

#### 1.7.0-alpha.6

* 修复与改进

#### 1.7.0-alpha.4

* 迁移 multiplex and UoT server to inbound **1**
* 添加 TCP Brutal support for multiplex **2**

**1**：

Starting in 1.7.0, multiplexing support is no longer enabled by default and needs to be turned on explicitly in inbound
options.

**2**

Hysteria Brutal Congestion Control Algorithm in TCP. A kernel module needs to be installed on the Linux server,
see [TCP Brutal](/zh/configuration/shared/tcp-brutal/) for details.

#### 1.7.0-alpha.3

* 添加 [HTTPUpgrade V2Ray transport](/zh/configuration/shared/v2ray-transport#HTTPUpgrade) support **1**
* 修复与改进

**1**：

Introduced in V2Ray 5.10.0.

The new HTTPUpgrade transport has better performance than WebSocket and is better suited for CDN abuse.

### 1.6.0

* 修复与改进

Important changes since 1.5:

* Our [Apple tvOS client](/zh/installation/clients/sft/) is now available in the App Store 🍎
* 更新 TUIC 和 Hysteria2 的 BBR 拥塞控制 **1**
* 更新 Hysteria2 的 brutal 拥塞控制
* 添加 `brutal_debug` option for Hysteria2
* 更新旧版 Hysteria 协议 **2**
* 添加 TLS self sign key pair generate command
* 移除 [Deprecated Features](/zh/deprecated/) by agreement

**1**：

None of the existing Golang BBR congestion control implementations have been reviewed or unit tested.
This update is intended to address the multi-send defects of the old implementation and may introduce new issues.

**2**

Based on discussions with the original author, the brutal CC and QUIC protocol parameters of
the old protocol (Hysteria 1) have been updated to be consistent with Hysteria 2

#### 1.7.0-alpha.2

* 修复 bugs introduced in 1.7.0-alpha.1

#### 1.7.0-alpha.1

* 添加 [exclude route support](/zh/configuration/inbound/tun/) for TUN inbound
* 添加 `udp_disable_domain_unmapping` [inbound listen option](/zh/configuration/shared/listen/) **1**
* 修复与改进

**1**：

If enabled, for UDP proxy requests addressed to a domain,
the original packet address will be sent in the response instead of the mapped domain.

This option is used for compatibility with clients that
do not support receiving UDP packets with domain addresses, such as Surge.

#### 1.5.5

* 修复 IPv6 `auto_route` for Linux **1**
* 添加 legacy builds for old Windows and macOS systems **2**
* 修复与改进

**1**：

When `auto_route` is enabled and `strict_route` is disabled, the device can now be reached from external IPv6 addresses.

**2**：

Built using Go 1.20, the last version that will run on
Windows 7, 8, Server 2008, Server 2012 and macOS 10.13 High
Sierra, 10.14 Mojave.

#### 1.6.0-rc.4

* 修复与改进

#### 1.6.0-rc.1

* 添加 legacy builds for old Windows and macOS systems **1**
* 修复与改进

**1**：

Built using Go 1.20, the last version that will run on
Windows 7, 8, Server 2008, Server 2012 and macOS 10.13 High
Sierra, 10.14 Mojave.

#### 1.6.0-beta.4

* 修复 IPv6 `auto_route` for Linux **1**
* 修复与改进

**1**：

When `auto_route` is enabled and `strict_route` is disabled, the device can now be reached from external IPv6 addresses.

#### 1.5.4

* 修复 Clash cache crash on arm32 devices
* 修复与改进

#### 1.6.0-beta.3

* 更新旧版 Hysteria 协议 **1**
* 修复与改进

**1**

Based on discussions with the original author, the brutal CC and QUIC protocol parameters of
the old protocol (Hysteria 1) have been updated to be consistent with Hysteria 2

#### 1.6.0-beta.2

* 添加 TLS self sign key pair generate command
* 更新 Hysteria2 的 brutal 拥塞控制
* 修复 Clash cache crash on arm32 devices
* 更新 golang.org/x/net 至 v0.17.0
* 修复与改进

#### 1.6.0-beta.3

* 更新旧版 Hysteria 协议 **1**
* 修复与改进

**1**

Based on discussions with the original author, the brutal CC and QUIC protocol parameters of
the old protocol (Hysteria 1) have been updated to be consistent with Hysteria 2

#### 1.6.0-beta.2

* 添加 TLS self sign key pair generate command
* 更新 Hysteria2 的 brutal 拥塞控制
* 修复 Clash cache crash on arm32 devices
* 更新 golang.org/x/net 至 v0.17.0
* 修复与改进

#### 1.5.3

* 修复 compatibility with Android 14
* 修复与改进

#### 1.6.0-beta.1

* 修复与改进

#### 1.6.0-alpha.5

* 修复 compatibility with Android 14
* 更新 TUIC 和 Hysteria2 的 BBR 拥塞控制 **1**
* 修复与改进

**1**：

None of the existing Golang BBR congestion control implementations have been reviewed or unit tested.
This update is intended to fix a memory leak flaw in the new implementation introduced in 1.6.0-alpha.1 and may
introduce new issues.

#### 1.6.0-alpha.4

* 添加 `brutal_debug` option for Hysteria2
* 修复与改进

#### 1.5.2

* Our [Apple tvOS client](/zh/installation/clients/sft/) is now available in the App Store 🍎
* 修复与改进

#### 1.6.0-alpha.3

* 修复与改进

#### 1.6.0-alpha.2

* 修复与改进

#### 1.5.1

* 修复与改进

#### 1.6.0-alpha.1

* 更新 TUIC 和 Hysteria2 的 BBR 拥塞控制 **1**
* 更新 quic-go 至 v0.39.0
* 更新 gVisor 至 20230814.0
* 移除 [Deprecated Features](/zh/deprecated/) by agreement
* 修复与改进

**1**：

None of the existing Golang BBR congestion control implementations have been reviewed or unit tested.
This update is intended to address the multi-send defects of the old implementation and may introduce new issues.

### 1.5.0

* 修复与改进

Important changes since 1.4:

* 添加 TLS [ECH server](/zh/configuration/shared/tls/) support
* 改进 TLS TCH client configuration
* 添加 TLS ECH key pair generator **1**
* 添加 TLS ECH support for QUIC based protocols **2**
* 添加 KDE support for the `set_system_proxy` option in HTTP inbound
* 添加 Hysteria2 protocol support **3**
* 添加 `interrupt_exist_connections` option for `Selector` and `URLTest` outbounds **4**
* 添加 DNS01 challenge support for ACME TLS certificate issuer **5**
* 添加 `merge` command **6**
* Mark [Deprecated Features](/zh/deprecated/)

**1**：

Command: `sing-box generate ech-keypair <plain_server_name> [--pq-signature-schemes-enabled]`

**2**：

All inbounds and outbounds are supported, including `Naiveproxy`, `Hysteria[/2]`, `TUIC` and `V2ray QUIC transport`.

**3**：

参阅 [Hysteria2 inbound](/zh/configuration/inbound/hysteria2/) and [Hysteria2 outbound](/zh/configuration/outbound/hysteria2/)

有关协议说明，请参阅 [https://v2.hysteria.network](https://v2.hysteria.network)

**4**：

Interrupt existing connections when the selected outbound has changed.

Only inbound connections are affected by this setting, internal connections will always be interrupted.

**5**：

Only `Alibaba Cloud DNS` and `Cloudflare` are supported, see [ACME Fields](/zh/configuration/shared/tls#acme-fields)
and [DNS01 Challenge Fields](/zh/configuration/shared/dns01_challenge/).

**6**：

This command also parses path resources that appear in the configuration file and replaces them with embedded
configuration, such as TLS certificates or SSH private keys.

#### 1.5.0-rc.6

* 修复与改进

#### 1.4.6

* 修复与改进

#### 1.5.0-rc.5

* 修复 an improper authentication vulnerability in the SOCKS5 inbound
* 修复与改进

**Security Advisory**

This update fixes an improper authentication vulnerability in the sing-box SOCKS inbound. This vulnerability allows an
attacker to craft special requests to bypass user authentication. All users exposing SOCKS servers with user
authentication in an insecure environment are advised to update immediately.

此更新修复了 sing-box SOCKS 入站中的一个不正确身份验证漏洞。 该漏洞允许攻击者制作特殊请求来绕过用户身份验证。建议所有将使用用户认证的
SOCKS 服务器暴露在不安全环境下的用户立更新。

#### 1.4.5

* 修复 an improper authentication vulnerability in the SOCKS5 inbound
* 修复与改进

**Security Advisory**

This update fixes an improper authentication vulnerability in the sing-box SOCKS inbound. This vulnerability allows an
attacker to craft special requests to bypass user authentication. All users exposing SOCKS servers with user
authentication in an insecure environment are advised to update immediately.

此更新修复了 sing-box SOCKS 入站中的一个不正确身份验证漏洞。 该漏洞允许攻击者制作特殊请求来绕过用户身份验证。建议所有将使用用户认证的
SOCKS 服务器暴露在不安全环境下的用户立更新。

#### 1.5.0-rc.3

* 修复与改进

#### 1.5.0-beta.12

* 添加 `merge` command **1**
* 修复与改进

**1**：

This command also parses path resources that appear in the configuration file and replaces them with embedded
configuration, such as TLS certificates or SSH private keys.

```
Merge configurations

Usage:
  sing-box merge [output] [flags]

Flags:
  -h, --help   help for merge

Global Flags:
  -c, --config stringArray             set configuration file path
  -C, --config-directory stringArray   set configuration directory path
  -D, --directory string               set working directory
      --disable-color                  disable color output
```

#### 1.5.0-beta.11

* 添加 DNS01 challenge support for ACME TLS certificate issuer **1**
* 修复与改进

**1**：

Only `Alibaba Cloud DNS` and `Cloudflare` are supported,
see [ACME Fields](/zh/configuration/shared/tls#acme-fields)
and [DNS01 Challenge Fields](/zh/configuration/shared/dns01_challenge/).

#### 1.5.0-beta.10

* 添加 `interrupt_exist_connections` option for `Selector` and `URLTest` outbounds **1**
* 修复与改进

**1**：

Interrupt existing connections when the selected outbound has changed.

Only inbound connections are affected by this setting, internal connections will always be interrupted.

#### 1.4.3

* 修复与改进

#### 1.5.0-beta.8

* 修复与改进

#### 1.4.2

* 修复与改进

#### 1.5.0-beta.6

* 修复 compatibility issues with official Hysteria2 server and client
* 修复与改进
* Mark [deprecated features](/zh/deprecated/)

#### 1.5.0-beta.3

* 修复与改进
* Updated Hysteria2 documentation **1**

**1**：

Added notes indicating compatibility issues with the official
Hysteria2 server and client when using `fastOpen=false` or UDP MTU >= 1200.

#### 1.5.0-beta.2

* 添加 hysteria2 protocol support **1**
* 修复与改进

**1**：

参阅 [Hysteria2 inbound](/zh/configuration/inbound/hysteria2/) and [Hysteria2 outbound](/zh/configuration/outbound/hysteria2/)

有关协议说明，请参阅 [https://v2.hysteria.network](https://v2.hysteria.network)

#### 1.5.0-beta.1

* 添加 TLS [ECH server](/zh/configuration/shared/tls/) support
* 改进 TLS TCH client configuration
* 添加 TLS ECH key pair generator **1**
* 添加 TLS ECH support for QUIC based protocols **2**
* 添加 KDE support for the `set_system_proxy` option in HTTP inbound

**1**：

Command: `sing-box generate ech-keypair <plain_server_name> [--pq-signature-schemes-enabled]`

**2**：

All inbounds and outbounds are supported, including `Naiveproxy`, `Hysteria`, `TUIC` and `V2ray QUIC transport`.

#### 1.4.1

* 修复与改进

### 1.4.0

* 修复 bugs and update dependencies

Important changes since 1.3:

* 添加 TUIC support **1**
* 添加 `udp_over_stream` option for TUIC client **2**
* 添加 MultiPath TCP support **3**
* 添加 `include_interface` and `exclude_interface` options for tun inbound
* 当没有网络或设备空闲时暂停定期任务
* 改进 Android and Apple platform clients

*1*:

参阅 [TUIC inbound](/zh/configuration/inbound/tuic/)
and [TUIC outbound](/zh/configuration/outbound/tuic/)

**2**：

This is the TUIC port of the [UDP over TCP protocol](/zh/configuration/shared/udp-over-tcp/), designed to provide a QUIC
stream based UDP relay mode that TUIC does not provide. Since it is an add-on protocol, you will need to use sing-box or
another program compatible with the protocol as a server.

This mode has no positive effect in a proper UDP proxy scenario and should only be applied to relay streaming UDP
traffic (basically QUIC streams).

*3*:

Requires sing-box to be compiled with Go 1.21.

#### 1.4.0-rc.3

* 修复与改进

#### 1.4.0-rc.2

* 修复与改进

#### 1.4.0-rc.1

* 修复 TUIC UDP

#### 1.4.0-beta.6

* 添加 `udp_over_stream` option for TUIC client **1**
* 添加 `include_interface` and `exclude_interface` options for tun inbound
* 修复与改进

**1**：

This is the TUIC port of the [UDP over TCP protocol](/zh/configuration/shared/udp-over-tcp/), designed to provide a QUIC
stream based UDP relay mode that TUIC does not provide. Since it is an add-on protocol, you will need to use sing-box or
another program compatible with the protocol as a server.

This mode has no positive effect in a proper UDP proxy scenario and should only be applied to relay streaming UDP
traffic (basically QUIC streams).

#### 1.4.0-beta.5

* 修复与改进

#### 1.4.0-beta.4

* 图形客户端：持久化组展开状态
* 修复与改进

#### 1.4.0-beta.3

* 修复与改进

#### 1.4.0-beta.2

* 添加 MultiPath TCP support **1**
* 由于上游变更，放弃对 Go 1.18 和 1.19 的 QUIC 支持
* 修复与改进

*1*:

Requires sing-box to be compiled with Go 1.21.

#### 1.4.0-beta.1

* 添加 TUIC support **1**
* 当没有网络或设备空闲时暂停定期任务
* 修复与改进

*1*:

参阅 [TUIC inbound](/zh/configuration/inbound/tuic/)
and [TUIC outbound](/zh/configuration/outbound/tuic/)

#### 1.3.6

* 修复与改进

#### 1.3.5

* 修复与改进
* 引入 our [Apple tvOS](/zh/installation/clients/sft/) client applications **1**
* 添加 per app proxy and app installed/updated trigger support for Android client
* 添加 profile sharing support for Android/iOS/macOS clients

**1**：

Due to the requirement of tvOS 17, the app cannot be submitted to the App Store for the time being, and can only be
downloaded through TestFlight.

#### 1.3.4

* 修复与改进
* We're now on the [App Store](https://apps.apple.com/us/app/sing-box/id6451272673), always free! It should be noted
  that due to stricter and slower review, the release of Store versions will be delayed.
* We've made a standalone version of the macOS client (the original Application Extension relies on App Store
  distribution), which you can download as SFM-version-universal.zip in the release artifacts.

#### 1.3.3

* 修复与改进

#### 1.3.1-rc.1

* 修复 bugs and update dependencies

#### 1.3.1-beta.3

* 引入 our [new iOS](/zh/installation/clients/sfi/) and [macOS](/zh/installation/clients/sfm/) client applications **1
  **
* 修复与改进

**1**：

The old testflight link and app are no longer valid.

#### 1.3.1-beta.2

* 修复 bugs and update dependencies

#### 1.3.1-beta.1

* 修复与改进

### 1.3.0

* 修复 bugs and update dependencies

Important changes since 1.2:

* 添加 [FakeIP](/zh/configuration/dns/fakeip/) support **1**
* 改进 multiplex **2**
* 添加 [DNS reverse mapping](/zh/configuration/dns#reverse_mapping) support
* 添加 `rewrite_ttl` DNS rule action
* 添加 `store_fakeip` Clash API option
* 添加 multi-peer support for [WireGuard](/zh/configuration/outbound/wireguard#peers) outbound
* 添加 loopback detect
* 添加 Clash.Meta API compatibility for Clash API
* 下载 Yacd-meta by default if the specified Clash `external_ui` directory is empty
* 添加 path and headers option for HTTP outbound
* Perform URLTest recheck after network changes
* 修复 `system` tun stack for ios
* 修复 network monitor for android/ios
* 更新 VLESS 和 XUDP 协议
* 使 splice work with traffic statistics systems like Clash API
* 显著降低空闲连接的内存使用
* 改进 DNS caching
* 添加 `independent_cache` [option](/zh/configuration/dns#independent_cache) for DNS
* 重实现 shadowsocks 客户端
* 添加 multiplex support for VLESS outbound
* 自动添加入站规则以使系统 tun 栈工作
* 修复 TLS 1.2 support for shadow-tls client
* 添加 `cache_id` [option](/zh/configuration/experimental#cache_id) for Clash cache file
* 修复 `local` DNS transport for Android

*1*:

参阅 [FAQ](/zh/faq/fakeip/) for more information.

*2*:

Added new `h2mux` multiplex protocol and `padding` multiplex option, see [Multiplex](/zh/configuration/shared/multiplex/).

#### 1.3-rc2

* 修复 `local` DNS transport for Android
* 修复 bugs and update dependencies

#### 1.3-rc1

* 修复 bugs and update dependencies

#### 1.3-beta14

* 修复与改进

#### 1.3-beta13

* 修复 resolving fakeip domains  **1**
* Deprecate L3 routing
* 修复 bugs and update dependencies

**1**：

If the destination address of the connection is obtained from fakeip, dns rules with server type fakeip will be skipped.

#### 1.3-beta12

* 自动添加入站规则以使系统 tun 栈工作
* 修复 TLS 1.2 support for shadow-tls client
* 添加 `cache_id` [option](/zh/configuration/experimental#cache_id) for Clash cache file
* 修复与改进

#### 1.3-beta11

* 修复 bugs and update dependencies

#### 1.3-beta10

* 改进 direct copy **1**
* 改进 DNS caching
* 添加 `independent_cache` [option](/zh/configuration/dns#independent_cache) for DNS
* 重实现 shadowsocks 客户端
* 添加 multiplex support for VLESS outbound
* 设置 TCP keepalive for WireGuard gVisor TCP connections
* 修复与改进

**1**：

* 使 splice work with traffic statistics systems like Clash API
* 显著降低空闲连接的内存使用

**2**：

Improved performance and reduced memory usage.

#### 1.3-beta9

* 改进 multiplex **1**
* 修复与改进

*1*:

Added new `h2mux` multiplex protocol and `padding` multiplex option, see [Multiplex](/zh/configuration/shared/multiplex/).

#### 1.2.6

* 修复 bugs and update dependencies

#### 1.3-beta8

* 修复 `system` tun stack for ios
* 修复 network monitor for android/ios
* 更新 VLESS 和 XUDP 协议 **1**
* 修复与改进

*1:

This is an incompatible update for XUDP in VLESS if vision flow is enabled.

#### 1.3-beta7

* 添加 `path` and `headers` options for HTTP outbound
* 添加 multi-user support for Shadowsocks legacy AEAD inbound
* 修复与改进

#### 1.2.4

* 修复与改进

#### 1.3-beta6

* 修复 WireGuard reconnect
* Perform URLTest recheck after network changes
* 修复 bugs and update dependencies

#### 1.3-beta5

* 添加 Clash.Meta API compatibility for Clash API
* 下载 Yacd-meta by default if the specified Clash `external_ui` directory is empty
* 添加 path and headers option for HTTP outbound
* 修复与改进

#### 1.3-beta4

* 修复 bugs

#### 1.3-beta2

* 如果指定的 Clash `external_ui` 目录为空，则下载 clash-dashboard
* 修复 bugs and update dependencies

#### 1.3-beta1

* 添加 [DNS reverse mapping](/zh/configuration/dns#reverse_mapping) support
* 添加 [L3 routing](/zh/configuration/route/ip-rule/) support **1**
* 添加 `rewrite_ttl` DNS rule action
* 添加 [FakeIP](/zh/configuration/dns/fakeip/) support **2**
* 添加 `store_fakeip` Clash API option
* 添加 multi-peer support for [WireGuard](/zh/configuration/outbound/wireguard#peers) outbound
* 添加 loopback detect

*1*:

It can currently be used to [route connections directly to WireGuard](/zh/examples/wireguard-direct/) or block connections
at the IP layer.

*2*:

参阅 [FAQ](/zh/faq/fakeip/) for more information.

#### 1.2.3

* 引入 our [new Android client application](/zh/installation/clients/sfa/)
* 改进 UDP domain destination NAT
* 更新 reality 协议
* 修复 TTL calculation for DNS response
* 修复 v2ray HTTP transport compatibility
* 修复 bugs and update dependencies

#### 1.2.2

* Accept `any` outbound in dns rule **1**
* 修复 bugs and update dependencies

*1*:

Now you can use the `any` outbound rule to match server address queries instead of filling in all server domains
to `domain` rule.

#### 1.2.1

* 修复 missing default host in v2ray http transport`s request
* Flush DNS cache for macOS when tun start/close
* 修复 tun's DNS hijacking compatibility with systemd-resolved

### 1.2.0

* 修复 bugs and update dependencies

Important changes since 1.1:

* 引入 our [new iOS client application](/zh/installation/clients/sfi/)
* 引入 [UDP over TCP protocol version 2](/zh/configuration/shared/udp-over-tcp/)
* 添加 [platform options](/zh/configuration/inbound/tun#platform) for tun inbound
* 添加 [ShadowTLS protocol v3](https://github.com/ihciah/shadow-tls/blob/master/docs/protocol-v3-en.md)
* 添加 [VLESS server](/zh/configuration/inbound/vless/) and [vision](/zh/configuration/outbound/vless#flow) support
* 添加 [reality TLS](/zh/configuration/shared/tls/) support
* 添加 [NTP service](/zh/configuration/ntp/)
* 添加 [DHCP DNS server](/zh/configuration/dns/server/) support
* 添加 SSH [host key validation](/zh/configuration/outbound/ssh/) support
* 添加 [query_type](/zh/configuration/dns/rule/) DNS rule item
* 添加 fallback support for v2ray transport
* 添加 custom TLS server support for http based v2ray transports
* 添加 health check support for http-based v2ray transports
* 添加 multiple configuration support

#### 1.2-rc1

* 修复 bugs and update dependencies

#### 1.2-beta10

* 添加 multiple configuration support **1**
* 修复 bugs and update dependencies

*1*:

Now you can pass the parameter `--config` or `-c` multiple times, or use the new parameter `--config-directory` or `-C`
to load all configuration files in a directory.

Loaded configuration files are sorted by name. If you want to control the merge order, add a numeric prefix to the file
name.

#### 1.1.7

* 改进 the stability of the VMESS server
* 修复 `auto_detect_interface` incorrectly identifying the default interface on Windows
* 修复 bugs and update dependencies

#### 1.2-beta9

* 引入 the [UDP over TCP protocol version 2](/zh/configuration/shared/udp-over-tcp/)
* 添加 health check support for http-based v2ray transports
* 移除 length limit on short_id for reality TLS config
* 修复 bugs and update dependencies

#### 1.2-beta8

* 更新 reality 和 uTLS 库
* 修复 `auto_detect_interface` incorrectly identifying the default interface on Windows

#### 1.2-beta7

* 修复 the compatibility issue between VLESS's vision sub-protocol and the Xray-core client
* 改进 the stability of the VMESS server

#### 1.2-beta6

* 引入 our [new iOS client application](/zh/installation/clients/sfi/)
* 添加 [platform options](/zh/configuration/inbound/tun#platform) for tun inbound
* 添加 custom TLS server support for http based v2ray transports
* 添加 generate commands
* 启用 XUDP by default in VLESS
* 更新 reality 服务器
* 更新 vision 协议
* 修复 [user flow in vless server](/zh/configuration/inbound/vless#usersflow)
* Bug 修复
* 更新依赖项

#### 1.2-beta5

* 添加 [VLESS server](/zh/configuration/inbound/vless/) and [vision](/zh/configuration/outbound/vless#flow) support
* 添加 [reality TLS](/zh/configuration/shared/tls/) support
* 修复 match private address

#### 1.1.6

* 改进 vmess request
* 修复 ipv6 redirect on Linux
* 修复 match geoip private
* 修复 parse hysteria UDP message
* 修复 socks connect response
* 如果启用传输则禁用 vmess 头保护
* 更新 QUIC v2 版本号和初始盐

#### 1.2-beta4

* 添加 [NTP service](/zh/configuration/ntp/)
* 添加 Add multiple server names and multi-user support for shadowtls
* 添加 strict mode support for shadowtls v3
* 添加 uTLS support for shadowtls v3

#### 1.2-beta3

* 更新 QUIC v2 版本号和初始盐
* 修复 shadowtls v3 implementation

#### 1.2-beta2

* 添加 [ShadowTLS protocol v3](https://github.com/ihciah/shadow-tls/blob/master/docs/protocol-v3-en.md)
* 添加 fallback support for v2ray transport
* 修复 parse hysteria UDP message
* 修复 socks connect response
* 如果启用传输则禁用 vmess 头保护

#### 1.2-beta1

* 添加 [DHCP DNS server](/zh/configuration/dns/server/) support
* 添加 SSH [host key validation](/zh/configuration/outbound/ssh/) support
* 添加 [query_type](/zh/configuration/dns/rule/) DNS rule item
* 添加 v2ray [user stats](/zh/configuration/experimental#statsusers) api
* 添加 new clash DNS query api
* 改进 vmess request
* 修复 ipv6 redirect on Linux
* 修复 match geoip private

#### 1.1.5

* 添加 Go 1.20 support
* 修复 inbound default DF value
* 修复 auth_user route for naive inbound
* 修复 gRPC lite header
* 在路由规则中忽略域名大小写

#### 1.1.4

* 修复 DNS log
* 修复 write to h2 conn after closed
* 修复 create UDP DNS transport from plain IPv6 address

#### 1.1.2

* 修复 http proxy auth
* 修复 user from stream packet conn
* 修复 DNS response TTL
* 修复 override packet conn
* 跳过覆盖系统代理绕过列表
* 改进 DNS log

#### 1.1.1

* 修复 acme config
* 修复 vmess packet conn
* 抑制 quic-go 设置 DF 错误

#### 1.1

* 修复 close clash cache

Important changes since 1.0:

* 添加 support for use with android VPNService
* 添加 tun support for WireGuard outbound
* 添加 system tun stack
* 添加 comment filter for config
* 添加 option for allow optional proxy protocol header
* 添加 Clash mode and persistence support
* 添加 TLS ECH and uTLS support for outbound TLS options
* 添加 internal simple-obfs and v2ray-plugin
* 添加 ShadowsocksR outbound
* 添加 VLESS outbound and XUDP
* 跳过等待 hysteria tcp 握手响应
* 添加 v2ray mux support for all inbound
* 添加 XUDP support for VMess
* 改进 websocket writer
* 完善 tproxy 写回
* 修复 DNS leak caused by
  Windows' ordinary multihomed DNS resolution behavior
* 添加 sniff_timeout listen option
* 添加 custom route support for tun
* 添加 option for custom wireguard reserved bytes
* 将 bind_address 拆分为 ipv4 和 ipv6
* 添加 ShadowTLS v1 and v2 support

#### 1.1-rc1

* 修复 TLS config for h2 server
* 修复 crash when input bad method in shadowsocks multi-user inbound
* 修复 listen UDP
* 修复 check invalid packet on macOS

#### 1.1-beta18

* 增强对 shadowtls 服务器主动探测的防御 **1**

**1**：

The `fallback_after` option has been removed.

#### 1.1-beta17

* 修复 shadowtls server **1**

*1*:

Added [fallback_after](/zh/configuration/inbound/shadowtls#fallback_after) option.

#### 1.0.7

* 添加 support for new x/h2 deadline
* 修复 copy pipe
* 修复 decrypt xplus packet
* 修复 macOS Ventura process name match
* 修复 smux keepalive
* 修复 vmess request buffer
* 修复 h2c transport
* 修复 tor geoip
* 修复 udp connect for mux client
* 修复 default dns transport strategy

#### 1.1-beta16

* 改进 shadowtls server
* 修复 default dns transport strategy
* 更新 uTLS 至 v1.2.0

#### 1.1-beta15

* 添加 support for new x/h2 deadline
* 修复 udp connect for mux client
* 修复 dns buffer
* 修复 quic dns retry
* 修复 create TLS config
* 修复 websocket alpn
* 修复 tor geoip

#### 1.1-beta14

* 添加 multi-user support for hysteria inbound **1**
* 添加 custom tls client support for std grpc
* 修复 smux keep alive
* 修复 vmess request buffer
* 修复 default local DNS server behavior
* 修复 h2c transport

*1*:

The `auth` and `auth_str` fields have been replaced by the `users` field.

#### 1.1-beta13

* 添加 custom worker count option for WireGuard outbound
* 将 bind_address 拆分为 ipv4 和 ipv6
* 移动 WFP manipulation to strict route
* 修复 WireGuard outbound panic when close
* 修复 macOS Ventura process name match
* 修复 QUIC connection migration by @HyNetwork
* 修复 handling QUIC client SNI by @HyNetwork

#### 1.1-beta12

* 修复 uTLS config
* 更新 quic-go 至 v0.30.0
* 更新 cloudflare-tls 至 go1.18.7

#### 1.1-beta11

* 添加 option for custom wireguard reserved bytes
* 修复 shadowtls v2
* 修复 h3 dns transport
* 修复 copy pipe
* 修复 decrypt xplus packet
* 修复 v2ray api
* 抑制无网络错误
* 改进 local dns transport

#### 1.1-beta10

* 添加 [sniff_timeout](/zh/configuration/shared/listen#sniff_timeout) listen option
* 添加 [custom route](/zh/configuration/inbound/tun#inet4_route_address) support for tun **1**
* 修复 interface monitor
* 修复 websocket headroom
* 修复 uTLS handshake
* 修复 ssh outbound
* 修复 sniff fragmented quic client hello
* 修复 DF for hysteria
* 修复 naive overflow
* 在 UDP 连接前检查目标
* 更新 uTLS 至 v1.1.5
* 更新 tfo-go 至 v2.0.2
* 更新 fsnotify 至 v1.6.0
* 更新 grpc 至 v1.50.1

*1*:

The `strict_route` on windows is removed.

#### 1.0.6

* 修复 ssh outbound
* 修复 sniff fragmented quic client hello
* 修复 naive overflow
* 在 UDP 连接前检查目标

#### 1.1-beta9

* 修复 windows route **1**
* 添加 [v2ray statistics api](/zh/configuration/experimental#v2ray-api-fields)
* 添加 ShadowTLS v2 support **2**
* 修复与改进

**1**：

* 修复 DNS leak caused by
  Windows' [ordinary multihomed DNS resolution behavior](https://learn.microsoft.com/en-us/previous-versions/windows/it-pro/windows-server-2008-R2-and-2008/dd197552%28v%3Dws.10%29)
* Flush Windows DNS cache when start/close

**2**：

参阅 [ShadowTLS inbound](/zh/configuration/inbound/shadowtls#version)
and [ShadowTLS outbound](/zh/configuration/outbound/shadowtls#version)

#### 1.1-beta8

* 修复 leaks on close
* 改进 websocket writer
* 完善 tproxy 写回
* 完善 4in6 processing
* 修复 shadowsocks plugins
* 修复 missing source address from transport connection
* 修复 fqdn socks5 outbound connection
* 修复 read source address from grpc-go

#### 1.0.5

* 修复 missing source address from transport connection
* 修复 fqdn socks5 outbound connection
* 修复 read source address from grpc-go

#### 1.1-beta7

* 添加 v2ray mux and XUDP support for VMess inbound
* 添加 XUDP support for VMess outbound
* 禁用 DF on direct outbound by default
* 修复 bugs in 1.1-beta6

#### 1.1-beta6

* 添加 [URLTest outbound](/zh/configuration/outbound/urltest/)
* 修复 bugs in 1.1-beta5

#### 1.1-beta5

* 在版本命令中打印标签
* 将 clash hello 重定向到外部 UI
* 移动 shadowsocksr implementation to clash
* 使 gVisor optional **1**
* 重构 to miekg/dns
* 重构 bind control
* 修复 build on go1.18
* 修复 clash store-selected
* 修复 close grpc conn
* 修复 port rule match logic
* 修复 clash api proxy type

*1*:

The build tag `no_gvisor` is replaced by `with_gvisor`.

The default tun stack is changed to system.

#### 1.0.4

* 修复 close grpc conn
* 修复 port rule match logic
* 修复 clash api proxy type

#### 1.1-beta4

* 添加 internal simple-obfs and v2ray-plugin [Shadowsocks plugins](/zh/configuration/outbound/shadowsocks#plugin)
* 添加 [ShadowsocksR outbound](/zh/configuration/outbound/shadowsocksr/)
* 添加 [VLESS outbound and XUDP](/zh/configuration/outbound/vless/)
* 跳过等待 hysteria tcp 握手响应
* 修复 socks4 client
* 修复 hysteria inbound
* 修复 concurrent write

#### 1.0.3

* 修复 socks4 client
* 修复 hysteria inbound
* 修复 concurrent write

#### 1.1-beta3

* 修复 using custom TLS client in http2 client
* 修复 bugs in 1.1-beta2

#### 1.1-beta2

* 添加 Clash mode and persistence support **1**
* 添加 TLS ECH and uTLS support for outbound TLS options **2**
* 修复 socks4 request
* 修复 processing empty dns result

*1*:

Switching modes using the Clash API, and `store-selected` are now supported,
see [Experimental](/zh/configuration/experimental/).

*2*:

ECH (Encrypted Client Hello) is a TLS extension that allows a client to encrypt the first part of its ClientHello
message, see [TLS#ECH](/zh/configuration/shared/tls#ech).

uTLS is a fork of "crypto/tls", which provides ClientHello fingerprinting resistance,
see [TLS#uTLS](/zh/configuration/shared/tls#utls).

#### 1.0.2

* 修复 socks4 request
* 修复 processing empty dns result

#### 1.1-beta1

* 添加 support for use with android VPNService **1**
* 添加 tun support for WireGuard outbound **2**
* 添加 system tun stack **3**
* 添加 comment filter for config **4**
* 添加 option for allow optional proxy protocol header
* 添加 half close for smux
* 设置 UDP DF by default **5**
* 设置 default tun mtu to 9000
* 更新 gVisor 至 20220905.0

*1*:

In previous versions, Android VPN would not work with tun enabled.

The usage of tun over VPN and VPN over tun is now supported, see [Tun Inbound](/zh/configuration/inbound/tun#auto_route).

*2*:

In previous releases, WireGuard outbound support was backed by the lower performance gVisor virtual interface.

It achieves the same performance as wireguard-go by providing automatic system interface support.

*3*:

It does not depend on gVisor and has better performance in some cases.

It is less compatible and may not be available in some environments.

*4*:

Annotated json configuration files are now supported.

*5*:

UDP fragmentation is now blocked by default.

Including shadowsocks-libev, shadowsocks-rust and quic-go all disable segmentation by default.

参阅 [Dial Fields](/zh/configuration/shared/dial#udp_fragment)
and [Listen Fields](/zh/configuration/shared/listen#udp_fragment).

#### 1.0.1

* 修复 match 4in6 address in ip_cidr
* 修复 clash api log level format error
* 修复 clash api unknown proxy type

#### 1.0

* 修复 wireguard reconnect
* 修复 naive 入站
* 修复 json format error message
* 修复 processing vmess termination signal
* 修复 hysteria stream error
* 修复 listener close when proxyproto failed

#### 1.0-rc1

* 修复 write log timestamp
* 修复 write zero
* 修复 dial parallel in direct outbound
* 修复 write trojan udp
* 修复 DNS routing
* 添加 attribute support for geosite
* 更新 [Dial Fields](/zh/configuration/shared/dial/) 的文档

#### 1.0-beta3

* 添加 [chained inbound](/zh/configuration/shared/listen#detour) support
* 添加 process_path rule item
* 添加 macOS redirect support
* 添加 ShadowTLS [Inbound](/zh/configuration/inbound/shadowtls/), [Outbound](/zh/configuration/outbound/shadowtls/)
  and [Examples](/zh/examples/shadowtls/)
* 修复 search android package in non-owner users
* 修复 socksaddr type condition
* 修复 smux session status
* 重构 inbound and outbound documentation
* 小修复

#### 1.0-beta2

* 添加 strict_route option for [Tun inbound](/zh/configuration/inbound/tun#strict_route)
* 添加 packetaddr support for [VMess outbound](/zh/configuration/outbound/vmess#packet_addr)
* 添加 better performing alternative gRPC implementation
* 添加 [docker image](https://github.com/SagerNet/sing-box/pkgs/container/sing-box)
* 修复 sniff override destination

#### 1.0-beta1

* 初始版本

##### 2022/08/26

* 修复 ipv6 route on linux
* 修复 read DNS message

##### 2022/08/25

* 如果启用 TLS，让 vmess 使用零而不是自动
* 添加 trojan fallback for ALPN
* 改进 ip_cidr rule
* 修复 format bind_address
* 修复 http proxy with compressed response
* 修复 route connections

##### 2022/08/24

* 修复 naive padding
* 修复 unix search path
* 修复 close non-duplex connections
* 添加 ACME EAB support
* 修复 early close on windows and catch any
* Initial zh-CN document translation

##### 2022/08/23

* 添加 [V2Ray Transport](/zh/configuration/shared/v2ray-transport/) support for VMess and Trojan
* 允许 plain http request in Naive inbound (It can now be used with nginx)
* 添加 proxy protocol support
* 启动后释放内存
* 解析 HTTP 请求中的 X-Forward-For
* 处理 SIGHUP 信号

##### 2022/08/22

* 添加 strategy setting for each [DNS server](/zh/configuration/dns/server/)
* 添加 bind address to outbound options

##### 2022/08/21

* 添加 [Tor outbound](/zh/configuration/outbound/tor/)
* 添加 [SSH outbound](/zh/configuration/outbound/ssh/)

##### 2022/08/20

* 尝试展开 ip-in-fqdn socksaddr
* 修复 read packages in android 12
* 修复 route on some android devices
* 改进 linux process searcher
* 修复 write socks5 username password auth request
* 跳过到接口的私有目标绑定连接
* 添加 [Trojan connection fallback](/zh/configuration/inbound/trojan#fallback)

##### 2022/08/19

* 添加 Hysteria [Inbound](/zh/configuration/inbound/hysteria/) and [Outbund](/zh/configuration/outbound/hysteria/)
* 添加 [ACME TLS certificate issuer](/zh/configuration/shared/tls/)
* 允许 read config from stdin (-c stdin)
* 更新 gVisor 至 20220815.0

##### 2022/08/18

* 修复 find process with lwip stack
* 修复 crash on shadowsocks server
* 修复 crash on darwin tun
* 修复 write log to file

##### 2022/08/17

* 改进 async dns transports

##### 2022/08/16

* 添加 ip_version (route/dns) rule item
* 添加 [WireGuard](/zh/configuration/outbound/wireguard/) outbound

##### 2022/08/15

* 添加 uid, android user and package rules support in [Tun](/zh/configuration/inbound/tun/) routing.

##### 2022/08/13

* 修复 dns concurrent write

##### 2022/08/12

* 性能改进
* 添加 UoT option for [SOCKS](/zh/configuration/outbound/socks/) outbound

##### 2022/08/11

* 添加 UoT option for [Shadowsocks](/zh/configuration/outbound/shadowsocks/) outbound, UoT support for all inbounds

##### 2022/08/10

* 添加 full-featured [Naive](/zh/configuration/inbound/naive/) inbound
* 修复 default dns server option [#9] by iKirby

##### 2022/08/09

之前没有变更日志。

[#9]: https://github.com/SagerNet/sing-box/pull/9
