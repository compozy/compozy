---
id: RT-session-worktree-resume-refusal
area: RT
title: Refuse resume after a bound worktree disappears
persona: Théo
journey: J-worktree-management
expected: Resuming a session whose checkout was removed outside Compozy returns the deterministic missing-worktree refusal, preserves its binding and transcript, starts no process, and never falls back to the parent workspace root.
entry_points: compozy session resume; POST /api/sessions/:session_id/resume; session permalink
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-session-worktree-lifecycle
---

QA impact: Task 03 re-resolves the durable binding before every resume and launch. The Phase C walk
must remove the checkout out of band, attempt resume through a public surface, and confirm that the
stored session metadata and readable history remain unchanged after the refusal.
