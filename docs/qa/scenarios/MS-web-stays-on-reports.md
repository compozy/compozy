---
id: MS-web-stays-on-reports
area: MS
title: A stays-on agent's reports, relayed approvals and supersede states read in its main session
persona: Dora
journey: J-31
expected: A report lands in the transcript without starting a turn. Picking a proposal sends one ordinary prompt and folds to "You picked". After an absence with enough check-ins, a While you were away card tells the night and only what needs you stays a card. Asks from child sessions dock on the main session with who asked and where, and answering settles the child's request. Expired, provider-ended and interrupted runs each say what happened and offer one ordinary prompt. A superseded session is read-only and links to the current one.
entry_points: web Sessions window; main session opened from the avatar row
qa_status: blocked-verify
bug_ids:
fix_status: fixed
retest_status: pending
fix_commits: f7a8bbf3d, 4a6fef558, 2e2487068, 11e90692f, 2f917de93, 9d9147a9f, 10183640d, 301b10320, f5d9906aa, 1300ed91f, 1a9b61852, 7fc9e2389, 6b13d0dec, e53e4593a, 11042786b, b280406e0, a64b1c981
evidence: docs/qa/reports/2026-11-01-stays-on-agents.md; .compozy/tasks/always-on-agent/qa/pr-evidence.md
last_report: docs/qa/reports/2026-11-01-stays-on-agents.md
overlaps: MS-web-stays-on-placement, ET-native-tool-approval-grants
---

Create a stays-on agent in a project, let a child session ask for permission, and answer it from the
main session. Then shorten `agents.stays_on.approval_timeout` and let one expire. Restart the daemon
during a held request to see the interrupted report.

QA impact 2026-11-01 (stays-on F3): proposals need the proactive slice, away reads need
`GET /api/agent-instances/{id}/away`, and the superseded state needs `instance_id` on the session
payload. Until each lands, those paths fall back to no digest and no read-only state. Focused suites
own the states. Not walked.

QA impact 2026-11-01 (stays-on B7 reports): publish more than twenty reports, then send a
human prompt. Verify that at most twenty published reports are delivered as fenced data and
that retrying the same message identity neither calls the provider nor delivers another batch.
Agent prompts and steering must not consume reports. Cancel, replace, clear, promote to steer,
or stop a queued prompt and verify that its reserved reports return to pending. A failed start
before provider dispatch also releases them. A provider dispatch with an unknown outcome leaves
reports indeterminate, never resends them, and adds a transcript note; repeat daemon recovery
and verify that the note is not duplicated. Check that report text containing a closing fence
cannot escape the data block.

Focused backend evidence: `TestInstanceReportPromptReservation`, `TestInstanceReportsReachProvider`
and `TestPromptReportFence` pass with the race detector. These checks cover SQLite transactions
and the provider I/O boundary; the Web journey remains unwalked until the B7 UI integration.

Rotation admission slice: after a main-session pointer changes, a new prompt or
steer against its predecessor must return HTTP 409 with `code=session_superseded`.
An exact retry of an earlier admitted command retains its existing receipt. Verify
that the current main still accepts new prompts and that ordinary sessions remain
unaffected. `TestInstanceSupersededAdmission` exercises the pointer fence with real
SQLite; the complete rotation and browser journey remain pending.

Rotation publication slice: verify the pointer changes only when the predecessor
has no live descendant, pending interaction, outstanding reply watch, queued input,
or unresolved admission. A prompt admitted after the initial eligibility snapshot
must prevent publication. A failed publication must roll back successor lineage.
`TestInstanceRotationPublication` covers persisted CAS and rollback using SQLite.
Runtime monitor synchronization, successor startup and cleanup are still pending.

Rotation service slice: reconciliation now invokes the configured
`MainSessionRotator`. Cleanup precedes eligibility and rotation, including for
paused instances. A cleanup failure leaves rotation for a later tick. A decision
for a different predecessor is rejected. `TestMainSessionRotation` owns this
service sequencing invariant; the runtime adapter is not yet configured.

Rotation eligibility slice: the runtime adapter reads the same typed context
telemetry as Goal and applies the shared maintenance decision at the configured
threshold. Unknown usage, provider compaction, active turns, pending interactions,
active or delivering monitors, and persisted dependencies prevent rotation.
Completed monitor history does not block it. `TestInstanceRotationEligibility`
covers this decision; successor creation and atomic runtime publication are pending.

Rotation startup slice: only one live candidate may register beside the published
main. Candidate registration preserves the pointer; a stopped candidate allows a
retry. `TestInstanceRotationCandidate` verifies this with SQLite. The daemon now
has a synchronous successor starter carrying runtime selection and fenced context,
and a cleanup method that retries noncurrent live sessions from persisted lineage.
The starter and cleanup are not yet wired into a complete rotation invocation.

