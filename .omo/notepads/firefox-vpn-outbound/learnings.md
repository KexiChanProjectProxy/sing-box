# Firefox VPN Outbound - Learnings

## Marshal-Pointer Gotcha (2026-07-03)
`Outbound.MarshalJSONContext` has a pointer receiver (`*Outbound`). When marshaling a non-addressable `Outbound` value (e.g. `json.MarshalContext(ctx, outbound)`), Go does NOT invoke the pointer-receiver method — it falls back to default struct encoding, which skips the `Options any` field (`json:"-"`). This caused `TestFirefoxVPNOptionsRoundTrip` to lose all option fields during round-trip, failing with "email is required" on re-unmarshal. Fix: always marshal as `&outbound`. Same pattern applies to `Inbound`, `Endpoint`, `Service`, `CertificateProvider` — all use the same `MarshalJSONContext` pointer-receiver idiom.

## Firefox VPN Dual Dependencies (2026-07-03)
`outbound.NewAdapterWithDialerOptions` only tracks `DialerOptions.Detour`. Firefox VPN needs both `detour` and `api_detour`, so the minimal fix is a protocol-local `Dependencies()` override that returns a deduplicated slice of non-empty tags. This preserves the existing outbound manager error paths (`dependency[...] not found`, `circular outbound dependency`) without changing the generic adapter framework.

## Test Module Invocation (2026-07-03)
The integration tests under `/test` are a nested Go module. Running `go test ./test` from the repo root fails module resolution (`main module ... does not contain package .../test`), so execute Firefox VPN verification commands from `/home/kexi/sing-box/test` with `go test . -run ...`.

## Control-Plane Client Injection Notes (2026-07-03)
The Firefox VPN control-plane client can keep system-root TLS by leaving `http.Transport.TLSClientConfig.RootCAs` nil and only wiring sing-box time/dial hooks. For unit tests, `newControlPlaneClient` can inject `httptest` endpoints directly; `NewControlPlaneHTTPClient` falls back to a stdlib `net.Dialer` when `api_detour` is empty so package-level tests do not need a fully initialized sing-box router context.

## Runtime Auth Controller Notes (2026-07-03)
The in-memory Firefox VPN auth controller was easiest to test by keeping the production field typed as `*ControlPlaneClient` and injecting a factory that returns `newControlPlaneClient(httptest.Server.Client(), endpoints)`. This preserves the real FxA + Guardian wire contract while avoiding token persistence, and it makes refresh-token preference / proxy-pass renewal assertions straightforward with per-endpoint call counters.

## File Size Guardrail Follow-Up (2026-07-03)
The first controller implementation exceeded the 250 pure-LOC skill limit. Splitting public lifecycle/setup code (`controller.go`) from runtime refresh/retry helpers (`controller_runtime.go`) kept the package cohesive while satisfying the architectural ceiling without changing behavior.

## TLS+h2 CONNECT Session Transport (2026-07-03)
- The dedicated Firefox VPN data-plane session was easiest to keep swappable by making each Session immutable around one proxy-pass token and letting the outbound replace the current session pointer while retaining retired sessions for existing tunnels.
- Client-side stream caps are enough to satisfy stream-limit handling for Todo 5; a small semaphore around CONNECT setup avoids opening extra HTTP/2 streams even before server SETTINGS become observable.
- HTTP/2 CONNECT timeout handling needs a request-lifetime context decoupled from the caller\s dial context; otherwise canceling the dial timeout or caller context can tear down an already-established tunnel.

## Integration Harness Constraints (2026-07-03)
- `api_detour` is resolved by directly calling the tagged outbound, so ordinary route rules were not enough to redirect Firefox Accounts / Guardian during the full-box integration test.
- The cleanest test-only workaround was to patch `protocol/firefoxvpn.NewControlPlaneClient` inside `test/firefoxvpn` so the real outbound still boots through `startInstance`, but its control-plane client points at local `httptest` FxA/Guardian endpoints with no internet dependency.
- Detour tags cannot point at an empty direct outbound (`detour to an empty direct outbound makes no sense`), so the test direct outbounds need at least one benign dialer option such as a connect timeout.

## Test-Only Override Hook Cleanup (2026-07-03)
Replaced unsafe/reflect/syscall runtime function patching in `test/firefoxvpn/happy_test.go` with a clean exported test-only override variable in `protocol/firefoxvpn/client.go`. The pattern: export a `var TestNewControlPlaneClientOverride func(...)` checked at the top of `NewControlPlaneClient`; the test sets it to delegate to `NewControlPlaneClientWithEndpoints` with custom endpoints. This removed ~70 lines of unsafe code (`patchFunction`, `absoluteJump`, `setUnexportedField`, etc.) and the `reflect`/`sync`/`syscall`/`unsafe` imports. The `ControlPlaneEndpoints` type was exported along with a `NewControlPlaneEndpoints` constructor to let external test code create endpoint values without accessing unexported struct fields.

