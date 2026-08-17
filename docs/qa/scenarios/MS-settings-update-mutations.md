---
id: MS-settings-update-mutations
area: MS
title: Apply and cancel an update through settings
persona: Dora
journey: J-desktop-update-moment
expected: Settings apply returns an accepted durable operation id; cancel archives only a dormant operation with outcome `canceled` and frees acquisition; HTTP and UDS expose the same typed state and refusal results.
entry_points: GET /api/settings/update; POST /api/settings/update/apply; POST /api/settings/update/cancel over HTTP and UDS
qa_status: pass
bug_ids:
fix_status: 
retest_status: 
fix_commits: 
evidence: docs/qa/reports/2026-08-17-electron-shell.md
last_report: docs/qa/reports/2026-08-17-electron-shell.md
overlaps: MS-034; APP-cancel-dormant-update
---

Added 2026-08-16 for the asynchronous settings update contract. Task 07 owns transport parity,
detached execution, live-holder refusal, and final projection checks.
