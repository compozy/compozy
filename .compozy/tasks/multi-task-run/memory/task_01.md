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

---

# V2 parallel-limit config (current task_01 in `_tasks.md`)

> The notes above are from the SUPERSEDED V1 task_01 (run_multiple_mode + slug parsing). The section below covers the CURRENT V2 task_01: "Add Parallel Limit Workspace Configuration".

## Objective Snapshot
- Add `[tasks.run] run_multiple_parallel_limit` config foundation for bounded parallel multi-run; default effective limit `2`; reject zero/negative; workspace-over-global precedence mirroring `run_multiple_mode`. No CLI or daemon scheduling changes in this task.

## Implementation
- `config_types.go`: added `DefaultRunMultipleParallelLimit = 2`, field `RunMultipleParallelLimit *int` (`toml:"run_multiple_parallel_limit"`), and method `EffectiveRunMultipleParallelLimit()` (nil -> default, else value).
- `config_merge.go`: merged the field in `buildEffectiveTaskRunConfig` via `cloneOptionalValue(preferOverlay(global, workspace))`, mirroring `RunMultipleMode`.
- `config_validate.go`: added `validateTaskRunMultipleParallelLimit` (rejects `<= 0`, names `tasks.run.run_multiple_parallel_limit`), called from `validateTaskRun`.
- `config_test.go`: parse/default/effective-helper/reject(0,-1)/workspace-over-global/global-fallthrough and a combined mode+limit precedence integration test.

## Important Decisions
- Extracted the positivity error template into package const `errMustBeGreaterThanZero` and reused it in the two existing `fix_reviews` checks plus the new limit check. Reason: a third identical literal would have tripped `goconst` (min-occurrences 3); this is the root-cause DRY fix, not a suppression. The `fix_reviews` error text is unchanged (tests assert on the field-name substring only).
- Int assertions in tests use inline `nil || *p != want` checks (the dominant int convention in this file) rather than adding a helper.

## Learnings
- `goconst` is disabled for `_test.go` and `min-occurrences: 3`; only non-test duplicates count.
- `loadConfigWithIsolatedHome` (mutex-guarded `osUserHomeDir` swap) is parallel-safe; global-config merge tests use `isolateWorkspaceConfigHome` (`t.Setenv`) and must NOT call `t.Parallel`.
- Focused coverage for `internal/core/workspace` after the change: 82.0%.

## Verification
- `env -u GOROOT make fmt` ok; `make lint` 0 issues; `make test` 3531 passed / 3 skipped (env-gated); `make go-build` ok.
