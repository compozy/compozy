# Daemon Improvements QA Test Plan

## Executive Summary

This plan defines the reusable QA scope for the daemon-improvement work completed in `task_03` through `task_07`. It gives `task_09` a fixed artifact root, stable case IDs, concrete command/spec references, and an execution order for the daemon-hardening surfaces that changed in this feature: canonical HTTP/UDS contracts, daemon client timeout behavior, public run-reader compatibility, bounded shutdown and logging, ACP supervision, and observability contracts for health, metrics, snapshots, and transcript replay.

The planning posture is intentionally evidence-driven and tied to the current repository state, not the idealized harness layout from the TechSpec. In this worktree:

- `make verify` is the only repository-wide verification gate in `Makefile`.
- there is no `make test-integration` target yet.
- most daemon integration-style tests still run in the default `go test` lane without `//go:build integration` tags.
- the branch has no `web/` directory and no browser automation harness, so browser validation is explicitly **Blocked / Out of Scope** unless a real daemon web surface is added later.

The feature source of truth remains under `.compozy/tasks/daemon-improvs/` for `_techspec.md`, ADRs, and `task_03.md` through `task_09.md`. Only the QA artifacts for this handoff are rooted under `.compozy/tasks/daemon-improvs/analysis/qa/` because that is the caller-required `qa-output-path`.

## Objectives

- Prove the canonical daemon transport contract remains parity-safe across HTTP, UDS, and SSE framing for status, health, snapshots, conflicts, heartbeats, overflow, and resume behavior.
- Prove daemon-facing clients use the TechSpec timeout classes and that `pkg/compozy/runs` preserves open/list/tail/watch/replay compatibility against the canonical contract.
- Prove runtime shutdown, logging, and SQLite close discipline remain deterministic, bounded, and observable.
- Prove ACP-backed runtime behavior is still protected by reusable automation for retries, pending jobs, structured failures, and reconcile honesty, while clearly identifying where true daemon-backed E2E proof is still missing.
- Prove richer observability contracts for health, metrics, sticky snapshot integrity, and transcript replay are traceable to concrete automation.
- Seed `task_09` with one external-workspace operator case so execution must validate at least one daemon-backed flow outside the repository’s own Go fixtures.

## Scope

### In Scope

- Canonical transport contract behavior owned by `internal/api/contract`, including request IDs, error envelopes, route parity, and SSE `event` / `heartbeat` / `overflow` framing.
- HTTP/UDS parity for daemon status, health, snapshot, metrics-adjacent observability, and run-stream resume behavior.
- Daemon client timeout classes (`probe`, `read`, `mutate`, `long_mutate`, `stream`) plus reconnect-on-heartbeat-gap behavior.
- Public run-reader compatibility for list/open/tail/watch/replay and sticky `Incomplete` semantics.
- Runtime shutdown ownership, stop-vs-force semantics, detached vs foreground logging behavior, and checkpoint-on-close discipline for `global.db` and `run.db`.
- ACP runtime supervision, retry, structured-failure, pending-job, and reconcile-adjacent fault scenarios.
- Observability surfaces: `/api/daemon/health`, `/api/daemon/metrics`, snapshot integrity reasons, transcript assembly, and contract decoding.
- One external workspace operator flow using the existing temp Node.js fixture and run-inspection commands.

### Out of Scope

- Executing the daemon flows or fixing bugs in `task_08`. This task defines the coverage and evidence layout only.
- Creating a new runtime harness package, a new `make test-integration` target, or any new browser framework as part of planning.
- Inventing a daemon web UI lane on a branch that has no web surface.
- Rewriting or moving the authoritative TechSpec/ADR/task files to match the `analysis/qa` output root.

## Test Strategy and Approach

### QA Lanes

- `E2E`: public CLI/operator flows exercised through real daemon-backed command tests or a realistic temporary workspace fixture.
- `Integration`: cross-boundary Go tests that validate handlers, daemon-manager seams, clients, run readers, store behavior, or executor fault handling without a full operator walkthrough.
- `Manual-only`: human judgment checks that still require a live terminal or UX reading and do not have a durable automation seam today.
- `Blocked`: flows that cannot be executed on this branch because the surface or harness does not exist.

### Coverage Matrix

