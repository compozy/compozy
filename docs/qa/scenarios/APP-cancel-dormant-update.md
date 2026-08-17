---
id: APP-cancel-dormant-update
area: APP
title: Cancel only a dormant update operation
persona: Ada
journey: J-desktop-agent-headless
expected: `compozy update --cancel` cancels and archives a waiting-for-app or expired-lease dormant operation in `update-history.jsonl`, frees acquisition, and returns canceled; a live executor returns blocked with its holder and keeps the operation intact.
entry_points: compozy update --cancel -o json; update-operation.json; update-history.jsonl
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: APP-update-recovery-state
---

Added 2026-08-16 for the durable operation lease and dormancy contract. Task 07 owns the competing
live-holder and expired-holder walks.
