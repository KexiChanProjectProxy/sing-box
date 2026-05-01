# Remaining Issues — Successor Handoff

This document describes all known issues remaining after the v1.13.11 upstream merge.
These are pre-existing problems that were NOT introduced by the merge but need to be fixed.

**Created**: 2026-04-30
**Merge commit**: `e8e05d124` (fork `30cecec5f` + upstream `553cfa1f9`)
**HEAD after merge**: `374f1f91c`

---

## Issue 1: quic-go / qpack HTTP/3 Dependency Mismatch

**Severity**: HIGH — blocks Hysteria2 and any QUIC/HTTP3-dependent protocol from building
**Pre-existing**: Yes (not caused by the v1.13.11 merge)

### Symptoms
```
# github.com/quic-go/quic-go/http3
http3/client.go:98:31: too many arguments in call to qpack.NewDecoder
http3/conn.go:196:27: c.decoder.DecodeFull undefined
http3/server.go:589:22: decoder.DecodeFull undefined
http3/stream.go:324:24: s.decoder.DecodeFull undefined
```

### Root Cause
The `quic-go` module at `v0.54.0` (used by `sagernet/sing-quic v0.6.1`) expects `qpack`
with `NewDecoder(callback)` signature and `DecodeFull()` method. However, the resolved
`qpack` version in the Go module graph has a newer API that:
- Removed the callback argument from `NewDecoder`
- Removed `DecodeFull` from `qpack.Decoder`

The `quic-go` package appears twice in the dependency graph:
1. `github.com/sagernet/quic-go` (direct, via sing-quic)
2. `github.com/quic-go/quic-go` (indirect, transitive)

These are different Go modules with different versioning.

### Affected Packages
- `protocol/hysteria2/...` — Hysteria2 outbound
- `protocol/router/...` — if it imports H2/H3 transports
- Any package transitively importing `quic-go/http3`

### How to Fix
1. Check if `sagernet/sing-quic` has a newer version that aligns with a compatible `qpack`
2. Alternatively, add a `replace` directive in `go.mod` to pin `qpack` to a compatible version:
   ```
   replace github.com/quic-go/qpack => github.com/quic-go/qpack v0.5.1
   ```
3. Test: `go build ./protocol/hysteria2/...` should pass after the fix
4. Verify: `go build ./...` should have zero errors (excluding any other pre-existing issues)

### References
- Evidence: `.sisyphus/evidence/task-19-verification.txt`
- Upstream sing-box v1.13.11 builds successfully, so the upstream `go.mod` has the right version alignment

---

## Issue 2: TestTLSFragment Handshake Failure

**Severity**: LOW — test-only, no production impact
**Pre-existing**: Yes (not caused by the v1.13.11 merge)

### Symptoms
```
--- FAIL: TestTLSFragment (0.58s)
    conn_test.go:21:
        Error Trace:  common/tlsfragment/conn_test.go:21
        Error:        Received unexpected error:
                      tls: first record does not look like a TLS handshake
```

### Root Cause
The `TestTLSFragment` test attempts a TLS handshake but the remote end or test server
is not responding with a valid TLS record. This is likely an environment issue:
- Network unavailability to the expected test endpoint
- Missing test server
- Go TLS version incompatibility

### Affected Packages
- `common/tlsfragment` — test only

### How to Fix
1. Investigate what endpoint the test expects to connect to
2. Check if the test needs a local TLS server or specific network access
3. If the test is inherently environment-dependent, consider adding a build tag or skip condition
4. Test: `go test ./common/tlsfragment/...` should pass

### References
- Evidence: `.sisyphus/evidence/task-19-verification.txt`

---

## Issue 3: libbox Legacy IPC Duplicate Method

**Severity**: LOW — only affects `libbox_legacy_ipc` build tag, not production builds
**Pre-existing**: Yes (existed before the v1.13.11 merge)

### Symptoms
When building with `libbox_legacy_ipc` tag, duplicate method errors appear in
`experimental/libbox/command_server.go`.

### Root Cause
The legacy IPC code path defines methods that conflict with the current IPC implementation.
The `libbox_legacy_ipc` build tag is not used in production builds.

### How to Fix
1. Decide whether legacy IPC is still needed
2. If yes, resolve the method name conflicts
3. If no, remove the `libbox_legacy_ipc` build tag and associated code
4. Test: `go build -tags libbox_legacy_ipc ./experimental/libbox/...` should pass

### References
- Evidence: `.sisyphus/evidence/task-7-libbox-edge.txt`, `.sisyphus/evidence/task-15-libbox-runtime-edge.txt`

---

## Issue 4: Docker-dependent Integration Tests

**Severity**: LOW — CI-only, not a code problem
**Pre-existing**: Yes

### Symptoms
Integration tests in `test/` that require Docker produce warnings:
```
WARN[0000] Cannot connect to the Docker daemon at unix:///var/run/docker.sock
```

### How to Fix
1. These tests require a running Docker daemon
2. In CI environments with Docker, they should pass automatically
3. No code changes needed — this is an infrastructure requirement

---

## Quick Verification Commands

After fixing any of the above, run these to verify the merge is still intact:

```bash
# Core build gate (should always pass)
go build ./option/... ./route/... ./protocol/group/... ./protocol/vless/... \
  ./experimental/libbox/... ./transport/v2rayxhttp/... ./transport/v2raycloudflared/...

# Core test gate (should always pass)
go test ./option/... ./route/... ./protocol/group/... ./protocol/vless/...

# Full build (currently fails on quic-go only)
go build ./...

# Zero conflict markers (should always be clean)
grep -rn '<<<<<<<\|=======\|>>>>>>>' --include='*.go' .

# Fork surfaces check (should always pass)
grep 'sing-anytls' go.mod
ls transport/v2rayxhttp/
ls transport/v2raycloudflared/
grep 'asnReader' route/router.go
grep 'geositeReader' route/router.go
grep 'sniffOverrideDestination' route/router.go
```

---

## Priority Order for Fixing

1. **Issue 1 (quic-go/qpack)** — HIGH priority, blocks real protocol builds
2. **Issue 3 (libbox legacy IPC)** — LOW priority, only affects non-production build tag
3. **Issue 2 (tlsfragment test)** — LOW priority, test-only, environment-dependent
4. **Issue 4 (Docker tests)** — LOW priority, infrastructure requirement only
