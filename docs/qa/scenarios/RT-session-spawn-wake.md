---
id: RT-session-spawn-wake
area: RT
title: Wake a parent when its child settles
persona: Cora
journey: J-respond-to-agent-attention
expected: A governed child that stops, fails, enters a needs-you state, or completes its turn normally queues one sanitized synthetic turn on its live parent by default, never interrupts an active parent prompt, and explicit notify_creator false suppresses only that child's wake with an auditable reason; a canceled child turn delivers no completed wake while its catalog, wait-badge, and attention effects still fire.
entry_points: compozy spawn --no-notify-creator; POST /api/agent/spawn over HTTP and UDS; compozy__session_spawn; parent session transcript
qa_status: untested
bug_ids:
fix_status:
retest_status: pending
fix_commits:
evidence: docs/qa/reports/2026-08-16-herdr-parity.md; internal/session/manager_lifecycle_contract_test.go
last_report: docs/qa/reports/2026-08-24-eng-147-ttl-cleanup.md
overlaps: RT-session-wait-state; RT-session-done-presence
---

Spawn children through CLI, HTTP/UDS, and the native tool with omitted, true, and false
`notify_creator`. Exercise needs-input, failure, and stop while the parent is idle, prompting, and
stopped. Confirm one wake per cause episode, correct child identity and badge metadata, queued
delivery after a busy parent settles, redaction and the 240-character bound, plus suppression for
disabled notification, self-wake, dead parent, and failed delivery.

QA impact 2026-08-16: Task 04 added the default-on governed-child wake bridge and presence-aware
opt-out on every spawn surface. Flag only; task_08 owns execution.

QA impact 2026-08-24: ENG-147 changes the stopped-child wake classification for settled TTL
cleanup. The focused lifecycle contract verifies one clean stopped wake; a provider-backed
public-surface walk remains blocked pending an isolated ACP provider and human verification.

QA impact 2026-09-29: PR #685 adds `SpawnWakeReasonCompleted` for a governed child's normal turn
completion, so `BadgeDone` now maps to a delivered `completed` wake. When the just-finished turn was
canceled, that completed spawn-wake is suppressed, while the catalog event, wait-badge edge, and
attention hook still fire; no badge semantics changed. Plan only — the owning unit regressions are
the `BadgeDone` delivery case and the canceled-turn suppression case in
`internal/session/spawn_wake_test.go`.

Extend the walk with two turns of a governed child whose parent is idle:

- Let the child finish a normal turn (`done`/`end_turn`). Confirm exactly one `completed` parent wake
  with the child's identity and badge metadata, that it never interrupts an active parent prompt, and
  that the existing `NotifyCreator`/opt-out gate still governs delivery.
- Cancel the child's in-flight turn. Confirm the parent receives NO completed wake, while the
  catalog event, wait-badge edge, and attention hook for the canceled turn still occur. Confirm
  `BadgeDone` still computes for the canceled turn — only its wake delivery is suppressed.

Automated evidence: `spawn_wake_test.go` `BadgeDone` delivery case and the canceled-turn suppression
regression. A provider-backed public-surface walk remains blocked on an isolated ACP provider, as in
the earlier entries.

QA 2026-08-16 Herdr parity: The isolated browser journey, focused attention Playwright lane, and full Web E2E exercised cross-workspace landing, permission resolution, counts, channel suppression, task canary, catalog scope/order, finished presence clearing, and honest quiet/stale states. The lab browser exposed its real notification capability; deterministic granted and denied branches ran in the canonical browser suite.

### Provider routing regression (issue #564)

Use one isolated workspace with global native provider command A, creator command B, a commandless
child, and an explicit child command C. Check that the actual ACP processes launch B/B/C, including
when the creator definition changes after its process starts. Change the selected provider and
confirm its own command wins. Change the child home/environment policy and confirm inheritance does
not override that boundary. Resume the child with a live and a stopped creator.

Spawn acceptance is not proof of future prompt authentication. Inject a provider authentication
failure at the ACP boundary and inspect the durable error and provider failure marker. Its
`provider_command_fingerprint` must match the selected route; the logical session remains available
for a later recovery prompt. Raw commands and account-directory values must not appear in markers.

Owning automated evidence: `TestSpawnProviderCommandPrecedence`,
`TestManagerIntegrationSpawnProviderCommandRouting`, and
`TestPromptGenericFailureKeepsSessionActive`. The integration case executes real ACP subprocesses
with isolated routing wrappers and SQLite; the controlled routes are not live OAuth accounts.

