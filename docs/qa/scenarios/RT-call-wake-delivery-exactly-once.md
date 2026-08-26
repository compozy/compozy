---
id: RT-call-wake-delivery-exactly-once
area: RT
title: Deliver every settled call to its caller exactly once, across a crash
persona: Ada
journey: J-delegate-work-to-an-agent
expected: Every terminal state produces exactly one result-carrying wake with the applicable payload, a committed call is never admission-denied on the way to delivery, and a crash between commit and notify redelivers the same wake identity instead of a second one.
entry_points: compozy call await call_01JBD8G2K7Q9 --timeout 120s; caller session ses_01JBD7ZZAAAA transcript wake row; HTTP and UDS GET /api/workspaces/{workspace_id}/calls/{call_id}; compozy task next --wait --lease-seconds 300; daemon restart; compozy logs --type call.settled
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-agent-call-golden-path; RT-agent-call-follow-up; RT-in-context-call-messages
---

Delivery is where an asynchronous design usually lies. Four properties have to hold together.

**Commit-then-notify.** The completion delivery row is written in the same transaction as
settlement and the notification fires only after commit. Prove it by killing the daemon inside that
window: on boot the caller receives the wake from the durable row, and because `wake_event_id`
dedupe is durable it receives exactly one — not two, not zero.

**Never admission-denied.** A committed call's delivery and activation cannot be blocked by a
budget, ceiling or admission funnel. Fill the per-root execution budget, then let an already-admitted
call settle: the wake still arrives. There is deliberately no delivery-skip event in the catalog, so
its absence is part of the evidence.

**Queued execution lives in `task_runs`.** Every child-starting call carries a `call_activation` run
claimed through the task authority — the fast path claims immediately, a budget-delayed run waits
visibly. Confirm the `calls` tables carry no claim or lease columns and that nothing scans call rows
to start work. Then fail an activation on purpose: the call must settle `failed` with a typed spawn
reason, never vanish between commit and start.

**The wake carries exactly the applicable payload.** For `completed`, a valid result reference and a
bounded preview; for every resultless terminal — failed, canceled, timeout, expired, invalid-result,
completed-without-result — the typed reason and evidence reference instead. A `completed` signal
without its result must not exist on any path. Compare the wake text against the daemon's own lines
rather than a paraphrase.
