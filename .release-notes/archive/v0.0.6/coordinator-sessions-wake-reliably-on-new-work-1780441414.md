---
title: Coordinator sessions wake reliably on new work
type: fix
---

Coordinator sessions now wake reliably when new work arrives. When a task run is enqueued for a workspace that already has a running coordinator session, AGH delivers a synthetic wake to that session — interrupting the agent's current turn if it is idle and waiting — so queued runs are picked up promptly instead of stalling. The same interrupt-if-waiting delivery now applies to harness re-entry and heartbeat wakes, force-retried and force-recovered runs re-trigger a coordinator wake, and heartbeat wakes skipped for non-transient reasons are recorded as dropped rather than silently lost. Wake delivery is de-duplicated per session and run and drained safely across daemon shutdown.
