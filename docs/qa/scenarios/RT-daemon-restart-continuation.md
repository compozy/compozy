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
