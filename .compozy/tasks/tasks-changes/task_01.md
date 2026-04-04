---
status: pending
domain: Core Runtime
type: Feature Implementation
scope: Full
complexity: high
dependencies: []
---

# Task 1: Schema Foundation — TypeRegistry, [tasks] Config, Metadata v2

## Overview

Establish the data foundation for the typed-task system: a new `internal/core/tasks` package holding the `TypeRegistry` + 8 built-in types, a new `[tasks].types` section in workspace config, and the v2 task metadata schema (add `Title`, drop `Domain`/`Scope`). This task is the root dependency of every downstream task — validator, migrate, preflight, TUI, and skill updates all rely on these types and fields existing.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- MUST create a new `internal/core/tasks` package exporting `BuiltinTypes` (`frontend, backend, docs, test, infra, refactor, chore, bugfix`), `TypeRegistry` struct, `NewRegistry([]string) (*TypeRegistry, error)`, `(*TypeRegistry).IsAllowed(string) bool`, `(*TypeRegistry).Values() []string` (see TechSpec "Core Interfaces")
- MUST validate type slugs against regex `^[a-z][a-z0-9_-]{1,31}$` and reject duplicates and empty lists at construction time
- MUST extend `workspace.ProjectConfig` with a `Tasks TasksConfig` field carrying `Types *[]string` under TOML section `[tasks]`
- MUST wire `workspace.ProjectConfig.Validate()` (config.go:153) to validate the new section and surface clear error messages consistent with existing validators
- MUST add `Title string` to `model.TaskFileMeta` and `model.TaskEntry` (model.go:171-188)
- MUST remove `Domain` and `Scope` from both `TaskFileMeta` and `TaskEntry` — update every caller that reads these fields
- MUST update `prompt.ParseTaskFile` (common.go:51-73) to populate `Title` from frontmatter and return a new `ErrV1TaskMetadata` sentinel when frontmatter contains legacy `scope` or `domain` keys
- MUST NOT validate `type` against the registry inside the parser (that responsibility belongs to task_02's validator); the parser only reads the raw string
- SHOULD keep `TasksConfig.Types == nil` meaning "use built-in defaults" and `len(*TasksConfig.Types) == 0` being a validation error (ADR-002)
</requirements>

## Subtasks
- [ ] 1.1 Create `internal/core/tasks/types.go` with `BuiltinTypes`, `TypeRegistry`, and constructors; add table-driven tests.
- [ ] 1.2 Extend `workspace.ProjectConfig` with `Tasks TasksConfig` and add the `[tasks].types` TOML section with validation (duplicates, slug format, empty list).
- [ ] 1.3 Update `model.TaskFileMeta` and `model.TaskEntry` to add `Title`, remove `Domain` and `Scope`.
- [ ] 1.4 Update `prompt.ParseTaskFile` to populate `Title` and return `ErrV1TaskMetadata` when legacy `scope`/`domain` keys are present in the frontmatter.
- [ ] 1.5 Update every caller of `TaskEntry.Domain` / `TaskEntry.Scope` (notably `internal/core/prompt/prd.go` and `internal/core/migrate.go`) to stop reading the removed fields.
- [ ] 1.6 Extend `workspace/config_test.go`, `model/model_test.go`, and `prompt/prompt_test.go` with table-driven cases covering the new fields and errors.

## Implementation Details

Create the package `internal/core/tasks` and place `types.go` + `types_test.go` there. Put only pure data/validator helpers in this package — no workspace or prompt imports — so that downstream packages can import it without cycles.

Extend `workspace.ProjectConfig` by adding one struct (`TasksConfig`) and one validation function (`validateTasks`) that mirrors the existing validator pattern at `workspace/config.go:169-213`. Decode behavior (`DisallowUnknownFields`) at `config.go:142` already flags typos.

In `model`, renaming fields is a compile-time propagation. After removing `Domain`/`Scope`, run the compiler to surface every reader and update each site. See TechSpec "Data Models" for the new struct shapes.

In `prompt.ParseTaskFile`, detect v1 frontmatter by decoding into an intermediate map (via `yaml.v3` `Node` or a second struct with `Scope`/`Domain` kept) and checking for presence of those keys. Return `ErrV1TaskMetadata` without further processing so that the migrate command can route such files to the v1→v2 pass.

Refer to TechSpec "Core Interfaces" for the exact `TypeRegistry` contract and to TechSpec "Data Models" for the frontmatter shape.

### Relevant Files
- `internal/core/tasks/types.go` — NEW package, holds `BuiltinTypes`, `TypeRegistry`, constructors.
- `internal/core/tasks/types_test.go` — NEW, table-driven registry tests.
- `internal/core/workspace/config.go` (lines 26-59, 153-213) — extend `ProjectConfig`, add `TasksConfig` + validator.
- `internal/core/workspace/config_test.go` — extend with `[tasks].types` TOML parsing cases.
- `internal/core/model/model.go` (lines 171-188) — add `Title`, remove `Domain`/`Scope` from `TaskFileMeta`/`TaskEntry`.
- `internal/core/model/model_test.go` — update tests that reference removed fields.
- `internal/core/prompt/common.go` (lines 51-73) — update `ParseTaskFile`, add `ErrV1TaskMetadata` sentinel.
- `internal/core/prompt/prompt_test.go` — extend to cover new fields + v1 detection error.

### Dependent Files
- `internal/core/prompt/prd.go` — currently reads `task.Domain`, `task.Scope` in the PRD prompt builder; must stop.
- `internal/core/migrate.go` (lines 243-250) — the legacy→v1 migration currently writes `Domain`/`Scope` into `TaskFileMeta`; must stop writing those keys (and task_03 will layer v1→v2 on top).
- `internal/core/prompt/common.go` (legacy parser at lines 75-102) — still populates `TaskEntry.Domain`/`.Scope`; those assignments must be removed along with the struct fields.

### Related ADRs
- [ADR-001: Task Metadata Schema v2](adrs/adr-001.md) — Rationale for adding `Title` and removing `Domain`/`Scope`.
- [ADR-002: Task Type Taxonomy](adrs/adr-002.md) — Rationale for the enum-based `TypeRegistry` and the `[tasks].types` config section.

## Deliverables
- New `internal/core/tasks` package with `TypeRegistry` and `BuiltinTypes` exported.
- `workspace.ProjectConfig` extended with the `[tasks]` section and validation.
- v2 `TaskFileMeta` / `TaskEntry` with `Title` (no `Domain`/`Scope`).
- `prompt.ParseTaskFile` populates `Title` and rejects v1 frontmatter via `ErrV1TaskMetadata`.
- All prior readers of `Domain`/`Scope` updated.
- Unit tests with 80%+ coverage **(REQUIRED)**.
- Integration test proving a v2 task file parses end-to-end into `TaskEntry` with the new `Title` value **(REQUIRED)**.

## Tests
- Unit tests:
  - [ ] `NewRegistry(nil)` returns a registry whose `Values()` equals `BuiltinTypes` sorted.
  - [ ] `NewRegistry([]string{"frontend","backend"})` returns a registry whose `Values()` equals `["backend","frontend"]`.
  - [ ] `NewRegistry([]string{"frontend","frontend"})` returns a duplicate error mentioning `"frontend"`.
  - [ ] `NewRegistry([]string{"Invalid Slug"})` returns a slug-format error mentioning `"Invalid Slug"`.
  - [ ] `NewRegistry([]string{})` returns an empty-list error.
  - [ ] `(*TypeRegistry).IsAllowed("backend")` returns true; `IsAllowed("nope")` returns false.
  - [ ] `workspace.LoadConfig` with `[tasks].types = []` returns a validation error.
  - [ ] `workspace.LoadConfig` with `[tasks].types = ["frontend","frontend"]` returns a duplicate error.
  - [ ] `workspace.LoadConfig` with `[tasks].types = ["frontend","backend"]` populates `ProjectConfig.Tasks.Types` with exactly those values.
  - [ ] `workspace.LoadConfig` with no `[tasks]` section leaves `ProjectConfig.Tasks.Types == nil`.
  - [ ] `prompt.ParseTaskFile` with v2 frontmatter (containing `title` and no `scope`/`domain`) populates `TaskEntry.Title` and returns no error.
  - [ ] `prompt.ParseTaskFile` with v1 frontmatter (containing `scope` or `domain`) returns `errors.Is(err, prompt.ErrV1TaskMetadata) == true`.
  - [ ] `prompt.ParseTaskFile` with v2 frontmatter missing `title` still succeeds (parser is not the validator); `TaskEntry.Title == ""`.
- Integration tests:
  - [ ] Load a fixture `.compozy/config.toml` declaring `[tasks].types = ["mobile","api"]`, resolve workspace, assert `ProjectConfig.Tasks.Types` equals those values.
  - [ ] Parse a v2 task file (title+type+complexity+deps) from a `t.TempDir()` fixture and assert all fields populate correctly.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `make verify` passes (fmt + lint + test + build with zero issues)
- `internal/core/tasks` package compiles without importing `workspace`, `prompt`, `run`, or `cli` (no cycles)
- Every grep for `TaskEntry.Domain` / `TaskEntry.Scope` / `TaskFileMeta.Domain` / `TaskFileMeta.Scope` in the repo returns zero hits after this task
