# firefox-vpn-outbound - Work Plan

## TL;DR (For humans)
<!-- Fill this LAST, after the detailed plan below is written, so it summarizes the REAL plan. -->
<!-- Plain English for a non-engineer: NO file paths, NO todo numbers, NO wave/agent/tool names. -->

**What you'll get:** A new Firefox VPN outbound for sing-box that logs in with your configured Firefox Account email/password, fetches and refreshes Firefox VPN proxy credentials in memory, and sends outbound TCP traffic over a single upstream HTTP/2 CONNECT session. It will support separate routing for Mozilla API traffic and for the proxied user traffic itself.

**Why this approach:** The plan keeps V1 tightly scoped to the protocol path the reference client already proves works today: HTTP/2 CONNECT, not MASQUE. It also separates control-plane and data-plane routing with `api_detour` plus normal `detour`, because sing-box's built-in single-detour helper is not enough for this feature's startup ordering or traffic isolation.

**What it will NOT do:**
- It will not add MASQUE in this first version.
- It will not persist tokens or proxy sessions to disk.
- It will not auto-pick locations or servers from Mozilla's server list.
- It will not proxy UDP in V1.

**Effort:** Large
**Risk:** Medium - the main risk is getting the single-connection HTTP/2 session lifecycle and dual-detour wiring correct without broad cross-cutting refactors, while still depending on an external Firefox VPN reference client that may drift from current Mozilla behavior.
**Decisions to sanity-check:** Explicit upstream proxy server fields in V1, memory-only token/session state, and Firefox-specific dual-dependency wiring instead of a generic multi-detour framework.

Your next move: start implementation now, or ask me to run a high-accuracy review of this plan first. Full execution detail follows below.

---

> TL;DR (machine): Large / medium-risk plan to add a CONNECT-only Firefox VPN outbound with config-file email/password auth, dual detours (`api_detour` + `detour`), in-memory token/session rotation, TDD-first verification, and EN/ZH docs.

## Scope
### Must have
- Add `firefox-vpn` to outbound type constants, option decoding, registry wiring, and outbound docs.
- Public option surface built on standard `ServerOptions`, standard dial fields, standard outbound TLS controls, plus `api_detour`, `email`, and `password`.
- Control-plane client logic for FxA login, Hawk OAuth exchange, refresh-token renewal, and Guardian proxy-pass retrieval.
- Control-plane HTTPS must use system-root verification while routing via `api_detour`; V1 does not expose a second public TLS config just for Mozilla APIs.
- Runtime controller that keeps tokens, proxy-pass data, and active HTTP/2 sessions in memory only.
- Data plane limited to a single upstream TLS+h2 connection with multiple CONNECT tunnels for TCP traffic.
- Firefox-specific dependency wiring that causes both `api_detour` and `detour` to participate in startup ordering when present.
- Regression coverage proving: config validation, control-plane auth flow, CONNECT transport behavior, concurrent tunnel reuse, session rotation, dual-detour routing, no credential/token leakage to logs/evidence, no token persistence, and TCP-only guardrails.
- English and Chinese outbound documentation plus index entries and a parseable example config.
### Must NOT have (guardrails, anti-slop, scope boundaries)
- No MASQUE, QUIC tunnel mode, or other non-CONNECT Firefox VPN transports.
- No disk cache, keychain integration, or persisted refresh/access/proxy tokens.
- No interactive login UX, browser/device auth, or secret prompts.
- No serverlist-driven country/city/server auto-selection in V1; upstream proxy endpoint must be explicit in config.
- No generic refactor that changes every outbound adapter just to support this feature.
- No UDP support hidden behind partial stubs; `ListenPacket` must reject packet mode clearly.
- No credential, access-token, refresh-token, or proxy-pass material in logs, test output, or `.omo/evidence/*` artifacts.

## Verification strategy
> Zero human intervention - all verification is agent-executed.
- Test decision: TDD + Go unit tests (`testing` + `stretchr/testify/require`) and local end-to-end harnesses modeled after `test/box_test.go` and `test/noisyshuttle/*`.
- Evidence: .omo/evidence/task-<N>-firefox-vpn-outbound.<ext>
- Evidence format: each evidence file must record the exact command/tool invocation, timestamp, pass/fail summary, and redacted output only; never store raw passwords, raw OAuth tokens, raw refresh tokens, or raw proxy-pass JWTs.
- Retry/backoff and timeout behavior must be verified by tests, not only code inspection.
- Targeted suites before implementation merges:
  - `go test ./option ./protocol/firefoxvpn -run TestFirefoxVPN...`
  - `go test ./test -run TestFirefoxVPN...`
