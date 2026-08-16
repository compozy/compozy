---
id: RT-web-attention-bell-jump
area: RT
title: Open the correct session from the attention bell
persona: Cora
journey: J-respond-to-agent-attention
expected: The bell separates Needs you from Finished, counts exact cross-workspace session needs-you truth plus the existing pending task approvals, preserves task-approval rows and honest quiet or disconnected states, and activation opens the named task or focuses the named session after any required workspace switch.
entry_points: web OS shell attention bell; pending task approval row; session needs-you or finished row
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-session-attention-catalog; ET-web-session-cross-workspace-confirm
---

Walk populated, quiet, stale, muted, 100-plus-row, same-workspace, and foreign-workspace states. Keep
one pending task approval beside the new session rows and prove its row, count, and task landing did
not regress.
Confirm a stale or already-resolved row still lands honestly and that a Finished arrival clears by
presence rather than by a manual dismiss control.

QA impact 2026-08-16: Task 03 added the web bell and shared cross-workspace jump. Flag only; Task 08
owns the real-user walk and evidence.
