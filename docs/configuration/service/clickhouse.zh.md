---
icon: material/new-box
---

!!! question "自 sing-box 1.14.0 起"

# ClickHouse

ClickHouse 服务把每条 TCP/UDP/TUN 会话写成一行插入 ClickHouse 表。

转发路径只入队，后台用 `PrepareBatch` + `Append` + `Send` 批量写入。送达不可靠：插入失败丢该批，队列满丢新事件。DNS hijack 不记录。`log` 块里的进程日志不会写入。

使用 native 协议（`host:9000`）。

### 结构

```json
{
  "type": "clickhouse",
  "tag": "gw-01",
  "server": "127.0.0.1:9000",
  "database": "logs",
  "table": "sessions",
  "username": "default",
  "password": "",
  "secure": false,
  "detour": "",
  "batch": {
    "max_entries": 100,
    "max_wait": "1s"
  }
}
```

### 字段

#### tag

写入 `node` 列的节点名，用来区分实例。

#### server

==必填==

ClickHouse native 地址，`host:port`。省略端口时用 `9000`。

#### database

数据库。为空则用服务器默认库。

#### table

==必填==

目标表。表必须事先建好，sing-box 不会建表。

#### username / password

ClickHouse 认证。

#### secure

native 连接启用 TLS（常见端口 `9440`）。

#### detour

访问 ClickHouse 的出站 tag。为空则走默认出站。

#### batch.max_entries

队列达到该条数时刷新。默认 `100`。

#### batch.max_wait

空闲超过该时间后刷新。默认 `1s`。

### 表

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

`action` 为 `allow` 或 `reject`。`close` 在能判断时为 `fin`、`rst`、`timeout`、`reject` 或 `drop`。失败记 `clickhouse.push_failed`。队列满记 `clickhouse.dropped`。
