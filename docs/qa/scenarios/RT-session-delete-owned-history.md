---
id: RT-session-delete-owned-history
area: RT
title: Delete a stopped session with owned runtime history
persona: Bruno
journey: J-11
expected: Confirming deletion of a stopped session removes its catalog row, transcript/history, permission log, token statistics, and other session-owned rows atomically while preserving every other session.
entry_points: Web session Delete session modal; HTTP session DELETE; global session catalog
qa_status: untested
bug_ids: BUG-20260714-session-delete-history-fk
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-consumer-saas-growth-20260714-194637-422214-lab/qa-artifacts/qa/session-delete/; /Users/pedronauck/dev/qa-labs/agh-consumer-saas-growth-20260714-194637-422214-lab/qa-artifacts/qa/screenshots/session-delete-fresh-web.png
last_report: docs/qa/reports/2026-07-14-consumer-saas-growth.md
overlaps: RT-034;RT-035;TA-task-create-async-activation
---

Exercise a real provider session with tool permission and token history; an empty synthetic session does not prove the ownership boundary.

2026-07-14 historical pre-rebase retest: the original failing task-role session `sess-12b2c865a27ecc72` survived that branch's v1 → v2 migration with five permission rows, then deleted successfully through the UI. Fresh HTTP read returned 404 and direct counts for its session, permission, and token rows were all zero. The merged stream carries the cascade in v3.

2026-07-14 final lifecycle retest: real Cursor/Grok user session `sess-6a4f5db74d195230` stored one authored prompt and response, stopped cleanly, and deleted through the named confirmation modal with `Session deleted.` The canonical manager suite also proved pre-commit directory rollback and post-commit tombstone retry semantics.

QA impact 2026-07-14: the API fallback was removed and daemon boot now requires the atomic workspace-removal preparer. Reset pending final-worktree deletion and foreign-session preservation evidence.

2026-07-14 final-worktree control: public detail, history, transcript, usage, event, and catalog reads stopped exposing the deleted session, while the neighboring session retained its history and usage. Retest promoted to pass.
