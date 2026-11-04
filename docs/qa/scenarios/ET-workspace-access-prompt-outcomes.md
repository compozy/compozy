---
id: ET-workspace-access-prompt-outcomes
area: ET
title: Resolve a cross-workspace prompt, remember a read, and expire a session answer
persona: Bruno
journey: J-cross-workspace-access
expected: An approve-reads session hitting the native-tool boundary raises one pending permission; a read offers allow_once, allow_always, reject_once, and reject_session, and a write offers allow_once, allow_session, reject_once, and reject_session. Once answers apply to that call only; an operator allow_always on a read stores a revocable tool approval scoped to the agent, tool, and target workspace/profile that later sessions reuse without prompting; session answers apply to every seam for the rest of the session, and stopping the session clears them.
entry_points: compozy__workspace_info; compozy__session_status; compozy tool approvals list; compozy tool approvals revoke; Settings → Tool approvals; compozy__task_run_claim_next; compozy spawn --workspace; compozy session approve <session-id> --request-id <request-id> --decision <allow-once|allow-always|reject-once|reject-always>; POST /api/workspaces/:workspace_id/sessions/:session_id/approve; compozy logs --type workspace.access_granted; GET /api/logs; compozy__logs; compozy__observe_search; /docs/sessions/permissions#the-prompt-in-approve-reads
qa_status: skipped
bug_ids: BUG-20260730-tool-invoke-202-empty-success
fix_status: fixed
retest_status:
fix_commits: 7285bf3c
evidence: /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-124649-419333-lab/qa-artifacts/qa/notes/cross-workspace-access-results.md
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-workspace-access-mode-matrix; ET-native-tool-approval-grants
---

Start an `approve-reads` session in workspace A and call a native read tool naming workspace B.
Confirm the prompt offers `allow_once`, `allow_always`, `reject_once`, and `reject_session`, labeled
Allow once, Always allow, Reject once, and Reject for this session. Writes offer `allow_session`
(Allow for this session) instead of `allow_always`.

Answer `allow-once` and confirm the call proceeds and the next crossing prompts again. Answer
`reject-once` and confirm the call is denied with the permission-mode hint while a later crossing
still prompts.

Answer `allow-always` on the read and confirm a scoped record appears in tool approvals. Another
session read with the same tool in workspace B and the same profile proceeds without a prompt,
including after actor session rotation and daemon restart. Other tools, workspaces, profiles,
agents, and writes remain outside that grant. Revoke it and confirm the next read prompts again.
The grant does not admit task or spawn operations.

For a write, answer `allow-always` and confirm it resolves to `allow_session`: later crossings by
that session succeed without a prompt, including task and spawn seams where policy permits.
This session answer creates no durable approval record and expires on stop or daemon restart.

Answer `reject-always` and confirm it resolves to `reject_session` and denies every later crossing by
that session at every seam.

Leave one prompt unanswered past `[tools.policy] approval_timeout_seconds` and confirm the request
denies and stores no session answer. Inspect `compozy logs`: the initial prompt-eligible policy miss
is `workspace.access_denied`, while later evaluations admitted by `allow_session` are
`workspace.access_granted`. Confirm target, seam, source, and mode remain attributable.

Answer through both operator surfaces — `compozy session approve <session-id> --request-id
<request-id> --decision <allow-once|allow-always|reject-once|reject-always>` and `POST
/api/workspaces/:workspace_id/sessions/:session_id/approve` — and confirm the same four option ids
and the same resulting behavior. Read the audit trail through all four reader surfaces (`compozy logs
--type`, `GET /api/logs`, `compozy__logs`, `compozy__observe_search`) and confirm they agree on the
same decisions. Audit appends are best effort, so a decision missing from a degraded event store is a
store finding, not proof the decision differed — verify against a healthy store.

QA impact 2026-07-29: new behavior from the cross-workspace access program (ADR-007). Planning flag
only; no QA replay ran in this documentation slice.

Planning 2026-07-29 (task 06): re-homed to `J-cross-workspace-access` and re-assigned from Ada to
Bruno — the agent triggers the prompt, but the answering, the consent lifetime, and the audit read
are all operator work. Entry points widened to the cross-seam reuse surfaces, both answer surfaces,
and all four audit readers. Settled by charter `CH-cross-workspace-consent-audit`.

QA 2026-07-29: allow/reject once and session answers passed through both operator answer surfaces.
Session grants crossed task and spawn seams, expired on stop and daemon restart, and
an unanswered request timed out without storing consent. CLI, HTTP, native logs, and native search
agreed on the attributable audit trail.

2026-09-27 scope update: current coverage follows the surviving product surfaces; a fresh walk is required.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

QA impact 2026-11-03 (#1169): an `approve-reads` session that calls a session tool with only a
foreign `session_id` raises one prompt titled `Read session <id> outside this workspace or
profile`, with the same title and options whether or not the session exists; `reject_once` returns
the consent-rejected denial and `allow_once` returns the session (redacted) or `session_not_found`.
A `profile` or `all_profiles` read raises the same four options titled `Read profile <id>` or
`Read all profiles`.


QA result 2026-11-03 (#1169): targeted isolated acpmock walk over hosted MCP and HTTP passed for the journeys above; Web and live providers were not walked. Evidence: `docs/qa/evidence/2026-11-03-issue-1169/verification-report.md`.

QA impact 2026-11-04 (#1270): an operator "Always allow" on a read prompt (`allow_always`) now
persists. Walk: answer `allow-always` on `Read session <id> outside this workspace or profile` from
an always-on agent, then confirm `compozy tool approvals list --workspace <actor-ws>` shows one
grant with `ACCESS SCOPE read:workspace:<target-ws>@<profile>` for that agent and tool, and Web
Settings → Tool approvals labels it `Reads in project <target-ws>`. A later read of another session
in the same target project — including from a new session of the same agent or after a daemon
restart — does not prompt. A different tool, target project, profile, or agent, and any write,
still prompts; writes still offer `allow_session`. Revoke the grant and confirm the next read
prompts again. An always-on agent answering another session's prompt with `allow-always` leaves no
grant. A missing session id raises the same read prompt and leaves no grant.

Review follow-up (#1270): repeat the remembered-read walk with the actor in User scope. List and
revoke with `--workspace @user` and Settings while focused on User; project-owned grants must not
appear in that list. Confirm concrete target profiles are distinguishable in the grant rows.
After each successful remembered read, confirm a `workspace.access_granted` event with source
`tool_consent` follows the initial policy miss without authorizing another seam.

Review follow-up: a profile-switch read prompt identifies both the selected profile and its
target workspace (or `@user`) in the title and raw input before consent can be remembered.
Cancel the originating request while its permission is pending, approve it, and confirm the
final consent audit is retained alongside the durable grant.

Audit review follow-up: consent recording applies its own bounded facts-lookup deadline after
originating-request cancellation. A read approved with session consent also emits the final
`tool_consent` granted event; a rejected cached consent emits no granted event.

Fallback review follow-up (#1270): a resolved read without a durable agent identity offers
`allow_session` and reuses that temporary answer. An unresolved session retains the opaque read
prompt. User-target profile reads say `in User scope`. The User-owned read integration journey
creates consent over hosted MCP, compares the `@user` HTTP list with the native list omitting its
workspace selector, replays without prompting, and revokes through `@user` before a new rejection.
