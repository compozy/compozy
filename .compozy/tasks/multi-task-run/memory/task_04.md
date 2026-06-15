# Task Memory: task_04.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Implement task_04: expose `compozy tasks run-multiple [slugs]` as a daemon-backed parent multi-run command with non-UI detach/stream behavior, while preserving `tasks run` single-slug behavior.
- Pre-change signal: `go run ./cmd/compozy tasks run-multiple --help` showed only `run` and `validate` under `tasks`; no `run-multiple` command is registered yet.

## Important Decisions
- Treat the PRD/TechSpec/ADRs as the approved design for this implementation run. The brainstorming skill's approval gate is satisfied by the existing approved PRD task and the user's explicit "begin now" authorization.
- Use the existing centralized slug parser (`internal/core/tasks.ParseCommaSeparatedSlugs`) and workspace mode constants/defaulting instead of duplicating validation or mode strings.
- Keep `tasks run-multiple` beside `tasks run` with shared task-run flags factored through a helper. Do not add `--name` to multi-run because the positional comma-separated slug list is the only accepted workflow selector.
- Stream mode now watches parent run events until a terminal snapshot and returns exit code 1 for failed/canceled/crashed parent runs, while detach mode prints only the parent run ID summary.

## Learnings
- ADR-002/ADR-003 supersede ADR-001's earlier wording around extending `tasks run`; task_04 must add a separate `run-multiple` command.
- The CLI stream renderer currently handles run/job/session events only; parent multi-run queue events need explicit text rendering for useful non-UI output.
- `make verify` runs Go through `env -u GOROOT` from the Makefile and succeeds with the local environment. Direct ad-hoc Go commands should continue using `rtk env -u GOROOT ...`.
- Package-wide `internal/cli` coverage remains below 80% due existing broad CLI surface, but the new command parsing/config helpers are covered directly: slug-list resolution 100%, multi-run preflight 84%+, and mode resolution 100%.

## Files / Surfaces
- Expected code surfaces: `internal/cli/daemon_commands.go`, `internal/cli/root.go`, `internal/cli/state.go`, `internal/cli/workspace_config.go`, `internal/cli/run.go`, `internal/cli/run_observe.go`, CLI test files, and in-process daemon test helpers.
- Implemented surfaces: `internal/cli/daemon_commands.go`, `internal/cli/root.go`, `internal/cli/state.go`, `internal/cli/workspace_config.go`, `internal/cli/run.go`, `internal/cli/run_observe.go`, `internal/cli/skills_preflight.go`, `internal/cli/extensions_bootstrap.go`, `internal/cli/commands_test.go`, `internal/cli/daemon_commands_test.go`, `internal/cli/daemon_exec_test_helpers_test.go`, `internal/cli/root_command_execution_test.go`, and `internal/cli/validate_tasks_test.go`.

## Errors / Corrections
- First full `make verify` failed at `golangci-lint` because `watchCLIRunUntilTerminalSuccess` had cyclomatic complexity 16 (>15). The fix split rendering/terminal-check/status mapping into helpers instead of suppressing lint.
- Initial daemon request-shape test compared a `/var/...` workspace path with the client-resolved `/private/var/...` path. The test now resolves symlinks before comparison, matching the CLI behavior.

## Ready for Next Run
- Full verification passed on 2026-05-17: `rtk make verify` completed fmt, lint, tests, build, and frontend e2e. Node emitted the known `NO_COLOR`/`FORCE_COLOR` warning during frontend steps, but lint reported 0 issues and verification exited successfully.
- task_05 can focus on the tabbed TUI attach experience; command registration, runtime overrides, mode fallback, detach output, and parent stream output are now wired.

---

# V2 — Add Multi-Run Event and Snapshot Worktree Metadata (current task_04)

> NOTE: The notes above are from the OLD V1 task_04 (`tasks run-multiple` CLI). The current V2 task_04 (`_tasks.md` row 04) is the EVENT/snapshot worktree metadata task below.

## Objective Snapshot
- Make parent `task.multi.*` events durable enough to reconstruct worktree-aware snapshots, additively and backward-compatibly. Persistence + snapshot only; worktree allocation and scheduler emission land in task_05/06/07.
- Pre-change signal: `kinds.TaskRunMultiplePayload` had no worktree/parallel-limit fields; the daemon snapshot builder copied only status/child_run_id/error. Target test packages green at baseline.

## Important Decisions
- Extended `kinds.TaskRunMultiplePayload` with additive `ParallelLimit` + `WorktreePath`/`BaseBranch`/`BaseCommit`/`WorktreeStatus` (all `omitempty`) to mirror the task_02 `contract.TaskRunMultipleItem`. Kept all existing `task.multi.*` event kind names and item statuses.
- Reconstruction stays event-derived (ADR-007): refactored the builder's item-event branch into `applyTaskMultiItemMetadata(item, payload)`, which overwrites only with non-empty values. This intentionally makes worktree metadata representable before a child run id exists (metadata-only updates) and keeps old events (no worktree fields) compatible. Guarding status on non-empty is a no-op for existing emitters (they always set status).
- Did NOT modify the scheduler emit path (`emitTaskMultiItemEvent`) — emission is task_06/07. Did NOT touch OpenAPI/generated TS: the event payload is opaque `json.RawMessage` on the API surface (confirmed it is absent from `openapi/compozy-daemon.json` and `web/src/generated`).

## Learnings
- `apicore.TaskRunMultipleItem` is a type alias for `contract.TaskRunMultipleItem`; the worktree item fields already existed from task_02, so only the event payload + daemon builder needed changes.
- Integration replay test seam: `rundb.RunDB.AppendSyntheticEvent` + `openRunDBForRunID` let a test inject worktree events into a real completed parent journal and re-run `RunMultipleSnapshot` to verify reconstruction through the real ListEvents path.
- gocritic `rangeValCopy` runs on `_test.go` too; iterate `snapshot.Items` by index (128-byte struct).
- Pre-existing lint blocker (unrelated to task_04): goconst `"none"` 3x in `internal/cli`. Fixed at root by `version.UnstampedCommit`.

## Files / Surfaces
- `pkg/compozy/events/kinds/task.go` (payload fields), `pkg/compozy/events/kinds/payload_compat_test.go` (old+new JSON tests).
- `internal/daemon/task_multi.go` (`applyTaskMultiItemMetadata` + builder switch), `internal/daemon/task_multi_test.go` (builder unit tests + journal replay integration test).
- `internal/version/version.go` + `internal/cli/daemon_commands.go` (out-of-scope goconst fix).

## Ready for Next Run
- `make fmt/lint/test/go-build` all green: lint 0 issues; full suite 3566 tests, 3 skipped. New-code coverage: `applyTaskMultiItemMetadata` 100%, builder `applyEvent` 81.8%.
- task_05/06/07: populate the payload worktree fields and emit them on existing `task.multi.*` item events BEFORE child launch; the snapshot already reconstructs them.
