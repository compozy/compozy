---
id: RT-session-wait-state
area: RT
title: Wait for one exact session state
persona: Ada
journey: J-15
expected: compozy session wait returns immediately for an already-satisfied state, reports the first requested transition across --until, distinguishes timeout and gone outcomes by exit code, and --unbounded resumes bounded server registrations without losing an intervening edge.
entry_points: compozy session wait --until/--timeout/--unbounded; POST /api/workspaces/{workspace_id}/sessions/{session_id}/wait over HTTP and UDS; compozy__session_wait
qa_status: pass
bug_ids: BUG-20260911-session-wait-client-timeout
fix_status: fixed
retest_status: pass
fix_commits: 06c5e9e10
evidence: docs/qa/reports/2026-08-16-herdr-parity.md; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/bootstrap-manifest.json;docs/qa/evidence/2026-09-10-qa-execution-unblock/session-wait-retest-proof.json
last_report: docs/qa/reports/2026-09-10-qa-execution-unblock.md
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

QA 2026-09-11: adjacent real Goal control observation found --timeout45s fails at30s due the CLI transport deadline. See BUG-20260911-session-wait-client-timeout and goal-controls-resume-wait.json. This adjacent regression does not add a row to the original357-scenario inventory.

Retest 2026-09-11: fixed in06c5e9e10. Real fresh session returned idle immediately, timed out at45s with exit75 and a resume ID, and observed stopped at35s with exit0. Caller cancellation and server wait ownership pass in the canonical race-enabled client suite. Unchanged state/outcome/resume semantics retain the prior Herdr-parity evidence; this targeted replay does not claim new coverage of every state transition.
