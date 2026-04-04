# TechSpec: Task Metadata v2 & Session Timeline Display

## Executive Summary

This change upgrades Compozy's task metadata from a free-text schema (v1) to a constrained, tool-friendly schema (v2): the human title moves into frontmatter, `scope` and `domain` are removed, and `type` becomes an enum validated against a user-configurable allowlist. A new `compozy validate-tasks` command is the single source of truth for the new contract and is embedded as a preflight check inside `compozy start`. The existing `compozy migrate` command gains a v1→v2 pass. The TUI session timeline panel header uses the new `title` + `type` to render a per-task banner and surfaces provider + model on the meta row.

**Primary technical trade-off**: Constraining the type taxonomy (enum vs. free text) requires every task file to be migrated and every new task to pass validation — we trade one-time migration churn for deterministic downstream tooling (TUI badges, filtering, future routing).

## System Architecture

### Component Overview

- **`internal/core/model/model.go`** — `TaskFileMeta` / `TaskEntry` structs gain `Title`, drop `Domain`/`Scope`. Canonical v2 schema.
- **`internal/core/tasks/` (existing package — extend)** — Already contains `store.go` / `store_test.go`. Add `types.go` (TypeRegistry, BuiltinTypes), `validate.go` (Validate, Report, Issue), `fix_prompt.go`, and the v1→v2 type mapping table. Keep the package pure and dependency-light; it is imported by CLI, migrate, run, and prompt.
- **`internal/core/workspace/config.go`** — Gains `TasksConfig` struct and `[tasks].types` TOML section. Validates the user list at config load.
- **`internal/core/prompt/common.go`** — `ParseTaskFile` updated to populate `Title`, reject files with `scope`/`domain`, surface legacy-v1 detection.
- **`internal/core/migrate.go`** — New v1→v2 pass chained after the existing legacy→v1 pass. Reuses existing walker and dry-run plumbing.
- **`internal/cli/validate_tasks.go` (new)** — Standalone `compozy validate-tasks` Cobra command.
- **`internal/cli/root.go`** — `compozy start` gains preflight hook that calls the validator and renders the Bubble Tea block form on failure.
- **`internal/core/run/validation_form.go` (new)** — Standalone Bubble Tea modal (Continue / Abort / Copy fix prompt).
- **`internal/core/run/ui_view.go`** — Timeline panel header updated to render task title + type badge and right-aligned provider+model.
- **`internal/core/run/types.go`** — `uiJob` gains `taskTitle`, `taskType` fields; `config` already has `provider`, `ide`, `model`.
- **`skills/cy-create-tasks/`** — SKILL.md + `references/task-template.md` + `references/task-context-schema.md` updated to document the new schema, the config read step, and the post-generation validate step.

### Data Flow

```
.compozy/config.toml → workspace.LoadConfig → TypeRegistry
                                                    ↓
                                    ┌───────────────┼───────────────┐
                                    ↓               ↓               ↓
                         validate-tasks CLI   compozy start    compozy migrate
                                    │         preflight               │
                                    ↓               ↓                 ↓
                             tasks.Validate ← ─ ─ ─ ─ ─ ─ ─ ─ ─ → tasks.Validate
                                    │               ↓                 │
                                    │      Bubble Tea form or         │
                                    │      --force path               │
                                    ↓               ↓                 ↓
                              exit code       jobs or abort      rewritten files
                                                    ↓
                                          TUI timeline header
                                    (title + [type] + provider·model)
```

### External System Interactions

None. All changes are local-filesystem and TUI.

## Implementation Design

### Core Interfaces

**Task type registry** (`internal/core/tasks/types.go`):

```go
package tasks

// Built-in default task types, used when .compozy/config.toml does not set [tasks].types.
var BuiltinTypes = []string{
    "frontend", "backend", "docs", "test",
    "infra", "refactor", "chore", "bugfix",
}

// TypeRegistry holds the resolved allowlist of task types for a workspace.
type TypeRegistry struct {
    values []string
    index  map[string]struct{}
}

// NewRegistry builds a registry from configured values, falling back to BuiltinTypes
// when configured is empty. It returns an error for duplicates or invalid slugs.
func NewRegistry(configured []string) (*TypeRegistry, error)

// IsAllowed reports whether slug is in the registry (case-sensitive).
func (r *TypeRegistry) IsAllowed(slug string) bool

// Values returns a copy of the resolved allowlist, sorted.
func (r *TypeRegistry) Values() []string
```

**Validator** (`internal/core/tasks/validate.go`):

