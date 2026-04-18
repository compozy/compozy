# Task Memory: task_15.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Migrate review fetch/list/show/fix and ad hoc exec flows onto daemon-backed lifecycle ownership without changing the authored review Markdown contract or the user-facing exec prompt/output contract.

## Important Decisions
- Reuse `internal/daemon.RunManager` plus daemon transport/client layers instead of adding new local review/exec execution paths.
- Keep existing top-level `fetch-reviews` and `fix-reviews` command surfaces working while introducing the daemon-backed `reviews` command family required by the TechSpec.
- Preserve current CLI-owned prompt-source resolution and JSON/raw-JSON error emission for `exec`; daemon start requests should receive already-resolved prompt text.

## Learnings
- The daemon manager already implements `StartReviewRun` and `StartExecRun`, but the daemon host does not yet expose `Reviews` or `Exec` services in `buildHostHandlers`.
- `internal/api/client` currently covers daemon/workspace/task/run/sync APIs, but not review fetch/list/show/fix or exec start calls.
- `internal/cli/run.go` still owns local `fetch-reviews`, `fix-reviews`, and `exec` execution, which is the concrete pre-change gap against task 15.

## Files / Surfaces
- `internal/daemon/{host.go,run_manager.go}`
- `internal/api/{core,client}`
- `internal/cli/{commands.go,daemon_commands.go,run.go,root.go,state.go}`
- `internal/core/{fetch.go,reviews,run/exec,extension}`
- `internal/store/globaldb`

## Errors / Corrections

## Ready for Next Run
- Start from the daemon transport layer: add review/exec services and client methods first, then rewire CLI commands and refresh tests around command output compatibility.
