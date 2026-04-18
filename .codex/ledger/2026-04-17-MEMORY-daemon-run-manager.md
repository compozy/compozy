Goal (incl. success criteria):

- Implement daemon task `05` (`Daemon Run Manager`) end to end.
- Success means: daemon-owned task/review/exec lifecycle reuses the existing planner/executor/run-scope/runtime stack, persists run allocation in `global.db` + `run.db` before child execution, exposes snapshot/watch/cancel operations, rejects duplicate `run_id`, allows different concurrent runs, mirrors terminal state into both stores, passes the required tests, and clears `make verify`.

Constraints/Assumptions:

- Must follow `AGENTS.md`, `CLAUDE.md`, `.compozy/tasks/daemon/_techspec.md`, `.compozy/tasks/daemon/_tasks.md`, `.compozy/tasks/daemon/task_05.md`, and ADRs `001`, `002`, and `004`.
- Required skills active for this run: `cy-workflow-memory`, `cy-execute-task`, `golang-pro`, `testing-anti-patterns`, `systematic-debugging`, `no-workarounds`, `cy-final-verify`. The approved daemon techspec is the design baseline for this implementation turn.
- Keep scope tight to task `05`; later CLI/TUI and review-fetch/client migration work stays out of this task unless required by the start/snapshot/watch/cancel contract.
- Do not touch unrelated dirty files in the worktree. No destructive git commands without explicit user permission.

Key decisions:

- Reuse the existing kernel run-start flow, `RunScope`, planner, executor, journal, and extension runtime instead of duplicating planning/execution inside the daemon manager.
- Treat `internal/api/core.RunService` plus the task/review/exec start surfaces as the transport boundary the daemon manager must satisfy.
- Use `globaldb.Run` + `rundb.RunDB` as the durable lifecycle sources, with `events.jsonl` and existing run artifacts preserved as compatibility outputs via the existing journal/result writers.

State:

- Completed after implementation, daemon-focused validation, full `make verify`, and tracking/memory updates.

Done:

- Read workspace instructions, required skill files, workflow memory, task docs, `_techspec.md`, `_tasks.md`, and ADRs `001`, `002`, and `004`.
- Scanned relevant daemon ledgers for cross-agent awareness.
- Implemented the daemon-owned run manager in `internal/daemon/run_manager.go` for task, review, and exec flows, reusing the existing planner/executor/run-scope/runtime stack while persisting lifecycle state through `global.db` and `run.db`.
- Added lifecycle/concurrency coverage in `internal/daemon/run_manager_test.go`, including duplicate `run_id` conflicts, parallel runs, dense snapshots, idempotent cancel, open-run-scope failure cleanup, and exec parity.
- Fixed terminal fallback durability by appending synthetic terminal events with `SubmitWithSeq(...)` before re-reading `run.db`.
- Synced the public SDK runtime mirror by adding `DaemonOwned` to `sdk/extension.RuntimeConfig`.
- Verified:
  - `golangci-lint run ./internal/daemon/...`
  - `go test ./internal/daemon -run 'TestRunManager|TestNewRunManager|TestHost|TestResolveTerminalState|TestRunManagerHelperEdgeCases' -count=1`
  - `go test -cover ./internal/daemon` (`81.2%`)
  - `go test ./internal/daemon ./internal/api/core ./internal/store/globaldb ./internal/store/rundb ./internal/core/run/exec ./internal/core/model`
  - `go test ./sdk/extension -run TestPublicHookAndHostTypesStayAlignedWithRuntime -count=1`
  - `make verify`

Now:

- Prepare the local code-only commit and final handoff.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-17-MEMORY-daemon-run-manager.md`
- `.compozy/tasks/daemon/memory/{MEMORY.md,task_05.md}`
- `.compozy/tasks/daemon/{task_05.md,_tasks.md,_techspec.md}`
- `.compozy/tasks/daemon/adrs/{adr-001.md,adr-002.md,adr-004.md}`
- `internal/daemon/{run_manager.go,run_manager_test.go}`
- `sdk/extension/types.go`
- Commands: `rg`, `sed`, `go test`, `golangci-lint`, `make verify`
