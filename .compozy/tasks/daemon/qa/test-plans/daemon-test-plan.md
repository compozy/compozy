# Daemon QA Test Plan

## Executive Summary

This plan defines the reusable QA scope for the daemon migration handoff. It gives `task_19` a fixed artifact root, traceable case IDs, and an execution order for the daemon's operator-visible surfaces: singleton bootstrap, workspace registry, task/review/exec runs, sync/archive behavior, attach/watch, transport parity, public run readers, and the performance-sensitive seams introduced by `task_16`.

The planning posture is intentionally evidence-driven. The repository already has strong Go-based CLI/API/integration coverage for the daemon control plane, so those suites are the primary automation source of truth. This branch does **not** expose a daemon web UI and does **not** contain a browser E2E harness (`playwright`, `cypress`, `webdriver`, `puppeteer`, `selenium`, or `chromedp` configs/specs were not found), so browser validation is explicitly **Blocked / Out of Scope** for `task_19` unless the execution branch adds a real web surface.

## Objectives

- Prove the home-scoped singleton daemon starts, recovers stale artifacts, and reconciles interrupted runs safely.
- Prove the operator-facing CLI control plane is stable for workspace registry, task runs, sync/archive, reviews, exec, attach, and watch.
- Prove UDS and localhost HTTP remain contract-equivalent for status, health, conflicts, snapshot access, and SSE resume.
- Prove `pkg/compozy/runs` remains daemon-backed and compatible for list/open/tail/watch consumers.
- Give `task_19` a fixed artifact layout under `.compozy/tasks/daemon/qa/` with stable case IDs, priorities, and automation annotations.

## Scope

### In Scope

- Daemon bootstrap, stale-artifact cleanup, same-home singleton reuse, and restart reconciliation.
- Workspace register, resolve, list, show, and unregister conflict behavior.
- `compozy tasks run` bootstrap, attach-mode resolution, watcher-driven sync, and duplicate run protection.
- `compozy sync` and `compozy archive` behavior from daemon-backed state, including subdirectory invocation and archive gating.
- `compozy reviews ...` and `compozy exec` daemon-backed lifecycle, persisted state, and authored workspace artifacts.
- `compozy runs attach` and `compozy runs watch` snapshot bootstrap, reconnect, heartbeat, overflow, and terminal EOF handling.
- UDS/HTTP parity, SSE cursor behavior, and daemon error/conflict envelopes.
- `pkg/compozy/runs` list/open/replay/tail/watch compatibility over daemon transport.
- Performance regression guards for event pagination, run listing, and CLI cold-start-sensitive seams introduced by `task_16`.

### Out of Scope

- Reintroducing legacy `compozy start`, `_tasks.md`, or `_meta.md` assumptions into the daemon QA surface.
- Executing browser/web validation on this branch.
- Creating a new browser or terminal E2E framework as part of QA planning.
- Fixing bugs pre-emptively in this planning task. Any discrepancy discovered during planning is documented for `task_19`.

## Test Strategy and Approach

### QA Lanes

- `E2E`: public CLI flows exercised through repository-owned integration tests that launch the real daemon and operator commands.
- `Integration`: service, transport, daemon-manager, and public package tests that validate cross-boundary behavior without inventing new operator harnesses.
- `Manual-only`: operator checks that still require human judgment in a real terminal session.
- `Blocked`: flows intentionally not executable on this branch because the surface or harness does not exist.

### Coverage Matrix

