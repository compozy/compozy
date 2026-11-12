---
id: RT-daemon-restart-continuation
area: RT
title: Continue interrupted sessions after a daemon restart
persona: Dora
journey: J-daemon-restart-continuation
expected: Eligible interrupted sessions preserve partial work, resume once, and finish without falsely reporting an agent failure; unsafe or explicitly stopped work waits for review.
entry_points: compozy daemon restart; compozy session events; session permalink; HTTP/UDS session reads
qa_status: untested
bug_ids: GH-1580
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-daemon-drain-admission; RT-subagent-restart
---

1. In an isolated home, start three long turns with partial text and queue two inputs per session.
   Restart normally, then repeat with SIGKILL of the owned daemon process. All sessions retain IDs
   and partial text, the old provider processes are reaped, and one continuation per source turn
   precedes queued inputs. Fresh structured reads and Web notices agree; no false failed badge.
2. Restart again after continuation admission and after completion. Previously consumed identities
   do not dispatch twice. A dispatch with uncertain delivery is never blindly resent.
3. Interrupt a tool after its side effect but before its result. History closes the call as aborted,
   with unknown outcome; the continuation asks the agent to inspect state before repeating actions.
4. Interrupt pending permission and clarification requests. Neither is automatically answered;
   continuation pauses with a reason and the operator can inspect retained history.
5. Repeat with changed provider command, selected runtime, tool permissions, or worktree; an
   archived session; and an interrupted maintenance command. Each requires review.
6. Set `session.restart.auto_continue = false`, then restart; no continuation runs. Restore the
   default and trigger four interruptions inside five minutes: the fourth is paused by the breaker.
7. Explicitly stop a session and then stop the daemon. Start again: neither automatically resumes.
   A turn that finishes during the grace period also receives no continuation.

The canonical composition-root test is `TestDaemonRestartContinuesSessions` in
`internal/daemon/restart_integration_test.go`; qualification, durable attempt limits and admission
identities belong to the session manager resume suite. Focused integration evidence does not by
itself certify all Web/CLI interaction branches above.

