---
id: ET-native-tool-approval-grants
area: ET
title: Set, remember, and revoke native-tool approval decisions
persona: Théo
journey: J-answer-agent-requests
expected: Allow-always and reject-always decisions survive daemon restart only for the exact workspace, agent, tool, and input digest; explicit agent-wide and tool-wide decisions set through Web, CLI, HTTP, UDS, and native tools survive restart without an input digest; every surface lists the same rows; revocation removes each decision everywhere and wider allows never exceed the configured tool-policy ceiling.
entry_points: Native-tool permission prompt; Web Settings / General; compozy tool approvals set/list/revoke; PUT/GET/DELETE /api/tool-approval-grants; compozy__tool_approvals_set/list/revoke
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-037; ET-038
---

Trigger native-tool approvals for allow and reject decisions from two agents and with distinct
inputs in one workspace. Restart the daemon and confirm only an exact workspace, agent, tool, and
input-digest match is reused without a second prompt. Set one agent-wide decision and one tool-wide
decision through each management surface, restart, and verify the wider keys contain no input
digest. Confirm another workspace sees no rows. Compare Web, CLI, HTTP, UDS, and native-tool list
output, revoke each row through a different management surface, then confirm the next matching
invocation prompts again. Under every tool mode, verify wider allows remain below the policy ceiling
and that one-shot approval tokens, and ACP subprocess permissions remain unchanged.

QA impact 2026-07-15: new durable workspace-scoped native-tool approval decisions, management
surfaces, and Web revoke flow. Planning flag only; no QA session ran in this implementation slice.

QA impact 2026-07-19: explicit agent-wide and tool-wide creation now spans native, CLI, HTTP, UDS,
and Web surfaces. Reset to `untested`; this is a planning flag, not a retest result.

Phase C planning 2026-07-19: linked to J-answer-agent-requests; settles US-001 (D1, ADR-001).

Forensic evidence contract (SD-006) — each item cites timestamp, exact command, observed output:

- Grant row before and after daemon restart via `compozy__tool_approvals_list`, CLI `-o json`, and HTTP
  parity (identical data).
- Explicit agent-wide and tool-wide set requests (native, CLI, HTTP, UDS, Web) with their persisted
  no-digest keys.
- Zero-prompt transcript for a matching call, a prompt for a non-matching agent, and the
  revoke → re-prompt transcript.
- A deny-all run proving a stored allow grant never overrides the mode ceiling, and a grant-store
  read-error run falling to the prompt (never auto-approve).

2026-08-13 Global runtime-binding re-walk: the focused daemon-served E2E created an allow-always
decision from a Global session, rendered the exact row in Web Settings / General through the hidden
operator-home runtime binding, revoked it, and confirmed the workspace-scoped HTTP list was empty.
`web/e2e/__tests__/tool-approval-grants.spec.ts` passed against a production Web build. The scenario
remains `blocked-verify` for its wider cross-surface and restart matrix, which this focused regression
walk did not claim to cover.

src: .compozy/tasks/hermes-comparison/_user_stories.md#us-001-durable-approval-grants-for-native-tools

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.

QA impact 2026-09-28: the Web rows read plain words (session normie pass). Tool ids render humanized
(`compozy__task_list` → "task list", raw id on hover); the decision pill reads "Allowed"/"Blocked";
scope reads "Only this exact request", "Every request from this agent", or "Every request, any agent";
the set action and dialog read "Add rule" / "Add a rule". The approval dock asks "Allow {agent} to …?"
with "Don't allow" / "Never allow". Contracts, test ids, and persisted data are unchanged.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

QA impact 2026-11-01 (B4 unattended approval waits): configure
`agents.stays_on.approval_timeout` (default `12h`, range `1m` through `72h`).
For a Stays on main and its child, trigger a native approval and verify the
request appears in pending interactions with the original request identity.
Approve through the existing session approval route. Repeat after adoption and
after archiving the instance while the process remains running: subsequent
requests must resolve current membership and use the applicable wait. Ordinary
sessions retain their existing native approval timeout and terminal policy.
This scenario remains pending execution; unit coverage alone is not a QA pass.

B4 deadline projection: pending permission interactions expose optional `expires_at`.
Verify that new ACP requests show their resolved approval deadline and that old
rows remain readable without a fabricated deadline after upgrading. Confirm the
same stored value in direct and descendant lists. Expiry/report atomicity and
native bridge deadline alignment require their remaining B4 checks.

B4 external initiators: sessions created by scheduled jobs and triggers retain
the automation definition ID. App-token requests retain the authenticated token
session ID without gaining an unrestricted grant. Agent-created sessions retain
the authenticated caller ID; descendants still inherit their root initiator.
Check attribution in permission audit reads for each origin, including a
cross-workspace agent creation without lineage. Manual QA remains pending.

