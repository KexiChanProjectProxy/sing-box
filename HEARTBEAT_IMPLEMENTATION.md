# AnyTLS Heartbeat Implementation

## Summary

Added heartbeat/keepalive functionality to the AnyTLS protocol to prevent NAT session timeouts.

## Changes Made

### 1. Modified `anytls` library (in `/tmp/sing-anytls/`)

#### `client.go`
- Added `Heartbeat time.Duration` field to `ClientConfig` struct
- Pass heartbeat parameter to session client

#### `session/client.go`
- Added `heartbeat time.Duration` field to `Client` struct
- Updated `NewClient()` signature to accept heartbeat parameter
- Pass heartbeat to each new session

#### `session/session.go`
- Added `heartbeat time.Duration` field to `Session` struct
- Updated `NewClientSession()` signature to accept heartbeat parameter
- Implemented `heartbeatLoop()` goroutine that:
  - Runs in a ticker loop at specified interval
  - Sends `cmdHeartRequest` frames periodically
  - Stops when session closes
- Added debug logging:
  - Client: "[Heartbeat] Sending heartbeat request"
  - Client: "[Heartbeat] Heartbeat sent successfully"
  - Client: "[Heartbeat] Failed to send heartbeat: <error>"
  - Server: "[Heartbeat] Received heartbeat request, sending response"
  - Both: "[Heartbeat] Received heartbeat response"

### 2. Modified `sing-box` (in `/home/kexi/sing-box-1.12.14/`)

#### `option/anytls.go`
- Added `Heartbeat badoption.Duration` field to `AnyTLSOutboundOptions`

#### `protocol/anytls/outbound.go`
- Pass `options.Heartbeat.Build()` to anytls.ClientConfig

#### `go.mod`
- Added replace directive: `replace github.com/anytls/sing-anytls => /tmp/sing-anytls`

## Configuration Example

```json
{
  "outbounds": [
    {
      "type": "anytls",
      "tag": "anytls-out",
      "server": "your.server.com",
      "server_port": 443,
      "password": "your-password",
      "heartbeat": "30s",
      "tls": {
        "enabled": true,
        "server_name": "your.server.com"
      }
    }
  ]
}
```

## Heartbeat Intervals

Recommended heartbeat intervals based on NAT timeout:
- **Conservative (default NAT)**: `30s` - Works with most NATs (60s+ timeout)
- **Aggressive (strict NAT)**: `10s` - For carriers with short timeouts (30s)
- **Moderate**: `60s` - For stable connections with longer NAT timeouts

## How It Works

1. **Client-side only**: Only the client needs to be updated
2. **Session-level**: Heartbeat operates on the underlying TCP+TLS connection
3. **Encrypted**: Heartbeat packets are encrypted within TLS, indistinguishable from data
4. **All sessions**: Keeps alive both idle and active sessions
5. **Server auto-responds**: Current servers already handle heartbeat responses

## Architecture

```
┌─────────────────────────────────────────────┐
│  AnyTLS Session (TCP+TLS Connection)        │  ← Heartbeat here
│                                              │
│  Client ─(HeartRequest)→ Server              │
│  Client ←(HeartResponse)─ Server             │
│                                              │
│  ┌────────────┐  ┌────────────┐             │
│  │ Stream #1  │  │ Stream #2  │  ...        │  ← Proxy connections
│  └────────────┘  └────────────┘             │
└─────────────────────────────────────────────┘
```

## Debug Logging

To see heartbeat activity, set log level to `debug`:

```json
{
  "log": {
    "level": "debug"
  }
}
```

You will see logs like:
- `[Heartbeat] Sending heartbeat request` (every interval on client)
- `[Heartbeat] Heartbeat sent successfully` (after successful send)
- `[Heartbeat] Received heartbeat request, sending response` (on server)
- `[Heartbeat] Received heartbeat response` (on client)

## Backward Compatibility

- **Old servers**: Automatically support heartbeat (already implemented)
- **Old clients**: Continue to work (heartbeat is optional)
- **No server update needed**: Current deployed servers work with new clients

## Testing

Build the modified sing-box:
```bash
go build -v -trimpath -tags "with_gvisor,with_quic,with_dhcp,with_wireguard,with_utls,with_acme,with_clash_api" ./cmd/sing-box
```

Run with debug logs:
```bash
./sing-box run -c config.json
```

Monitor logs for heartbeat activity.

## NAT Session Keepalive

The heartbeat keeps NAT sessions alive by:
1. Sending TCP packets every X seconds (configurable via `heartbeat`)
2. NAT sees traffic → resets idle timeout → keeps mapping alive
3. Works for both idle sessions (in pool) and active sessions with idle streams
4. Prevents connection drops due to NAT timeout

## Notes

- If `heartbeat` is not set or set to `0`, heartbeat is disabled
- Heartbeat adds minimal overhead (7 bytes header, periodic timer)
- Recommended to set slightly less than your NAT timeout (e.g., 30s for 60s NAT timeout)
- Heartbeat continues as long as session is alive (idle or active)
