### 结构

```json
{
  "type": "trojan",
  "tag": "trojan-out",
  
  "server": "127.0.0.1",
  "server_port": 1080,
  "server_unix": "",
  "password": "8JCsPssfgS8tiRwiMlhARg==",
  "network": "tcp",
  "tls": {},
  "multiplex": {},
  "transport": {},

  ... // 拨号字段
}
```

### 字段

#### server

==必填==

服务器地址。

设置 [server_unix](#server_unix) 时不需要。

#### server_port

==必填==

服务器端口。

设置 [server_unix](#server_unix) 时不需要。

#### server_unix

!!! quote ""

    仅支持 Linux。

要连接的 Unix 域套接字路径，用于替代 `server`:`server_port`，例如本地反向代理，或使用 [listen_unix](/zh/configuration/shared/listen/#listen_unix) 监听的另一个 sing-box。

设置后，`server` 和 `server_port` 不会被连接或解析，并变为可选。如果设置了它们，它们仍会作为 TLS `server_name` 的默认值，以及 V2Ray 传输层（如有）的默认 `Host`；否则请设置 `tls.server_name`。

套接字上只承载 TCP。如果协议在 TCP 连接内传输 UDP（例如 `udp_over_tcp`），UDP 仍可使用。

不能与 `detour`、`tcp_fast_open`、`tcp_multi_path`、`bind_interface`、`routing_mark` 或 `network_strategy` 同时使用。

#### password

==必填==

Trojan 密码。

#### network

启用的网络协议。

`tcp` 或 `udp`。

默认所有。

#### tls

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#出站)。

#### multiplex

参阅 [多路复用](/zh/configuration/shared/multiplex#出站)。

#### transport

V2Ray 传输配置，参阅 [V2Ray 传输层](/zh/configuration/shared/v2ray-transport/)。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
