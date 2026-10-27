---
id: RT-daemon-restart-continuation
area: RT
title: Continue interrupted sessions after a daemon restart
persona: Dora
journey: J-daemon-restart-continuation
expected: Eligible interrupted sessions preserve partial work, resume once, and finish without falsely reporting an agent failure; unsafe or explicitly stopped work waits for review.
entry_points: compozy daemon restart; compozy session events; session permalink; HTTP/UDS session reads
qa_status: untested
bug_ids:
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
