# sing-box Actual Upstream v1.13.11 Merge

## TL;DR
> **Summary**: Merge upstream `v1.13.11` into the current fork head using a true three-way merge, then reconcile dependency, schema, protocol, transport, and mobile/runtime conflicts while preserving explicitly-owned fork-only behavior.
> **Deliverables**:
> - Real upstream merge commit or equivalent resolved merge state based on `v1.13.11`
> - Preserved fork-only surfaces with explicit conflict decisions
> - Updated dependency graph and package-level verification evidence
> - Consolidated final verification proving the merge adopted upstream changes rather than preserving stale fork code
> **Effort**: XL
> **Parallel**: YES - 5 waves, with maximum safe Wave-1 fan-out after one serial choke point
> **Critical Path**: Task 1 → (Tasks 2-7 in parallel) → Task 8 → (Tasks 9-14 in parallel) → (Tasks 15-18 in parallel) → Task 19 → Task 20

## Context
### Original Request
- Merge the real upstream `v1.13.11` changes into this fork.
- Preserve fork business logic rather than replacing it blindly.
- Maximize safe parallelism and start as many agents as possible in one wave.

### Interview Summary
- Prior work mostly produced annotations, reports, and small fixes rather than the actual upstream merge.
- Current branch is `master` at `30cecec5` and tracks `origin/master`.
- Fork previously merged upstream `v1.13.2`; the actual merge target now is upstream tag `v1.13.11` at `553cfa1f`.
- The merge surface is large and concentrated in `go.mod`, `option/`, `route/`, `protocol/`, `transport/`, and `experimental/libbox/`.
- User explicitly asked for maximum safe parallelism in the earliest possible wave.

### Metis Review (gaps addressed)
- Corrected the premise: this plan is for the real upstream merge, not annotation work.
- Added an explicit serial dependency choke point before high-concurrency Wave 1 execution.
- Added guardrails to prevent executors from silently keeping stale fork code where upstream behavior should win.
- Added preservation checks for fork-forward commits and fork-only surfaces such as `sing-anytls` replace, VLESS pool removal, and fork-only transports.

## Work Objectives
### Core Objective
Integrate upstream `v1.13.11` into the fork with a real merge-based migration that preserves intentional fork-only behavior and adopts upstream changes everywhere else.

### Deliverables
- Resolved merge state against upstream `v1.13.11`
- Reconciled `go.mod` / `go.sum` / dependency graph
- Reconciled schema/runtime/protocol/transport/mobile surfaces
- Evidence bundle per task under `.sisyphus/evidence/`
- Final verification report proving upstream adoption and fork preservation decisions

### Definition of Done (verifiable conditions with commands)
- `GIT_MASTER=1 git merge-base HEAD v1.13.11` is no longer the pre-merge divergence point and the repository contains the resolved upstream merge state
- `go build ./option/... ./route/... ./protocol/group/... ./protocol/vless/... ./experimental/libbox/...` passes
- `go test ./option/... ./route/... ./protocol/group/... ./protocol/vless/... ./experimental/libbox/...` passes
- `cd test && go test -run 'TestDirect|TestBox$' .` passes
- `git diff --name-only v1.13.11..HEAD` shows only intended fork-preservation and post-merge changes, not accidental omission of upstream surfaces
- All explicitly preserved fork-only surfaces remain present and verified after merge

### Must Have
- Real three-way merge strategy based on upstream `v1.13.11`
- Upstream-first conflict resolution except for explicitly-owned fork-only surfaces
- Single-writer ownership for dependency, schema, and core route/protocol choke points
- Maximum safe parallelism after the dependency baseline task completes
- Explicit handling of pre-existing build/test blockers so they do not mask merge regressions

### Must NOT Have (guardrails, AI slop patterns, scope boundaries)
- Must NOT rebase or cherry-pick upstream history as the primary strategy
- Must NOT substitute docs/annotations for real upstream adoption work
- Must NOT allow `go mod tidy` to silently decide merge conflicts in `go.mod`
- Must NOT drop `replace github.com/anytls/sing-anytls => github.com/KexiChanProjectProxy/sing-anytls v0.0.13` without explicit justification and verification
- Must NOT reintroduce VLESS connection-pool support removed by the fork unless the plan explicitly changes that decision
- Must NOT treat pre-existing blockers in `transport/v2rayxhttp`, `protocol/tailscale`, `service/derp`, quic-go/qpack, Docker/uTLS integration, or `common/tlsfragment` as proof the upstream merge itself failed

## Verification Strategy
> ZERO HUMAN INTERVENTION - all verification is agent-executed.
- Test decision: tests-after + targeted package gates first, full-suite only after blocker isolation
- QA policy: Every task includes agent-executed happy-path and failure/edge scenarios
- Evidence: `.sisyphus/evidence/task-{N}-{slug}.{ext}`
- Reliable early verification commands:
  - `go build ./option/... ./route/... ./protocol/group/... ./protocol/vless/... ./experimental/libbox/...`
  - `go test ./option/... ./route/... ./protocol/group/... ./protocol/vless/... ./experimental/libbox/...`
  - `cd test && go test -run 'TestDirect|TestBox$' .`
- Known non-gating pre-existing blockers unless the merge changes them deliberately:
  - `transport/v2rayxhttp` missing `option.V2RayXHTTPOptions` / related types
  - `protocol/tailscale` / `service/derp` missing `tun.ICMPForwarder`, `tun.DefaultNIC`, `tun.NewICMPForwarder`, `ping.ConnectGVisor`
  - quic-go/qpack mismatch in HTTP/3 paths
  - `common/tlsfragment` handshake failure
  - Docker/uTLS dependent integration tests under `test/`

