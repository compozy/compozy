---
id: APP-cancel-dormant-update
area: APP
title: Cancel only a dormant update operation
persona: Ada
journey: J-desktop-agent-headless
expected: `compozy update --cancel` cancels and archives a waiting-for-app or expired-lease dormant operation in `update-history.jsonl`, frees acquisition, and returns canceled; a live executor returns blocked with its holder and keeps the operation intact.
entry_points: compozy update --cancel -o json; update-operation.json; update-history.jsonl
qa_status: pass
bug_ids:
fix_status: 
retest_status: 
fix_commits: 
evidence: docs/qa/reports/2026-08-17-electron-shell.md
last_report: docs/qa/reports/2026-08-17-electron-shell.md
overlaps: APP-update-recovery-state
---

Added 2026-08-16 for the durable operation lease and dormancy contract. Task 07 owns the competing
live-holder and expired-holder walks.

Issue 665 regression: a shell that exited during `applying` must not block cancellation merely
because its holder lease remains unexpired. Check the actual PID/start identity; a matching live
holder stays blocked. Once the operation deadline expires and no matching live installer holds
`applying`, an authoritative `update --check` read or the daemon's periodic recovery pass must
archive the pre-handoff failure without opening the app again. A matching live installer retains
its operation even after its lease or deadline expires and blocks competing acquisition. Repeat
`--check` after that holder exits and confirm no `applying` projection and one history record.
Installer handoff and runtime replacement retain their existing recovery ownership.
