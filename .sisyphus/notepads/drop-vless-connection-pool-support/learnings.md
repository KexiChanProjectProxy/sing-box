# Learnings - Drop VLESS Connection Pool Support

## Codebase Context
- This is a Go project (sing-box proxy platform)
- VLESS connection pool is a fork-specific feature (`// fork-extension:` comments)
- Key files: `option/vless.go`, `protocol/vless/outbound.go`, `protocol/vless/pool.go`, `protocol/vless/pool_test.go`, `protocol/vless/integration_test.go`
- `CURRENT_ISSUES.md` sections 2.2 and 3.1 describe the pool as an incomplete integration
- `scripts/README.md` line 105 has generic "memory leak in connection pool" text in example changelog

## Key Observations
- `option/vless.go` has `ConnectionPoolOptions` struct (lines 32-43) and `ConnectionPool *ConnectionPoolOptions` field (line 29)
- `protocol/vless/outbound.go` has pool field (line 43), construction (lines 100-115), lifecycle (lines 162-164, 170-172), and dial branch (lines 195-196)
- `protocol/vless/pool.go` is 450 lines, entirely fork-specific
- `protocol/vless/pool_test.go` is 669 lines, all pool-specific tests
- `protocol/vless/integration_test.go` is 302 lines, all pool-related tests
- `CURRENT_ISSUES.md` lines 110-148 recommend wiring the pool in - must be reversed

## Pre-existing Build Issues (NOT caused by this change)
- quic-go HTTP/3 dependency mismatch
- v2rayxhttp missing option types
- Tailscale missing tun/ping symbols
- tlsfragment test failure

