---
id: LP-terminal-outcome-notification
area: LP
title: Deliver the effect for the terminal outcome that actually committed
persona: Loop operator
journey:
expected: Exactly the declared effect for the committed done, no-op, blocked, failed, exhausted, or stalled outcome is delivered once, and its result is observable without firing another lifecycle hook.
entry_points: Loop definition; loop run event stream
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

QA impact 2026-08-02: Task 03 implements committed-boundary selection, including store-decided budget exhaustion. A real-user walk is blocked until Task 07 exposes effect result and custom-event kinds through the public event contract and SSE parity; Task 05 will add the canceled terminal outcome.
