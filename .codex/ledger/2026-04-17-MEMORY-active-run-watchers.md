Goal (incl. success criteria):

- Complete daemon `task_08` (`Active-Run Watchers and Legacy Metadata Cleanup`).
- Success means: task/review runs auto-sync before start, active runs own scoped workflow watchers with debounce and clean teardown, watcher-triggered updates reuse the sync ingestion path and persist checkpoints, manual Markdown edits flow back into daemon state during the run, generated `_tasks.md`/`_meta.md` artifacts are cleaned once and not recreated by touched code paths, required tests pass, and `make verify` is green.

Constraints/Assumptions:

- Follow `AGENTS.md`, `CLAUDE.md`, daemon `_techspec.md`, `_tasks.md`, `task_08.md`, ADRs, workflow memory, and the `cy-execute-task` / `cy-workflow-memory` / `cy-final-verify` workflow.
- Required skills in use for this task: `cy-workflow-memory`, `cy-execute-task`, `cy-final-verify`, `golang-pro`, `testing-anti-patterns`.
- `brainstorming` is intentionally skipped because this task already has an approved techspec/task spec and `cy-execute-task` is the governing implementation workflow.
- The worktree is already dirty in unrelated daemon tracking files and ledgers; do not touch or revert unrelated changes.
- No destructive git commands without explicit user permission.

Key decisions:

- Use the existing sync ingestion path in `internal/core/sync.go` as the single reconciliation path for both pre-run sync and watcher-triggered sync rather than inventing watcher-specific parsing logic.
- Keep watcher scope at one workflow root per active run and filter reparsing to Markdown artifacts under that root; ignore events outside the owned workflow.
- Treat current `_meta.md` writers (`tasks.RefreshTaskMeta`, `reviews.RefreshRoundMeta`, archive checks, run lifecycle hooks) as part of the migration seam that task `08` must stop depending on for active-run flows.
- Perform the one-time workflow sync after run ID reservation / `global.db` allocation but before watcher startup, so duplicate run IDs fail through the canonical reservation path instead of racing in the sync phase.

State:

- Completed after watcher implementation, metadata migration fixes, daemon package coverage at 80.0%, and a clean `make verify`.

Done:

- Read `AGENTS.md`, `CLAUDE.md`, daemon task docs (`_techspec.md`, `_tasks.md`, `task_08.md`), ADRs, workflow memory, and relevant daemon ledgers.
- Read required skill instructions for `cy-workflow-memory`, `cy-execute-task`, `cy-final-verify`, `golang-pro`, and `testing-anti-patterns`.
- Checked the dependent package API with `go doc github.com/fsnotify/fsnotify`.
- Captured the pre-change signal:
  - `internal/daemon/run_manager.go` does not sync workflows before starting runs and has no watcher ownership.
  - `internal/core/sync.go` reconciles into `global.db` but only on explicit sync calls.
  - `internal/core/archive.go`, `internal/core/plan/prepare.go`, `internal/core/run/executor/*`, and extension host writes still refresh or require legacy `_meta.md` files, so touched flows can recreate metadata artifacts today.
  - `pkg/compozy/events` already has `artifact.updated` and `rundb` already projects artifact sync history, so watcher events can reuse that event surface.
- Added scoped watcher implementation, watcher-backed `artifact.updated` emission, metadata snapshot helpers, and run-manager integration/tests across the daemon/task/review seams.
- Reproduced the first post-change failures:
  - `TestRunManagerRejectsConcurrentDuplicateRunID` failed because workflow sync ran before duplicate run ID reservation, so the losing concurrent start could fail in the sync phase instead of returning `ErrRunAlreadyExists`.
  - `internal/core/run/executor` tests were still reading stale `_meta.md` files even though the touched task/review flows now intentionally compute snapshots without rewriting those files.
- Moved the initial workflow sync into `startRun(...)` after run allocation and before watcher startup, and updated executor tests to assert snapshots instead of rewritten metadata files.
- Added daemon helper coverage for watcher classification/error handling, purge wrapper delegation, exec failure terminal state, and start-run sync failure cleanup to bring `internal/daemon` to 80.0% coverage.
- Refactored `RunManager` startup helper resolution and the watcher event loop into smaller helpers to satisfy repository `funlen` / `gocyclo` limits, then reran the full repository verification gate successfully.

Now:

- Completion bookkeeping only.

Next:

- Create the local implementation commit with code changes only; leave workflow memory / task tracking artifacts out of the commit.

Open questions (UNCONFIRMED if needed):

- Archive eligibility still rewrites/depends on metadata through `internal/core/archive.go`; the planned DB-backed archive rewrite remains explicitly deferred to `task_09`.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-17-MEMORY-active-run-watchers.md`
- `.compozy/tasks/daemon/{_techspec.md,_tasks.md,task_08.md,task_09.md}`
- `.compozy/tasks/daemon/memory/{MEMORY.md,task_08.md}`
- `internal/daemon/{run_manager.go,run_manager_test.go}`
- `internal/core/{sync.go,sync_test.go,archive.go,archive_test.go}`
- `internal/core/{tasks/store.go,reviews/store.go,memory/store.go,plan/prepare.go,run/executor/lifecycle.go,run/executor/review_hooks.go}`
- `internal/store/{globaldb/sync.go,rundb/run_db.go}`
- `pkg/compozy/events/{event.go,kinds/extension.go}`
- Commands:
- `git status --short`
- `go doc github.com/fsnotify/fsnotify`
- `rg`
- `sed -n`
