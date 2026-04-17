Goal (incl. success criteria):

- Implement `task_02` for the daemon feature: create the durable `global.db` schema plus workspace/workflow/run registry services and tests.
- Success requires deterministic migrations with `schema_migrations`, normalized workspace identity collapse across equivalent paths, workflow slug uniqueness for active rows, active-run protection on unregister, clean verification via `make verify`, and task/memory tracking updates.

Constraints/Assumptions:

- Follow `AGENTS.md`, `CLAUDE.md`, `.compozy/tasks/daemon/_techspec.md`, `.compozy/tasks/daemon/_tasks.md`, `.compozy/tasks/daemon/task_02.md`, and ADRs under `.compozy/tasks/daemon/adrs/`.
- Required skills loaded for this run: `cy-workflow-memory`, `cy-execute-task`, `golang-pro`, `testing-anti-patterns`, `cy-final-verify`. The task spec itself acts as the approved design baseline for this implementation turn.
- Workspace is already dirty in daemon tracking files (`_meta.md`, `_tasks.md`, `task_01.md`, and untracked memory dir); do not disturb unrelated changes.
- `internal/store` does not exist yet, so this task introduces the initial store package surface.
- Completion requires `make verify` to pass; no destructive git commands are allowed.

Key decisions:

- Reuse the home layout from `internal/config/home.go` and AGH-style SQLite open/WAL configuration patterns instead of inventing separate DB boot logic.
- Keep the registry API narrow around the task requirements: workspace resolve/register/get/list/unregister plus workflow/run identity persistence helpers needed by later daemon tasks.
- Treat path normalization as symlink-aware canonicalization so logically identical workspace roots collapse to one durable workspace row.

State:

- In progress after context loading and pre-change inspection.

Done:

- Read repo instructions, required skill guides, workflow memory, techspec, task file, master task list, and daemon ADRs.
- Scanned cross-agent ledgers for daemon architecture and prior task handoffs.
- Confirmed the pre-change gap:
  - no `internal/store` package exists
  - `go.mod` has no SQLite driver dependency
  - workspace identity is still filesystem/cwd-driven
- Identified core code seams:
  - `internal/config/home.go`
  - `internal/core/workspace/config.go`
  - `internal/core/workflow_target.go`
  - `internal/core/model/{workspace_paths.go,artifacts.go,run_scope.go}`
- Inspected AGH SQLite/globaldb helpers as the reference for shared SQLite open and recovery patterns.

Now:

- Design the `internal/store/globaldb` package structure and inspect the exact adjacent interfaces/types to wire tests and durable identity cleanly.

Next:

- Implement migrations and registry operations, then add the required unit/integration tests.

Open questions (UNCONFIRMED if needed):

- UNCONFIRMED: whether this task should also add a small shared `internal/store` SQLite helper package now, or keep the open/configure logic local to `globaldb` until `rundb` arrives in task_03.
- UNCONFIRMED: exact row/type shapes for workflow/run persistence can stay internal to `globaldb` unless a later task needs exported transport DTOs earlier.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-17-MEMORY-global-db-registry.md`
- `.compozy/tasks/daemon/_techspec.md`
- `.compozy/tasks/daemon/_tasks.md`
- `.compozy/tasks/daemon/task_02.md`
- `.compozy/tasks/daemon/memory/{MEMORY.md,task_02.md}`
- `AGENTS.md`
- `CLAUDE.md`
- `internal/config/home.go`
- `internal/core/workspace/config.go`
- `internal/core/workflow_target.go`
- `internal/core/model/{workspace_paths.go,artifacts.go,run_scope.go}`
- `/Users/pedronauck/dev/compozy/agh/internal/store/{sqlite.go,globaldb/global_db.go,sessiondb/session_db.go}`
- Commands:
- `git status --short`
- `rg --files`
- `sed -n`
