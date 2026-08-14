---
id: RT-worktree-web-exit-progress
area: RT
title: Follow an exit action through its announced phases
persona: Ada
journey: J-worktree-management
expected: Starting an exit action announces every phase up front, advances each phase from the per-worktree stream, states a skipped phase's reason in the daemon's words, attributes a mid-chain failure to its own phase with the captured command output, and ends with the single follow-up action the daemon named.
entry_points: S14 Worktree context -> exit action -> progress
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: internal/daemon/daemon_worktree_e2e_integration_test.go; web/e2e/__tests__/worktrees.spec.ts
last_report: docs/qa/reports/2026-08-13-worktree-support.md
overlaps: RT-worktree-exit-push-publish
---

QA impact: Task 07 adds streamed exit progress driven by per-worktree events.
