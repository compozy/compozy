---
id: LP-duplicate-event-suppressed
area: LP
title: Suppress duplicate watch events loudly and durably
persona: Ada
journey: J-07
expected: Redelivering one valid watch event creates exactly one Loop run while every duplicate returns a structured suppression result, records a duplicate_suppressed diagnostic, increments counters, and remains suppressed across restart until the configured horizon expires.
entry_points: watch-source extension poll response; Loop start over CLI/HTTP/UDS/native tools; Loop diagnostics and counters
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: internal/daemon/loop_node_lifecycle_e2e_integration_test.go; public QA has no event replay injector or controllable dedupe-horizon clock
last_report: docs/qa/reports/2026-08-03-loop-node-lifecycle.md
overlaps: LP-durable-wait-restart
---

acceptance-walk: Submit one event without event_key and confirm fail-closed rejection, then redeliver one valid event before and after daemon restart and again after the configured horizon. Confirm one run before expiry, loud duplicate_suppressed results and counters for repeats, and one newly admitted run only after expiry across native, CLI, and HTTP reads.
