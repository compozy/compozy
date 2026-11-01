---
id: MS-web-stays-on-reports
area: MS
title: A stays-on agent's reports, relayed approvals and supersede states read in its main session
persona: Dora
journey: J-31
expected: A report lands in the transcript without starting a turn. Picking a proposal sends one ordinary prompt and folds to "You picked". After an absence with enough check-ins, a While you were away card tells the night and only what needs you stays a card. Asks from child sessions dock on the main session with who asked and where, and answering settles the child's request. Expired, provider-ended and interrupted runs each say what happened and offer one ordinary prompt. A superseded session is read-only and links to the current one.
entry_points: web Sessions window; main session opened from the avatar row
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
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
