---
id: ET-workspace-access-mode-matrix
area: ET
title: Decide cross-workspace requests from the session permission mode
persona: Ada
journey: J-cross-workspace-access
expected: An approve-all session reaches another workspace at every seam, a deny-all session is denied at every seam with the permission-mode hint and no prompt, and an approve-reads session is denied with the same hint at the agent-identity, task and spawn seams; each policy evaluation produces the expected workspace.access_granted or workspace.access_denied audit event in a healthy store, naming target, seam, source, and mode.
entry_points: compozy__workspace_info; compozy__memory_list; compozy__task_run_claim_next; compozy task next --workspace; compozy spawn --workspace; POST /api/agent/spawn (HTTP+UDS); POST /api/agent/tasks/claim-next (HTTP+UDS); GET /api/agent/me (HTTP+UDS); compozy logs --type workspace.access_denied; /docs/cli/spawn; /docs/agents/spawning; /docs/autonomy/safe-spawn; /docs/configuration/config-toml; /docs/hooks/event-catalog; /docs/sessions/permissions#cross-workspace-access; /docs/workspaces; /docs/workspaces/resolver#isolation-and-cross-workspace-access; skills/compozy/references/native-tools.md; skills/compozy/references/agent-definitions.md
qa_status: skipped
bug_ids: BUG-20260730-tool-invoke-202-empty-success; BUG-20260910-cursor-mcp-error-schema; BUG-20260910-cursor-denial-hides-workspace-policy; BUG-20260910-task-claim-workspace-identity
fix_status: pending
retest_status:
fix_commits: 4ef8e8c;7285bf3c
evidence: docs/qa/evidence/2026-09-10-qa-execution-unblock/cross-deny-events-final.json; docs/qa/evidence/2026-09-10-qa-execution-unblock/cross-deny-audit.json
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-workspace-access-prompt-outcomes; ET-native-workspace-scope-isolation; MS-workspace-resolution-chain
---

Create source and target workspaces and exercise the listed seams with approve-all, deny-all, and approve-reads sessions. Read and write decisions must follow the configured mode with the permission-mode hint on refusal and no hidden prompt. Repeat via CLI, native tools, HTTP and UDS. Inspect workspace.access_granted and workspace.access_denied for the target, seam, source, and mode; unauthenticated requests must never gain a cross-workspace grant.


2026-09-27 scope update: current coverage follows the surviving product surfaces; a fresh walk is required.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.
