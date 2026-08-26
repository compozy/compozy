---
id: ET-terminal-agent-handoff-input
area: ET
title: Watch and safely control an agent terminal
persona: Marina
journey: J-supervise-agent-terminal
expected: An approved agent command streams live while the operator watches read-only, control transfers explicitly, typing grants remain terminal-scoped, and hidden input can be answered or rejected without entering the visible transcript.
entry_points: Terminal app; pending approvals; compozy__terminal_exec; compozy__terminal_open; compozy__terminal_write; compozy__terminal_read; compozy__terminal_wait; compozy__terminal_signal; compozy__terminal_close; compozy__terminal_list; compozy__terminal_request_input; compozy__terminal_yield; compozy__terminal_claim; terminal input request card
qa_status: untested
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-20260826-074528-452132-lab/qa-artifacts/qa/test-e2e-runtime-after-fix.log; docs/qa/reports/2026-08-26-integrated-terminal.md
last_report: docs/qa/reports/2026-08-26-integrated-terminal.md
overlaps:
---

Flagged by integrated-terminal task 06. Task 10 owns the real-user walk, evidence, and verdict.

Walk:

1. Exercise `compozy__terminal_exec` and `compozy__terminal_open`, then use `compozy__terminal_list`, `compozy__terminal_read`, and `compozy__terminal_wait` to watch live output without taking control.
2. Use `compozy__terminal_claim`, take control from a second client, verify presence and control announcements, then use `compozy__terminal_yield` to release it.
3. Grant typing for that terminal, exercise `compozy__terminal_write`, verify follow-up writes do not prompt, and verify another terminal does.
4. Exercise `compozy__terminal_request_input`; answer one hidden request and reject another, then confirm only the length marker reaches the stream.
5. Exercise `compozy__terminal_signal` and `compozy__terminal_close`; verify their results and final terminal state agree.
