---
icon: material/new-box
---

!!! question "Since sing-box 1.14.0"

# ClickHouse

ClickHouse service inserts one row per TCP/UDP/TUN session into a ClickHouse table.

Rows are queued off the data path and flushed with `PrepareBatch` + `Append` + `Send`. Delivery is unreliable: a failed insert drops that batch, and a full queue drops new events. DNS hijack sessions are not recorded. Process logs from the `log` block are not forwarded.

Native protocol (`host:9000`) is the default. HTTP (`host:8123`) is optional.

### Structure

```json
{
  "type": "clickhouse",
  "tag": "gw-01",
  "server": "127.0.0.1:9000",
  "database": "logs",
  "table": "sessions",
  "username": "default",
  "password": "",
  "protocol": "native",
  "tls": {},
  "detour": "",
  "batch": {
    "max_entries": 100,
    "max_wait": "1s"
  }
}
```

### Fields

#### tag

Node name written to the `node` column. Identifies this instance.

#### server

==Required==

ClickHouse address, `host:port`.

Default ports:

* native: `9000`
* native + TLS: `9440`
* http: `8123`
* http + TLS: `8443`

#### database

ClickHouse database. The server default is used if empty.

#### table

==Required==

Destination table. Must already exist; sing-box does not create it.

#### username / password

ClickHouse authentication.

#### protocol

ClickHouse transport.

Values:

* `native` (default)
* `http`

#### tls

TLS configuration, see [TLS](/configuration/shared/tls/#outbound).

Required for HTTPS (`protocol: http` with `tls.enabled`).

#### detour

Outbound tag used to reach ClickHouse. The default outbound is used if empty.

#### batch.max_entries

Flush when this many sessions are queued. `100` if empty.

#### batch.max_wait

Flush after this idle interval. `1s` if empty.

### Table

```sql
CREATE TABLE logs.sessions
(
    node                LowCardinality(String),
    id                  String,
    start               DateTime64(3),
    end                 DateTime64(3),
    duration_ms         Int64,
    action              LowCardinality(String),
    network             LowCardinality(String),
    protocol            LowCardinality(String),
    user                String,
    source_ip           String,
    source_port         UInt16,
    source_mac          String,
    destination_domain  String,
    destination_ip      String,
    destination_port    UInt16,
    inbound             LowCardinality(String),
    inbound_type        LowCardinality(String),
    outbound            LowCardinality(String),
    outbound_type       LowCardinality(String),
    chain               Array(String),
    rule                String,
    upload              Int64,
    download            Int64,
    close               LowCardinality(String),
    process             String
)
ENGINE = MergeTree
ORDER BY (node, start, id)
```

`action` is `allow` or `reject`. `close` is `fin`, `rst`, `timeout`, `reject`, or `drop` when known. Failed inserts are logged as `clickhouse.push_failed`. A full queue logs `clickhouse.dropped`.
