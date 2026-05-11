---
icon: material/horse
---

# Trojan

Trojan 是最常用的中国制造 TLS 代理。它可以与多种组合配合使用。

| 协议和实现组合                | 规范                                                              | 抵抗被动检测 | 抵抗主动探测 |
|-----------------------------|------------------------------------------------------------------|-------------|-------------|
| Origin / trojan-gfw         | [trojan-gfw.github.io](https://trojan-gfw.github.io/trojan/protocol) | :material-check: | :material-check: |
| 基础 Go 实现                 | /                                                                 | :material-alert: | :material-check: |
| 配合 V2Ray 私有传输          | 无正式定义                                                        | :material-alert: | :material-alert: |
| 配合 uTLS                   | 无正式定义                                                        | :material-help: | :material-check: |

## :material-text-box-check: 密码生成器

| 生成密码                 | 操作                                                           |
|------------------------|----------------------------------------------------------------|
| <code id="password"><code> | <button class="md-button" onclick="generate()">刷新</button> |

<script>
    function generate() {
        const array = new Uint8Array(16);
        window.crypto.getRandomValues(array);
        document.getElementById("password").textContent = btoa(String.fromCharCode.apply(null, array));
    }
    generate();
</script>

## :material-server: 服务器示例

=== ":material-harddisk: 使用本地证书"

    ```json
    {
      "inbounds": [
        {
          "type": "trojan",
          "listen": "::",
          "listen_port": 8080,
          "users": [
            {
              "name": "example",
              "password": "password"
            }
          ],
          "tls": {
            "enabled": true,
            "server_name": "example.org",
            "key_path": "/path/to/key.pem",
            "certificate_path": "/path/to/certificate.pem"
          },
          "multiplex": {
            "enabled": true
          }
        }
      ]
    }
    ```

=== ":material-auto-fix: 使用 ACME"

    ```json
    {
      "inbounds": [
        {
          "type": "trojan",
          "listen": "::",
          "listen_port": 8080,
          "users": [
            {
              "name": "example",
              "password": "password"
            }
          ],
          "tls": {
            "enabled": true,
            "server_name": "example.org",
            "acme": {
              "domain": "example.org",
              "email": "admin@example.org"
            }
          },
          "multiplex": {
            "enabled": true
          }
        }
      ]
    }
    ```

=== ":material-cloud: 使用 ACME 和 Cloudflare API"

    ```json
    {
      "inbounds": [
        {
          "type": "trojan",
          "listen": "::",
          "listen_port": 8080,
          "users": [
            {
              "name": "example",
              "password": "password"
            }
          ],
          "tls": {
            "enabled": true,
            "server_name": "example.org",
            "acme": {
              "domain": "example.org",
              "email": "admin@example.org",
              "dns01_challenge": {
                "provider": "cloudflare",
                "api_token": "my_token"
              }
            }
          },
          "multiplex": {
            "enabled": true
          }
        }
      ]
    }
    ```

## :material-cellphone-link: 客户端示例

=== ":material-web-check: 使用有效证书"

    ```json
    {
      "outbounds": [
        {
          "type": "trojan",
          "server": "127.0.0.1",
          "server_port": 8080,
          "password": "password",
          "tls": {
            "enabled": true,
            "server_name": "example.org"
          },
          "multiplex": {
            "enabled": true
          }
        }
      ]
    }
    ```

=== ":material-check: 使用自签名证书"

    !!! info "提示"

        使用 `sing-box merge` 命令将配置和证书合并为一个文件。

    ```json
    {
      "outbounds": [
        {
          "type": "trojan",
          "server": "127.0.0.1",
          "server_port": 8080,
          "password": "password",
          "tls": {
            "enabled": true,
            "server_name": "example.org",
            "certificate_path": "/path/to/certificate.pem"
          },
          "multiplex": {
            "enabled": true
          }
        }
      ]
    }
    ```

=== ":material-alert: 忽略证书验证"

    ```json
    {
      "outbounds": [
        {
          "type": "trojan",
          "server": "127.0.0.1",
          "server_port": 8080,
          "password": "password",
          "tls": {
            "enabled": true,
            "server_name": "example.org",
            "insecure": true
          },
          "multiplex": {
            "enabled": true
          }
        }
      ]
    }
    ```