| Flow | Primary Case IDs | Source of Truth | Lane | Automation Status | Existing Harness |
|---|---|---|---|---|---|
| Daemon bootstrap and recovery | `TC-INT-001` | TechSpec Testing Approach, ADR-001, task_14/task_17 | Integration | Existing | `internal/daemon/boot_integration_test.go`, `internal/daemon/reconcile_test.go` |
| Workspace registry CLI | `TC-FUNC-001` | TechSpec Workspaces rules, ADR-001/004, task_14 | E2E | Existing | `internal/cli/operator_commands_integration_test.go`, `internal/cli/daemon_commands_test.go` |
| Task runs, attach mode, and watcher sync | `TC-FUNC-002` | TechSpec Task workflows + attach semantics, ADR-004, task_12/task_14/task_17 | E2E | Existing | `internal/cli/root_command_execution_test.go`, `internal/daemon/run_manager_test.go` |
| Sync and archive | `TC-FUNC-003` | TechSpec Sync rules, ADR-002/004, task_14/task_17 | E2E | Existing | `internal/cli/operator_commands_integration_test.go`, `internal/cli/archive_command_integration_test.go`, `internal/core/{sync,archive}_test.go` |
| Review runs | `TC-FUNC-004` | TechSpec Reviews, ADR-002/003, task_15 | E2E | Existing | `internal/cli/reviews_exec_daemon_additional_test.go`, `internal/daemon/review_exec_transport_service_test.go` |
| Exec runs | `TC-FUNC-005` | TechSpec Sync and exec, ADR-002/004, task_15 | E2E | Existing | `internal/cli/reviews_exec_daemon_additional_test.go`, `internal/daemon/run_manager_test.go`, `internal/api/client/reviews_exec_test.go` |
| Attach/watch operator flows | `TC-FUNC-006`, `TC-UI-001` | TechSpec CLI/TUI clients + SSE, ADR-004, task_12 | E2E + Manual-only | Existing + N/A | `internal/cli/root_command_execution_test.go`, `internal/core/run/ui/remote_test.go`, `pkg/compozy/runs/remote_watch_test.go` |
| UDS/HTTP parity and SSE resume | `TC-INT-002` | TechSpec Transport Contract, ADR-003 | Integration | Existing | `internal/api/httpapi/transport_integration_test.go`, `internal/api/core/*handlers*_test.go` |
| `pkg/compozy/runs` compatibility | `TC-INT-003` | TechSpec Public run readers, ADR-002/003, task_13 | Integration | Existing | `pkg/compozy/runs/{integration,transport,watch,tail,run}_test.go` |
| Performance-sensitive regression guards | `TC-PERF-001` | task_16 + perf ledger + TechSpec Known Risks | Integration | Existing | daemon/rundb benchmarks and focused CLI timing adjuncts |
| Browser/web validation | n/a | TechSpec scope boundary, task_18 requirement 6 | Blocked / Out of Scope | Blocked | No daemon web UI surface and no browser harness in this branch |

### Automation Rules for `task_19`

- Treat the existing Go-based daemon suites as the canonical automation lane. Do not replace them with ad hoc shell scripts.
- When a case lists both CLI/API and supporting daemon/service specs, run the public-interface command first, then the supporting seam if root-cause evidence is needed.
- If `task_19` finds a daemon bug in a public flow, add or update the narrowest durable automated regression in the matching package instead of broadening unrelated suites.
- Keep browser validation blocked unless the execution branch adds both a real daemon web UI and a runnable browser harness.

## Environment Requirements

| Area | Requirement |
|---|---|
| OS | macOS or Linux with support for Unix domain sockets and temporary home directories |
| Go toolchain | Repository-compatible Go version from `go.mod` and `make verify` |
| Terminal | Real TTY required for `TC-UI-001`; non-interactive shells are sufficient for the automated suite |
| Runtime root | Isolated `$HOME` per execution slice when tests need daemon singleton or persisted run isolation |
| Fixture workspace | Minimal daemon workflow workspace rooted under `.compozy/tasks/daemon` or temporary test fixtures created by the existing Go suites |
| Transport | UDS is primary; localhost HTTP must bind to `127.0.0.1` only |
| Browser | Not required for this branch; browser lane is blocked/out of scope |

## Entry Criteria

