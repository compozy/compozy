---
title: Built-in QA workflow extension
type: feature
---

Compozy now ships a built-in `cy-qa-workflow` extension that automatically attaches QA-planning and QA-execution tasks to any PRD-driven workflow, with curated runtimes per task. The extension lives at `extensions/cy-qa-workflow/` and follows the same on-disk contract as user extensions, so it can be customized or replaced project-by-project.

When enabled, every `compozy tasks run <slug>` over a PRD-mode workflow ends up with two extra tasks at the tail of `_tasks.md`:

| Task                                                   | Purpose                                                                                                                                | Type   | Complexity |
| ------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------- | ------ | ---------- |
| `<Workflow> QA plan and regression artifacts`          | Generates feature-level test plans, execution-ready test cases, and regression suites under `.compozy/tasks/<workflow>/qa/`            | `docs` | `high`     |
| `<Workflow> QA execution and operator-flow validation` | Executes the generated plan, files bug reports for confirmed failures, fixes root causes, and finishes only after `make verify` passes | `test` | `critical` |

The execution task depends on the report task; the report task depends on every other implementation task in the workflow, so QA always runs last.

### Curated runtimes

The extension also pins per-task runtimes via the new `plan.pre_resolve_task_runtime` hook so each QA task runs on the IDE/model best suited to it — no manual `--task-runtime` needed:

| QA task      | IDE      | Model     | Reasoning effort |
| ------------ | -------- | --------- | ---------------- |
| QA report    | `claude` | `opus`    | `xhigh`          |
| QA execution | `codex`  | `gpt-5.5` | `xhigh`          |

Override on a per-run basis with `--task-runtime`, or per-project via `[[tasks.run.task_runtime_rules]]`.

### Prompt augmentation

`cy-qa-workflow` also patches the agent session at create time:

- The QA execution prompt is prefixed with `/goal …` so the agent enters goal-driven mode and only finishes after `make verify` passes.
- The QA report prompt sets `CLAUDE_CODE_EFFORT_LEVEL=xhigh` in the session env to lift Claude's effort ceiling for plan generation.

### Manifest

```toml
# extensions/cy-qa-workflow/extension.toml
[extension]
name = "cy-qa-workflow"
version = "0.1.0"
description = "Adds Compozy QA report and QA execution tasks to workflow runs"
min_compozy_version = "0.1.10"

[subprocess]
command = "go"
args = ["run", "."]

[security]
capabilities = ["plan.mutate", "agent.mutate", "tasks.read", "tasks.create"]

[[hooks]]
event = "plan.pre_discover"
required = true

[[hooks]]
event = "plan.pre_resolve_task_runtime"
required = true

[[hooks]]
event = "agent.pre_session_create"
required = true
```

### Idempotency

- Tasks are detected by HTML markers (`<!-- compozy-qa-workflow:qa-report -->` / `<!-- compozy-qa-workflow:qa-execution -->`) plus title/type heuristics, so re-running the workflow does not duplicate them.
- `update_index = true` is set on the new `host.tasks.create` request, so the entries appear in `_tasks.md` in the right order on first run.

### SDK additions used by the extension

- `TaskCreateRequest.UpdateIndex` (`update_index` in JSON / TS) — when `true`, the host appends the created task to `_tasks.md`. Documented in `docs/extensibility/host-api-reference.md`.
- `TaskFrontmatter.Dependencies` — extensions can now seed task dependencies directly when creating a task.
- `SessionRequest` / `ResumeSessionRequest` now use a stable readable JSON contract (prompts are plain strings, not base64), matching the runtime-side ACP contract used by hook payloads and patches.
