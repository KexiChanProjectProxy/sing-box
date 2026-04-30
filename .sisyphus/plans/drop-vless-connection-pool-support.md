# Drop VLESS Connection Pool Support

## TL;DR
> **Summary**: Hard-remove the fork-only VLESS connection pool feature from runtime wiring, option schema, and tests, then scrub stale in-repo references so old `connection_pool` configs fail fast instead of being silently accepted.
> **Deliverables**:
> - Remove VLESS pool implementation and outbound lifecycle wiring
> - Remove `connection_pool` from `option.VLESSOutboundOptions`
> - Replace pool-positive tests with removal/regression coverage
> - Clean stale references in issue/release-support docs if still applicable
> **Effort**: Medium
> **Parallel**: YES - 2 waves
> **Critical Path**: 1 → 2/3 → 4/5 → 6

## Context
### Original Request
Drop vless connection pool support.

### Interview Summary
- Removal mode is **hard remove**.
- In-repo documentation/example cleanup is **in scope** if stale references exist.
- No backward-compatibility shim or soft deprecation path should be planned.

### Metis Review (gaps addressed)
- Make config rejection explicit for legacy `connection_pool` users; do not treat removal as a silent no-op.
- Verify no VLESS pool helpers or stale references remain outside the obvious implementation files.
- Treat `CURRENT_ISSUES.md` and `scripts/README.md` as likely stale-reference cleanup targets.
- Keep scope limited to VLESS-only pooling; do not broaden into pooling changes for other protocols.

## Work Objectives
### Core Objective
Eliminate the fork-specific VLESS connection pool feature so VLESS outbound behavior always uses non-pooled dialing paths, and configurations that still declare `connection_pool` are rejected by the remaining config surface.

### Deliverables
- Updated `protocol/vless/outbound.go` with all pool-specific fields, construction, reset, close, and dial branches removed.
- Deleted `protocol/vless/pool.go` and any VLESS-only helper types that become dead code with it.
- Updated `option/vless.go` with `ConnectionPoolOptions` and `VLESSOutboundOptions.ConnectionPool` removed.
- Updated tests in `protocol/vless/` to remove pool-specific coverage and add hard-removal regression coverage for config rejection and normal outbound behavior.
- Updated stale references in `CURRENT_ISSUES.md`, `scripts/README.md`, and any nearby in-repo release notes/examples discovered during execution.

### Definition of Done (verifiable conditions with commands)
- `go test ./protocol/vless` passes.
- `go test ./...` passes.
- `cd test && go test -tags "$(TAGS_TEST)" .` passes.
- `grep -R "connection_pool" protocol/vless option CURRENT_ISSUES.md scripts README.md docs test` returns no VLESS-pool references except intentional changelog/migration text added by the executor.
- `grep -R "ConnectionPoolOptions\|NewConnectionPool\|GetConn(ctx).*pool" protocol/vless option` returns no VLESS pool implementation remnants.

### Must Have
- Hard failure for VLESS configs that still contain `connection_pool`.
- No runtime pool lifecycle code left in VLESS outbound.
- No orphaned tests or comments describing the removed feature as still supported.
- Verification evidence captured for every task under `.sisyphus/evidence/`.

### Must NOT Have (guardrails, AI slop patterns, scope boundaries)
- Must NOT introduce a soft-deprecation path that accepts and ignores `connection_pool`.
- Must NOT modify pooling logic for non-VLESS protocols.
- Must NOT rewrite unrelated VLESS features such as multiplex, TLS, packet encoding, or transport behavior beyond what is required to remove pool branches.
- Must NOT preserve dead compatibility structs or comments “for future use”.
- Must NOT mark work complete until Final Verification Wave approvals are presented and the user explicitly says okay.

## Verification Strategy
> ZERO HUMAN INTERVENTION - all verification is agent-executed.
- Test decision: **tests-after** using standard Go tests, `testify/require`, `goleak`, and the repository `make test` pattern from `Makefile:225-235`.
- QA policy: Every task includes agent-executed happy-path and failure/edge scenarios.
- Evidence: `.sisyphus/evidence/task-{N}-{slug}.{ext}`

## Execution Strategy
### Parallel Execution Waves
> Target: 5-8 tasks per wave. <3 per wave (except final) = under-splitting.
> Extract shared dependencies as Wave-1 tasks for max parallelism.

Wave 1: Task 1 (schema contract), Task 2 (runtime removal), Task 3 (unit/integration test refactor)

Wave 2: Task 4 (repo reference cleanup), Task 5 (full regression/build verification), Task 6 (final branch hygiene and evidence packaging)

