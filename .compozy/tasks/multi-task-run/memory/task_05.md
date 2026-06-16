# Task Memory: task_05.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Implement the tabbed multi-run TUI attach path for daemon-owned `task_multi` parents: tabs for every requested slug, isolated child run UI state per tab, live parent/child updates, and queue-level quit dialog semantics.
- Preserve single-run `tasks run` UI behavior; scope is limited to multi-run parent attach and the CLI bridge that chooses it.

## Important Decisions
- Follow ADR-004 exactly: `Close TUI` detaches from the parent queue, `Stop Run` cancels the parent coordinator, and `Cancel` returns to the tabbed UI without side effects.
- Implemented the multi-run TUI as a wrapper around the existing single-run `uiModel` instead of changing the single-run layout; each tab owns its own child `uiModel` and `uiEventTranslator`.
- Chose `[` and `]` for tab navigation so `tab` / `shift+tab` continue to control existing pane focus inside the child run UI.
- CLI attach now detects parent snapshots with mode `task_multi` and routes UI/auto presentation to `ui.AttachRemoteMultiple`; single-run settled-before-attach behavior remains unchanged.

## Learnings
- Parent queue snapshots are the initial source of tab ordering and queued/not-started visibility; `task.multi.*` parent events are used for incremental tab status and child run discovery.
- Child streams should start once per discovered child run ID and route events back by slug, otherwise job indexes/transcripts can bleed across tabs.
- `Close TUI` must never call parent cancel from the CLI bridge; only the queue-level `Stop Run` quit action or context-owned cancellation should request `CancelRun` on the parent.

## Files / Surfaces
- Expected implementation surfaces: `internal/core/run/ui/*`, `internal/cli/run_observe.go`, and focused TUI/remote tests.
- Added `internal/core/run/ui/multi_remote.go` for the remote multi-run attach model/controller.
- Added `internal/core/run/ui/multi_remote_test.go` for tab rendering/state, quit behavior, and remote attach parent/child stream tests.
- Updated `internal/cli/run_observe.go` to route `task_multi` parent runs to the multi-run TUI attach session.
- Updated `internal/core/run/ui/update.go`, `view.go`, and `update_test.go` to centralize key literals required by lint while preserving single-run behavior.

## Errors / Corrections
- Initial full `make verify` failed on `gocyclo` in `attachRemoteCLIRunUI`; extracted request normalization and shared remote UI wait/cancel handling.
- Earlier lint also required key literal constants and a no-result parent-event helper; fixed before the passing full gate.

## Ready for Next Run
- Verified with `env -u GOROOT go test -timeout=60s ./internal/core/run/ui ./internal/cli`.
- Verified with `env -u GOROOT go test -coverprofile=/tmp/compozy-ui.cover ./internal/core/run/ui` at 80.2% statement coverage.
- Verified with `env -u GOROOT make lint`.
- Verified with `env -u GOROOT make verify`; full pipeline passed: frontend bootstrap/lint/typecheck/tests/build, Go fmt/lint/race tests/build, and frontend e2e.

---

## V2 — Add Git Worktree Allocation and Path Planning (current numbering)

> The notes above are the OLD V1 task_05 (tabbed multi-run TUI attach). The notes below are the CURRENT V2 task_05 from `_tasks.md`: the git worktree allocator/path planner. The two numbering schemes do not correspond.

### Objective Snapshot
- Add a daemon-local git worktree allocator boundary that resolves the parent branch+HEAD once, plans short deterministic detached-worktree paths, creates `git worktree add --detach` worktrees, and returns metadata for events/snapshots. No branch creation, merge, push, or worktree deletion (V1 preserves all worktrees).

### Important Decisions
- Allocator lives in `package daemon` (file `internal/daemon/task_multi_worktree.go`), mirroring the `review_watch_git.go` narrow git boundary. This satisfies the requirement "MUST NOT live under internal/core/run/internal/worktree" (daemon cannot import that internal package) and keeps it next to its only consumer, `task_multi.go`. Production wiring into the scheduler is intentionally deferred to task_06/07 (per the task's dependent-file note).
- Path layout: `<WorktreesDir>/<workspace-sha256[:12]>/<parent-short[:12]>/<NN-slug>`. Added `config.WorktreesDirName = "worktrees"` + `HomePaths.WorktreesDir` (= `state/worktrees`); also created at boot by `EnsureHomeLayout`.
- Slug/parent sanitization: lowercase; keep `[a-z0-9_]`; collapse every other rune (spaces, `/`, `\`, `.`) to a single `-`; trim dashes; cap (parent 12, slug 40). Dots collapse to `-` so `..` traversal is impossible.
- Detached-HEAD detection via `git rev-parse --abbrev-ref HEAD == "HEAD"`. Collision via `os.Stat` pre-check returning a clear "already exists" error before invoking git.
- `Allocate` takes a `taskMultiWorktreeSpec{WorkspaceRoot,ParentRunID,Slug,Index,Base}` and returns `taskMultiWorktreeAllocation{Path,BaseBranch,BaseCommit,WorktreeStatus}`; `WorktreeStatus` is the const `"preserved"`.

### Learnings
- staticcheck `unused` (golangci `tests: true`) counts same-package (`package daemon`) `_test.go` references, so the allocator is "used" without production wiring — verified by a clean `make lint`.
- goconst `ignore-calls` is on (3x `"rev-parse"` literals as call args already pass), so git arg literals don't trip goconst; `"HEAD"` still routed through `taskMultiWorktreeHeadRef` const to stay safe in the non-call comparison.
- revive `redefines-builtin-id` flags `max` as a local var name — use `maxLeaf` etc.

### Files / Surfaces
- Added `internal/daemon/task_multi_worktree.go` (allocator, path planner, sanitizer, default git runner).
- Added `internal/daemon/task_multi_worktree_test.go` (unit + real-git integration; reuses `runGitOutput` from `review_watch_test.go`).
- Updated `internal/config/home.go` (+`WorktreesDirName`, `HomePaths.WorktreesDir`, EnsureHomeLayout entry) and `internal/config/home_test.go`.

### Ready for Next Run
- `env -u GOROOT make fmt|lint|test|go-build` all green; lint 0 issues; daemon tests pass under `-race`; allocator helpers at ~100% coverage (Allocate 86.7%, runner 84.6%).
- task_06/07 handoff: instantiate `newTaskMultiWorktreeAllocator(homePaths.WorktreesDir)`, call `ResolveBase(ctx, parentWorkspaceRoot)` once per parent, then `Allocate(ctx, spec)` per child; map the returned allocation onto `kinds.TaskRunMultiplePayload.{WorktreePath,BaseBranch,BaseCommit,WorktreeStatus}` and emit BEFORE child launch.
