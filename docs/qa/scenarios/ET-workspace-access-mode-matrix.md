---
id: ET-workspace-access-mode-matrix
area: ET
title: Decide cross-workspace requests from the session permission mode
persona: Ada
journey: J-cross-workspace-access
expected: An approve-all session reaches another workspace at every seam, a deny-all session is denied at every seam with the permission-mode hint and no prompt, and an approve-reads session is denied with the same hint at the agent-identity, task, spawn, and coordination seams; each policy evaluation produces the expected workspace.access_granted or workspace.access_denied audit event in a healthy store, naming target, seam, source, and mode.
entry_points: compozy__workspace_info; compozy__memory_list; compozy__task_run_claim_next; compozy task next --workspace; compozy spawn --workspace; compozy network peers --workspace; compozy network channels update --workspace; compozy network coordination status --workspace; POST /api/agent/spawn (HTTP+UDS); POST /api/agent/tasks/claim-next (HTTP+UDS); GET /api/agent/me (HTTP+UDS); GET /api/workspaces/:workspace_id/network/peers (HTTP+UDS); PATCH /api/workspaces/:workspace_id/network/channels/:channel (HTTP+UDS); GET /api/workspaces/:workspace_id/network-coordination (HTTP+UDS); PUT /api/workspaces/:workspace_id/network-coordination (HTTP+UDS); compozy logs --type workspace.access_denied; /docs/cli/spawn; /docs/agents/spawning; /docs/autonomy/safe-spawn; /docs/configuration/config-toml; /docs/hooks/event-catalog; /docs/sessions/permissions#cross-workspace-access; /docs/workspaces; /docs/workspaces/resolver#isolation-and-cross-workspace-access; skills/compozy/references/native-tools.md; skills/compozy/references/agent-definitions.md
qa_status: fail
bug_ids: BUG-20260729-coordination-cli-drops-agent-identity; BUG-20260730-tool-invoke-202-empty-success; BUG-20260910-cursor-mcp-error-schema; BUG-20260910-cursor-denial-hides-workspace-policy; BUG-20260910-task-claim-workspace-identity
fix_status: pending
retest_status: fail
fix_commits: 4ef8e8c;7285bf3c
evidence: docs/qa/evidence/2026-09-10-qa-execution-unblock/cross-deny-events-final.json; docs/qa/evidence/2026-09-10-qa-execution-unblock/cross-deny-audit.json
last_report: docs/qa/reports/2026-09-10-qa-execution-unblock.md
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

## 2026-09-10 execution checkpoint

The approve-all Cursor Grok 4.6 High Fast walk reached the foreign workspace, but complete native
results were missing from MCP text content. See [BUG-20260910-hosted-mcp-text-result](../bugs/BUG-20260910-hosted-mcp-text-result.md).
The separate spawn schema error is still under diagnosis; a background worker raced the claim
fixture, so the no-claimable result is not proven to be a product defect. This partial walk does
not change the scenario verdict.

The supported foreign CLI spawn retest exposed [BUG-20260910-terminal-agent-identity](../bugs/BUG-20260910-terminal-agent-identity.md): native terminal processes lost the caller identity. The coordination read succeeded without agent identity and is not a permission pass. The session ended; repair and real retest are in progress.


## 2026-09-10 restricted Cursor result

The prior MCP text and managed-terminal identity defects were repaired and retested before main integration; their current commits are `790039e93` and `dcb9d25a4`. The fresh deny-all walk instead fails the specified diagnostic contract: Cursor refuses native calls before workspace evaluation, including the terminal needed to launch agent CLI commands. No workspace prompt occurred, but no workspace_access_denied/hint/CLI exit77 or workspace audit occurred either. See [the provider preemption finding](../bugs/BUG-20260910-cursor-denial-hides-workspace-policy.md). The scenario is fail on this observed branch; remaining unwalked surfaces are not passing evidence. Approve-reads and supported seams remain in progress.

The approve-reads CLI spawn/coordination/peers branches independently verified exit77 with the canonical hint and matching audits. A clean task-next retry exited77 without the hint. Approve-all native exact claim returned empty while the run remained queued for over a minute; diagnosis identified stable-vs-registration workspace identity confusion. [Claim boundary repair](../bugs/BUG-20260910-task-claim-workspace-identity.md) is in progress.

Claim boundary repair is now fixed and retested: fresh approve-all native foreign-name claim completed the exact run/file; fresh approve-reads CLI task-next returned77 with the canonical hint. Independent task inspect, target file, terminal journal, and workspace audits agree (`claim-retest-proof.json`). Both sessions stopped. The full scenario remains fail because the independent Cursor deny-all diagnostic defect remains open; unwalked HTTP/UDS branches are not awarded passes.

### PR integration validation boundary

PR #624 additionally repairs ACP terminal identity selection (Compozy scope versus provider session ID) and avoids duplicating an already complete MCP JSON preview. These corrections are exercised by the existing ACP conformance, hosted-proxy and daemon runtime E2E suites. They do not close the remaining real-provider workspace permission matrix; its unresolved disposition is unchanged.