### Dependency Matrix (full, all tasks)
| Task | Depends On | Blocks |
|---|---|---|
| 1 | None | 2, 3, 4, 5 |
| 2 | 1 | 5, 6 |
| 3 | 1 | 5, 6 |
| 4 | 1 | 6 |
| 5 | 2, 3 | 6 |
| 6 | 2, 3, 4, 5 | F1-F4 |

### Agent Dispatch Summary (wave → task count → categories)
- Wave 1 → 3 tasks → `quick`, `unspecified-low`
- Wave 2 → 3 tasks → `quick`, `unspecified-low`
- Final Verification Wave → 4 tasks → `oracle`, `unspecified-high`, `deep`

## TODOs
> Implementation + Test = ONE task. Never separate.
> EVERY task MUST have: Agent Profile + Parallelization + QA Scenarios.

- [x] 1. Remove VLESS `connection_pool` from option schema and make legacy configs fail

  **What to do**: Delete `ConnectionPoolOptions` and the `ConnectionPool *ConnectionPoolOptions` field from `option.VLESSOutboundOptions`. Confirm the remaining JSON decode/option validation path no longer accepts VLESS `connection_pool`, and add/replace tests so a VLESS outbound config containing `connection_pool` now fails parsing/validation instead of succeeding.
  **Must NOT do**: Do not add a deprecated alias, ignored field, custom compatibility shim, or cross-protocol schema changes.

  **Recommended Agent Profile**:
  - Category: `quick` - Reason: small, localized option-surface change with direct regression tests.
  - Skills: `[]` - No special skill required.
  - Omitted: `['/git-master']` - No git operation is part of this task.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: 2, 3, 4, 5 | Blocked By: none

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `option/vless.go:19-43` - Current outbound option surface including `ConnectionPool` and `ConnectionPoolOptions` definitions.
  - Pattern: `protocol/vless/integration_test.go:42-158` - Existing tests currently proving pool config parses successfully; these must be inverted/replaced.
  - Pattern: `CURRENT_ISSUES.md:110-148` - Stale note that currently recommends adding the field; removal must reverse that guidance.

  **Acceptance Criteria** (agent-executable only):
  - [ ] `option/vless.go` no longer contains `ConnectionPoolOptions` or `json:"connection_pool,omitempty"`.
  - [ ] `go test ./protocol/vless -run 'Test.*VLESS|Test.*ConnectionPool|Test.*TCPFastOpen'` passes with replacement coverage for config rejection.
  - [ ] A VLESS outbound config fixture or inline JSON containing `connection_pool` now fails with a deterministic parse/validation error checked by test assertions.

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Legacy VLESS config is rejected
    Tool: Bash
    Steps: Run `go test ./protocol/vless -run Test.*Legacy.*ConnectionPool.*Rejected -v | tee .sisyphus/evidence/task-1-schema-removal.txt`
    Expected: Test passes only if VLESS config containing `connection_pool` returns the expected error text or failure condition.
    Evidence: .sisyphus/evidence/task-1-schema-removal.txt

  Scenario: Option surface no longer exposes pool field
    Tool: Bash
    Steps: Run `grep -n 'ConnectionPool\|connection_pool' option/vless.go | tee .sisyphus/evidence/task-1-schema-removal-error.txt`
    Expected: Command produces no matches for VLESS pool schema identifiers.
    Evidence: .sisyphus/evidence/task-1-schema-removal-error.txt
  ```

  **Commit**: YES | Message: `refactor(vless): remove connection pool options` | Files: `option/vless.go`, `protocol/vless/integration_test.go`

- [x] 2. Remove runtime VLESS pool implementation and outbound lifecycle branches

  **What to do**: Delete `protocol/vless/pool.go`, remove the `pool *ConnectionPool` field from `protocol/vless/outbound.go`, remove pool construction in `NewOutbound`, remove pool reset/close handling in `InterfaceUpdated` and `Close`, and simplify `vlessDialer.DialContext` so TCP always uses `dialRawConn` before the existing VLESS client wrapping. Preserve non-pool behavior for TLS, transport, packet encoding, UDP, and multiplex.
  **Must NOT do**: Do not change multiplex semantics, packet encoding logic, or transport/TLS setup beyond removing pool references. Do not touch inbound VLESS.

  **Recommended Agent Profile**:
  - Category: `unspecified-low` - Reason: a slightly broader refactor touching runtime wiring and deletion of a feature file.
  - Skills: `[]` - No special skill required.
  - Omitted: `['/refactor']` - The change is localized and should stay manual/explicit.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: 5, 6 | Blocked By: 1

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `protocol/vless/outbound.go:30-44` - `Outbound` struct currently owns the pool field.
  - Pattern: `protocol/vless/outbound.go:100-115` - Pool construction from option values.
  - Pattern: `protocol/vless/outbound.go:155-173` - Lifecycle reset/close handling that must collapse cleanly after removal.
  - Pattern: `protocol/vless/outbound.go:189-199` - TCP dial branch currently prefers `h.pool.GetConn(ctx)`.
  - Pattern: `protocol/vless/pool.go:1-220` and remainder of file - Entire fork-only implementation targeted for deletion.

  **Acceptance Criteria** (agent-executable only):
  - [ ] `protocol/vless/pool.go` is deleted.
  - [ ] `protocol/vless/outbound.go` contains no `pool` field, no `ConnectionPoolConfig`, and no `GetConn` branch.
  - [ ] `go test ./protocol/vless` passes without runtime references to pool types.

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: VLESS outbound still works without pooling path
    Tool: Bash
    Steps: Run `go test ./protocol/vless -run 'Test.*VLESS|Test.*Outbound|Test.*Multiplex' -v | tee .sisyphus/evidence/task-2-runtime-removal.txt`
    Expected: Remaining VLESS tests pass and no compile errors mention `ConnectionPool`, `NewConnectionPool`, or `GetConn`.
    Evidence: .sisyphus/evidence/task-2-runtime-removal.txt

  Scenario: No pool runtime code survives
    Tool: Bash
    Steps: Run `grep -R -n 'ConnectionPool\|NewConnectionPool\|GetConn(ctx)' protocol/vless | tee .sisyphus/evidence/task-2-runtime-removal-error.txt`
    Expected: No matches in `protocol/vless` for VLESS pool runtime symbols after the refactor.
    Evidence: .sisyphus/evidence/task-2-runtime-removal-error.txt
  ```

  **Commit**: YES | Message: `refactor(vless): drop pooled dialing runtime` | Files: `protocol/vless/outbound.go`, `protocol/vless/pool.go`

