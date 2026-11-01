---
id: MS-web-stays-on-placement
area: MS
title: Stays-on agents sit as avatars above sessions and in Kanban headers
persona: Dora
journey: J-31
expected: With no instance there is no row and no empty space. Global instances sit in a row labeled Global above all projects; each project's instances sit under its name, and in its Kanban lane header. Anyone needing the operator is ordered first and shows the Needs you triangle. A click opens the main session, created again if it was lost, and the open one shows a selected background. Hover or focus opens a card with what it's doing, last and next check, Open conversation and Pause or Resume. The context menu offers Pause or Resume with the pause reason, Open agent, Check-ins and Archive; Resume is disabled when the definition is gone. More agents than fit move into `+N`, a searchable panel grouped by state. A collapsed project shows who needs you by face. Main sessions leave session rows, cards and counts. Sessions they spawn stay ordinary rows and cards with a creator badge. A project with only an agent still gets its group. A project's `+` offers New session, New folder and Add stays-on agent; Add opens a picker of stays-on agents not yet there, with Create new opening New agent on Stays on with that project chosen. At the limit the rows and Create new are blocked and the popover shows it. Flat mode shows one row: Global first, then the focused project's. The dock's Home app reads Dashboard.
entry_points: web Sessions window (list and Kanban); project `+`; avatar row; avatar context menu
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-web-agent-create-simple-advanced
---

Needs the instance routes from the stays-on backend slices. Create a stays-on agent in a project and
one Global, then walk the row, its card and menu, the picker, Kanban and the collapsed signal. Pause
and resume from the menu, archive and confirm the session returns to history, and watch
`agent_instance.updated` move a pose without a reload.

2026-11-01 (D-5) provider boundary evidence: isolated live ACP triage completed
with Claude `plan`, Codex `read-only`, and OpenCode's config-only `plan`. Each
returned `NOTHING_TO_REPORT`; Claude also refused a requested scratch-file write.
See `../reports/2026-11-01-always-on-b4-remediation.md` for unavailable providers,
scope limits, and exact evidence. This does not mark the Web journey walked.

QA impact 2026-11-01 (stays-on F1): built against the `_dx.md` contract with typed adapters and mocks,
before those routes merged. Focused suites own the states. Not walked.

Proactive core follow-up (2026-11-01): use CLI `agent instance check --dry-run`, then a real
check with `--no-wait` and read its occurrence. Confirm one run when timer and manual check
race, and unchanged timer cadence for an early manual check. Disable checks, restart, and
verify no wake; enable and verify one-cadence scheduling. A quiet check leaves no hidden
session; a proposal produces one report and notification without a main-session turn.
Ten consecutive failures pause with `fatigue`. Keep the existing manual QA status until
these controls and notification delivery have been walked in an isolated lab.
Automated evidence owners: `TestInstanceManualCheckRace`, `TestInstanceWakeMigration`,
`TestInstanceCheckFatigue`, `TestTriageDispatcher`, `TestInstanceReportPublication`, and
`TestAgentInstanceCommands`.

B1 contract remediation: configure `agents.stays_on.max_active_instances = 7` and
`agents.heartbeat.fatigue_limit = 4`. Create two instances in the same profile and
filter the instance list to one agent. The envelope must still report
`active_count: 2` and `max_active_instances: 7`. GET, PATCH, native reads and
`agent_instance.updated` must include the persisted `checks_enabled`, `last_wake_at`,
`next_wake_at`, `quiet_streak`, `failure_streak`, and configured `fatigue_limit`.
Turn checks off and refetch; both responses must show false. Empty wake timestamps
are empty strings and counters are zero before the first check. The focused
`TestAgentInstanceCheckProjection` exercises shared HTTP/UDS handlers against real
SQLite; browser/native/SSE scenario execution remains pending.

B1 grant admission: give a depth-zero agent and its definition the unrestricted
grant, then create its own Global and workspace instances through an approved
native call. Both instances must persist an empty grant ceiling. Repeat as the
operator: only the Global instance retains unrestricted. Permission intersections
remain enforced. `TestInstanceCreateAdmission` covers all four admission outcomes;
the native runtime walk remains pending.

B1 resume authority: race an own-instance agent resume against an operator pause,
then against a fatigue pause. Resume must read and authorize the current pause
reason inside the same write transaction as the cap check and state transition.
The agent receives `agent_instance_forbidden` and the pause remains; an operator
can still resume it. `TestInstanceResumeAuthority` injects each pause after the
service's initial read and verifies both denial and operator recovery with SQLite.

B1 approval admission: with authenticated agent credentials, POST create and DELETE
archive under approve-reads and deny-all must return `403 tool_approval_required`
with the corresponding native tool hint and leave persisted instances untouched.
Operator and approve-all callers proceed through normal definition/ownership checks.
Native create/archive/real-check must accept approve-all without a per-call receipt,
while explicit approval remains required for other agent modes. The API regression
uses real SQLite; native tests retain ownership denial after mode-based admission.

