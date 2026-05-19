---
icon: material/send
---

# Shadowsocks

Shadowsocks 是最知名的国产代理协议。
它存在多个版本，但只推荐使用 AEAD 2022 加密算法的 TCP 多路复用。

| 加密算法        | 规范                                                    | 密码学安全 | 抵抗被动检测 | 抵抗主动探测 |
|----------------|---------------------------------------------------------|:----------:|:------------:|:------------:|
| 流式加密        | [shadowsocks.org](https://shadowsocks.org/doc/stream.html) | :material-alert: | :material-alert: | :material-alert: |
| AEAD           | [shadowsocks.org](https://shadowsocks.org/doc/aead.html) | :material-check: | :material-alert: | :material-alert: |
| AEAD 2022      | [shadowsocks.org](https://shadowsocks.org/doc/sip022.html) | :material-check: | :material-check: | :material-help: |

（我们强烈建议使用多路复用通过 TCP 发送 UDP 流量，因为不这样做容易受到被动检测。）

## :material-text-box-check: 密码生成器

| 用于 `2022-blake3-aes-128-gcm` 加密算法 | 用于其他加密算法         | 操作                                                            |
|--------------------------------------|-------------------------------|-----------------------------------------------------------------|
| <code id="password_16"><code>        | <code id="password_32"><code> | <button class="md-button" onclick="generate()">刷新</button> |

<script>
    function generatePassword(element, length) {
        const array = new Uint8Array(length);
        window.crypto.getRandomValues(array);
        document.getElementById(element).textContent = btoa(String.fromCharCode.apply(null, array));
    }
    function generate() {
      generatePassword("password_16", 16);
      generatePassword("password_32", 32);
    }
    generate();
</script>

## :material-server: 服务端示例

=== ":material-account: 单用户"

    ```json
     {
      "inbounds": [
        {
          "type": "shadowsocks",
          "listen": "::",
          "listen_port": 8080,
          "network": "tcp",
          "method": "2022-blake3-aes-128-gcm",
          "password": "<password>",
          "multiplex": {
            "enabled": true
          }
        }
      ]
    }
    ```

=== ":material-account-multiple: 多用户"

    ```json
     {
      "inbounds": [
        {
          "type": "shadowsocks",
          "listen": "::",
          "listen_port": 8080,
          "network": "tcp",
          "method": "2022-blake3-aes-128-gcm",
          "password": "<server_password>",
          "users": [
            {
              "name": "sekai",
              "password": "<user_password>"
            }
          ],
          "multiplex": {
            "enabled": true
          }
        }
      ]
    }
    ```

## :material-cellphone-link: 客户端示例

=== ":material-account: 单用户"

    ```json
    {
      "outbounds": [
        {
          "type": "shadowsocks",
          "server": "127.0.0.1",
          "server_port": 8080,
          "method": "2022-blake3-aes-128-gcm",
          "password": "<pasword>",
          "multiplex": {
            "enabled": true
          }
        }
      ]
    }
    ```

=== ":material-account-multiple: 多用户"

    ```json
    {
      "outbounds": [
        {
          "type": "shadowsocks",
          "server": "127.0.0.1",
          "server_port": 8080,
          "method": "2022-blake3-aes-128-gcm",
          "password": "<server_pasword>:<user_password>",
          "multiplex": {
            "enabled": true
          }
        }
      ]
    }
    ```