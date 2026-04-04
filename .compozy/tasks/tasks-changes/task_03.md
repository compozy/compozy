---
status: pending
domain: Migration
type: Feature Implementation
scope: Full
complexity: medium
dependencies:
  - task_01
---

# Task 3: compozy migrate v1→v2 Pass + Fixture Migration

## Overview

Extend `compozy migrate` with a v1→v2 pass that chains after the existing legacy→v1 pass: extract the task title from the body H1, drop `scope`/`domain`, apply a best-effort type remap table, and flag files whose `type` cannot be mapped. Run the migration on the repository's existing `.compozy/tasks/acp-integration/` fixtures and hand-fix the flagged types so the repo is v2-clean after this task.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- MUST detect v1 task files by presence of `scope` or `domain` frontmatter keys, or absence of `title` frontmatter key (per ADR-004)
- MUST extract the title from the body H1 using the fallback sequence `# Task N: <title>` → `# Task N - <title>` → `# <title>` and strip the `Task N:` / `Task N -` prefix
- MUST write the v2 frontmatter using `frontmatter.Format(model.TaskFileMeta, body)` with `Title` populated and without `Domain`/`Scope`
- MUST apply a case-insensitive type-remap table (e.g., `"Bug Fix"→bugfix`, `"Refactor"→refactor`, `"Documentation"→docs`, `"Test"→test`, `"Chore"→chore`, `"Configuration"→infra`, `"Feature Implementation"→""`), falling back to exact (case-insensitive) match against `registry.Values()` before giving up
- MUST leave `type: ""` when no mapping applies and record the path in a new `MigrationResult.UnmappedTypeFiles` slice
- MUST be idempotent: running migrate on a v2 file produces no change
- MUST chain with the existing legacy→v1 pass so legacy XML files become v2 in a single run (no intermediate v1 write)
- MUST print a summary at the end of migration listing unmapped-type files and including the ADR-003 fix prompt for the user
- MUST successfully migrate `.compozy/tasks/acp-integration/task_01.md`, `task_02.md`, `task_03.md` and leave them passing `compozy validate-tasks`
</requirements>

## Subtasks
- [ ] 3.1 Add `migrateV1ToV2(path string, content string, registry *tasks.TypeRegistry) (*pendingFileMigration, migrationOutcome, error)` to `internal/core/migrate.go`.
- [ ] 3.2 Add an H1 title extractor helper with the three-format fallback and `Task N:` / `Task N -` prefix stripping.
- [ ] 3.3 Add the type-remap table (either inline in `migrate.go` as a `var` or in a new `internal/core/tasks/type_remap.go`).
- [ ] 3.4 Extend detection in `inspectTaskArtifact` (`migrate.go:222-257`) to route v1 files into the new pass and chain legacy→v1→v2 when both apply.
- [ ] 3.5 Extend `MigrationResult` (`internal/core/api.go:126-137`) with `V1ToV2Migrated int` and `UnmappedTypeFiles []string`; print them in the migrate summary.
- [ ] 3.6 Execute `compozy migrate` against `.compozy/tasks/acp-integration/` and hand-fix the three fixture files to valid v2 (and commit them in this task).
- [ ] 3.7 Add table-driven tests covering mapping hits, mapping misses, title-extractor fallbacks, idempotency, and legacy→v2 chaining.

## Implementation Details

Detection precedence (ADR-004): (1) legacy XML markers → legacy→v1 first, then v1→v2 in the same write; (2) `scope`/`domain` frontmatter keys present → v1→v2; (3) `title` frontmatter key absent → v1→v2 (title-extraction only); (4) otherwise skip.

The type-remap table is intentionally small and hand-curated. After the explicit mapping, attempt a case-insensitive exact match against `registry.Values()` as a second-chance (supports user-defined types). Only then fall back to empty.

Use the existing `frontmatter.Format` helper at `internal/core/frontmatter/frontmatter.go:98` for the write path. Do not hand-construct YAML.

Chain the passes inside `inspectTaskArtifact` (`migrate.go:222`): if legacy detection succeeds, run legacy→v1 in memory, then feed the result into v1→v2 and return the final v2 content. This avoids intermediate writes and keeps the function idempotent.

