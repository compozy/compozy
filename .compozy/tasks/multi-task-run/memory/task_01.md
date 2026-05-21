# Task Memory: task_01.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Implement task_01 foundations: `[tasks.run] run_multiple_mode` config parsing/merge/validation/defaulting plus reusable comma-separated multi-run slug parsing.
- Keep existing `compozy tasks run <slug>` behavior unchanged; V1 accepts only `enqueued` and `parallel`, with effective default `enqueued`.

## Important Decisions
- ADR-002/ADR-003 supersede the older ADR-001 wording around extending `tasks run`; this task targets dedicated `tasks run-multiple` foundations only.
- Place slug parsing in `internal/core/tasks` rather than the large `internal/cli` package; this keeps it reusable by CLI/API/daemon work without daemon imports and gives the parser focused package coverage.

## Learnings
- Pre-change code has no `RunMultipleMode` / `run_multiple_mode` implementation under `internal/core/workspace` or `internal/cli`; strict TOML decoding would reject the key.
- `GOROOT` is stale in this shell (`/Users/matheusbbarni/.local/go`); Go verification must run with `env -u GOROOT`.
- Focused coverage after moving the parser: `internal/core/workspace` 81.5%, `internal/core/tasks` 84.6%.

## Files / Surfaces
- Expected config surfaces: `internal/core/workspace/config_types.go`, `config_merge.go`, `config_validate.go`, `config_test.go`.
- Parser surfaces: `internal/core/tasks/slug_list.go` and `slug_list_test.go`.

## Errors / Corrections
- Initial parser placement in `internal/cli` passed focused tests but package coverage was below the task target because `internal/cli` is broad; moved the parser into `internal/core/tasks`.
- Unknown task-run TOML keys still fail through the strict decoder, but the decoder error does not include the unknown field name; tests assert the decode failure rather than field-name text.

## Ready for Next Run
- Implementation and verification complete for task_01. `make verify` passed with `env -u GOROOT`: frontend lint/typecheck/tests/build, Go fmt/lint/test/build, and frontend e2e all completed successfully.
