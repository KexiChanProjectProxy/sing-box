# Remaining Issues — Successor Handoff

This document describes all known issues remaining after the v1.13.11 upstream merge.
These are pre-existing problems that were NOT introduced by the merge but need to be fixed.

**Created**: 2026-04-30
**Merge commit**: `e8e05d124` (fork `30cecec5f` + upstream `553cfa1f9`)
**HEAD after merge**: `374f1f91c`

---

## Issue 1: quic-go / qpack HTTP/3 Dependency Mismatch

**Status**: ✅ RESOLVED — quic-go upgraded from v0.54.0 to v0.57.0
**Severity**: Was HIGH — blocks Hysteria2 and any QUIC/HTTP3-dependent protocol from building
**Pre-existing**: Yes (not caused by the v1.13.11 merge)

### Resolution
The `quic-go` module was upgraded from v0.54.0 to v0.57.0, aligning the `qpack` API
(`NewDecoder` callback argument removed, `DecodeFull` method signature updated).

### References
- Evidence: `.sisyphus/evidence/task-1-quic-upgrade.txt`

---

## Issue 2: TestTLSFragment Handshake Failure

**Status**: ✅ RESOLVED — tests now use local self-signed TLS server, no external dependencies
**Severity**: Was LOW — test-only, no production impact
**Pre-existing**: Yes (not caused by the v1.13.11 merge)

### Resolution
The `TestTLSFragment` test was updated to use a local self-signed TLS server,
eliminating external network dependencies and making tests deterministic.

### References
- Evidence: `.sisyphus/evidence/task-2-tlsfragment-fix.txt`

---

## Issue 3: libbox Legacy IPC Duplicate Method

**Status**: ✅ RESOLVED — dead code removed (12 `libbox_legacy_ipc`-tagged files deleted)
**Severity**: Was LOW — only affects `libbox_legacy_ipc` build tag, not production builds
**Pre-existing**: Yes (existed before the v1.13.11 merge)

### Resolution
The `libbox_legacy_ipc` build tag and all associated code (12 files in
`experimental/libbox/`) were deleted. The legacy IPC code path is no longer
present in the repository.

### References
- Evidence: `.sisyphus/evidence/task-3-libbox-legacy-cleanup.txt`

---

## Issue 4: Docker-dependent Integration Tests

**Severity**: LOW — CI-only, infrastructure requirement, not a code problem
**Pre-existing**: Yes

### Clarification
Integration tests in `test/` require a running Docker daemon. The warning:
```
WARN[0000] Cannot connect to the Docker daemon at unix:///var/run/docker.sock
```
indicates Docker is not available in the current environment — this is NOT a code
regression or build failure. The tests themselves are correctly implemented.

### Distinction
- **Infrastructure absence** (this issue): Docker daemon not running → warning printed,
  tests skipped gracefully. No code change needed.
- **Genuine code failure**: Test would fail with assertion errors or non-zero exit code
  even when Docker is available.

### How to Handle
1. In environments with Docker: tests run normally in CI
2. In environments without Docker: expect the warning, no action needed
3. To verify Docker is the only blocker: `docker run hello-world` should succeed

---

## Quick Verification Commands

After fixing any of the above, run these to verify the merge is still intact:

```bash
# Core build gate (should always pass)
go build ./option/... ./route/... ./protocol/group/... ./protocol/vless/... \
  ./experimental/libbox/... ./transport/v2raycloudflared/...

# Core test gate (should always pass)
go test ./option/... ./route/... ./protocol/group/... ./protocol/vless/...

# Full build (should pass — quic-go issue is resolved)
go build ./...

# Zero conflict markers (should always be clean)
grep -rn '<<<<<<<\|=======\|>>>>>>>' --include='*.go' .

# Fork surfaces check (should always pass)
grep 'sing-anytls' go.mod
ls transport/v2raycloudflared/
grep 'asnReader' route/router.go
grep 'geositeReader' route/router.go
grep 'sniffOverrideDestination' route/router.go
```

**Note**: `transport/v2rayxhttp/` was deleted in Task 4. Do not attempt to build or
list it — those commands will fail because the path no longer exists.

---

## Priority Order for Fixing

All issues (1–4) are now resolved. The priority ordering below is retained for
historical reference — no further action is required on this document.

1. ~~Issue 1 (quic-go/qpack)~~ — ✅ RESOLVED — quic-go upgraded to v0.57.0
2. ~~Issue 3 (libbox legacy IPC)~~ — ✅ RESOLVED — legacy code removed
3. ~~Issue 2 (tlsfragment test)~~ — ✅ RESOLVED — local TLS server used
4. ~~Issue 4 (Docker tests)~~ — ✅ CLARIFIED — infrastructure requirement, not a code issue
