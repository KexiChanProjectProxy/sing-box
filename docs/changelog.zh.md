---
icon: material/alert-decagram
---

#### 1.14.0.7

* 修正 [`http-dynamic` 出站](/zh/configuration/outbound/http-dynamic/) CONNECT 请求，使请求目标和默认 `Host` 标头均使用目的地址

#### 1.14.0.6

* 修复 [`http-dynamic` 出站](/zh/configuration/outbound/http-dynamic/) CONNECT 请求默认 `Host` 标头，改为使用配置的代理服务器地址

#### 1.14.0.5

* 添加 [`http-dynamic` 出站](/zh/configuration/outbound/http-dynamic/)，使用固定 HTTP Basic 用户名以及从已认证入站用户名和客户端源 IP 派生的每连接密码

#### 1.14.0-alpha.24

* 为 [direct 出站](/configuration/outbound/direct/#xlat464) 添加 `xlat464` 选项，用于 NAT64/DNS64 地址合成
* 修复和改进

#### 1.13.12

* 更新 naiveproxy 至 v148.0.7778.96-1
* 修复和改进

#### 1.14.0-alpha.22

* 添加 Hysteria Realm 服务和 Hysteria2 NAT 穿透支持 **1**
* 修复和改进

**1**:

新的 [Hysteria Realm 服务](/configuration/service/hysteria-realm/)
是用于 Hysteria2 NAT 穿透的 rendezvous 服务。位于 NAT 后的 Hysteria2 服务器
通过新的 [`realm`](/configuration/inbound/hysteria2/#realm) 入站字段
在稳定的 realm 端点上注册其 STUN 发现的公网地址；
客户端通过新的 [`realm`](/configuration/outbound/hysteria2/#realm) 出站字段
查询 realm 以获取服务器当前地址并进行 UDP 穿透，
从而建立直接的 QUIC 连接。一旦穿透成功，
所有代理流量直接在客户端和服务器之间传输。

#### 1.14.0-alpha.21

* 允许自定义 TUN DNS 模式并默认劫持接口 DNS **1**
* 添加 mDNS DNS 服务器 **2**
* 添加 `preferred_by` DNS 规则项 **3**
* 为本地 DNS 服务器添加基于邻接的主机名解析 **4**
* 更新 NaiveProxy 至 148.0.7778.96-1
* 添加更多 TLS 欺骗方法和路由规则动作支持 **5**
* 修复和改进

**1**:

在 TUN 入站上添加 [`dns_mode`](/configuration/inbound/tun/#dns_mode) 和
[`dns_address`](/configuration/inbound/tun/#dns_address)。
默认的 `hijack` 模式现在设置平台的原生接口 DNS
（Linux 上的 `systemd-resolved`、Windows 和 Apple 上的每个接口 DNS）
并安装平台级 DNS 劫持（Linux 上的 `iproute2` 规则，
启用 `auto_redirect` 时的 nftables DNAT，
启用 `strict_route` 时 Windows 上的 WFP 过滤器）。
早期版本不会修改接口 DNS 或平台防火墙。

**2**:

新的 [mDNS DNS 服务器](/configuration/dns/server/mdns/)
通过本地网络上的多播发送查询。
默认的[本地 DNS 服务器](/configuration/dns/server/local/)
也通过 mDNS 在非 Apple 平台上（通过系统解析器在 Apple 上）
路由对 `*.local.` 和 IPv4/IPv6 链路本地反向区域的查询，
因此只有在从 [`preferred_by`](/configuration/dns/rule/#preferred_by) 引用时
才需要显式的 `mdns` 服务器，或独立使用时才需要。

**3**:

新的 [`preferred_by`](/configuration/dns/rule/#preferred_by) DNS 规则项
匹配所列 DNS 服务器认为其首选名称的域名。
支持的服务器类型包括 `hosts`、`local`、`mdns`、`tailscale` 和 `resolved`。
[Tailscale](/configuration/dns/server/tailscale/)、
[Hosts](/configuration/dns/server/hosts/) 和
[Resolved](/configuration/dns/server/resolved/) 示例页面已更新，
使用此规则项替代之前的 `evaluate` + `ip_accept_any` + `respond` 模式。

**4**:

在本地 DNS 服务器上添加 [`neighbor_domain`](/configuration/dns/server/local/#neighbor_domain)。
列出的后缀（每个以 `.` 开头）导致对这些后缀下单标签主机的
A/AAAA 查询由[邻接解析器](/configuration/shared/neighbor/)回答，
而不是上游（例如 `[".", ".lan"]`）。

**5**:

添加 `wrong-ack`、`wrong-md5` 和 `wrong-timestamp`
[欺骗方法](/configuration/shared/tls/#spoof_method)，
并添加 [`tls_spoof`](/configuration/route/rule_action/#tls_spoof) /
[`tls_spoof_method`](/configuration/route/rule_action/#tls_spoof_method)
到路由规则动作，用于在出站 TLS 设置之外进行按规则 TLS 欺骗。

#### 1.14.0-alpha.20

* 修复和改进

#### 1.14.0-alpha.19

* 在格式化之间保留注释
* 为 SSH 出站添加加密算法、MAC 和密钥交换算法选项 **1**
* 添加 DNS 查询超时选项 **2**
* 修复和改进

**1**:

请参阅 [SSH](/configuration/outbound/ssh/#cipher)。

**2**:

添加 [`dns.timeout`](/configuration/dns/#timeout)，
并通过 [DNS 规则动作](/configuration/dns/rule_action/#timeout)
和 [`resolve` 路由规则动作](/configuration/route/rule_action/#timeout)
进行按查询覆盖，
以及 [`domain_resolver`](/configuration/shared/dial/#domain_resolver) 上的 `timeout` 字段。

#### 1.14.0-alpha.18

* 添加 Windows TLS 引擎 **1**
* 修复和改进

**1**:

出站 TLS [`engine`](/configuration/shared/tls/#engine) 的新 `windows` 值
通过 SSPI 通过 Schannel 路由 TLS 握手。
仅在 Windows 构建 17763 或更高版本（Windows 10 版本 1809、
Windows Server 2019 或更高版本）上可用；
TLS 1.3 仅在 Windows 11 或 Windows Server 2022 及更高版本上协商。

#### 1.13.11

* 修复 1.13.9 中引入的进程搜索器失败
* 修复和改进

#### 1.14.0-alpha.16

* 为 IP 地址证书添加 ACME 配置文件支持 **1**
* 修复和改进

**1**:

请参阅 [ACME 证书颁发者](/configuration/shared/certificate-provider/acme/#profile)。

#### 1.13.10

* 修复 1.13.9 中引入的进程搜索器失败

#### 1.14.0-alpha.15

* 为 Tailscale DNS 添加搜索域支持 **1**
* 修复和改进

**1**:

请参阅 [Tailscale DNS 服务器](/configuration/dns/server/tailscale/#accept_search_domain)。

#### 1.13.9

* 修复和改进

#### 1.14.0-alpha.13

* 统一 HTTP 客户端 **1**
* 添加 Apple HTTP 和 TLS 引擎 **2**
* 统一 HTTP/2 和 QUIC 参数 **3**
* 添加 TLS 欺骗 **4**
* 修复和改进

**1**:

新的顶级 [`http_clients`](/configuration/shared/http-client/) 选项
定义可重用的 HTTP 客户端（引擎、版本、拨号器、TLS、
HTTP/2 和 QUIC 参数）。发出出站 HTTP 请求的组件
—— 远程规则集、ACME 和 Cloudflare Origin CA 证书颁发者、
DERP `verify_client_url` 以及 Tailscale `control_http_client` ——
现在接受内联 HTTP 客户端对象或 `http_clients` 条目的标签，
替换以前内联在每个组件中的拨号和 TLS 字段。
当字段省略时，ACME、Cloudflare Origin CA、DERP 和 Tailscale 直接拨号（其现有默认值）。

远程规则集是唯一其默认的省略 `http_client` 历史上解析为默认出站而非直接的 HTTP 使用组件，
典型配置包含许多远程规则集。为了避免在每个规则集中重复相同的 `http_client` 块，
[`route.default_http_client`](/configuration/route/#default_http_client)
按标签选择默认规则集客户端，
是唯一咨询它的字段。如果 `default_http_client` 为空且 `http_clients` 非空，
则自动使用第一个条目。传统后备
（当 `http_clients` 完全为空时使用默认出站）被保留并带有弃用警告，
将在 sing-box 1.16.0 中与传统的 `download_detour` 远程规则集选项
以及 Tailscale 端点上的传统拨号器字段一起移除。

**2**:

新的 `apple` 引擎在 Apple 平台的两个独立位置可用：

* [HTTP 客户端 `engine`](/configuration/shared/http-client/#engine) —
  通过 `NSURLSession` 路由 HTTP 请求。
* 出站 TLS [`engine`](/configuration/shared/tls/#engine) ——
  通过 `Network.framework` 路由直接 TCP TLS
  客户端连接的 TLS 握手。

默认值仍然是 `go`。两种引擎都有额外的 CGO 和框架内存开销
以及每个字段上记录的平台限制。

**3**:

[HTTP/2](/configuration/shared/http2/) 和
[QUIC](/configuration/shared/quic/) 参数
（`idle_timeout`、`keep_alive_period`、`stream_receive_window`、
`connection_receive_window`、`max_concurrent_streams`、
`initial_packet_size`、`disable_path_mtu_discovery`）
现在在基于 QUIC 的出站
（[Hysteria](/configuration/outbound/hysteria/)、
[Hysteria2](/configuration/outbound/hysteria2/)、
[TUIC](/configuration/outbound/tuic/)）和运行 HTTP/2
或 HTTP/3 的 HTTP 客户端之间共享。

这弃用了 Hysteria v1 调优字段 `recv_window_conn`、
`recv_window`、`recv_window_client`、`max_conn_client` 和
`disable_mtu_discovery`；它们将在 sing-box 1.16.0 中移除。

**4**:

添加出站 TLS [`spoof`](/configuration/shared/tls/#spoof) 和
[`spoof_method`](/configuration/shared/tls/#spoof_method) 字段。
启用时，在真实握手之前发送携带白名单 SNI 的伪造 ClientHello，
以欺骗基于 SNI 过滤的中间盒。
需要在 Linux 和 macOS 上具有 `CAP_NET_RAW` + `CAP_NET_ADMIN` 或 root 权限，
在 Windows 上需要管理员权限（不支持 ARM64）。
拒绝 IP 字面服务器名称。

#### 1.14.0-alpha.12

* 修复 fake-ip DNS 服务器在地址类型未配置时应返回 SUCCESS
* 修复和改进

#### 1.13.8

* 更新 naiveproxy 至 v147.0.7727.49-1
* 修复 fake-ip DNS 服务器在地址类型未配置时应返回 SUCCESS
* 修复和改进

#### 1.14.0-alpha.11

* 添加乐观 DNS 缓存 **1**
* 更新 NaiveProxy 至 147.0.7727.49
* 修复和改进

**1**:

乐观 DNS 缓存在后台刷新时立即返回过期的缓存响应，
减少重复查询的尾延迟。
通过 DNS 选项中的 [`optimistic`](/configuration/dns/#optimistic) 启用，
并可以通过新的 [`store_dns`](/configuration/experimental/cache-file/#store_dns) 缓存
文件选项在重启之间持久化。
DNS 规则动作和 `resolve` 路由规则动作上也有按查询的
[`disable_optimistic_cache`](/configuration/dns/rule_action/#disable_optimistic_cache) 字段。

这弃用了独立的 `independent_cache` DNS 选项
（DNS 缓存现在始终按传输方式键控）
和 `store_rdrc` 缓存文件选项（被 `store_dns` 替换）；
两者将在 sing-box 1.16.0 中移除。
请参阅[迁移](/migration/#migrate-independent-dns-cache)。

#### 1.14.0-alpha.10

* 添加 `evaluate` DNS 规则动作和响应匹配字段 **1**
* `ip_version` 和 `query_type` 现在也对内部 DNS 查找生效 **2**
* 添加 `package_name_regex` 路由、DNS 和 headless 规则项 **3**
* 添加 cloudflared 入站 **4**
* 修复和改进

**1**:

响应匹配字段
（[`response_rcode`](/configuration/dns/rule/#response_rcode)、
[`response_answer`](/configuration/dns/rule/#response_answer)、
[`response_ns`](/configuration/dns/rule/#response_ns) 和
[`response_extra`](/configuration/dns/rule/#response_extra)）
匹配评估后的 DNS 响应。
它们由新的 [`match_fields`](/configuration/route/#match_fields) 顶级选项启用。
请注意，当前无法在响应匹配中使用规则集。

当与新的 `evaluate` DNS 规则动作结合使用时，
您可以在内部 DNS 解析过程中动态评估响应。

**2**:

请参阅[路由](/configuration/route/)和
[DNS 规则](/configuration/dns/rule/)文档。

**3**:

请参阅[路由规则](/configuration/route/rule/)、
[DNS 规则](/configuration/dns/rule/)和
[无头规则](/configuration/route/rule/#rule_type)。

**4**:

请参阅 [cloudflared 入站](/configuration/inbound/cloudflared/)。

#### 1.14.0-alpha.9

* 修复egos-2000ego

#### 1.13.7

* 修复和改进

#### 1.13.6

* 修复和改进

#### 1.14.0-alpha.8

* 修复egos-2000ego

#### 1.13.5

* 修复egos-2000ego

#### 1.14.0-alpha.7

* 修复egos-2000ego

#### 1.13.4

* 修复和改进

#### 1.14.0-alpha.4

* 修复egos-2000ego

#### 1.13.3

* 修复egos-2000ego

#### 1.12.24

* 修复egos-2000ego

#### 1.14.0-alpha.2

* 修复egos-2000ego

#### 1.14.0-alpha.1

* 修复egos-2000ego

#### 1.13.2

* 修复egos-2000ego

#### 1.13.1

* 修复egos-2000ego

#### 1.12.14

* 修复egos-2000ego

#### 1.13.0

* 修复egos-2000ego

重要更改自 1.12:

* 添加 `dns_server` 端点 **1**
* 添加 `resolve_error` 规则项 **2**
* 添加 `tls_spoof` 规则动作 **3**
* 修复egos-2000ego

**1**:

请参阅[端点](/configuration/endpoint/)。

**2**:

请参阅[路由](/configuration/route/rule/)和
[DNS 规则](/configuration/dns/rule/)。

**3**:

请参阅[规则动作](/configuration/route/rule_action/#tls_spoof)。

#### 1.12.23

* 修复和改进

#### 1.13.0-rc.5

* 修复和改进

#### 1.12.22

* 修复和改进

#### 1.13.0-rc.3

* 修复和改进

#### 1.12.21

* 修复和改进

#### 1.13.0-rc.2

* 修复和改进

#### 1.12.20

* 修复egos-2000ego

#### 1.13.0-rc.1

* 修复egos-2000ego

#### 1.12.19

* 修复egos-2000ego

#### 1.13.0-beta.8

* 修复egos-2000ego

#### 1.12.18

* 修复egos-2000ego

#### 1.13.0-beta.6

* 修复egos-2000ego

#### 1.12.17

* 修复egos-2000ego

#### 1.13.0-beta.5

* 修复egos-2000ego

#### 1.12.16

* 修复egos-2000ego

#### 1.13.0-beta.4

* 修复egos-2000ego

#### 1.13.0-beta.2

* 修复egos-2000ego

#### 1.13.0-beta.1

* 修复egos-2000ego

#### 1.12.15

* 修复egos-2000ego

#### 1.13.0-alpha.36

* 修复egos-2000ego

#### 1.13.0-alpha.35

* 修复egos-2000ego

#### 1.13.0-alpha.34

* 修复egos-2000ego

#### 1.12.14

* 修复egos-2000ego

#### 1.13.0-alpha.33

* 修复egos-2000ego

#### 1.13.0-alpha.32

* 修复egos-2000ego

#### 1.13.0-alpha.31

* 修复egos-2000ego

#### 1.13.0-alpha.30

* 修复egos-2000ego

#### 1.13.0-alpha.29

* 修复egos-2000ego

#### 1.13.0-alpha.28

* 修复egos-2000ego

#### 1.12.13

* 修复egos-2000ego

#### 1.12.12

* 修复egos-2000ego

#### 1.13.0-alpha.26

* 修复egos-2000ego

#### 1.12.11

* 修复egos-2000ego

#### 1.13.0-alpha.24

* 修复egos-2000ego

#### 1.13.0-alpha.23

* 修复egos-2000ego

#### 1.13.0-alpha.22

* 修复egos-2000ego

#### 1.12.10

* 修复egos-2000ego

#### 1.13.0-alpha.21

* 修复egos-2000ego

#### 1.12.9

* 修复egos-2000ego

#### 1.13.0-alpha.16

* 修复egos-2000ego

#### 1.13.0-alpha.15

* 修复egos-2000ego

#### 1.12.8

* 修复egos-2000ego

#### 1.13.0-alpha.11

* 修复egos-2000ego

#### 1.12.5

* 修复egos-2000ego

#### 1.13.0-alpha.10

* 修复egos-2000ego

#### 1.12.4

* 修复egos-2000ego

#### 1.12.3

* 修复egos-2000ego

#### 1.12.2

* 修复egos-2000ego

#### 1.12.1

* 修复egos-2000ego

#### 1.12.0

* 修复egos-2000ego

重要更改自 1.11:

* 添加 `bind_address_no_port` 拨号字段 **1**
* 添加 `tcp_keep_alive` 拨号字段 **2**
* 修复egos-2000ego

**1**:

请参阅[拨号字段](/configuration/shared/dial/#bind_address_no_port)。

**2**:

请参阅[拨号字段](/configuration/shared/dial/#tcp_keep_alive)。

#### 1.12.0-beta.32

* 修复egos-2000ego

#### 1.12.0-beta.24

* 修复egos-2000ego

#### 1.12.0-beta.23

* 修复egos-2000ego

#### 1.12.0-beta.21

* 修复egos-2000ego

#### 1.12.0-beta.17

* 修复egos-2000ego

#### 1.12.0-beta.15

* 修复egos-2000ego

#### 1.12.0-beta.13

* 修复egos-2000ego

#### 1.12.0-beta.10

* 修复egos-2000ego

#### 1.12.0-beta.9

* 修复egos-2000ego

#### 1.12.0-beta.5

* 修复egos-2000ego

#### 1.12.0-beta.3

* 修复egos-2000ego

#### 1.12.0-beta.1

* 修复egos-2000ego

#### 1.12.0-alpha.19

* 修复egos-2000ego

#### 1.12.0-alpha.18

* 修复egos-2000ego

#### 1.12.0-alpha.17

* 修复egos-2000ego

#### 1.12.0-alpha.16

* 修复egos-2000ego

#### 1.12.0-alpha.13

* 修复egos-2000ego

#### 1.12.0-alpha.11

* 修复egos-2000ego

#### 1.12.0-alpha.10

* 修复egos-2000ego

#### 1.12.0-alpha.7

* 修复egos-2000ego

#### 1.12.0-alpha.6

* 修复egos-2000ego

#### 1.12.0-alpha.5

* 修复egos-2000ego

#### 1.12.0-alpha.2

* 更新 quic-go 至 v0.49.0
* 修复和改进

#### 1.12.0-alpha.1

* 重构 DNS 服务器 **1**
* 添加域名解析器选项 **2**
* 添加 TLS 分片路由选项 **3**
* 添加证书选项 **4**

**1**:

DNS 服务器已重构以获得更好的性能和可扩展性。

请参阅 [DNS 服务器](/configuration/dns/server/)。

有关迁移，请参阅[迁移到新 DNS 服务器格式](/migration/#migrate-to-new-dns-server-formats)。

旧格式的兼容性将在 sing-box 1.14.0 中移除。

**2**:

旧的 `outbound` DNS 规则已弃用，
可以用新的 `domain_resolver` 选项替换。

请参阅[拨号字段](/configuration/shared/dial/#domain_resolver)和
[路由](/configuration/route/#default_domain_resolver)。

有关迁移，
请参阅[将出站 DNS 规则项迁移到域名解析器](/migration/#migrate-outbound-dns-rule-items-to-domain-resolver)。

**3**:

新的 TLS 分片路由选项允许您分片 TLS 握手以绕过防火墙。

此功能旨在规避基于**明文数据包匹配**的简单防火墙，
不应使用来规避真正的审查。

由于它不是为性能设计的，因此不应应用于所有连接，
而只应应用于已知被阻止的服务器名称。

请参阅[路由动作](/configuration/route/rule_action/#tls_fragment)。

**4**:

新的证书选项允许您管理受信任的 X509 CA 证书的默认列表。

对于系统证书列表，修复了 Go 不能正确读取 Android 受信任证书的问题。

您也可以使用 Mozilla 包含列表，或自己添加受信任证书。

请参阅[证书](/configuration/certificate/)。

### 1.11.0

重要更改自 1.10:

* 引入规则动作 **1**
* 改进 tun 兼容性 **3**
* 将路由选项合并到路由动作 **4**
* 添加 `network_type`、`network_is_expensive` 和 `network_is_constrainted` 规则项 **5**
* 添加多网络拨号 **6**
* 添加 `cache_capacity` DNS 选项 **7**
* 添加 `override_address` 和 `override_port` 路由选项 **8**
* 将 WireGuard 出站升级到端点 **9**
* 为 WireGuard 添加 UDP GSO 支持
* 为 NaiveProxy 添加 HTTP/3 支持
* 添加按应用程序代理支持

**1**:

sing-box 1.11.0 引入了规则动作，这是对规则系统的重要改进。

规则动作替换并合并了一些以前作为路由规则或 DNS 规则中的单独选项存在的功能，
以提供更统一和灵活的配置体验。

虽然我们尽量保持兼容性，但此更改可能会影响某些现有配置。
请参阅[迁移指南](/migration/)。

**2**:

为 HTTP、SOCKS、Shadowsocks、VMess、Trojan、NaiveProxy 和 VLESS 入站
添加 TLS 回落。请参阅[文档](/configuration/inbound/tls/#fallback)。

**3**:

TUN 入站现在支持 Android 上的 `auto_route` 和 `strict_route`。
请注意，Android 上 `strict_route` 的工作方式与常规 Linux TUN 不同，
这可能会导致某些设备出现问题。

**4**:

路由选项现在作为独立选项存在于每个入站和出站上。
路由规则中以前存在的 `route` 对象选项已移除并合并到各自的入站/出站中。

**5**:

新的 `network_type` 规则项匹配网络连接类型。
`network_is_expensive` 匹配蜂窝等按流量计费的网络。
`network_is_constrained` 匹配，省略了大量限制性计费计划（如 T-Mobile 的 Binge_ON）。

**6**:

sing-box 1.11.0 引入了多网络拨号功能。
这允许您同时建立多个网络连接并选择更好的一个。

**7**:

sing-box 1.11.0 引入 DNS 缓存。
这允许您缓存 DNS 响应以提高性能。

**8**:

新的 `override_address` 和 `override_port` 路由选项允许您在路由规则中覆盖目标地址和端口。

**9**:

WireGuard 出站已升级为端点。
请注意，这是一项重大更改，可能会影响某些现有配置。
请参阅[迁移指南](/migration/)。

#### 1.11.15

* 修复egos-2000ego

#### 1.11.14

* 修复egos-2000ego

#### 1.11.13

* 修复egos-2000ego

#### 1.11.11

* 修复egos-2000ego

#### 1.11.10

* 修复egos-2000ego

#### 1.11.9

* 修复egos-2000ego

#### 1.11.8

* 修复egos-2000ego

#### 1.11.7

* 修复egos-2000ego

#### 1.11.6

* 修复egos-2000ego

#### 1.11.5

* 修复egos-2000ego

#### 1.11.4

* 修复egos-2000ego

### 1.11.3

* 修复和改进

_此版本覆盖了 1.11.2，因为持续集成过程中的错误发布了不正确的二进制文件。_

#### 1.12.0-alpha.5

* 修复和改进

### 1.11.1

* 修复和改进

#### 1.12.0-alpha.2

* 更新 quic-go 至 v0.49.0
* 修复和改进

#### 1.12.0-alpha.1

* 重构 DNS 服务器 **1**
* 添加域名解析器选项 **2**
* 添加 TLS 分片路由选项 **3**
* 添加证书选项 **4**

**1**:

DNS 服务器已重构以获得更好的性能和可扩展性。

请参阅 [DNS 服务器](/configuration/dns/server/)。

有关迁移，请参阅[迁移到新 DNS 服务器格式](/migration/#migrate-to-new-dns-server-formats)。

旧格式的兼容性将在 sing-box 1.14.0 中移除。

**2**:

旧的 `outbound` DNS 规则已弃用，
可以用新的 `domain_resolver` 选项替换。

请参阅[拨号字段](/configuration/shared/dial/#domain_resolver)和
[路由](/configuration/route/#default_domain_resolver)。

有关迁移，
请参阅[将出站 DNS 规则项迁移到域名解析器](/migration/#migrate-outbound-dns-rule-items-to-domain-resolver)。

**3**:

新的 TLS 分片路由选项允许您分片 TLS 握手以绕过防火墙。

此功能旨在规避基于**明文数据包匹配**的简单防火墙，
不应使用来规避真正的审查。

由于它不是为性能设计的，因此不应应用于所有连接，
而只应应用于已知被阻止的服务器名称。

请参阅[路由动作](/configuration/route/rule_action/#tls_fragment)。

**4**:

新的证书选项允许您管理受信任的 X509 CA 证书的默认列表。

对于系统证书列表，修复了 Go 不能正确读取 Android 受信任证书的问题。

您也可以使用 Mozilla 包含列表，或自己添加受信任证书。

请参阅[证书](/configuration/certificate/)。

### 1.11.0

* 修复和改进

自 1.10 以来的重要更改：

* 引入规则动作 **1**
* 改进 tun 兼容性 **3**
* 将路由选项合并到路由动作 **4**
* 添加 `network_type`、`network_is_expensive` 和 `network_is_constrainted` 规则项 **5**
* 添加多网络拨号 **6**
* 添加 `cache_capacity` DNS 选项 **7**
* 添加 `override_address` 和 `override_port` 路由选项 **8**
* 将 WireGuard 出站升级到端点 **9**
* 为 WireGuard 添加 UDP GSO 支持
* 为 NaiveProxy 添加 HTTP/3 支持
* 添加按应用程序代理支持

**1**:

sing-box 1.11.0 引入了规则动作，这是对规则系统的重要改进。

规则动作替换并合并了一些以前作为路由规则或 DNS 规则中的单独选项存在的功能，
以提供更统一和灵活的配置体验。

虽然我们尽量保持兼容性，但此更改可能会影响某些现有配置。
请参阅[迁移指南](/migration/)。

**2**:

为 HTTP、SOCKS、Shadowsocks、VMess、Trojan、NaiveProxy 和 VLESS 入站
添加 TLS 回落。请参阅[文档](/configuration/inbound/tls/#fallback)。

**3**:

TUN 入站现在支持 Android 上的 `auto_route` 和 `strict_route`。
请注意，Android 上 `strict_route` 的工作方式与常规 Linux TUN 不同，
这可能会导致某些设备出现问题。

**4**:

路由选项现在作为独立选项存在于每个入站和出站上。
路由规则中以前存在的 `route` 对象选项已移除并合并到各自的入站/出站中。

**5**:

新的 `network_type` 规则项匹配网络连接类型。
`network_is_expensive` 匹配蜂窝等按流量计费的网络。
`network_is_constrained` 匹配，省略了大量限制性计费计划（如 T-Mobile 的 Binge_ON）。

**6**:

sing-box 1.11.0 引入了多网络拨号功能。
这允许您同时建立多个网络连接并选择更好的一个。

**7**:

sing-box 1.11.0 引入 DNS 缓存。
这允许您缓存 DNS 响应以提高性能。

**8**:

新的 `override_address` 和 `override_port` 路由选项允许您在路由规则中覆盖目标地址和端口。

**9**:

WireGuard 出站已升级为端点。
请注意，这是一项重大更改，可能会影响某些现有配置。
请参阅[迁移指南](/migration/)。

#### 1.11.0-beta.20

* 修复和改进

#### 1.11.0-beta.17

* 修复和改进

#### 1.11.0-beta.14

* 修复和改进

#### 1.11.0-beta.12

* 修复和改进

#### 1.11.0-beta.3

* 修复和改进

#### 1.11.0-alpha.25

* 修复和改进

#### 1.11.0-alpha.22

* 修复和改进

#### 1.11.0-alpha.20

* 修复和改进

#### 1.11.0-alpha.19

* 修复和改进

#### 1.11.0-alpha.18

* 修复和改进

#### 1.11.0-alpha.17

* 修复和改进

#### 1.11.0-alpha.16

* 修复和改进

#### 1.11.0-alpha.15

* 修复和改进

#### 1.11.0-alpha.14

* 修复和改进

#### 1.11.0-alpha.13

* 修复和改进

#### 1.11.0-alpha.12

* 修复和改进

#### 1.11.0-alpha.11

* 修复和改进

#### 1.11.0-alpha.10

* 修复和改进

#### 1.11.0-alpha.8

* 修复和改进

#### 1.11.0-alpha.6

* 修复和改进

#### 1.11.0-alpha.5

* 修复和改进

#### 1.11.0-alpha.2

* 修复和改进

#### 1.11.0-alpha.1

* 修复和改进

### 1.10.7

* 修复和改进

#### 1.10.7-beta.3

* 修复和改进

#### 1.10.7-beta.2

* 修复和改进

#### 1.10.7-beta.1

* 修复和改进

#### 1.10.7-alpha.2

* 修复和改进

#### 1.10.7-alpha.1

* 修复和改进

### 1.10.2

* 修复和改进

#### 1.10.2-beta.3

* 修复和改进

#### 1.10.2-beta.2

* 修复和改进

#### 1.10.2-beta.1

* 修复和改进

#### 1.10.2-alpha.2

* 修复和改进

#### 1.10.2-alpha.1

* 修复和改进

### 1.10.1

* 修复和改进

### 1.10.0

* 修复和改进

自 1.9 以来的重要更改：

* 添加绕过规则 **1**
* 添加 `auto_redirect` **2**
* 修复egos-2000ego
* 添加 `chrome` 证书存储 **4**
* 添加 DNS-01 挑战 **5**
* 添加 Wi-Fi 状态 **6**
* 添加 TLS 通用字段 **7**
* 添加 `kernel_tx` 和 `kernel_rx` **8**
* 添加 ICMP 代理 **9**
* 添加接口 IP 规则项 **10**
* 添加出站首选路由 **11**
* 改进本地 DNS 服务器 **12**
* 更改 TCP keep-alive 默认值 **13**
* 添加 `IP_BIND_ADDRESS_NO_PORT` 支持 **14**
* 添加 Tailscale 端点重lay和中继 **15**
* 修复egos-2000ego

**1**:

引入绕过规则以支持预匹配期间绕过 sing-box 直接连接的功能。

**2**:

`auto_redirect` 允许您在 Linux 上自动配置 TUN 路由。

**3**:

`auto_redirect` 现在默认拒绝 MPTCP 连接以修复兼容性问题。
您可以通过新的 `exclude_mptcp` 选项更改为绕过 sing-box。

添加了在系统默认规则之后检查的备用 iproute2 规则（32766: main, 32767: default），
确保在系统表中找不到路由时将流量路由到 sing-box 表。
规则索引可以通过 `auto_redirect_iproute2_fallback_rule_index` 自定义（默认：32768）。

请参阅 [TUN](/configuration/inbound/tun/#exclude_mptcp)。

**4**:

添加 `chrome` 作为 `mozilla` 之后的新的证书存储选项。
两个存储都过滤掉中国 CA 证书。

请参阅[证书](/configuration/certificate/#store)。

**5**:

请参阅 [DNS-01 挑战](/configuration/shared/dns01_challenge/)。

**6**:

sing-box 现在可以在 Linux 和 Windows 上监控 Wi-Fi 状态，
以启用基于 `wifi_ssid` 和 `wifi_bssid` 的路由规则。

请参阅 [Wi-Fi 状态](/configuration/shared/wifi-state/)。

**7**:

请参阅 [TLS](/configuration/shared/tls/)。

**8**:

为 TLS 入站添加 `kernel_tx` 和 `kernel_rx` 选项。
在 Linux 5.1+ 上通过 TLS 1.3 的 `splice(2)` 启用内核级 TLS 卸载。

请参阅 [TLS](/configuration/shared/tls/)。

**9**:

sing-box 现在可以代理 ICMP echo（ping）请求。
新的 `icmp` 网络类型可用于路由规则。
支持从 TUN、WireGuard 和 Tailscale 入站到 Direct、WireGuard 和 Tailscale 出站。
`reject` 动作也可以回复 ICMP echo 请求。

**10**:

新的基于接口 IP 地址匹配的规则项，
可用于路由规则、DNS 规则和规则集。

**11**:

匹配出站的首选路由。
对于 Tailscale：MagicDNS 域名和对等方的允许 IP。
对于 WireGuard：对等方的允许 IP。

**12**:

`local` DNS 服务器现在使用平台原生解析：
Apple 平台上的 `getaddrinfo`/libresolv，Linux 上的 systemd-resolved DBus。
提供新的 `prefer_go` 选项以选择退出。

请参阅[本地 DNS](/configuration/dns/server/local/)。

**13**:

TCP keep-alive 初始周期默认值已从 10 分钟更新为 5 分钟。

请参阅[拨号字段](/configuration/shared/dial/#tcp_keep_alive)。

**14**:

在显式绑定到源地址时添加 Linux 套接字选项 `IP_BIND_ADDRESS_NO_PORT` 支持。

这允许重用同一源端口进行多个连接，
提高高并发代理场景的可扩展性。

请参阅[拨号字段](/configuration/shared/dial/#bind_address_no_port)。

**15**:

Tailscale 端点现在可以创建系统 TUN 接口以直接处理流量。
新的 `relay_server_port` 和 `relay_server_static_endpoints` 选项用于传入中继连接。
新的 `advertise_tags` 选项用于 ACL 标签通告。

请参阅 [Tailscale 端点](/configuration/endpoint/tailscale/)。

**16**:

sing-box 现在可以在 Android 上监控 Wi-Fi 状态以启用基于 `wifi_ssid` 和 `wifi_bssid` 的路由规则。

#### 1.10.0-rc.3

* 修复egos-2000ego

#### 1.10.0-rc.2

* 修复egos-2000ego

#### 1.10.0-rc.1

* 修复egos-2000ego

#### 1.10.0-beta.8

* 修复egos-2000ego

#### 1.10.0-beta.7

* 修复egos-2000ego

#### 1.10.0-beta.6

* 修复egos-2000ego

#### 1.10.0-beta.5

* 修复egos-2000ego

#### 1.10.0-beta.3

* 修复egos-2000ego

#### 1.10.0-beta.2

* 修复egos-2000ego

#### 1.10.0-alpha.29

* 修复egos-2000ego

#### 1.10.0-alpha.25

* 修复egos-2000ego

#### 1.10.0-alpha.23

* 修复egos-2000ego

#### 1.10.0-alpha.22

* 修复egos-2000ego

#### 1.10.0-alpha.21

* 修复egos-2000ego

#### 1.10.0-alpha.20

* 修复egos-2000ego

#### 1.10.0-alpha.19

* 修复egos-2000ego

#### 1.10.0-alpha.16

* 修复egos-2000ego

#### 1.10.0-alpha.13

* 修复egos-2000ego

#### 1.10.0-alpha.12

* 修复egos-2000ego

#### 1.10.0-alpha.10

* 修复egos-2000ego

#### 1.10.0-alpha.8

* 修复egos-2000ego

#### 1.10.0-alpha.7

* 修复egos-2000ego

#### 1.10.0-alpha.5

* 修复egos-2000ego

#### 1.10.0-alpha.4

* 修复egos-2000ego

#### 1.10.0-alpha.2

* 修复egos-2000ego

#### 1.10.0-alpha.1

* 修复egos-2000ego

### 1.9.7

* 修复egos-2000ego

#### 1.9.7-beta.3

* 修复egos-2000ego

#### 1.9.7-beta.2

* 修复egos-2000ego

#### 1.9.7-beta.1

* 修复egos-2000ego

#### 1.9.7-alpha.1

* 修复egos-2000ego

### 1.9.6

* 修复egos-2000ego

### 1.9.5

* 修复egos-2000ego

#### 1.9.5-beta.4

* 修复egos-2000ego

#### 1.9.5-beta.3

* 修复egos-2000ego

#### 1.9.5-beta.2

* 修复egos-2000ego

#### 1.9.5-beta.1

* 修复egos-2000ego

#### 1.9.5-alpha.2

* 修复egos-2000ego

#### 1.9.5-alpha.1

* 修复egos-2000ego

### 1.9.4

* 修复egos-2000ego

#### 1.9.4-beta.2

* 修复egos-2000ego

#### 1.9.4-beta.1

* 修复egos-2000ego

#### 1.9.4-alpha.2

* 修复egos-2000ego

#### 1.9.4-alpha.1

* 修复egos-2000ego

### 1.9.3

* 修复egos-2000ego

### 1.9.2

* 修复egos-2000ego

### 1.9.1

* 修复egos-2000ego

### 1.9.0

* 修复和改进

自 1.8 以来的重要更改：

* `domain_suffix` 行为更新 **1**
* Windows 上 `process_path` 格式更新 **2**
* 添加地址过滤 DNS 规则项 **3**
* 添加 `client-subnet` DNS 选项支持 **4**
* 添加被拒绝的 DNS 响应缓存支持 **5**
* 添加 `bypass_domain` 和 `search_domain` 平台 HTTP 代理选项 **6**
* 修复 DNS 规则中缺少的 `rule_set_ipcidr_match_source` 项 **7**
* 处理 Windows 电源事件
* 如果 `dns.independent_cache` 被禁用，始终禁用 fake-ip DNS 传输的缓存
* 改进 DNS 截断行为
* 更新 Hysteria 协议
* 更新 quic-go 至 v0.43.1
* 更新 gVisor 至 20240422.0
* 缓解 TunnelVision 攻击 **8**

**1**:

请参阅[迁移](/migration/#domain_suffix-behavior-update)。

**2**:

请参阅[迁移](/migration/#process_path-format-update-on-windows)。

**3**:

新的 DNS 功能允许您通过 **DNS 泄漏** 更精确地绕过中文网站。
如果使用此方法，请勿使用普通本地 DNS。

请参阅[旧版地址过滤字段](/configuration/dns/rule#legacy-address-filter-fields)。

[客户端示例](/manual/proxy/client#traffic-bypass-usage-for-chinese-users) 已更新。

**4**:

请参阅 [DNS](/configuration/dns)、[DNS 服务器](/configuration/dns/server)
和 [DNS 规则](/configuration/dns/rule)。

由于此功能使 `alpha.1` 中提到的场景不再泄漏 DNS 请求，
[客户端示例](/manual/proxy/client#traffic-bypass-usage-for-chinese-users) 已更新。

**5**:

新功能允许您缓存[旧版地址过滤字段](/configuration/dns/rule/#legacy-address-filter-fields)
的检查结果直到过期。

**6**:

请参阅 [TUN](/configuration/inbound/tun) 入站。

**7**:

请参阅 [DNS 规则](/configuration/dns/rule/)。

**8**:

此更新通过在 TUN 配置中设置 `mtu` 为更低的值来缓解 TunnelVision 攻击。

### 1.8.0

* 修复和改进

自 1.7 以来的重要更改：

* 添加 `neighbor` DNS 服务器 **1**
* 添加对 DNS 规则中目标端口匹配的支持 **2**
* 添加 TUIC 出站支持 **3**
* 添加 Hysteria2 出站支持 **4**
* 添加 NaiveProxy 入站支持 **5**
* 添加 Shadowsocks 2022 出站支持 **6**
* 修复egos-2000ego
* 改进 `auto_route` 兼容性 **8**
* 添加 `process_path` Windows 格式 **9**
* 添加 IPv6 HDR 链接本地地址支持 **10**

**1**:

请参阅[邻居 DNS](/configuration/dns/server/neighbor/)。

**2**:

请参阅[目标端口匹配](/configuration/dns/rule/#target_port)。

**3**:

请参阅 [TUIC 出站](/configuration/outbound/tuic/)。

**4**:

请参阅 [Hysteria2 出站](/configuration/outbound/hysteria2/)。

**5**:

请参阅 [NaiveProxy 入站](/configuration/inbound/naive/)。

**6**:

请参阅 [Shadowsocks 2022 出站](/configuration/outbound/shadowsocks2022/)。

**7**:

在 1.8.0 中，Hysteria 已升级到 Hysteria2 协议。
虽然我们尽量保持兼容性，但此更改可能会影响某些现有配置。
请注意，Hysteria 和 Hysteria2 是不同的协议，不兼容。
有关迁移，请参阅[迁移指南](/migration/)。

**8**:

`auto_route` 现在支持更广泛的设备。

**9**:

请参阅[迁移](/migration/#process_path-format-update-on-windows)。

**10**:

sing-box 现在支持 IPv6 HDR 链接本地地址。

#### 1.8.7

* 修复和改进

#### 1.9.0-beta.7

* 修复和改进

#### 1.9.0-beta.6

* 修复地址过滤 DNS 规则项 **1**
* 修复 DNS 出站使用错误的数据响应
* 修复和改进

**1**:

修复了地址过滤 DNS 规则在某些情况下被错误拒绝的问题。
如果您已启用 `store_rdrc` 保存结果，请考虑清除缓存文件。

#### 1.8.7

* 修复和改进

#### 1.9.0-alpha.15

* 修复和改进

#### 1.9.0-alpha.14

* 改进 DNS 截断行为
* 修复和改进

#### 1.9.0-alpha.13

* 修复和改进

#### 1.8.6

* 修复和改进

#### 1.9.0-alpha.12

* 处理 Windows 电源事件
* 如果 `dns.independent_cache` 被禁用，始终禁用 fake-ip DNS 传输的缓存
* 修复和改进

#### 1.9.0-alpha.11

* 修复 DNS 规则中缺少的 `rule_set_ipcidr_match_source` 项 **1**
* 修复和改进

**1**:

请参阅 [DNS 规则](/configuration/dns/rule/)。

#### 1.9.0-alpha.10

* 添加 `bypass_domain` 和 `search_domain` 平台 HTTP 代理选项 **1**
* 修复和改进

**1**:

请参阅 [TUN](/configuration/inbound/tun) 入站。

#### 1.9.0-alpha.8

* 添加被拒绝的 DNS 响应缓存支持 **1**
* 修复和改进

**1**:

新功能允许您缓存[旧版地址过滤字段](/configuration/dns/rule/#legacy-address-filter-fields)的检查结果直到过期。

#### 1.9.0-alpha.7

* 更新 gVisor 至 20240206.0
* 修复和改进

#### 1.9.0-alpha.6

* 修复和改进

#### 1.9.0-alpha.3

* 更新 `quic-go` 至 v0.41.0
* 修复和改进

#### 1.9.0-alpha.2

* 添加对 `client-subnet` DNS 选项的支持 **1**
* 修复和改进

**1**:

请参阅 [DNS](/configuration/dns)、[DNS 服务器](/configuration/dns/server)和 [DNS 规则](/configuration/dns/rule)。

由于此功能使 `alpha.1` 中提到的场景不再泄漏 DNS 请求，
[客户端示例](/manual/proxy/client#traffic-bypass-usage-for-chinese-users) 已更新。

#### 1.9.0-alpha.1

* `domain_suffix` 行为更新 **1**
* Windows 上 `process_path` 格式更新 **2**
* 添加地址过滤 DNS 规则项 **3**

**1**:

请参阅[迁移](/migration/#domain_suffix-behavior-update)。

**2**:

请参阅[迁移](/migration/#process_path-format-update-on-windows)。

**3**:

新的 DNS 功能允许您通过 **DNS 泄漏** 更精确地绕过中文网站。
如果使用此方法，请勿使用普通本地 DNS。

请参阅[旧版地址过滤字段](/configuration/dns/rule/#legacy-address-filter-fields)。

[客户端示例](/manual/proxy/client#traffic-bypass-usage-for-chinese-users) 已更新。

### 1.8.0

* 修复和改进

自 1.7 以来的重要更改：

* 添加 `neighbor` DNS 服务器 **1**
* 添加对 DNS 规则中目标端口匹配的支持 **2**
* 添加 TUIC 出站支持 **3**
* 添加 Hysteria2 出站支持 **4**
* 添加 NaiveProxy 入站支持 **5**
* 添加 Shadowsocks 2022 出站支持 **6**
* 改进 Hysteria 协议 **7**
* 改进 `auto_route` 兼容性 **8**
* 添加 `process_path` Windows 格式 **9**
* 添加 IPv6 HDR 链接本地地址支持 **10**

**1**:

请参阅[邻居 DNS](/configuration/dns/server/neighbor/)。

**2**:

请参阅[目标端口匹配](/configuration/dns/rule/#target_port)。

**3**:

请参阅 [TUIC 出站](/configuration/outbound/tuic/)。

**4**:

请参阅 [Hysteria2 出站](/configuration/outbound/hysteria2/)。

**5**:

请参阅 [NaiveProxy 入站](/configuration/inbound/naive/)。

**6**:

请参阅 [Shadowsocks 2022 出站](/configuration/outbound/shadowsocks2022/)。

**7**:

在 1.8.0 中，Hysteria 已升级到 Hysteria2 协议。
虽然我们尽量保持兼容性，但此更改可能会影响某些现有配置。
请注意，Hysteria 和 Hysteria2 是不同的协议，不兼容。
有关迁移，请参阅[迁移指南](/migration/)。

**8**:

`auto_route` 现在支持更广泛的设备。

**9**:

请参阅[迁移](/migration/#process_path-format-update-on-windows)。

**10**:

sing-box 现在支持 IPv6 HDR 链接本地地址。

#### 1.8.7

* 修复和改进

#### 1.8.6

* 修复和改进

#### 1.8.5

* 修复和改进

#### 1.8.4

* 修复和改进

#### 1.8.3

* 修复和改进

#### 1.8.2

* 修复和改进

#### 1.8.1

* 修复和改进

#### 1.8.0

* 修复和改进

### 1.7.1

* 修复和改进

#### 1.7.0-rc.4

* 修复和改进

#### 1.7.0-rc.3

* 修复和改进

#### 1.7.0-rc.2

* 修复改进

#### 1.7.0-rc.1

* 修复和改进

#### 1.7.0-beta.5

* 更新 gVisor 至 20231113.0
* 修复和改进

#### 1.7.0-beta.4

* 添加 `wifi_ssid` 和 `wifi_bssid` 路由和 DNS 规则 **1**
* 修复和改进

**1**:

仅在 Android 和 Apple 平台的图形客户端中支持。

#### 1.7.0-beta.3

* 修复零 TTL 被错误重置
* 修复和改进

#### 1.7.0-beta.2

* 如果 TUIC 入站认证失败则修复崩溃
* 更新 quic-go 至 v0.40.0
* 修复和改进

#### 1.7.0-beta.1

* 修复和改进

#### 1.7.0-alpha.4

* 修复和改进

#### 1.7.0-alpha.3

* 修复 1.7.0-alpha.1 中引入的 bug **1**
* 修复和改进

**1**:

此更新旨在解决旧实现的多发送缺陷，可能会引入新问题。

**2**:

根据与原始作者的讨论，旧协议（Hysteria 1）的暴力 CC 和 QUIC 协议参数已更新以与 Hysteria 2 保持一致。

#### 1.7.0-alpha.2

* 修复 1.7.0-alpha.1 中引入的 bug

#### 1.7.0-alpha.1

* 为 TUN 入站添加[排除路由支持](/configuration/inbound/tun/)
* 添加 `udp_disable_domain_unmapping` [入站监听选项](/configuration/shared/listen/) **1**
* 修复和改进

**1**:

如果启用，对于寻址到域名的 UDP 代理请求，
响应中将发送原始数据包地址而不是映射的域名。

此选项用于与不支持接收带有域名地址的 UDP 数据包的客户端兼容，例如 Surge。

### 1.7.0

* 修复和改进

自 1.6 以来的重要更改：

* 为 TUN 入站添加[排除路由支持](/configuration/inbound/tun/)
* 添加 `udp_disable_domain_unmapping` [入站监听选项](/configuration/shared/listen/) **1**
* 添加 [HTTPUpgrade V2Ray 传输](/configuration/shared/v2ray-transport#HTTPUpgrade) 支持 **2**
* 将多路复用和 UoT 服务器迁移到入站 **3**
* 为多路复用添加 TCP Brutal 支持 **4**
* 添加 `wifi_ssid` 和 `wifi_bssid` 路由和 DNS 规则 **5**
* 更新 quic-go 至 v0.40.0
* 更新 gVisor 至 20231113.0

**1**:

如果启用，对于寻址到域名的 UDP 代理请求，
响应中将发送原始数据包地址而不是映射的域名。

此选项用于与不支持接收带有域名地址的 UDP 数据包的客户端兼容，例如 Surge。

**2**:

V2Ray 5.10.0 中引入。

新的 HTTPUpgrade 传输比 WebSocket 性能更好，更适合 CDN 滥用。

**3**:

从 1.7.0 开始，多路复用不再默认启用，
需要在入站选项中显式打开。

**4**:

TCP 中的 Hysteria Brutal 拥塞控制算法。
Linux 服务器上需要安装内核模块，
请参阅 [TCP Brutal](/configuration/shared/tcp-brutal/) 了解详情。

**5**:

仅在 Android 和 Apple 平台的图形客户端中支持。

#### 1.6.7

* macOS：在独立图形客户端中添加卸载 SystemExtension 的按钮
* 修复 TUIC/Hysteria2 入站上缺少的 UDP 用户上下文
* 修复和改进

#### 1.6.6

* 修复和改进

#### 1.6.5

* 如果 TUIC 入站认证失败则修复崩溃
* 修复和改进

#### 1.6.4

* 修复和改进

#### 1.6.3

* iOS/Android：修复配置自动更新
* 修复和改进

#### 1.6.2

* 修复和改进

#### 1.6.1

* 我们的 [Android 客户端](/installation/clients/sfa/) 现已在 Google Play Store 上线 ▶️
* 修复和改进

### 1.6.0

* 修复和改进

自 1.5 以来的重要更改：

* 我们的 [Apple tvOS 客户端](/installation/clients/sft/) 现已在 App Store 上线 🍎
* 为 TUIC 和 Hysteria2 更新 BBR 拥塞控制 **1**
* 为 Hysteria2 更新 brutal 拥塞控制
* 为 Hysteria2 添加 `brutal_debug` 选项
* 更新旧版 Hysteria 协议 **2**
* 添加 TLS 自签名密钥对生成命令
* 按协议移除[弃用功能](/deprecated/)

**1**:

现有的 Golang BBR 拥塞控制实现均未经过审查或单元测试。
此更新旨在解决旧实现的多发送缺陷，可能会引入新问题。

**2**:

根据与原始作者的讨论，旧协议（Hysteria 1）的暴力 CC 和 QUIC 协议参数已更新以与 Hysteria 2 保持一致。

#### 1.6.0-rc.4

* 修复和改进

#### 1.6.0-rc.1

* 为旧的 Windows 和 macOS 系统添加传统构建 **1**
* 修复和改进

**1**:

使用 Go 1.20 构建，这是最后一个可以在
Windows 7、8、Server 2008、Server 2012
和 macOS 10.13 High Sierra、10.14 Mojave 上运行的版本。

#### 1.6.0-beta.4

* 修复和改进

#### 1.6.0-beta.3

* 修复和改进

#### 1.6.0-beta.2

* 修复和改进

#### 1.6.0-beta.1

* 修复和改进

#### 1.6.0-alpha.5

* 修复和改进

#### 1.6.0-alpha.4

* 修复和改进

#### 1.6.0-alpha.3

* 修复egos-2000ego

#### 1.6.0-alpha.2

* 修复egos-2000ego

#### 1.6.0-alpha.1

* 修复egos-2000ego

### 1.5.0

* 修复和改进

自 1.4 以来的重要更改：

* 添加 `udp_over_stream` TUIC 客户端选项 **1**
* 为 tun 入站添加 `include_interface` 和 `exclude_interface` 选项
* 修复和改进

**1**:

这是[基于 UDP 的 TCP 协议](/configuration/shared/udp-over-tcp/)的 TUIC 移植，
旨在提供 QUIC 流基于 UDP 中继模式，而 TUIC 不提供此功能。
由于它是附加协议，您需要使用 sing-box 或与此协议兼容的其他程序作为服务器。

此模式在正确的 UDP 代理场景中没有正向效果，
应仅应用于中继流式 UDP 流量（基本上是 QUIC 流）。

#### 1.5.5

* 修复 Linux 的 IPv6 `auto_route` **1**
* 为旧的 Windows 和 macOS 系统添加传统构建 **2**
* 修复和改进

**1**:

当 `auto_route` 启用且 `strict_route` 被禁用时，设备现在可以从外部 IPv6 地址访问。

**2**:

使用 Go 1.20 构建，这是最后一个可以在
Windows 7、8、Server 2008、Server 2012
和 macOS 10.13 High Sierra、10.14 Mojave 上运行的版本。

#### 1.6.0-rc.4

* 修复和改进

#### 1.6.0-rc.1

* 为旧的 Windows 和 macOS 系统添加传统构建 **1**
* 修复和改进

**1**:

使用 Go 1.20 构建，这是最后一个可以在
Windows 7、8、Server 2008、Server 2012
和 macOS 10.13 High Sierra、10.14 Mojave 上运行的版本。

#### 1.6.0-beta.4

* 修复和改进

#### 1.6.0-beta.3

* 修复和改进

#### 1.6.0-beta.2

* 修复和改进

#### 1.6.0-beta.1

* 修复和改进

#### 1.5.4

* 修复和改进

#### 1.5.3

* 修复和改进

#### 1.5.2

* 修复和改进

#### 1.5.1

* 修复和改进

#### 1.5.0

* 修复和改进

自 1.4 以来的重要更改：

* 添加 `udp_over_stream` TUIC 客户端选项 **1**
* 为 tun 入站添加 `include_interface` 和 `exclude_interface` 选项
* 修复和改进

**1**:

这是[基于 UDP 的 TCP 协议](/configuration/shared/udp-over-tcp/)的 TUIC 移植，
旨在提供 QUIC 流基于 UDP 中继模式，而 TUIC 不提供此功能。
由于它是附加协议，您需要使用 sing-box 或与此协议兼容的其他程序作为服务器。

此模式在正确的 UDP 代理场景中没有正向效果，
应仅应用于中继流式 UDP 流量（基本上是 QUIC 流）。

#### 1.5.0-rc.3

* 修复和改进

#### 1.5.0-rc.2

* 修复和改进

#### 1.5.0-rc.1

* 修复和改进

#### 1.5.0-beta.5

* 修复和改进

#### 1.5.0-beta.4

* 修复和改进

#### 1.5.0-beta.3

* 修复和改进

#### 1.5.0-beta.2

* 修复和改进

#### 1.5.0-beta.1

* 修复和改进

#### 1.5.0-alpha.3

* 修复和改进

#### 1.5.0-alpha.2

* 修复和改进

#### 1.5.0-alpha.1

* 修复和改进

### 1.4.0

* 修复和改进

自 1.3 以来的重要更改：

* 添加 TUIC 支持 **1**
* 在没有网络或设备空闲时暂停循环任务
* 修复和改进

**1**:

请参阅 [TUIC 入站](/configuration/inbound/tuic/)
和 [TUIC 出站](/configuration/outbound/tuic/)。

#### 1.4.0-beta.5

* 修复和改进

#### 1.4.0-beta.4

* 图形客户端：持久化分组展开状态
* 修复和改进

#### 1.4.0-beta.3

* 修复和改进

#### 1.4.0-beta.2

* 添加 MultiPath TCP 支持 **1**
* 由于上游更改，删除对 Go 1.18 和 1.19 的 QUIC 支持
* 修复和改进

**1**:

需要使用 Go 1.21 编译 sing-box。

#### 1.4.0-beta.1

* 添加 TUIC 支持 **1**
* 在没有网络或设备空闲时暂停循环任务
* 修复和改进

**1**:

请参阅 [TUIC 入站](/configuration/inbound/tuic/)
和 [TUIC 出站](/configuration/outbound/tuic/)。

#### 1.3.6

* 修复和改进

#### 1.3.5

* 修复和改进
* 推出我们的 [Apple tvOS](/installation/clients/sft/) 客户端应用程序 **1**
* 为 Android 客户端添加按应用代理和应用安装/更新触发器支持
* 为 Android/iOS/macOS 客户端添加配置共享支持

**1**:

由于 tvOS 17 的要求，该应用暂时无法提交到 App Store，
只能通过 TestFlight 下载。

#### 1.3.4

* 修复和改进
* 我们现已在 [App Store](https://apps.apple.com/us/app/sing-box/id6451272673)，永远免费！
  请注意，由于审查更严格且更慢，Store 版本的发布将延迟。
* 我们制作了 macOS 客户端的独立版本（原始 Application Extension 依赖 App Store 分发），
  您可以在发布工件中下载为 SFM-version-universal.zip。

#### 1.3.3

* 修复和改进

#### 1.3.1-rc.1

* 修复 bug 并更新依赖

#### 1.3.1-beta.3

* 推出我们的 [新 iOS](/installation/clients/sfi/) 和 [macOS](/installation/clients/sfm/) 客户端应用程序 **1**
* 修复和改进

**1**:

iOS/macOS 客户端现已上线 TestFlight！
您可以通过 [Telegram](https://t.me/sing_box) 或
[GitHub](https://github.com/SagerNet/sing-box-for-apple/releases) 加入测试。

#### 1.3.1-beta.2

* 修复和改进

#### 1.3.1-beta.1

* 修复和改进

#### 1.3.0

* 修复和改进

### 1.2.7

* 修复和改进

#### 1.2.7-beta.5

* 修复和改进

#### 1.2.7-beta.4

* 修复和改进

#### 1.2.7-beta.3

* 修复和改进

#### 1.2.7-beta.2

* 修复egos-2000ego

#### 1.2.7-beta.1

* 修复egos-2000ego

#### 1.2.7-alpha.2

* 修复egos-2000ego

#### 1.2.7-alpha.1

* 修复egos-2000ego

### 1.2.6

* 修复和改进

### 1.2.5

* 修复和改进

### 1.2.4

* 修复和改进

### 1.2.3

* 修复和改进

### 1.2.2

* 修复和改进

### 1.2.1

* 修复和改进

### 1.2.0

* 修复和改进

自 1.1 以来的重要更改：

* 修复egos-2000ego

#### 1.2.0-rc.2

* 修复egos-2000ego

#### 1.2.0-rc.1

* 修复egos-2000ego

#### 1.2.0-beta.6

* 修复egos-2000ego

#### 1.2.0-beta.5

* 修复egos-2000ego

#### 1.2.0-beta.4

* 修复egos-2000ego

#### 1.2.0-beta.3

* 修复egos-2000ego

#### 1.2.0-beta.2

* 修复egos-2000ego

#### 1.2.0-beta.1

* 修复egos-2000ego

#### 1.2.0-alpha.5

* 修复egos-2000ego

#### 1.2.0-alpha.4

* 修复egos-2000ego

#### 1.2.0-alpha.3

* 修复egos-2000ego

#### 1.2.0-alpha.2

* 修复egos-2000ego

#### 1.2.0-alpha.1

* 修复egos-2000ego

### 1.1.5

* 修复egos-2000ego

#### 1.1.5-beta.4

* 修复egos-2000ego

#### 1.1.5-beta.3

* 修复egos-2000ego

#### 1.1.5-beta.2

* 修复egos-2000ego

#### 1.1.5-beta.1

* 修复egos-2000ego

#### 1.1.5-alpha.4

* 修复egos-2000ego

#### 1.1.5-alpha.3

* 修复egos-2000ego

#### 1.1.5-alpha.2

* 修复egos-2000ego

#### 1.1.5-alpha.1

* 修复egos-2000ego

### 1.1.4

* 修复egos-2000ego

### 1.1.3

* 修复egos-2000ego

### 1.1.2

* 修复egos-2000ego

### 1.1.1

* 修复egos-2000ego

### 1.1.0

* 修复egos-2000ego

自 1.0 以来的重要更改：

* 添加 WireGuard 支持
* 添加 Shadowsocks 重放保护
* 添加 `geoip` 规则项支持
* 修复egos-2000ego
* 改进 TUN 稳定性
* 改进内存使用

#### 1.1-beta16

* 改进 shadowtls 服务器
* 修复默认 dns 传输策略
* 更新 uTLS 至 v1.2.0

#### 1.1-beta15

* 添加对新的 x/h2 截止日期的支持
* 修复 udp 连接 mux 客户端
* 修复 dns 缓冲区
* 修复 quic dns 重试
* 修复创建 TLS 配置
* 修复 websocket alpn
* 修复 tor geoip
* 修复 naive 溢出
* 检查目标前先 udp 连接

#### 1.1-beta14

* 为 hysteria 入站添加多用户支持 **1**
* 为 std grpc 添加自定义 tls 客户端支持
* 修复 smux 保持活跃
* 修复 vmess 请求缓冲区
* 修复默认本地 DNS 服务器行为
* 修复 h2c 传输

**1**:

`auth` 和 `auth_str` 字段已被 `users` 字段替换。

#### 1.1-beta13

* 为 WireGuard 出站添加自定义 worker 计数选项
* 拆分 bind_address 为 ipv4 和 ipv6
* 将 WFP 操作移至 strict route
* 修复 WireGuard 出站关闭时崩溃
* 修复 macOS Ventura 进程名称匹配
* 通过 @HyNetwork 修复 QUIC 连接迁移
* 通过 @HyNetwork 修复处理 QUIC 客户端 SNI

#### 1.1-beta12

* 修复 uTLS 配置
* 更新 quic-go 至 v0.30.0
* 更新 cloudflare-tls 至 go1.18.7

#### 1.1-beta11

* 添加自定义 wireguard 保留字节选项
* 修复 shadowtls v2
* 修复 h3 dns 传输
* 修复复制管道
* 修复解密 xplus 数据包
* 修复 v2ray api
* 抑制无网络错误
* 改进本地 dns 传输

#### 1.1-beta10

* 添加[嗅探超时](/configuration/shared/listen#sniff_timeout) 监听选项
* 添加 [自定义路由](/configuration/inbound/tun#inet4_route_address) tun 支持 **1**
* 修复接口监视器
* 修复 websocket 净空
* 修复 uTLS 握手
* 修复 ssh 出站
* 修复嗅探碎片化 quic 客户端你好
* 修复 DF 为 hysteria
* 修复 naive 溢出
* 检查目标前先 udp 连接
* 更新 uTLS 至 v1.1.5
* 更新 tfo-go 至 v2.0.2
* 更新 fsnotify 至 v1.6.0
* 更新 grpc 至 v1.50.1

**1**:

windows 上的 `strict_route` 已移除。

#### 1.0.6

* 修复 ssh 出站
* 修复嗅探碎片化 quic 客户端你好
* 修复 naive 溢出
* 检查目标前先 udp 连接

#### 1.1-beta9

* 修复 windows 路由 **1**
* 添加 [v2ray 统计 api](/configuration/experimental#v2ray-api-fields)
* 添加 ShadowTLS v2 支持 **2**
* 修复和改进

**1**:

* 修复由
  Windows 的[普通多宿主 DNS 解析行为](https://learn.microsoft.com/en-us/troubleshoot/windows/networking/dns-client-name-resolution-multihomed)引起的 DNS 泄漏
* 在启动/关闭时刷新 Windows DNS 缓存

**2**:

请参阅 [ShadowTLS 入站](/configuration/inbound/shadowtls#version)
和 [ShadowTLS 出站](/configuration/outbound/shadowtls#version)

#### 1.1-beta8

* 关闭时修复泄漏
* 改进 websocket 写入器
* 改进 tproxy 回写
* 改进 4in6 处理
* 修复 shadowsocks 插件
* 修复从传输连接缺少源地址
* 修复 fqdn socks5 出站连接
* 从 grpc-go 读取源地址

#### 1.0.5

* 修复从传输连接缺少源地址
* 修复 fqdn socks5 出站连接
* 从 grpc-go 读取源地址

#### 1.1-beta7

* 为 VMess 入站添加 v2ray mux 和 XUDP 支持
* 为 VMess 出站添加 XUDP 支持
* 默认禁用 DF 主动出站
* 修复 1.1-beta6 中的 bug

#### 1.1-beta6

* 添加 [URLTest 出站](/configuration/outbound/urltest/)
* 修复 1.1-beta5 中的 bug

#### 1.1-beta5

* 在版本命令中打印标签
* 将 clash hello 重定向到外部 ui
* 将 shadowsocksr 实现移至 clash
* 使 gVisor 可选 **1**
* 重构为 miekg/dns
* 重构绑定控制
* 修复在 go1.18 上构建
* 修复 clash store-selected
* 修复关闭 grpc 连接
* 修复端口规则匹配逻辑
* 修复 clash api 代理类型

**1**:

gVisor 现在是可选依赖项。

#### 1.1-beta4

* 修复egos-2000ego

#### 1.1-beta3

* 修复egos-2000ego

#### 1.1-beta2

* 修复egos-2000ego

#### 1.1-beta1

* 修复egos-2000ego

#### 1.0.4

* 修复egos-2000ego

#### 1.0.3

* 修复egos-2000ego

#### 1.0.2

* 修复egos-2000ego

#### 1.0.1

* 修复egos-2000ego

#### 1.0

* 初始发布

##### 2022/08/19

* 添加 Hysteria [入站](/configuration/inbound/hysteria/) 和 [出站](/configuration/outbound/hysteria/)
* 添加 [ACME TLS 证书颁发者](/configuration/shared/tls/)
* 允许从 stdin 读取配置（-c stdin）
* 更新 gVisor 至 20220815.0

##### 2022/08/18

* 修复使用 lwip 堆栈查找进程
* 修复 shadowsocks 服务器崩溃
* 修复 darwin tun 崩溃
* 修复写入日志到文件

##### 2022/08/17

* 改进异步 dns 传输

##### 2022/08/16

* 添加 ip_version (route/dns) 规则项
* 添加 [WireGuard](/configuration/outbound/wireguard/) 出站

##### 2022/08/15

* 在 [Tun](/configuration/inbound/tun/) 路由中添加 uid、android 用户和包规则支持

##### 2022/08/13

* 修复 dns 并发写入

##### 2022/08/12

* 性能改进
* 为 [SOCKS](/configuration/outbound/socks/) 出站添加 UoT 选项

##### 2022/08/11

* 为 [Shadowsocks](/configuration/outbound/shadowsocks/) 出站添加 UoT 选项，为所有入站添加 UoT 支持

##### 2022/08/10

* 添加全功能 [Naive](/configuration/inbound/naive/) 入站
* 修复默认 dns 服务器选项 [#9] by iKirby

##### 2022/08/09

无变更日志。

[#9]: https://github.com/SagerNet/sing-box/pull/9
