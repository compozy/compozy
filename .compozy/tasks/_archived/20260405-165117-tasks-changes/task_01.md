---
status: completed
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
- MUST extend the existing `internal/core/tasks` package (already contains `store.go` / `store_test.go`) by adding `types.go` exporting `BuiltinTypes` (`frontend, backend, docs, test, infra, refactor, chore, bugfix`), `TypeRegistry` struct, `NewRegistry([]string) (*TypeRegistry, error)`, `(*TypeRegistry).IsAllowed(string) bool`, `(*TypeRegistry).Values() []string` (see TechSpec "Core Interfaces")
- MUST validate type slugs against regex `^[a-z][a-z0-9_-]{1,31}$` and reject duplicates and empty lists at construction time
- MUST extend `workspace.ProjectConfig` with a `Tasks TasksConfig` field carrying `Types *[]string` under TOML section `[tasks]`
- MUST wire `workspace.ProjectConfig.Validate()` (config.go:153) to validate the new section and surface clear error messages consistent with existing validators
- MUST add `Title string` to `model.TaskFileMeta` and `model.TaskEntry` (model.go:171-188)
- MUST remove `Domain` and `Scope` from both `TaskFileMeta` and `TaskEntry` — update every caller that reads these fields
- MUST update `prompt.ParseTaskFile` (common.go:51-73) to populate `Title` from frontmatter and return a new `ErrV1TaskMetadata` sentinel when frontmatter contains legacy `scope` or `domain` keys
- MUST update every existing call site that special-cases `ErrLegacyTaskMetadata` so it also routes `ErrV1TaskMetadata` correctly. The known sites are: `internal/core/migrate.go:210`, `internal/core/plan/input.go:309`, `internal/core/tasks/store.go:226`. Missing any of them means `compozy start` or `compozy migrate` will mis-handle v1 files between task_01 and task_03
- MUST NOT validate `type` against the registry inside the parser (that responsibility belongs to task_02's validator); the parser only reads the raw string
- SHOULD keep `TasksConfig.Types == nil` meaning "use built-in defaults" and `len(*TasksConfig.Types) == 0` being a validation error (ADR-002)
</requirements>

## Subtasks
- [x] 1.1 Add `internal/core/tasks/types.go` (new file in existing package) with `BuiltinTypes`, `TypeRegistry`, and constructors; add table-driven tests in `internal/core/tasks/types_test.go`.
- [x] 1.2 Extend `workspace.ProjectConfig` with `Tasks TasksConfig` and add the `[tasks].types` TOML section with validation (duplicates, slug format, empty list).
- [x] 1.3 Update `model.TaskFileMeta` and `model.TaskEntry` to add `Title`, remove `Domain` and `Scope`.
- [x] 1.4 Update `prompt.ParseTaskFile` to populate `Title` and return `ErrV1TaskMetadata` when legacy `scope`/`domain` keys are present in the frontmatter.
- [x] 1.5 Update every caller of `TaskEntry.Domain` / `TaskEntry.Scope` to stop reading the removed fields: `internal/core/prompt/common.go:60-67,93-97`, `internal/core/prompt/prd.go:41-45`, `internal/core/migrate.go:243-249`.
- [x] 1.5b Update every `errors.Is(err, prompt.ErrLegacyTaskMetadata)` site to also handle the new `ErrV1TaskMetadata` sentinel: `internal/core/migrate.go:210`, `internal/core/plan/input.go:309`, `internal/core/tasks/store.go:226`.
- [x] 1.6 Extend `workspace/config_test.go`, `model/model_test.go`, and `prompt/prompt_test.go` with table-driven cases covering the new fields and errors.

## Implementation Details

Create the package `internal/core/tasks` and place `types.go` + `types_test.go` there. Put only pure data/validator helpers in this package — no workspace or prompt imports — so that downstream packages can import it without cycles.

Extend `workspace.ProjectConfig` by adding one struct (`TasksConfig`) and one validation function (`validateTasks`) that mirrors the existing validator pattern at `workspace/config.go:169-213`. Decode behavior (`DisallowUnknownFields`) at `config.go:142` already flags typos.

In `model`, renaming fields is a compile-time propagation. After removing `Domain`/`Scope`, run the compiler to surface every reader and update each site. See TechSpec "Data Models" for the new struct shapes.

In `prompt.ParseTaskFile`, detect v1 frontmatter by decoding into an intermediate map (via `yaml.v3` `Node` or a second struct with `Scope`/`Domain` kept) and checking for presence of those keys. Return `ErrV1TaskMetadata` without further processing so that the migrate command can route such files to the v1→v2 pass.

Refer to TechSpec "Core Interfaces" for the exact `TypeRegistry` contract and to TechSpec "Data Models" for the frontmatter shape.

### Relevant Files
- `internal/core/tasks/types.go` — NEW file in the existing `tasks` package; holds `BuiltinTypes`, `TypeRegistry`, constructors.
- `internal/core/tasks/types_test.go` — NEW, table-driven registry tests.
- `internal/core/workspace/config.go` (lines 26-59, 153-213) — extend `ProjectConfig`, add `TasksConfig` + validator.
- `internal/core/workspace/config_test.go` — extend with `[tasks].types` TOML parsing cases.
- `internal/core/model/model.go` (lines 171-188) — add `Title`, remove `Domain`/`Scope` from `TaskFileMeta`/`TaskEntry`.
- `internal/core/model/model_test.go` — update tests that reference removed fields.
- `internal/core/prompt/common.go` (lines 20-22, 51-73) — update `ParseTaskFile`, add `ErrV1TaskMetadata` sentinel alongside the existing `ErrLegacyTaskMetadata`.
- `internal/core/prompt/prompt_test.go` (lines 130-199) — extend to cover new fields + v1 detection error.

### Dependent Files
- `internal/core/prompt/prd.go` (lines 41-45) — currently reads `task.Domain`, `task.Scope` in the PRD prompt builder; must stop.
- `internal/core/migrate.go` (lines 243-250) — the legacy→v1 migration currently writes `Domain`/`Scope` into `TaskFileMeta`; must stop writing those keys (and task_03 will layer v1→v2 on top).
- `internal/core/migrate.go` (line 210) — `errors.Is(err, prompt.ErrLegacyTaskMetadata)` gate; must also accept `ErrV1TaskMetadata`.
- `internal/core/plan/input.go` (line 309) — `errors.Is(err, prompt.ErrLegacyTaskMetadata)` gate; must also accept `ErrV1TaskMetadata` so `compozy start` recognizes v1 files.
- `internal/core/tasks/store.go` (lines 92-95, 213-229, especially line 226) — `errors.Is(err, prompt.ErrLegacyTaskMetadata)` gate; must also accept `ErrV1TaskMetadata` so task refresh/meta computation handles v1 files.
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
  - [x] `NewRegistry(nil)` returns a registry whose `Values()` equals `BuiltinTypes` sorted.
  - [x] `NewRegistry([]string{"frontend","backend"})` returns a registry whose `Values()` equals `["backend","frontend"]`.
  - [x] `NewRegistry([]string{"frontend","frontend"})` returns a duplicate error mentioning `"frontend"`.
  - [x] `NewRegistry([]string{"Invalid Slug"})` returns a slug-format error mentioning `"Invalid Slug"`.
  - [x] `NewRegistry([]string{})` returns an empty-list error.
  - [x] `(*TypeRegistry).IsAllowed("backend")` returns true; `IsAllowed("nope")` returns false.
  - [x] `workspace.LoadConfig` with `[tasks].types = []` returns a validation error.
  - [x] `workspace.LoadConfig` with `[tasks].types = ["frontend","frontend"]` returns a duplicate error.
  - [x] `workspace.LoadConfig` with `[tasks].types = ["frontend","backend"]` populates `ProjectConfig.Tasks.Types` with exactly those values.
  - [x] `workspace.LoadConfig` with no `[tasks]` section leaves `ProjectConfig.Tasks.Types == nil`.
  - [x] `prompt.ParseTaskFile` with v2 frontmatter (containing `title` and no `scope`/`domain`) populates `TaskEntry.Title` and returns no error.
  - [x] `prompt.ParseTaskFile` with v1 frontmatter (containing `scope` or `domain`) returns `errors.Is(err, prompt.ErrV1TaskMetadata) == true`.
  - [x] `prompt.ParseTaskFile` with legacy XML markers still returns `errors.Is(err, prompt.ErrLegacyTaskMetadata) == true` (regression guard — legacy path unchanged).
  - [x] `prompt.ParseTaskFile` with v2 frontmatter missing `title` still succeeds (parser is not the validator); `TaskEntry.Title == ""`.
  - [x] Each of `internal/core/migrate.go:210`, `internal/core/plan/input.go:309`, and `internal/core/tasks/store.go:226` has a test that exercises the v1 branch and confirms the callsite does not silently misclassify.
- Integration tests:
  - [x] Load a fixture `.compozy/config.toml` declaring `[tasks].types = ["mobile","api"]`, resolve workspace, assert `ProjectConfig.Tasks.Types` equals those values.
  - [x] Parse a v2 task file (title+type+complexity+deps) from a `t.TempDir()` fixture and assert all fields populate correctly.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `make verify` passes (fmt + lint + test + build with zero issues)
- `internal/core/tasks/types.go` introduces no new imports beyond standard library, `fmt`, and `regexp`; does not import `workspace`, `run`, or `cli`
- Every grep for `TaskEntry.Domain` / `TaskEntry.Scope` / `TaskFileMeta.Domain` / `TaskFileMeta.Scope` in the repo returns zero hits after this task
- Every grep for `ErrLegacyTaskMetadata` in the repo is paired with an `ErrV1TaskMetadata` branch in the same expression or immediately adjacent
