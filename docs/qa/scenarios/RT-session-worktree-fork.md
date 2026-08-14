---
id: RT-session-worktree-fork
area: RT
title: Fork a live session into a worktree after confirmation
persona: Bruno
journey: J-worktree-management
expected: Choosing /worktree on a live idle session explains the consequences, creates a fresh clean session in the confirmed ready or newly created worktree, and leaves the original session and its uncommitted files untouched; canceling or invoking it mid-turn creates nothing.
entry_points: web session composer /worktree; session command catalog; HTTP/UDS POST /api/workspaces/:workspace_id/sessions/:session_id/worktree-fork
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: web/e2e/__tests__/worktrees.spec.ts; internal/daemon/daemon_worktree_e2e_integration_test.go
last_report: docs/qa/reports/2026-08-13-worktree-support.md
overlaps: RT-session-worktree-lifecycle
---

QA impact: Task 03 exposes the runtime-owned command and fork contract; Task 07 supplies the visual
confirmation. The Phase C walk must cover ready and newly created targets, cancellation, repeated
confirmed invocations, and the explicit unavailable reason while a prompt turn is active.
