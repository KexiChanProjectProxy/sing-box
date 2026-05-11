!!! question "自 sing-box 1.10.0 起"

### 结构

```json
{
  "debug": {
    "listen": "",
    "gc_percent": 0,
    "max_stack": 0,
    "max_threads": 0,
    "panic_on_fault": false,
    "trace_back": "",
    "memory_limit": "",
    "oom_killer": false
  }
}
```

### 字段

#### listen

调试 HTTP 服务器监听地址。

如果未设置，调试 HTTP 服务器将不会启动。

#### gc_percent

为调试 HTTP 服务器设置 `GOGC`。默认值：`0`（使用 Go 运行时默认值）。

#### max_stack

为调试 HTTP 服务器设置 `maxstack`。默认值：`0`（使用 Go 运行时默认值）。

#### max_threads

为调试 HTTP 服务器设置 `maxthread`。默认值：`0`（使用 Go 运行时默认值）。

#### panic_on_fault

控制调试 HTTP 服务器是否应在故障时 panic。

#### trace_back

为调试 HTTP 服务器设置 `traceback`。

#### memory_limit

为调试 HTTP 服务器设置 `memorylimit`。

支持后缀如 `KiB`、`MiB`、`GiB`（1024 的幂），或 `KB`、`MB`、`GB`（1000 的幂）。

#### oom_killer

为调试 HTTP 服务器启用内存不足杀手。