Rotation runtime guards: monitor publication holds the monitor creation lock and
retires the predecessor only after the publication callback succeeds. Session
publication holds lifecycle and prompt-setup locks, refuses active work, and closes
new prompt setup after success. Failed callbacks leave admission open. The durable
pointer remains authoritative across restart; these guards serialize the live
publication boundary. Tests: `TestSessionRotation` and
`TestInstanceRotationAdmissionGuard`. The orchestration callback is not yet wired.

Rotation orchestration slice: the daemon now configures `MainSessionRotator` for
reconciliation, using `agents.stays_on.rotation_threshold` (default 0.8). It starts
a successor with bounded recent conversation context, publishes under monitor and
session admission locks plus the SQLite CAS, notifies the catalog, then stops the
predecessor. Pre-publication failures stop the candidate and append
`rotation_failed`; post-publication cleanup failures retain the successor for retry.
`TestInstanceRotationFlow` covers these ownership boundaries. Pointer-based late
child re-entry, the one-hour deferral report and an isolated runtime walk remain pending.

Rotation deferral slice: a persisted wait begins when context requires rotation but
dependencies prevent it. After one hour, append one `rotation_deferred` report per
main session; repeated ticks and restart must not duplicate it. Clear the wait when
rotation becomes eligible or context no longer requires it. The additive global
migration preserves existing data. `TestInstanceRotationDeferral` verifies the
one-hour boundary, deduplication and reopen against SQLite. Late child re-entry and
the full browser/runtime walk remain pending.

Web remediation: a relayed ask stays docked until the daemon accepts the answer, and only then
shows its receipt; force a failed answer and confirm the ask remains. Leave a main session after
reading fewer than six check-ins, then come back: nothing already read returns as While you were
away. Any report kind in `indeterminate` shows the may-not-have-reached note, however old. Read it
and See check-ins navigate in place, in the instance's own scope, without reloading the app.

Rotation catalog invalidation regression: after a successful main-session pointer
swap, subscribers must receive the instance catalog change before predecessor
cleanup. A cleanup failure must not suppress that change; a refused publication
must emit none. `TestInstanceRotationFlow` covers these daemon boundaries.
The browser re-walk remains part of the enclosing integration QA.

Approval expiry attribution regression: a provider-ended permission request must
persist `approval_expired` with `expired_by: provider`; a deadline expiry must
persist `expired_by: timeout`. `TestPendingInteractionReportsAtomicity` exercises
both causes with real SQLite, transactional rollback, subprocess crashes, reopen,
and duplicate transitions. The report remains unique per interaction.

Native approval deadline regression: expiry of the bridge's deadline must emit a
terminal permission event attributed to `timeout`, persist `timed_out`, and enter
the transactional approval-expiry report path. Ordinary caller cancellation keeps
its existing attribution. ACP permission-boundary and session lifecycle suites
cover event attribution and durable status respectively.

ACP expiry ownership regression: timeout removes the approval from the actionable
set before emitting its terminal event. An operator answer that already won stays
authoritative; later answers fail. Without a provider rejection option, expiry
emits a canceled decision attributed to timeout so the persisted request closes.
The ACP permission-expiry suite covers both winner orders and missing rejection.

Report notification regression: republishing a pending proposal or fatigue report
on the successor must still append its transcript entry, without issuing a second
operator notification. Publication carries the report's existing publication
state internally; this does not add a field to the transcript payload.

Indeterminate-report recovery must continue past an unavailable session and write
notes for later recoverable reports, returning the individual failures to the
caller. The report-delivery suite injects a missing session before a valid report
and verifies the later durable note. After durable note persistence, recovery
acknowledges the report in the global catalog. Subsequent recovery scans exclude
acknowledged reports, including after restart; failed sessions remain retryable.
A crash before acknowledgement retries the idempotent event append without
duplicating the note.

Interruption attribution regression: boot expiry reports `daemon_restart`; recovery
of a lost provider turn during session resume reports `provider_interrupted`.
Approval expiry preserves the canonical tool ID when present, otherwise the
provider's request title, and falls back to `provider` only when both are absent.
The atomic pending-report suite verifies these payloads across crash and reopen.

Instance maintenance runs in a separate owned periodic worker from session
supervision. A slow reconciliation or successor startup must not delay process
reconciliation or inactivity supervision. Shutdown cancels and joins instance
maintenance; the daemon instance suite exercises a blocked reconciliation through
that lifecycle. The existing periodic-worker cadence and no-overlap rules apply.

Terminal-report retention runs in bounded maintenance batches. Pending and
reserved reports never expire. Delivered and indeterminate reports remain for
at least max(agents.heartbeat.wake_event_retention, 48h), measured from delivery
when present, otherwise creation. Rows exactly on the cutoff remain until a later
sweep. The real SQLite report suite covers both configured and minimum retention.

