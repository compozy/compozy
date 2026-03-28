# Task Context XML Schema

The `<task_context>` XML block is parsed by Looper's `ParseTaskFile()` function in `internal/looper/prompt/common.go`. Each field is extracted via regex from the content between `<task_context>` and `</task_context>` tags.

## Required Fields

- `<domain>`: Feature area the task belongs to. Examples: "Authentication", "API", "Frontend", "Database", "Infrastructure", "CLI".
- `<type>`: Type of work. Examples: "Feature Implementation", "Bug Fix", "Refactor", "Configuration", "Migration".
- `<scope>`: Coverage of the task. Examples: "Full", "Partial".
- `<complexity>`: Difficulty rating. Must be one of: `low`, `medium`, `high`, `critical`.
- `<dependencies>`: Comma-separated task file names that must be completed before this task. Use `"none"` if there are no dependencies. Examples: `"task_01, task_02"`, `"none"`.

## Status Line

The `## status: <value>` heading must appear before the `<task_context>` block. Valid status values:

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

Looper reads task files matching the regex `^task_\d+\.md$`. Files with the old `_task_` prefix are not recognized. The file MUST start with `## status:` followed by the `<task_context>` block for proper parsing by `ParseTaskFile()`.
