---
id: LP-terminal-completion-time
area: LP
title: Show when a Loop run actually ended
persona: Ada
journey: J-loop-terminal-recovery
expected: Live runs omit completed_at, terminal runs expose the exact terminal transition time across CLI, HTTP, UDS, native tools, and Web duration, and rerun clears the completion time until the run ends again.
entry_points: compozy loop status -o json; GET /api/workspaces/:workspace_id/loop-runs/:run_id; compozy__loop_status; Web Runs
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-terminal-loop-settlement; LP-time-travel-rerun
---

Compare the terminal status-change event timestamp with `completed_at`, refresh the public surfaces,
and confirm the displayed terminal duration remains stable instead of advancing with wall time.
