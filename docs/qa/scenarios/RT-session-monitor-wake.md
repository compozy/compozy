---
id: RT-session-monitor-wake
area: RT
title: A daemon-owned monitor wakes an idle agent session when a background command finishes
persona: Ada
journey: J-13
expected: An agent that runs a long command through `compozy__terminal_exec` (`still_running`) and registers `compozy__monitor_create` on the returned terminal can end its turn; when the process exits (or its output matches, or a timer elapses) the daemon starts a new turn in the same session with a `monitor` synthetic wake (`[COMPOZY MONITOR - NOT USER INPUT]`, monitor id, status, untrusted output tail), the Web transcript shows a "Monitor fired · <description> · <outcome>" divider before the agent's report, a busy session runs the wake after its current turn, a full input queue retries the wake (`delivering`), stopping the session cancels its monitors, and Claude sessions never expose the provider-internal Monitor/ScheduleWakeup/Cron tools, refuse Bash and PowerShell `run_in_background` with guidance toward `compozy__monitor_create`, and keep background Agent subagents. While an idle session has an active or delivering monitor its canonical badge is `waiting`, `waiting_on` is `monitor`, and its list row shows the amber `monitoring` mark instead of `done`, and the composer shows a monitor button whose popover lists each monitor and cancels it without waking the session — for sessions in every profile, including from the All profiles lens (#956). A `kind: "pull_request"` monitor reads the forge, stays armed across coalesced wakes, and ends on merge, close, timeout, or cancel.
entry_points: native tools compozy__monitor_create / compozy__monitor_list / compozy__monitor_cancel (toolset compozy__sessions); compozy__terminal_exec; Web session transcript, session list row, composer monitor popover; HTTP/UDS `GET|POST /api/workspaces/{ws}/sessions/{id}/monitors[/{monitor_id}/cancel]` (list takes `profile` or `all_profiles=true`, cancel takes `profile`); claude-agent-acp session/new + session/load `_meta.claudeCode.options`
qa_status: partial
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: 2026-10-20 isolated lab (branch feat/session-monitor, real claude-agent-acp + codex-acp) — step 1 pass (Claude: terminal_exec still_running → monitor_create → turn ended; wake 45s later with `exited with code 0` and `build-finished-7731`, agent reported unprompted), step 3 pass (Codex: wake `mon-d31de57c72c5`, reply `codex-done-5521`), step 4 timer leg pass (15s timer → `timer fired 4410`), step 7 pass (Claude reports no Monitor/ScheduleWakeup/CronCreate and no Bash run_in_background). Remaining live: step 2 Web row (covered by session-thread.test.tsx), step 4 match leg, steps 5–6 (covered by internal/monitor and daemon waker unit suites). 2026-10-21 isolated lab (branch feat/session-monitor-visibility, real claude-agent-acp sonnet) — step 8 pass: agent registered `mon-57506a4b5f2f` (900s timer); catalog row `badge=done` + `monitor_summary.active=1` rendered `data-badge="monitoring"` (amber radar), composer showed "Monitors, 1 active", popover read "QA: check CI on PR #875 · Timer · fires in 11m"; × cancel returned the monitor `canceled` over HTTP, removed the composer button, and the row left `monitoring` with no wake turn. 2026-10-24 isolated lab (branch fix/issue-956-monitor-popover-404, acpmock builder, profiles `default` + `marketing`, All projects + All profiles lens) — step 9 pass: before the fix the unscoped list returned 404 for the marketing session only (200 for default); with the fix the popover read `GET …/monitors?all_profiles=true` → 200 and listed "Check CI on the release PR · Timer · fires in 56m"; × sent `POST …/cancel?profile=marketing` → 200, the composer button disappeared, and the row left `Monitoring`; the default-profile session passed the same walk with `?profile=default`. 2026-11-02 isolated lab (branch feat/pull-request-monitor, real daemon, acpmock agent calling the native tool, wakes in `docs/qa/reports/artifacts/2026-11-02-pull-request-monitor/wakes.txt`) — step 13 pass except the Web walk, which the composer and transcript suites own: real GitHub read-only on compozy/compozy#1152 through the bundled forge-github woke on 3 failures of 49 checks, woke again naming only the one newly failing check, and stayed silent on a new head; a lab forge extension over the real extension protocol covered reuse, default-branch refusal, a failed lookup honoring its retry-after, one coalesced review wake 2 minutes after the burst was read, a late review on a superseded head ignored, a restart with changes pending woke at boot, and merge ended the monitor `fired`.; docs/qa/reports/artifacts/2026-11-03-stays-on-hardening-qa/ (2026-11-03 always-on QA walk)
last_report: docs/qa/reports/2026-11-03-stays-on-hardening-qa.md
overlaps: RT-020; ET-agent-terminal-window-materialization
---

