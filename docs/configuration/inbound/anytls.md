---
icon: material/new-box
---

!!! quote "Changes in sing-box 1.14.2.11"

    :material-plus: [dns](#dns)  
    :material-plus: [probe](#probe)

!!! quote "Changes in sing-box 1.14.2.2"

    :material-plus: [fallback](#fallback)

!!! question "Since sing-box 1.12.0"

### Structure

```json
{
  "type": "anytls",
  "tag": "anytls-in",

  ... // Listen Fields

  "users": [
    {
      "name": "sekai",
      "password": "8JCsPssfgS8tiRwiMlhARg=="
    }
  ],
  "padding_scheme": [],
  "tls": {},
  "fallback": {
    "server": "127.0.0.1",
    "server_port": 8080
  },
  "dns": {
    "enabled": false,
    "server": ""
  },
  "probe": {
    "disabled": false,
    "max_bytes": 33554432,
    "max_duration": "5s",
    "cooldown": "60s",
    "max_concurrent": 4
  }
}
```

### Listen Fields

See [Listen Fields](/configuration/shared/listen/) for details.

### Fields

#### users

==Required==

AnyTLS users.

#### padding_scheme

AnyTLS padding scheme line array.

Default padding scheme:

```json
[
  "stop=8",
  "0=30-30",
  "1=100-400",
  "2=400-500,c,500-1000,c,500-1000,c,500-1000,c,500-1000",
  "3=9-9,500-1000",
  "4=500-1000",
  "5=500-1000",
  "6=500-1000",
  "7=500-1000"
]
```

#### tls

TLS configuration, see [TLS](/configuration/shared/tls/#inbound).

If empty, the inbound accepts plaintext AnyTLS connections. Use this only behind a reverse proxy that terminates TLS in front of sing-box, such as nginx `stream` or HAProxy in TCP mode.

#### fallback

!!! question "Since sing-box 1.14.2.2"

Fallback server configuration. Disabled if empty.

Connections that fail AnyTLS authentication, including those with an unknown password or a malformed first packet, are forwarded to this server with their original bytes. With `tls` enabled, the server receives the decrypted stream.

TLS handshake failures are not forwarded.

#### dns

!!! question "Since sing-box 1.14.2.11"

Answer DNS queries that clients send inside their AnyTLS session.

A client sends each query as a raw DNS message in a frame of its own, without opening a stream.
The server offers this in its session settings, so only clients that support it send queries, and other AnyTLS clients are unaffected.
On the client, use an [AnyTLS DNS server](/configuration/dns/server/anytls/).

##### dns.enabled

Answer DNS queries inside sessions.

##### dns.server

The tag of the [DNS server](/configuration/dns/server/) that answers the queries.

Queries go to this server directly and skip the DNS rules, but share its cache with every other query sent to it, including the ones sing-box makes for itself.
EDNS options that only concern the client's link, such as cookies and padding, are removed first so that this works; a client subnet is kept.
A [group DNS server](/configuration/dns/server/group/) is the usual choice.

If empty, queries go through the [DNS rules](/configuration/dns/rule/).
Rules can match them with `inbound`, `auth_user` and `source_ip_cidr`, which are set to this inbound, the AnyTLS user and the client address.

#### probe

!!! question "Since sing-box 1.14.2.11"

Limits for the throughput probes clients ask for. Probes are on unless disabled.

A client measures the path to this server by asking it to fill one session with filler for a few seconds (a download), or by sending filler itself (an upload).
The sender then has more to send than the path carries, so its TCP_INFO describes the path rather than the application.
The session is taken out of the client's pool for the probe, as a stream would take it.

A download stops early once the sending kernel's bandwidth estimate (BBR's own, or else the delivery rate of samples the application did not limit) has grown by less than 25% for three round trips, the test BBR uses to leave startup.
The server then waits for the filler to be acknowledged and sends its measurement:

| Key | Meaning |
|---|---|
| `goodput_bps` | Bytes acknowledged per second of time the sender was limited by the network, not by the receiver's window or its own buffer. The figure to rank paths on. |
| `net_limited_us`, `acked_bytes` | The two parts of `goodput_bps`. The time is counted in kernel ticks of 1 to 10 ms. |
| `bbr_bw_bps`, `peak_bw_bps` | BBR's bandwidth estimate at the end and the highest estimate seen. It reads high on paths that deliver in bursts. |
| `srtt_us`, `min_rtt_us` | Smoothed RTT at the end, under load, and the lowest RTT, without queueing. Their difference is the queueing delay the probe built up. |
| `sent_bytes`, `retrans_bytes`, `dsack_dups`, `rtos` | Bytes sent and retransmitted, retransmissions the receiver already had, and retransmission timeouts. |
| `received_bytes`, `rcv_rtt_us` | For an upload, what the server received. |

Rates are in bytes per second and durations in microseconds. A key the server's kernel does not report is left out.

Probe filler counts towards the user's traffic: it is passed to the traffic statistics and connection logs as one connection to `probe.anytls.arpa` through the outbound `anytls-probe`, which does not exist.

The server offers probes in its session settings, so other AnyTLS clients are unaffected.

##### probe.disabled

Do not offer probes.

##### probe.max_bytes

The most filler one probe moves, in bytes.

`33554432` (32 MiB) is used by default.

##### probe.max_duration

The longest one probe runs.

`5s` is used by default.

##### probe.cooldown

How long a session waits between two probes in the same direction.

`60s` is used by default.

##### probe.max_concurrent

The most probes running at once across this inbound. Others are refused as busy.

`4` is used by default.
