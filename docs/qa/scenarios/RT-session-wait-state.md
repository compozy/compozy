---
id: RT-session-wait-state
area: RT
title: Wait for one exact session state
persona: Ada
journey: J-15
expected: compozy session wait returns immediately for an already-satisfied state, reports the first requested transition across --until, distinguishes timeout and gone outcomes by exit code, and --unbounded resumes bounded server registrations without losing an intervening edge.
entry_points: compozy session wait --until/--timeout/--unbounded; POST /api/workspaces/{workspace_id}/sessions/{session_id}/wait over HTTP and UDS; compozy__session_wait
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-session-attention-catalog
---

Drive one session through running, waiting-for-input, idle/done, and stopped while separate clients
exercise explicit `--until`, the settled default, timeout, and `--unbounded`. Confirm CLI, HTTP, UDS,
and native-tool results agree; `done` satisfies `idle`; a deleted or replaced session cannot satisfy
the original wait; resume-grace expiry, overflow, client cancellation, and concurrent-wait caps end
with deterministic outcomes and no orphaned registration.

QA impact 2026-08-16: Task 04 generalized session wait across CLI, HTTP, UDS, and native tools. Flag
only; task_08 owns execution.
