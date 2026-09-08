---
id: ET-terminal-agent-handoff-input
area: ET
title: Watch and safely control an agent terminal
persona: Marina
journey: J-supervise-agent-terminal
expected: An approved agent command streams live while the operator watches read-only, control transfers explicitly, typing grants remain terminal-scoped, and hidden input can be answered or rejected without entering the visible transcript.
entry_points: Terminal app; pending approvals; compozy__terminal_exec; compozy__terminal_open; compozy__terminal_write; compozy__terminal_read; compozy__terminal_wait; compozy__terminal_signal; compozy__terminal_close; compozy__terminal_list; compozy__terminal_request_input; compozy__terminal_yield; compozy__terminal_claim; terminal input request card
qa_status: skipped
bug_ids: BUG-20260901-private-passphrase-session-composer; BUG-20260902-private-input-shell-leak
fix_status: fixed
retest_status: pass
fix_commits: pending-remediation-batch
evidence: /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-review-r2-20260902-020216-937662-lab/qa-artifacts/qa/logs/stream-lease-redaction-session.md; /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-review-r2-20260902-020216-937662-lab/qa-artifacts/qa/screenshots/dora-recorded-private-input.png; docs/qa/reports/2026-09-01-integrated-terminal-review-r2.md
last_report: docs/qa/reports/2026-09-01-integrated-terminal-review-r2.md
overlaps:
---

Retired 2026-09-04: the product no longer has an agent/operator control handoff or terminal-scoped
typing grant. `ET-terminal-shared-control` owns the replacement behavior. Historical evidence remains
linked here because this file is durable QA memory.

qa-impact: 2026-09-01 deep-review round 2 changed prompt recovery, secret answer concurrency,
terminal action guards, and controller cleanup. Reset for a focused agent handoff and input re-walk.

Flagged by integrated-terminal task 06. Task 10 owns the real-user walk, evidence, and verdict.

Walk:

1. Exercise `compozy__terminal_exec` and `compozy__terminal_open`, then use `compozy__terminal_list`, `compozy__terminal_read`, and `compozy__terminal_wait` to watch live output without taking control.
2. Use `compozy__terminal_claim`, take control from a second client, verify presence and control announcements, then use `compozy__terminal_yield` to release it.
3. Grant typing for that terminal, exercise `compozy__terminal_write`, verify follow-up writes do not prompt, and verify another terminal does.
4. Exercise `compozy__terminal_request_input`; answer one hidden request and reject another, then confirm only the length marker reaches the stream.
5. Exercise `compozy__terminal_signal` and `compozy__terminal_close`; verify their results and final terminal state agree.

2026-09-02 re-walk: passed after remediation. A fresh managed agent routed private input through a
running terminal, the runtime refused ordinary visible-shell input, and a hidden foreground reader
accepted one answer and one decline without exposing the value to chat, screen, journal, or quote.
