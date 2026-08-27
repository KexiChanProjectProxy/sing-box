# 引言

sing-box 使用 JSON 或 YAML 作为配置文件格式。

接受 `.json`、`.yaml` 和 `.yml` 文件。`-c` 可以指向任一格式；
`-C` 会加载目录中所有 `.json`/`.yaml`/`.yml` 文件。未指定这两个参数时，
sing-box 依次尝试 `config.json`、`config.yaml`、`config.yml`。

YAML 会转换成 JSON 配置模型，因此下文记录的字段在两种格式中相同。
布尔值请使用 `true`/`false`；若字符串会被 YAML 当成布尔值，请加引号
（`"yes"`、`"no"`、`"on"`、`"off"`）。支持锚点、别名和合并键（`<<`）。
每个文件只能包含一个 YAML 文档。

### 结构

```json
{
  "$schema": "https://sing-box.sagernet.org/schema.json",
  "log": {},
  "dns": {},
  "ntp": {},
  "certificate": {},
  "certificate_providers": [],
  "http_clients": [],
  "network_namespaces": [],
  "endpoints": [],
  "inbounds": [],
  "outbounds": [],
  "route": {},
  "services": [],
  "experimental": {}
}
```

```yaml
$schema: https://sing-box.sagernet.org/schema.json
log: {}
dns: {}
ntp: {}
certificate: {}
certificate_providers: []
http_clients: []
network_namespaces: []
endpoints: []
inbounds: []
outbounds: []
route: {}
services: []
experimental: {}
```

### 字段

| Key            | Format                 |
|----------------|------------------------|
| `$schema`      | [JSON Schema](./schema/) |
| `log`          | [日志](./log/)           |
| `dns`          | [DNS](./dns/)          |
| `ntp`          | [NTP](./ntp/)          |
| `certificate`  | [证书](./certificate/)   |
| `certificate_providers` | [证书提供者](./shared/certificate-provider/) |
| `http_clients` | [HTTP 客户端](./shared/http-client/) |
| `network_namespaces` | [网络命名空间](./network-namespace/) |
| `endpoints`    | [端点](./endpoint/)      |
| `inbounds`     | [入站](./inbound/)       |
| `outbounds`    | [出站](./outbound/)      |
| `route`        | [路由](./route/)         |
| `services`     | [服务](./service/)       |
| `experimental` | [实验性](./experimental/) |

### 检查

```bash
sing-box check
```

### 格式化

```bash
sing-box format -w -c config.json -D config_directory
```

`format -w` 会按源文件扩展名写回 JSON 或 YAML。
`merge` 在输出路径以 `.yaml` 或 `.yml` 结尾时写出 YAML。

### 合并

```bash
sing-box merge output.json -c config.json -D config_directory
```