## Execution Strategy
### Parallel Execution Waves
> Target: 5-8 tasks per wave. <3 per wave (except final) = under-splitting.
> Wave 1 is maximally parallel only AFTER the single-writer dependency baseline is complete.

Wave 0: serial baseline choke point
- Task 1: establish merge base and reconcile dependency baseline

Wave 1: maximum safe fan-out immediately after baseline
- Tasks 2, 3, 4, 5, 6, 7 in parallel

Wave 2: core conflict resolution after reconnaissance converges
- Task 8 serial core merge checkpoint
- Tasks 9, 10, 11, 12, 13, 14 in parallel

Wave 3: downstream integrations and fork-surface adoption
- Tasks 15, 16, 17, 18 in parallel

Wave 4: integration and blocker accounting
- Task 19: consolidated merge verification and blocker isolation
- Task 20: final cleanup of stale conflict decisions and evidence rollup

### Dependency Matrix (full, all tasks)
| Task | Depends On | Blocks |
|---|---|---|
| 1 | none | 2-20 |
| 2 | 1 | 8,9 |
| 3 | 1 | 8,10,11 |
| 4 | 1 | 8,12 |
| 5 | 1 | 8,10,13 |
| 6 | 1 | 8,14,17 |
| 7 | 1 | 8,15,18 |
| 8 | 2,3,4,5,6,7 | 9-20 |
| 9 | 8,2 | 19 |
| 10 | 8,3,5 | 19 |
| 11 | 8,3 | 19 |
| 12 | 8,4 | 19 |
| 13 | 8,5 | 19 |
| 14 | 8,6 | 19 |
| 15 | 8,7 | 19 |
| 16 | 8,9,10,11 | 19 |
| 17 | 8,6 | 19 |
| 18 | 8,7 | 19 |
| 19 | 9-18 | 20,F1-F4 |
| 20 | 19 | F1-F4 |

### Agent Dispatch Summary (wave → task count → categories)
- Wave 0 → 1 task → `deep`
- Wave 1 → 6 tasks → `deep` (5), `unspecified-high` (1)
- Wave 2 → 7 tasks → `deep` (5), `unspecified-high` (2)
- Wave 3 → 4 tasks → `deep` (2), `unspecified-high` (2)
- Wave 4 → 2 tasks → `deep` (1), `unspecified-high` (1)
- Final Verification → 4 parallel review agents

## TODOs
> Implementation + Test = ONE task. Never separate.
> EVERY task MUST have: Agent Profile + Parallelization + QA Scenarios.

