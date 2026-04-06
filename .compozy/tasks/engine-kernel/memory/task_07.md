# Task Memory: task_07.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Completed the public `pkg/compozy/runs/` API with list/open/replay/tail/watch semantics, dependency additions, and package coverage above 80%.

## Important Decisions
- Task 03 already landed the minimal public replay foundation needed to validate journal output: `RunSummary`, `Open`, `Summary`, `Replay`, partial-final-line tolerance, and schema-version checks. Task 07 should build on that surface rather than recreating it.
- Public run summaries normalize terminal statuses for external consumers and backfill missing workflow-run status from `result.json` plus `events.jsonl` because workflow `run.json` lacks terminal state.
- `WatchWorkspace` initializes its base and per-run watches before returning and treats transient `run.json` decode failures as retryable so new-run/status events are not lost during file writes.

## Learnings
- Supporting both `"cancelled"` and `"canceled"` terminal statuses must avoid duplicate literal switch cases because `golangci-lint --fix` rewrites British spellings.
- `Tail` must snapshot the live-follow offset before replay and short-circuit event/error sends on canceled contexts; otherwise replay/live handoff and race-mode cancellation become flaky.
- `fsnotify` setup must finish before `WatchWorkspace` returns or a newly created run can be absorbed by the initial seed scan instead of emitting `RunEventCreated`.

## Files / Surfaces
- `pkg/compozy/runs/run.go`
- `pkg/compozy/runs/scanner.go`
- `pkg/compozy/runs/run_test.go`
- `pkg/compozy/runs/summary.go`
- `pkg/compozy/runs/tail.go`
- `pkg/compozy/runs/watch.go`
- `pkg/compozy/runs/doc.go`
- `pkg/compozy/runs/list_test.go`
- `pkg/compozy/runs/tail_test.go`
- `pkg/compozy/runs/watch_test.go`
- `pkg/compozy/runs/helpers_test.go`
- `pkg/compozy/runs/integration_test.go`
- `pkg/compozy/runs/test_helpers_test.go`
- `go.mod`
- `go.sum`
- `internal/core/run/exec_flow.go`

## Errors / Corrections
- The original task 03/task 07 dependency conflict was already resolved; task 07 stayed additive (`List`, `Tail`, `WatchWorkspace`, deps, broader contract tests).
- Repo-wide linting required a `Tail` refactor below the `gocyclo` threshold plus constant hoists to avoid `goconst` and `"cancelled"` auto-fix regressions.

## Ready for Next Run
- Task implementation, package-level validation, and full `make verify` are complete; next run only needs normal downstream consumption of the public reader API.