| Flow | Primary Case IDs | Source of Truth | Lane | Automation Status | Existing Harness | E2E Follow-up |
|---|---|---|---|---|---|---|
| Daemon control plane lifecycle, stop semantics, and log policy | `TC-FUNC-001` | TechSpec `Monitoring and Observability`, `Known Risks`; ADR-002/003/004; `task_05.md` | E2E | Existing | `internal/cli/operator_commands_integration_test.go`, `internal/daemon/boot_integration_test.go` | None beyond normal regression reruns |
| Canonical transport parity across HTTP/UDS/SSE | `TC-INT-001` | TechSpec `API Endpoints`, `Testing Approach`; ADR-001/003; `task_03.md` | Integration | Existing | `internal/api/httpapi/transport_integration_test.go`, `internal/api/contract/contract_integration_test.go` | `P0`: task `09` should add live-daemon HTTP+UDS proof for status/health/snapshot/stream because current coverage is server/handler integration only |
| Daemon client timeout classes and public run-reader compatibility | `TC-INT-002` | TechSpec `Timeout Policy`, `Snapshot Integrity Semantics`; ADR-001/004; `task_04.md` | Integration | Existing | `internal/api/client/client_contract_test.go`, `pkg/compozy/runs/{integration,remote_watch,run,tail,watch}_test.go` | `P1`: current run-reader coverage is fixture-server based; task `09` should exercise at least one live-daemon open/list/watch path when feasible |
| Runtime shutdown, logging, and checkpoint discipline | `TC-INT-003` | TechSpec `Technical Dependencies`, `Monitoring and Observability`; ADR-002/004; `task_05.md` | Integration | Existing | `internal/daemon/{runtime,shutdown,boot_integration}_test.go`, `internal/logger/logger_test.go`, `internal/store/{sqlite,globaldb,rundb}/*_test.go` | Covered publicly by `TC-FUNC-001`; no separate E2E gap |
| ACP liveness, retries, fault handling, and reconcile honesty | `TC-INT-004` | TechSpec `Integration Points`, `Testing Approach`, `Known Risks`; ADR-002/003; `task_06.md` | Integration | Existing but not daemon-E2E | `internal/core/run/executor/{execution_acp_integration,execution_acp}_test.go`, `internal/daemon/reconcile_test.go` | `P1`: task `09` should seek daemon-backed proof that ACP failures surface correctly through public task/review/exec flows when the harness supports it |
| Observability contracts, snapshot integrity, and transcript replay | `TC-INT-005` | TechSpec `Monitoring and Observability`, `Snapshot Integrity Semantics`; ADR-004; `task_07.md` | Integration | Existing | `internal/daemon/{service,run_integrity,run_manager}_test.go`, `internal/api/httpapi/transport_integration_test.go`, `internal/api/contract/contract_integration_test.go` | `P0`: task `09` should capture live-daemon health/metrics/snapshot evidence because most coverage is service/transport integration today |
| Temporary external-workspace operator flow and run inspection | `TC-FUNC-002` | `task_09.md` requirement 4, TechSpec `Testing Approach`; ADR-003/004 | E2E | Existing | `internal/cli/root_command_execution_test.go` temp Node fixture and run-inspection tests | Re-run on execution branch with fresh evidence; extend only if a missing public seam is found |
| Browser/web validation | none | `task_08.md` requirement 6, `task_09.md` requirement 8 | Blocked / Out of Scope | Blocked | No `web/` directory, no browser framework config, no daemon web UI | Remains blocked unless the execution branch adds a real web surface and harness |

## Environment Matrix

| Area | Requirement | Evidence / Notes |
|---|---|---|
| OS | macOS or Linux preferred; Unix domain sockets required for UDS parity | Current daemon/operator integration tests rely on loopback HTTP and UDS behavior |
| Go toolchain | Repository-compatible Go version from `go.mod` | Required for `make verify` and focused `go test` runs |
| Verification gate | `make verify` | Repository-wide baseline gate in `Makefile` |
| Focused automation | `go test` with targeted `-run` filters, plus `-tags integration` only where files actually require it | Needed because no `make test-integration` target exists yet |
| Transport | UDS primary, localhost HTTP bound to `127.0.0.1` only | Matches TechSpec and `internal/api/httpapi` validation |
| Runtime isolation | Fresh temp home/runtime dirs for daemon singleton cases | Already used by existing daemon/operator suites |
| External fixture | Temp Node.js workspace used by `TestTaskAndReviewCommandsExecuteDryRunAgainstTempNodeWorkspace` | Execution must keep at least one realistic non-repo workspace path |
| Terminal | Real TTY only when a manual-only terminal judgement call is added later | No browser or UI-web setup required for this branch |
| Browser | Not required | No daemon web surface exists in the current worktree |

## Automation Strategy

1. Use `make verify` as the broad repository gate before and after execution work.
2. Use the existing focused `go test` seams as the planning source of truth rather than inventing a missing `make test-integration` abstraction.
3. Treat `internal/api/httpapi`, `internal/daemon`, `internal/core/run/executor`, `internal/api/client`, and `pkg/compozy/runs` as the main regression buckets for this feature.
4. Use the temp Node.js workspace CLI test as the required realistic operator-flow proof for `task_09`.
5. Keep browser validation blocked unless the execution branch adds both:
   - a real daemon web surface
   - a runnable browser harness

## Explicit P0 / P1 Public Flows Requiring E2E Follow-up

