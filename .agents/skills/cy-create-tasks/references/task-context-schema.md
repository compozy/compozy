# Task Frontmatter Schema

Task metadata is parsed from YAML frontmatter by Compozy's `ParseTaskFile()` function in `internal/core/prompt/common.go`.

## Required Fields

- `status`: Task lifecycle state.
- `title`: Human-readable task title. It must match the first H1 in the task body.
- `type`: Allowed work type slug. Use `[tasks].types` from `.compozy/config.toml` when configured; otherwise use the built-in defaults `frontend`, `backend`, `docs`, `test`, `infra`, `refactor`, `chore`, `bugfix`.
- `complexity`: Difficulty rating. Must be one of: `low`, `medium`, `high`, `critical`.
- `dependencies`: YAML list of task file names that must be completed before this task. Use `[]` when there are no dependencies.

## Status Values

Valid `status` values:

- `pending` — task has not been started.
- `in_progress` — task is currently being worked on.
- `completed` — task is finished and verified.
- `done` — treated as completed.
- `finished` — treated as completed.

## File Naming

Task files must match the pattern `task_\d+\.md` with zero-padded numbers:
- `task_01.md`, `task_02.md`, `task_10.md`, `task_99.md`

The leading underscore prefix is reserved for meta documents:
- `_prd.md` — Product Requirements Document
- `_techspec.md` — Technical Specification
- `_tasks.md` — Master task list

## Parser Compatibility

Compozy reads task files matching the regex `^task_\d+\.md$`. Files with the old `_task_` prefix are not recognized. The file MUST start with YAML frontmatter for `ParseTaskFile()` to read the metadata.
