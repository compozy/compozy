# Task Frontmatter Schema

Task metadata is parsed from YAML frontmatter by Compozy's `ParseTaskFile()` function in `internal/core/prompt/common.go`.

## Required Fields

- `status`: Task lifecycle state.
- `domain`: Feature area the task belongs to. Examples: "Authentication", "API", "Frontend", "Database", "Infrastructure", "CLI".
- `type`: Type of work. Examples: "Feature Implementation", "Bug Fix", "Refactor", "Configuration", "Migration".
- `scope`: Coverage of the task. Examples: "Full", "Partial".
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