- Tasks `12` through `17` remain completed and no new daemon contract changes are pending outside `task_18`.
- The QA artifact root exists at `.compozy/tasks/daemon/qa/` with `test-plans/`, `test-cases/`, `issues/`, and `screenshots/`.
- The execution branch still has no daemon web UI or browser harness unless `task_19` explicitly records a new one.
- The repository verification contract remains `make verify`.
- Task `19` reads this plan, the regression suite, the test cases, `_techspec.md`, and ADR-001 through ADR-004 before execution.

## Exit Criteria

- All `P0` daemon cases pass.
- At least 90% of `P1` daemon cases pass and any failure has a documented root-cause fix plan or bug artifact.
- `make verify` passes after the last daemon fix.
- `.compozy/tasks/daemon/qa/verification-report.md` records the commands, outcomes, blockers, and any new regression coverage.
- Any discovered issue is documented under `.compozy/tasks/daemon/qa/issues/BUG-*.md` with the originating case ID.

## Risk Assessment

| Risk | Probability | Impact | Mitigation | Primary Cases |
|---|---|---|---|---|
| Singleton bootstrap leaves stale socket/info artifacts and blocks startup | Medium | High | Prioritize daemon bootstrap/recovery checks before all other flows | `TC-INT-001` |
| Workspace registry resolves the wrong workspace or allows unregister during active runs | Medium | High | Run real-daemon workspace CLI coverage early in smoke | `TC-FUNC-001` |
| Task runs or review/exec flows drift from daemon lifecycle ownership | Medium | High | Keep public CLI flow tests in smoke/targeted suites and verify persisted-state semantics | `TC-FUNC-002`, `TC-FUNC-004`, `TC-FUNC-005` |
| Sync/archive regresses to metadata-file heuristics or archives incomplete workflows | Medium | High | Pair public CLI checks with core sync/archive integration tests | `TC-FUNC-003` |
| Attach/watch breaks snapshot bootstrap, reconnect, or terminal EOF handling | Medium | High | Run attach/watch lane before any manual TUI judgment call | `TC-FUNC-006`, `TC-UI-001` |
| UDS and HTTP diverge on status/error/SSE behavior | Medium | High | Keep transport parity as a P0 integration lane | `TC-INT-002` |
| Public `pkg/compozy/runs` callers break because workspace-local assumptions return | High | High | Preserve the dedicated daemon-backed run-reader suite in every full pass | `TC-INT-003` |
| Performance work from `task_16` regresses silently | Medium | Medium | Run benchmark-based guardrails and compare with the task-16 baseline before release | `TC-PERF-001` |

## Artifact Ownership and Handoff

| Path | Owner in Task 18 | Consumer in Task 19 | Notes |
|---|---|---|---|
| `.compozy/tasks/daemon/qa/test-plans/daemon-test-plan.md` | Create and keep current | Read before execution | This document is the feature-level QA contract |
| `.compozy/tasks/daemon/qa/test-plans/daemon-regression.md` | Create and keep current | Execute in listed order | Defines smoke, targeted, and full suites |
| `.compozy/tasks/daemon/qa/test-cases/TC-*.md` | Create and keep current | Use as execution matrix seed | Case IDs are stable; do not rename during `task_19` |
| `.compozy/tasks/daemon/qa/issues/BUG-*.md` | Create only if planning finds a concrete discrepancy | Create/update during execution | Must reference the originating case ID |
| `.compozy/tasks/daemon/qa/screenshots/` | Create directory only | Populate only if manual/TUI evidence is captured | Browser lane remains blocked |
| `.compozy/tasks/daemon/qa/verification-report.md` | Do not create in task 18 | Required task-19 output | Must cite fresh commands and results |

## Timeline and Deliverables

1. Planning complete in `task_18`: plan, cases, and regression suite exist under `.compozy/tasks/daemon/qa/`.
2. Execution begins in `task_19`: run smoke first, then targeted/full coverage based on this artifact set.
3. Bug handling during `task_19`: reproduce, fix at the root, add/update the narrowest durable regression, rerun the impacted lane, then rerun `make verify`.
4. Final handoff: `verification-report.md` plus any `BUG-*` artifacts become the durable daemon QA evidence set.
