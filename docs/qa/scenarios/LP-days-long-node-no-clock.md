---
id: LP-days-long-node-no-clock
area: LP
title: Let a healthy Loop node run for days without a hidden clock
persona: Ada
journey: J-bound-runaway-work
expected: A node without an authored timeout stays live across far-forward clocks and daemon restarts, observable evidence prevents a silence flag, silence alone only raises attention, and later evidence clears attention without interrupting the prompt.
entry_points: `compozy loop runs show <run-id> -o json`; Loop node inventory and event history over CLI/HTTP/UDS
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: TA-action-run-liveness; LP-crash-death-resume
---

QA impact 2026-08-02: Task 05 implements evidence-only liveness and removes the hidden action
deadline. A real-user walk is blocked until Task 07 exposes lifecycle fields and attention events
through public structured surfaces. Task 13 owns the far-forward-clock and restart walk.
