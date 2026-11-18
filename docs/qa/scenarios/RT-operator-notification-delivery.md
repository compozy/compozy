---
id: RT-operator-notification-delivery
area: RT
title: Report truthful operator notification delivery outcomes
persona: Ada
journey: J-15
expected: compozy notify, POST /api/agent/notify, and compozy__notify return matching delivered, no-client, rate-limited, and muted-workspace outcomes; delivered requires a live operator stream subscriber, no-client retains the sanitized notice for the sender profile's bell, and muted or rate-limited sends do neither.
entry_points: compozy notify; POST /api/agent/notify over HTTP and UDS; compozy__notify; GET /api/sessions/catalog-stream
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-08-16-herdr-parity.md; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/bootstrap-manifest.json; docs/qa/reports/2026-11-18-attention-pr1820.md
last_report: docs/qa/reports/2026-11-18-attention-pr1820.md
overlaps: RT-session-attention-catalog
---

Send notifications with no catalog client, with one live catalog client, twice inside one second,
and from a muted workspace. Compare human and structured CLI output with HTTP and native-tool
results, confirm redaction and whitespace normalization before delivery, and prove the stream emits
only the delivered notification with its workspace and sender-session ownership intact.

Start a session monitor, close every operator catalog client, and send a notice. Verify the
result is `no-client`, then restart the daemon. Open the attention bell
in the sender's profile and check that the queued title and body are sanitized. Open the sender's
session from the row. Mark the row read and reopen the shell; the notice must stay read. Repeat
with a User-scope session and verify that a different profile cannot read the queued notice.

QA impact 2026-08-16: Task 02 added the notify service and all agent-managed transports. Flag only;
task_08 owns execution after the web notifier lands.

QA 2026-08-16 Herdr parity: The full runtime E2E exercised the public HTTP, UDS, CLI, and native-tool paths, including matching persisted projections, restart recovery, scoped denials, bounded wait/notify/cancel/stop races, and stable negative outcomes (65/66/69/75/78, agent_scope_denied, and queue-full).

QA impact 2026-11-09 (User-scope notifications): send from a User session (no workspace) through
`compozy__notify` and the check-in source/authority approval notices. Each returns `delivered`
with one live catalog client and carries an empty workspace id; the User scope cannot be
workspace-muted, so `muted_workspace` never applies. Project-session mutes are unchanged.
Focused coverage: `TestPublishOperatorNotification` with the real SQLite attention repository.

QA 2026-11-18, PR #1820: targeted real-daemon CLI and Chromium journeys passed. Confirmed offline notice recovery after daemon restart, sanitized content, sender-session navigation and read receipts after refresh. The CLI runtime journey confirmed no-client, delivered and rate limiting. Coverage, fixture-provider use and limits are recorded in docs/qa/reports/2026-11-18-attention-pr1820.md.
