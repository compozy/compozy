# Daemon Improvements Regression Suite

## Purpose

This suite turns the daemon-improvements QA plan into an execution order for `task_09`. It groups the critical transport, runtime, ACP, run-reader, and observability flows into smoke, targeted, and full passes while preserving one artifact root and one final repository gate: `make verify`.

## Execution Rules

1. Run the smoke suite first. If any `P0` smoke item fails, stop and fix before continuing.
2. Run the targeted suite next for the changed daemon slice or bug that is under investigation.
3. Run the full suite before closing `task_09`, including the repository gate and the required E2E follow-up checks where the current harness supports them.
4. After any bug fix, rerun the narrow failing case, then the affected suite, then `make verify`.
5. Browser validation remains blocked/out of scope on this branch unless a real daemon web surface and browser harness are added during execution.

## Priority Bands

- `P0`: daemon control-plane lifecycle, canonical transport parity, runtime shutdown/logging/checkpoint behavior, and operator-visible observability contracts.
- `P1`: client timeout classes, public run-reader compatibility, ACP fault/reconcile behavior, and the external-workspace operator flow.
- `P2`: any future daemon web/browser surface once it exists.

## Smoke Suite

**Goal:** prove the daemon improvements are healthy enough for deeper execution.

**Stop condition:** any `P0` failure blocks the rest of the suite.

| Order | Case ID / Step | Priority | Flow | Lane | Command / Spec |
|---|---|---|---|---|---|
| 1 | Baseline gate | P0 | Repository verification baseline | Baseline | `make verify` |
| 2 | `TC-FUNC-001` | P0 | Daemon lifecycle, stop semantics, and logging policy | E2E | `go test ./internal/cli -run 'TestDaemonStatusAndStopCommandsOperateAgainstRealDaemon' -count=1` plus the supporting daemon logging/stop seam from the case file |
| 3 | `TC-INT-001` | P0 | Canonical HTTP/UDS/SSE parity | Integration | `go test ./internal/api/httpapi -run 'Test(HTTPAndUDSServeMatchingStatusSnapshotAndConflict|HTTPAndUDSServeCanonicalParityAcrossRouteGroups|HTTPAndUDSEmitEquivalentCanonicalSSEStreams|HTTPStreamResumesAfterLastEventIDAndEmitsHeartbeat|HTTPStreamRejectsInvalidAndStaleCursor|MetricsAndTerminalStreamRemainObservable)' -count=1` |
| 4 | `TC-INT-003` | P0 | Runtime shutdown, logging, and checkpoint discipline | Integration | Run the focused daemon/logger/store commands from `TC-INT-003` |
| 5 | `TC-INT-005` | P0 | Health, metrics, snapshot integrity, and transcript replay | Integration | Run the focused daemon/httpapi/contract commands from `TC-INT-005` |

## Targeted Suite

**Goal:** validate the daemon slices most likely to regress after a focused fix or follow-up change.

**Use when:** a branch changes one or more daemon-hardening packages, or a smoke-suite bug was fixed.

| Order | Case ID | Priority | Flow | Lane | Command / Spec |
|---|---|---|---|---|---|
| 1 | `TC-INT-002` | P1 | Timeout classes and public run-reader compatibility | Integration | Run the focused client + `pkg/compozy/runs` commands from `TC-INT-002` |
| 2 | `TC-INT-004` | P1 | ACP liveness, retry, fault, and reconcile behavior | Integration | Run the focused executor + reconcile commands from `TC-INT-004` |
| 3 | `TC-FUNC-002` | P1 | External-workspace operator flow and run inspection | E2E | `go test ./internal/cli -run 'Test(TaskAndReviewCommandsExecuteDryRunAgainstTempNodeWorkspace|ReviewsFixCommandExecuteDryRunRawJSONStreamsCanonicalEvents|RunsAttachCommandUsesRemoteUIAttach|RunsAttachCommandFallsBackToWatchWhenRunIsAlreadySettled|RunsWatchCommandStreamsWithoutLaunchingUI)' -count=1` |

## Full Suite

**Goal:** release-level daemon-improvement validation with public flows, supporting integration seams, and the repository gate.

**Pass condition:** all `P0` pass, at least 90% of `P1` pass, no critical daemon bug remains open, and `make verify` passes.

| Order | Scope | Required Cases / Commands |
|---|---|---|
| 1 | Smoke prerequisite | Run every smoke-suite item in listed order |
| 2 | Targeted regression | Run every targeted-suite item in listed order |
| 3 | Tagged contract decode seam | `go test -tags integration ./internal/api/contract -run 'Test(DaemonHealthRouteDecodesIntoCanonicalContract|RunSnapshotAndStreamDecodeIntoCanonicalContract)' -count=1` |
| 4 | Live-daemon dual-transport follow-up | Use `task_09` to prove one live managed-daemon status/health/snapshot/stream path over both HTTP and UDS, or record the exact blocker if the harness cannot support it yet |
| 5 | Live-daemon ACP follow-up | Use `task_09` to exercise at least one daemon-backed task/review/exec flow that exposes ACP failure or timeout behavior through a public surface, or record the exact blocker |
| 6 | Repository gate | `make verify` |

## Explicit P0 / P1 Mapping

| Case ID | Priority | Notes |
|---|---|---|
| `TC-FUNC-001` | P0 | Public daemon lifecycle control plane must stay healthy before deeper operator work |
| `TC-INT-001` | P0 | Transport divergence invalidates parity claims for every daemon client |
| `TC-INT-003` | P0 | Unbounded shutdown or broken log/checkpoint semantics are release blockers |
| `TC-INT-005` | P0 | Health/metrics/snapshot drift breaks operator observability and cold inspection |
| `TC-INT-002` | P1 | Timeout and public run-reader compatibility are high-risk but can follow core daemon health |
| `TC-INT-004` | P1 | ACP fault behavior is critical, but current automation is not yet true daemon E2E |
| `TC-FUNC-002` | P1 | External workspace proof is required to catch regressions missed by repository-native fixtures |

## Required E2E Follow-up

- `P0`: live-daemon parity for `/api/daemon/status`, `/api/daemon/health`, `/api/runs/:run_id/snapshot`, and `/api/runs/:run_id/stream` over both HTTP and UDS.
- `P0`: live-daemon evidence that health, metrics, and snapshot integrity are operator-visible through public transport output, not only service/handler tests.
- `P1`: daemon-backed ACP failure surfacing through `compozy tasks run`, `compozy reviews fix`, or `compozy exec`.
- `P1`: live-daemon `pkg/compozy/runs` open/list/watch against a realistic workspace fixture when the execution harness can support it.

## Blocked and Manual-Only Notes

- **Browser validation:** blocked/out of scope. This branch has no daemon web UI surface and no browser harness.
- **Manual terminal-only judgment:** optional adjunct only. If `task_09` performs a real-terminal attach/watch check, capture it as supplemental evidence, not as a replacement for automation.
- **Missing integration target:** the repo currently has no `make test-integration` target. Use the case-specific `go test` commands above instead of inventing one.

## Evidence Output

- Record each executed command and result in `.compozy/tasks/daemon-improvs/analysis/qa/verification-report.md`.
- File any discovered issue as `.compozy/tasks/daemon-improvs/analysis/qa/issues/BUG-*.md` and reference the originating case ID.
- Use `.compozy/tasks/daemon-improvs/analysis/qa/screenshots/` only for evidence that materially helps execution or handoff.
