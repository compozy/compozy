---
title: Safer parallel task and multi-run worktree lifecycle
type: fix
---

Parallel execution no longer surprises you with worktrees or silent merges. Activation is explicit, results are retained on named branches, and cleanup refuses to delete uncommitted or unretained output.

### Explicit activation for `--parallel-tasks`

`[tasks.run.parallel] enabled` is now a compatibility field only — it does **not** authorize worktree-backed execution by itself. A single-workflow run uses per-task worktrees and an integration branch only after an explicit per-run choice:

- `--parallel-tasks=true` (or the wizard)
- `runtime_overrides.parallel_tasks.enabled=true`
- `--parallel-tasks=false` keeps the standard serial runner

### Named result branches and safe cleanup

Multi-spec (`--multiple --parallel`) children get deterministic `compozy/multi-*` result branches. Output is **never auto-merged** into the user's branch. After settlement:

- a clean worktree can be removed while committed output stays on the result branch
- an empty result branch is deleted when it still points at the base
- dirty trees or output without a proven retention point stay `preserved` with an explicit reason

`compozy runs purge` applies the same ownership, dirty-tree, and commit-retention checks.

### Single-workflow waves

Within one workflow, successful dependency-wave output is squash-merged into a temporary integration branch and fast-forwarded only if every required task succeeds. On failure the user's branch is unchanged; partial output and unsafe trees are preserved for inspection.

### Clearer handoff and wizard

The CLI prints the resolved execution kind, whether worktrees are used, and the source of that choice before starting. Stream/TUI handoff now includes `result_branch`, `worktree_status`, and `worktree_reason` so every retained or preserved tree is locatable.
