# Task Memory: task_02.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Implement `tasks.Validate`, `tasks.FixPrompt`, and the `compozy validate-tasks` CLI with text/json output, deterministic issue reporting, and the `0/1/2` exit-code contract.

## Important Decisions
- Resolve the task type registry once from workspace config in the CLI and pass it into `tasks.Validate`; do not reload types per file.
- Carry command-specific exit codes through returned errors and unwrap them in `cmd/compozy/main.go`, because the current entrypoint exits `1` for every CLI error.
- Treat v1 frontmatter (`scope` / `domain`) as validation issues instead of fatal parse errors so the validator can report actionable schema violations without aborting the run.

## Learnings
- `prompt.ParseTaskFile` already distinguishes v1 frontmatter via `ErrV1TaskMetadata`, but the current CLI has no command-specific exit-code plumbing.
- `internal/cli/root_test.go` provides a reusable Cobra help/output harness, but there is no existing subprocess-style CLI exit-code test helper.
- `internal/core/tasks` package coverage is `80.6%` after adding direct error-path tests for missing registries and legacy XML metadata.
- `make verify` initially failed on the new subprocess integration tests because `golangci-lint` forbids `exec.Command`; switching those helpers to `exec.CommandContext` resolved the lint failure cleanly.

## Files / Surfaces
- `internal/core/tasks`
- `internal/cli`
- `cmd/compozy/main.go`
- `command/command.go`
- `internal/core/tasks/testdata`

## Errors / Corrections
- Replaced `exec.Command` with `exec.CommandContext` in the CLI integration test helper after the first `make verify` run failed lint.
- Removed an unused local exit-code interface from `internal/cli/exit.go` after the first lint pass.

## Ready for Next Run
- Task implementation is complete and verified.
- Verification evidence:
- `go test ./internal/core/tasks -cover -count=1` -> `80.6%`
- `./bin/compozy validate-tasks --help` -> exit 0, help text printed
- `make verify` -> pass (fmt + lint + test + build)
