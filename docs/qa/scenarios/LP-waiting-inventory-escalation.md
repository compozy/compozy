---
id: LP-waiting-inventory-escalation
area: LP
title: Inspect a waiting node and its bounded escalation ladder
persona: Ada
journey: J-07
expected: Structured waiting inventory reports the timer, event, or approval reason and current escalation step truthfully; due steps emit once through the effect relay, three failed resume admissions require intervention, and any accepted decision cancels later steps.
entry_points: `compozy loop waits list -o json`; Loop wait detail and events over HTTP/UDS/native tools; daemon restart
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-approval-link-journey; LP-durable-wait-restart
---

QA impact 2026-08-02: Task 06 implements truthful durable wait inventory, authored escalation
effects, bounded resume admission, and intervention state. A real-user walk is blocked until Task
07 publishes the inventory and decision surfaces; Task 13 owns the isolated clock and restart walk.
