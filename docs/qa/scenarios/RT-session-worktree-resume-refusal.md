---
id: RT-session-worktree-resume-refusal
area: RT
title: Refuse resume after a bound worktree disappears
persona: Théo
journey: J-worktree-management
expected: Resuming a session whose checkout was removed outside Compozy returns the deterministic missing-worktree refusal, preserves its binding and transcript, starts no process, and never falls back to the parent workspace root.
entry_points: compozy session resume; POST /api/sessions/:session_id/resume; session permalink
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: web/e2e/__tests__/worktrees.spec.ts; internal/daemon/daemon_worktree_e2e_integration_test.go
last_report: docs/qa/reports/2026-08-13-worktree-support.md
overlaps: RT-session-worktree-lifecycle
---

QA impact: Task 03 re-resolves the durable binding before every resume and launch. The Phase C walk
must remove the checkout out of band, attempt resume through a public surface, and confirm that the
stored session metadata and readable history remain unchanged after the refusal.

2026-10-27 (#992) QA impact: existing-session bind/rebind/unbind is additive. Verify idle-only
admission, unchanged session/transcript/window identity, next-turn cwd and file access, previous
process exit, and restart recovery with Claude and Codex. A missing binding still refuses resume;
explicit unbind returns to the workspace root. Session creation and conversation/worktree fork
retain their existing contracts. New worktree creation must reach ready before an existing
session can bind. New behavior awaits this branch's real-run evidence; historical passes do not
validate the added path.
