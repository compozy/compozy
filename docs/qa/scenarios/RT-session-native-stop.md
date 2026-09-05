---
id: RT-session-native-stop
area: RT
title: Stop another session through the governed native tool
persona: Ada
journey: J-15
expected: compozy__session_stop stops one live same-workspace target through the canonical session stop path, returns the terminal winner once, applies destructive approval policy, denies self or foreign-workspace targets, and leaves repeated or raced callers with deterministic structured outcomes.
entry_points: compozy__session_stop; compozy session stop; POST /api/workspaces/{workspace_id}/sessions/{session_id}/stop over HTTP and UDS; compozy session status <session-id>
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-08-16-herdr-parity.md; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/bootstrap-manifest.json;docs/qa/reports/2026-09-05-sessions-stability-task01-02.md;/Users/pedronauck/dev/qa-labs/compozy-sessions-stability-task01-02-20260905-154017-502928-lab/qa-artifacts/qa/evidence/walkA2-stop-wait.json;/Users/pedronauck/dev/qa-labs/compozy-sessions-stability-task01-02-20260905-154017-502928-lab/qa-artifacts/qa/evidence/walkA2-events.json;/Users/pedronauck/dev/qa-labs/compozy-sessions-stability-task01-02-20260905-154017-502928-lab/qa-artifacts/qa/evidence/walkE-status-after-restart.json;/Users/pedronauck/dev/qa-labs/compozy-sessions-stability-task01-02-20260905-154017-502928-lab/qa-artifacts/qa/evidence/screenshots/f2-02-stopping-4s.png;/Users/pedronauck/dev/qa-labs/compozy-sessions-stability-task01-02-20260905-154017-502928-lab/qa-artifacts/qa/evidence/screenshots/f2-03-stopped-confirmed.png
last_report: docs/qa/reports/2026-09-05-sessions-stability-task01-02.md
overlaps: RT-session-wait-state; RT-session-prompt-cancel
---

Start two managed sessions in one workspace and one in a neighboring workspace. From Ada's session,
stop the live sibling, race a second stop over another public surface, then compare fresh status and
the terminal event over CLI, HTTP, and UDS. Confirm destructive approval policy, target ownership,
self-action denial, cross-workspace denial, and that prompt cancellation remains the non-terminal
choice when only the current turn should end.

QA impact 2026-08-16: Task 04 added `compozy__session_stop` as one of the seven Herdr parity native
tools. The earlier QA flags covered the other six tools but no scenario owned native stop, so this
content-addressed row closes that journey-derived gap for Task 08.

QA 2026-08-16 Herdr parity: The full runtime E2E exercised the public HTTP, UDS, CLI, and native-tool paths, including matching persisted projections, restart recovery, scoped denials, bounded wait/notify/cancel/stop races, and stable negative outcomes (65/66/69/75/78, agent_scope_denied, and queue-full).

QA impact 2026-09-05 sessions-stability: verify explicit wait:false accepts without waiting and
wait:true returns verified/escalated/phase/cause/elapsed truth across native, CLI, HTTP and UDS.
An exhausted unverified kill must return stopping + stop_verification_failed, never stopped.
Confirm a repeated completed stop does not restart a ladder, self/foreign targets remain denied,
and the default CLI session resource retains its established fields. Exercise the v0.4 compatibility
requests without wait (HTTP204/native legacy shape plus warning). Final real walk remains pending.

Stop a session, delete its history, then repeat the stop request twice. Both requests must report
the missing session instead of replaying the old successful stop. A failed deletion that retains
history must retain the existing stop outcome; pending waiters still receive their original result.

Reconnect after an exhausted stop and confirm the session resource, attention list/summary and
catalog stream retain needs-attention + stop_verification_failed. Compare compact status
lifecycle_state/verified/escalated/attention across HTTP, UDS and CLI formats. Retry from Web:
duplicate activation stays guarded, request failure remains retryable, a second failed ladder
restores the action, and verified death clears attention without losing the draft or scroll position.

Stop while a launcher ignores cancellation and has not returned a process handle. After the
configured cooperative grace plus the forced/kill budgets, require a settled unverified stop
result and durable stop-verification attention, with no stopped notification. Release the
launcher later and verify that its returned process is terminated through the shared ladder,
or that a canceled launch with no process completes normally; retry stop remains available.

