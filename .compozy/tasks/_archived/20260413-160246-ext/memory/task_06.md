# Task Memory: task_06.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Implement all eleven Host API service methods behind `HostAPIRouter` using in-process kernel/service paths only, with typed request/result structs, recursion guard, path scoping, prompt rendering, event publication, and test coverage at or above 80% for `internal/core/extension`.

## Important Decisions
- Kept the task within the three-file service split required by the spec: `host_writes.go`, `host_reads.go`, and `host_helpers.go`.
- Introduced `KernelOps` plus `NewDefaultKernelOps` inside `internal/core/extension` so later lifecycle work can inject real kernel bindings without shelling out.
- Reused the Markdown memory model by exporting document read/write helpers from `internal/core/memory/store.go` instead of inventing a task-06-only writer.
- Closed the `host.events.publish` contract gap by adding `journal.SubmitWithSeq` and returning the assigned `seq` in the Host API response.

## Learnings
- `tasks.ReadTaskEntries` wraps missing-directory errors, so `host.tasks.list` needed `errors.Is(err, fs.ErrNotExist)` rather than only `os.IsNotExist`.
- `host.tasks.get` needed to return JSON-RPC invalid params for non-positive task numbers; a plain Go error was not consistent with the Host API contract.
- Direct helper-path tests were required to push `internal/core/extension` coverage from `77.5%` to the task target of `80.0%`.

## Files / Surfaces
- `internal/core/extension/host_helpers.go`
- `internal/core/extension/host_reads.go`
- `internal/core/extension/host_writes.go`
- `internal/core/extension/host_helpers_test.go`
- `internal/core/extension/host_reads_test.go`
- `internal/core/extension/host_writes_test.go`
- `internal/core/extension/chain.go`
- `internal/core/extension/host_api_errors.go`
- `internal/core/memory/store.go`
- `internal/core/model/runtime_config.go`
- `internal/core/run/journal/journal.go`
- `internal/core/run/journal/journal_test.go`
- `pkg/compozy/events/event.go`
- `pkg/compozy/events/kinds/extension.go`
- `pkg/compozy/events/docs_test.go`
- `docs/events.md`

## Errors / Corrections
- Fixed revive failures by renaming unused `context.Context` parameters in read-side methods to `_`.
- Fixed the missing-workflow-list path to return an empty task list instead of surfacing a wrapped `ENOENT`.
- Fixed the Host API event publish response to include the actual journal sequence rather than the zero value.

## Ready for Next Run
- Verification evidence is fresh: `go test ./internal/core/extension -cover` passed at `80.0%`, and `make verify` passed with `1289` tests and a successful build.
- Task tracking files are ready to be marked complete; per repo instructions they should stay out of the commit unless explicitly required.
