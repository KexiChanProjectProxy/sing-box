# 日志

### 结构

```json
{
  "log": {
    "disabled": false,
    "level": "info",
    "output": "box.log",
    "format": "json",
    "timestamp": true
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

#### format

日志格式。为空或未指定时，输出纯文本日志。设置为 `json` 时，输出结构化 JSON 日志。
无效值将被拒绝。

#### timestamp

添加时间到每行。