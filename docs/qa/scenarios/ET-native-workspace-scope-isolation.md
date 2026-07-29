---
id: ET-native-workspace-scope-isolation
area: ET
title: Bind native tool calls to the caller workspace before policy
persona: Ada
journey: J-operate-workspace-context
expected: A workspace-bound session omits workspace input for same-workspace native operations, while a foreign workspace reference is canonicalized and sent through the shared cross-workspace policy before memory, automation, workspace, hook, or task-claim handlers execute; policy denial prevents every handler-visible read or write.
entry_points: compozy__workspace_info; compozy__memory_*; compozy__automation_*; compozy__hooks_*; compozy__task_run_claim_next
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-workspace-access-mode-matrix; ET-workspace-access-prompt-outcomes; MS-workspace-resolution-chain; ET-workspace-host-api-mcp
---

Start sessions in two isolated registered workspaces. From workspace A, invoke representative native
reads and mutations with no `workspace` input and confirm dispatch fills workspace A. Then supply
workspace B by ID, name, and path and confirm each reference resolves to B's canonical ID before the
shared policy runs. Under a denying mode, confirm the denial occurs before any handler-visible read
or write. Under a mode or session answer that allows crossing, confirm the handler receives the
canonical B scope. Verify pre-call hooks cannot rewrite the rebound workspace.

Mode outcomes, prompt semantics, and operator bypass are owned by
`ET-workspace-access-mode-matrix` and `ET-workspace-access-prompt-outcomes`; do not duplicate their
full matrices here.

QA impact 2026-07-28: native workspace binding moved into the shared dispatch chokepoint and tool
schemas now use optional `workspace`. Planning flag only; no QA replay ran in this implementation
slice.

QA impact 2026-07-29: foreign native workspace input is no longer unconditionally rejected. The
binder now canonicalizes the target and consults the shared mode-anchored policy before the handler.
Status remains untested; no QA replay ran in this documentation slice.

Planning 2026-07-29 (task 06): stays on `J-operate-workspace-context` — it owns same-workspace
binding and the pre-handler boundary, not the mode outcomes. Its overlapping neighbours
`ET-workspace-access-mode-matrix` and `ET-workspace-access-prompt-outcomes` moved to
`J-cross-workspace-access`, which makes this file the adjacent regression canary for that cycle.
Settled by charter `CH-workspace-binding-canary`.
