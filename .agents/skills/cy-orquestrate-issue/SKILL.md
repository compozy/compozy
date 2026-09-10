---
name: cy-orquestrate-issue
description: Orchestrate GitHub bug resolution through a new CompozyOS managed worktree and Codex Astra High session per issue. Investigate reported bugs, create missing issues, and dispatch a Goal-backed executor responsible for a detailed PR, green CI, and all CodeRabbit and Greptile feedback. Use when asked to delegate issue resolution; excludes spec task graphs, direct implementation, and review-only work on an existing PR.
---

# Orchestrate Issue Resolution

The orchestrator investigates and dispatches; a dedicated CompozyOS session implements each issue
through PR delivery.

## Contract and inputs

- Accept issue numbers/URLs or a bug report with screenshots, logs, or reproduction steps. Resolve
  the repository from the workspace remote; ask only if the target remains ambiguous.
- Default executor: agent `general`, provider `codex`, model `gpt-6-astra`, reasoning effort `high`.
  Explicit user overrides win. Verify the effective runtime; never silently substitute a model.
- Create one **new managed worktree and new logical session per issue**. Follow-ups for that issue
  go to its recorded session. Never repurpose an unrelated active session.
- An invocation to resolve issues authorizes the scoped issue, branch, push, PR, review replies,
  and review-remediation work. It does not authorize merge, destructive Git, deleting worktrees,
  installing reviewers, or changing the user's host runtime. Existing session authorization wins.
- Default orchestration ends after verified dispatch, with the executor responsible for the full
  delivery contract. If asked to accompany delivery, supervise until that contract is satisfied.
  A launch report must say **in progress**, never that the issue is resolved.

## 1. Load the runtime contract

Use the active harness to resolve `compozy__skill_view`. Load the Compozy skill and the full
references selected by its router for tools, worktrees, sessions, orchestration, and Goals:
`references/tools-and-skills.md`, `references/native-tools.md`, `references/worktrees.md`,
`references/runtime-operations.md`, `references/tasks-and-orchestration.md`, and
`references/loops.md`. Reuse complete current reads from this session. Follow source-qualified
`command_id` and retained-result continuation instructions when supplied.

Discover capabilities with `compozy__tool_search` and inspect live descriptors with
`compozy__tool_info`. The canonical IDs below are discovery keys, not harness invocation names.
Use the returned references and current schemas. A denied tool/skill read is not permission to
bypass policy through CLI or filesystem access.

## 2. Investigate and establish the issue

Read workspace instructions, Git status, the issue and its comments, and the affected code paths.
Reuse prior evidence; perform focused read-only investigation before dispatching a new bug report.
For runtime incidents, compare public daemon state and relevant logs with the UI report. Separate
confirmed causes, hypotheses, and unverified observations. Do not mutate user sessions to reproduce.
Use bounded independent explorers only when authorized and useful.

For a report without an issue, check for duplicates, then create an English GitHub issue containing
reproduction, actual/expected behavior, evidence and code pointers, acceptance criteria, and relevant
QA/compatibility impact. Verify publication. Sanitize private paths, internal runtime identifiers,
credentials, and unrelated log content. Use structured arguments or a body file for multiline text.
For an existing issue, reuse it; do not create a duplicate. A closed issue or existing PR requires
checking whether work remains before creating another implementation.

Several symptoms may belong to one evidenced defect. Keep that scope together; independent issues
get separate worktrees. Include later user corrections in the same issue's briefing. Avoid a broad
spec, audit, or unrelated cleanup unless requested.

## 3. Create and bind the executor

Refresh the remote base without changing the root checkout. Use the repository's target branch,
normally `origin/main`. Keep a compact mapping of issue URL, base SHA, worktree ID/path/branch,
session ID, prompt idempotency key, and eventual PR URL in the orchestration context. Preserve that
mapping in a handoff when needed; daemon state remains authoritative.

Perform these dependent operations in order:

1. `compozy__worktree_create`: workspace, base ref, unique name such as
   `issue-<number>-<slug>`, branch such as `fix/issue-<number>-<slug>`.
2. `compozy__worktree_inspect`: confirm creation reached `ready`, and retain the returned path/ID.
3. `compozy__session_create`: selected agent, descriptive issue name, exact workspace/worktree ID.
4. Load [assets/executor-prompt.md](assets/executor-prompt.md), fill every field, and send the full
   briefing with `compozy__session_prompt`. Set runtime explicitly on this first prompt:
   `{"provider":"codex","model":"gpt-6-astra","reasoning_effort":"high"}` unless overridden.
   Use unique stable `message_id` and `idempotency_key`; use `wait:false` for background dispatch.
5. `compozy__session_status`: confirm the prompt is accepted, runtime ready with the requested
   effective selection, and the session bound to the intended worktree. The executor verifies cwd
   and branch before editing. Inspect `compozy__goal_control` status to confirm actual activation;
   distinguish requested, activated, and executing successfully.

If creation or prompt admission has an ambiguous result, reconcile the worktree/session catalog
against the recorded mapping before retrying. Replay the same prompt idempotency key where supported;
do not blindly create duplicates. Respect session limits; never change the session type to bypass a
denial. This flow uses logical worktree-bound sessions, not the spec-task spawn/lease workflow.

If a required operation fails, complete independent work and report the precise blocker. Do not
claim a pending worktree, unbound runtime, or merely accepted prompt is executing successfully.

## 4. Follow-ups and delivery

Send additional evidence only to the same issue's recorded executor. Inspect its current turn and
busy-input capabilities: queue nonurgent additions; steer only when timely correction is needed,
using the expected active turn and checking whether the runtime would interrupt. Do not assume a
slash-shaped string in an agent prompt activates a skill or Goal; require the native operation.

The executor prompt owns implementation, validation, PR description, and the complete review cycle.
Do not duplicate its checks or routinely interrupt it for status. If supervising, investigate
finished CI failures while other jobs run and inspect new review evidence after relevant pushes.
An idle turn or active Goal flag alone is neither completion nor a blocker. If Goal supervision
fails while ordinary work continues, preserve the evidence, avoid repeated activation, and continue
authorized work; do not repair the host or clear runtime records as a shortcut.

Report the issue links, worktree/branch and session mapping, verified runtime/Goal state, and actual
launch or delivery status concisely. Do not stop the executor or remove its worktree after dispatch.
At delivery, verify the PR head, required checks, both reviewers' completed coverage, all finding
dispositions, and current description. Missing/red/pending CI or missing reviewer evidence means
delivery is still in progress or externally blocked. No merge unless separately requested.
