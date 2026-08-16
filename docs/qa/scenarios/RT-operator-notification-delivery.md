---
id: RT-operator-notification-delivery
area: RT
title: Report truthful operator notification delivery outcomes
persona: Ada
journey: J-15
expected: compozy notify, POST /api/agent/notify, and compozy__notify return matching delivered, no-client, rate-limited, and muted-workspace outcomes; delivered is returned only when at least one live operator catalog subscriber receives the sanitized operator_notification event, and muted or rate-limited sends publish nothing.
entry_points: compozy notify; POST /api/agent/notify; compozy__notify; GET /api/sessions/catalog-stream
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-session-attention-catalog
---

Send notifications with no catalog client, with one live catalog client, twice inside one second,
and from a muted workspace. Compare human and structured CLI output with HTTP and native-tool
results, confirm redaction and whitespace normalization before delivery, and prove the stream emits
only the delivered notification with its workspace and sender-session ownership intact.

QA impact 2026-08-16: Task 02 added the notify service and all agent-managed transports. Flag only;
task_08 owns execution after the web notifier lands.
