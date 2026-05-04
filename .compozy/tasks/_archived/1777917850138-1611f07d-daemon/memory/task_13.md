# Task Memory: task_13.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Migrate `pkg/compozy/runs` to daemon-backed readers without changing the exported package shape for callers that use `Open`, `List`, `Replay`, `Tail`, and `WatchWorkspace`.
- Replace direct reads of workspace-local run files with daemon list/snapshot/events/stream queries and keep stable normalization for status, timestamps, ordering, and replay semantics.

## Important Decisions
- Use one daemon-backed reader implementation in the public package; do not keep the local filesystem reader as a fallback path.
- Preserve `WatchWorkspace` as a workspace-level watch surface by comparing daemon-backed list snapshots instead of watching `.compozy/runs` on disk.
- Keep daemon bootstrap out of the public reader; when the daemon is unavailable, return a stable reader error instead of silently reading local files.
- Keep the public package decoupled from `internal/daemon` and `internal/api/core` by speaking the daemon transport contract directly over HTTP/UDS from `pkg/compozy/runs/run.go`.

## Learnings
- `internal/daemon.RunManager` already exposes the required run read contract through `List`, `Get`, `Snapshot`, `Events`, and `OpenStream`.
- `pkg/compozy/runs/remote_watch.go` already contains the reconnect/overflow/heartbeat logic needed for daemon SSE follow behavior.
- The daemon transport does not project old top-level run metadata like `IDE`, `Model`, or `WorkspaceRoot` on the summary row, so the public package must derive or default those fields deterministically.
- Downstream tests that only need compatibility-mirror assertions should read `events.jsonl` directly instead of assuming `pkg/compozy/runs.Open` can bootstrap from a temp workspace without daemon info.

## Files / Surfaces
- `internal/api/client/runs.go`
- `pkg/compozy/runs/run.go`
- `pkg/compozy/runs/summary.go`
- `pkg/compozy/runs/replay.go`
- `pkg/compozy/runs/tail.go`
- `pkg/compozy/runs/watch.go`
- `pkg/compozy/runs/*_test.go`
- `internal/core/run/journal/journal_test.go`
- `internal/core/run/executor/execution_acp_integration_test.go`
- `test/public_api_test.go`

## Errors / Corrections
- Fixed daemon request path handling so query strings populate `RawQuery` instead of corrupting the transport path.
- Replaced a copied-by-value SSE frame accumulator with pointer mutation so heartbeat/overflow/event frames are parsed correctly.
- Removed the obsolete filesystem scanner path and updated downstream journal/executor tests that still depended on workspace-local public-reader bootstrapping.

## Ready for Next Run
- Task complete. Verification evidence: `go test -cover ./pkg/compozy/runs` (`80.2%`), `go test ./pkg/compozy/runs ./internal/core/run/journal ./internal/core/run/executor ./test`, and `make verify`.
