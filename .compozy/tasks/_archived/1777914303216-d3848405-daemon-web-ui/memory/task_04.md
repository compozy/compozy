# Task Memory: task_04.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Build the internal daemon query/read-model layer for dashboard, workflow overview/board, spec, memory, task detail, review detail, and run detail without adding HTTP/OpenAPI wiring in this task.
- Use canonical workspace markdown for spec/task/review/memory documents, use `global.db` / `run.db` for projections, and hide memory file paths behind stable opaque IDs.

## Important Decisions
- Keep the new read-model service inside `internal/daemon` so later transport services can consume it directly without reaching into handlers.
- Extend `internal/store/globaldb` with small read/query helpers for synced task items instead of duplicating SQL in the daemon layer.
- Use a daemon-side markdown reader with mtime-aware caching for individual documents; keep memory index generation separate so watcher invalidation can hook in later without changing payload shapes.
- Treat the approved task + TechSpec as the design authority; no scope expansion into transport routes or browser middleware.
- Normalize `"cancelled"` to the daemon’s canonical `runStatusCancelled` in read-model helpers instead of carrying duplicate spellings through switches; this keeps spell-fixing/lint stable while preserving browser-facing behavior.

## Learnings
- Current transport services already cover workspace, workflow summary, reviews, and run snapshots, but there is no shared internal query assembler and task item reads are not yet exposed from `global.db`.
- `core/sync.go` classifies canonical workflow artifacts (`prd`, `techspec`, `adr`, `memory`, `review_issue`, etc.) and persists task/review projections in `global.db`; that gives the new service stable DB and filesystem seams to compose.
- The existing `internal/daemon` run-manager test harness already creates real home/globaldb/workspace fixtures, so it can cover the required DB-plus-filesystem integration tests for this task.
- The task’s `>=80%` coverage gate is effectively package-scoped in this repository, so the query-layer tests had to cover adjacent daemon/globaldb helper branches rather than only the new files.
- On macOS, daemon host tests need a short `/tmp/...` home path when exercising real UDS startup; long temp paths can exceed Unix socket limits.

## Files / Surfaces
- `internal/daemon/{query_models.go,query_documents.go,query_service.go}`
- `internal/store/globaldb/{read_queries.go}`
- `internal/daemon/{query_service_test.go,query_helpers_test.go,run_snapshot_test.go,sync_transport_service_test.go,host_runtime_test.go,watcher_error_test.go,transport_service_test.go}`
- `internal/store/globaldb/{read_queries_test.go,query_coverage_test.go}`
- `.codex/ledger/2026-04-20-MEMORY-daemon-query-layer.md`

## Errors / Corrections
- Initial coverage on the new query tests left `internal/daemon` and `internal/store/globaldb` below the task gate; additional direct package tests were required to reach `80.0%` and `80.8%`.
- `golangci-lint --fix` rewrote the British-spelling literal `"cancelled"` during verification; the read-model code now normalizes that spelling before the lane-title switch instead of matching both literals in-place.

## Ready for Next Run
- Task complete. Next task should reuse `internal/daemon.NewQueryService` and its typed read models/errors when wiring transport services, instead of reaching directly into workspace documents or rebuilding projections.
