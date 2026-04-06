# Task Memory: task_07.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Complete the public `pkg/compozy/runs/` API with list/open/replay/tail/watch semantics and dependency additions.

## Important Decisions
- Task 03 already landed the minimal public replay foundation needed to validate journal output: `RunSummary`, `Open`, `Summary`, `Replay`, partial-final-line tolerance, and schema-version checks. Task 07 should build on that surface rather than recreating it.

## Learnings
- `gotestsum -race` already validates the current `Open`/`Replay` surface against journal-produced event files, including truncated-final-line behavior.
- Supporting both `"cancelled"` and `"canceled"` terminal statuses must avoid duplicate literal switch cases because `golangci-lint --fix` rewrites British spellings.

## Files / Surfaces
- `pkg/compozy/runs/run.go`
- `pkg/compozy/runs/scanner.go`
- `pkg/compozy/runs/run_test.go`

## Errors / Corrections
- The original task 03/task 07 dependency conflict is resolved; remaining task 07 scope is additive (`List`, `Tail`, `WatchWorkspace`, deps, broader contract tests).

## Ready for Next Run
- Add `ListOptions`, `List`, `Tail`, `WatchWorkspace`, and the required `nxadm/tail` / `fsnotify` dependencies on top of the existing replay foundation.
