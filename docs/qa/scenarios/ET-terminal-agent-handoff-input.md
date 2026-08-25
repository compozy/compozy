---
id: ET-terminal-agent-handoff-input
area: ET
title: Watch and safely control an agent terminal
persona: Marina
journey: J-supervise-agent-terminal
expected: An approved agent command streams live while the operator watches read-only, control transfers explicitly, typing grants remain terminal-scoped, and hidden input can be answered or rejected without entering the visible transcript.
entry_points: Terminal app; pending approvals; compozy__terminal_exec; compozy__terminal_write; terminal input request card
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

Flagged by integrated-terminal task 06. Task 10 owns the real-user walk, evidence, and verdict.

Walk:

1. Approve a deliberate agent command and watch its live output without taking control.
2. Take control from a second client, verify presence and control announcements, then release it.
3. Grant typing for that terminal, verify follow-up writes do not prompt, and verify another terminal does.
4. Answer one hidden input request and reject another; confirm only the length marker reaches the stream.

