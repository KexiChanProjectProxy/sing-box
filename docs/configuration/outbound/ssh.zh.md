!!! quote "sing-box 1.14.0 中的更改"

    :material-plus: [cipher](#cipher)  
    :material-plus: [mac](#mac)  
    :material-plus: [kex_algorithm](#kex_algorithm)

### 结构

```json
{
  "type": "ssh",
  "tag": "ssh-out",

  "server": "127.0.0.1",
  "server_port": 22,
  "server_unix": "",
  "user": "root",
  "password": "admin",
  "private_key": "",
  "private_key_path": "$HOME/.ssh/id_rsa",
  "private_key_passphrase": "",
  "host_key": [
    "ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdH..."
  ],
  "host_key_algorithms": [],
  "client_version": "SSH-2.0-OpenSSH_7.4p1",
  "cipher": [],
  "mac": [],
  "kex_algorithm": [],

  ... // 拨号字段
}
```

### 字段

#### server

==必填==

服务器地址。

设置 [server_unix](#server_unix) 时不需要。

#### server_port

服务器端口，默认使用 22。

#### server_unix

!!! quote ""

    仅支持 Linux。

要连接的 Unix 域套接字路径，用于替代 `server`:`server_port`，例如本地反向代理，或使用 [listen_unix](/zh/configuration/shared/listen/#listen_unix) 监听的另一个 sing-box。

设置后，`server` 和 `server_port` 不会被连接或解析，并变为可选。如果设置了它们，它们仍会作为 TLS `server_name` 的默认值，以及 V2Ray 传输层（如有）的默认 `Host`；否则请设置 `tls.server_name`。

套接字上只承载 TCP。如果协议在 TCP 连接内传输 UDP（例如 `udp_over_tcp`），UDP 仍可使用。

不能与 `detour`、`tcp_fast_open`、`tcp_multi_path`、`bind_interface`、`routing_mark` 或 `network_strategy` 同时使用。

#### user

SSH 用户, 默认使用 root。

#### password

密码。

#### private_key

密钥。

#### private_key_path

密钥路径。

#### private_key_passphrase

密钥密码。

#### host_key

主机密钥，留空接受所有。

#### host_key_algorithms

主机密钥算法。

#### client_version

客户端版本，默认使用随机值。

#### cipher

!!! question "自 sing-box 1.14.0 起"

允许的加密算法。留空使用默认值。

#### mac

!!! question "自 sing-box 1.14.0 起"

允许的 MAC 算法。留空使用默认值。

#### kex_algorithm

!!! question "自 sing-box 1.14.0 起"

允许的密钥交换算法。留空使用默认值。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
