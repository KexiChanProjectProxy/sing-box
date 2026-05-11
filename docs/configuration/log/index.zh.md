# 日志

### 结构

```json
{
  "log": {
    "disabled": false,
    "level": "info",
    "output": "box.log",
    "timestamp": true,
    "format": "",
    "outputs": [],
    "event_bus": {
      "enabled": false,
      "buffer_size": 0,
      "webhooks": []
    }
  }
}
```

### 字段

#### disabled

禁用日志，启动后不输出日志。

#### level

日志等级，可选值：`trace` `debug` `info` `warn` `error` `fatal` `panic`。

#### output

输出文件路径，启动后将不输出到控制台。

#### timestamp

添加时间到每行。

#### format

!!! question "自 sing-box 1.10.0 起"

日志输出格式。

可选值：`text`、`json`。

默认使用 `text`。

#### outputs

!!! question "自 sing-box 1.10.0 起"

多个日志输出列表。

```json
{
  "log": {
    "outputs": [
      {
        "type": "file",
        "format": "text",
        "path": "box.log",
        "timestamp": true,
        "disable_color": false
      },
      {
        "type": "stdout",
        "format": "text",
        "timestamp": true,
        "disable_color": false
      },
      {
        "type": "stderr",
        "format": "text",
        "timestamp": true,
        "disable_color": false
      },
      {
        "type": "http",
        "format": "json",
        "url": "",
        "jwt_token": "",
        "batch_size": 100,
        "flush_interval": "5s",
        "timeout": "10s",
        "timestamp": true,
        "hostname": "",
        "version": ""
      }
    ]
  }
}
```

##### type

输出类型。可选值：`file`、`stdout`、`stderr`、`http`。

##### format

输出格式。可选值：`text`、`json`。

##### path

文件路径。当 `type` 为 `file` 时必填。

##### url

HTTP 端点 URL。当 `type` 为 `http` 时必填。

##### jwt_token

HTTP Webhook 认证的 JWT 令牌。

##### batch_size

发送前批处理的日志条目数量。默认值：`100`。

##### flush_interval

刷新待处理日志条目的间隔。默认值：`5s`。

##### timeout

HTTP 请求超时时间。默认值：`10s`。

##### timestamp

在每行添加时间。

##### disable_color

为文本格式禁用颜色输出。

##### hostname

HTTP Webhook 负载中包含的主机名。

##### version

HTTP Webhook 负载中包含的 sing-box 版本。

#### event_bus

!!! question "自 sing-box 1.10.0 起"

用于结构化日志事件的 Event Bus 配置。

##### enabled

启用 Event Bus。

##### buffer_size

Event Bus 缓冲区大小。默认值：`0`（无缓冲）。

##### webhooks

Webhook 订阅者列表。

###### url

Webhook 端点 URL。

###### headers

随 Webhook 请求发送的自定义 HTTP 头。

###### batch_size

发送前批处理的事件数量。

###### flush_interval

刷新待处理事件的间隔。

###### timeout

Webhook 请求超时时间。
