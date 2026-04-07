# Task Memory: task_02.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Refactor ACP session ingress handling in `internal/core/agent` so update publication uses a 1024-entry buffer, 5-second timed backpressure, explicit slow/drop counters, rate-limited drop warnings, and context-aware callers/tests.

## Important Decisions
- ADR-006 is the authority for this task's behavior contract; keep the change local to `internal/core/agent` and preserve existing `Session.Updates()` consumption semantics.
- Extend the exported `agent.Session` interface with `SlowPublishes()` and `DroppedUpdates()` so downstream observability and later tasks can read the counters without depending on `sessionImpl`.
- Keep `publish` serialized under the existing session mutex while waiting on the timed send so `finish()` cannot close `updates` concurrently and trigger send-on-closed panics.

## Learnings
- `session.publish` is currently called from `clientImpl.SessionUpdate` plus helper tests; there is no dedicated `session_test.go` yet.
- The helper ACP process in `client_test.go` needed paced update emission support to exercise the required 1000-update, 100/sec live session path without changing production code.
- Package-wide `internal/core/agent` coverage remains below 80% because of pre-existing low-coverage tool-normalization and registry helpers, but the changed files meet the task target: `session.go` 81.1% and `client.go` 80.5%.

## Files / Surfaces
- `internal/core/agent/session.go`
- `internal/core/agent/client.go`
- `internal/core/agent/session_helpers_test.go`
- `internal/core/agent/client_test.go`
- `internal/core/agent/session_test.go`
- `internal/core/run/execution_acp_test.go`

## Errors / Corrections
- Initial long-stream integration assertion assumed the first visible text block would always be `chunk-0000`; replaced it with a set-based check that all 1000 text chunks survive the full paced stream with zero drops.

## Ready for Next Run
- Final verification on the completed implementation passed with `go vet ./...`, `go test ./internal/core/agent -count=1`, `go test ./internal/core/agent -count=1 -coverprofile=/tmp/agent-cover.out`, and `make verify`.
