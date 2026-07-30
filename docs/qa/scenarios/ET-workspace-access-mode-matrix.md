---
id: ET-workspace-access-mode-matrix
area: ET
title: Decide cross-workspace requests from the session permission mode
persona: Ada
journey: J-cross-workspace-access
expected: An approve-all session reaches another workspace at every seam, a deny-all session is denied at every seam with the permission-mode hint and no prompt, and an approve-reads session is denied with the same hint at the agent-identity, task, spawn, and coordination seams; each policy evaluation produces the expected workspace.access_granted or workspace.access_denied audit event in a healthy store, naming target, seam, source, and mode.
entry_points: compozy__workspace_info; compozy__memory_list; compozy__task_run_claim_next; compozy task next --workspace; compozy spawn --workspace; compozy network peers --workspace; compozy network channels update --workspace; compozy network coordination status --workspace; POST /api/agent/spawn (HTTP+UDS); POST /api/agent/tasks/claim-next (HTTP+UDS); GET /api/agent/me (HTTP+UDS); GET /api/workspaces/:workspace_id/network/peers (HTTP+UDS); PATCH /api/workspaces/:workspace_id/network/channels/:channel (HTTP+UDS); GET /api/workspaces/:workspace_id/network-coordination (HTTP+UDS); PUT /api/workspaces/:workspace_id/network-coordination (HTTP+UDS); compozy logs --type workspace.access_denied; /docs/cli/spawn; /docs/agents/spawning; /docs/autonomy/safe-spawn; /docs/configuration/config-toml; /docs/hooks/event-catalog; /docs/sessions/permissions#cross-workspace-access; /docs/workspaces; /docs/workspaces/resolver#isolation-and-cross-workspace-access; skills/compozy/references/native-tools.md; skills/compozy/references/agent-definitions.md
qa_status: blocked-verify
bug_ids: BUG-20260729-coordination-cli-drops-agent-identity; BUG-20260730-tool-invoke-202-empty-success
fix_status: fixed
retest_status: pass
fix_commits: 4ef8e8c;7285bf3c
evidence: /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-124649-419333-lab/qa-artifacts/qa/notes/cross-workspace-access-results.md
last_report: docs/qa/reports/2026-07-29-site-improvs-deep-review.md
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

Walk each mode across all four public shapes of the same crossing, not only the native tool: the
agent-driven CLI (`compozy task next --workspace`, `compozy spawn --workspace`, `compozy network
coordination status --workspace`), which must exit 77 with a daemon-origin denial rather than a local
pre-flight block; the agent identity routes over both HTTP and UDS (`GET /api/agent/me`, `POST
/api/agent/spawn`, `POST /api/agent/tasks/claim-next`); and the coordination routes (`GET`/`PUT
/api/workspaces/:workspace_id/network-coordination`), where reads follow the mode while writes still
require the operator. Confirm the exit code, reason code, and hint text match across surfaces for the
same decision.

Finally read the shipped guidance as an operator would and confirm it tells the truth about what you
just observed: the CLI spawn reference; the agent spawning, safe-spawn, configuration, event-catalog,
permissions, workspace-index, and resolver pages; and the official skill's native-tool and
agent-definition references.

`ET-native-workspace-scope-isolation` owns same-workspace binding, canonical target resolution, and
the pre-handler policy boundary; this file owns the mode outcomes and operator bypass.

QA impact 2026-07-29: new behavior from the cross-workspace access program (ADR-007). The built-in
default `[permissions] mode` is `approve-all`, so a default install crosses workspaces — cover that
default explicitly. Planning flag only; no QA replay ran in this documentation slice.

Planning 2026-07-29 (task 06): re-homed from `J-operate-workspace-context` to the new
`J-cross-workspace-access` flow, which owns the mode branches, prompt outcomes, consent lifetime, and
audit visibility. Entry points widened to the agent-driven CLI, the HTTP/UDS identity and
coordination routes, and the shipped site/official-skill guidance. Settled by charter
`CH-cross-workspace-mode-seams`.

QA 2026-07-29: the deny/read/all matrix passed across native tools, agent CLI, HTTP, and UDS after
fixing the coordination CLI identity transport. Denials carried the exact daemon hint, deny-all
raised zero workspace prompts, approve-reads prompted only at the tool seam, and approve-all crossed
promptless. All four audit readers agreed on attributable events.

QA impact 2026-07-29 (deep-review remediation): reset to `untested` after workspace policy coverage
expanded from coordination to every workspace-scoped Network read and mutation, canonical workspace
ULIDs became valid policy targets, and foreign child counts began enforcing the target workspace's
`max_active_per_workspace` cap. Recheck deny/read/all over CLI, HTTP, and UDS Network routes and the
foreign-target spawn cap; do not reuse the earlier pass as evidence for these changed paths.
