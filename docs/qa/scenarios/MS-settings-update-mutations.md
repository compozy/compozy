---
id: MS-settings-update-mutations
area: MS
title: Apply and cancel an update through settings
persona: Dora
journey: J-desktop-update-moment
expected: Settings apply returns an accepted durable operation id, cancel releases only a dormant operation, and HTTP and UDS expose the same typed state and refusal results.
entry_points: GET /api/settings/update; POST /api/settings/update/apply; POST /api/settings/update/cancel over HTTP and UDS
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-034; APP-cancel-dormant-update
---

Added 2026-08-16 for the asynchronous settings update contract. Task_07 owns transport parity,
detached execution, live-holder refusal, and final projection checks.
