Goal (incl. success criteria):

- Implement daemon `task_07` by rewriting `compozy sync` from `_meta.md` refresh into `global.db` reconciliation for workflow artifacts.
- Success requires single-workflow and workspace-wide sync to upsert workflow/task/review snapshot state plus sync checkpoints without rewriting authored artifacts, to stop generating `_tasks.md`/workflow `_meta.md`, and to pass `make verify`.

Constraints/Assumptions:

- Follow `AGENTS.md`, `CLAUDE.md`, `.compozy/tasks/daemon/_techspec.md`, `.compozy/tasks/daemon/_tasks.md`, `.compozy/tasks/daemon/task_07.md`, and ADR-002.
- Required skills loaded for this run: `cy-workflow-memory`, `cy-execute-task`, `cy-final-verify`, `golang-pro`, `testing-anti-patterns`; the daemon techspec is treated as the approved design baseline.
- The worktree is already dirty in unrelated daemon tracking files; do not touch unrelated changes or use destructive git commands.
- Keep scope tight to task_07; archive/runtime consumers still have follow-up tasks and should only be changed when required to keep the sync rewrite coherent.

Key decisions:

- Reuse existing task parsing (`internal/core/tasks/{parser,walker,store}.go`), review parsing/store (`internal/core/reviews/{parser,store}.go`), and memory file handling (`internal/core/memory/store.go`) rather than inventing new Markdown readers.
- Build the new persistence seam under `internal/store/globaldb` and have `internal/core/sync.go` open `global.db`, resolve/register the workspace, and reconcile one or many workflow directories into DB rows.
- Treat the legacy pre-change signal as: `internal/core/sync.go` only calls `tasks.RefreshTaskMeta`, `SyncResult` and CLI output still speak in metadata-file terms, and no `globaldb` sync adapter exists yet.

State:

- Completed with local commit `eaec007`; final response pending.

Done:

- Read required skill guides, repository instructions, workflow memory, daemon techspec, task_07, `_tasks.md`, and ADR-002.
- Scanned related daemon ledgers for task slicing and global DB handoff context.
- Inspected current sync implementation plus task/review/memory parsers, `globaldb` schema/helpers/tests, workspace discovery, sync CLI output, and relevant AGH reference files.
- Added `internal/store/globaldb/sync.go` plus tests to upsert/delete workflow artifact snapshots, task items, review rounds/issues, and sync checkpoints.
- Rewrote `internal/core/sync.go` to resolve/register workspaces, scan authored workflow artifacts, clean legacy workflow `_meta.md` plus noncanonical `_tasks.md`, and reconcile single-workflow or whole-root sync into `global.db`.
- Updated sync-facing CLI/public contracts (`internal/cli/commands_simple.go`, `internal/cli/root.go`, `internal/core/model/workflow_ops.go`, `test/public_api_test.go`) to describe DB reconciliation instead of metadata refresh.
- Added integration coverage for single-workflow sync, workspace-wide sync, stable task identity across re-sync, mixed artifact ingestion, and one-time legacy cleanup.
- Verified file-level coverage on the new sync seams:
  - `internal/core/sync.go`: `84.1%`
  - `internal/store/globaldb/sync.go`: `83.8%`
- Ran `make verify` successfully after the final lint/security fixes.
- Re-ran `make verify` after the task tracking updates; it passed cleanly again before the final commit.
- Created local commit `eaec007` with the implementation/test files only: `feat: reconcile workflow state into global db`.

Now:

- Final handoff only.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- Review-round `_meta.md` cleanup is deferred for later review-flow migration; task_07 keeps ingesting existing round metadata while removing workflow `_meta.md` and only noncanonical/generated `_tasks.md`.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-17-MEMORY-sync-persistence-rewrite.md`
- `.compozy/tasks/daemon/{_techspec.md,_tasks.md,task_07.md,adrs/adr-002.md}`
- `.compozy/tasks/daemon/memory/{MEMORY.md,task_07.md}`
- `internal/core/{sync.go,sync_test.go,workflow_target.go,api.go}`
- `internal/core/tasks/{parser.go,walker.go,store.go,store_test.go}`
- `internal/core/reviews/{parser.go,store.go,store_test.go}`
- `internal/core/memory/store.go`
- `internal/core/model/{task_review.go,workflow_ops.go,workspace_paths.go}`
- `internal/store/{sqlite.go,values.go}`
- `internal/store/globaldb/{global_db.go,migrations.go,registry.go,migrations_test.go,registry_test.go,registry_integration_test.go}`
- `internal/cli/commands_simple.go`
- `/Users/pedronauck/dev/compozy/agh/internal/store/globaldb/global_db.go`
- `/Users/pedronauck/dev/compozy/agh/internal/observe/observer.go`