- Final repository gate after all todos: `go test ./...`
- Every new public config example must also be parsed by a test; docs are not considered done if the example is unverified.

## Execution strategy
### Parallel execution waves
> Target 5-8 todos per wave. Fewer than 3 (except the final) means you under-split.

- **Wave 1 - public surface and primitives:** Todos 1-3 establish type/option shape, registry/dependency scaffolding, and FxA/Guardian auth primitives.
- **Wave 2 - runtime transport:** Todos 4-6 add the in-memory controller, single-session HTTP/2 CONNECT transport, and outbound lifecycle wiring.
- **Wave 3 - verification and docs:** Todos 7-9 add full-stack regression coverage, dual-detour/no-persistence guards, and EN/ZH documentation with tested examples.

### Dependency matrix
| Todo | Depends on | Blocks | Can parallelize with |
| --- | --- | --- | --- |
| 1 | none | 2, 3, 4, 5, 9 | none |
| 2 | 1 | 6, 7, 8 | 3 |
| 3 | 1 | 4, 6, 7, 8 | 2 |
| 4 | 3 | 6, 7, 8 | 5 |
| 5 | 1 | 6, 7, 8 | 4 |
| 6 | 2, 4, 5 | 7, 8, 9 | none |
| 7 | 6 | 8 | 9 |
| 8 | 6, 7 | F1-F4 | 9 |
| 9 | 1, 2 | F1-F4 | 7, 8 |

## Todos
> Implementation + Test = ONE todo. Never separate.
<!-- APPEND TASK BATCHES BELOW THIS LINE WITH edit/apply_patch - never rewrite the headers above. -->
- [x] 1. [Public config surface] Define `firefox-vpn` options and validation to expose dual-detour auth-driven CONNECT outbound - expect parseable config with explicit guardrails
  What to do / Must NOT do: Add `TypeFirefoxVPN = "firefox-vpn"`, create `option/firefox_vpn.go`, and define the public option shape around `ServerOptions`, `DialerOptions`, `OutboundTLSOptionsContainer`, `api_detour`, `email`, and `password`. Constructor- or option-level validation must reject missing credentials, missing upstream server/port, and unsupported packet-mode assumptions. Must NOT add public token-cache fields, MASQUE fields, separate Mozilla-API TLS knobs, or serverlist auto-selection knobs in V1.
  Parallelization: Wave 1 | Blocked by: none | Blocks: 2, 3, 4, 5, 9
  References (executor has NO interview context - be exhaustive): `constant/proxy.go:3-39`, `option/outbound.go:19-59`, `option/outbound.go:67-94`, `option/simple.go:22-40`, `option/tor.go:3-9`, `option/noisyshuttle.go:27-37`, `docs/configuration/outbound/tor.md:1-50`
  Acceptance criteria (agent-executable): `go test ./option -run 'TestFirefoxVPN(Options|Validation)'` passes and a config containing `type: firefox-vpn`, `api_detour`, `email`, `password`, `server`, and `server_port` unmarshals successfully while malformed variants fail with precise errors.
  QA scenarios (name the exact tool + invocation): Happy - `go test ./option -run 'TestFirefoxVPNOptionsRoundTrip|TestFirefoxVPNValidationAcceptsMinimalConfig'`; Failure - `go test ./option -run 'TestFirefoxVPNValidationRejects(MissingPassword|MissingServerPort|TokenPersistenceFields)'`; Evidence `.omo/evidence/task-1-firefox-vpn-outbound.txt`.
  Commit: Y | feat(firefox-vpn): add option surface and validation

