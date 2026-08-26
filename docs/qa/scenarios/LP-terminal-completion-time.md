---
id: LP-terminal-completion-time
area: LP
title: Show when a Loop run actually ended
persona: Ada
journey: J-loop-terminal-recovery
expected: Live runs omit completed_at, terminal runs expose the exact terminal transition time across CLI, HTTP, UDS, native tools, and Web duration, and rerun clears the completion time until the run ends again.
entry_points: compozy loop status -o json; GET /api/workspaces/:workspace_id/loop-runs/:run_id; compozy__loop_status; Web Runs
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-loop-issues-fixes-20260826-192350-462800-lab/qa-artifacts/qa/logs/halt-run-status-pinned.json; /Users/pedronauck/dev/qa-labs/compozy-loop-issues-fixes-20260826-192350-462800-lab/qa-artifacts/qa/logs/halt-run-api.json; /Users/pedronauck/dev/qa-labs/compozy-loop-issues-fixes-20260826-192350-462800-lab/qa-artifacts/qa/logs/halt-run-native.json; /Users/pedronauck/dev/qa-labs/compozy-loop-issues-fixes-rerun-20260826-200713-569291-lab/qa-artifacts/qa/logs/terminal-status-current-head.json; /Users/pedronauck/dev/qa-labs/compozy-loop-issues-fixes-rerun-20260826-200713-569291-lab/qa-artifacts/qa/logs/api-status-current-head.json; /Users/pedronauck/dev/qa-labs/compozy-loop-issues-fixes-rerun-20260826-200713-569291-lab/qa-artifacts/qa/logs/web-terminal-current-head.json
last_report: docs/qa/reports/2026-08-26-loop-issues-fixes.md
overlaps: LP-terminal-loop-settlement; LP-time-travel-rerun
---

Compare the terminal status-change event timestamp with `completed_at`, refresh the public surfaces,
and confirm the displayed terminal duration remains stable instead of advancing with wall time.

QA 2026-08-26: explicit rerun omitted `completed_at` while generation 2 was live, then exposed the
new terminal timestamp after failure. CLI, HTTP, and the native status tool agreed, and Web kept the
duration at `1m 13s` across refresh while the relative age advanced.
