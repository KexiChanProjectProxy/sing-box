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

Log format. Optional. Only `json` is accepted. When omitted, logs are written as JSONL (one JSON object per line).
Invalid values are rejected. `text` has been removed.

#### timestamp

Add time to each line.