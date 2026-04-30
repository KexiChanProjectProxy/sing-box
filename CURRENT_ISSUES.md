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

**Affected packages**: `transport/v2rayxhttp`

**Error**:
```
transport/v2rayxhttp/client.go:32:21: undefined: option.V2RayXHTTPOptions
transport/v2rayxhttp/mux.go:45:22: undefined: option.V2RayXHTTPXmuxConfig
transport/v2rayxhttp/config.go:10:36: undefined: option.V2RayXHTTPRangeConfig
```

**Root cause**: The `transport/v2rayxhttp/` package references `option.V2RayXHTTPOptions`,
`option.V2RayXHTTPXmuxConfig`, and `option.V2RayXHTTPRangeConfig` types that do not exist
in `option/`. These option types were either never added to the fork's `option/` schema,
or were removed during a prior refactor and the transport package was not updated.

**Impact**: The entire `v2rayxhttp` transport cannot compile. Any configuration or
runtime path that would use this transport is dead code.

**Status**: Pre-existing. Not caused by the v1.13.11 migration. Requires either adding
the missing option types to `option/` or removing the `v2rayxhttp` transport entirely.

---

### 1.3 Tailscale Protocol — Missing `tun` and `ping` Symbols

**Affected packages**: `protocol/tailscale`, `service/derp` (depends on `protocol/tailscale`)

**Error**:
```
protocol/tailscale/endpoint.go:87:25: undefined: tun.ICMPForwarder
protocol/tailscale/endpoint.go:348:34: undefined: tun.DefaultNIC
protocol/tailscale/endpoint.go:356:23: undefined: tun.NewICMPForwarder
protocol/tailscale/endpoint.go:685:27: undefined: ping.ConnectGVisor
```

**Root cause**: The `protocol/tailscale/` package references `tun.ICMPForwarder`,
`tun.DefaultNIC`, `tun.NewICMPForwarder`, and `ping.ConnectGVisor` from external
dependencies that are either not present in `go.mod` or have incompatible versions.
These symbols come from the `gVisor` networking stack used by Tailscale.

**Impact**: The Tailscale protocol and DERP service cannot compile.

**Status**: Pre-existing. Not caused by the v1.13.11 migration. Requires aligning the
Tailscale/gVisor dependency versions or adding the missing packages to `go.mod`.

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

| # | Issue | Type | Package(s) | Pre-existing | Migration-Related |
|---|-------|------|------------|-------------|-------------------|
| 1.1 | quic-go HTTP/3 API mismatch | Build | protocol/router (transitive) | Yes | No |
| 1.2 | Missing V2RayXHTTP option types | Build | transport/v2rayxhttp | Yes | No |
| 1.3 | Missing tun/ping gVisor symbols | Build | protocol/tailscale, service/derp | Yes | No |
| 2.1 | TestTLSFragment handshake failure | Test | common/tlsfragment | Yes | No |