- [x] 2. [Registry and dependency topology] Register the outbound and expose both `detour` and `api_detour` as startup dependencies - expect deterministic start order without global adapter refactors
  What to do / Must NOT do: Wire the new outbound into `include/registry.go` and create a Firefox-specific adapter/dependency helper that returns both dependency edges (deduped when equal) instead of relying on `outbound.NewAdapterWithDialerOptions(...)`'s single-detour behavior. Must NOT broaden `adapter/outbound/adapter.go` into a generic framework unless the Firefox-specific helper proves insufficient.
  Parallelization: Wave 1 | Blocked by: 1 | Blocks: 6, 7, 8
  References (executor has NO interview context - be exhaustive): `include/registry.go:78-106`, `adapter/outbound/registry.go:13-72`, `adapter/outbound.go:30-36`, `adapter/outbound/adapter.go:24-35`, `adapter/outbound/manager.go:96-166`, `common/dialer/detour.go:19-99`
  Acceptance criteria (agent-executable): `go test ./test -run 'TestFirefoxVPN(Registers|Dependencies|MissingDependency)'` passes, proving registration works and startup errors identify missing `api_detour`/`detour` tags correctly.
  QA scenarios (name the exact tool + invocation): Happy - `go test ./test -run 'TestFirefoxVPNRegistersAndStartsWithDualDependencies'`; Failure - `go test ./test -run 'TestFirefoxVPNMissingApiDetourFails|TestFirefoxVPNCircularDependencyFails'`; Evidence `.omo/evidence/task-2-firefox-vpn-outbound.txt`.
  Commit: Y | feat(firefox-vpn): register outbound and dual dependencies

- [x] 3. [FxA and Guardian primitives] Port the reference login/token/proxy-pass logic behind an injectable control-plane client - expect deterministic unit coverage of the auth chain
  What to do / Must NOT do: First verify the cited reference files under `/home/kexi/firefox-vpn-client/` are still accessible and still describe the intended login/CONNECT flow; if they drift materially, pause and surface the diff before changing public surface decisions. Then create control-plane code under `protocol/firefoxvpn/` that ports the reference algorithms for `account/login`, Hawk OAuth exchange, refresh-token renewal, and Guardian proxy-pass retrieval, but make endpoints injectable internally for tests instead of turning them into public config. Must route HTTPS over `api_detour` with system-root verification. Must NOT shell out, prompt, or store credentials outside process memory.
  Parallelization: Wave 1 | Blocked by: 1 | Blocks: 4, 6, 7, 8
  References (executor has NO interview context - be exhaustive): `/home/kexi/firefox-vpn-client/fxa.go:45-229`, `/home/kexi/firefox-vpn-client/guardian.go:15-173`, `service/ocm/service.go:148-217`, `service/ccm/service.go:129-196`, `common/httpclient/client.go:17-85`, `option/http.go:27-46`
  Acceptance criteria (agent-executable): `go test ./protocol/firefoxvpn -run 'TestFirefoxVPN(ControlPlane|Fxa|Guardian|Refresh)'` passes against stub HTTP servers validating request shape and parsed responses.
  QA scenarios (name the exact tool + invocation): Happy - `go test ./protocol/firefoxvpn -run 'TestFirefoxVPNFxaLoginOAuthChain|TestFirefoxVPNGuardianProxyPassParses'`; Failure - `go test ./protocol/firefoxvpn -run 'TestFirefoxVPNRefreshHandlesHTTPError|TestFirefoxVPNGuardianRejectsEmptyToken'`; Evidence `.omo/evidence/task-3-firefox-vpn-outbound.txt`.
  Commit: Y | feat(firefox-vpn): add FxA and Guardian primitives

- [x] 4. [Runtime auth controller] Build the in-memory token/proxy-pass manager over `api_detour` - expect refresh-before-expiry without password re-login on healthy refresh paths
  What to do / Must NOT do: Build a controller that constructs a control-plane HTTP client using `api_detour`, performs the initial login chain, refreshes access tokens when needed, refreshes proxy passes before expiry, and keeps all state in memory only. Add bounded retry/backoff for login/refresh/proxy-pass failures and make timeout sources explicit. Must prefer refresh-token renewal over repeated password login while the process stays alive, and must NOT write caches to disk or add persistence side effects.
  Parallelization: Wave 2 | Blocked by: 3 | Blocks: 6, 7, 8
  References (executor has NO interview context - be exhaustive): `/home/kexi/firefox-vpn-client/cmd/proxy-demo/main.go:318-321`, `/home/kexi/firefox-vpn-client/cmd/proxy-demo/main.go:641-865`, `service/ocm/service.go:273-302`, `common/httpclient/manager.go:98-126`, `option/http.go:27-46`, `common/dialer/dialer.go:40-146`
  Acceptance criteria (agent-executable): `go test ./protocol/firefoxvpn -run 'TestFirefoxVPN(RuntimeAuth|ProxyPassRenewal|MemoryOnly|Backoff|NoSecretLogging)'` passes and confirms refresh/login behavior uses only in-memory state, retries are bounded, and secrets never appear in captured logs.
  QA scenarios (name the exact tool + invocation): Happy - `go test ./protocol/firefoxvpn -run 'TestFirefoxVPNRuntimeAuthRefreshesBeforeExpiry|TestFirefoxVPNProxyPassRenews'`; Failure - `go test ./protocol/firefoxvpn -run 'TestFirefoxVPNNoRefreshTokenFallsBackToLogin|TestFirefoxVPNBackoffAfterRepeatedFailure|TestFirefoxVPNDoesNotPersistTokens|TestFirefoxVPNLogsRedactSecrets'`; Evidence `.omo/evidence/task-4-firefox-vpn-outbound.txt`.
  Commit: Y | feat(firefox-vpn): add runtime auth controller