```go
package tasks

// Issue describes one problem found in one task file.
type Issue struct {
    Path    string // absolute path to the task file
    Field   string // "title", "type", "status", "dependencies", "scope", "domain", "title_h1_sync"
    Message string // human-readable
}

// Report holds the outcome of a validation pass.
type Report struct {
    TasksDir string
    Scanned  int
    Issues   []Issue
}

func (r Report) OK() bool { return len(r.Issues) == 0 }

// Validate scans every task_*.md under tasksDir and returns a Report.
// registry provides the allowed type list. Errors are reserved for I/O failures;
// schema violations populate Report.Issues.
func Validate(ctx context.Context, tasksDir string, registry *TypeRegistry) (Report, error)

// FixPrompt renders an LLM-ready prompt listing the offending files and issues.
func FixPrompt(report Report, registry *TypeRegistry) string
```

**Updated task metadata** (`internal/core/model/model.go`):

```go
type TaskFileMeta struct {
    Status       string   `yaml:"status"`
    Title        string   `yaml:"title"`
    TaskType     string   `yaml:"type,omitempty"`
    Complexity   string   `yaml:"complexity,omitempty"`
    Dependencies []string `yaml:"dependencies,omitempty"`
}

type TaskEntry struct {
    Content      string
    Status       string
    Title        string
    TaskType     string
    Complexity   string
    Dependencies []string
}
```

### Data Models

**`.compozy/config.toml` — new `[tasks]` section**:

```toml
[tasks]
# Optional. If omitted, the built-in list applies:
#   frontend, backend, docs, test, infra, refactor, chore, bugfix
types = ["frontend", "backend", "docs", "test", "mobile", "api"]
```

Config struct additions (`internal/core/workspace/config.go`):

```go
type ProjectConfig struct {
    Defaults     DefaultsConfig     `toml:"defaults"`
    Start        StartConfig        `toml:"start"`
    Tasks        TasksConfig        `toml:"tasks"`        // new
    FixReviews   FixReviewsConfig   `toml:"fix_reviews"`
    FetchReviews FetchReviewsConfig `toml:"fetch_reviews"`
}

type TasksConfig struct {
    Types *[]string `toml:"types"` // nil = use built-ins; empty list = validation error
}
```

**Slug validation** (applied in both `TasksConfig.Validate` and `NewRegistry`): regex `^[a-z][a-z0-9_-]{1,31}$`.

**v2 task file example**:

```yaml
---
status: pending
title: ACP Agent Layer & Content Model
type: backend
complexity: high
dependencies: []
---

# Task 1: ACP Agent Layer & Content Model

## Overview
...
```

### API Endpoints

None. CLI only:

- `compozy validate-tasks [--name <n>] [--tasks-dir <path>] [--format text|json]`
- `compozy start` now runs preflight before job setup; new flags: `--skip-validation`, `--force` (continue despite validation errors in non-TTY mode).
- `compozy migrate` — no flag changes; internally performs v1→v2 pass in addition to legacy→v1.

## Integration Points

None outside the codebase. All changes are internal to the Compozy CLI/workspace.

## Impact Analysis