## Documentation and Field Coverage Testing (2026-07-03)
- The docs field coverage test (`TestFirefoxVPNDocsCoverAllPublicFields`) only checks FirefoxVPNOutboundOptions' own fields (api_detour, email, password, tls). DialerOptions/ServerOptions fields are documented in shared sections (Dial Fields, TLS) and intentionally excluded.
- The example config test (`TestFirefoxVPNExampleConfigParses`) embeds the JSON via `//go:embed` and parses it through the full `option.Options` unmarshal path (including the outbound registry in context via `include.Context`).
- Using `github.com/sagernet/sing/common/json` (not `encoding/json`) for `UnmarshalContext` since the options unmarshaling requires a context with the outbound registry.

## Detour Regression Harness Split (2026-07-03)
- The observable detour probe works best as a tiny SOCKS5 server in the test harness: record the requested destination before dialing, optionally rewrite fake hostnames to 127.0.0.1 for local httptest backends, and optionally force a dial failure to simulate broken detours.
- `api_detour` traffic is directly observable at the SOCKS layer (FxA + Guardian hosts). The Firefox VPN `detour` only wraps the upstream TLS session to the Firefox VPN h2 proxy; the inner tunnel target (echo backend) stays observable at the fake h2 proxy CONNECT layer, not at the detour proxy itself.
- `protocol/firefoxvpn.TestNewControlPlaneClientOverride` is global mutable state, so the integration regressions cannot run with `t.Parallel()` without cross-test endpoint contamination.

## F3 Rejection Fix: Test Package Consolidation (2026-07-03)
- Moved `TestFirefoxVPNListenPacketRejected` and `TestFirefoxVPNDialContextRejectsNonTCP` from `protocol/firefoxvpn/outbound_test.go` to `test/firefoxvpn/outbound_test.go`.
- The `stubOutboundManager` already existed in `test/firefoxvpn/detour_regressions_helpers_test.go`, so no duplication needed.
- `newTestOutbound` helper was copied and adapted: uses `protocolfirefoxvpn.NewOutbound` and `protocolfirefoxvpn.Outbound` via the existing import alias.
- The old `protocol/firefoxvpn/outbound_test.go` was deleted entirely — no tests remain there.
- The F3 smoke-test command `(cd test && go test ./firefoxvpn -run 'TestFirefoxVPNHappyPath|TestFirefoxVPNListenPacketRejected|TestFirefoxVPNDialContextRejectsNonTCP' -v)` now covers all three tests in one package.

## TCP-Only Guardrail Unit Tests (2026-07-03)
- `Outbound.ListenPacket` unconditionally returns `os.ErrInvalid` — no start required, no network argument needed.
- `Outbound.DialContext` rejects non-TCP networks via `E.Extend(N.ErrUnknownNetwork, network)` before any session/auth logic runs.
- `outbound.Adapter` exposes `Network() []string` (not `Networks()`); the adapter is registered with `[]string{N.NetworkTCP}`.
- The integration test package (`test/firefoxvpn_test.go`) is `package main` and defines its own `stubOutboundManager`; package-level tests in `protocol/firefoxvpn` must define a local stub.

## F3 Smoke-Test Package Path (2026-07-03)
The F3 final-verification smoke test must target `./test/firefoxvpn` (the nested Go module's test package), not `./test` (which fails module resolution from the repo root). Similarly, Todo 6 QA scenarios under `./test` should be invoked as `(cd test && go test . -run ...)`.

## F2 Secret-Leakage Fix (2026-07-03)
Fixed potential secret/token leakage through upstream error responses. In `fxa.go` and `guardian.go`, all HTTP non-2xx error paths previously included the raw response body (`string(body)`) in the returned error. These bodies can contain credential material, token data, or PII from upstream servers. Changed all 5 error sites to report only the HTTP status code: `fmt.Errorf("login failed (HTTP %d)", response.StatusCode)`. In `controller_runtime.go`, replaced `log.Err(err)` with category-based `log.String("error", "refresh_failed")` / `log.String("error", "retry_failed")` to prevent wrapped errors (which may chain the now-sanitized upstream errors) from being serialized into logs. The `isRefreshTokenInvalid` helper continues to work because it checks for "http 400" (case-insensitive) which is still present in the status-code-only error format.