- [x] 3. Replace pool-focused tests with hard-removal regression coverage

  **What to do**: Delete or rewrite `protocol/vless/pool_test.go` and the pool-positive portions of `protocol/vless/integration_test.go`. Keep only tests that still make sense after removal, and add focused regression coverage for: (a) legacy `connection_pool` config rejection, (b) unchanged multiplex parsing without pool support, and (c) normal VLESS outbound option parsing still succeeds when pool fields are absent.
  **Must NOT do**: Do not leave empty placeholder test files, skipped tests, or assertions that still expect pool defaults or coexistence semantics.

  **Recommended Agent Profile**:
  - Category: `quick` - Reason: test-surface rewrite with clear scope.
  - Skills: `[]` - No special skill required.
  - Omitted: `['/playwright']` - This is non-browser Go test work.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: 5, 6 | Blocked By: 1

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `protocol/vless/pool_test.go:1-260` and remainder - Entire file is dedicated to pool behavior and should be removed or fully replaced.
  - Pattern: `protocol/vless/integration_test.go:42-302` - Current pool-positive tests for configuration, defaults, and multiplex coexistence.
  - Pattern: `Makefile:225-235` - Canonical repository test commands that task output should remain compatible with.

  **Acceptance Criteria** (agent-executable only):
  - [ ] No test in `protocol/vless/` references `ConnectionPool`, `connection_pool`, or pool-default behavior except the explicit rejection test.
  - [ ] `go test ./protocol/vless` passes.
  - [ ] Replacement tests cover both a valid pool-free VLESS config and an invalid legacy pool-bearing config.

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Pool-free VLESS option parsing still succeeds
    Tool: Bash
    Steps: Run `go test ./protocol/vless -run 'Test.*Valid.*VLESS|Test.*Multiplex.*WithoutPool' -v | tee .sisyphus/evidence/task-3-test-refresh.txt`
    Expected: Updated tests confirm standard VLESS configs still parse and compile successfully.
    Evidence: .sisyphus/evidence/task-3-test-refresh.txt

  Scenario: No stale pool-positive tests remain
    Tool: Bash
    Steps: Run `grep -R -n 'TestConnectionPool\|EnsureIdleSession\|MaxConnectionLifetime' protocol/vless/*_test.go | tee .sisyphus/evidence/task-3-test-refresh-error.txt`
    Expected: No stale pool-specific tests or assertions remain.
    Evidence: .sisyphus/evidence/task-3-test-refresh-error.txt
  ```

  **Commit**: YES | Message: `test(vless): replace connection pool coverage` | Files: `protocol/vless/pool_test.go`, `protocol/vless/integration_test.go`

- [x] 4. Clean stale in-repo references to the removed feature

  **What to do**: Update `CURRENT_ISSUES.md` to stop recommending adding/wiring the VLESS pool. Update `scripts/README.md` only if the generic example text is now misleading in project context; if it is intentionally generic, leave it unchanged and document why in task evidence. Search nearby in-repo docs/examples/release notes for VLESS-specific `connection_pool` references and remove or reword them.
  **Must NOT do**: Do not invent broad documentation rewrites or touch outbound VLESS docs unless a stale pool reference actually exists.

  **Recommended Agent Profile**:
  - Category: `quick` - Reason: narrow cleanup of known stale references and adjacent search hits.
  - Skills: `[]` - No special skill required.
  - Omitted: `['/frontend-ui-ux']` - Documentation cleanup does not need UI/design specialization.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 6 | Blocked By: 1

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `CURRENT_ISSUES.md:110-148` - Currently describes the pool as an incomplete integration that should be wired in; this becomes stale after removal.
  - Pattern: `scripts/README.md:90-106` - Generic release-note example mentioning a connection pool memory leak; evaluate whether it falsely implies an existing VLESS feature.
  - Pattern: `docs/configuration/outbound/vless.md` and `docs/configuration/outbound/vless.zh.md` - Exploration found no `connection_pool` docs today; only update if executor finds fresh stale references during implementation.

  **Acceptance Criteria** (agent-executable only):
  - [ ] `CURRENT_ISSUES.md` no longer says the VLESS pool should be wired back in.
  - [ ] Repo-wide search results for VLESS `connection_pool` references are either removed or intentionally documented as generic/non-VLESS.
  - [ ] Any retained generic pool wording is justified in evidence with file/path context.

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Stale VLESS guidance is removed
    Tool: Bash
    Steps: Run `grep -R -n 'VLESS.*connection pool\|connection_pool' CURRENT_ISSUES.md scripts README.md docs test | tee .sisyphus/evidence/task-4-reference-cleanup.txt`
    Expected: No stale VLESS-support guidance remains; any surviving generic text is explicitly reviewed and acceptable.
    Evidence: .sisyphus/evidence/task-4-reference-cleanup.txt

  Scenario: Generic release-doc example is handled deliberately
    Tool: Bash
    Steps: Capture a short note explaining whether `scripts/README.md` changed, then run `git diff -- CURRENT_ISSUES.md scripts/README.md | tee .sisyphus/evidence/task-4-reference-cleanup-error.txt`
    Expected: Diff shows intentional cleanup only, with no unrelated documentation churn.
    Evidence: .sisyphus/evidence/task-4-reference-cleanup-error.txt
  ```

  **Commit**: YES | Message: `docs(vless): remove stale connection pool references` | Files: `CURRENT_ISSUES.md`, `scripts/README.md`, any directly matched docs/examples

