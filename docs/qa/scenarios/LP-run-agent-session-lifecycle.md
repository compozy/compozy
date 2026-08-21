---
id: LP-run-agent-session-lifecycle
area: LP
title: Stop completed run-agent child sessions
persona: Bruno
journey: J-complete-partial-loop
expected: A session-started Loop creates each run-agent worker as a run-owned system child of the nearest origin session, stops it after successful terminal settlement, and keeps it active only while a retry can resume the same cell.
entry_points: compozy__loop_run; compozy loop status; compozy session status; session catalog HTTP/UDS/CLI
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-08-20-run-agent-session-lifecycle.md
last_report: docs/qa/reports/2026-08-20-run-agent-session-lifecycle.md
overlaps: LP-run-agent-output-ownership; RT-loop-goal-origin-session-lineage; LP-crash-death-resume
---

Added for GitHub issues #444 and #445. The acceptance walk starts a nested Loop from a real
Batuta session, captures the `run-agent` worker session while it is active, and confirms its
`parent_session_id` and `root_session_id` point to the nearest originating session without borrowing
that session. After schema-valid completion, fresh structured reads must show the worker stopped and
unbound while the Batuta session remains active. A retry probe must keep the exact worker binding
active until the cell reaches a true terminal boundary.
