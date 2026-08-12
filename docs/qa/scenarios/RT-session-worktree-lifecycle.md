---
id: RT-session-worktree-lifecycle
area: RT
title: Run a session inside a selected worktree
persona: Ada
journey: J-worktree-management
expected: Starting through CLI, HTTP, UDS, or native tools persists one worktree binding, runs the agent and local tools inside that checkout, shares parent workspace memory, inherits the binding on spawn, and exposes the same identity through filtered session reads.
entry_points: compozy session new --worktree|--new-worktree; POST /api/sessions; compozy__session_create
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-worktree-api-surface-parity; RT-worktree-cli-lifecycle
---

QA impact: Task 03 adds structural worktree binding to session creation, containment, persistence,
spawn, memory context, and list filtering. The Phase C walk must compare the same bound session over
every structured surface and prove that sibling and parent checkout files stay outside its tool root.
