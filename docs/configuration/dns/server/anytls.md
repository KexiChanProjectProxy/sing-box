---
icon: material/new-box
---

!!! question "Since sing-box 1.14.2.11"

# AnyTLS

Sends queries to the server of an [AnyTLS outbound](/configuration/outbound/anytls/) inside its sessions.
The server answers them with its own DNS.

### Structure

```json
{
  "dns": {
    "servers": [
      {
        "type": "anytls",
        "tag": "",

        "outbound": "anytls-out"
      }
    ]
  }
}
```

### Fields

#### outbound

==Required==

The tag of the [AnyTLS outbound](/configuration/outbound/anytls/).

A selector or another group of outbounds is not accepted.

### How it works

Each query travels as a raw DNS message in a frame of its own inside an AnyTLS session, without opening a stream.
It goes on the open session with the lowest latency, and a session is dialed when there is none.
A session that closes mid-query is retried once on another.

The server must enable [`dns`](/configuration/inbound/anytls/#dns) on its AnyTLS inbound.
Other servers do not offer it, and queries fail at once with an error saying so, while proxied connections keep working.

!!! warning

    Do not resolve the server address of the AnyTLS outbound with this DNS server, through its `domain_resolver` or `route.default_domain_resolver`.
    The outbound would need this server to connect, and this server would need the outbound.
