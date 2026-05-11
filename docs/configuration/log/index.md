# Log

### Structure

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

### Fields

#### disabled

Disable logging, no output after start.

#### level

Log level. One of: `trace` `debug` `info` `warn` `error` `fatal` `panic`.

#### output

Output file path. Will not write log to console after enable.

#### timestamp

Add time to each line.

#### format

!!! question "Since sing-box 1.10.0"

Log output format.

One of: `text`, `json`.

`text` will be used by default.

#### outputs

!!! question "Since sing-box 1.10.0"

List of multiple log outputs.

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

Output type. One of: `file`, `stdout`, `stderr`, `http`.

##### format

Output format. One of: `text`, `json`.

##### path

File path. Required when `type` is `file`.

##### url

HTTP endpoint URL. Required when `type` is `http`.

##### jwt_token

JWT token for HTTP webhook authentication.

##### batch_size

Number of log entries to batch before sending. Default: `100`.

##### flush_interval

Interval to flush pending log entries. Default: `5s`.

##### timeout

HTTP request timeout. Default: `10s`.

##### timestamp

Add time to each line.

##### disable_color

Disable color output for text format.

##### hostname

Hostname to include in HTTP webhook payload.

##### version

sing-box version to include in HTTP webhook payload.

#### event_bus

!!! question "Since sing-box 1.10.0"

Event bus configuration for structured log events.

##### enabled

Enable event bus.

##### buffer_size

Event bus buffer size. Default: `0` (unbuffered).

##### webhooks

List of webhook subscribers.

###### url

Webhook endpoint URL.

###### headers

Custom HTTP headers to send with webhook requests.

###### batch_size

Number of events to batch before sending.

###### flush_interval

Interval to flush pending events.

###### timeout

Webhook request timeout.
