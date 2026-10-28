!!! quote "Changes in sing-box 1.14.0"

    :material-plus: [cipher](#cipher)  
    :material-plus: [mac](#mac)  
    :material-plus: [kex_algorithm](#kex_algorithm)

### Structure

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

  ... // Dial Fields
}
```

### Fields

#### server

==Required==

Server address.

Not required when [server_unix](#server_unix) is set.

#### server_port

Server port. 22 will be used if empty.

#### server_unix

!!! quote ""

    Only supported on Linux.

Path of a Unix domain socket to connect to instead of `server`:`server_port`, such as a local reverse proxy or another sing-box listening with [listen_unix](/configuration/shared/listen/#listen_unix).

When set, `server` and `server_port` are never dialed or resolved and become optional. If they are set, they are still used as the default TLS `server_name` and as the default `Host` of the V2Ray transport, if any. Otherwise set `tls.server_name`.

Only TCP is carried over the socket. UDP keeps working where the protocol carries it inside the TCP connection, for example with `udp_over_tcp`.

Cannot be used with `detour`, `tcp_fast_open`, `tcp_multi_path`, `bind_interface`, `routing_mark` or `network_strategy`.

#### user

SSH user, root will be used if empty.

#### password

Password.

#### private_key

Private key.

#### private_key_path

Private key path.

#### private_key_passphrase

Private key passphrase.

#### host_key

Host key. Accept any if empty.

#### host_key_algorithms

Host key algorithms.

#### client_version

Client version. Random version will be used if empty.

#### cipher

!!! question "Since sing-box 1.14.0"

Allowed ciphers. Default values are used if empty.

#### mac

!!! question "Since sing-box 1.14.0"

Allowed MAC algorithms. Default values are used if empty.

#### kex_algorithm

!!! question "Since sing-box 1.14.0"

Allowed key exchange algorithms. Default values are used if empty.

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
