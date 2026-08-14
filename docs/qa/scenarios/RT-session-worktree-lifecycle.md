---
id: RT-session-worktree-lifecycle
area: RT
title: Run a session inside a selected worktree
persona: Ada
journey: J-worktree-management
expected: Starting through CLI, HTTP, UDS, or native tools persists one ready-worktree binding, runs the agent and local tools inside that checkout, rejects cwd changes outside it, shares parent workspace memory, inherits the binding on child spawn, and exposes the same identity through filtered session reads without cross-workspace leakage.
entry_points: compozy session new --worktree|--new-worktree; compozy session list --worktree; HTTP/UDS POST /api/sessions; GET /api/sessions?worktree=; compozy__session_create.worktree|new_worktree; compozy__session_list.worktree
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/native-handoff-fixed-task.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/browser-worktree-bound-context.png; internal/daemon/daemon_worktree_e2e_integration_test.go
last_report: docs/qa/reports/2026-08-13-worktree-support.md
overlaps: RT-worktree-api-surface-parity; RT-worktree-cli-lifecycle
---

QA impact: Task 03 adds structural worktree binding to session creation, containment, persistence,
spawn, memory context, and list filtering. The Phase C walk must compare the same bound session over
every structured surface and prove that sibling and parent checkout files stay outside its tool root.
It must also prove a hook cannot rewrite the resolved cwd and a child cannot select or fall back to
another checkout.
