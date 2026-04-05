---
status: completed
domain: Documentation
type: Documentation
scope: Full
complexity: low
dependencies:
  - task_01
  - task_02
  - task_04
---

# Task 6: Skill and Documentation Updates

## Overview

Bring every piece of documentation and every installable skill up to date with the v2 schema: update `skills/cy-create-tasks` to instruct LLMs to read `.compozy/config.toml` for allowed types and to run `compozy validate-tasks` after generation, update the task template and schema reference, and update the project README + CLI help strings so users see the new commands and flags. This task closes the loop — without it, new task files will still be produced with the old shape.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- MUST update `skills/cy-create-tasks/SKILL.md` to (a) add a new first step instructing the LLM to read `.compozy/config.toml` and extract `[tasks].types`, falling back to the documented built-in list when absent, and (b) add a new final step mandating `compozy validate-tasks --name <feature>` until exit 0
- MUST update `skills/cy-create-tasks/references/task-template.md` to show the v2 frontmatter shape (add `title`, remove `domain`, remove `scope`, keep `status`/`type`/`complexity`/`dependencies`)
- MUST update `skills/cy-create-tasks/references/task-context-schema.md` to document the new `Title` field, the enum-constrained `Type` field with reference to the 8 built-in defaults, and to explicitly remove `Domain` and `Scope`
- MUST update `README.md` to describe the v2 task frontmatter and mention the new `validate-tasks` command + `--skip-validation`/`--force` flags on `start`
- MUST update CLI help strings in `internal/cli/root.go` for the start command so the new flags have clear descriptions
- MUST NOT introduce any new references to `domain` or `scope` in documentation (grep must return zero hits in `skills/` and `README.md` after this task)
- SHOULD keep the skill's existing tone and structure — only the affected sections should change
</requirements>

## Subtasks
- [x] 6.1 Update `skills/cy-create-tasks/SKILL.md`: add "Load type registry" step and "Run validate-tasks" step; remove references to `domain`/`scope`.
- [x] 6.2 Update `skills/cy-create-tasks/references/task-template.md` to the v2 frontmatter example.
- [x] 6.3 Update `skills/cy-create-tasks/references/task-context-schema.md` with the v2 field definitions and the 8 built-in types.
- [x] 6.4 Update `README.md` with a "Task Schema v2" subsection + mention of `validate-tasks` and new `start` flags.
- [x] 6.5 Update CLI help strings for `newStartCommand()` in `internal/cli/root.go` to describe `--skip-validation` and `--force`.
- [x] 6.6 Grep the repo for remaining `domain:` / `scope:` references in docs and skills and remove or migrate them.

## Implementation Details

The `SKILL.md` workflow currently lists 5 steps (Load context → Break down → Present breakdown → Generate files → Enrich). Insert a new first step before "Load context": **"Read `.compozy/config.toml`. If it contains `[tasks].types`, use that list as the allowed `type` values. Otherwise use the built-in defaults: `frontend, backend, docs, test, infra, refactor, chore, bugfix`."** Insert a new step after "Enrich each task file": **"Run `compozy validate-tasks --name <feature>`. If it exits non-zero, fix the reported issues and re-run. Do not mark the skill complete until it exits 0."**

In `task-template.md`, replace the frontmatter block (lines 5-15) with the v2 shape — show `title`, keep `status`, `type`, `complexity`, `dependencies`, drop `domain` and `scope`. Update the inline example comments to reference the 8 built-in types.

In `task-context-schema.md`, rewrite the "Required Fields" list (lines 5-12): add `title`, remove `domain` and `scope`, reword `type` to reference the enum + config override. Keep `Status Values`, `File Naming`, and `Parser Compatibility` sections intact.

For `README.md`, add or update the section describing task metadata. If the README doesn't currently document tasks deeply, add a short "Task Schema" subsection with a v2 example and a one-line pointer to `compozy validate-tasks`.

CLI help strings live in `internal/cli/root.go` near the start command flag registrations (lines 154-174). Extend the existing `cmd.Flags().BoolVar(...)` calls with clear, one-line descriptions.

Refer to the approved TechSpec "Impact Analysis" for the full file list.

### Relevant Files
- `skills/cy-create-tasks/SKILL.md` — insert 2 new workflow steps + remove `domain`/`scope` references.
- `skills/cy-create-tasks/references/task-template.md` — update frontmatter example to v2.
- `skills/cy-create-tasks/references/task-context-schema.md` — rewrite field definitions for v2.
- `README.md` — add/update task schema section and mention new command/flags.
- `internal/cli/root.go` (lines 154-174) — update flag help strings for `--skip-validation` and `--force`.

### Dependent Files
- `.compozy/tasks/tasks-changes/*.md` (these task files) — they themselves are in v1 format and will be migrated to v2 by task_03.
- Any other skills under `skills/` that reference task metadata (grep-audit in subtask 6.6).
- Any additional in-repo guides that reference the old schema (there is no `ai-docs/` directory at the repo root; audit whatever markdown exists).

### Related ADRs
- [ADR-001: Task Metadata Schema v2](adrs/adr-001.md) — What's documented in the skill and template.
- [ADR-002: Task Type Taxonomy](adrs/adr-002.md) — The config-reading step and 8 built-in defaults.
- [ADR-003: Validation Command Architecture](adrs/adr-003.md) — The `validate-tasks` enforcement step.

## Deliverables
- Updated `cy-create-tasks` SKILL.md, task-template.md, task-context-schema.md.
- Updated `README.md` covering v2 schema and new commands.
- Updated CLI help strings for `start` flags.
- Repo-wide grep for `domain:` / `scope:` in `skills/` and `README.md` returns zero hits.
- Documentation linting (if configured) passes **(REQUIRED)**.

## Tests
- Unit tests:
  - [x] Snapshot test (or golden file) of the rendered `compozy start --help` output showing `--skip-validation` and `--force` descriptions.
  - [x] Grep-based test (executable via `go test` using `os.ReadFile` over `skills/` + `README.md`) asserting zero occurrences of `^domain:` and `^scope:` on any line.
- Integration tests:
  - [x] Using the existing Cobra help-text harness at `internal/cli/root_test.go:200-218,928-935`, capture `compozy start --help` output and assert it contains `--skip-validation` and `--force`.
  - [x] Read `skills/cy-create-tasks/SKILL.md` via `os.ReadFile` and assert both the new "read .compozy/config.toml" step and the "run compozy validate-tasks" step are present by substring match.
- Test coverage target: >=80% (help strings are thin; the key coverage is the grep test and the CLI help assertion)
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `make verify` passes (fmt + lint + test + build with zero issues)
- Repo-wide grep for `^domain:` and `^scope:` in `skills/` and `README.md` returns zero hits
- `compozy start --help` shows descriptions for both new flags
- The two new workflow steps are present in `skills/cy-create-tasks/SKILL.md`
