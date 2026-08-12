---
id: TA-task-inspect-single-live-stream
area: TA
title: Keep task detail and Inspect on one live stream
persona: Bruno
journey: J-24
expected: Opening Inspect on a live task reuses the task detail SSE owner, so one server event updates detail and Inspect once; closing and reopening Inspect creates no second connection, duplicate row, or duplicate refresh, and the restored window catches up without reload.
entry_points: Web Tasks catalog; task detail; Inspect drawer; browser Network panel
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-11-frontend-performance/task-inspect-stream-before-event.png;docs/qa/evidence/2026-08-11-frontend-performance/task-inspect-stream-after-event.png;docs/qa/evidence/2026-08-11-frontend-performance/task-inspect.har
last_report: docs/qa/reports/2026-08-11-frontend-performance.md
overlaps: TA-003; TA-016; TA-019
---

Created for the 2026-08-11 frontend performance remediation. The stream connection count and the user-visible event count must agree throughout the Inspect lifecycle.

QA 2026-08-11: a fresh task detail reload owned one task EventSource. Opening, closing, and reopening Inspect added no connection. Pausing the task through the public UI produced one cursor reconnect and one visible state update shared by detail and Inspect.
