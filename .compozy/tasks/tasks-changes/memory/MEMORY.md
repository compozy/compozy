# Workflow Memory

Keep only durable, cross-task context here. Do not duplicate facts that are obvious from the repository, PRD documents, or git history.

## Current State

- Task metadata schema foundation is implemented in code: task types resolve through `internal/core/tasks.NewRegistry`, workspace config supports `[tasks].types`, and v2 task metadata uses `title` instead of `domain` / `scope`.
- `compozy migrate` now converts legacy and v1 task files to v2 in one pass, resolves the workspace task type registry for remapping, and skips workflow `memory/` trees plus task-workflow `_meta.md` files during scans.

## Shared Decisions

- `TasksConfig.Types == nil` means "use built-in defaults"; an explicit empty list is invalid at config load.
- `prompt.ParseTaskFile` returns `ErrV1TaskMetadata` when frontmatter contains `domain` or `scope`; parser-level type validation remains deferred to the validator task.
- Legacy and v1 task migrations now write explicit `title` / `type` fields via `frontmatter.Format(model.TaskFileMeta, body)`; unmapped legacy types are preserved as `type: ""` and surfaced through `MigrationResult.UnmappedTypeFiles` plus the migrate CLI fix prompt.

## Shared Learnings

- Normal task fixtures and tests now need `title` with no `domain` / `scope`; dedicated v1 fixtures should exist only where migration or error routing is under test.
- Workflow task-memory notes can live under `memory/task_*.md`; migration must ignore that subtree or it will misclassify memory files as real task artifacts.

## Open Risks

- The `tasks-changes` workflow artifacts are still on mixed metadata versions until the dedicated migration task updates the workflow files.

## Handoffs