- [x] 5. [Single-connection CONNECT transport] Implement the dedicated TLS+h2 session and stream-to-`net.Conn` wrapper for Firefox proxy traffic - expect one upstream connection serving many CONNECT tunnels
  What to do / Must NOT do: Implement a dedicated upstream proxy session using a single TLS connection and `http2.ClientConn`, then wrap each CONNECT stream as a `net.Conn` for the outbound data plane. Reuse ideas from `transport/v2rayhttp/*` for the connection wrapper, but do NOT replace the design with a generic `http.RoundTripper` pool because V1 needs a single owned upstream session. Respect server-advertised stream limits and define client-side dial/stream timeouts.
  Parallelization: Wave 2 | Blocked by: 1 | Blocks: 6, 7, 8
  References (executor has NO interview context - be exhaustive): `/home/kexi/firefox-vpn-client/cmd/proxy-demo/main.go:510-581`, `/home/kexi/firefox-vpn-client/cmd/proxy-demo/main.go:598-639`, `transport/v2rayhttp/client.go:121-152`, `transport/v2rayhttp/conn.go:132-176`, `common/httpclient/http2_transport.go:17-52`, `common/httpclient/http2_fallback_transport.go:20-98`
  Acceptance criteria (agent-executable): `go test ./protocol/firefoxvpn -run 'TestFirefoxVPN(H2Session|ConnectTunnel|ConcurrentStreams|Timeouts)'` passes against a local TLS+h2 test server, proving CONNECT success, bounded timeouts, stream-limit handling, and non-2xx failure propagation.
  QA scenarios (name the exact tool + invocation): Happy - `go test ./protocol/firefoxvpn -run 'TestFirefoxVPNConnectTunnelStreamsTCP|TestFirefoxVPNConcurrentStreamsShareSingleSession'`; Failure - `go test ./protocol/firefoxvpn -run 'TestFirefoxVPNConnectRejectsNon2xx|TestFirefoxVPNRejectsNonH2Proxy|TestFirefoxVPNConnectTimeout'`; Evidence `.omo/evidence/task-5-firefox-vpn-outbound.txt`.
  Commit: Y | feat(firefox-vpn): add h2 CONNECT transport

- [x] 6. [Outbound lifecycle integration] Wire controller + session transport into `DialContext`/`Start`/`Close` with TCP-only semantics - expect smooth session swaps and explicit UDP rejection
  What to do / Must NOT do: Assemble the outbound so startup acquires credentials and opens an initial session, `DialContext` opens CONNECT tunnels on the active session, active tunnels keep old sessions alive during rotation, and `Close` cancels pending dials and drains/forces old sessions with an explicit bounded shutdown policy. State access must be concurrency-safe for many simultaneous `DialContext` calls. `ListenPacket` must reject packet mode clearly because V1 is CONNECT-only. Must NOT quietly fake UDP support or rebuild a new upstream connection for every outbound TCP stream.
  Parallelization: Wave 2 | Blocked by: 2, 4, 5 | Blocks: 7, 8, 9
  References (executor has NO interview context - be exhaustive): `protocol/tor/outbound.go:33-220`, `protocol/http/outbound.go:26-66`, `protocol/socks/outbound.go:29-122`, `/home/kexi/firefox-vpn-client/cmd/proxy-demo/main.go:691-895`, `adapter/outbound/manager.go:96-166`, `adapter/outbound.go:30-47`
  Acceptance criteria (agent-executable): `go test ./protocol/firefoxvpn ./test -run 'TestFirefoxVPN(Outbound|TCPOnly|SessionSwap|ConcurrentDial|Shutdown)'` passes, proving start/close behavior, retained in-flight tunnels during session replacement, concurrency safety, bounded shutdown, and packet-mode rejection.
  QA scenarios (name the exact tool + invocation): Happy - `go test ./test -run 'TestFirefoxVPNOutboundReusesSessionAndSwapsGracefully|TestFirefoxVPNConcurrentDialsSafe'`; Failure - `go test ./test -run 'TestFirefoxVPNListenPacketRejected|TestFirefoxVPNStartFailsWhenInitialLoginFails|TestFirefoxVPNCloseCancelsPendingDial'`; Evidence `.omo/evidence/task-6-firefox-vpn-outbound.txt`.
  Commit: Y | feat(firefox-vpn): wire outbound lifecycle

