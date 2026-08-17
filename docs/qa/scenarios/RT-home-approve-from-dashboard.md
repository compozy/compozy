---
id: RT-home-approve-from-dashboard
area: RT
title: Approve and reject task from the home attention zone
persona: Cora
journey: J-operate-home-dashboard
expected: An approval-gated task appears in both the Home Needs you zone and the OS attention bell; Approve resolves the Home row into a success-tint "Approved — <title> is starting" state, removes the bell row, decrements the task-backed count after refetch, and starts one task run; Reject resolves the row as rejected; failed-run rows offer Retry only when a retryable run id exists.
entry_points: web `/` Needs you zone; web OS shell attention bell task-approval row; `POST /api/tasks/:id/approve`; `POST /api/tasks/:id/reject`; `POST /api/runs/:id/retry`
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: web/src/systems/dashboard/components/home-attention-zone.tsx; internal/observe/overview_attention.go;/Users/pedronauck/dev/qa-labs/compozy-qa-rt-current-source-20260730-20260730-061631-252740-lab/qa-artifacts/qa; docs/qa/reports/2026-08-16-herdr-parity.md; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/screenshots/herdr-cross-workspace-needs-you-fixed.png; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/screenshots/herdr-attention-all-quiet-cleared.png; .compozy/tasks/herdr-parity/evidence/visual/task_03
last_report: docs/qa/reports/2026-08-16-herdr-parity.md
overlaps:
---

story: As an end user I clear approvals and failures directly from Home without opening the task inbox.

New behavior shipped 2026-07-23. The attention list is composed server-side (inbox approval/failed lanes + needs_attention tasks) so the CLI (`compozy observe overview -o json`) sees the same items and actions the UI renders.

QA impact 2026-08-16: selected as the Herdr parity adjacent canary because the rewritten OS bell
must preserve its pre-existing task-approval rows and task-backed count while adding session
attention sections. Reset to `untested`; Task 08 owns the paired bell/Home walk.

QA 2026-08-16 Herdr parity: The isolated browser journey, focused attention Playwright lane, and full Web E2E exercised cross-workspace landing, permission resolution, counts, channel suppression, task canary, catalog scope/order, finished presence clearing, and honest quiet/stale states. The lab browser exposed its real notification capability; deterministic granted and denied branches ran in the canonical browser suite.
