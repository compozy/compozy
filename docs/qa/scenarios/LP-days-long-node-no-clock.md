---
id: LP-days-long-node-no-clock
area: LP
title: Let a healthy Loop node run for days without a hidden clock
persona: Bruno
journey: J-recover-loop-node-failure
expected: A node without an authored timeout stays live across far-forward clocks and daemon restarts, observable evidence prevents a silence flag, silence alone only raises attention, and later evidence clears attention without interrupting the prompt.
entry_points: `compozy loop status --run-id <run-id> -o json`; Loop node inventory and event history over CLI/HTTP/UDS
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: looprun-98913; exact resume_at persisted across daemon restarts; public QA has no controllable far-forward clock
last_report: docs/qa/reports/2026-08-03-loop-node-lifecycle.md
overlaps: TA-action-run-liveness; LP-crash-death-resume
---

acceptance-walk: Start a node with no authored timeout, advance the isolated clock across days and restart the daemon while emitting real progress evidence. Confirm the node remains live, silence alone only raises attention, later evidence clears it without interrupting the prompt, and CLI and HTTP history show no hidden deadline transition.
