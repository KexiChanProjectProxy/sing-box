# Log

### Structure

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

### Fields

#### disabled

Disable logging, no output after start.

#### level

Log level. One of: `trace` `debug` `info` `warn` `error` `fatal` `panic`.

#### output

Output file path. Will not write log to console after enable.

#### format

Log format. When empty or not specified, outputs plain text logs. When set to `json`, outputs structured JSON logs.
Invalid values are rejected.

#### timestamp

Add time to each line.