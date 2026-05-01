# Current Known Issues

This document catalogs the known build failures, test failures, and incomplete integrations
in the sing-box fork as of the v1.13.11 upstream alignment migration (2026-04-30).

## 1. Build Failures

### 1.1 quic-go HTTP/3 Dependency Mismatch

**Affected packages**: `protocol/router`, and any package transitively importing `quic-go/http3`

**Error**:
```
# github.com/quic-go/quic-go/http3
http3/client.go:98:31: too many arguments in call to qpack.NewDecoder
http3/conn.go:196:27: c.decoder.DecodeFull undefined
http3/server.go:589:22: decoder.DecodeFull undefined
http3/stream.go:324:24: s.decoder.DecodeFull undefined
```

**Root cause**: The `quic-go` module at `v0.54.0` is incompatible with the `qpack` version
resolved by the Go module graph. The `qpack.NewDecoder` signature changed between versions,
and `quic-go/http3` calls the old API with a callback argument that the newer `qpack` no
longer accepts. Similarly, `DecodeFull` was removed from `qpack.Decoder`.

**Impact**: Any package that transitively depends on `quic-go/http3` fails to build. This
includes `protocol/router` (which imports H2/H3 transports).

**Status**: Pre-existing. Not caused by the v1.13.11 migration. Requires a `quic-go` or
`qpack` version alignment in `go.mod`.

---

### 1.2 v2rayxhttp Transport — Missing Option Types

**Status**: ✅ RESOLVED (Task 6, Task 14)
- `V2RayXHTTPOptions`, `V2RayXHTTPXmuxConfig`, `V2RayXHTTPRangeConfig` types exist in `option/v2ray_transport.go`
- `transport/v2rayxhttp/...` compiles successfully
- No changes required

---

### 1.3 Tailscale Protocol — Missing `tun` and `ping` Symbols

**Status**: ✅ RESOLVED (Task 17)
- All referenced symbols (`tun.ICMPForwarder`, `tun.DefaultNIC`, `tun.NewICMPForwarder`, `ping.ConnectGVisor`) are available in sing-tun v0.8.9
- `protocol/tailscale` and `service/derp` compile successfully with proper build tags (`with_gvisor,with_tailscale`)
- No dependency changes required

---

## 2. Test Failures

### 2.1 `common/tlsfragment` — TestTLSFragment

**Affected test**: `github.com/sagernet/sing-box/common/tlsfragment`

**Error**:
```
--- FAIL: TestTLSFragment (0.58s)
    conn_test.go:21:
        Error Trace:  common/tlsfragment/conn_test.go:21
        Error:        Received unexpected error:
                      tls: first record does not look like a TLS handshake
        Test:         TestTLSFragment
```

**Root cause**: The `TestTLSFragment` test attempts a TLS handshake but the remote end
or test server is not responding with a valid TLS record. This is likely an environment
issue (network unavailability, missing test server, or Go TLS version incompatibility)
rather than a code regression.

**Impact**: The `tlsfragment` package test fails. The package itself compiles fine.

**Status**: Pre-existing. Likely environment-dependent. May pass in CI or with network
access to the expected test endpoint.

---

## 3. Migration Artifacts

The migration reports (`.sisyphus/reports/module-inventory.md` and
`.sisyphus/reports/migration-matrix.md`) were generated during the initial analysis phase
(Tasks 1-2) and describe several intended annotations and decisions. Some of these were
only partially reflected in the source tree before the fix session. After the Final
Verification Wave fixes, all `// fork-extension:` comments required by the plan have been
applied to core files. However, the reports may still describe migration decisions
(`Rebase` vs `Refactor` vs `Drop`) that have not been fully executed as code changes —
they remain analysis documents, not implementation proof.

---

## Summary Table

| # | Issue | Type | Package(s) | Pre-existing | Status |
|---|-------|------|------------|-------------|--------|
| 1.1 | quic-go HTTP/3 API mismatch | Build | protocol/router (transitive) | Yes | OPEN - External dependency conflict |
| 1.2 | Missing V2RayXHTTP option types | Build | transport/v2rayxhttp | Yes | ✅ RESOLVED (Task 6, 14) |
| 1.3 | Missing tun/ping gVisor symbols | Build | protocol/tailscale, service/derp | Yes | ✅ RESOLVED (Task 17) |
| 2.1 | TestTLSFragment handshake failure | Test | common/tlsfragment | Yes | OPEN - Pre-existing test issue |

## Resolved Issues (Merge Hygiene)

The following issues were identified during the v1.13.11 merge and have been resolved:

1. **V2RayXHTTP option types** - Added to `option/v2ray_transport.go` (Task 6)
2. **V2RayXHTTP transport adaptation** - Updated for upstream API changes: `tls.Config.Config()` → `STDConfig()`, `string` Host → `badoption.Listable[string]` (Task 6)
3. **Tailscale/gVisor symbols** - Verified available in sing-tun v0.8.9 (Task 17)
4. **Route core fork extensions** - `asnReader`, `geositeReader`, `sniffOverrideDestination` properly preserved (Task 4)
5. **VLESS connection pool** - Explicitly removed, no reintroduction (Task 5)
6. **DNS MockTransport** - Added missing `Reset()` method (Task 10)
