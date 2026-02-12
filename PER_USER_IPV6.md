# Per-User IPv6 Address Assignment

## Overview

This feature allows each authenticated user to automatically get a unique, deterministic IPv6 address for outbound connections. The IPv6 address is computed as `hash(user_id, src_ip)` within a configurable IPv6 prefix.

## Use Case

When you have a routed IPv6 range and want each user to use a distinct IPv6 address for outbound connections, this feature provides automatic, deterministic address assignment without manual configuration.

## Configuration

### Outbound-Level Configuration

Add `inet6_bind_prefix` to your direct outbound:

```json
{
  "outbounds": [{
    "type": "direct",
    "tag": "direct-ipv6",
    "inet6_bind_prefix": "2001:db8:abcd::/48"
  }]
}
```

### Rule Action-Level Configuration

Apply per-user IPv6 binding based on routing rules:

```json
{
  "route": {
    "rules": [{
      "auth_user": ["alice", "bob"],
      "action": "direct",
      "inet6_bind_prefix": "2001:db8:abcd::/48"
    }]
  }
}
```

## How It Works

1. **Authentication**: Client connects through HTTP/SOCKS inbound with user authentication
2. **Hash Generation**: System computes a deterministic hash from `user_id` + `source_ip` using FNV-128a
3. **Address Assignment**: The hash is combined with the configured prefix to generate a unique IPv6 address
4. **Binding**: Outbound connections bind to the computed IPv6 address

### Data Flow

```
Client (user=alice, src=192.168.1.1)
  → Inbound (sets metadata: User="alice", Source=192.168.1.1)
  → Router (selects outbound with inet6_bind_prefix)
  → Per-User IPv6 Dialer
    → Computes: fnv128a("alice" + "192.168.1.1") → hash
    → Generates: 2001:db8:abcd::<hash_host_bits>
    → Binds outbound connection to computed address
  → Internet (source IPv6 = 2001:db8:abcd::xxxx:xxxx:xxxx:xxxx)
```

## Key Features

- **Deterministic**: Same user + source IP always produces the same IPv6 address
- **Unique**: Different users or source IPs produce different IPv6 addresses
- **Automatic**: No manual address management required
- **Flexible**: Works with any prefix length from /32 to /127
- **Fallback**: Gracefully falls back to default behavior when no user is authenticated

## Edge Cases

- **No user authenticated**: Falls back to default binding behavior (no special IPv6 address)
- **IPv4 destination**: Per-user IPv6 binding is not applied (IPv4 uses default dialer)
- **Dual-stack connections**: IPv6 path uses per-user address, IPv4 path uses default
- **Hash collisions**: Negligible probability with FNV-128a and typical prefix sizes (e.g., /48 provides 2^80 address space)

## Validation

The following validations are performed at startup:

1. `inet6_bind_prefix` and `inet6_bind_address` are mutually exclusive
2. `inet6_bind_prefix` must be a valid IPv6 prefix
3. `inet6_bind_prefix` must have host bits available (prefix length < 128)

## Requirements

- Routed IPv6 prefix assigned to your server
- User authentication enabled on inbound (HTTP/SOCKS with auth)
- IPv6 connectivity on the outbound network

## Example Configuration

Full example with HTTP inbound and per-user IPv6:

```json
{
  "inbounds": [{
    "type": "http",
    "tag": "http-in",
    "listen": "0.0.0.0",
    "listen_port": 8080,
    "users": [
      {"username": "alice", "password": "pass1"},
      {"username": "bob", "password": "pass2"}
    ]
  }],
  "outbounds": [{
    "type": "direct",
    "tag": "direct",
    "inet6_bind_prefix": "2001:db8:1234::/48"
  }],
  "route": {
    "rules": [{
      "inbound": ["http-in"],
      "outbound": "direct"
    }]
  }
}
```

In this configuration:
- Alice connecting from 192.168.1.1 will always use the same IPv6 address (e.g., `2001:db8:1234:a1b2:c3d4:e5f6:7890:1234`)
- Bob connecting from 192.168.1.1 will use a different IPv6 address
- Alice connecting from a different source IP will use a different IPv6 address
- Unauthenticated connections fall back to default behavior

## Testing

Unit tests verify:
- Determinism: same user+source → same IPv6
- Uniqueness: different user or source → different IPv6
- Prefix preservation: generated addresses stay within the configured prefix
- Various prefix lengths (/32, /48, /64, /80, /96)
- Edge cases (no user, no context)

Run tests with:
```bash
go test ./common/dialer -run TestPerUserIPv6 -v
```
