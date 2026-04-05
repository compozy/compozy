# Task Memory: task_06.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Completed the bundled task-generation doc updates, `start` CLI help wording updates, and the required tests for task schema v2.

## Important Decisions
- Use Go tests to scan only `README.md` and `skills/` for `domain:` / `scope:` lines so the assertions match the task requirement without tripping on intentional migration fixtures elsewhere.
- Reuse the existing Cobra help harness in `internal/cli/root_test.go` for the required `start --help` assertions.

## Learnings
- The bundled `cy-create-tasks` skill needed updates in three places to fully align with schema v2: workflow steps, the task template, and the task schema reference.
- A repo-level guard is now in place via Go tests to keep `README.md` plus `skills/` free of legacy `domain:` / `scope:` task frontmatter keys.
- `compozy start --help` is now protected by both substring assertions and a golden snapshot for the validation flag descriptions.

## Files / Surfaces
- `skills/cy-create-tasks/SKILL.md`
- `skills/cy-create-tasks/references/task-template.md`
- `skills/cy-create-tasks/references/task-context-schema.md`
- `README.md`
- `internal/cli/root.go`
- `internal/cli/root_test.go`
- `internal/cli/testdata/start_help.golden`
- `test/skills_bundle_test.go`

## Errors / Corrections
- None.

## Ready for Next Run
- Verification completed successfully:
- `go test ./internal/cli -count=1`
- `go test ./test -count=1`
- `rg -n '^[[:space:]]*(domain|scope):' README.md skills`
- `make verify`
- Local implementation commit created: `d3a7ebc` (`docs: update task schema v2 guidance`).
- Task tracking files were updated locally but intentionally left out of the commit per the task instruction to keep tracking-only files unstaged during auto-commit.