Routing-slice verification: [2026-09-09 issue #564 report](../reports/2026-09-09-issue-564-provider-routing.md).
`TestSpawnProviderRouteDiagnostics` additionally verifies that a startup auth failure is stopped,
retained for inspection, and correlated with exactly one fingerprint per selected/error JSON log.
This slice does not replace the unrelated wake-delivery walkthroughs above.

QA impact 2026-10-19 — Add a pre-start hook that wraps its input command. Create an unbound
parent, bind it with its first prompt, and spawn a compatible commandless child. Both launches
must contain exactly one wrapper; both routing snapshots and accepted fingerprints must retain
the configured command and its origin, and rewritten launch options must be non-builtin. Repeat
with built-in OpenCode, only npm's `npx` available, and a hook matching only the parent agent:
the parent must launch its wrapper and the compatible child must use the npm fallback without
inheriting that wrapper. Both saved routes must remain built-in. Focused race-enabled
evidence: `TestAgentPreStartHookProviderRouting` passes alongside the launch-origin and spawn
command-precedence suites. The provider-backed public-surface walk remains pending.

QA impact 2026-10-09 — Subagents service slice: delegated children use `spawn_role=subagent`,
`notify_creator=false`, and the dedicated subagent wake reactor. Verify batched wakes, status
acknowledgement removing a queued result, canceled queued wakes returning to pending, and user/agent
interrupt disposing delivery without stopping children. Steer delivery must never cancel the parent
turn; unsupported or failed injection requeues once. Owning automated checks are
`TestSubagentLifecycle`, `TestSubagentLifecycleBoundaries`, and `TestSubagentPromptCancel` in
`internal/session`. Real SQLite and ACP subprocess coverage is in
`internal/daemon/subagent_integration_test.go`: cross-provider delegation and replay, queue priority
and acknowledgement pruning, steering/refusal fallback, bounded wait, permission waiting, recursive
completion, synchronous stop, interrupt disposal, provider launch failure, executed pre-create hook
denial, native Agent/Task rows, and boot-reactor reconciliation of missing inputs and orphan sessions.
The controller's final public-surface walkthrough remains the CLI/HTTP/Web verification owner. The
boot-reactor test models an observation gap in a running daemon; the killed-daemon restart is owned by
IT-012 in the same file, which kills the daemon-hosting process mid child turn and boots it again over
the same database.

QA impact 2026-10-09 (Subagents tools slice): delegated subagents use separate wake delivery,
`subagent.settled` observation, and a depth/live situation summary; existing spawn contracts remain.
Scoped evidence: `TestSubagentHookPayloads` and `TestSubagentSituation`. The lifecycle/service slice
and final spec QA own the integrated completion, acknowledgement, and wake-suppression walk.

QA impact 2026-10-21 (spawn orchestration): `compozy__session_spawn.prompt` (CLI `--prompt`,
HTTP/UDS `prompt`) sends the child's first prompt from the parent with `notify_on_complete`. While
the parent holds an open reply watch on the child, the bare `completed` spawn wake is suppressed
with `suppression_reason=reply_watch_covers`, and the reply wake delivers the child's final answer
instead. Needs-you, failure, and stop wakes are unchanged. A completed wake without a watch now
points at `compozy__session_history`. Walk: spawn with `prompt`, let the child finish, and confirm
exactly one parent wake that contains the answer text. Owning automated evidence: the
reply-watch suppression case in `internal/session/spawn_wake_test.go`.

Issue #933 backend acceptance: a reaper tick must ignore stopped history while
retaining live sessions not yet rebound after restart. Expired spawned/system
sessions, stopped parents, missing parents and subagent restart exceptions must
retain their existing stop/wake behavior. A parent lookup failure must abort the
sweep rather than classify that child as orphaned. Backend evidence and limits:
[session inventory performance](../reports/2026-10-23-session-inventory.md).

Release regression 2026-10-24: isolated managed delivery must expose the final
commit/push/PR facts after the delivery fence releases. Those facts come from the
forge observation, independently of the UI's PR display cache. The existing
TestIsolatedSubagentDeliveryIntegration passed locally with real Git and ACP,
using a stub only for external forge I/O (6.171s).

QA impact 2026-10-30 (issue #1065, early stop): a child with any parent lineage (spawned or
`provenance`) whose turn ends with `end_turn` while no open work signal (monitor, child session,
awaited reply, Loop run, task lease, scheduled wait), no pending interaction, no parent reply watch
on that turn, and no linked pull request remains now reads `needs-attention` with
`attention: "ended_early"` (board column Needs you, row reason "Stopped before delivering", card
"Stopped early") instead of `done`. The parent gets one `needs_attention` wake whose prompt carries
the child's final answer (bounded to 4,000 characters) even with `notify_creator: false`; normal
completions of opted-out children stay suppressed. The mark is durable attention state
(`sessions.ended_early_turn_id`), survives idle runtime release and catalog reads after restart, and
the child's next prompt clears it. Walk: create a child with `compozy__session_create` and a
parent, prompt it with a task it answers without delivering, and confirm the badge, the parent wake
text, and that a follow-up prompt clears the mark; repeat with a registered monitor, a delegated
child, a linked PR (including one opened during the turn on a newly created branch), a turn that
stops on the token limit, and a top-level session and confirm no flag; a transient pull request
lookup failure also leaves the turn unflagged. Owning automated evidence:
`TestManagerEarlyStopSupervision` in `internal/session/spawn_wake_test.go`, the early-stop cases in
`TestCanonicalBadgeAttentionPrecedence`, the idle-release case in
`TestSupervisionReleasesIdleRuntime`, and `TestSessionAttentionEndedEarlyMarkDrivesDurableCatalog`.

QA impact 2026-11-08: when acknowledgement clears a child's last question, configure the creator's
synchronous input.pre_submit hook to call child presence. Verify the attention edge arrives first,
the hook completes successfully, and the creator wake is delivered after the child's attention
lock is released. Session-layer SQLite and synchronous wake-boundary regression coverage lives in
TestManagerAttentionHookRunsAfterCanonicalCommitAndFailsOpen. CI owns the live daemon journey;
this change does not claim a new local E2E run.

QA impact 2026-11-09 (harness token efficiency): replay the same child's attention revision
after its first wake was admitted, including after daemon restart. The durable message identity
must prevent a second turn. A new revision still wakes the parent, and failure to read the ledger
must be reported rather than acknowledged as delivered. Monitor batching, reply-watch coverage
and informative child events keep their existing behavior.

QA impact 2026-11-09 (lifecycle baseline ordering): repeat the synchronous creator-to-child
presence callback for lifecycle publication as well as ordinary and reconciled interaction
publication. The callback must finish after the child publication lock is released. The canonical
attention hook suite owns this boundary check; completed-wake suppression for canceled turns
remains covered by `TestFinishPromptPumpSpawnWakeCancelSuppression`.

QA impact 2026-11-09 (native settlement callback): complete a parent turn with unfinished
provider-native work. Native interruption and parent attention settlement must finish before
lifecycle publication takes its attention lock. Verify the native row becomes interrupted and
the parent receives a durable settled revision without waiting for cancellation. The canonical
attention hook suite covers this with the real subagent service, SQLite store, and manager runtime.

QA impact 2026-11-09 (direct attention publication): delay clarification resolution publication
until after an early-stop dismissal. Require exactly one waiting-for-input-to-idle event/hook,
with no creator wake. Repeat with a first dismissal and delayed presence publication to ensure
the clearing edge is retained once. Reconcile a padded session ID together with its canonical
ID, retry it, then remove the session; require one edge and no retained baseline. Close worker
admission before a transient ordinary publication read failure and require bounded synchronous
recovery with one hook and one creator wake after unlock. The existing attention ordering and
hook suites verify these SQLite schedules; CI owns the full transport/extension journey.

QA impact 2026-11-10 (dismissal response read failure): let the initial Status and fenced
dismissal succeed, then fail the final Status read. Require the canonical SQLite projection
to be cleared and exactly one truthful clearing catalog event and attention hook before retry.
Repeat mark-seen and require no duplicate event/hook or creator wake, including a child with
notify_creator enabled and a return from Later. The existing Manager dismissal/ordering suite
executes these SQLite schedules with an injected store read failure. The response still reports
the read error; CI owns the broader transport/extension journey.

QA impact 2026-11-10 (native parent settlement): capture parent turn-end snapshots while a
permission keeps both badges at `waiting-for-auth`, resolve the permission, then publish the
delayed lifecycle snapshots. Unfinished native work must become interrupted exactly once;
canonical parent attention must settle and catalog/hook edges must agree without duplicate
permission-clearing edges. Owning SQLite/manager regression: the native-work cases in
`TestManagerAttentionHookRunsAfterCanonicalCommitAndFailsOpen`. Retain canceled-turn wake
suppression, unrelated-session progress, and shutdown lock cancellation. CI owns the full
public-surface walkthrough; focused daemon integration uses real ACP subprocesses and SQLite.

QA impact 2026-11-10 (delayed native cancellation): cancel turn N while a follow-up prompt
is queued, and delay N's native cleanup until N+1 records its background Agent calls and
early task observations. N's unfinished native rows become interrupted exactly once;
N+1 retains task-result stitching and completes on its own terminal evidence or turn-end
fallback. Repeating N's interruption must not affect N+1 or emit a completed wake for N.
The existing native service, native ACP daemon integration, and canceled-turn spawn-wake
suites own these checks; CI owns the full public-surface journey.

QA impact 2026-11-11 (native call turn identity): launch a background Agent call in turn A,
then deliver a non-task update for the same call with turn B or no turn. Interrupt B twice,
then finish A twice. The row must retain A, complete once through the background fallback,
and produce no delegated-child wake. Repeat with an empty persisted turn and a named update;
the empty identity remains authoritative. `TestSubagentNativeRecoveryEdges` owns this
deterministic service regression. The existing native daemon integration exercises real ACP
subprocesses and SQLite; CI owns the full transport journey. ET-042's hook contract and
attention lock ordering are unchanged; its native-settlement coverage remains applicable.
