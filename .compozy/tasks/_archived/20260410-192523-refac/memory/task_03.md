# Task Memory: task_03.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Complete Phase 2 domain restructuring without behavior changes.
- Outcome: task/review parsing now lives in `internal/core/tasks` and `internal/core/reviews`, workflow operation shapes live in `internal/core/model`, shared workflow-target/task-walker helpers are extracted, `preputil` is folded into `plan`, and provider registry wiring is clarified via `providerdefaults`.
- Final validation: `make verify` passed and the prompt parser call grep is empty.

## Important Decisions
- Keep the refactor as code motion and import rewiring only; no behavior changes or silent scope expansion.
- Treat `run/review_hooks.go`, CLI adapters/tests, workspace validation, and fetch/provider wiring as in-scope because they currently depend on the moved parsing/types/default registry.
- Move parser tests with their owning domain package instead of leaving them in `prompt`; keep only prompt-building tests in `prompt`.
- Use a separate `internal/core/providerdefaults` package for default provider composition; moving `DefaultRegistry` into `provider` itself creates an import cycle with `provider/coderabbit`.

## Learnings
- Current baseline still has 20 `prompt` parsing/sentinel references under `internal/`.
- There is no `adrs/` directory under `.compozy/tasks/refac`, so the PRD has no additional ADR context for this task.
- The worktree already contains unrelated tracking-file edits from earlier tasks; preserve them.
- Subtask `3.1` passed `make verify` after moving task parsing into `internal/core/tasks`; the next phase is review parsing.
- Subtasks `3.2` and `3.3` passed `make verify` after moving review parsing into `internal/core/reviews` and removing parsing ownership from `prompt`.
- The shared workflow-target helper and shared task walker landed cleanly once the duplicated logic was reduced to selector/path resolution only.
- The final repository gate passed with 967 tests and the build succeeding after the provider composition package was renamed to `providerdefaults`.

## Files / Surfaces
- `internal/core/prompt/common.go`
- `internal/core/tasks/`
- `internal/core/reviews/`
- `internal/core/plan/`
- `internal/core/run/review_hooks.go`
- `internal/core/api.go`
- `internal/core/model/`
- `internal/core/kernel/`
- `internal/core/kernel/commands/`
- `internal/core/sync.go`
- `internal/core/archive.go`
- `internal/core/migrate.go`
- `internal/core/fetch.go`
- `internal/core/workspace/config_validate.go`
- `internal/core/providerdefaults/defaults.go`
- `internal/cli/`
- `internal/core/prompt/prd.go`
- `internal/core/prompt/prompt_test.go`
- `internal/core/tasks/parser.go`
- `internal/core/tasks/parser_test.go`
- `internal/core/reviews/parser.go`
- `internal/core/reviews/parser_test.go`
- `internal/core/reviews/store_test.go`
- `internal/core/workflow_target.go`
- `internal/core/workflow_target_test.go`
- `internal/core/plan/journal.go`
- `internal/core/tasks/walker.go`
- `internal/core/tasks/walker_test.go`

## Errors / Corrections
- Missed one `frontmatter` import in `internal/core/prompt/common.go` after removing task parsing; fixed before the full verification run.
- Review relocation first failed `make verify` on a new `goconst` lint issue; fixed by centralizing review status literals in `internal/core/reviews/parser.go`.
- Moving `DefaultRegistry` into `internal/core/provider` caused an import cycle with `provider/coderabbit`; corrected by introducing `internal/core/providerdefaults`.

## Ready for Next Run
- Task 03 is complete. The next workflow task should treat the new package boundaries as the baseline.
