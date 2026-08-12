---
id: RT-worktree-api-surface-parity
area: RT
title: Read and mutate identical worktree state across agent surfaces
persona: Bruno
journey: J-worktree-management
expected: HTTP, UDS, CLI, native tools, and worktree streams preserve workspace isolation, canonical payloads, deterministic error codes, ordered replay, and repository capability diagnostics for the same runtime state.
entry_points: /api/workspaces/:workspace_id/worktrees; /api/workspaces/:workspace_id/worktrees/:worktree_id/exit; /api/worktrees/catalog-stream; compozy__worktree
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-worktree-cli-lifecycle
---

QA impact: Task 02 adds the full agent-operable worktree surface. The Phase C walk must compare
payloads and refusal bodies across transports, verify foreign-workspace calls disclose no target
data, and reconnect to the durable stream without gaps or duplicate events.

QA impact: Task 05 adds exit planning, actions, cancellation, and progress events. The Phase C walk
must compare the complete exit payload and deterministic failures across HTTP, UDS, and CLI.
