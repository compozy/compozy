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

Fix commit: pending. No production edit yet. Collect a bounded successful and non-successful
owner sample before choosing the minimal durable classification at its owning boundary.
The existing daemon outbox relay suite owns forwarding/retry/ack behavior; existing session
stop suites own stop-reason/event projection. No new test file is planned.

## Verification

Original symptom confirmed; repair and fresh original-persona replay remain pending.
