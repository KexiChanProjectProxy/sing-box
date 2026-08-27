# Introduction

sing-box uses JSON or YAML for configuration files.

`.json`, `.yaml`, and `.yml` files are accepted. `-c` can point at either format;
`-C` loads every `.json`/`.yaml`/`.yml` file in the directory. If neither flag is
set, sing-box tries `config.json`, then `config.yaml`, then `config.yml`.

YAML is converted to the JSON configuration model, so every field documented
below is the same in both formats. Use `true`/`false` for booleans, and quote
strings that YAML would otherwise treat as booleans (`"yes"`, `"no"`, `"on"`,
`"off"`). Anchors, aliases, and merge keys (`<<`) are supported. A file must
contain a single YAML document.

### Structure

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

### Fields

| Key            | Format                          |
|----------------|---------------------------------|
| `$schema`      | [JSON Schema](./schema/)        |
| `log`          | [Log](./log/)                   |
| `dns`          | [DNS](./dns/)                   |
| `ntp`          | [NTP](./ntp/)                   |
| `certificate`  | [Certificate](./certificate/)   |
| `certificate_providers` | [Certificate Provider](./shared/certificate-provider/) |
| `http_clients` | [HTTP Client](./shared/http-client/) |
| `network_namespaces` | [Network Namespace](./network-namespace/) |
| `endpoints`    | [Endpoint](./endpoint/)         |
| `inbounds`     | [Inbound](./inbound/)           |
| `outbounds`    | [Outbound](./outbound/)         |
| `route`        | [Route](./route/)               |
| `services`     | [Service](./service/)           |
| `experimental` | [Experimental](./experimental/) |

### Check

```bash
sing-box check
```

### Format

```bash
sing-box format -w -c config.json -D config_directory
```

`format -w` writes JSON or YAML to match the source file extension.
`merge` writes YAML when the output path ends with `.yaml` or `.yml`.

### Merge

```bash
sing-box merge output.json -c config.json -D config_directory
```
