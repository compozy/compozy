---
id: RT-agent-call-deadline-timeout
area: RT
title: Settle a call on its opt-in deadline through one authority
persona: Bruno
journey: J-delegate-work-to-an-agent
expected: A call with no deadline runs until it settles or is canceled, a call with one settles timeout exactly once through the deadline sweeper, and an await box that elapses returns a resume handle at exit code 3 without touching the call.
entry_points: compozy call reviewer "Run until the deadline" --deadline 5s; compozy call await call_01JBD8H9PW2M --timeout 120s and --resume cawait_01JBD8KQ33; HTTP and UDS POST /api/workspaces/{workspace_id}/calls with {"target":{"agent":"reviewer"},"prompt":"Run until the deadline","deadline_seconds":5}; HTTP and UDS POST /api/workspaces/{workspace_id}/calls/{call_id}/await with {"timeout_ms":120000,"resume":"cawait_01JBD8KQ33"}; compozy__call_await with {"call_ids":["call_01JBD8H9PW2M"],"timeout_ms":120000,"resume":"cawait_01JBD8KQ33"}
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-agent-call-golden-path; RT-agent-call-cancel
---

Two different clocks share the word "timeout" and must never be confused. The **await box** belongs
to the waiter: when it elapses the call keeps running and the waiter gets `state: timeout` plus a
`resume` handle at exit code 3 — a checkpoint, not a failure — and `--resume` picks the same wait
back up. The **call deadline** belongs to the call and is opt-in only: there is no default, so a
call without `--deadline` runs until it settles, is canceled, or its parent drains.

Walk both. Confirm a call created with no deadline shows none in its record and is still running
well past any await box. Then set one and let it elapse: the deadline sweeper must be the only path
that terminalizes it, fencing the activation run or managed-stopping the running child, settling
through the single settlement writer, and delivering the result-carrying terminal wake. Race it
deliberately — return-versus-deadline and cancel-versus-deadline must each resolve to exactly one
terminal outcome, with the loser preserved as superseded evidence rather than overwriting the
winner. Finally check the clamps: `--timeout` and `timeout_ms` clamp to 30 minutes on every surface
and say so rather than rejecting, while a zero, negative or non-integer deadline is rejected up
front with `call_deadline_invalid`.
