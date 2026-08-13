---
id: ET-worktree-hook-event-contract
area: ET
title: Gate worktree mutations while lifecycle observations stay fail-open
persona: Ada
journey: J-worktree-management
expected: Explicit pre_create and pre_remove denials block with the hook name and reason, hook execution errors fail open, adopted worktrees skip pre_create, observe-only consumers cannot block transitions, and every durable worktree event is ordered, workspace-attributed, correlated, and redacted on replay.
entry_points: worktree.pre_create|pre_remove hooks; worktree.created|adopted|removed observations; HTTP/UDS worktree mutations; per-worktree SSE; worktree catalog SSE; compozy hooks list|info|events|runs -o json
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-worktree-api-surface-parity; TA-task-per-run-worktree-isolation
---

QA impact: Task 01 adds the hook family and canonical event registry. The walk must cover manual and
per-run creation, removal, adoption, setup failure, creation cancellation, status refresh, safe
branch reclamation, and every exit-action terminal path, then reconnect and prove replay has no gap,
duplicate, foreign-workspace frame, or raw planted secret.
