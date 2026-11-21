---
icon: material/new-box
---

!!! quote "Changes in sing-box 1.14.4.3"

    :material-plus: [non_tls_handler](#non_tls_handler)

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
  },
  "non_tls_handler": ""
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

Connections that do not start with a TLS ClientHello go to [non_tls_handler](#non_tls_handler) when it is set, not here.

#### non_tls_handler

!!! question "Since sing-box 1.14.4.3"

The tag of a [Shadowsocks inbound](/configuration/inbound/shadowsocks/) that takes the connections whose first bytes are not a TLS ClientHello, so that AnyTLS and Shadowsocks share one port.

Requires `tls`. Only Shadowsocks inbounds are accepted, since a plaintext protocol would show itself on the port.

Before the TLS handshake, the inbound looks at up to the first 6 bytes of each connection without consuming them: a TLS handshake record header (`16 03 00`–`16 03 04`, with a record length the TLS server accepts) followed by the ClientHello type `01`.
A connection is handed over as soon as one byte rules that out, so a Shadowsocks client is usually recognised by its first byte.
Random bytes such as a Shadowsocks salt look like a ClientHello about once in 3 × 10⁹ connections.

A connection that has not sent enough bytes to decide within 2 seconds is also handed over, so that a slow prober sees what the Shadowsocks inbound shows.

The connection is passed straight to the Shadowsocks inbound, not through the router, and is logged, routed and counted as that inbound's, as if it had accepted it.
On Linux the inbound peeks at the socket with `MSG_PEEK` and hands over the socket itself, so the Shadowsocks inbound reads it with no extra layer.

The Shadowsocks inbound still serves UDP. Give it the same port with `"network": "udp"`, since this inbound already listens on TCP there:

```json
{
  "inbounds": [
    {
      "type": "anytls",
      "tag": "anytls-in",
      "listen": "::",
      "listen_port": 443,
      "users": [
        {
          "name": "sekai",
          "password": "8JCsPssfgS8tiRwiMlhARg=="
        }
      ],
      "tls": {
        "enabled": true,
        "certificate_path": "/path/to/cert.pem",
        "key_path": "/path/to/key.pem"
      },
      "non_tls_handler": "ss-in"
    },
    {
      "type": "shadowsocks",
      "tag": "ss-in",
      "listen": "::",
      "listen_port": 443,
      "network": "udp",
      "method": "2022-blake3-aes-128-gcm",
      "password": "8JCsPssfgS8tiRwiMlhARg=="
    }
  ]
}
```

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

A download stops early once the kernel's delivery rate, counting only samples the application did not limit, has grown by less than 25% for three round trips, the test BBR uses to leave startup.
An upload ends when the client has sent what it was allowed, and the server answers once it has counted all of it.
The sender then waits for the filler to be acknowledged and reports its measurement:

| Key | Meaning |
|---|---|
| `goodput_bps`, `goodput_us` | Bytes acknowledged per second over the round trips after the ramp up in which only the network held the sender back, and how long those round trips lasted. The figure to rank paths on. |
| `avg_rate_bps` | `acked_bytes` over `busy_us`: the rate whenever the sender had data to send, whatever held it back. |
| `acked_bytes`, `busy_us` | Bytes acknowledged, and how long the sender had data to send. |
| `rwnd_limited_us`, `sndbuf_limited_us`, `net_limited_us` | How much of `busy_us` the receiver's window, the sender's buffer and the network held the sender back. |
| `peak_bw_bps` | The highest delivery rate sampled while the application did not limit it. |
| `bbr_bw_bps`, `bbr_min_rtt_us` | BBR's model of the path at the end, when the sender runs BBR. Its bandwidth reads high on paths that deliver in bursts. |
| `srtt_us`, `min_rtt_us` | Smoothed RTT at the end, under load, and the lowest RTT, without queueing. |
| `sent_bytes`, `retrans_bytes`, `dsack_dups`, `rtos` | Bytes sent and retransmitted, retransmissions the receiver already had, and retransmission timeouts. |
| `cc`, `drained` | The congestion control algorithm, and whether the filler was acknowledged within 3 seconds of the last write. |
| `received_bytes`, `rcv_rtt_us` | On the receiving end, what it received and its RTT estimate. |

`goodput_bps` is left out when the network alone limited the sender for less than 100 ms, for example when the receiver's window held it back throughout; `avg_rate_bps` and the limited times then say what the connection achieved and why.
The times are counted in kernel ticks of 1 to 10 ms.
The difference between `srtt_us` and `min_rtt_us` is the queueing delay the probe built up.

Rates are in bytes per second and durations in microseconds. A key the server's kernel does not report is left out.

Probe filler counts towards the user's traffic: it is passed to the traffic statistics and connection logs as one connection to `probe.anytls.arpa` through the outbound `anytls-probe`, which does not exist.
Filler a client sends outside an upload is counted the same way when its session closes, and a session that sends more of it than one probe allows is closed.
A probe that runs 10 seconds past its time, for example because the client stopped reading, has its session closed.

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

How long a user waits between two probes in the same direction, across all of the user's sessions.

`60s` is used by default.

##### probe.max_concurrent

The most probes running at once across this inbound. Others are refused as busy.

`4` is used by default.