Superseded prompt admission checks the current instance pointer before restarting
a stopped predecessor, for both default and explicit queue modes. The rejection
retains `session_superseded` and identifies the current main in its error message.
Previously completed idempotent receipts remain replayable; transaction admission
rechecks the pointer to fence a concurrent rotation.

Rotation retry regression: indeterminate admissions are terminal uncertainty and
must not permanently block rotation; reserved and dispatch-committed work still
blocks it. Failed candidates share a report key per predecessor. Retry waits for
agents.heartbeat.default_interval after each failure, using a process-local clock
fence; restarting the daemon may retry immediately, while persisted report
source-key deduplication survives restart. No new configuration key is introduced.

Provider context refusal regression: an error event explicitly identifying an
exhausted context window or an overlong prompt writes one instance failure report
per turn, reason `context_exhausted`. Repeated error events deduplicate by turn.
Deadline and generic provider errors do not enter this classification. The report
uses fixed descriptive text, avoiding propagation of raw provider error content.

2026-11-01 walk: report entry, proposal pick, While you were away, relayed approval and its failed answer passed, and reports reach `delivered` on the next human prompt. Still unwalked: expired, provider-ended and interrupted approvals, rotation and the superseded state.

Cadence remediation: configure a 5-minute minimum and 30-minute default, then author
10 minutes in HEARTBEAT.md. Enabling checks must schedule ten minutes ahead; a
failed timer check must back off from ten minutes, not from the default. The real
SQLite case in `TestInstanceChecksToggle` passes with `-race`; the browser walk
remains pending with this scenario.

Direct prompt recovery: fail the first runtime bind after a published report is
reserved. Verify that the report returns to pending immediately, then restore the
provider and retry both with the original key and with a new key. Each successful
prompt delivers at most twenty reports, once. `TestInstanceReportsReachProvider`
now covers both retry identities using the session harness and real SQLite.

Heartbeat configuration: set `max_interval` and `max_concurrent_triage` under
`[agents.heartbeat]`, restart, and verify unchanged-digest timing and concurrent
triage admission use those values. The old stays_on spellings were unreleased
and are removed. `max_interval` must be at least `quiet_backoff_cap`.

Triage model selection: configure a model under `[agents.heartbeat.triage_models]`
for the agent's provider, then run a proactive check and inspect the hidden session's
runtime model. The main session retains its model. Remove the provider entry to
restore the agent's default. `TestTriageDispatcher` exercises the TOML-to-launch path.

Fatigue resume: accumulate quiet backoff and enough failed checks to trigger fatigue,
then resume as operator. Both quiet and failure streaks reset atomically with resume;
one subsequent failure must leave the instance active. `TestInstanceCheckFatigue`
exercises the full wake/admission/finalization/resume sequence against SQLite.

Default busy preference: leave busy mode at `steer` and send an ordinary prompt to
an idle main session. It must receive pending reports, capped at twenty. If a direct
admission races into actual steering, all its reports return to pending in the same
transaction as steer staging. The session and SQLite report suites cover both paths.

Quiet output contract: return `NOTHING_TO_REPORT` for silence. An empty proposals
array, fenced or unfenced, must finalize as `triage_output_invalid`, with a failure
report rather than incrementing quiet streaks. A valid fenced proposal remains
accepted. Parser and outcome suites cover these distinctions.

Quiet triage recovery: interrupt the daemon after quiet triage deletes its session
but before the occurrence finishes. Restart and verify boot succeeds, the held
slot is released, and the occurrence records `triage_interrupted` once. A missing
triage session is already stopped; other stop failures remain errors.

First automatic check: create or adopt an instance with standing instructions and
verify the next scheduler tick schedules its first check for creation time plus
the resolved policy interval, without calling the model. Verify the check runs
when that interval elapses; an explicit manual check remains available immediately.

Session health context: retain stopped and archived sessions alongside an active
session without a health row. Verify only the active session contributes unknown
health to the proactive digest. Hidden triage sessions must remain excluded.

Triage data boundary: include triple backticks in a digest key and in a prior
report body. Verify both are JSON unicode escapes in the provider prompt and
cannot close the untrusted-data fence. Quiet triage still completes normally.

Global automation context: create a failed user-scope automation with no workspace
and a failed workspace automation. A Global instance without cross-workspace reach
must include only the user-scope failure in its proactive digest.

Structured configuration: set the rotation threshold, active-instance cap, proposal
cap, triage timeout, maximum interval and triage concurrency through config tools.
Verify the values persist and validate with their declared types. With no timeout
override, a proactive triage turn has a ten-minute deadline.

Report delivery exits: cancel queued input through a task force-run mutation and
verify its reserved reports return to pending atomically. After an asynchronous
provider error, verify delivered reports become indeterminate with one transcript
note. Automatic prompt recovery must not resend those uncertain reports.