- [x] 7. [Happy-path end-to-end harness] Add a local fake FxA/Guardian/h2 proxy stack and a mixed-inbound integration test - expect real TCP proxying through a started sing-box instance
  What to do / Must NOT do: Build a reusable test harness under `test/firefoxvpn/` (or a single root test file if reuse stays small) that starts stub FxA and Guardian services, a TLS+h2 CONNECT proxy, and a TCP echo backend, then boots sing-box with the new outbound through the existing `startInstance` helper. Use ephemeral ports / isolated listeners so the suite is parallel-safe. Must NOT rely on external Mozilla services or flaky internet dependencies.
  Parallelization: Wave 3 | Blocked by: 6 | Blocks: 8
  References (executor has NO interview context - be exhaustive): `test/box_test.go:38-70`, `test/noisyshuttle/config.go:18-136`, `test/noisyshuttle/harness.go:168-260`, `/home/kexi/firefox-vpn-client/cmd/proxy-demo/main.go:331-508`
  Acceptance criteria (agent-executable): `go test ./test -run 'TestFirefoxVPNHappyPath'` passes, demonstrating a client can reach the echo backend through mixed inbound -> firefox-vpn outbound -> fake CONNECT proxy.
  QA scenarios (name the exact tool + invocation): Happy - `go test ./test -run 'TestFirefoxVPNHappyPath'`; Failure - `go test ./test -run 'TestFirefoxVPNHappyPathRejectsBadProxyPass'`; Evidence `.omo/evidence/task-7-firefox-vpn-outbound.txt`.
  Commit: Y | test(firefox-vpn): add happy-path integration harness

- [x] 8. [Dual-detour and guardrail regressions] Prove `api_detour` and `detour` route different traffic and that restart/login guardrails hold - expect no hidden persistence or routing leaks
  What to do / Must NOT do: Extend the harness with two observable detour outbounds (for example local SOCKS proxies) so tests can prove control-plane traffic uses `api_detour` while the upstream CONNECT session uses `detour`. Instrument those proxies to record destination authorities/tags, and assert them directly in tests. Add regressions for restart-caused re-login, `api_detour == detour` deduped dependency behavior, no token files on disk, and failure behavior when either detour is broken. Must NOT settle for log-grep-only evidence.
  Parallelization: Wave 3 | Blocked by: 6, 7 | Blocks: F1-F4
  References (executor has NO interview context - be exhaustive): `common/dialer/detour.go:19-99`, `common/proxybridge/bridge.go:22-119`, `protocol/socks/outbound.go:29-122`, `adapter/outbound/manager.go:144-163`, `/home/kexi/firefox-vpn-client/cmd/proxy-demo/main.go:779-865`
  Acceptance criteria (agent-executable): `go test ./test -run 'TestFirefoxVPN(ApiDetour|DataDetour|RestartRequiresLogin|BrokenDetour|EqualDetours)'` passes and asserts recorded destinations from both test detour proxies by exact destination match.
  QA scenarios (name the exact tool + invocation): Happy - `go test ./test -run 'TestFirefoxVPNApiDetourSeparateFromDataDetour|TestFirefoxVPNRestartRequiresFreshLogin|TestFirefoxVPNEqualDetoursDedupDependencies'`; Failure - `go test ./test -run 'TestFirefoxVPNBrokenApiDetourFailsControlPlane|TestFirefoxVPNBrokenDataDetourFailsTunnel'`; Evidence `.omo/evidence/task-8-firefox-vpn-outbound.txt`.
  Commit: Y | test(firefox-vpn): add detour and persistence regressions

