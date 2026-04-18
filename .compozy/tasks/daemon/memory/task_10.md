# Task Memory: task_10.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Preserve executable extensions as run-scoped stdio JSON-RPC subprocesses while adapting host access, review-provider bridging, auditing, and shutdown ownership to daemon-managed task/review/exec runs.
- Required evidence: daemon-owned lifecycle wiring, per-run Host API capability token handling for daemon callbacks, persisted hook/JSON-RPC audit records in `run.db`, and tests that cover daemon-owned hook execution, host access rejection, review-provider behavior, and clean shutdown.

## Important Decisions
- Keep the extension subprocess transport unchanged and solve the daemon migration by changing ownership seams around runtime startup, host access, and teardown.
- Reuse existing audit persistence (`extensions.jsonl` + `journal.RecordHookRun` -> `run.db.hook_runs`) instead of inventing a second daemon-only audit store.
- Route daemon-owned `host.runs.start` calls through a narrow `DaemonHostBridge` injected into run-scope startup instead of teaching extensions a new transport or bypassing `internal/daemon.RunManager`.
- Resolve extension-backed review providers through the active run-local runtime manager before consulting the process-global provider registry so concurrent daemon runs do not fight over shared overlays.

## Learnings
- `internal/daemon.RunManager` already marks daemon-managed runs with `RuntimeConfig.DaemonOwned = true` and starts `scope.RunManager()`, so the missing work is not run startup itself but the extension-side host/service wiring.
- `internal/core/extension/defaultKernelOps.StartRun` still uses `kernel.Dispatch(...)`, which bypasses daemon-owned run lifecycle when an extension invokes `host.runs.start`.
- Daemon review runs do not currently mirror the CLI bootstrap that activates extension-backed review-provider overlays.
- The daemon bridge can stay entirely on the run side: a per-run capability token in the extension subprocess environment plus run-scoped bridge wiring is enough to preserve the existing SDK Host API surface.
- Verified package coverage after the task changes: `internal/core/extension` at `80.5%`, `internal/daemon` at `80.1%`.

## Files / Surfaces
- `internal/daemon/run_manager.go`
- `internal/daemon/extension_bridge.go`
- `internal/core/extension/runtime.go`
- `internal/core/extension/host_api.go`
- `internal/core/extension/host_helpers.go`
- `internal/core/extension/host_writes.go`
- `internal/core/extension/review_provider_bridge.go`
- `internal/core/extension/review_provider_runtime.go`
- `internal/core/extension/manager_spawn.go`
- `internal/core/run/executor/review_hooks.go`
- `internal/store/rundb/run_db.go`

## Errors / Corrections
- Pre-change gap correction to implement: daemon-managed extension child runs and review-provider overlays are not yet daemon-owned; current code still relies on local kernel/CLI bootstrap behavior in those paths.
- Corrected the review child-run workflow root to use the workflow directory rather than the round directory before sync/watch startup.
- Corrected daemon run-scope startup to bind the bridge context before `openRunScope(...)`; otherwise the extension manager never sees the daemon-owned callback seam.

## Ready for Next Run
- Task implementation and verification are complete. Downstream daemon tasks can assume run-local extension review-provider resolution and daemon-owned extension child-run startup are available.
