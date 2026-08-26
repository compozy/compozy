---
id: RT-session-wait-state
area: RT
title: Wait for one exact session state
persona: Ada
journey: J-15
expected: compozy session wait returns immediately for an already-satisfied state, reports the first requested transition across --until, distinguishes timeout and gone outcomes by exit code, and --unbounded resumes bounded server registrations without losing an intervening edge.
entry_points: compozy session wait --until/--timeout/--unbounded; POST /api/workspaces/{workspace_id}/sessions/{session_id}/wait over HTTP and UDS; compozy__session_wait
qa_status: fail
bug_ids: BUG-20260826-bounded-wait-client-timeout
fix_status: fixed
retest_status: pending
fix_commits: pending QA remediation commit
evidence: /Users/pedronauck/dev/qa-labs/compozy-agent-comms-20260826-20260826-065104-728050-lab/qa-artifacts/qa/bounded-wait-client-timeout-reproduction.md
last_report: docs/qa/reports/2026-08-26-agent-comms.md
overlaps: RT-session-attention-catalog
---

Drive one session through running, waiting-for-input, idle/done, and stopped while separate clients
exercise explicit `--until`, the settled default, timeout, and `--unbounded`. Confirm CLI, HTTP, UDS,
and native-tool results agree; `done` satisfies `idle`; a deleted or replaced session cannot satisfy
the original wait; resume-grace expiry, overflow, client cancellation, and concurrent-wait caps end
with deterministic outcomes and no orphaned registration.

QA impact 2026-08-16: Task 04 generalized session wait across CLI, HTTP, UDS, and native tools. Flag
only; task_08 owns execution.

QA 2026-08-16 Herdr parity: The full runtime E2E exercised the public HTTP, UDS, CLI, and native-tool paths, including matching persisted projections, restart recovery, scoped denials, bounded wait/notify/cancel/stop races, and stable negative outcomes (65/66/69/75/78, agent_scope_denied, and queue-full).

QA regression 2026-08-26: local UDS `session wait --timeout 55s` was interrupted by the CLI's
fixed 30-second HTTP timeout. See `BUG-20260826-bounded-wait-client-timeout`; focused fix is green and
the public retest is pending a rebuilt binary.