B1 archived workspace errors: attempting instance work in an archived workspace
must return HTTP/UDS 409 with code `workspace_archived` and the existing recovery
message. `TestAgentInstanceErrorResponses` owns this error mapping.

D-3 create result: first creation returns HTTP 201 and `instance.created: true`.
Repeating the same agent/profile/scope returns HTTP 200, `instance.created: false`,
and the same instance/main IDs even at the active cap. The native create tool
returns the same boolean. `TestAgentInstanceCreateResult` verifies both HTTP/UDS
responses and exactly one main session registration against SQLite.

N-21 paused checks: after pausing an instance, a real check must return HTTP/UDS
409 `agent_instance_paused`; the native call returns the same domain error. A dry
run remains available and must not mutate wake state or dispatch triage.
`TestInstanceCheckGates` owns the paused/dry-run behavior with SQLite; the existing
API error-response suite owns the 409 mapping.

D-6 workspace directories: creating an instance in a workspace with configured
additional directories preserves the root and every resolved additional directory
in its permission ceiling. `TestInstanceDefinitionPaths` resolves a persisted
workspace to verify that admission does not drop the extra paths.

D-4 root policy: start an ordinary root session with enabled skills, MCP servers
and additional workspace directories, then adopt/restart it as an instance. Its
persisted policy must retain those capabilities and launch paths. Delegated and
instance ceilings remain restrictive, including empty categories. The Session
Manager lineage suite creates a session, reads persisted policy, and resumes it
under that ceiling; provider process I/O uses the canonical test driver.

N-44 stream authority: an agent with an aggregate workspace grant still receives
`agent_instance.updated` only for its own instance. Foreign-instance events are
omitted; the operator retains all in-scope events. The canonical session catalog
stream suite exercises both identities against stored instances.

N-51 instance reads: with more instances than one page, filter by agent/state and
verify the first matching page is populated even when matching IDs lie beyond the
first unfiltered page. HTTP/UDS and native reads apply filters before the SQL
limit. Payload enrichment reads the main session by ID and one attention aggregate;
pending interactions in any retained instance session's descendant tree and pending
failure reports preserve attention without loading the entire session catalog.
The existing instance isolation/lineage suites own these SQLite invariants.

B1 N-49 contract verification: create/adopt, pause/resume, archive, checks and ensure-main
CLI output identifies the agent label and scope using the approved DX text. A completed
reported check prints its proposal count and persisted delivery state. HTTP errors retain
stable codes and actionable messages, including the configured active limit. Lifecycle
hooks expose `reason` for pauses. Existing CLI HTTP fixtures, API error and hook dispatch
suites cover these contracts; this does not claim a provider or browser walkthrough.

B1 D-24: hidden triage sessions must be absent from workspace/session projections and
extension session lists. Supplying their IDs to public HTTP/UDS, native tools, or
extension status/prompt/stop endpoints returns not found; internal triage execution
continues through the Session Manager. The existing SQLite workspace-read, handler,
native dispatch and extension host suites verify these boundaries.

D-4 follow-up: explicit root resource grants and persisted resource grants, including empty categories,
remain authoritative across startup/resume. Fresh roots without a supplied policy materialize
the complete resolved policy. Existing spawn inheritance and permission-refusal suites pass
without changing their expected grants.

B1 ASTRA-010/D-9: in approve-reads with no approval channel, own-instance pause,
ensure-main, checks-off and dry-run checks remain callable. Real checks, create and
archive still require approval. A pre-call hook changing dry-run to real or checks-off
to checks-on must trigger the stricter decision before execution. Deny-all and explicit
tool restrictions remain effective; foreign-instance access remains denied by native
authority. The registry dispatch suite owns the approval matrix and hook escalation
cases; the native instance suite verifies ownership after registry admission.

D-4 regression follow-up: root native-tool materialization also applies to fork/continue
roots with no tool budget yet. Resource grants remain preserved. Directory-only skill
entries use the existing workspace API identity rule (directory basename), including
instance ceiling filtering. Existing derive and extension host integration suites retain
their original assertions; the root policy suite additionally covers a directory-only skill.

2026-11-01 (always-on-agent D-23) QA impact: remove the filesystem root of a
registered workspace containing an instance, then read instance lists and session
facets. The unavailable definition uses its agent name/default appearance and the
registered workspace name. Other instances, session activity, and pending report
attention remain readable. Owning daemon integration: `TestInstancePayloadIntegration`.
The current browser scenario verdict remains unchanged pending its integration walk.

2026-11-01 (always-on-agent N-53) QA impact: when main recovery encounters a
stopped creation attempt, retain its archived history and create a new candidate.
Never publish that stopped session as the new main. Owning integration:
`TestInstancePayloadIntegration/Should_replace_a_stopped_main_attempt_instead_of_republishing_it`.

2026-11-01 (ASTRA-006, N-45, N-46) QA impact: archive serializes against main
creation/recovery and stops the main or registered candidate before committing
archive state. A failed stop leaves the instance visible and retryable. Repeat
archive after stop recovery and verify the instance leaves the active catalog.
Owning SQLite coverage: `TestInstanceServiceRecreation`.
