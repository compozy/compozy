Goal (incl. success criteria):

- Implement `daemon-web-ui` task `04` (`Daemon Projection and Document Query Layer`).
- Success means: add an internal, transport-neutral daemon query layer that assembles dashboard, workflow overview, spec, memory, task detail, review detail, and run-related read models from `global.db`, `run.db`, and canonical workspace markdown; use opaque memory file IDs; add unit/integration tests; keep scope out of HTTP handlers; pass `make verify`; update workflow/task tracking only after clean verification.

Constraints/Assumptions:

- Must follow the scoped repository instructions from the worktree `AGENTS.md` / `CLAUDE.md`, the task spec, `_techspec.md`, ADRs `003` and `004`, and the provided workflow memory files.
- Required skills in use for this task: `brainstorming` (lightweight design pass against the already-approved techspec), `cy-workflow-memory`, `cy-execute-task`, `golang-pro`, `testing-anti-patterns`, and `cy-final-verify`.
- The worktree is already dirty in unrelated tracking/docs files; do not touch or revert unrelated changes.
- No destructive git commands without explicit user permission.
- Task scope is the internal read-model/query layer only; HTTP handlers/OpenAPI wiring belong to later tasks.

Key decisions:

- Use the approved task spec and daemon-web-ui TechSpec as the design authority instead of reopening feature design.
- Treat current `internal/daemon/*transport_service.go` implementations as the baseline seam to refactor behind a reusable query service instead of bolting more logic into handlers/transport stubs.
- Keep browser-facing path concerns out of payloads; memory documents must be addressed through daemon-issued opaque file IDs only.

State:

- Completed after clean verification and refreshed package coverage evidence.

Done:

- Read the required skill instructions for `brainstorming`, `cy-workflow-memory`, `cy-execute-task`, `cy-final-verify`, `golang-pro`, and `testing-anti-patterns`.
- Read worktree `AGENTS.md` and `CLAUDE.md`.
- Read workflow memory files:
  - `.compozy/tasks/daemon-web-ui/memory/MEMORY.md`
  - `.compozy/tasks/daemon-web-ui/memory/task_04.md`
- Read task docs:
  - `.compozy/tasks/daemon-web-ui/_techspec.md`
  - `.compozy/tasks/daemon-web-ui/_tasks.md`
  - `.compozy/tasks/daemon-web-ui/task_04.md`
  - `.compozy/tasks/daemon-web-ui/adrs/adr-003.md`
  - `.compozy/tasks/daemon-web-ui/adrs/adr-004.md`
- Scanned cross-agent ledgers relevant to daemon web UI and watcher/cache context.
- Captured the baseline pre-change signal:
  - `internal/daemon/task_transport_service.go` only implements workflow list/get and start/archive; `ListItems` and `Validate` are still unavailable.
  - `internal/daemon/review_exec_transport_service.go` exposes only latest/round/issues/start-run transport reads; there is no internal review-detail projection.
  - No dedicated spec/memory/document query service or opaque memory file identity layer exists yet.
  - Run snapshots exist via `internal/daemon/run_snapshot.go` / `RunManager.Snapshot`, but no richer transport-neutral run-detail projection layer exists for the web UI task surfaces.
- Added daemon query/read-model types and errors in `internal/daemon/query_models.go`.
- Added the mtime-aware markdown document reader and opaque memory ID helpers in `internal/daemon/query_documents.go`.
- Added `internal/daemon/query_service.go` with dashboard, workflow overview, task board, spec, memory index/file, task detail, review detail, and run detail projections.
- Added `internal/store/globaldb/read_queries.go` for task-item, artifact-snapshot, and review-round reads used by the query layer.
- Added executable coverage across the query layer, sync transport helpers, run snapshot branches, watcher error handling, and `globaldb` review/run query helpers.
- Fresh verification passed:
  - `go test -coverprofile=/tmp/daemon.cover ./internal/daemon` → `80.0%`
  - `go test -coverprofile=/tmp/globaldb.cover ./internal/store/globaldb` → `80.8%`
  - `make verify` → pass (`0 issues`, `DONE 2460 tests, 1 skipped`, build succeeded)

Now:

- No technical work remains for task_04.

Next:

- Task_05 should consume `NewQueryService` instead of re-reading workspace documents or rebuilding projections inside transport handlers.

Open questions (UNCONFIRMED if needed):

- None currently blocking.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-20-MEMORY-daemon-query-layer.md`
- `.compozy/tasks/daemon-web-ui/{_techspec.md,_tasks.md,task_04.md}`
- `.compozy/tasks/daemon-web-ui/adrs/{adr-003.md,adr-004.md}`
- `.compozy/tasks/daemon-web-ui/memory/{MEMORY.md,task_04.md}`
- `internal/api/core/interfaces.go`
- `internal/daemon/{query_models.go,query_documents.go,query_service.go,query_service_test.go,query_helpers_test.go,run_snapshot_test.go,sync_transport_service_test.go,host_runtime_test.go,watcher_error_test.go,task_transport_service.go,review_exec_transport_service.go,workspace_transport_service.go,transport_mappers.go,run_snapshot.go,watchers.go,run_manager.go}`
- `internal/core/{tasks,reviews}/`
- `internal/store/globaldb/{read_queries.go,read_queries_test.go,query_coverage_test.go}`
- Commands:
  - `go test ./internal/daemon`
  - `go test ./internal/store/globaldb`
  - `go test -coverprofile=/tmp/daemon.cover ./internal/daemon`
  - `go test -coverprofile=/tmp/globaldb.cover ./internal/store/globaldb`
  - `make verify`
