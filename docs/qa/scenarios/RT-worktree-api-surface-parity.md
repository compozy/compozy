---
id: RT-worktree-api-surface-parity
area: RT
title: Read and mutate identical worktree state across agent surfaces
persona: Bruno
journey: J-worktree-management
expected: HTTP, UDS, CLI, native tools, and both worktree streams preserve workspace isolation, canonical payloads, deterministic error codes, risk and approval metadata, ordered replay, redacted events, and repository capability diagnostics for the same runtime state.
entry_points: HTTP/UDS GET|POST /api/workspaces/:workspace_id/worktrees; POST .../adopt; GET .../:worktree_id; GET .../status|exit|stream; POST .../cancel|dismiss|exit/actions|exit/cancel; DELETE .../:worktree_id; GET /api/worktrees/catalog-stream; compozy__worktree_list|inspect|create|remove
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
data, assert native risk/approval/capability metadata, and reconnect through `after_sequence` and
`Last-Event-ID` without gaps, duplicate events, unredacted diagnostics, or cache crossover.

QA impact: Task 05 adds exit planning, actions, cancellation, and progress events. The Phase C walk
must compare the complete exit payload and deterministic failures across HTTP, UDS, and CLI.
