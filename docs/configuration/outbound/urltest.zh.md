### 结构

```json
{
  "type": "urltest",
  "tag": "auto",

  "outbounds": [
    "proxy-a",
    "proxy-b",
    "proxy-c"
  ],
  "url": "",
  "interval": "",
  "tolerance": 50,
  "idle_timeout": "",
  "interrupt_exist_connections": false,
  "prefer_domain": true
}
```

### 字段

#### outbounds

==必填==

用于测试的出站标签列表。

#### url

用于测试的链接。默认使用 `https://www.gstatic.com/generate_204`。

#### interval

测试间隔。 默认使用 `3m`。

#### tolerance

以毫秒为单位的测试容差。 默认使用 `50`。

#### idle_timeout

空闲超时。默认使用 `30m`。

#### interrupt_exist_connections

当选定的出站发生更改时，中断现有连接。

仅入站连接受此设置影响，内部连接将始终被中断。

#### prefer_domain

与 dial 字段中的 [prefer_domain](/zh/configuration/shared/dial/#prefer_domain) 选项相同。

在组出站上启用时，IP 目标会被嗅探的域名覆盖，然后将连接传递给选定的子出站。