Targeted automated verification 2026-10-23 (#939): the changed runtime contracts passed in the
canonical session/daemon suites and isolated real-process probes. Restart/stop detail and persisted
events were independently read over HTTP and UDS after replacement and a third boot; no false failed
badge or duplicate continuation was observed. See
[the scoped report and public readback evidence](../reports/2026-10-23-issue-939-restart-continuation.md).
The full persona/Web/CLI journey remains untested; this automated evidence does not replace that verdict.

2026-10-27 update continuity regression: start concurrent work and a delegated child before
app Update. After native installation and runtime replacement, the same session IDs and partial
text remain; eligible parents and children continue without a manual prompt or a false failure.
The child completes and delivers its durable result once. An explicitly stopped control remains
stopped, and a subsequent restart does not duplicate continuation or child delivery. A failed
bundle install leaves the live daemon untouched; failure to request coordinated restart never
falls back to user-stop signaling. Existing opt-out, pending-interaction and binding-change gates
remain in force. E2E-016 owns the app/daemon handoff; session/daemon suites own recovery safety.
Evidence and execution limits: [continuity report](../reports/2026-10-27-update-session-continuity.md).

QA impact 2026-11-12 (#1580): repeat the restart with one or more active always-on instances
whose main sessions have queued input and need their provider resumed. Instance service setup
still precedes the listeners, while reconciliation and its catalog-change callback are activated
only after HTTP and UDS start. Expect the retained input to run on the first resume attempt, with
no hosted MCP bind-deadline delay, `session.start.driver_start_failed`, or
`daemon: reconcile instances at startup` error. Record time-to-ready and the instance resume count.
`TestBootResumesInstancesAfterServersStart` in the daemon integration suite checks the pending
input and unbound runtime before UDS creation, then delivery without startup errors after boot.
Validation for this change is CI-only; the desktop restart walk has not been rerun locally.

QA impact 2026-11-12 (#1614, continuation correctness): a failure left on an interrupted session
no longer pauses continuation by itself. Interrupt a turn, then make the next provider start fail
for an infrastructure reason before recovery runs: a hosted MCP bind deadline, an adapter prepare
or initialize timeout, a start canceled by daemon shutdown, or a provider pipe closed during start
(`startup_failure`, `handshake_failure`, `timeout`, `cancellation`, or `transport_failure` without
a prompt reason code). Restart again: the turn continues once, no
`restart continuation requires operator input` line or "Automatic continuation paused: agent
failure" notice appears, and after the resume structured session reads and `meta.json` carry no
`failure` and no failed badge. Repeat with a genuine agent failure retained on the interrupted
session (`process_exit`, `prompt_failure`, `protocol_failure`, `provider_auth_failure`,
`permission_failure`, `load_session_failure`, or a `transport_failure` with
`prompt_stream_incomplete`): continuation still pauses with reason `agent_failure` and the failure
stays visible for review. Separately, resume a resumable crashed session by hand: it keeps its crash stop
reason until its next stop, and its retained failure is cleared once the runtime is bound.
`TestRestartContinuationRetainedFailure` and `TestResumeClearsRetainedCrashFailure` in the session
manager suite own these rules. Validation for this change is CI-only; the walk has not been rerun.

## Safe relaunch pre-flight (#1614, requirement 6)

QA impact 2026-11-12 (#1614, safe relaunch): before a restart signals the running daemon, it runs
the replacement binary's hidden `compozy daemon preflight`, which loads and validates the
configuration as launch and boot do and plans the global database migrations through a read-only
handle. Per-session databases are not opened. Walk, in an isolated home only:

1. With the daemon running, add an unknown key to `config.toml` and request a restart
   (Settings → Restart CompozyOS, or `POST /api/settings/actions/restart`). Expect `409` with the
   config reason, the same daemon pid still serving, no `compozy daemon relaunch` helper process,
   and one `restarts/<id>.json` with `status: failed` and a `failure_reason` starting with
   `restart pre-flight:`. Remove the key and restart: it completes with a new pid.
2. Replace the installed binary with an older release whose global schema head is behind the
   database, then request a restart. Expect the same refusal with a `migration_plan` reason naming
   the ahead schema; the running daemon keeps serving and the database is unchanged.
3. Replace the installed binary with a release that predates the pre-flight command and request a
   restart. Expect the restart to proceed as before, with
   `daemon: replacement pre-flight did not conclude; restarting without it` in the daemon log.
4. Run `compozy daemon preflight` by hand against the lab home: one JSON line with
   `schema: compozy.daemon.preflight/v1`, exit `0` when `ok` is true and non-zero otherwise, and
   no change to `compozy.db` or `restarts/`.

`TestRequestRestartPreflightsReplacementBeforeSignal`, `TestClassifyRestartPreflight`,
`TestDefaultRestartPreflightRunsReplacementCommand` and `TestRunRestartPreflightReportsBootReadiness`
in the daemon restart suite and `TestDaemonPreflightCommandReportsBootReadiness` in the CLI suite
own these rules. Validation for this change is CI-only; the walk has not been run.

## Issue 1614 requirement 7: what clients show during the serving phase

QA impact 2026-11-12 (#1614): the web shell reads `daemon.readiness` from `GET /api/status`.
Validation for this change is CI-only; the walk has not been run.

1. Restart a lab daemon with sessions interrupted mid-turn and keep the web UI (or the desktop
   app) open. While `daemon.readiness.phase` is `serving`, expect one notice on the desktop:
   the current step in plain words ("Reconnecting subagents", "Resuming interrupted sessions",
   "Restoring monitors", "Checking the session list", "Restoring your agents", or "Resuming your
   sessions" for a step with no name) and "You can read your sessions now. Sending opens again
   when this finishes." Session lists, transcripts and status load normally.
2. Expect the notice to clear within about 2 seconds of `phase: "ready"`, with no page reload.
3. Against a daemon that reports no `daemon.readiness`, expect no notice.
4. After the restart, expect lists and open sessions to show current data without a reload: the
   page marks its cached reads stale when `/api/status/identity` reports a new `pid` or
   `started_at`.


## Issue 1614 requirement 5: graceful, bounded shutdown

QA impact 2026-11-12 (#1614): a restart no longer spends `daemon.shutdown_grace` waiting on turns
it will checkpoint and resume, starts nothing once shutdown begins, and always ends the process.
Validation for this change is CI-only; the walk has not been run. Walk in an isolated home only:

1. With `daemon.shutdown_grace = "30s"` and three sessions streaming a long turn, request
   `compozy daemon restart`. Expect the old daemon log to show `drain_and_checkpoint_sessions`
   ending within about 5 seconds and, when turns were still running,
   `daemon: restart drain ended with turns still running` with `running_turns` and `drain_ms`.
   With one session parked on an approval or a question and none streaming, expect that phase to
   end immediately. With `shutdown_grace = "0s"`, expect no wait at all.
2. After the replacement boots, expect each interrupted session to record the shutdown stop reason,
   no `failure` in its metadata, no failure row in its transcript, and exactly one terminal event
   for the interrupted turn. The continuation runs as before.
3. Once the old daemon has received the restart request, expect it to start no provider process
   and no automatic runtime recovery; a start that raced the shutdown logs
   `session.start.refused_daemon_shutdown` and leaves the session resumable rather than failed.
4. With a session mid tool call on a stdio MCP server, restart. Expect the log order
   `stop_workers_and_sessions` (with `stop_workers` and `stop_sessions` inside it) before
   `close_session_dependencies`, then `close_servers_and_hooks` and `close_persistent_resources`,
   and no MCP transport error recorded for that call before its session stopped.
5. With a provider that ignores the stop ladder so session cleanup times out, restart. Expect the
   `close_persistent_resources` step to still run, `compozy.db-wal` to be checkpointed (small or
   empty after exit), and the replacement to boot without a long WAL recovery.
6. With a teardown that never returns (lab fault injection), expect the process to exit with code
   `1` no later than the total shutdown budget plus 2 seconds, after logging
   `daemon: shutdown watchdog forcing process exit` with the `phase` and `step` still running. A
   normal `compozy daemon stop` or restart never logs that line.

`TestDaemonShutdownOperation` (drain cap, shutdown mark before admission closes, MCP pool after
sessions, database close budget, terminal incomplete close, watchdog) and
`TestShutdownTearsDownInRequiredOrder` in the daemon shutdown suite, and
`TestDaemonShutdownRefusesProviderStarts`, `TestDaemonShutdownExitReclassification` and
`TestPromptFatalProcessFailureStopsSessionAndPreservesReadOnlyHistory` in the session suites own
these rules.
