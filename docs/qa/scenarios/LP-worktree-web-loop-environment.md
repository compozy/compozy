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
evidence: docs/qa/evidence/2026-09-01-issue-512-loop-worktree-tools/directory-preserved-after-save.png
last_report: docs/qa/reports/2026-09-01-issue-512-loop-worktree-tools.md
overlaps: LP-loop-environment-resolution
---

QA impact: Issue 512 removes directory and per-run from Web authoring without removing those API/CLI
contracts. The walk must prove both the reduced choice set and lossless preservation of a loaded
read-only value.

QA completion 2026-09-01: the live Run form offered only Inherit, Workspace root, and Named
worktree, then completed against the selected ready Worktree. A CLI-authored directory remained
read-only and byte-for-byte unchanged after an unrelated Human approval gate save and a fresh CLI
read. A subsequent CLI-authored per-run value rendered as `Per-run (read-only)`.
