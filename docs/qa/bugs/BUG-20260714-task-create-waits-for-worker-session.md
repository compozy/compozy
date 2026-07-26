# BUG-20260714-task-create-waits-for-worker-session: Creating a Task waits for its worker session

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-complete-task-tree, create an advanced Task assigned to an agent pool
- **Scenarios:** TA-task-create-async-activation
- **Found:** 2026-07-14 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** Live Cursor/Grok Task creation replay

## Summary

Creating an already-ready Task assigned to the `general` pool left the creation modal and route pending for about 20 seconds. The Task itself was durable immediately, but the HTTP run-enqueue response waited for sandbox synchronization and a live Cursor session to finish provisioning.

## Reproduction

1. Open Tasks and create an Advanced workspace Task.
2. Assign it to the `general` pool and leave it ready for automatic execution.
3. Submit the form while Cursor/Grok 4.5 is the configured runtime.
4. Observe route navigation and correlate the Task/run requests with session provisioning.

**Expected:** Durable Task and run creation return immediately; the Task detail opens while worker activation continues and reports waiting/running truthfully.
**Actual:** `POST /api/tasks/:id/runs` held its 201 response until the worker session was fully provisioned, keeping the Browser on `/tasks/new` for about 20 seconds.

## Evidence

- Pre-fix Task `task-1bfc42bfafb6659a`; run creation response took 19.822 seconds, including 15.529 seconds of sandbox synchronization and 4.108 seconds of Cursor session creation.
- Post-fix Task `task-6087b6cffe877fb7`; run `run-1056af4670c94d26`; real Cursor/Grok session `sess-12b2c865a27ecc72`.
- Post-fix request correlation: Task create returned 201 in 2 ms and run create returned 201 in 3 ms; session provisioning then continued for about 20 seconds before the live Task transitioned waiting → running → completed without reload.

## Fix

- **Root cause:** The post-commit Task enqueue observer invoked task-role session provisioning synchronously, so the public run-enqueue request inherited sandbox synchronization and ACP startup latency. The first detached correction also needed explicit lifecycle ownership: otherwise provisioning could create a session after daemon shutdown had already snapshotted sessions to stop.
- **Correction:** Enqueue returns after the durable commit and transfers activation to a daemon-owned goroutine. Admission is serialized against shutdown, a drain context cancels provisioning, shutdown joins every activation, and session shutdown runs only after task/coordinator/scheduler creators are drained.
- **Fix commit:** pending final whole-diff commit.
- **Regression test:** `internal/daemon/task_role_runtime_test.go` blocks `Session.Create` deterministically, requires `OnTaskRunEnqueued` to return before provisioning completes, and proves daemon shutdown cancels and joins the owned activation.

## Verification

- The focused Task/Loop runtime cases and full `internal/daemon/...` package passed under `-race`.
- `make lint` and `make build` passed before live acceptance.
- The real Browser opened the Task route immediately, then observed the original run bind one Cursor/Grok task-role session and complete exactly once without reload.
- The consolidated shutdown regression blocks session creation, starts daemon shutdown, and proves activation cancellation/join occurs before the session manager takes its stop snapshot. The complete daemon package passed under `-race` in 160.258 seconds.
