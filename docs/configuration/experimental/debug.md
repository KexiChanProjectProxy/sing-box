!!! question "Since sing-box 1.10.0"

### Structure

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

### Fields

#### listen

Debug HTTP server listening address.

If not set, the debug HTTP server will not start.

#### gc_percent

Sets `GOGC` for the debug HTTP server. Default: `0` (use Go runtime default).

#### max_stack

Sets `maxstack` for the debug HTTP server. Default: `0` (use Go runtime default).

#### max_threads

Sets `maxthread` for the debug HTTP server. Default: `0` (use Go runtime default).

#### panic_on_fault

Controls whether the debug HTTP server should panic on fault.

#### trace_back

Sets `traceback` for the debug HTTP server.

#### memory_limit

Sets `memorylimit` for the debug HTTP server.

Supports suffixes like `KiB`, `MiB`, `GiB` (powers of 1024), or `KB`, `MB`, `GB` (powers of 1000).

#### oom_killer

Enables the out-of-memory killer for the debug HTTP server.
