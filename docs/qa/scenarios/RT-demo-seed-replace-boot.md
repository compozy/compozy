---
id: RT-demo-seed-replace-boot
area: RT
title: Recreate a populated demo seed without changing history
persona: Dora
journey: J-prepare-demo-recording
expected: Replacing the seed keeps counts stable, removes obsolete seed-owned state, and daemon boot preserves every imported Loop outcome as read-only history
entry_points: go run ./scripts/demo-seed; compozy daemon start; GET /api/workspaces
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-demo-seed-history-20260822-181255-352261-lab/qa-artifacts/qa/seed-replace.json; /Users/pedronauck/dev/qa-labs/compozy-demo-seed-history-20260822-181255-352261-lab/qa-artifacts/qa/loop-runs-api-after-replace-boot.json; /Users/pedronauck/dev/qa-labs/compozy-demo-seed-history-20260822-181255-352261-lab/qa-artifacts/qa/historical-run-detail.png; /Users/pedronauck/dev/qa-labs/compozy-demo-seed-history-20260822-181255-352261-lab/qa-artifacts/qa/historical-runs-list.png; /Users/pedronauck/dev/qa-labs/compozy-demo-seed-history-20260822-181255-352261-lab/qa-artifacts/qa/unowned-replace-refusal.txt
last_report: docs/qa/reports/2026-08-22-demo-seed-history.md
overlaps:
---

This scenario owns the cross-surface seed contract. It covers repeatable fixture creation, safe ownership boundaries, live-daemon reconciliation, and truthful read-only presentation of imported Loop runs.

The 2026-08-22 targeted walk confirmed stable IDs and counts across replace, zero live Loop aggregates after daemon boot, read-only historical controls, preserved unowned files, and the populated Goal, worktree, memory, notification, automation, task, and transcript surfaces.
