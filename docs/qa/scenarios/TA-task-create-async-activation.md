---
id: TA-task-create-async-activation
area: TA
title: Create a ready Task without waiting for worker provisioning
persona: Bruno
journey: J-complete-task-tree
expected: Creating a ready Task assigned to an agent pool commits and opens its Task detail immediately; sandbox and ACP provisioning continue under daemon ownership while the same run transitions waiting to running to terminal exactly once.
entry_points: Web Create task Advanced mode; POST Task; POST Task run; Task detail live state
qa_status: untested
bug_ids: BUG-20260714-task-create-waits-for-worker-session
fix_status: fixed
retest_status: pass
fix_commits:
evidence: docs/qa/reports/2026-07-13-automation-features.md
last_report: docs/qa/reports/2026-07-13-automation-features.md
overlaps: TA-task-role-session-activation;TA-016
---

The public enqueue boundary must not inherit provider startup latency. Shutdown must still cancel and join an activation that is provisioning after the durable response.

2026-07-14 retest: Task `task-6087b6cffe877fb7` and run `run-1056af4670c94d26` returned from their public create requests in 2 ms and 3 ms. The Browser opened detail immediately, then observed the same run bind real Cursor/Grok session `sess-12b2c865a27ecc72` and converge waiting → running → completed without reload.

2026-07-14 final shutdown proof: the complete daemon race suite passed after ordering task-role drain ahead of the session stop snapshot. The deterministic case observed activation cancellation before `ListAll` and admitted no late session.

2026-07-21: qa_status reset to untested — the opendesign redesigns restructured this scenario's web entry surface (task detail/run detail 3-tab IA, settings takeover shell, or providers page); the pass verdict predates that surface.
