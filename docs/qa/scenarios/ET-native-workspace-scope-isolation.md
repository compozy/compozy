---
id: ET-native-workspace-scope-isolation
area: ET
title: Keep native tool calls inside the caller workspace
persona: Ada
journey: J-operate-workspace-context
expected: A workspace-bound session omits workspace input for same-workspace native operations, while any foreign workspace input is rejected before memory, automation, workspace, hook, or task-claim handlers execute.
entry_points: compozy__workspace_info; compozy__memory_*; compozy__automation_*; compozy__hooks_*; compozy__task_run_claim_next
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-workspace-resolution-chain; ET-workspace-host-api-mcp
---

Start sessions in two isolated registered workspaces. From workspace A, invoke representative native
reads and mutations with no `workspace` input and confirm dispatch fills workspace A. Then supply
workspace B by ID, name, and path and confirm the same denial occurs before any handler-visible
read or write. Repeat an authorized operator invocation and capture the explicit bypass audit path.
Verify pre-call hooks cannot rewrite the rebound workspace.

QA impact 2026-07-28: native workspace binding moved into the shared dispatch chokepoint and tool
schemas now use optional `workspace`. Planning flag only; no QA replay ran in this implementation
slice.