| Component | Impact Type | Description and Risk | Required Action |
|-----------|-------------|---------------------|-----------------|
| `internal/core/model/model.go` | modified | `TaskFileMeta`/`TaskEntry` field changes; ALL consumers of `Domain`/`Scope` break at compile time. Low risk — compile catches everything. | Add `Title`, remove `Domain`/`Scope` fields, update all call sites. |
| `internal/core/prompt/common.go` | modified | `ParseTaskFile` populates `Title`; rejects files with legacy v1 `scope`/`domain` fields by producing a specific error that migrate knows how to detect. | Update parser; add `ErrV1TaskMetadata` sentinel. |
| `internal/core/tasks/` | extended (existing) | Adds `types.go`, `validate.go`, `fix_prompt.go`, type-remap table alongside existing `store.go`. | Add files in-place. |
| `internal/core/workspace/config.go` | modified | Add `TasksConfig` + validation. | Add struct, validator, TOML section. |
| `internal/core/migrate.go` | modified | Add v1→v2 detection and migration; chain after legacy→v1. | Extend detection switch; add `migrateV1ToV2`. |
| `internal/core/run/ui_view.go` | modified | Timeline panel header renders title + type badge + provider/model row. | Replace `session.timeline` static label with dynamic title; add right-aligned meta. |
| `internal/core/run/types.go` | modified | `uiJob` gains `taskTitle`, `taskType`; job setup wires them from parsed task metadata. | Add fields + wiring. |
| `internal/core/model/model.go` (`type Job`) | modified | Add `TaskTitle`/`TaskType` so parsed task metadata flows from plan preparation into the runner. | Add fields + populate from `TaskEntry` at `plan/prepare.go:156-165`. |
| `internal/core/run/ui_model.go` | modified | Populate `jobQueuedMsg.TaskTitle`/`.TaskType` inside `newUIController` at lines 214-224 (this is the actual emission site, NOT `execution.go`). | Extend emitter + message shape. |
| `internal/core/run/types.go` (internal `job`) | modified | Copy the new fields inside `newJobs()` at line 316 so they travel with the in-process job. | Add fields to the internal job struct. |
| `internal/core/run/validation_form.go` | new file | Bubble Tea modal for preflight failures. | Implement model + view + update. |
| `internal/cli/validate_tasks.go` | new file | Cobra command wiring. | Implement. |
| `internal/cli/root.go` | modified | Register `validate-tasks`; add preflight in `start`; add `--skip-validation`/`--force`. | Edit. |
| `skills/cy-create-tasks/SKILL.md` | modified | Add "read config types" step and "run validate-tasks" step. | Edit. |
| `skills/cy-create-tasks/references/task-template.md` | modified | Show new frontmatter shape. | Edit. |
| `skills/cy-create-tasks/references/task-context-schema.md` | modified | Document new fields + enum. | Edit. |
| Existing task files under `.compozy/tasks/` | migrated | All files converted by `compozy migrate`; those with unmappable types get `type: ""` + flag. | Run migrate + fix flagged types. |

## Testing Approach

### Unit Tests

- **`internal/core/tasks/types_test.go`** — `NewRegistry` happy path, duplicate rejection, invalid slug rejection, empty-list rejection, fallback to defaults when configured is nil. Table-driven.
- **`internal/core/tasks/validate_test.go`** — Every issue type exercised: missing title, title/H1 desync, invalid type, legacy scope/domain present, unknown dependency, bad status, bad complexity. Table-driven with `t.TempDir()` fixtures.
- **`internal/core/tasks/fix_prompt_test.go`** — Golden-file test for the LLM prompt formatting given a synthetic report.
- **`internal/core/prompt/prompt_test.go`** (lines 130-199) — Extend existing table: v2 frontmatter parses; v1 frontmatter (with scope/domain) returns `ErrV1TaskMetadata`; legacy XML still returns `ErrLegacyTaskMetadata`. (The test file is `prompt_test.go`, not `common_test.go`.)
- **`internal/core/workspace/config_test.go`** — Extend with `[tasks].types` variants: absent, empty list (rejected), duplicates (rejected), invalid slug (rejected), valid custom list.
- **`internal/core/migrate_test.go`** — v1→v2 happy path, unmappable type flagged, mixed legacy+v1 in same directory, already-v2 no-op, H1 extraction fallbacks.
- **`internal/core/run/validation_form_test.go`** — Bubble Tea model update/view test: all three actions produce expected messages; keyboard focus transitions.

### Integration Tests

- **`internal/cli/validate_tasks_test.go`** — Run the Cobra command against a synthetic tasks dir; assert exit code + JSON output shape.
- **`internal/core/run/ui_update_test.go`** (lines 263-301, around `handleJobQueued`) and **`internal/core/run/ui_view_test.go`** (lines 142-166, 476-479) — Extend with v2 metadata cases asserting `taskTitle`/`taskType` reach `uiJob` via `jobQueuedMsg` and render in the timeline panel output. (`execution_acp_integration_test.go` exercises the ACP session flow, not the timeline renderer.)
- **`compozy start` e2e** (existing harness) — Add scenario: bad task metadata → preflight blocks → `--force` bypass succeeds.

## Development Sequencing

### Build Order