After verified stop, deliver delayed message chunks and process errors from the old turn. Confirm
that they neither appear as agent output nor change the stopped reason/attention, and that the
transcript records a post-stop discard marker. Include the finalization overlap and reconnect replay
in the final runtime walk; focused manager tests alone do not verify that concurrent journey.
With the Web transcript already open on that session, the discard marker must appear in place
without navigating, remounting, reconnecting the stream, or polling: the catalog wake re-reads the
session detail and re-reads the transcript only once that read is no longer live, so a wake that
lands while the page still shows active/stopping never fakes a stopped state. Another open session
in the same or another workspace must not re-read its transcript.

Restart with an unresolved stop, reconnect, and retry the catalog-only session. Verify the
persisted attention and metadata survive another unverified attempt. For a remote sandbox,
a missing or reused local PID must neither prove remote exit nor trigger host process-group
signals. Exercise the verified local orphan path and concurrent retry/resume before accepting
this journey; those end-to-end cases remain unverified.

For recovered local stops, exercise a temporary terminal metadata/catalog failure after process
exit. Retry the same session after restoring persistence: confirm the original cause/phase remain,
no additional escalation occurs, one terminal event/notification is delivered, and resume cannot
bypass pending terminal persistence. Clear, delete and workspace removal must also refuse pending settlement;
for a recovered unverified process, both preserve its history until exit is proven. Keep this distinct from a ledger cleanup error after a
completed terminal write, whose original diagnostic remains available through AwaitStopped.

For an active session, reject the catalog write of the stop classification after verified exit.
The session must remain `stopping`, with no terminal event or stopped notification. Restore the
catalog and retry: preserve the original termination phase, emit the terminal result once, and
do not repeat process signals or `session.post_stop` hooks. Existing waiters must still await
the hook even when classification persistence fails. The full finalization/restart walk must
also cover failures after classification, in terminal-event and final-state persistence.

After terminal-event persistence, reject the final stopped-state metadata write and catalog
update separately. Require `stopping`, no stopped notification, and refusal of resume/delete
while persistence is pending. Restore storage and retry: preserve the original verified phase,
commit stopped metadata/catalog, and observe one terminal event, one cleanup and one notification.

Reject terminal-event writes through both the prompt recorder and the stored-session writer.
Retry after restoring storage and confirm one terminal event and notification. Repeat with a
write that commits but loses its acknowledgment: the retry must reuse the same event identity.
If only the prompt recorder fails, the stored-session writer may complete the durable event;
the final stop remains verified and retains the original recorder diagnostic.

Restart an active-session stop after its terminal recovery receipt is durable, covering an
unwritten terminal event, a committed event with lost acknowledgment, and a failed final catalog
write. Boot must retain the original cause, phase and elapsed time and leave exactly one terminal
row. Refusing the receipt write must prevent terminal publication; restoring storage permits retry.
Verify that successful settlement removes the receipt. A crash before this receipt is written
uses interrupted-stop classification and fresh process-identity verification; it need not retain
an outcome that had not yet been journaled. Include that branch in the final boot journey.

Restart the daemon after a recovered process exits but terminal metadata/catalog persistence
fails. Boot must recover the original verified outcome and complete the terminal event before
admitting new work; a direct resume against a fresh manager must respect the same pending fence.
Repeat boot after successful settlement and confirm no extra terminal event or notification.
Malformed, incompatible or wrong-owner settlement receipts must remain intact and block recovery.

During boot, an orphan with matching PID/start identity must use the same stop ladder as an
operator request, retaining agent_crashed attribution and canonical escalation/terminal events.
There is no second daemon-owned TERM/KILL attempt after the manager settles or exhausts a stop.
An unverified remote process keeps stopping/attention and permits boot only after its diagnostic
is durable; failure to persist recovered metadata, catalog state or stop events aborts recovery.

For a recovered local sandbox, verify sync-from-runtime precedes optional destruction according
to its stored profile. Cover both destroy policies and a provider sync failure: terminal state
stays stopped, sandbox diagnostics/state match the catalog, and session name and creation
metadata remain intact. The full runtime walk remains deferred to final QA.

Exercise delayed pre-record and post-record hooks during stop for both individual messages and
chunk batches. Release the hook after stop settles: a closed recorder must not produce a new
prompt transport failure, and delayed output must not reach attach/notifier consumers. Output
committed before stop remains in history before the terminal event; the discarded delivery gets
a post-stop marker. Also verify ordinary active-session persistence failures still surface.

Inject ledger and network cleanup timeout after verified exit. Confirm stopped metadata and
notification still settle, then reopen history and inspect the redacted runtime_warning for
the failed step. Ledger errors remain available from the stop result; canceled network cleanup
keeps its existing successful-stop behavior only if the warning persists. Diagnostic persistence
failure must be returned, and no cleanup error may turn the session active again.

