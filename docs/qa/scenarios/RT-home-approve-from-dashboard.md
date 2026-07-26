---
id: RT-home-approve-from-dashboard
area: RT
title: Approve and reject task from the home attention zone
persona: End user
journey:
expected: An approval-gated task appears in Needs you with Approve (primary) + Reject + Open; Approve resolves the row into a success-tint "Approved — <title> is starting" state, decrements the count chip and the Needs-you KPI after refetch, and the task run starts; Reject resolves the row as rejected; failed-run rows offer Retry only when a retryable run id exists.
entry_points: web `/` Needs you zone; `POST /api/tasks/:id/approve`; `POST /api/tasks/:id/reject`; `POST /api/runs/:id/retry`
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: web/src/systems/dashboard/components/home-attention-zone.tsx; internal/observe/overview_attention.go
last_report:
overlaps:
---

story: As an end user I clear approvals and failures directly from Home without opening the task inbox.

New behavior shipped 2026-07-23. The attention list is composed server-side (inbox approval/failed lanes + needs_attention tasks) so the CLI (`agh observe overview -o json`) sees the same items and actions the UI renders.
