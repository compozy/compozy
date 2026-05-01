---
title: Per-task runtime overrides on tasks run
type: feature
---

Pick a different IDE, model, or reasoning effort per task type — or per individual task — for a single `compozy tasks run` invocation, instead of running the whole batch on one global runtime. Selection is exposed three ways: a repeatable `--task-runtime` CLI flag, an interactive form on `tasks run`, and a `[[tasks.run.task_runtime_rules]]` TOML section. A new `plan.pre_resolve_task_runtime` extension hook lets extension authors resolve per-task runtime programmatically.

### CLI

`--task-runtime` is repeatable. Each value is a comma-separated rule with a selector (`id=` **or** `type=`) and at least one override (`ide=`, `model=`, `reasoning-effort=`).

```bash
# All frontend tasks → Codex with high reasoning, plus one task forced to xhigh
compozy tasks run multi-repo \
  --ide claude --model opus \
  --task-runtime "type=frontend,ide=codex,model=gpt-5.5,reasoning-effort=high" \
  --task-runtime "id=task_07,reasoning-effort=xhigh"
```

### TOML

Persistent type-scoped defaults live under `[[tasks.run.task_runtime_rules]]`. `id=` selectors are CLI/TUI-only by design — config rejects them.

```toml
[defaults]
ide = "codex"
model = "gpt-5.5"
reasoning_effort = "medium"

[[tasks.run.task_runtime_rules]]
type = "frontend"
model = "gpt-5.5"
reasoning_effort = "high"

[[tasks.run.task_runtime_rules]]
type = "docs"
ide = "claude"
model = "opus"
```

### Rule keys

| Key                                     | Where        | Description                                                                       |
| --------------------------------------- | ------------ | --------------------------------------------------------------------------------- |
| `id`                                    | CLI/TUI only | Match a single task by PRD task id                                                |
| `type`                                  | CLI/TUI/TOML | Match all tasks of this type (e.g. `frontend`, `docs`)                            |
| `ide`                                   | all          | `claude`, `codex`, `copilot`, `cursor-agent`, `droid`, `gemini`, `opencode`, `pi` |
| `model`                                 | all          | Any model accepted by the chosen IDE                                              |
| `reasoning-effort` / `reasoning_effort` | all          | `low`, `medium`, `high`, `xhigh`                                                  |

Each rule must have a selector and at least one override. Mixing `id` and `type` in a single rule is an error.

### Precedence (high → low at execution)

1. CLI/TUI `id=` rules
2. CLI/TUI `type=` rules
3. Config `[[tasks.run.task_runtime_rules]]` (type-only)
4. `[defaults]`

### Extension hook

Extension authors can resolve runtime programmatically via the new `plan.pre_resolve_task_runtime` hook (helper: `onPlanPreResolveTaskRuntime`). Later hooks (`plan.post_prepare_jobs`, `job.pre_execute`, `run.pre_start`) are now hard-guarded against runtime mutation for workflow runs — use the new hook instead.