1. **Type registry & config** — Create `internal/core/tasks/types.go`, extend `workspace.ProjectConfig` with `TasksConfig`, validate at load. **No dependencies.**
2. **Metadata schema v2** — Update `model.TaskFileMeta`/`TaskEntry`; update `prompt.ParseTaskFile`; add `ErrV1TaskMetadata`. Depends on step 1 only for consistency of consumer signatures. **Depends on step 1.**
3. **Validator & fix-prompt** — Create `internal/core/tasks/validate.go` + `fix_prompt.go`. **Depends on steps 1, 2.**
4. **`compozy validate-tasks` CLI** — Add command wiring. **Depends on step 3.**
5. **Migrate v1→v2 pass** — Add to `internal/core/migrate.go`; chain with existing legacy pass. **Depends on steps 1, 2, 3.**
6. **Run existing fixtures through migrate** — Execute `compozy migrate` on `.compozy/tasks/` in the repo; hand-fix any flagged types. Commit the migrated files. **Depends on step 5.**
7. **Bubble Tea validation form** — Create `internal/core/run/validation_form.go`. **Depends on step 3.**
8. **`compozy start` preflight** — Wire validator + form into start; add `--skip-validation` / `--force`. **Depends on steps 3, 7.**
9. **TUI timeline header** — Extend `uiJob` with `taskTitle`/`taskType`; update `renderTimelinePanel` to show title + badge + right-aligned provider/model; wire from `jobQueuedMsg`. **Depends on step 2.**
10. **Skill updates** — Update SKILL.md, task-template.md, task-context-schema.md for cy-create-tasks. **Depends on steps 1, 2, 4.**
11. **Documentation & examples** — Update README / CLI help strings. **Depends on all previous.**

### Technical Dependencies

- None external. All steps can be executed within this repo by a single engineer or agent sequence.

## Monitoring and Observability

- `compozy validate-tasks` prints structured text or JSON; CI consumers parse via `--format json`.
- `compozy start` preflight logs (via `slog`) the validation outcome (ok/failed) + issue count before rendering the form.
- `compozy migrate` already logs a summary; extended with v1→v2 counters (`V1ToV2Migrated`, `UnmappedTypeFiles`).

## Technical Considerations

### Key Decisions

- **Decision**: Title lives in both frontmatter and body H1 (see ADR-001).
  **Rationale**: Frontmatter is canonical for tooling; H1 preserves human readability in editors, diffs, and GitHub renders.
  **Trade-off**: Two-location sync enforced by the validator.
  **Alternatives rejected**: H1-only (fragile regex); frontmatter-only (degrades readability).

- **Decision**: Type taxonomy is a configurable enum that **replaces** defaults when set (see ADR-002).
  **Rationale**: Explicit user lists beat implicit merging; users can fully tune their taxonomy.
  **Trade-off**: Users who want to add to defaults must paste them.
  **Alternatives rejected**: Merge semantics (hides provenance); rich keyed map (YAGNI).

- **Decision**: Validation is a standalone command **plus** an embedded preflight in `start` (see ADR-003).
  **Rationale**: One validator, three call sites (CLI, skill, start) — DRY. Start preflight prevents wasted agent runs.
  **Trade-off**: Adds a new CLI surface and blocks start on failure.
  **Alternatives rejected**: Start-only (no skill/CI story); warning-only (defeats purpose); no interactive form (loses UX).

- **Decision**: Migration uses structural conversion + a small hand-curated remap table, leaving unmappable `type` empty (see ADR-004).
  **Rationale**: Deterministic structural edits; best-effort type mapping without false-positive silent misclassification.
  **Trade-off**: Files with exotic v1 types require manual user action post-migrate.
  **Alternatives rejected**: No remap (too much manual work); aggressive domain-based inference (too many false positives).

### Known Risks

- **Frontmatter/H1 desync**: Mitigated by `validate-tasks` running at generation, start-preflight, and CI.
- **Skill drift**: The LLM may ignore the config.toml read step. Mitigated by the post-generation `validate-tasks` enforcement — the skill cannot complete without a clean validation.
- **Non-TTY start blocking**: Users in CI may hit the preflight and get stuck. Mitigated by `--skip-validation` + printed fix prompt + clear error messaging.
- **Extended migration pauses mid-work**: A user with many unmappable types may have to fix each file. Mitigated by the batch fix prompt output listing all issues at once for a single LLM paste.

## Architecture Decision Records

- [ADR-001: Task Metadata Schema v2](adrs/adr-001.md) — Add `title` to frontmatter, remove `scope`/`domain`, keep H1 synced.
- [ADR-002: Task Type Taxonomy](adrs/adr-002.md) — Constrained enum with 8 built-in defaults, user-overridable via `[tasks].types` in `.compozy/config.toml`.
- [ADR-003: Validation Command Architecture](adrs/adr-003.md) — `compozy validate-tasks` + `start` preflight with Bubble Tea modal and LLM fix-prompt output.
- [ADR-004: Migration Strategy](adrs/adr-004.md) — Extend `compozy migrate` with a v1→v2 pass: structural conversion + best-effort type remapping, flag unmappable cases.
