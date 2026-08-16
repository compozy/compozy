---
id: APP-cancel-dormant-update
area: APP
title: Cancel only a dormant update operation
persona: Ada
journey: J-desktop-agent-headless
expected: `compozy update --cancel` frees a dormant operation with a canceled result, while a live executor returns blocked with its holder and keeps the operation intact.
entry_points: compozy update --cancel -o json; update-operation.json
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: APP-update-recovery-state
---

Added 2026-08-16 for the durable operation lease and dormancy contract. Task_07 owns the competing
live-holder and expired-holder walks.
