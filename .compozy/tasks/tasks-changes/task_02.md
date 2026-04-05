---
status: completed
domain: Validation
type: Feature Implementation
scope: Full
complexity: medium
dependencies:
  - task_01
---

# Task 2: Validator + compozy validate-tasks CLI Command

## Overview

Build the authoritative task-metadata validator as a pure function (`tasks.Validate`) returning a structured `Report`, pair it with a `FixPrompt` builder that produces an LLM-ready remediation prompt, and expose both through a new `compozy validate-tasks` Cobra command supporting `--format text|json`. This validator is the single source of truth consumed by the skill, the start preflight (task_04), and CI pipelines.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- MUST implement `tasks.Validate(ctx context.Context, tasksDir string, registry *TypeRegistry) (Report, error)` that returns I/O errors only for filesystem/parse failures and surfaces every schema violation as an entry in `Report.Issues`
- MUST check each of the 7 rules in ADR-003: title present, title/H1 sync (strip `Task N:` / `Task N -` prefix), type in registry, status in `{pending,in_progress,completed,blocked}`, complexity in `{low,medium,high,critical}` (or empty), dependencies reference existing task files in the same directory, frontmatter contains no legacy `scope`/`domain` keys
- MUST implement `tasks.FixPrompt(report Report, registry *TypeRegistry) string` that emits a deterministic, paste-ready prompt listing offending files, issues, and the allowed type list
- MUST add `compozy validate-tasks` as a new Cobra subcommand with flags `--name <feature>`, `--tasks-dir <path>`, `--format text|json`
- MUST use exit code 0 for clean runs, 1 for schema violations, and 2 for I/O or config errors
- MUST print human-readable text by default and emit structured JSON when `--format json` is set
- MUST NOT fail when the target directory has no task files (print a friendly "no tasks found" message, exit 0)
- SHOULD resolve the `TypeRegistry` once from `workspace.Resolve()` and pass it to `Validate` — no per-file reloading
</requirements>

## Subtasks
- [x] 2.1 Create `internal/core/tasks/validate.go` with `Issue`, `Report`, `Validate()`; table-driven tests covering each of the 7 rules.
- [x] 2.2 Implement the title/H1 sync check with prefix-stripping (`Task N:` / `Task N -`) and a golden-file test suite.
- [x] 2.3 Create `internal/core/tasks/fix_prompt.go` with `FixPrompt()`; golden-file test for the rendered prompt.
- [x] 2.4 Create `internal/cli/validate_tasks.go` implementing the Cobra command with `--name`, `--tasks-dir`, `--format` flags and exit-code contract.
- [x] 2.5 Register the new command in `internal/cli/root.go` alongside the existing subcommands (follow the `newMigrateCommand()` / `newSetupCommand()` pattern).
- [x] 2.6 Add an integration test that invokes the command against a `t.TempDir()` fixture and asserts exit code + JSON schema.

## Implementation Details

`tasks.Validate` is a pure function that walks `tasksDir` for `task_*.md` files, parses each with `prompt.ParseTaskFile`, and runs the 7 checks. It collects all issues for all files (does not short-circuit on first failure). When `prompt.ParseTaskFile` returns `ErrV1TaskMetadata` from task_01, the validator still emits an Issue (so the user learns that file must be migrated) rather than aborting.

The title/H1 sync rule compares the frontmatter `Title` against the first H1 in the body: extract `# Task N: <title>` / `# Task N - <title>` / `# <title>` and compare stripped values. Mismatch produces an Issue.

`tasks.FixPrompt` takes a `Report` and formats an LLM-directed message: it lists every offending file and its issues, restates the allowed type list, and gives clear "rewrite the frontmatter to …" instructions. Keep it under 40 source lines — it is a string-template function.

The CLI command lives at `internal/cli/validate_tasks.go`, follows the Cobra factory pattern used by `newSetupCommand()` / `newMigrateCommand()` in `root.go:87-95`, and resolves the workspace via the existing `workspace.Resolve()` helper. When validation fails, the command prints the fix prompt at the end of the output in both text and JSON modes (as a `fix_prompt` field in JSON).

