---
title: Dependency-aware parallel task execution
type: feature
---

`compozy tasks run <slug> --parallel-tasks` now executes the pending task files of a single PRD workflow **in dependency-aware waves** instead of one task at a time. Independent tasks in the same wave run concurrently, each in its own isolated git worktree, and dependent tasks wait for their prerequisites to finish.

### Starting a parallel-tasks run

```bash
# Run one workflow's tasks by dependency waves
compozy tasks run my-feature --parallel-tasks
```

`--parallel-tasks` targets a single workflow and cannot be combined with `--multiple` (that flag drives the separate multi-slug queue). It overrides the workspace `[tasks.run.parallel] enabled` value for one invocation.

### Task-graph manifest

Waves are computed from a `_tasks.md` task-graph manifest (`compozy.tasks/v2`) that describes the task nodes and their dependency edges. The manifest is the source of truth for which tasks may run together and which must wait, so ordering is explicit and reproducible rather than inferred at runtime.

### Configuration

Set defaults per workspace under `[tasks.run.parallel]`:

| Key                                      | Default | Meaning                                                                                    |
| ---------------------------------------- | ------- | ------------------------------------------------------------------------------------------ |
| `enabled`                                | `false` | Turn on dependency-aware parallel task execution                                           |
| `max_concurrency`                        | `4`     | Cap on concurrent task worktrees within a single wave                                      |
| `[tasks.run.parallel.conflict_resolver]` | —       | Agent (`ide`, `model`, `reasoning_effort`, `max_attempts`) used to resolve merge conflicts |

### Agentic conflict resolution

When concurrent task worktrees are squash-merged back and collide, a bounded **conflict-resolver agent** attempts to resolve the merge automatically before the run fails. You can override the resolver per invocation with the hidden `--parallel-conflict-resolver-ide`, `--parallel-conflict-resolver-model`, and `--parallel-conflict-resolver-reasoning` flags, or configure it under `[tasks.run.parallel.conflict_resolver]`.

### Notes

- The task-run wizard and CLI both understand parallel-tasks mode, and the run emits richer parallel plan/start and per-task failure events so the TUI and journal reflect wave progress.
- Worktree isolation means concurrent tasks never edit the same checkout; merges back to the workspace are serialized deterministically.
