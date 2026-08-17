---
id: RT-session-native-stop
area: RT
title: Stop another session through the governed native tool
persona: Ada
journey: J-15
expected: compozy__session_stop stops one live same-workspace target through the canonical session stop path, returns the terminal winner once, applies destructive approval policy, denies self or foreign-workspace targets, and leaves repeated or raced callers with deterministic structured outcomes.
entry_points: compozy__session_stop; compozy session stop; POST /api/workspaces/{workspace_id}/sessions/{session_id}/stop over HTTP and UDS; compozy session status <session-id>
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-08-16-herdr-parity.md; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/bootstrap-manifest.json
last_report: docs/qa/reports/2026-08-16-herdr-parity.md
overlaps: RT-session-wait-state; RT-session-prompt-cancel
---

Start two managed sessions in one workspace and one in a neighboring workspace. From Ada's session,
stop the live sibling, race a second stop over another public surface, then compare fresh status and
the terminal event over CLI, HTTP, and UDS. Confirm destructive approval policy, target ownership,
self-action denial, cross-workspace denial, and that prompt cancellation remains the non-terminal
choice when only the current turn should end.

QA impact 2026-08-16: Task 04 added `compozy__session_stop` as one of the seven Herdr parity native
tools. The earlier QA flags covered the other six tools but no scenario owned native stop, so this
content-addressed row closes that journey-derived gap for Task 08.

QA 2026-08-16 Herdr parity: The full runtime E2E exercised the public HTTP, UDS, CLI, and native-tool paths, including matching persisted projections, restart recovery, scoped denials, bounded wait/notify/cancel/stop races, and stable negative outcomes (65/66/69/75/78, agent_scope_denied, and queue-full).
