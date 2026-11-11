---
id: MS-web-stays-on-placement
area: MS
title: Always-on agents sit as avatars above sessions and in Kanban headers
persona: Dora
journey: J-31
expected: With no instance there is no row and no empty space. Global instances sit in a row labeled Global above all projects; each project's instances sit under its name, and in its Kanban lane header. Anyone needing the operator is ordered first and shows the Needs you triangle. A click opens the main session, created again if it was lost, and the open one shows a selected background. Hover or focus opens a card with what it's doing, last and next check, Open conversation and Pause or Resume. The context menu offers Pause or Resume with the pause reason, Open agent, Check-ins and Archive; Resume is disabled when the definition is gone. More agents than fit move into `+N`, a searchable panel grouped by state. A collapsed project shows who needs you by face. Main sessions leave session rows, cards and counts. Sessions they spawn stay ordinary rows and cards; the card carries a creator badge, while the row leaves it to the session itself (#1191). A project with only an agent still gets its group. A project's `+` offers New session, New folder and Add always-on agent; Add opens a picker of always-on agents not yet there, with Create new opening New agent on Always-on with that project chosen. The list toolbar's `+` offers Add always-on agent in every grouping: with one scope it adds there; when the list spans several projects it first asks where the agent lives (Global or a project), with the focused or last-used project preselected. At the limit the rows and Create new are blocked and the popover shows it. Flat mode shows one row: Global first, then the focused project's. The dock's Home app reads Dashboard.
entry_points: web Sessions window (list and Kanban); project `+`; list toolbar `+`; avatar row; avatar context menu
qa_status: pass
bug_ids: BUG-20261103-resume-all-rate-limited
fix_status: fixed
retest_status: verified
fix_commits: f7a8bbf3d, 4a6fef558, 2e2487068, 11e90692f, 2f917de93, 9d9147a9f, 10183640d, 301b10320, f5d9906aa, 1300ed91f, 1a9b61852, 7fc9e2389, 6b13d0dec, e53e4593a, 11042786b, b280406e0, a64b1c981, 4056dbcc2
evidence: docs/qa/reports/2026-11-01-stays-on-agents.md; .compozy/tasks/always-on-agent/qa/pr-evidence.md; docs/qa/reports/artifacts/2026-11-03-stays-on-hardening-qa/ (2026-11-03 always-on QA walk)
last_report: docs/qa/reports/2026-11-03-vocabulary-alignment-qa.md
overlaps: MS-web-agent-create-simple-advanced
---

Needs the instance routes from the always-on backend slices. Create an always-on agent in a project and
one Global, then walk the row, its card and menu, the picker, Kanban and the collapsed signal. Pause
and resume from the menu, archive and confirm the session returns to history, and watch
`agent_instance.updated` move a pose without a reload.

2026-11-01 (D-5) provider boundary evidence: isolated live ACP triage completed
with Claude `plan`, Codex `read-only`, and OpenCode's config-only `plan`. Each
returned `NOTHING_TO_REPORT`; Claude also refused a requested scratch-file write.
See `../reports/2026-11-01-always-on-b4-remediation.md` for unavailable providers,
scope limits, and exact evidence. This does not mark the Web journey walked.

QA impact 2026-11-01 (always-on F1): built against the `_dx.md` contract with typed adapters and mocks,
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

B1 contract remediation: configure `agents.always_on.max_active_instances = 7` and
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

Instance permission level: configure one always-on agent with `permissions: approve-all` and one with
`approve-reads`, with `[permissions].mode = "approve-all"`. Create both instances, then prompt a turn
that requests an edit. The first is allowed with no prompt, the second waits for the operator. A third
always-on agent with no `permissions` waits for the operator too. A session created under the `approve-reads` instance with no level of its own runs at `approve-all` and
keeps the instance's tool, skill, MCP server, and path ceilings. Hidden triage stays read-only.
`TestInstancePermissionMode` and `TestTriageSessionPermissions` own the level rules; the same walk ran
through a real daemon with a scripted ACP provider.

Always-on CompozyOS tool prompts (#1200): create the built-in Orchestrator instance (it ships
`permissions: auto`) and an always-on agent pinned to `approve-edits`, both on Claude, with
`[permissions].mode = "approve-all"`. Ask each to call `monitor_create`, `worktree_create` and
`session_create`, then `worktree_remove` on the worktree it made. Count `permission` events in
`compozy session events <main>`: the three coordination calls produce none, and `worktree_remove`
produces exactly one pending prompt, which runs once approved. Repeat on Codex (`workspace-write`).
`TestHostedToolPermissionGates`, `TestCoordinationPermissionLevels` and
`TestDaemonNativeRuntimePolicyResolver` own the gate rules.

QA verification 2026-11-03 (#1200, isolated lab, live Claude and Codex): Pass. On main the built-in
Orchestrator asked 7 times for `skill_view` x4, `monitor_create`, `worktree_create` and
`session_create`. On the fix it asked 0 times for those and once for `worktree_remove`; the
`approve-edits` agent matched. Codex at `workspace-write` runs non-destructive MCP calls without
asking and asks for destructive ones by tool call id only; the fix leaves one prompt for
`worktree_remove`. Not walked here: the Web placement journey above.

B1 resume authority: race an own-instance agent resume against an operator pause,
then against a fatigue pause. Resume must read and authorize the current pause
reason inside the same write transaction as the cap check and state transition.
The agent receives `agent_instance_forbidden` and the pause remains; an operator
can still resume it. `TestInstanceResumeAuthority` injects each pause after the
service's initial read and verifies both denial and operator recovery with SQLite.

Managed Goal pause recovery (#1315): start two child Goals, pause one directly,
then pause their always-on instance. Maintenance must retain ownership of the other
Goal's pause while its provider turn drains and until its coordinator parks the run.
Resume the instance: only its own Goal resumes; the directly paused Goal stays paused.
Repeat an immediate instance pause/resume while a Goal is draining. Managed Goal turns
do not increment the unattended counter; child lifecycle notifications are separate
autonomous inputs. Check that counter after resumed work starts and before another resume
resets it. An interrupted operation already finalized as ambiguous must not be queued again
when its pause is resumed; the Goal must still accept stop after the repeated pause/resume.
`TestInstanceManagedGoalPauseIntegration` owns the real daemon/ACP
walk, and `TestGoalTurnRuntimeLifecycleIntegration/Should_retain_an_instance_pause_until_the_coordinator_parks_the_run`
forces maintenance into the interval between Goal turn settlement and coordinator pause.

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

2026-11-01 walk: row, hover card, context menu, picker, cap, Kanban header and Dashboard dock passed in an isolated lab. Still unwalked: `+N`, collapsed signal, flat-mode switching, the project `+` menu, live pose updates and keyboard paths.

Backend startup regression: `TestBootAgentInstances` uses real SQLite and a
registered project whose folder is absent. Starting instance services must keep
the API available, retain the manually paused instance, and log its reconciliation
error. A missing project must not prevent access to other projects.

Spend-brake configuration coverage (stays-on-hardening B1): set
`agents.always_on.unattended_turn_limit` to `2` and restart the daemon.
Two autonomous turns pause the instance with reason `unattended`; the next input
remains queued. Repeat with an explicit `0`: autonomous turns continue. A
negative value must fail configuration validation. The default is `300`.
Configuration and explicit-zero merge coverage lives in
`TestAlwaysOnTriageConfiguration`; real SQLite trip and disabled-brake coverage
lives in `TestGlobalDBInstanceUnattendedBrake`. The combined product walk remains
pending until pause side effects and public read fields are integrated.

Pause cancellation coverage: while a read-only check is running, pause the
instance. The check must stop and its occurrence must finish as `cancelled`,
without increasing failure streak or adding a failure report. A working turn
already running must finish. Backend coverage: `TestInstancePauseCancelsCheck`
uses real SQLite for manual, fatigue, unattended and definition pause reasons;
`TestInstanceUnattendedPauseDispatch` checks the queue-to-post-commit handoff and
the selected turn reaching the provider boundary. Goal/Loop coordination and
the combined public-surface walk remain pending.

Check counter coverage: reported and failed occurrences increment
`unattended_turns` once at finalization. Quiet and cancelled occurrences do not.
Replay the same finalized occurrence and verify that it adds neither a count
nor another pause effect. At the limit, the pause returned to the runtime must
be the snapshot committed by that finalization. Canonical SQLite coverage:
`TestInstanceCheckUnattendedCounter` and
`TestInstanceCheckUnattendedPauseEffects`.

Always-on hardening F1 web (S1, S4, S7): a brake pause reads "Paused after N steps
without you." in the hover card and context menu, and "Took N steps without you" on the
agent's places. The hover card shows "Wakes up next" with the wake's description or kind
and its time, and for a paused agent how many things wait for resume. With two or more
agents paused for `unattended` in one scope, the `+N` crew panel offers Resume all,
which resumes only those, one at a time; one such pause offers none. When the
`agents.always_on.max_active_instances` cap refuses some, the panel names them with the
daemon's reason ("Ops Lead and Release Watch didn't resume. You have 10 active always-on
agents. …"); a refused resume leaves the agent paused with its counter unchanged. The agent's What it cost reads
the token count with "No price available" when usage has no price, never "≈ $0.00" and
never a plan claim, since the data carries no billing basis.
Web suites: `always-on-row.test.tsx`, `agent-instance.test.ts`, `agent-team.test.ts`
and `agent-profile-home.test.tsx`.

QA verification 2026-11-03 (always-on hardening B1/F1, soh/qa 583415ef5, isolated acpmock lab): Fail on S4. Passing legs: with `unattended_turn_limit = 3`, a 5-second monitor-timer chain paused `ops-lead` with reason `unattended` after 3 steps, and the next wake was held (`held_wakes: 1`). A human prompt on a paused main ran, kept the pause and reset the counter to 0. `-1` was rejected ("must be nonnegative") with the file unchanged. An explicit `0`, after a restart, let 5 autonomous turns run with the agent active. A reported event check raised `unattended_turns` by 1. CLI and HTTP instance reads match on all 26 fields (`it008-*.json`). The hover card read "Paused after 3 steps without you. · 2 things are waiting for you to resume" (`01b-s1-hover-paused.png`) and "Wakes up next · Re-check the staging deploy, in 15m" (`07-s7-hover-next-wake.png`). What it cost read "154K tokens · No price available" (`11-s11-usage.png`; paper cut: project name clipped with no ellipsis). Fail: the +N panel showed Resume all with two `unattended` agents, but pressing it sent parallel resumes, all but one got 429, no error appeared and the rest stayed paused (`04-s4-resume-all.png`, BUG-20261103-resume-all-rate-limited). Paper cut: a paused Release Watch was ordered under Paused in the overflow rather than Needs you.

Re-walk 2026-11-03 on feat/stays-on-hardening 4056dbcc2 (S4): Pass. BUG-20261103-resume-all-rate-limited is verified fixed. With `max_active_instances = 7`, 6 active and 3 agents paused for `unattended`, Resume all resumed Alpha Watch first. The other two requests, each sent after the previous one finished, returned 429. The panel read "Beta Watch and Ops Lead didn't resume. You have 7 active always-on agents. Pause one or raise `agents.always_on.max_active_instances`." (`https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/stays-on-hardening/4056dbcc2/04-s4-resume-all.png`). With `max_active_instances = 20`, all three resumed in sequence (200, 180–214 ms; `https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/stays-on-hardening/4056dbcc2/04b-s4-resume-all-room-for-all.png`, `rewalk-s4-resume-requests.txt`). Paper cuts: the open panel keeps a resumed agent under Needs you and its count until it refetches, and the config key renders with literal backticks.

Issue #1181 (toolbar `+`): in a grouped All projects list, the toolbar `+` shows New session,
New folder and Add always-on agent. Add opens a place list (Global, then Projects) with the
preselected place checked; picking a project opens that project's agent picker, and adding
creates the instance there. A list bound to one project still adds there without asking.
Web suites: `session-list.test.tsx` and `always-on-add-menu.test.tsx`.

## Open agent from All profiles (#1183)

With All profiles selected, open an agent from the avatar menu (Open agent and Heartbeat) for
instances owned by a nondefault profile: one Global, one in a project, with definitions that exist
only in that profile's layer. The agent window must show the agent every time, never "Agent not
found", and every agent read in it must carry the instance's profile. Reload the page with the
window open and confirm it still resolves there. Web suites: `always-on-row.test.tsx`,
`agent-window-location.test.ts`, `use-profile-selection.test.tsx`.

Verified 2026-11-03 on fix/issue-1183-open-agent-all-profiles 1dd4c5745 (isolated targeted lab,
profiles default/open-design/ops, remembered profile default): Pass. Before the fix, the release
bundle read `ops_lead` and `ops_global` with `profile=default` and showed "Agent not found" on 6 of 6
opens (`https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-1183/1dd4c5745/01-before-Ops-Lead.png`).
With the fix, 60 of 60 opens across three agents and two runs resolved with `profile=ops`
(`https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-1183/1dd4c5745/02-final-Ops-Lead.png`), and
Check-ins survived 5 reloads (`https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-1183/1dd4c5745/04-after-reload.png`).

QA impact 2026-11-03 (#1199 vocabulary, v0.5.1): the row, its aria label ("Always-on agents"), the topbar pill and the add action read Always-on ("Add always-on agent"); the avatar menu item reads Heartbeat; capacity uses `agents.always_on.max_active_instances`; the v0.5.0 table name still applies through v0.5.1 with a daemon warning. Not walked; S4 walks it on the integration head.

QA impact 2026-11-05 (#1303 always-on main chrome): opening an always-on agent's main session
shows the agent identity, the Always-on pill, the agent's `…` menu and the window controls in the head.
The head no longer shows the "N linked" chip, Stop, or the sessions and context sidebar toggles; a panel
left open keeps its toggle only as the control that closes it. The transcript has no right-edge message trail, and the composer has no context meter (the daemon
rotates the main session when its context runs out). An ordinary session keeps all of these. The gate is
the instance's `main_session_id`, never the agent name. Verified on the Storybook route stories
`AgentSession/AlwaysOnMain` and `AgentSession/LinkedWorkWithTrail` in headless Chromium, by DOM probe
and screenshot before and after the change. Not walked in a live lab.

QA impact 2026-11-11 (#1487): the sessions list toggle moved to the start of every session head, before the project name, and stays there while the list is collapsed; before, an always-on main lost the toggle once the list closed and could not reopen it from the head. The main's right side now holds only the Always-on pill and the agent's `…` menu, with no divider before them; a context sidebar left open still adds its close toggle. Walked in an isolated lab (branch daemon and web bundle, acpmock provider) at 1600px and 1280px: the list toggle opened and closed the list in both states, ⌘B still toggled it, and the base bundle reproduced the missing toggle.