Repeat boot recovery with a process that died before inventory, while metadata still says active
or stopping. Verify recorded PID/start exit proof, one new terminal event/notification, crash
attribution and no new escalation. Restart between writing the verified settlement receipt and
updating metadata; the still-active metadata must not bypass receipt recovery. A second completed
boot must not repeat settlement. Missing process identity is a separate unverified case.

Recover active, stopping and starting metadata with no liveness object, empty identity, or PID
without start time. Each must retain stopping and verification-failed attention without signaling
an unproven process or discarding history. Separately, after verified crash recovery, resume must
first attempt the saved ACP session ID and use the existing missing-session fallback only when
the provider rejects loading it. A verified incomplete start still gets a fresh ACP session.

For an interrupted start whose process is verifiably gone, require failed/startup attribution,
a cleared incomplete ACP ID and exactly one new terminal event/notification. Repeat with a
restart after receipt persistence but before starting metadata is updated. Neither path may
skip cleanup or start another termination ladder; repeated completed boot remains idempotent.

Repeat interrupted-start recovery with a matching live process. Persist the intermediate
stopping inventory, restart again, and verify failed/startup attribution survives while the
shared ladder terminates and verifies the process. Clear the incomplete ACP ID only after exit;
retain any provider-auth diagnostic without relabeling it as a generic process error. Repeat
completed recovery and confirm no additional event, notification or escalation.

For Daytona sidecar stop failures, confirm DELETE retains the process record and repeated
requests preserve the original failure rather than returning a false success or missing record.
Successful stop still removes the record after termination. The full remote-restart journey
must also validate durable launcher identity, remote exit proof and existing-sidecar upgrade;
local sidecar tests alone do not establish those properties.

Query the Daytona sidecar process status before and after termination. A live process has no
exit code; command completion exposes its exit code, while `exitVerified` is true only after
the sidecar has verified process-group exit. An unknown or deleted ID returns 404 and is not
exit proof. Repeat through daemon recovery once durable remote identity integration is available.

For new Daytona starts, verify provider state contains the reserved launcher process ID before
agent creation and the launch response preserves it. Diagnostic probes must not consume that ID.
Concurrent duplicate launches create one process; subsequent reuse after removal is rejected
within the same sidecar lifetime. An older sidecar must reject identified launch before creating
an anonymous process. Sidecar restart persistence and safe upgrade remain part of the full journey.

Recover a session with a valid stored Daytona launcher identity and a still-running sidecar.
Verify the daemon applies close-input, terminate and kill through the provider as needed, with
each call bounded by the shared phase deadline. Signals retain the remote record; status must
match the requested ID and include command completion, exit code and process-group proof before
terminal settlement. Missing IDs, unavailable transport and mismatched sandbox/process identities
retain attention. Recovery must not bootstrap or replace a sidecar while looking up exit proof.

Upgrade a sandbox with a running v1 sidecar: new preparations reserve v2 and use its separate
binary path and port, leaving v1 processes intact. The saved launcher_sidecar_version must
route recovery to the original endpoint; an older identified record without this field maps
to v1. Unknown versions fail explicitly before connection. Reusing a healthy current sidecar
must not upload its executable again. Verify HTTP and websocket tunnel dialing respects cancellation.

If the sidecar's exit observer fails to finish after forced termination, Stop must return a
deadline error after its bounded observer wait rather than hang. Keep the process record and
unverified status available; a completed process-group action alone must not fabricate the
missing command-exit observation. Repeated requests retain the original failure.

With a known process handle and an unfinished exit observer, repeat Stop after a failed attempt.
Concurrent callers must share one new bounded attempt, rather than reuse the exhausted attempt
forever or run parallel termination sequences. Completed outcomes and failures without a pending
process retain their existing result.

QA 2026-09-05 sessions-stability task_02: `compozy session stop --wait -o json` on an acpmock agent returned
stopped / verified true / escalated true / phase forced with the process gone and session.stop_escalated +
session_stopped persisted; a cancel-ignoring turn (acpmock hold_ignoring_cancel) kept the Web primary
control on "Stopping…" through the 10 s cooperative grace before the ladder escalated (stop_escalated
scope turn, elapsed_ms 10000) and the session stayed promptable; `kill -9` of the daemon mid-turn followed
by restart left the session stopped / failed / dead with stop_reason agent_crashed and no phantom or orphan.
Not walked here: compozy__session_stop from a governed agent session, UDS parity, the unverifiable-kill
attention branch, delete-after-stop replay, and the Daytona remote-exit journey — the scenario stays
blocked-verify for those branches.
