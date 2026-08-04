---
id: LP-waiting-inventory-escalation
area: LP
title: Inspect a waiting node and its bounded escalation ladder
persona: Ada
journey: J-07
expected: Structured waiting inventory reports the timer, event, or approval reason and current escalation step truthfully; due steps emit once through the effect relay, three failed resume admissions require intervention, and any accepted decision cancels later steps.
entry_points: `compozy loop nodes --state waiting -o json`; Loop wait detail and events over HTTP/UDS/native tools; daemon restart
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: live timer and approval waits were inventoried across CLI, HTTP, Web, and native tools; public QA has no controllable escalation clock or admission-failure injector
last_report: docs/qa/reports/2026-08-03-loop-node-lifecycle.md
overlaps: LP-approval-link-journey; LP-durable-wait-restart
---

acceptance-walk: Create timer, event, and approval waits, advance the isolated clock through authored escalation steps, and force three failed resume admissions before accepting a decision. Confirm inventory reason and step, one delivery per due effect, intervention after the third rejection, cancellation of later steps, and native, CLI, and HTTP parity.