Refer to TechSpec "Core Interfaces" for the `Report`/`Issue` contract and to ADR-003 for the command's UX contract.

### Relevant Files
- `internal/core/tasks/validate.go` — NEW, validator function + `Report` / `Issue` types.
- `internal/core/tasks/validate_test.go` — NEW, table-driven rule coverage.
- `internal/core/tasks/fix_prompt.go` — NEW, LLM prompt builder (<40 lines).
- `internal/core/tasks/fix_prompt_test.go` — NEW, golden-file test.
- `internal/cli/validate_tasks.go` — NEW, Cobra command.
- `internal/cli/root.go` (lines 87-95) — register new subcommand in `NewRootCommand`.

### Dependent Files
- `internal/cli/root.go` — command registration must be kept in order with other commands; help text updates happen in task_06.
- Downstream consumers (task_04 preflight, task_03 migrate reporting, task_06 skill) import `tasks.Validate` and `tasks.FixPrompt`.

### Related ADRs
- [ADR-003: Validation Command Architecture](adrs/adr-003.md) — Defines the 7 checks, exit codes, and output formats.
- [ADR-002: Task Type Taxonomy](adrs/adr-002.md) — `TypeRegistry` is consumed by the type check.
- [ADR-001: Task Metadata Schema v2](adrs/adr-001.md) — Title/H1 sync rule.

## Deliverables
- `tasks.Validate` function + `Report` / `Issue` types.
- `tasks.FixPrompt` function.
- `compozy validate-tasks` Cobra command with text and JSON output.
- Unit tests with 80%+ coverage **(REQUIRED)**.
- Integration test exercising the command end-to-end against a `t.TempDir()` fixture **(REQUIRED)**.

## Tests
- Unit tests:
  - [x] A task file with empty `title` frontmatter produces an `Issue{Field:"title"}`.
  - [x] A task file whose frontmatter `title` differs from its body H1 (after prefix stripping) produces `Issue{Field:"title_h1_sync"}`.
  - [x] A task file with `type: nope` against a registry containing only built-ins produces `Issue{Field:"type"}` listing the allowed values.
  - [x] A task file with `status: in-progress` (hyphenated) produces `Issue{Field:"status"}`.
  - [x] A task file with `complexity: extreme` produces `Issue{Field:"complexity"}`.
  - [x] A task file whose `dependencies: [task_99]` references a missing file produces `Issue{Field:"dependencies"}`.
  - [x] A task file containing legacy `scope:` or `domain:` frontmatter keys produces `Issue{Field:"scope"}` / `Issue{Field:"domain"}`.
  - [x] A clean v2 fixture produces `Report.OK() == true` with zero issues.
  - [x] Title/H1 sync accepts `# Task 1: ACP Agent Layer`, `# Task 1 - ACP Agent Layer`, and `# ACP Agent Layer` when frontmatter says `title: ACP Agent Layer`.
  - [x] `FixPrompt(report, registry)` includes every offending path, every `Issue.Message`, and the full `registry.Values()` list.
  - [x] `Validate` on an empty directory returns `Report{Scanned:0}` with no issues and no error.
- Integration tests:
  - [x] Run `compozy validate-tasks --tasks-dir <tempdir> --format json` on a mixed fixture (2 valid, 2 invalid files with one known issue each); assert exit code 1, JSON has `issues` array covering exactly the two offending file paths (assert by distinct `path` values, not array length), and a non-empty `fix_prompt` string.
  - [x] Run the command on an all-valid fixture; assert exit code 0 and text output contains `"all tasks valid"`.
  - [x] Run the command with a non-existent `--tasks-dir`; assert exit code 2 and a clear error message.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `make verify` passes (fmt + lint + test + build with zero issues)
- `compozy validate-tasks --help` prints the command documentation
- Exit codes follow the 0/1/2 contract exactly
