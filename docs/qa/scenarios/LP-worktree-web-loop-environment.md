---
id: LP-worktree-web-loop-environment
area: LP
title: Declare loop and node execution environments in the builder
persona: Bruno
journey: J-isolated-task-loop-execution
expected: Loop configure, run, and node Environment controls author only root or a named worktree, while API/CLI-authored directory and per-run values remain visible, read-only, and byte-for-byte preserved by unrelated saves or publishes.
entry_points: S12 Loop configure dialog -> Environment default; S13 loop builder -> node inspector -> Environment
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-09-01-pr-519-review-fixes/directory-read-only-disabled.png; docs/qa/evidence/2026-09-01-pr-519-review-fixes/per-run-read-only-disabled.png
last_report: docs/qa/reports/2026-09-01-pr-519-review-fixes.md
overlaps: LP-loop-environment-resolution
---

QA impact: Issue 512 removes directory and per-run from Web authoring without removing those API/CLI
contracts. The walk must prove both the reduced choice set and lossless preservation of a loaded
read-only value.

QA completion 2026-09-01: the live Run form offered only Inherit, Workspace root, and Named
worktree, then completed against the selected ready Worktree. A CLI-authored directory remained
read-only and byte-for-byte unchanged after an unrelated Human approval gate save and a fresh CLI
read. A subsequent CLI-authored per-run value rendered as `Per-run (read-only)`.

QA follow-up 2026-09-01: fresh Web loads rendered both CLI-authored `Directory (read-only)` and
`Per-run (read-only)` controls as disabled while preserving the directory value exactly.