- `P0`: live-daemon dual-transport proof for daemon status, health, snapshot, and run-stream resume over both HTTP and UDS. Current parity coverage is integration-only.
- `P0`: live-daemon proof that health, metrics, and snapshot integrity are observable through operator-facing transport responses rather than only service/transport unit seams.
- `P1`: daemon-backed ACP fault surfacing through public task/review/exec flows. Current ACP helper coverage proves executor behavior, but not a managed-daemon end-to-end path.
- `P1`: public `pkg/compozy/runs` open/list/watch against a live daemon and realistic workspace fixture. Current compatibility coverage uses in-process/test-server fixtures.

## Entry Criteria

- Tasks `03` through `07` are treated as implemented source material for this QA handoff.
- The artifact root exists at `.compozy/tasks/daemon-improvs/analysis/qa/` with `test-plans/`, `test-cases/`, `issues/`, and `screenshots/`.
- `task_09` reads:
  - `.compozy/tasks/daemon-improvs/_techspec.md`
  - `.compozy/tasks/daemon-improvs/adrs/adr-001.md` through `adr-004.md`
  - this plan
  - the regression suite
  - all `TC-*.md` case files
- The execution branch still has no daemon web surface unless `task_09` documents a new one explicitly.

## Exit Criteria

- All `P0` cases pass.
- At least 90% of `P1` cases pass, and any failure has a documented bug artifact or clear execution blocker.
- `make verify` passes after the last execution-side fix.
- `.compozy/tasks/daemon-improvs/analysis/qa/verification-report.md` records commands, outcomes, blockers, and any new regression additions.
- Any issue discovered during execution is captured under `.compozy/tasks/daemon-improvs/analysis/qa/issues/BUG-*.md` with its originating case ID.

## Risk Assessment

| Risk | Probability | Impact | Mitigation | Primary Cases |
|---|---|---|---|---|
| HTTP and UDS drift on envelope fields, request IDs, or stream framing | Medium | High | Keep transport parity in smoke and require live-daemon parity follow-up in `task_09` | `TC-INT-001` |
| Timeout classes regress to blanket behavior or stream reconnect stops honoring heartbeat gaps | Medium | High | Keep client and run-reader compatibility as a named regression bucket rather than absorbing it into generic CLI checks | `TC-INT-002` |
| Daemon shutdown/logging fixes regress close semantics or log visibility | Medium | High | Pair operator daemon lifecycle checks with logger/store close-path tests | `TC-FUNC-001`, `TC-INT-003` |
| ACP helper coverage gives false confidence because failures are not exercised through a managed daemon | Medium | High | Keep ACP faults explicit and call out the daemon-backed E2E follow-up as required, not optional | `TC-INT-004` |
| Health/metrics/snapshot changes pass service tests but drift in operator-visible transport output | Medium | High | Require both observability integration checks and live-daemon follow-up | `TC-INT-005` |
| Repository-native fixtures miss a real workspace regression | Medium | High | Keep one temp Node.js workspace operator-flow case in the plan and regression suite | `TC-FUNC-002` |
| Browser validation gets invented despite no real surface | Low | Medium | Mark browser validation blocked/out of scope in every planning artifact | plan + regression suite |

## Artifact Ownership and Handoff

| Path | Owner in Task 08 | Consumer in Task 09 | Notes |
|---|---|---|---|
| `.compozy/tasks/daemon-improvs/analysis/qa/test-plans/daemon-improvs-analysis-test-plan.md` | Create and keep current | Read before execution | Feature-level QA contract |
| `.compozy/tasks/daemon-improvs/analysis/qa/test-plans/daemon-improvs-analysis-regression.md` | Create and keep current | Execute in listed order | Smoke, targeted, and full suite order |
| `.compozy/tasks/daemon-improvs/analysis/qa/test-cases/TC-*.md` | Create and keep current | Use as execution matrix seed | Do not rename IDs during execution |
| `.compozy/tasks/daemon-improvs/analysis/qa/issues/BUG-*.md` | Create only if planning discovers a concrete discrepancy | Create/update during execution | Must reference originating case ID |
| `.compozy/tasks/daemon-improvs/analysis/qa/screenshots/` | Create directory only | Populate only if execution captures useful evidence | Browser lane stays blocked unless the branch changes |
| `.compozy/tasks/daemon-improvs/analysis/qa/verification-report.md` | Do not create in task 08 | Required task-09 output | Must cite fresh commands and results |

## Timeline and Deliverables

1. Planning complete in `task_08`: feature test plan, regression suite, and test cases exist under `.compozy/tasks/daemon-improvs/analysis/qa/`.
2. Execution begins in `task_09`: run smoke first, then targeted/full coverage, and capture evidence under the same root.
3. Bug handling in `task_09`: reproduce, fix at the root, add/update the narrowest durable regression, rerun the impacted lane, then rerun `make verify`.
4. Final handoff: `verification-report.md` plus any `BUG-*` artifacts become the durable daemon-improvements QA evidence set.
