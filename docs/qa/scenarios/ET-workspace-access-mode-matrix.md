---
id: ET-workspace-access-mode-matrix
area: ET
title: Decide cross-workspace requests from the session permission mode
persona: Ada
journey: J-operate-workspace-context
expected: An approve-all session reaches another workspace at every seam, a deny-all session is denied at every seam with the permission-mode hint and no prompt, and an approve-reads session is denied with the same hint at the agent-identity, task, spawn, and coordination seams; each policy evaluation produces the expected workspace.access_granted or workspace.access_denied audit event in a healthy store, naming target, seam, source, and mode.
entry_points: compozy__workspace_info; compozy__memory_list; compozy__task_run_claim_next; compozy spawn --workspace; compozy logs
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-workspace-access-prompt-outcomes; ET-native-workspace-scope-isolation; MS-workspace-resolution-chain
---

Register two workspaces in one isolated `COMPOZY_HOME`. Start a session in workspace A for an agent
whose `permissions` is `approve-all`, and from it name workspace B on a native tool call, a task
claim, a spawn, and a workspace coordination read. Confirm each crossing succeeds and that the
downstream behavior in B is the same it would be at home.

Repeat with a `deny-all` agent and confirm every seam denies, no prompt is raised anywhere, and the
denial carries the exact hint `cross-workspace access is denied by this session's permission mode;
ask the operator to set the agent's permissions.mode to approve-all, or approve the prompt when
asked`. Native denials must report reason code `workspace_access_denied`.

Repeat with an `approve-reads` agent and confirm the non-tool seams deny with the same hint and never
prompt. The native-tool prompt itself is `ET-workspace-access-prompt-outcomes`.

Confirm the operator path is unaffected: operator commands and global reads still reach both
workspaces. Then read `compozy logs --type workspace.access_granted` and `compozy logs --type
workspace.access_denied`; confirm one event per policy evaluation, scoped to the actor's own
workspace, with target workspace, seam, decision source, and mode in the payload. Spawn keeps both
validation phases, so one spawn can produce two policy evaluations.

`ET-native-workspace-scope-isolation` owns same-workspace binding, canonical target resolution, and
the pre-handler policy boundary; this file owns the mode outcomes and operator bypass.

QA impact 2026-07-29: new behavior from the cross-workspace access program (ADR-007). The built-in
default `[permissions] mode` is `approve-all`, so a default install crosses workspaces — cover that
default explicitly. Planning flag only; no QA replay ran in this documentation slice.