- [x] 5. Run full regression, integration, and repo-scan verification

  **What to do**: Execute the repository test commands appropriate to this change: `go test ./protocol/vless`, `go test ./...`, and the integration test step from `Makefile` (`cd test && go test -tags "$(TAGS_TEST)" .`). Run targeted searches to prove there are no surviving VLESS pool code or config references. Capture all outputs as evidence.
  **Must NOT do**: Do not skip failing commands, trim outputs, or replace real results with summaries. If unrelated pre-existing failures appear, clearly separate them from this change and record exact output.

  **Recommended Agent Profile**:
  - Category: `unspecified-low` - Reason: command-heavy verification with possible triage.
  - Skills: `[]` - No special skill required.
  - Omitted: `['/review-work']` - Final verification wave handles review separately.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 6 | Blocked By: 2, 3

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `Makefile:225-235` - Canonical `test` and `test_stdio` command structure.
  - Pattern: `protocol/vless/outbound.go:155-173` - Removal should not break reset/close lifecycle.
  - Pattern: `protocol/vless/integration_test.go:42-302` - Test surface expected to change from pool-positive to removal regression coverage.

  **Acceptance Criteria** (agent-executable only):
  - [ ] `go test ./protocol/vless` passes.
  - [ ] `go test ./...` passes, or any unrelated pre-existing failures are proven with exact logs and isolated from VLESS pool removal.
  - [ ] `cd test && go test -tags "$(TAGS_TEST)" .` passes, or unrelated pre-existing failures are isolated with evidence.
  - [ ] Repo scans show no surviving VLESS pool implementation/config references.

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Full verification passes or isolates unrelated breakage
    Tool: Bash
    Steps: Run `go test ./protocol/vless && go test ./... && (cd test && go test -tags "$(TAGS_TEST)" .) | tee .sisyphus/evidence/task-5-full-verification.txt`
    Expected: Commands pass, or the output clearly proves any failure is unrelated to VLESS pool removal.
    Evidence: .sisyphus/evidence/task-5-full-verification.txt

  Scenario: Repo scan proves feature removal completeness
    Tool: Bash
    Steps: Run `grep -R -n 'connection_pool\|ConnectionPoolOptions\|NewConnectionPool\|EnsureIdleSession' protocol/vless option CURRENT_ISSUES.md scripts README.md docs test | tee .sisyphus/evidence/task-5-full-verification-error.txt`
    Expected: No unexpected VLESS pool references remain.
    Evidence: .sisyphus/evidence/task-5-full-verification-error.txt
  ```

  **Commit**: NO | Message: `n/a` | Files: none

- [x] 6. Package final evidence and prepare verification handoff

  **What to do**: Review the branch diff to confirm only intended VLESS/runtime/test/doc files changed, normalize evidence file names, and prepare a concise executor handoff summarizing: removed files, changed files, legacy-config rejection behavior, and any unrelated failures discovered during verification. Ensure the branch is ready for the mandatory Final Verification Wave.
  **Must NOT do**: Do not add new code changes except tiny evidence/documentation clarifications strictly required to complete verification.

  **Recommended Agent Profile**:
  - Category: `quick` - Reason: narrow wrap-up and hygiene task.
  - Skills: `[]` - No special skill required.
  - Omitted: `['/git-master']` - No commit/rebase work is required here.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: F1-F4 | Blocked By: 2, 3, 4, 5

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `CURRENT_ISSUES.md:110-148` - One of the tracked stale-reference targets.
  - Pattern: `Makefile:225-235` - Verification outputs should align with the repository's existing test entry points.
  - Pattern: `.sisyphus/evidence/` - Existing evidence-directory convention in this repo.

  **Acceptance Criteria** (agent-executable only):
  - [ ] Final diff is limited to intended runtime/schema/test/doc cleanup for VLESS pool removal.
  - [ ] Evidence files exist for Tasks 1-5 and are named consistently.
  - [ ] Handoff summary explicitly states how legacy `connection_pool` configs now fail.

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Final diff stays within intended scope
    Tool: Bash
    Steps: Run `git diff --stat | tee .sisyphus/evidence/task-6-handoff.txt`
    Expected: Diff touches only VLESS pool removal, related tests, and intentional stale-reference cleanup.
    Evidence: .sisyphus/evidence/task-6-handoff.txt

  Scenario: Evidence set is complete
    Tool: Bash
    Steps: Run `ls .sisyphus/evidence/task-{1,2,3,4,5}* | tee .sisyphus/evidence/task-6-handoff-error.txt`
    Expected: All prior task evidence files are present.
    Evidence: .sisyphus/evidence/task-6-handoff-error.txt
  ```

  **Commit**: NO | Message: `n/a` | Files: none

## Final Verification Wave (MANDATORY — after ALL implementation tasks)
> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.
- [x] F1. Plan Compliance Audit — oracle
- [x] F2. Code Quality Review — unspecified-high
- [x] F3. Real Manual QA — unspecified-high (+ playwright if UI)
- [x] F4. Scope Fidelity Check — deep

## Commit Strategy
- Prefer one commit per implementation task that changes code/docs materially:
  - `refactor(vless): remove connection pool options`
  - `refactor(vless): drop pooled dialing runtime`
  - `test(vless): replace connection pool coverage`
  - `docs(vless): remove stale connection pool references`
- Do not create commits for pure verification/evidence tasks.
- If the executor combines Tasks 1-3 into one atomic code change for bisectability, use: `refactor(vless): remove connection pool support`.

## Success Criteria
- VLESS outbound no longer contains a connection pool implementation, config surface, or pool-specific tests.
- Legacy VLESS `connection_pool` configs are explicitly rejected.
- Standard VLESS outbound behavior remains intact for multiplex, TLS, transport, TCP, and UDP code paths unrelated to pooling.
- Repo references no longer instruct maintainers to wire the feature back in.
- Final Verification Wave approves the work and the user explicitly says okay.
