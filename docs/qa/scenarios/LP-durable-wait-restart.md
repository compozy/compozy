---
id: LP-durable-wait-restart
area: LP
title: Resume one durable Loop wait across a daemon restart
persona: Ada
journey: J-07
expected: A timer or event wait survives daemon restart, resumes exactly once from its durable row, ignores an event at or before the ahead cursor, consumes a valid ahead arrival once, and preserves workspace isolation throughout recovery.
entry_points: `compozy loop status --run-id <run-id> -o json`; Loop waiting inventory over CLI/HTTP/UDS/native tools; daemon restart
qa_status: blocked-verify
bug_ids: BUG-20260803-loop-boot-active-coordinator-lease; BUG-20260803-cross-origin-coordinator-duplicate
fix_status: fixed
retest_status: pass
fix_commits: Task 13 checkpoint
evidence: looprun-98913; exact resume_at survived repeated daemon restarts; internal/daemon/scheduler_runtime_test.go; public QA has no event-cursor injector
last_report: docs/qa/reports/2026-08-03-loop-node-lifecycle.md
overlaps: LP-duplicate-event-suppressed
---

acceptance-walk: Park separate timer and event waits, restart the isolated daemon, deliver one event at or behind the stored cursor and one valid ahead event, and inspect the timer due scan. Confirm each wait resumes exactly once, the stale event is ignored, and workspace-scoped native, CLI, and HTTP reads agree after refresh.
