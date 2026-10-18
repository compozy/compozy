# QA Run Report — 2026-10-18 — Dependency completion auto-enqueue

- Scope: CLI completion of a blocking task automatically queues its opted-in dependent.
- Cadence: targeted; Bruno, desktop CLI, local macOS; real isolated daemon and SQLite.
- Build: initial base 07aba5e6ee52b906b8bf78e847728c71cc003b8e, followed by the scoped fix.
- Status: closed.

## Session Matrix

| Scenario | Journey | Tour | Status |
|---|---|---|---|
| TA-027 completion and opted-in dependency | J-finish-task-run | State | Fixed |
| Opted-out dependency and failed blocker canaries | J-finish-task-run | State | Pass |
| Restart and completion replay | J-finish-task-run | State | Pass |

## Charter

Bruno completes the preparation task and expects its dependent delivery task to begin without a second manual start. Create A and opted-in B, add a blocks edge, start and complete A through CLI, inspect B and its runs through CLI and HTTP. Repeat with opt-out and failure; restart only the lab daemon, replay the same completion, and inspect run identity/count. Scheduler is paused in this lab to keep queued work observable; no provider execution is needed. Time box: 20 minutes. Browser UI and provider behavior are outside this backend change.

## Session Debriefs

Bruno walked the literal task-create/dependency/start/run-start/run-complete commands through the CLI, with real SQLite and a real daemon. The base binary left B `ready` with `[]` runs. The corrected binary returned one `queued` run, independently confirmed through `GET /api/tasks/{B}/runs`. After restarting the lab daemon, replaying the completion returned the existing invalid-transition diagnostic; CLI and HTTP still returned the same single dependent run.

The opt-out canary became `ready` with zero runs. The failed-blocker canary remained `blocked` with zero runs. No new paper cuts were observed in these bounded flows. Initial lab setup lacked a default provider; setting the lab's provider through public config resolved that precondition before the successful base and fixed walks. No operator daemon changes were made.

## Fix and regression evidence

- Bug: `BUG-20261018-dependent-run-missing` / GitHub #798.
- Root cause: dependent auto-enqueue lived only in leased completion publication; direct completion reconciled dependents without enqueuing them.
- Fix: reuse one bounded completed-run dependent dispatcher from direct and leased completion, after reconciliation and completion notifications. Preserve the Loop-worker exclusion and canonical enqueue/idempotency guards.
- Canonical SQLite suite: `TestTaskManagerAutoEnqueueOnReadyEnqueuesDependentOnCompletionIntegration`, extended for manual and leased completion, concurrent blockers, opt-out, and completion replay after service recreation. Both manual cases failed before the fix with zero dependent runs; all four pass after it.
- Focused race validation also passed completion/rollup, block breaker, transient expiry, approval, recovery, and terminal-session suites (16.234s); final auto-enqueue rerun passed (4.679s).
- Pinned golangci-lint v2.13.1 on `internal/task` without integration tags: zero issues. With integration tags it additionally exposed eight pre-existing findings; the one introduced formatting finding was corrected. The installed binary was built with an older Go version, so the pinned tool was run with the repository's current toolchain.
- Test-convention heuristic: 16 identical findings on base and changed file; none introduced by this change.

## Limits and cleanup

Scheduler dispatch was paused inside the isolated lab to keep the enqueued run observable. This proves enqueue and persistence, not downstream provider execution. Browser rendering and Linux runtime QA were not run; no UI or platform-specific code changed. Failed/expired-blocker behavior is unchanged; the real failure canary passed, while lease-related coverage remains in the existing focused suites.

The lab strict audit and cleanup receipts are retained in its scratch `qa/` directory. Targeted teardown reported `TEARDOWN_ALL_CLEAN=true`; the receipt has `clean: true`. No lab process remains.

## Final Status

Targeted behavior: PASS. One Blocks-Completion bug fixed and re-walked; no outstanding findings in this slice. Heavy local gates were explicitly prohibited; focused checks passed. PR readiness remains governed by current-head required CI, CodeRabbit, Greptile and React Doctor.


## Review remediation re-walk

Greptile finding `discussion_r4307465873` identified a shared-deadline regression: enqueue hooks had moved ahead of leased completion notifications. The canonical SQLite suite now verifies the completion hook is observed before a dependent enqueue hook. That assertion failed before the ordering fix. The final implementation calls the shared dependent dispatcher from both completion paths after their completion notifications.

| Scenario | Status |
|---|---|
| Reviewed binary: CLI/HTTP enqueue, opt-out/failure, restart/replay | Pass |


The reviewed binary passed the fresh CLI/HTTP re-walk: one queued dependent run, opt-out zero runs, failed blocker still blocked with zero runs, and unchanged single-run identity after restart/replay. The focused auto-enqueue, completion/rollup, terminal-session and hook suites passed with race (13.942s); integration-tag lint reported zero new findings. Both lab passes retain their strict audit and clean teardown receipts in scratch evidence. PR readiness still awaits the final head's CI and reviewer coverage.


## Coordinator completion follow-up

Greptile finding `discussion_r4307497709` identified the separate coordinator completion publisher. Coordinator completion now calls the same dependent dispatcher after both task completion and Loop terminal notifications, retaining the Loop-worker exclusion. The existing coordinator suite was extended with an opted-in dependent and asserts one queued run plus notification-before-enqueue ordering. It failed with zero runs before this fix. Focused coordinator, SQLite auto-enqueue/rollup and hook suites passed with race (11.050s). The earlier public CLI/HTTP evidence remains applicable to the unchanged direct/leased implementation; a live Loop execution was not part of the targeted public walk.