Record unmapped files via `result.UnmappedTypeFiles = append(result.UnmappedTypeFiles, path)`. After the walker finishes, if `len(result.UnmappedTypeFiles) > 0`, print the ADR-003 fix prompt (via `tasks.FixPrompt` once task_02's function exists — or a local stub if task_02 has not yet landed in the integration branch).

For the fixture migration (subtask 3.6): execute the command, inspect the three migrated files, and set `type` explicitly per task's content. The expected mappings are roughly: `task_01` (ACP Agent Layer) → `backend`, `task_02` (Execution & Logging Pipeline Migration) → `backend`, `task_03` → assess against content.

Refer to TechSpec "Core Interfaces" / "Data Models" and to ADR-004 for the full detection precedence and remap behavior.

### Relevant Files
- `internal/core/migrate.go` (lines 1-307, specifically 222-257 for the task artifact path) — extend with v1→v2 pass.
- `internal/core/migrate_test.go` — extend with v1→v2 test cases.
- `internal/core/api.go` (lines 126-137) — extend `MigrationResult` with new counters.
- `internal/core/frontmatter/frontmatter.go` (line 98) — `Format` helper reused here.
- `internal/core/tasks/type_remap.go` — NEW (optional location for the remap table).
- `.compozy/tasks/acp-integration/task_01.md`, `task_02.md`, `task_03.md` — migrated fixtures committed in this task.

### Dependent Files
- `internal/cli/root.go` — `newMigrateCommand()` summary output may need extension to show the new counters; keep wording changes minimal.
- Skill documentation (task_06) references these fixtures.

### Related ADRs
- [ADR-004: Migration Strategy](adrs/adr-004.md) — v1→v2 design, detection precedence, remap table.
- [ADR-001: Task Metadata Schema v2](adrs/adr-001.md) — Title extraction / H1 sync.
- [ADR-002: Task Type Taxonomy](adrs/adr-002.md) — Registry used for second-chance type matching.

## Deliverables
- `migrateV1ToV2` function + H1 extractor + type remap table.
- Extended `MigrationResult` counters and summary output.
- Chained legacy→v1→v2 single-pass behavior.
- Migrated `.compozy/tasks/acp-integration/` fixtures committed.
- Unit tests with 80%+ coverage **(REQUIRED)**.
- Integration test using `t.TempDir()` fixtures covering all detection branches **(REQUIRED)**.

## Tests
- Unit tests:
  - [ ] v1 fixture with `type: "Bug Fix"` maps to `bugfix` after migration.
  - [ ] v1 fixture with `type: "Refactor"` maps to `refactor`.
  - [ ] v1 fixture with `type: "Documentation"` maps to `docs`.
  - [ ] v1 fixture with `type: "Feature Implementation"` leaves `type: ""` and appends the path to `UnmappedTypeFiles`.
  - [ ] v1 fixture with `type: "Frontend"` (case-insensitive exact match against registry `frontend`) maps to `frontend`.
  - [ ] H1 extractor handles `# Task 1: ACP Agent Layer` → `"ACP Agent Layer"`.
  - [ ] H1 extractor handles `# Task 10 - Cleanup` → `"Cleanup"`.
  - [ ] H1 extractor handles `# Plain Title` → `"Plain Title"`.
  - [ ] H1 extractor returns empty when the body has no H1.
  - [ ] Running migrate on an already-v2 file is a no-op (content unchanged).
  - [ ] Legacy XML fixture chains through legacy→v1→v2 in a single pass (no `Domain`/`Scope` in output, `Title` populated).
- Integration tests:
  - [ ] Running `compozy migrate` on a directory with mixed v1/v2/legacy files writes exactly the v1/legacy ones and leaves v2 untouched; `MigrationResult.V1ToV2Migrated` equals the v1-input count.
  - [ ] After migration, `compozy validate-tasks` run on the same directory returns exit 0 only when `UnmappedTypeFiles` is empty; otherwise exit 1 with the fix prompt.
  - [ ] The committed `.compozy/tasks/acp-integration/` fixtures pass `compozy validate-tasks` (exit 0).
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `make verify` passes (fmt + lint + test + build with zero issues)
- `.compozy/tasks/acp-integration/task_01.md`, `task_02.md`, `task_03.md` contain v2 frontmatter (no `domain`/`scope`, have `title`, have allowed `type`) and pass `compozy validate-tasks`
- Running `compozy migrate` twice in a row produces identical output the second time (idempotency)
