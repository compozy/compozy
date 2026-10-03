---
id: ET-native-workspace-scope-isolation
area: ET
title: Bind native tool calls to the caller workspace before policy
persona: Ada
journey: J-operate-workspace-context
expected: A workspace-bound session omits workspace input for same-workspace native operations, while a foreign workspace reference is canonicalized and sent through the shared cross-workspace policy before memory, automation, workspace, hook, or task-claim handlers execute; policy denial prevents every handler-visible read or write, and global/all scope remains operator-only for workspace-bound sessions.
entry_points: compozy__workspace_info; compozy__memory_*; compozy__automation_*; compozy__hooks_*; compozy__task_run_claim_next
qa_status: fail
bug_ids: BUG-20260729-nearest-workspace-case-alias; BUG-20261002-native-approval-input-mismatch; BUG-20261002-hook-tool-matcher-docs; BUG-20261002-native-workspace-identity-boundaries; BUG-20261002-native-hook-dispatch-missing; BUG-20261002-native-task-filter-error
fix_status: pending
retest_status:
fix_commits: 4e81f17
evidence: /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-124649-419333-lab/qa-artifacts/qa/notes/cross-workspace-access-results.md;/Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-10-02-untested.md
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

QA 2026-07-29: an approve-reads session invoked workspace info with omitted workspace and its own
workspace by ID, name, and path. All four calls reached the same canonical ID with zero permission
request and zero cross-workspace policy event. The adjacent case-alias discovery regression was
fixed and retested in the same canary.

QA impact 2026-07-29 (deep-review remediation): reset to `untested` after the generic tool envelope
began resolving aliases and paths before policy evaluation and workspace-bound sessions regained the
pre-handler denial for `scope=global` and `scope=all`. Recheck representative config, hook,
automation and memory tools; operator global scope must remain available.


2026-09-27 scope update: current coverage follows the surviving product surfaces; a fresh walk is required.

QA 2026-10-02: the scoped operator approval mismatch is repaired and replayed through CLI, HTTP
and UDS, including changed-input and one-shot-token refusals. Operator session selectors do not
establish agent authority. The real hosted-agent boundary and hook-rewrite legs remain unverified;
two hook creation attempts exposed stale `tool_name` guidance. Corrected `tool_id` authoring is
verified through create/restart/catalog/delete, but the scoped operator invocation did not fire
the hook. See the dated report for receipts and the explicit Pending matrix disposition.

The hosted operations agent subsequently reached Studio and Editorial through actual native
workspace, memory, automation, hook and task reads; ID and path references resolved to Editorial.
The deny-all librarian stopped at tool discovery approval, which is not evidence of foreign-handler
denial. Both sessions are stopped. Mutation, global/all and hook-rewrite legs remain Pending.

A subsequent fresh walk saved a Studio reference memory, created a disabled weekly job, and made
one empty task claim through omitted-workspace native calls. Independent memory read-back passed,
but the operator could not retrieve the job. An approve-reads session was correctly denied global
job scope, yet incorrectly prompted for access to its own project path. The installed required
pre-call hook had no runs after native workspace-info execution. Those two failures are filed above;
all caller sessions are stopped and the owned hook is removed. Full scenario disposition remains Pending.

Identity repair `563990440` is verified: own aliases reach Studio, foreign rejected reads disclose no
Editorial data, and native/operator automation paths retrieve the same persisted job. The separate
task-filter error classification finding is linked above; hook and remaining mutation legs stay Pending.

The native hook repair now passes a fresh hosted replay: an own-project memory mutation is refused
when its pre-call hook tries to redirect it to Editorial, and a separate explicit Editorial mutation
is refused after the operator rejects canonical cross-workspace access. Each hook runs exactly once
with durable session lifecycle events. Both public memory catalogs remain unchanged; sessions are
stopped and the hook removed. See `native-hook-fixed-*` and the dated report. Remaining allowed
foreign-mutation and `all`-scope evidence still needs reconciliation before this full row is closed.
