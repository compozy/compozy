---
title: Dependency-driven auto-enqueue
type: feature
---

Tasks can now opt into dependency-driven auto-enqueue. When a task is marked `auto_enqueue_on_ready`, AGH enqueues its next task run automatically as soon as a blocking dependency completes and the task reconciles to `ready` — so a dependency graph advances without a manual `agh task enqueue` at each step. The behavior is conservative: only a successful completion satisfies a `blocks` edge, paused dependents are skipped, and the queued-run reservation keeps at most one open run per dependent under concurrent or retried completions. Enqueue happens only after the completion has durably committed and is best-effort — a failed enqueue is logged, never rolled back onto the completing run. It is off by default; enable it per task with `--auto-enqueue-on-ready` on `agh task create` / `agh task child create`, toggle it with `agh task update`, or set the `auto_enqueue_on_ready` field over HTTP/UDS and the extension SDK.