B4 audit filter: the observe permission query's unattended filter includes only
`automation`, `daemon`, and `extension` initiators. Unknown historical rows,
operator, app, and agent-session initiators are excluded. Filtering precedes
the row limit and composes with session and decision filters. Public transport
and CLI exposure remain outstanding.

B4 CLI relay: run `compozy session pending <main-session-id> --descendants`
and compare its SESSION and EXPIRES AT columns with `-o json` fields
`session_id` and `expires_at`. The deadline is UTC; an absent deadline remains
absent in JSON and displays a dash in the table. Answer using the displayed
original session and provider request IDs.

B4 native relay: call `compozy__session_pending_interactions` with the main `session_id`,
its `workspace`, and `descendants: true`. Compare `interactions` with the CLI
JSON read, including original session/request IDs and optional `expires_at`.
A foreign workspace must fail before reading interactions.

B4 daily audit aggregate: group permission decisions by UTC day, agent and
decision across the complete filtered population. The service requires an
explicit profile read scope and a bounded half-open time interval; workspace
and unattended filters apply before grouping. Compare counts for human and
automation requests and confirm another profile's decisions are excluded.
HTTP and CLI expose this aggregate through the rollup endpoint and activity command below.

B4 permission rollup API: `GET /api/permissions/rollup?days=7&unattended=true`
returns `since`, `until`, and `rollups` grouped by UTC day, agent and decision.
Use `profile` or `all_profiles` for the profile lens and optional `agent` for
a single agent. Verify invalid days (outside 1–90) and invalid booleans fail,
and compare the totals with persisted decisions in the selected profile.
The endpoint is available over HTTP and UDS to operators and granted agents.

B4 activity CLI: `compozy observe activity --days 7 --unattended -o json`
reads the permission rollup endpoint; `--agent` narrows the agent and the
existing `--profile` / `--all-profiles` flags select the profile lens. Compare
the JSON envelope and table totals with the API. Days outside 1–90 must fail
before transport; omitting `--unattended` includes every initiator kind.

B4 provider expiry (IT-034): end a provider permission request before its
Compozy deadline. Its persisted interaction must read `status: timed_out`,
`resolution: expired-by-provider`, and `resolved_by: provider`, with no pending
approval badge. A canceled daemon-originated native approval remains
`status: canceled` and `resolved_by: system`. Neither accepts a later answer.

B4 persistence proof (IT-019/IT-020): the canonical
`TestPendingInteractionReportsAtomicity` suite injects report/transition failures,
then runs a subprocess that exits without database cleanup before and after the
transition. Reopening the real SQLite database preserves the pending request
before commit and exactly one expiry/interruption report after commit. Repeated
process exits do not duplicate reports; terminal requests reject a late answer.
`TestInstanceApprovalCrashRestartDaemonIntegration` additionally kills the daemon-hosting
process with one expired and one pending child interaction, boots the same isolated
home, and boots it again. It checks terminal decisions, the preserved expiry report,
one interruption report, and `already-resolved` responses retaining the original
decision. The original subagent continuation journey remains unchanged.

B4 surface verification: `TestSessionPendingDescendants`,
`TestNativeSessionPending`, and `TestSessionInteractionDescendants` cover the CLI,
native registry, and HTTP handlers. `TestObserveActivityCommand`,
`TestPermissionRollupClient`, and `TestPermissionRollupHandler` cover unattended
rollups. OpenAPI, generated Go operation parameters, and generated TypeScript
operation types expose `descendants` and `unattended`; the Go generated-operation
suite and the TypeScript client typecheck pass. IT-018 provider timing is covered by
`TestInstancePermissionBeyondDefaultTimeout` using eleven minutes of virtual time;
`TestInstanceNativeApprovalBeyondDefaultTimeout` covers the native bridge's same
wait. `TestSubagentPermissionDaemonIntegration` boots a real daemon and ACP child,
adopts its parent into an instance, reads the child request through the main
session's native descendants query, and answers through the main session's native
approval tool. It also opens a native approval in the same child through the real
permission manager, lists both requests, and answers the native request from the
main session before completing the provider approval.

The repository `make codegen-check` passed after retrying with cache access.
The test-shape checker reports existing cases outside the changed region in
`handlers_test.go`; both new timeout cases follow the canonical subtest shape.

N-23 canonical surfaces: invoke `compozy__session_pending_interactions` for pending
requests and `compozy agent draft "<goal>" --provider <provider>` to draft without
saving. The former unshipped names have no aliases. Native relay and CLI command
suites cover dispatch and the unchanged HTTP drafting boundary.
