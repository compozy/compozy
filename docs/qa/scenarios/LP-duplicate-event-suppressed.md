---
id: LP-duplicate-event-suppressed
area: LP
title: Suppress duplicate watch events loudly and durably
persona: Ada
journey: J-16
expected: Redelivering one valid watch event creates exactly one Loop run while every duplicate returns a structured suppression result, records a duplicate_suppressed diagnostic, increments counters, and remains suppressed across restart until the configured horizon expires.
entry_points: watch-source extension poll response; Loop start over CLI/HTTP/UDS/native tools; Loop diagnostics and counters
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-durable-wait-restart
---

QA impact 2026-08-02: Task 06 implements fail-closed `event_key` validation and an atomic
workspace/loop/source/event admission claim with durable expiry and loud suppression diagnostics.
A real-user redelivery walk is blocked until Task 07 exposes the structured start and diagnostic
surfaces; Task 13 owns the isolated restart and horizon walk.
