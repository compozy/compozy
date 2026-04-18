Goal (incl. success criteria):

- Implement daemon task `10` (`Extension Runtime Daemon Adaptation`) end to end.
- Success means: executable extensions remain run-scoped stdio JSON-RPC subprocesses; daemon-managed task/review/exec runs own extension startup and shutdown; daemon-owned Host API callbacks use an explicit per-run capability token model; extension hook / Host API / JSON-RPC failures persist into `run.db` hook audit state without crashing unrelated runs; review-provider bridge behavior still works in daemon-managed review runs; required tests and `make verify` pass.

Constraints/Assumptions:

- Must follow repo instructions from `AGENTS.md` / `CLAUDE.md`, daemon task docs under `.compozy/tasks/daemon/`, and ADRs `002` and `003`.
- Required skills for this run: `cy-workflow-memory`, `cy-execute-task`, `brainstorming`, `golang-pro`, `testing-anti-patterns`, and `cy-final-verify`. If verification/debugging exposes bugs, use `systematic-debugging` and `no-workarounds`.
- Keep scope tight to task `10`; do not fix unrelated dirty files or reset existing task-tracking churn in the worktree.
- The workspace is already dirty. Do not touch unrelated files in `.compozy/tasks/daemon/` or other ledgers unless this task explicitly requires updates.

Key decisions:

- Use the existing extension subprocess/stdin JSON-RPC model; do not introduce a new transport for extensions.
- Treat the main daemon gap as ownership, not protocol: daemon-managed runs already set `RuntimeConfig.DaemonOwned`, but extension host callbacks still use local `DefaultKernelOps` and `host.runs.start` bypasses `internal/daemon.RunManager`.
- Preserve current audit persistence paths (`AuditLogger` -> `journal.RecordHookRun` -> `run.db.hook_runs`) instead of inventing a second daemon-only audit sink.
- Prefer a narrow daemon bridge seam injected into extension runtime startup over broad rewrites of the planner/executor or SDK Host API surface.

State:

- Implemented and verified; tracking and commit remain.

Done:

- Read repository guidance, required skill files, workflow memory, task docs, `_techspec.md`, `_tasks.md`, and ADRs `002` / `003`.
- Scanned relevant daemon/extension ledgers for cross-agent awareness.
- Confirmed current worktree is dirty before edits.
- Confirmed daemon techspec requires per-run Host API capability tokens for daemon callbacks while preserving stdio JSON-RPC subprocess boundaries.
- Identified pre-change gaps in current code:
  - `internal/daemon.RunManager` sets `RuntimeConfig.DaemonOwned = true` and starts run-scoped extension runtimes, but `internal/core/extension` still constructs local `DefaultKernelOps`.
  - `host.runs.start` still routes through `kernel.Dispatch(...)`, so nested extension-started runs bypass daemon-owned run lifecycle/persistence.
  - Daemon run paths do not activate the CLI-only extension review-provider overlay bootstrap, so extension-backed review providers are not preserved automatically for daemon-managed review flows.
- Implemented `internal/daemon/extension_bridge.go` and wired `RunManager.startRun(...)` to inject a per-run daemon host bridge into extension run-scope startup.
- Routed daemon-owned `host.runs.start` through that bridge, added explicit missing-token errors, and injected the per-run host capability token into extension subprocess environments.
- Added run-local review-provider resolution in `internal/core/extension/review_provider_runtime.go` plus executor lookup that prefers the active runtime manager over the global provider overlay registry.
- Extended test coverage across extension runtime, executor provider lookup, and daemon bridge task/review/exec paths.
- Verified:
  - `go test ./internal/core/extension -count=1`
  - `go test ./internal/core/run/executor -count=1`
  - `go test ./internal/daemon -count=1`
  - `go test -cover ./internal/core/extension` (`80.5%`)
  - `go test -cover ./internal/daemon` (`80.1%`)
  - `make verify`

Now:

- Update task tracking, review the diff, and create the local commit.

Next:

- Create one local commit with the task-local code and required tracking updates.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-17-MEMORY-extension-runtime-daemon.md`
- `.compozy/tasks/daemon/memory/{MEMORY.md,task_10.md}`
- `.compozy/tasks/daemon/{task_10.md,_tasks.md,_techspec.md}`
- `.compozy/tasks/daemon/adrs/{adr-002.md,adr-003.md}`
- `internal/daemon/{run_manager.go,extension_bridge.go,run_manager_test.go}`
- `internal/core/extension/{runtime.go,host_api.go,host_helpers.go,host_reads.go,host_writes.go,review_provider_bridge.go,review_provider_runtime.go,manager_spawn.go,manager.go,manager_test.go,host_writes_test.go,review_provider_runtime_test.go}`
- `internal/store/rundb/run_db.go`
- `internal/core/run/{executor/hooks.go,executor/review_hooks.go,executor/execution_test.go,exec/hooks.go}`
- `sdk/extension/{host_api.go,types.go,extension.go}`
- Commands: `rg`, `sed`, `git status --short`, `go test`, `make verify`
