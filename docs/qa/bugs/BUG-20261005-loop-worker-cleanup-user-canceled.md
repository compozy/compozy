# BUG-20261005-loop-worker-cleanup-user-canceled: Automatic Loop cleanup claims operator cancellation

- **Status:** open
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Bruno
- **Journey Step:** J-01, inspect the finished worker after a successful implementation
- **Scenarios:** LP-003
- **Found:** 2026-10-05 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

A successfully completed Loop stops its owned agent session through automatic cleanup,
but the durable history records user_canceled and Session interrupted by operator.
The operator never canceled this run or session.

## Reproduction

1. Launch implement-tasks with the authored studio-welcome task and let it finish unattended.
2. Open its Done run from the Loop catalog, inspect execute_default and follow Open session.
3. Read that session's final history through CLI, HTTP and UDS.

Expected: automatic retirement preserves the truthful stop origin and does not invent an
operator action. Actual: session_stopped carries stop_reason=user_canceled,
stop_detail=Loop session cleanup: terminal, and a cancellation failure. A following
transcript_marker.created says Session interrupted by operator.

## Evidence

Run looprun-1d74598c588bf783 is Done; collect reports one success, per_task_done succeeds,
and the orchestrated branch is not taken. The task file and authored quick-start note exist.
Worker sess_bad163413cdbe63d256d772fa065d625 is stopped; events 196 and 197 carry the false
cancellation classification. HTTP/UDS preserve the same evidence after the earlier daemon restart.

Receipts under docs/qa/evidence/2026-10-02-untested/:
loops-first-run-lea-worker-history.json; loops-terminal-bruno-worker-history-{http,uds}.json;
loops-terminal-bruno-{terminal,why,done,worker-observation,session-ended,status-http,status-uds}.json.
The 11-frame loops-terminal-bruno recording is closed; the Done screenshot was inspected.
The Web transcript did not display the final marker in the observed viewport; the finding's
classification proof is the independent public history, not an unobserved visual label.

The initial wait for a Transcript button was a driver error; the session opened correctly.
Related historical Goal clear/judge bugs concern different symptoms and are not reopened.

## Fix

Root cause: goalSessionOutboxRelay.deliverPendingCleanups sends CauseUserRequested for
every obligation. The shared session stop classifier consequently emits cancellation
and the transcript marker. Cleanup CauseTerminal alone does not prove work succeeded:
failed, interrupted and successful owners share it. The correction must preserve this
distinction instead of mapping every terminal cleanup to completed.

Fix commit: pending. The relay now distinguishes automatic terminal/reseed/revoked-binding
retirement from explicit stop/cancel. The former uses the additive owner_released stop reason;
the latter retains user-requested cancellation. Node cancellation enqueues the explicit stop
cause. Existing durable text storage needs no migration. The API restart projection, generated
contracts, Web status fold and public documentation co-ship; see the 2026-10-05 worker-stop
entry in docs/_memory/change-impact.md.

The existing TestGoalSessionOutboxRelay suite owns forwarding/retry/ack behavior;
TestStopTransitionsToStoppedAndNotifies owns reason/event/failure/marker persistence;
TestGlobalDBLoopNodeCancellationShouldCommitTerminalCellAtomically owns node-cancel origin.
The existing Web thread-status and runtime-activity suites own stop attribution and failure
interpretation. Behavioral red/green receipts are retained; all focused race checks and 15
Web assertions pass. Generated contracts, Web/site typechecks and production builds pass.
React Doctor reports 100/100. No new test file was needed. Test-shape findings in unrelated
cases predate this edit; the baseline comparison reports no introduced violations.

## Verification

Bruno's successful, exhausted and explicit-node-cancel replays pass on binary
5781c6b4553aa2e9956136cf61c580245d04f37c13afda9c8195f0f4b57b077c and Web index-DQD-q9Mf.js:

- Successful run looprun-c69eb40eeca49d4e is Done; its worker
  sess_155bffcd881c6b007cee70ca5655e0cb records owner_released at sequence 125, without a
  cancellation failure or operator marker. The saved source index and completed task run agree.
- Budget run looprun-c600576569285100 is Exhausted, with 87,043 tokens and 2m55s against
  120K/20s. per_task_done stays pending and undispatched. Its task run completed, while worker
  sess_eae23a03e3d9fd3aa18542e33584aee7 records owner_released at sequence 128, without a
  cancellation failure or operator marker. The saved decision register is retained.
- Explicitly canceling execute_default in looprun-b1d0fd712e0182b5 stops active worker
  sess_5e31f1814581344d9a573c4d3cfadc2c with user_canceled/user_requested and the operator
  marker. One prompt receives ACP cancelled; no repair prompt starts. The parent ends Failed
  because collect cannot satisfy its contract after a required node is canceled. This was node
  cancellation, not whole-Run cancellation, and the parent outcome is reported unchanged.

CLI, HTTP and UDS reads independently establish the outcomes. Fresh Web reads show Done and
Exhausted separately, with both automatic worker rows reading "no longer needed"; explicit
cancellation retains the verified escalated-stop wording. Screenshots were visually inspected.
All three classifications survive a daemon restart, including API reconstruction of stop_cause
from durable metadata. The exact 25-frame loops-cleanup-bruno recording is closed. Two browser
driver waits looked for Open session in the wrong panel/canceled-node view; they are retained as
driver evidence, not product failures. The successful/exhausted workers were reached through the
Inspect graph; the canceled worker was checked through its public session URL.

Receipts: loops-cleanup-bruno-{success,budget,node-cancel}-*.json,
loops-cleanup-durable-*.json, loops-cleanup-{success,budget,node-cancel}-owned.json,
loops-cleanup-*-regression-{red,green}.json and loops-cleanup-web-site-build.json, under
docs/qa/evidence/2026-10-02-untested/. Delivery gate and commit remain pending.

## Re-found — 2026-10-05, exhausted owner

Bruno's fresh CH-012 budget walk ends looprun-4f2b4617dbd32be1 as Exhausted, while its
worker task run completed successfully and per_task_done was never dispatched.
Worker sess_ebff5331023169eebf11579cb39b71b5 stops automatically at 10:18:53.843526Z,
but sequence 94 records user_canceled plus a cancellation failure and sequence 95 says
Session interrupted by operator. No operator stop occurred. The task-run read independently
proves completed; the Loop read proves exhausted. This demonstrates why copying the parent
outcome or mapping every cleanup to completed would also be untruthful.

The repair will represent automatic owner release as its own stop reason, preserving
existing provider failures and the separate task/Loop outcome. Explicit operator stop
and cancel remain user-requested. This uses the existing unconstrained persisted stop-reason
text and adds a public enum value; no database shape or existing history is rewritten.
Owning invariants: daemon cleanup relay selects the correct initiator; session lifecycle
persists that initiator without inventing success or cancellation failure. Reuse the existing
TestGoalSessionOutboxRelay and TestStopTransitionsToStoppedAndNotifies suites.
