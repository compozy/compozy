# Task Memory: task_01.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot

- Build the typed-task schema foundation for task_01: registry, `[tasks].types` config, v2 task metadata, parser v1 detection, and required callsite/test updates.
- Finish only when clean verification evidence exists, task tracking is updated, and the repo no longer reads `TaskEntry.Domain` / `TaskEntry.Scope`.

## Important Decisions

- Existing PRD + TechSpec + ADRs are the approved design baseline for this implementation run.
- `ParseTaskFile` should detect v1 by key presence (`domain` or `scope`) and return `ErrV1TaskMetadata` before semantic validation.
- `workspace` validation should treat `Tasks.Types == nil` as "use built-in defaults" and explicit empty list as an error.
- `internal/core/migrate.go` should defer v1 frontmatter as already-frontmatter for now; only legacy XML is rewritten in this task.

## Learnings

- `internal/core/prompt/common.go` still populates `Domain` and `Scope`, and the critical migrate/start/store callsites only special-case `ErrLegacyTaskMetadata`.
- Multiple tests across `plan`, `tasks`, `prompt`, `sync`, `archive`, and `run` still use v1-style frontmatter fixtures and will need coordinated updates.
- `test/public_api_test.go` also depended on v1 task fixtures; the full `make verify` gate exposed that regression and the fixtures were converted to v2.
- Fresh coverage for the touched packages is above the task target: `tasks` 81.9%, `workspace` 83.1%, `prompt` 90.5%, `model` 88.5%.

## Files / Surfaces

- `internal/core/tasks/*`
- `internal/core/workspace/config.go`
- `internal/core/model/model.go`
- `internal/core/prompt/common.go`
- `internal/core/prompt/prd.go`
- `internal/core/migrate.go`
- `internal/core/plan/input.go`
- `internal/core/*_test.go` files that build task frontmatter fixtures
- `test/public_api_test.go`

## Errors / Corrections

- Corrected a test-only mistake where a new migrate test used `t.Parallel()` together with `t.Chdir()`.
- Corrected prompt helper assertions/fixtures after targeted coverage runs exposed mismatches in the test expectations.

## Ready for Next Run

- Verification is clean. Next run should start from `.codex/CONTINUITY-schema-foundation.md`, review the tracking updates, and avoid staging unrelated `.compozy/tasks/acp-integration/_meta.md` changes.