- [x] 1. Establish merge baseline and reconcile dependency choke point

  **What to do**: Fetch and merge upstream `v1.13.11` into current `master` in a dedicated single-writer zone. Resolve the initial `git merge` state, then manually reconcile `go.mod` / `go.sum` without letting `go mod tidy` silently choose winners. Preserve the fork's `sing-anytls` replace directive unless a verified incompatibility requires a documented alternative. Record which upstream dependency version wins for each sagernet core module and why.
  **Must NOT do**: Must NOT rebase. Must NOT cherry-pick 99 commits. Must NOT use `go mod tidy` as the primary conflict resolver. Must NOT drop fork-only dependency customizations without explicit verification.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: single highest-risk serialization point controlling all downstream compile behavior
  - Skills: `[]` - no additional skill required
  - Omitted: [`git-master`] - executor work is broader than git-only operations

  **Parallelization**: Can Parallel: NO | Wave 0 | Blocks: 2-20 | Blocked By: none

  **References** (executor has NO interview context - be exhaustive):
  - Current head: `30cecec5`
  - Target upstream tag: `v1.13.11` at `553cfa1f`
  - Prior upstream merge precedent: commit `240232e4`
  - Dependency file: `go.mod:1-195`
  - Required fork replace directive: `go.mod:195`
  - Test target definitions: `Makefile:225-254`
  - Known blocker document: `CURRENT_ISSUES.md`

  **Acceptance Criteria** (agent-executable only):
  - [ ] Repository is in a resolved post-merge state against upstream `v1.13.11`
  - [ ] `go.mod` retains explicitly approved fork-only customizations and adopts upstream dependency versions where appropriate
  - [ ] `go build ./option/... ./route/... ./protocol/group/... ./protocol/vless/... ./experimental/libbox/...` succeeds or fails only for documented pre-existing blockers outside these targets

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Merge and dependency baseline happy path
    Tool: Bash
    Steps: Perform merge against `v1.13.11`, resolve `go.mod`/`go.sum`, then run `go build ./option/... ./route/... ./protocol/group/... ./protocol/vless/... ./experimental/libbox/...`
    Expected: Merge state resolves and targeted core build gate passes
    Evidence: .sisyphus/evidence/task-1-merge-baseline.txt

  Scenario: Replace directive preservation edge case
    Tool: Bash
    Steps: Verify merged `go.mod` still contains approved `sing-anytls` replace or contains a documented, tested replacement decision
    Expected: No silent loss of fork dependency ownership
    Evidence: .sisyphus/evidence/task-1-merge-baseline-edge.txt
  ```

  **Commit**: YES | Message: `merge(upstream): bring fork baseline to v1.13.11` | Files: `go.mod`, `go.sum`, merge-resolved files across repo

- [x] 2. Inventory upstream-vs-fork dependency deltas after merge baseline

  **What to do**: Enumerate the post-merge dependency winners and losers across `go.mod`, with special attention to `github.com/sagernet/sing`, `sing-quic`, `sing-tun`, `tailscale`, `quic-go`, `qpack`, `gin`, `metacubex/tfo-go`, and the `sing-anytls` replace. Produce a precise reconciliation note in evidence so downstream agents use the same dependency truth.
  **Must NOT do**: Must NOT change code outside dependency-related files for this task.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: dependency truth source for all downstream execution
  - Skills: `[]` - no additional skill required
  - Omitted: [`playwright`] - no UI interaction needed

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: 8,9 | Blocked By: 1

  **References**:
  - `go.mod:1-195`
  - Upstream merge result from Task 1
  - `CURRENT_ISSUES.md`

  **Acceptance Criteria**:
  - [ ] Evidence clearly records final dependency/version decisions and any preserved fork-only replacements
  - [ ] Downstream agents can consume this dependency matrix without re-deciding versions

  **QA Scenarios**:
  ```
  Scenario: Dependency matrix extraction
    Tool: Bash
    Steps: Diff merged `go.mod` against pre-merge HEAD and upstream `v1.13.11`; record version winners and fork-only exceptions
    Expected: One authoritative dependency decision record exists
    Evidence: .sisyphus/evidence/task-2-dependency-deltas.txt

  Scenario: Dependency drift edge case
    Tool: Bash
    Steps: Verify no unexpected removal of fork-only direct dependencies or replace directives
    Expected: All removals are explicit and justified
    Evidence: .sisyphus/evidence/task-2-dependency-deltas-edge.txt
  ```

  **Commit**: NO | Message: `n/a` | Files: evidence only

- [x] 3. Reconcile option schema conflicts and missing transport option types

  **What to do**: Resolve schema conflicts under `option/` created by the upstream merge. Determine whether upstream `v1.13.11` now provides equivalents for `V2RayXHTTPOptions`, `V2RayXHTTPXmuxConfig`, and `V2RayXHTTPRangeConfig`; if not, create or adapt the fork-owned schema contract so `transport/v2rayxhttp/` is no longer structurally orphaned. Reconcile all JSON field differences with upstream-first rules except where fork-only behavior is intentional.
  **Must NOT do**: Must NOT delete fork-only transport schema just because upstream lacks it. Must NOT invent parallel field names when upstream already defines the semantics.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: schema is a contract-defining choke point for multiple packages
  - Skills: `[]`
  - Omitted: [`review-work`] - reserved for final verification

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: 8,10,11 | Blocked By: 1

  **References**:
  - `transport/v2rayxhttp/client.go:28-54`
  - `option/` package after merge
  - `CURRENT_ISSUES.md` section on missing `V2RayXHTTP*` option types

  **Acceptance Criteria**:
  - [ ] `option/...` compiles after merge
  - [ ] Missing `V2RayXHTTP*` option-type failures are either resolved or explicitly isolated with code-level ownership documented
  - [ ] Upstream field semantics win unless fork-only transport requires extension

  **QA Scenarios**:
  ```
  Scenario: Option schema compile gate
    Tool: Bash
    Steps: Run `go build ./option/...` and targeted transport-related package builds after schema reconciliation
    Expected: Option package compiles cleanly and transport option contracts are defined consistently
    Evidence: .sisyphus/evidence/task-3-option-schema.txt

  Scenario: Missing xhttp option edge case
    Tool: Bash
    Steps: Build `transport/v2rayxhttp/...` or explicitly isolate remaining failures to non-schema code if any persist
    Expected: No undefined `V2RayXHTTP*` option symbols remain
    Evidence: .sisyphus/evidence/task-3-option-schema-edge.txt
  ```

  **Commit**: YES | Message: `refactor(option): reconcile upstream schema with fork transport contracts` | Files: `option/`, transport option consumers

- [x] 4. Reconcile route core merge conflicts and preserve explicit fork router fields

  **What to do**: Resolve upstream merge conflicts in route core files, especially `route/router.go`, using upstream behavior as the default. Preserve explicit fork-only fields and behaviors that are structurally owned by the fork, including ASN/geosite readers and `sniffOverrideDestination`, but do not retain stale fork flow if upstream has superseded the same logic.
  **Must NOT do**: Must NOT keep fork route code wholesale when upstream changed execution order. Must NOT remove explicit fork-only fields without documenting why.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: route core is a conflict-heavy semantic choke point
  - Skills: `[]`
  - Omitted: [`git-master`] - work is semantic, not just git conflict operation

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: 8,12 | Blocked By: 1

  **References**:
  - `route/router.go:25-49`
  - route package post-merge
  - prior fork custom marker: `route/router.go:43-48`

  **Acceptance Criteria**:
  - [ ] `go build ./route/...` passes
  - [ ] Upstream route behavior changes are adopted unless explicitly overridden by fork-owned fields/features
  - [ ] Fork-owned route fields remain correctly wired after merge

  **QA Scenarios**:
  ```
  Scenario: Route core merge happy path
    Tool: Bash
    Steps: Run `go build ./route/... && go test ./route/...`
    Expected: Route package compiles and tests pass with merged upstream behavior
    Evidence: .sisyphus/evidence/task-4-route-core.txt

  Scenario: Fork route field preservation edge case
    Tool: Bash
    Steps: Verify `asnReader`, `geositeReader`, and `sniffOverrideDestination` remain present and reachable where intended
    Expected: Fork-owned route extensions are not silently dropped
    Evidence: .sisyphus/evidence/task-4-route-core-edge.txt
  ```

  **Commit**: YES | Message: `refactor(route): reconcile upstream v1.13.11 route core` | Files: `route/`, route-owned helpers

- [x] 5. Reconcile VLESS outbound and preserve pool-removal decision

  **What to do**: Merge upstream changes into `protocol/vless/` while preserving the fork’s explicit removal of connection-pool support. Apply upstream non-pool improvements, logging, transport, and packet handling updates where compatible, but do not reintroduce `connection_pool` or equivalent runtime behavior.
  **Must NOT do**: Must NOT re-add VLESS pool support. Must NOT ignore upstream non-pool fixes just because the fork removed one feature.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: protocol semantics plus explicit fork design decision
  - Skills: `[]`
  - Omitted: [`playwright`] - no browser interaction required

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: 8,10,13 | Blocked By: 1

  **References**:
  - `protocol/vless/outbound.go:26-228`
  - fork commit note: `bd8d2078 refactor(vless): remove connection pool support`
  - `protocol/vless/...` tests

  **Acceptance Criteria**:
  - [ ] `go build ./protocol/vless/... && go test ./protocol/vless/...` passes
  - [ ] No VLESS runtime code reintroduces connection-pool support
  - [ ] Upstream v1.13.11 VLESS improvements unrelated to pool support are adopted

  **QA Scenarios**:
  ```
  Scenario: VLESS outbound merge happy path
    Tool: Bash
    Steps: Run `go build ./protocol/vless/... && go test ./protocol/vless/...`
    Expected: VLESS packages compile and tests pass after merge reconciliation
    Evidence: .sisyphus/evidence/task-5-vless.txt

  Scenario: Pool-removal preservation edge case
    Tool: Bash
    Steps: Search merged VLESS runtime for `connection_pool` or equivalent reintroduced pool wiring
    Expected: Fork removal decision remains intact
    Evidence: .sisyphus/evidence/task-5-vless-edge.txt
  ```

  **Commit**: YES | Message: `refactor(vless): merge upstream changes without restoring pool support` | Files: `protocol/vless/`, related tests

- [x] 6. Reconcile fork-only transports against upstream interfaces

  **What to do**: Reconcile `transport/v2rayxhttp/` and `transport/v2raycloudflared/` against the merged upstream interface contracts. Preserve these fork-only transports, but adapt them to upstream option, transport, and dialer interfaces so they no longer drift structurally.
  **Must NOT do**: Must NOT delete these directories because upstream lacks them. Must NOT force upstream abstractions backward to match stale fork code if the fork transport can be adapted locally.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: fork-only transport surfaces with interface drift risk
  - Skills: `[]`
  - Omitted: [`git-master`] - interface adaptation task

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: 8,14,17 | Blocked By: 1

  **References**:
  - `transport/v2rayxhttp/client.go:28-176`
  - `transport/v2raycloudflared/`
  - merged transport interfaces and option contracts

  **Acceptance Criteria**:
  - [ ] Fork-only transport packages compile or are isolated to explicitly documented remaining non-interface blockers
  - [ ] No undefined option/interface symbols remain in fork transport code
  - [ ] Upstream transport contracts are consumed rather than replaced

  **QA Scenarios**:
  ```
  Scenario: Fork transport compile gate
    Tool: Bash
    Steps: Run `go build ./transport/...` or focused transport package builds after interface reconciliation
    Expected: Fork transport packages either compile or fail only on documented external blockers
    Evidence: .sisyphus/evidence/task-6-transports.txt

  Scenario: Transport contract drift edge case
    Tool: Bash
    Steps: Verify no references remain to removed or renamed upstream interfaces after merge adaptation
    Expected: Transport code is structurally aligned with merged upstream contracts
    Evidence: .sisyphus/evidence/task-6-transports-edge.txt
  ```

  **Commit**: YES | Message: `refactor(transport): adapt fork transports to merged upstream contracts` | Files: `transport/`, transport-owned consumers

- [x] 7. Reconcile libbox and build-wrapper ownership boundaries

  **What to do**: Assess and reconcile the ABI/runtime surfaces under `experimental/libbox/` plus build wrapper hooks referenced by `Makefile`. This task is reconnaissance plus minimal adaptation ownership definition for downstream merge tasks; it must identify which libbox/build surfaces changed upstream and which are still fork-owned.
  **Must NOT do**: Must NOT mix dependency or route logic into this task. Must NOT make speculative ABI changes before core merge checkpoint Task 8.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: broad but bounded ABI/build ownership mapping
  - Skills: `[]`
  - Omitted: [`review-work`] - final review only

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: 8,15,18 | Blocked By: 1

  **References**:
  - `experimental/libbox/...`
  - `Makefile:237-254`

  **Acceptance Criteria**:
  - [ ] Evidence records libbox/runtime/build ownership boundaries after merge baseline
  - [ ] `go build ./experimental/libbox/... && go test ./experimental/libbox/...` passes or remaining failures are explicitly isolated

  **QA Scenarios**:
  ```
  Scenario: Libbox ownership verification
    Tool: Bash
    Steps: Run `go build ./experimental/libbox/... && go test ./experimental/libbox/...`
    Expected: Libbox surfaces build/test cleanly or yield clearly isolated blockers
    Evidence: .sisyphus/evidence/task-7-libbox-scout.txt

  Scenario: Build-wrapper boundary edge case
    Tool: Bash
    Steps: Compare libbox runtime surfaces against `Makefile` mobile targets and record mismatches
    Expected: Downstream tasks know exactly which build wrappers must be adapted
    Evidence: .sisyphus/evidence/task-7-libbox-scout-edge.txt
  ```

  **Commit**: NO | Message: `n/a` | Files: evidence only unless minimal ABI alignment is required

- [x] 8. Execute the core merge checkpoint across shared choke points

  **What to do**: Integrate the results of Tasks 2-7 into one stabilized checkpoint that resolves cross-cutting conflicts across `go.mod`, `option/`, `route/`, `protocol/vless/`, fork-only transports, and libbox boundary assumptions. This task decides final shared contracts so downstream tasks can run in parallel without reopening them.
  **Must NOT do**: Must NOT leave unresolved shared contract ambiguities. Must NOT defer `go.mod`, `option/`, or route-core decisions further downstream.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: authoritative merge-integration checkpoint
  - Skills: `[]`
  - Omitted: [`playwright`] - no browser verification required

  **Parallelization**: Can Parallel: NO | Wave 2 | Blocks: 9-20 | Blocked By: 2-7

  **References**:
  - Outputs/evidence from Tasks 2-7
  - `go.mod`, `option/`, `route/`, `protocol/vless/`, `transport/`, `experimental/libbox/`

  **Acceptance Criteria**:
  - [ ] Shared contracts are stabilized and documented in evidence
  - [ ] Targeted build/test gates for the stabilized core all pass together
  - [ ] Downstream tasks no longer need to make dependency/schema/route-core design decisions

  **QA Scenarios**:
  ```
  Scenario: Core checkpoint build/test gate
    Tool: Bash
    Steps: Run `go build ./option/... ./route/... ./protocol/group/... ./protocol/vless/... ./experimental/libbox/...` and `go test ./option/... ./route/... ./protocol/group/... ./protocol/vless/... ./experimental/libbox/...`
    Expected: Shared contract surfaces pass together
    Evidence: .sisyphus/evidence/task-8-core-checkpoint.txt

  Scenario: Cross-surface ambiguity edge case
    Tool: Bash
    Steps: Verify no unresolved merge markers, TODO placeholders, or duplicate temporary compatibility shims remain across shared surfaces
    Expected: Core merge checkpoint is clean and authoritative
    Evidence: .sisyphus/evidence/task-8-core-checkpoint-edge.txt
  ```

  **Commit**: YES | Message: `refactor(core): stabilize shared contracts after upstream merge` | Files: shared core surfaces touched by 2-7

- [x] 9. Reconcile protocol/group runtime against merged core contracts

  **What to do**: Merge/adapt `protocol/group/` runtime behavior to the stabilized core checkpoint, preserving fork load-balance/group behaviors only where explicitly intended.
  **Must NOT do**: Must NOT redefine option schema or route-core behavior locally.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: runtime strategy behavior depends on stabilized core contracts
  - Skills: `[]`
  - Omitted: [`review-work`]

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 19 | Blocked By: 8,2

  **References**:
  - `protocol/group/...`
  - Task 8 checkpoint

  **Acceptance Criteria**:
  - [ ] `go build ./protocol/group/... && go test ./protocol/group/...` passes
  - [ ] No local redefinition of shared contracts remains

  **QA Scenarios**:
  ```
  Scenario: Group runtime happy path
    Tool: Bash
    Steps: Run `go build ./protocol/group/... && go test ./protocol/group/...`
    Expected: Group runtime compiles and tests pass against merged core contracts
    Evidence: .sisyphus/evidence/task-9-group-runtime.txt

  Scenario: Shared contract drift edge case
    Tool: Bash
    Steps: Verify group runtime consumes Task 8 contracts without reintroducing schema forks
    Expected: No duplicate contract logic remains
    Evidence: .sisyphus/evidence/task-9-group-runtime-edge.txt
  ```

  **Commit**: YES | Message: `refactor(group): align runtime with merged upstream core` | Files: `protocol/group/`

- [x] 10. Reconcile direct and DNS-coupled route consumers

  **What to do**: Adapt direct outbound and DNS-coupled downstream consumers to the stabilized route/option contracts. Preserve fork IPv6 source range and related direct-path behavior where explicitly intended.
  **Must NOT do**: Must NOT reopen VLESS or route-core decisions.

  **Recommended Agent Profile**:
  - Category: `deep`
  - Skills: `[]`
  - Omitted: [`git-master`]

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 19 | Blocked By: 8,3,5

  **References**:
  - `protocol/direct/`
  - `dns/`
  - direct IPv6 source range fork commit context

  **Acceptance Criteria**:
  - [ ] Targeted direct and DNS packages compile/test against merged core
  - [ ] Fork IPv6 source-range behavior remains present if still in scope

  **QA Scenarios**:
  ```
  Scenario: Direct and DNS consumer gate
    Tool: Bash
    Steps: Run targeted builds/tests for direct/dns packages after contract adaptation
    Expected: Direct and DNS consumers compile/test without reopening core contracts
    Evidence: .sisyphus/evidence/task-10-direct-dns.txt

  Scenario: IPv6 source-range preservation edge case
    Tool: Bash
    Steps: Verify direct-path code still contains intended IPv6 source-range logic after merge
    Expected: Fork direct customization is preserved intentionally
    Evidence: .sisyphus/evidence/task-10-direct-dns-edge.txt
  ```

  **Commit**: YES | Message: `refactor(direct): adapt direct and dns consumers to merged contracts` | Files: `protocol/direct/`, `dns/`, related consumers

- [x] 11. Reconcile logging/event surfaces with merged upstream behavior

  **What to do**: Reconcile fork structured logging/event surfaces against upstream `v1.13.11`, preserving the fork's tests and intentional event extensions where they still map cleanly.
  **Must NOT do**: Must NOT preserve a parallel event taxonomy if upstream now models the same signal.

  **Recommended Agent Profile**:
  - Category: `unspecified-high`
  - Skills: `[]`
  - Omitted: [`playwright`]

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 19 | Blocked By: 8,3

  **References**:
  - `log/`
  - fork test additions around structured events

  **Acceptance Criteria**:
  - [ ] `go build ./log/... && go test ./log/...` passes if package exists in scope
  - [ ] Fork event tests remain meaningful after upstream reconciliation

  **QA Scenarios**:
  ```
  Scenario: Logging reconciliation happy path
    Tool: Bash
    Steps: Run targeted logging package builds/tests after reconciliation
    Expected: Logging surfaces compile/test with merged event semantics
    Evidence: .sisyphus/evidence/task-11-logging.txt

  Scenario: Duplicate taxonomy edge case
    Tool: Bash
    Steps: Verify no duplicate fork-only event model survives where upstream provides the same semantic type
    Expected: One coherent event model remains
    Evidence: .sisyphus/evidence/task-11-logging-edge.txt
  ```

  **Commit**: YES | Message: `refactor(log): reconcile fork events with upstream v1.13.11` | Files: `log/`, event consumers

- [x] 12. Reconcile rule and matcher surfaces against merged route core

  **What to do**: Adapt rule and matcher surfaces to the stabilized route core so route-time metadata and fork matcher behavior remain coherent after upstream merge.
  **Must NOT do**: Must NOT re-thread route order or add new metadata fields without explicit need.

  **Recommended Agent Profile**:
  - Category: `deep`
  - Skills: `[]`
  - Omitted: [`review-work`]

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 19 | Blocked By: 8,4

  **References**:
  - `adapter/rule.go`, `route/rule/`, matcher-related surfaces

  **Acceptance Criteria**:
  - [ ] Rule/matcher packages compile/test against merged route core
  - [ ] No stale pre-v1.13.11 route assumptions survive in matcher logic

  **QA Scenarios**:
  ```
  Scenario: Rule/matcher gate
    Tool: Bash
    Steps: Run targeted builds/tests for rule and matcher surfaces
    Expected: Rule and matcher logic compiles/tests cleanly post-merge
    Evidence: .sisyphus/evidence/task-12-rules.txt

  Scenario: Metadata drift edge case
    Tool: Bash
    Steps: Verify matcher logic uses the merged route metadata model rather than stale fork-only assumptions
    Expected: One coherent metadata path remains
    Evidence: .sisyphus/evidence/task-12-rules-edge.txt
  ```

  **Commit**: YES | Message: `refactor(rule): align matcher surfaces with merged route core` | Files: rule and matcher surfaces

- [x] 13. Reconcile V2Ray transport consumers outside VLESS

  **What to do**: Adapt non-VLESS V2Ray transport consumers to the stabilized transport/option contracts after the main protocol checkpoint.
  **Must NOT do**: Must NOT reopen transport schema decisions from Tasks 3 or 6.

  **Recommended Agent Profile**:
  - Category: `unspecified-high`
  - Skills: `[]`
  - Omitted: [`git-master`]

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 19 | Blocked By: 8,5

  **References**:
  - `transport/v2ray/`, dependent protocol consumers

  **Acceptance Criteria**:
  - [ ] Dependent transport consumers compile after merged contract adoption
  - [ ] No duplicate compatibility shims remain

  **QA Scenarios**:
  ```
  Scenario: V2Ray consumer gate
    Tool: Bash
    Steps: Run targeted builds for V2Ray transport consumers after reconciliation
    Expected: Dependent consumers compile against the stabilized transport contracts
    Evidence: .sisyphus/evidence/task-13-v2ray-consumers.txt

  Scenario: Compatibility shim edge case
    Tool: Bash
    Steps: Verify no temporary compatibility wrappers remain duplicated in consumer packages
    Expected: Consumer packages use final contracts directly
    Evidence: .sisyphus/evidence/task-13-v2ray-consumers-edge.txt
  ```

  **Commit**: YES | Message: `refactor(transport): align downstream V2Ray consumers after merge` | Files: transport consumers outside VLESS

- [x] 14. Reconcile transport registration and adapter glue

  **What to do**: Align transport registration and adapter glue with merged upstream registration patterns, preserving fork-only transport registration where required.
  **Must NOT do**: Must NOT move protocol semantics into registration glue.

  **Recommended Agent Profile**:
  - Category: `unspecified-high`
  - Skills: `[]`
  - Omitted: [`review-work`]

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 19 | Blocked By: 8,6

  **References**:
  - `transport/`, registration-oriented `protocol/` adapters

  **Acceptance Criteria**:
  - [ ] Registration-oriented packages compile with merged adapters
  - [ ] Fork-only transport registration remains explicit and minimal

  **QA Scenarios**:
  ```
  Scenario: Registration glue gate
    Tool: Bash
    Steps: Run targeted builds for registration-oriented packages
    Expected: Adapter and registration glue compile successfully
    Evidence: .sisyphus/evidence/task-14-registration.txt

  Scenario: Fork registration edge case
    Tool: Bash
    Steps: Verify fork-only transport registration remains wired without broad adapter divergence
    Expected: Registration changes stay isolated
    Evidence: .sisyphus/evidence/task-14-registration-edge.txt
  ```

  **Commit**: YES | Message: `refactor(adapter): align transport registration with merged upstream patterns` | Files: registration-oriented transport/protocol glue

- [x] 15. Reconcile libbox runtime with merged core and protocol surfaces

  **What to do**: Adapt `experimental/libbox/` runtime surfaces to the merged core/protocol contracts after Task 8.
  **Must NOT do**: Must NOT change wrapper/build surfaces that belong to Task 18.

  **Recommended Agent Profile**:
  - Category: `deep`
  - Skills: `[]`
  - Omitted: [`git-master`]

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: 19 | Blocked By: 8,7

  **References**:
  - `experimental/libbox/...`

  **Acceptance Criteria**:
  - [ ] `go build ./experimental/libbox/... && go test ./experimental/libbox/...` passes
  - [ ] Runtime surfaces consume merged core/protocol contracts without ABI drift beyond documented intent

  **QA Scenarios**:
  ```
  Scenario: Libbox runtime gate
    Tool: Bash
    Steps: Run `go build ./experimental/libbox/... && go test ./experimental/libbox/...`
    Expected: Libbox runtime compiles/tests cleanly after merge adaptation
    Evidence: .sisyphus/evidence/task-15-libbox-runtime.txt

  Scenario: ABI drift edge case
    Tool: Bash
    Steps: Verify exported runtime-facing types/signatures remain intentionally compatible or are explicitly documented
    Expected: No accidental ABI drift
    Evidence: .sisyphus/evidence/task-15-libbox-runtime-edge.txt
  ```

  **Commit**: YES | Message: `refactor(libbox): align runtime with merged upstream core` | Files: `experimental/libbox/`

- [x] 16. Reconcile inbound/auth/sniffer behavior integrations

  **What to do**: Merge/adapt inbound and auth/sniffer-related protocol integrations to the stabilized route/adapter contracts.
  **Must NOT do**: Must NOT reopen route-core sequencing decisions.

  **Recommended Agent Profile**:
  - Category: `unspecified-high`
  - Skills: `[]`
  - Omitted: [`playwright`]

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: 19 | Blocked By: 8,9,10,11

  **References**:
  - inbound/protocol integration surfaces post-merge

  **Acceptance Criteria**:
  - [ ] Targeted inbound/protocol integration builds/tests pass
  - [ ] Auth/sniffer behavior remains consistent with merged core assumptions

  **QA Scenarios**:
  ```
  Scenario: Inbound integration gate
    Tool: Bash
    Steps: Run targeted builds/tests for inbound/auth/sniffer-related packages
    Expected: Integrations compile/test against merged route/adapter surfaces
    Evidence: .sisyphus/evidence/task-16-inbound.txt

  Scenario: Sniffer/auth edge case
    Tool: Bash
    Steps: Verify merged behavior does not rely on stale route-core assumptions
    Expected: Integration behavior remains coherent
    Evidence: .sisyphus/evidence/task-16-inbound-edge.txt
  ```

  **Commit**: YES | Message: `refactor(inbound): align auth and sniffer integrations after merge` | Files: inbound/protocol integration surfaces

- [x] 17. Reconcile tailscale/derp blocker surfaces without broadening scope

  **What to do**: Isolate and, if feasible within the merged dependency graph, reconcile `protocol/tailscale` and `service/derp` symbol/API mismatches. If full repair is out of scope for the upstream merge itself, convert the remaining failures into explicitly documented post-merge blockers with exact symbol-level evidence.
  **Must NOT do**: Must NOT let this task derail the main upstream merge if the failures remain pre-existing or upstream-external.

  **Recommended Agent Profile**:
  - Category: `deep`
  - Skills: `[]`
  - Omitted: [`review-work`]

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: 19 | Blocked By: 8,6

  **References**:
  - `protocol/tailscale/`
  - `service/derp/`
  - `CURRENT_ISSUES.md` tailscale/gvisor section

  **Acceptance Criteria**:
  - [ ] Remaining tailscale/derp failures are either fixed or precisely documented as out-of-scope blockers
  - [ ] No ambiguous "still broken" state remains

  **QA Scenarios**:
  ```
  Scenario: Tailscale/derp blocker assessment
    Tool: Bash
    Steps: Run targeted builds for `./protocol/tailscale/...` and `./service/derp/...`
    Expected: Either successful build or precise, reproducible documented blocker output
    Evidence: .sisyphus/evidence/task-17-tailscale-derp.txt

  Scenario: Scope-control edge case
    Tool: Bash
    Steps: Verify any unresolved failures are clearly marked as external/pre-existing rather than misattributed to unrelated merge work
    Expected: Blocker accounting remains honest and bounded
    Evidence: .sisyphus/evidence/task-17-tailscale-derp-edge.txt
  ```

  **Commit**: YES/NO | Message: `fix(tailscale): reconcile merged dependency API drift` | Files: tailscale/derp surfaces if changed

- [x] 18. Reconcile mobile/build wrapper surfaces after libbox stabilization

  **What to do**: Adapt build-wrapper/mobile targets in `Makefile` and related build helpers to the stabilized libbox/runtime surfaces.
  **Must NOT do**: Must NOT change runtime ABI here unless Task 15 documented the need.

  **Recommended Agent Profile**:
  - Category: `unspecified-high`
  - Skills: `[]`
  - Omitted: [`git-master`]

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: 19 | Blocked By: 8,7

  **References**:
  - `Makefile:237-254`
  - build helpers under `cmd/internal/build_libbox` and related wrapper paths

  **Acceptance Criteria**:
  - [ ] Relevant mobile/libbox build entrypoints are aligned with the stabilized runtime surface
  - [ ] Failures, if any, are limited to external toolchain prerequisites and documented as such

  **QA Scenarios**:
  ```
  Scenario: Build-wrapper verification
    Tool: Bash
    Steps: Run relevant libbox build entrypoints where environment permits, or verify command resolution paths and compile-time surfaces
    Expected: Build-wrapper layer matches the stabilized libbox runtime contracts
    Evidence: .sisyphus/evidence/task-18-build-wrappers.txt

  Scenario: Toolchain edge case
    Tool: Bash
    Steps: Distinguish code-level wrapper failures from missing external toolchain prerequisites
    Expected: Only real code issues are treated as merge work
    Evidence: .sisyphus/evidence/task-18-build-wrappers-edge.txt
  ```

  **Commit**: YES | Message: `build(libbox): align wrappers with merged runtime surface` | Files: build-wrapper surfaces

- [x] 19. Run consolidated merge verification and isolate residual blockers

  **What to do**: Execute the consolidated targeted verification suite, then run the broadest possible build/test commands without conflating residual pre-existing blockers with merge regressions. Produce one authoritative blocker ledger distinguishing fixed, merge-introduced, and pre-existing unresolved failures.
  **Must NOT do**: Must NOT claim success based only on narrow package checks if broader commands reveal new merge regressions.

  **Recommended Agent Profile**:
  - Category: `deep`
  - Skills: `[]`
  - Omitted: [`playwright`] - CLI/build-oriented verification only

  **Parallelization**: Can Parallel: NO | Wave 4 | Blocks: 20,F1-F4 | Blocked By: 9-18

  **References**:
  - All prior task evidence
  - `Makefile:225-229`
  - `CURRENT_ISSUES.md`

  **Acceptance Criteria**:
  - [ ] Targeted core verification suite passes
  - [ ] `cd test && go test -run 'TestDirect|TestBox$' .` passes
  - [ ] Broad build/test failures, if any, are categorized accurately as fixed / merge-induced / pre-existing residual blockers

  **QA Scenarios**:
  ```
  Scenario: Consolidated verification happy path
    Tool: Bash
    Steps: Run targeted build/test suite plus focused integration smoke in `test/`
    Expected: Core merged surfaces verify successfully end-to-end
    Evidence: .sisyphus/evidence/task-19-consolidated-verification.txt

  Scenario: Residual blocker classification edge case
    Tool: Bash
    Steps: Run broader commands such as `go build ./...` and classify each remaining failure against pre-merge blocker inventory
    Expected: No residual failure is mislabeled
    Evidence: .sisyphus/evidence/task-19-consolidated-verification-edge.txt
  ```

  **Commit**: NO | Message: `n/a` | Files: evidence/reporting only

- [x] 20. Final merge hygiene and stale-conflict cleanup

  **What to do**: Remove any leftover temporary compatibility code, stale conflict comments, obsolete annotations contradicted by the merged result, and temporary evidence-side notes that no longer reflect the final merged state. Ensure the repository reflects one coherent post-merge architecture.
  **Must NOT do**: Must NOT perform opportunistic unrelated cleanup.

  **Recommended Agent Profile**:
  - Category: `unspecified-high`
  - Skills: `[]`
  - Omitted: [`review-work`]

  **Parallelization**: Can Parallel: NO | Wave 4 | Blocks: F1-F4 | Blocked By: 19

  **References**:
  - All merged files touched by prior tasks
  - final blocker ledger from Task 19

  **Acceptance Criteria**:
  - [ ] No merge markers, stale temporary shims, or contradictory annotations remain
  - [ ] Final tree reflects one coherent merged design

  **QA Scenarios**:
  ```
  Scenario: Final hygiene pass
    Tool: Bash
    Steps: Search for merge markers, temporary compatibility notes, and stale comments after final integration
    Expected: No stale merge scaffolding remains
    Evidence: .sisyphus/evidence/task-20-final-hygiene.txt

  Scenario: Contradictory-annotation edge case
    Tool: Bash
    Steps: Verify preserved fork-only annotations/comments still describe real surviving behavior
    Expected: No annotation claims remain for code that upstream replaced or removed
    Evidence: .sisyphus/evidence/task-20-final-hygiene-edge.txt
  ```

  **Commit**: YES | Message: `chore(merge): clean final upstream integration leftovers` | Files: final cleanup surfaces only

## Final Verification Wave (MANDATORY — after ALL implementation tasks)
> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.
- [x] F1. Plan Compliance Audit — oracle
- [x] F2. Code Quality Review — unspecified-high
- [x] F3. Real Manual QA — unspecified-high (+ interactive_bash if CLI build flow needs live verification)
- [x] F4. Scope Fidelity Check — deep

## Commit Strategy
- Task 1 produces the authoritative merge commit / resolved merge baseline.
- Tasks 3-8 should remain isolated by ownership zone; do not squash schema/route/vless/transport/core checkpoint work together.
- Tasks 9-18 should commit per surface so regressions are reversible independently.
- Tasks 19-20 are verification/hygiene; only Task 20 should create a final cleanup commit if code changed.

## Success Criteria
- Upstream `v1.13.11` is actually merged into fork history/state, not merely analyzed.
- Fork-only surfaces remain present only where explicitly intended and verified.
- Shared contracts (`go.mod`, `option/`, `route/`, protocol choke points) are stabilized once and then consumed downstream without reopening them.
- Targeted core build/test gates pass and residual blockers are classified honestly.
- User receives a consolidated verification report proving the work is the real upstream merge and not a documentation substitute.