story: As an operator I ask an agent to run something slow and report back, and the report arrives without me nudging the session.

1. **Terminal exit wake (Claude).** In an isolated lab, start a Claude session and prompt: "Run `sleep 20 && echo done-xyz` and tell me the output when it finishes." Expect the agent to call `compozy__terminal_exec` (result `still_running` with `terminal_id`), then `compozy__monitor_create` with `kind: "terminal"`, and end its turn. About 20s later, without any operator input, a new turn starts; `compozy session events <id>` shows a synthetic prompt whose metadata carries `kind: "monitor"`, `reason: "monitor_exit"`, and the monitor id; the agent's reply mentions `done-xyz`.
2. **Web row.** Open the session in Web: the wake renders as one divider "Monitor fired · <description> · Process exited" (the `[COMPOZY MONITOR…]` text is never shown) followed by the agent's turn.
3. **Codex parity.** Repeat step 1 on a Codex session; the same wake arrives.
4. **Timer and match.** `compozy__monitor_create {kind:"timer", delay_seconds: 10, description: …}` wakes after ~10s with "Timer elapsed"; a `until:"match"` monitor on a terminal that prints the pattern wakes with "Output matched".
5. **Busy and stopped sessions.** Fire a monitor while the session runs a turn: the wake queues and runs after it. Stop the session before the monitor fires: no turn starts, and `compozy__monitor_list` from a resumed session shows no monitor from before the stop.
6. **Cancel and limits.** `compozy__monitor_cancel` on an active monitor returns `canceled` and no wake ever arrives; a ninth active monitor in one session, or a fifth on one terminal, returns `tool_conflict`.
7. **Claude provider tools.** In a Claude session, Monitor, ScheduleWakeup, and CronCreate/CronDelete/CronList are absent; a Bash or PowerShell call with `run_in_background: true` is denied with guidance to use `compozy__terminal_exec` + `compozy__monitor_create`; a background Agent subagent still runs and returns inside its turn.
8. **Operator visibility and cancel.** Register a 10-minute timer monitor and let the turn end. The session row's mark reads `monitoring` (amber radar), not `done`; the composer shows "Monitors, 1 active". Open the popover: the row shows the description and "Timer · fires in 10m". Click × : the row disappears, the button goes away, the list row returns to `done`, and no wake turn ever starts. `GET …/monitors` then reports the monitor as `canceled`.
9. **Other profile and All profiles lens (#956).** With two profiles, give a session in the non-default profile an active monitor. Switch to All projects and the All profiles lens, open that session, and open the composer monitor popover: it lists the monitor instead of "Couldn't load monitors.". The list request carries `all_profiles=true`. Click × : the cancel request carries `profile=<owner>`, returns 200, and the button goes away. Without either selector the HTTP list still returns 404 for that session, and `all_profiles=true` on cancel returns 400. If the session itself is gone, the popover reads "This session is no longer available." and offers no retry.
10. **Shell-wait redirect (#1122).** In a Claude session, ask for `gh run watch <id>`, `timeout 570 gh run watch <id>`, a `for … sleep 20` polling loop, and `/bin/sleep 6`. Each Bash call is denied before it runs and the reason names `compozy__terminal_exec` with the exact `command` and `args` plus `compozy__monitor_create` (`kind: "timer"` for the delay). `gh pr checks <n> --watch` is denied with a `compozy__monitor_create` `kind: "pull_request"` call instead. `sleep 1`, `git log`, `gh pr checks <n>`, and `git commit -m "use gh run watch"` run. The redirected path works end to end: `terminal_exec` returns `still_running`, `monitor_create` registers on it, the turn ends, and a `monitor` wake arrives on exit. A session whose agent denies `compozy__monitor_create` or `compozy__terminal_exec` keeps a working `sleep 6`.
11. **Long external wait and idempotent registration (#1128).** Start a terminal whose command polls an external condition for minutes (`terminal exec -- sh -c 'until [ -s <marker> ]; do sleep 2; done; echo done'`), have the agent register `compozy__monitor_create` on it twice with the same `terminal_id`, `until`, and `pattern`, and let the turn end. Expect one `active` monitor (the second call reports it already watches the condition), an idle session, and, once the condition flips and the process exits, exactly one `monitor` wake whose status line reads `exited with code 0`, followed by the agent's report. `monitor_list` then shows one `fired` monitor. A different `until` or `pattern` on the same terminal registers a separate monitor.

12. **Durable monitors across a daemon restart (#1128).** In an isolated lab, register a 150 s timer and a terminal
    monitor (`sleep 3000`) from one session, and a timer from a second session that the operator then stops with
    `session stop`; request a daemon restart (`POST /api/settings/actions/restart`). Expect: the first session is
    resumed, its terminal monitor fires `terminal_unavailable` (the command is never rerun) and its timer fires on
    its original deadline, one `monitor` wake each; the operator-stopped session's monitor is `canceled`
    (`session_stopped`), the session is not resumed and nothing wakes it. With `session.restart.auto_continue=false`
    the monitors of a restart-interrupted session read `interrupted` with reason `owner_disabled` in
    `monitor_list` and over HTTP, and no wake or resume ever happens. Saved evidence:
    `docs/qa/reports/artifacts/2026-11-01-issue-1128/`.

13. **Pull request monitor.** In an isolated lab with a forge that serves one open pull request on the session's
    branch, have the agent call `compozy__monitor_create {kind:"pull_request", description:…}` on that branch. Expect
    one `active` monitor with `branch` set; a second identical call reports it already watches the branch; a call on a
    default branch is refused with `tool_invalid_input`. On a branch whose only pull request had already merged, the
    monitor stays `active` with no wake until a new pull request opens on it. With checks pending, no wake arrives. When a check fails, a
    `monitor` wake arrives within about a minute (metadata reason `monitor_pull_request_changed`, message id
    `monitor:<id>:1`), naming the head and the failed check inside the `untrusted="true"` block, and the monitor returns
    to `active` with `fire_count: 1`. When a new review arrives, a second wake (`monitor:<id>:2`) follows after 2 minutes
    of quiet and names the author and state, with no review text. A push wakes nothing by itself, and a late review or
    thread on the old head wakes nothing either. With the input queue full, the wake keeps its id past 15 minutes and is
    delivered even if the check recovered meanwhile, and later changes arrive in the next wake. A daemon restart keeps
    the monitor armed with its count, and a change during the downtime still wakes. A forge read that fails only for
    reviews keeps checks flowing and turns no existing review into a baseline. Merging fires a final wake and
    `monitor_list` shows `fired` with reason `pull_request_merged`. The Web composer popover shows "Pull request on
    <branch>", and the transcript divider reads "Monitor fired · <description> · Pull request changed".

14. **Waiting on other sessions.** Start two top-level sessions with slow prompts and, from a third session, call
    `compozy__monitor_create` with `kind: "session"`, both ids in `target_session_ids`, and `until: "all"`, then let
    its turn end. Expect the owner to stay `waiting` (`active_monitor` signal) past the first target settling, and
    exactly one `monitor` wake once both settle, with `reason: "monitor_condition_met"`, both targets listed, and the
    Web divider reading "Sessions settled". Repeat with `until: "any"` (wakes on the first). Stop one target
    mid-run: it counts as settled. Restart the daemon while the monitor is active: the restart itself fires nothing,
    and the monitor fires once after the boot when the targets have settled. Target the caller or a session from
    another workspace: refused, no monitor is created. The composer popover reads "All of 2 sessions".

    Evidence: `docs/qa/reports/2026-11-02-session-monitor-kind.md` (acpmock lab). Web divider, popover, and workspace
    refusal are covered by their automated suites.

15. **Wake burst during one long turn (#1154).** On a provider-free mock with `queue_cap = 2`, fire more monitors than
    the cap during one turn. Expect `compozy__session_inputs_list` to succeed with one synthetic row reporting
    `coalesced_wakes`, Web to show one "N monitors fired" row without `full`, and operator `queue` input to be accepted.
    Past `queue_promote_after` a provider with injection receives the row in the live turn; a provider without it
    receives it at the next boundary. The agent sees every outcome once, each monitor ends `fired`, and a restart
    between firing and delivery adds no duplicate turn and loses no outcome.

Step 15 evidence 2026-11-02 (branch fix/monitor-wake-queue, acpmock, `queue_cap = 2`, `queue_promote_after = "10s"`): six
timers fired during one held turn and produced one row with `coalesced_wakes: 6`; two operator `queue` inputs were
accepted and the third hit `queue_full` on the operator pool. Without injection, a daemon restart kept all rows and the
next boundaries ran operator, operator, then one wake carrying six outcomes. With injection, the row ended
`steer_delivery: injected` while the turn stayed running. All six monitors ended `fired`. Web showed "2 queued" without
`full` and a "6 monitors fired" row.

Sessions Kanban regression (pending integrated re-walk): open a monitor immediately before the turn
settles, without waiting for a supervision sweep. Read the session over CLI and HTTP: expect
`badge=waiting`, `waiting_on=monitor`, and an `active_monitor` work signal. No finished delivery may
appear. `session wait --until idle` must return. Cancel the last monitor without prompting: the next
supervision pass publishes exactly one `waiting → done` edge for unseen work. Repeat after marking
the settled turn seen: closure yields `idle`, without a finished delivery. A failed signal inspection
must yield `needs-attention`, never `done`.

QA impact 2026-10-30 (issue #1065): the runtime envelope, the `compozy__monitor_create` and
`compozy__subagent_delegate` descriptions, and the official skill references no longer tell agents to
end the turn unconditionally after registering a monitor. They end it only when every remaining step
waits on a registered monitor or subagent, and otherwise keep working until the requested outcome is
delivered. Owning automated evidence: `TestComposedAssemblerRuntimeEnvelopeEndTurnGuidance`.

QA impact 2026-10-31 (issue #1122): the runtime envelope covers every wait on external work, inside or
outside the turn; a shell-wait guard refuses foreground waits with the exact replacement call when the
session can call `compozy__terminal_exec` and `compozy__monitor_create`; both tools are always loaded on
Claude. Owning automated evidence: `internal/shellwait` tables, `TestSessionCanRedirectShellWaits`, and
step 10.

QA impact 2026-10-31 (issue #1128): `compozy__monitor_create` returns the already-active monitor when a
session registers the same terminal condition (terminal, `until`, `pattern`) again, so a retried call adds no
second wake; the descriptor, official skill, and site docs state that remote state such as CI is watched by
running its bounded watch command in a terminal and setting `timeout_seconds`. The daemon logs structured
`monitor.*` lifecycle events without descriptions, patterns, or output. Owning automated evidence:
`TestServiceCancelAndLimits` (reuse and per-terminal cap) and step 11.

QA impact 2026-11-01 (issue #1128, durability): monitors are stored (`session_monitors`, migration `00162`) and
recovered at boot. Timers keep their deadline, fired-but-undelivered wakes are re-delivered once under the stable
message id, a vanished terminal fires `terminal_unavailable`, an owner stopped on purpose cancels its monitors,
and a restart-interrupted owner is readmitted only through the restart-continuation eligibility rules, otherwise
the monitor is `interrupted` (visible, no wake). Owning automated evidence: `TestServiceRecoversDurableMonitorsAfterRestart`,
`TestServiceRecoveryHonorsOwnerAuthority`, `TestServiceDurabilityOfDecisions`, `TestResumeMonitorOwner`,
`TestGlobalDBSessionMonitors`, and step 12.

QA impact 2026-11-02 (`pull_request` monitor): a third monitor kind reads the forge through `PullRequestService`, stays
armed across wakes on one durable row (migration `00175`), and wakes under `monitor:<id>:<seq>`. Terminal and timer
monitors keep their exact behavior and message id. A bare `gh pr checks --watch` now redirects to this kind; naming a pull request or repository keeps the terminal redirect. Owning automated
evidence: `TestDetectPullRequest`, `TestServicePullRequestMonitor`, `TestServicePullRequestMonitorRestart`,
`TestProviderGraphQL`, `TestMonitorPullRequestObservation`, `TestGlobalDBSessionMonitorKindMigration`, and step 13.
A wake that the queue refuses keeps its durable fire and id; a pull request first seen ended is history; thread
wakes follow the head through each thread's opening commit; a failed review read differs from a forge without reviews.

B3 idle-release regression (pending integrated re-walk): with a provider supporting
`session/load`, leave only a daemon-owned monitor active and pass `idle_release_after`.
Expect the session to remain active with runtime unbound, without cancelling the monitor.
On its wake, verify the same ACP session is loaded. Repeat for each monitor kind.
Without session/load, or with another work signal, expect the runtime to remain bound.

B3 collector and retry regression (pending integrated re-walk): retry the same
timer request within one second and verify the same monitor ID, `reused`, and
one wake. For every monitor kind, a delivered wake must leave the heartbeat
fingerprint unchanged, including when its retained history expires. Interrupted
and undelivered wakes remain eligible facts for `monitors_fired`.

B3 next-wake projection (pending integrated re-walk): list instances with a
timer earlier than the next check and verify timer kind, timestamp, and
description. Without a scheduled check, an untimed terminal monitor must still
appear without `at`. Finish a working turn and confirm `now` becomes empty.

B3 doctrine regression (pending integrated re-walk): inspect startup guidance
for an instance main and its child. Both receive always-on doctrine; only the main
is told to start Goals elsewhere. An ordinary session and an Orchestrator
conductor without instance ancestry must not receive this section.

B3 post-compaction regression (pending integrated re-walk): after a provider
reports completed compaction, send the next prompt and verify a context notice
and full situation sections. A subsequent prompt returns to unchanged markers.
Failed compactions must not produce the completion notice.

B3 Goal regression (pending integrated re-walk): set and replace on a current
instance main return `goal_on_instance_main` through each public caller. A Goal
started before main binding keeps status/pause/stop, and blocks rotation until
it ends. A created independent session may still start a Goal.

B3 exhaustion regression (pending integrated re-walk): high occupancy alone
must not rotate. A real context refusal records one exhaustion report and marks
the current main for rotation. Hold a monitor or queued input and verify a
rotation-deferred report names the blocker. Resolve it; a later reconcile uses
the existing brief and publishes a successor, clearing the marker.

B3 backend evidence (2026-11-03, branch soh/b3-waits-context):
TestMonitorIdleReleaseIntegration passed with the actual timer waking a released
runtime and loading the same ACP session. TestDaemonNativeCompaction passed with
notice/full-section and subsequent-no-notice assertions.
TestInstanceContextExhaustionIntegration passed with a real provider refusal,
queued-input blocker report and successor publication after that input clears.
TestInstanceNativeCreateJourney passed set/replace refusal through command, HTTP
and native registry. The unit inventory and validation limits are recorded in
`docs/qa/stays-on-hardening-b3-progress.md`; integrated Web walks remain pending.

### Always-on pause and restart

Pause an instance, stop its main session, and send an agent message. Confirm that it
is queued and the provider stays stopped. Restart the daemon: the unattended count
and queue entry remain unchanged, and queue selection returns no autonomous row.
Resume the instance and confirm one delivery in the same main. Repeat the resume
nudge; no second prompt appears. A stopped descendant keeps its queue until it is
started. Archive cancels held input; quiet windows alone do not hold monitor wakes.

Backend evidence: `TestInstancePausedRuntimeIntegration` exercises the stopped-main
path with a real ACP process, durable SQLite and daemon restart. The queue suite
covers descendants through two levels and a rotated predecessor, human priority,
prepared Goals, batch accounting, independent capacity pools and archive. UI walks
remain owned by the frontend delivery.

QA verification 2026-11-03 (always-on hardening B3, soh/qa 583415ef5, isolated acpmock lab): Partial. Pause and restart: a timer wake fired while the instance was paused and was held, the held rows and `unattended_turns` survived a daemon restart, and an operator resume delivered once. Next wake: a 900-second timer earlier than the next check projected `next_wake {kind: timer, at, description: "Re-check the staging deploy"}`; a 3600-second timer later than the check projected `kind: check`. Doctrine: an always-on main's startup sections include `always_on_doctrine`, and an ordinary on-demand session's do not (`doctrine-sections.txt`). Quiet window (UT-028 receipt): with HEARTBEAT `quiet_windows` 05:00–07:30 UTC, triage-desk's timer fired at 06:02:47 as a `[COMPOZY MONITOR - NOT USER INPUT]` turn with `held_wakes: 0` (`ut028-triage-events.txt`). Not walked: idle release, collector retry, post-compaction notice, Goal refusal, context exhaustion, and descendant queues.

QA impact 2026-11-03 (#1199 vocabulary, v0.5.1): instance mains and their children receive the `always_on_doctrine` startup section, wrapped in `<compozy-always-on-doctrine>`. Not walked; S4 walks it on the integration head.

### Pull request merge conflicts

Register a `pull_request` monitor for an open PR. Introduce a base conflict and
verify one wake names the PR, base, and rebase action while the monitor stays armed.
Repeated conflicting reads must not wake again. Unknown/null mergeability and
forge failures must neither wake nor clear the remembered conflict, including
after daemon restart. Clear the conflict with a known mergeable observation, then
introduce another conflict and verify a second wake. A changed head alone does not
clear the conflict. While another wake is held for admission, observe a conflict
then push a new head: the undelivered conflict must survive. Repeat with a known
clearance before admission: omit stale rebase guidance while preserving any other
queued check/review changes. A known clearance without a head must also cancel the
queued conflict without resetting known check results. A same-head retarget updates
the named base; a push with no base retains the last known base. REST retargets must
name the base from the detail read, and closed or mismatched-head details must not
introduce a conflict.
Merge queue removal is not currently observable independently.

Backend coverage: `TestDetectPullRequest` and `TestServicePullRequestMonitorRestart`
exercise transition detection, durable recovery and wake rendering. `TestMonitorIdleReleaseIntegration` verifies conflict wake admission through real
SQLite and an ACP subprocess. Live GitHub conflict/restart walk remains unverified.

### Definition pauses and dispatched children

Create an always-on main with a dispatched child and grandchild. Queue their own
monitor wakes and a peer message. Remove the creator's definition, producing
`definition_missing`: children deliver once, while the main retains its inputs.
Repeat after rotation and with `definition_not_always_on`. Already queued work is
released by instance reconciliation; stopped descendants stay stopped. Idle-release a
child using the removed definition before releasing its queued work: the accepted
runtime reloads on the same route and both inputs are delivered. Repeat Pi startup
finalization after definition removal: accepted guidance remains byte-identical
across reloads rather than accumulating prior startup sections. A new session or
runtime switch still requires an available definition. Repeat with a
prepared Goal ahead of those inputs: normal child continuations still deliver,
while the Goal retains its explicit pause guard. Explicitly pause the definition-paused
instance manually: the reason becomes `manual`, and later child messages and wakes
stay held. Definition reconciliation must not weaken that manual brake.

Repeat with `manual`, `fatigue`, `unattended` and an unknown pause reason: all
autonomous lineage input stays held. Read prompt results and input lists for
`held_reason` and `held_instance_id`, then session status for the `held_input`
attention signal and Needs attention badge. A human prompt still runs. Resume and
confirm the hold explanation and attention clear. Under a definition pause, lower
the unattended limit and let a child reach it: the pause becomes `unattended`,
and later child wakes stay held. Adoption preserves child lineage and does not
implicitly transfer it to another instance. Archive an instance paused manually or for either definition reason, then
queue later child monitor and peer input: the receipt, queue and attention say `instance_archived`,
and the input remains fenced rather than suggesting a resumable pause. A later
human message still runs; no autonomous input is released behind it.

Backend verification 2026-11-06 (#1328): passed the owning paused-input queue,
coalesced wake, prompt-admission and unattended-brake tests; the prepared Goal
lifecycle integration rejected claims for both definition reasons on descendants
and retained queued rows without dispatch tokens. The HTTP/ACP child journey
verified held attention, manual takeover, idle release, definition removal and
queued wake/message delivery without resuming the creator. Browser and live-provider
walks are not claimed.

15. **Claude provider-native subagent refusal (#1439).** In an isolated lab using the candidate binary,
    ask a Claude background Agent subagent to register both a terminal monitor on its pending command
    and a timer monitor. Each call must be denied before MCP dispatch, explaining that monitors wake
    the parent, and advising return of pending work and terminal ID to the parent or a separate
    CompozyOS child session. The parent monitor list must contain neither refused registration and
    receive no wake from either. A foreground shell wait in the subagent must give the same handback
    guidance, and the background-shell refusal must explain the subagent exception. Have the parent
    register the terminal monitor and a timer itself: both register normally and wake the parent;
    the parent can then resume its subagent. Repeat after session load/resume. Keep background Agent
    execution enabled. On Windows, repeat through PowerShell with COMPOZY_BIN pointing to the candidate.

    Candidate verification (2026-11-10): the built CLI and POSIX PreToolUse wrapper refused terminal
    and timer calls carrying a child `agent_id` and emitted no decision for the same parent calls.
    The ACP start/load/resume contract and owning-session wake suite passed. A live-model background
    Agent walk and the Windows runtime walk remain unrun; CI owns E2E under this assignment.