Away quiet count: complete more than one page of quiet checks, along with a failed
check and a quiet check belonging to another instance. Verify Away counts only
this instance's quiet events. After marking seen, exclude events at or before the
seen timestamp; later quiet checks remain visible. Reading Away stays passive.

Triage denial audit: attempt writes, terminal creation and unknown native calls
from a read-only triage turn. Verify each refusal produces a permission event and
its count reaches the occurrence's `denied_calls`. Quiet-session deletion and
failed dispatch must preserve this durable count.

Approval expiry activity: let an ACP approval time out and explicitly decline a
second approval. Activity must classify the first as No answer and the second as
declined, with separate daily counts; the permission log records `expired` only
for the timeout resolution.

Human prompt backoff: accumulate quiet checks, then admit a new human prompt on
the instance's main session. Verify quiet streak resets while the failure streak
is preserved. Agent prompts and replaying the same admission must not reset it.

Triage proposal bounds: accept titles through 120 characters and bodies through
500 characters, including multibyte text. Longer retained proposals produce
`triage_output_invalid`. Entries beyond `max_proposals` are discarded even when
their fields are malformed; they must not invalidate accepted proposals.
Focused parser coverage lives in `TestParseTriageReply`; the live provider walk
remains part of the trailing integration QA.

Triage initiator audit: a timer check records initiator kind `daemon` and the
instance ID on its hidden session. Verify triage denials appear in unattended
permission activity before archived-session retention removes that session.

Triage lifecycle retention: a failed provider start leaves no session row or
directory. On daemon recovery, stop and archive each interrupted hidden triage
before finalizing its occurrence. After `wake_event_retention`, supervision deletes
archived triage while retaining recent triage and ordinary archived sessions.
The owning Manager regression cases are in `TestTriageDispatcher`; include restart
and retention in the trailing live-provider QA walk.

Provider command delivery: with pending reports, send an advertised provider
command such as `/compact keep decisions` in queue mode. The provider receives
exactly that command and the reports remain pending. A subsequent normal prompt
receives the reports once; retrying its admission does not resend them.

Agent activity quiet totals: complete more than one page of quiet checks on one
day and quiet checks on a day without reports. The activity chart includes both
days, preserves Told and Needed you totals, and excludes other profiles, agents,
workspaces and checks outside the selected time window. `TestInstanceActivity`
owns the SQLite aggregate regression; verify the chart in trailing integration QA.

Preflight retry fairness: fail policy resolution, the before-check hook, or digest
collection on a due timer check. Verify one diagnostic check-in is recorded and
the next attempt is scheduled after the cadence without changing the completed
digest or automation cursor. Other due instances must progress with batch size 1;
manual checks and dry runs must preserve the existing timer deadline.

Skipped-check backoff: accumulate quiet or failed checks, then skip a timer check
with an empty or unchanged digest, a canceled hook, or a preparation error. Verify
the next timer keeps the existing backoff and applicable cap, plus its normal
stagger. Skips must preserve streaks, the completed digest and automation cursor.

Finalized timer cadence: configure a policy minimum longer than the quiet cap or
maximum backoff. Quiet, reported and failed outcomes must never schedule earlier
than that minimum. Finalization applies the same instance-derived stagger as
admission (up to 10% of cadence), with a stable offset across retries. Manual
checks and replayed finalizations preserve the existing timer deadline.

Hook cancellation detail: return `{"cancel":true,"reason":"maintenance_window"}`
from `agent.heartbeat.wake.before`. Persisted timer wake events retain `hook_cancelled` with
`maintenance_window` in `detail`; no triage starts. Reasons are redacted and bounded
to 300 bytes. Omit `reason` and verify cancellation still applies.

Recent triage reports: seed more than 20 reports, including delivered reports and
reports with equal timestamps. A check receives the 20 newest reports for its
instance in descending `(created_at, id)` order. Newer reports from another
instance stay excluded. Public report history keeps its chronological pagination.

Triage cleanup failures: fail archiving after a completed proposal. The proposal
is still finalized as reported, and cleanup failure is logged. After recovery,
supervision retries stopping or archiving hidden triage and applies normal
retention. A primary dispatch failure remains an error even when cleanup fails.

Read-only triage: a provider `current_mode_update` selecting a writable or unknown
mode must cancel the active check with an error before applying the mode. Verify
with `TestTriagePolicy/Should_fail_the_active_triage_run` in the ACP suite.

Collector selection: omit HEARTBEAT.md `context` to use all five collectors;
set `context: {include: []}` to disable collectors. Save, reload and roll back
the policy; the explicit empty selection must survive. The owning checks are
`TestCollectDigest` and
`TestManagedHeartbeatAuthoringServicePutValidateAndCAS/Should_preserve_disabled_collectors`.
