---
icon: material/new-box
---

!!! quote "Changes in sing-box 1.14.0"

    :material-alert: [netns](#netns)

!!! quote "Changes in sing-box 1.13.0"

    :material-plus: [disable_tcp_keep_alive](#disable_tcp_keep_alive)  
    :material-alert: [tcp_keep_alive](#tcp_keep_alive)

!!! quote "Changes in sing-box 1.12.0"

    :material-plus: [netns](#netns)  
    :material-plus: [bind_interface](#bind_interface)  
    :material-plus: [routing_mark](#routing_mark)  
    :material-plus: [reuse_addr](#reuse_addr)

!!! quote "Changes in sing-box 1.11.0"

    :material-delete-clock: [sniff](#sniff)  
    :material-delete-clock: [sniff_override_destination](#sniff_override_destination)  
    :material-delete-clock: [sniff_timeout](#sniff_timeout)  
    :material-delete-clock: [domain_strategy](#domain_strategy)  
    :material-delete-clock: [udp_disable_domain_unmapping](#udp_disable_domain_unmapping)

### Structure

```json
{
  "listen": "",
  "listen_port": 0,
  "listen_unix": "",
  "bind_interface": "",
  "routing_mark": 0,
  "reuse_addr": false,
  "netns": "",
  "tcp_fast_open": false,
  "tcp_multi_path": false,
  "disable_tcp_keep_alive": false,
  "tcp_keep_alive": "",
  "tcp_keep_alive_interval": "",
  "udp_fragment": false,
  "udp_timeout": "",
  "detour": "",

  // Deprecated
  
  "sniff": false,
  "sniff_override_destination": false,
  "sniff_timeout": "",
  "domain_strategy": "",
  "udp_disable_domain_unmapping": false
}
```

### Fields

#### listen

==Required==

Listen address.

Not required when [listen_unix](#listen_unix) is set.

#### listen_port

Listen port.

#### listen_unix

!!! quote ""

    Only supported on Linux.

Path of a Unix domain socket to listen on instead of `listen`:`listen_port`.

Cannot be used with `listen` or `listen_port`. An existing socket file at the path is replaced at startup, and the file is removed when the inbound closes. A path that exists and is not a socket is an error. The socket file is created with the process umask.

Only TCP is accepted. An inbound that would also listen on UDP, such as `shadowsocks`, only listens on TCP and logs a warning. UDP-only inbounds such as `hysteria`, `hysteria2` and `tuic`, and SOCKS `UDP ASSOCIATE`, are not supported. UDP still works where the protocol carries it inside the TCP connection.

Connections have no source address, so rules on `source_ip_cidr`, `source_port` or the process do not match them.

Cannot be used with `tcp_fast_open`, `tcp_multi_path`, `bind_interface`, `routing_mark`, `reuse_addr` or `set_system_proxy`. `netns` is supported.

#### bind_interface

!!! question "Since sing-box 1.12.0"

The network interface to bind to.

#### routing_mark

!!! question "Since sing-box 1.12.0"

!!! quote ""

    Only supported on Linux.

Set netfilter routing mark.

Integers (e.g. `1234`) and string hexadecimals (e.g. `"0x1234"`) are supported.

#### reuse_addr

!!! question "Since sing-box 1.12.0"

Reuse listener address.

#### netns

!!! question "Since sing-box 1.12.0"

!!! quote ""

    Only supported on Linux.

Set network namespace, name or path.

Since sing-box 1.14.0, the tag of a [network namespace](/configuration/network-namespace/) can also be used.

#### tcp_fast_open

Enable TCP Fast Open.

#### tcp_multi_path

!!! warning ""

    Go 1.21 required.

Enable TCP Multi Path.

#### disable_tcp_keep_alive

!!! question "Since sing-box 1.13.0"

Disable TCP keep alive.

#### tcp_keep_alive

!!! question "Since sing-box 1.13.0"

    Default value changed from `10m` to `5m`.

TCP keep alive initial period.

`5m` will be used by default.

#### tcp_keep_alive_interval

TCP keep alive interval.

`75s` will be used by default.

#### udp_fragment

Enable UDP fragmentation.

#### udp_timeout

UDP NAT expiration time.

`5m` will be used by default.

#### detour

If set, connections will be forwarded to the specified inbound.

Requires target inbound support, see [Injectable](/configuration/inbound/#fields).

#### sniff

!!! failure "Deprecated in sing-box 1.11.0"

    Inbound fields are deprecated and will be removed in sing-box 1.13.0, check [Migration](/migration/#migrate-legacy-inbound-fields-to-rule-actions).

Enable sniffing.

See [Protocol Sniff](/configuration/route/sniff/) for details.

#### sniff_override_destination

!!! failure "Deprecated in sing-box 1.11.0"

    Inbound fields are deprecated and will be removed in sing-box 1.13.0.

Override the connection destination address with the sniffed domain.

If the domain name is invalid (like tor), this will not work.

#### sniff_timeout

!!! failure "Deprecated in sing-box 1.11.0"

    Inbound fields are deprecated and will be removed in sing-box 1.13.0, check [Migration](/migration/#migrate-legacy-inbound-fields-to-rule-actions).

Timeout for sniffing.

`300ms` is used by default.

#### domain_strategy

!!! failure "Deprecated in sing-box 1.11.0"

    Inbound fields are deprecated and will be removed in sing-box 1.13.0, check [Migration](/migration/#migrate-legacy-inbound-fields-to-rule-actions).

One of `prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`.

If set, the requested domain name will be resolved to IP before routing.

If `sniff_override_destination` is in effect, its value will be taken as a fallback.

#### udp_disable_domain_unmapping

!!! failure "Deprecated in sing-box 1.11.0"

    Inbound fields are deprecated and will be removed in sing-box 1.13.0, check [Migration](/migration/#migrate-legacy-inbound-fields-to-rule-actions).

If enabled, for UDP proxy requests addressed to a domain, 
the original packet address will be sent in the response instead of the mapped domain.

This option is used for compatibility with clients that 
do not support receiving UDP packets with domain addresses, such as Surge.
