---
id: RT-session-native-interaction-resolution
area: RT
title: Resolve another session's pending interaction
persona: Théo
journey: J-answer-agent-requests
expected: compozy__session_approve and compozy__session_clarify_answer resolve live and restart-orphaned requests in the caller's workspace, return the original winner on a race, leave a queue-full request pending for retry, and deny self-action or a foreign-workspace target.
entry_points: compozy__session_approve; compozy__session_clarify_answer; compozy session interactions; compozy session status
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-08-16-herdr-parity.md; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/bootstrap-manifest.json
last_report: docs/qa/reports/2026-08-16-herdr-parity.md
overlaps: RT-021; RT-session-clarification-roundtrip
---

Resolve a live permission request and clarification from another managed session, then restart the
daemon with both request kinds pending and resolve them through the native tools. Confirm
`applied`/`answered`, `resolved-after-restart`, `already-resolved`, and `queue-full` receipts;
one-based clarification choice input; durable winner identity; workspace isolation; and the exact
self-approval versus self-target denial reasons.

QA impact 2026-08-16: Task 04 added scoped native approval and clarification-answer tools over the
canonical durable interaction path. Flag only; task_08 owns execution.

QA 2026-08-16 Herdr parity: The full runtime E2E exercised the public HTTP, UDS, CLI, and native-tool paths, including matching persisted projections, restart recovery, scoped denials, bounded wait/notify/cancel/stop races, and stable negative outcomes (65/66/69/75/78, agent_scope_denied, and queue-full).