- [x] 9. [Docs and config examples] Document the outbound in EN/ZH and verify example configs parse - expect discoverable public usage with no undocumented fields
  What to do / Must NOT do: Add `docs/configuration/outbound/firefox-vpn.md` and `.zh.md`, update outbound index pages, and include a minimal example showing explicit upstream server fields, `api_detour`, `detour`, `email`, and `password`. Back the example with a parse test or fixture so docs cannot drift, and ensure every exported public field is documented in both languages. Must NOT document MASQUE, disk cache, serverlist auto-selection, or a separate Mozilla-API TLS config as supported features.
  Parallelization: Wave 3 | Blocked by: 1, 2 | Blocks: F1-F4
  References (executor has NO interview context - be exhaustive): `docs/configuration/outbound/index.md:16-49`, `docs/configuration/outbound/index.zh.md:16-49`, `docs/configuration/outbound/tor.md:1-50`, `docs/configuration/outbound/noisy-shuttle.md:1-120`, `option/outbound.go:19-59`
  Acceptance criteria (agent-executable): `go test ./test -run 'TestFirefoxVPNExampleConfigParses|TestFirefoxVPNDocsCoverAllPublicFields'` passes, and `grep 'firefox-vpn' docs/configuration/outbound/index.md docs/configuration/outbound/index.zh.md` returns both new index rows.
  QA scenarios (name the exact tool + invocation): Happy - `go test ./test -run 'TestFirefoxVPNExampleConfigParses|TestFirefoxVPNDocsCoverAllPublicFields'`; Failure - `grep 'firefox-vpn' docs/configuration/outbound/index.md docs/configuration/outbound/index.zh.md` must fail if index rows are missing; Evidence `.omo/evidence/task-9-firefox-vpn-outbound.txt`.
  Commit: Y | docs(firefox-vpn): add outbound documentation

## Final verification wave
> Runs in parallel after ALL todos. ALL must APPROVE. All verification work is agent-executed; once receipts are collected, surface results and wait for the user's explicit sign-off before declaring the implementation complete.
- [ ] F1. Plan compliance audit - verify every changed file maps to Todos 1-9, every guardrail in Scope OUT still holds, and evidence files exist for all completed todos.
- [ ] F2. Code quality review - run `go test ./...`, inspect diagnostics, and review for secret persistence, over-generalized adapter refactors, and unintended UDP/MASQUE code paths.
- [ ] F3. Agent-run end-to-end smoke test - run `go test ./test/firefoxvpn -run 'TestFirefoxVPNHappyPath|TestFirefoxVPNListenPacketRejected' -v` and require one successful TCP tunnel plus one explicit packet-mode rejection.
- [ ] F4. Scope fidelity - verify the public config surface matches the approved choices: CONNECT-only, memory-only, `api_detour`, config-file `email/password`, TDD-backed implementation.

## Commit strategy
- Default commit model: one commit per todo marked `Commit: Y`, using the todo's message as the preferred subject.
- Squash/fixup is allowed after verification, but only if the final history still preserves three logical buckets: feature surface, runtime transport/auth behavior, and tests/docs.
- If implementation pressure forces fewer commits, preserve at least one code commit and one verification/docs commit, and explicitly fold Todo 6 into the same feature commit bucket as Todos 3-5.
- Do not mix unrelated cleanup into these commits.

## Success criteria
- sing-box accepts a `firefox-vpn` outbound config with explicit upstream server fields, `email/password`, `api_detour`, and normal `detour`.
- Startup ordering recognizes both dependency edges when present and fails clearly on missing/circular references.
- Runtime control-plane login, refresh, and proxy-pass renewal work entirely in memory.
- TCP traffic reaches a target service through a single owned upstream HTTP/2 CONNECT session; UDP is rejected explicitly.
- Regression tests prove control-plane traffic follows `api_detour`, tunnel traffic follows `detour`, equal detours are handled deliberately, concurrent tunnels reuse the owned session safely, and restart requires fresh login because nothing persisted.
- Tests and captured evidence show secrets are redacted from logs and never written to `.omo/evidence/*`.
- English and Chinese docs plus example config are present and verified by tests.
