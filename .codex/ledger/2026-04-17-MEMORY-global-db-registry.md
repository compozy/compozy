Goal (incl. success criteria):

- Implement `task_02` for the daemon feature: create the durable `global.db` schema plus workspace/workflow/run registry services and tests.
- Success requires deterministic migrations with `schema_migrations`, normalized workspace identity collapse across equivalent paths, workflow slug uniqueness for active rows, active-run protection on unregister, clean verification via `make verify`, and task/memory tracking updates.

Constraints/Assumptions:

- Follow `AGENTS.md`, `CLAUDE.md`, `.compozy/tasks/daemon/_techspec.md`, `.compozy/tasks/daemon/_tasks.md`, `.compozy/tasks/daemon/task_02.md`, and ADRs under `.compozy/tasks/daemon/adrs/`.
- Required skills loaded for this run: `cy-workflow-memory`, `cy-execute-task`, `golang-pro`, `testing-anti-patterns`, `systematic-debugging`, `no-workarounds`, `cy-final-verify`. The task spec itself acts as the approved design baseline for this implementation turn.
- Workspace is already dirty in daemon tracking files (`_meta.md`, `_tasks.md`, `task_01.md`, and untracked memory dir); do not disturb unrelated changes.
- `internal/store` and `internal/store/globaldb` already exist in the repository with partial task_02 implementation and tests; extend/fix that surface rather than recreating it.
- Completion requires `make verify` to pass; no destructive git commands are allowed.

Key decisions:

- Reuse the home layout from `internal/config/home.go` and AGH-style SQLite open/WAL configuration patterns instead of inventing separate DB boot logic.
- Keep the registry API narrow around the task requirements: workspace resolve/register/get/list/unregister plus workflow/run identity persistence helpers needed by later daemon tasks.
- Treat path normalization as symlink-aware canonicalization so logically identical workspace roots collapse to one durable workspace row.

State:

- Completed after targeted coverage additions, full verification, tracking updates, and the local commit.

Done:

- Read repo instructions, required skill guides, workflow memory, techspec, task file, master task list, and daemon ADRs.
- Scanned cross-agent ledgers for daemon architecture and prior task handoffs.
- Reconciled the live repo state against stale notes:
  - `internal/store` and `internal/store/globaldb` already contain the SQLite helper, schema, registry code, and tests
  - `go test ./internal/store/...` passes
  - `make lint` passes cleanly
- Confirmed the current pre-change gap:
  - required concurrent registration and reopen-persistence integration cases are still missing
  - `go test -cover ./internal/store/globaldb` reports `79.8%`, below the task target of `>=80%`
- Identified core code seams:
  - `internal/config/home.go`
  - `internal/core/workspace/config.go`
  - `internal/core/workflow_target.go`
  - `internal/core/model/{workspace_paths.go,artifacts.go,run_scope.go}`
- Identified task-local implementation surfaces:
  - `internal/store/{sqlite.go,schema.go,store.go,values.go}`
  - `internal/store/globaldb/{global_db.go,migrations.go,registry.go}`
  - `internal/store/globaldb/{migrations_test.go,registry_test.go,registry_integration_test.go}`
- Inspected AGH SQLite/globaldb helpers as the reference for shared SQLite open and recovery patterns.
- Added `internal/store/globaldb` integration coverage for:
  - concurrent registration collapse onto one durable workspace row
  - reopen persistence for workspace, workflow, and run index visibility
- Confirmed `go test -cover ./internal/store/globaldb` now reports `80.0%`.
- Ran `make verify` successfully after the code change.
- Updated workflow memory plus `task_02.md` / `_tasks.md` to mark the task complete.
- Created local commit `50e87e6` with the staged code change only.

Now:

- Final response only.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None blocking.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-17-MEMORY-global-db-registry.md`
- `.compozy/tasks/daemon/_techspec.md`
- `.compozy/tasks/daemon/_tasks.md`
- `.compozy/tasks/daemon/task_02.md`
- `.compozy/tasks/daemon/memory/{MEMORY.md,task_02.md}`
- `AGENTS.md`
- `CLAUDE.md`
- `internal/store/{sqlite.go,schema.go,store.go,values.go}`
- `internal/store/globaldb/{global_db.go,migrations.go,registry.go}`
- `internal/store/globaldb/{migrations_test.go,registry_test.go,registry_integration_test.go}`
- `internal/config/home.go`
- `internal/core/workspace/config.go`
- `internal/core/workflow_target.go`
- `internal/core/model/{workspace_paths.go,artifacts.go,run_scope.go}`
- `/Users/pedronauck/dev/compozy/agh/internal/store/{sqlite.go,globaldb/global_db.go,sessiondb/session_db.go}`
- Commands:
- `git status --short`
- `rg --files`
- `sed -n`
- `go test ./internal/store/...`
- `go test -cover ./internal/store/globaldb`
- `make lint`
