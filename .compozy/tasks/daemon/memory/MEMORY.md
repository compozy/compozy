# Workflow Memory

Keep only durable, cross-task context here. Do not duplicate facts that are obvious from the repository, PRD documents, or git history.

## Current State
- `task_01` is complete: the daemon bootstrap now resolves home-scoped paths from `$HOME`, owns singleton lock/info cleanup, and exposes readiness through `internal/daemon`.
- `task_02` is complete: `internal/store/globaldb` now owns `global.db` bootstrap/migrations plus durable workspace/workflow/run identity with coverage for symlink-aware registration collapse, active-run unregister conflicts, concurrent register collapse, and reopen persistence.
- `task_03` is complete: `internal/store/rundb` now owns per-run `run.db` creation/migrations under `~/.compozy/runs/<run-id>/`, and the run journal persists canonical events plus projections there before syncing the compatibility `events.jsonl` mirror.
- `task_04` is complete: `internal/api/core` now owns the shared daemon transport contract, and `internal/api/httpapi` plus `internal/api/udsapi` expose the same route/resource model over localhost HTTP and UDS.
- `task_05` is complete: `internal/daemon.RunManager` now implements the daemon-owned run lifecycle for task, review, and exec flows, reusing the existing planner/executor/run-scope/runtime stack while persisting lifecycle state through `global.db` and `run.db`.
- `task_06` is complete: daemon startup now reconciles interrupted runs to `crashed` before readiness, `run.db` records synthetic `run.crashed` events only when the existing DB is still openable, forced stop returns `409` unless `force=true`, and terminal-run retention/purge is driven by home-scoped `[runs]` settings plus `compozy runs purge`.
- `task_07` is complete: `compozy sync` now reconciles authored workflow artifacts into `global.db` (`artifact_snapshots`, `task_items`, `review_rounds`, `review_issues`, `sync_checkpoints`) for single-workflow and workspace-wide scopes, with file-level coverage above 80% for `internal/core/sync.go` and `internal/store/globaldb/sync.go`.
- `task_08` is complete: daemon-managed task/review runs now perform a scoped pre-run sync after run allocation, own one debounced `fsnotify` watcher per active workflow, emit watcher-driven `artifact.updated` events through the existing run journal, and keep live Markdown edits synchronized into `global.db` without rewriting workflow `_meta.md` or generated `_tasks.md` in the touched run paths.
- `task_09` is complete: `compozy archive` now computes eligibility from synced `global.db` task/review/run projections, returns typed single-workflow conflicts for active or incomplete workflows, skips workspace-wide ineligible workflows deterministically, and archives into `.compozy/tasks/_archived/<timestamp-ms>-<shortid>-<slug>` while rolling the rename back if catalog archival persistence fails.
- `task_10` is complete: daemon-managed runs now bind a per-run extension host bridge and capability token into run-scope startup, extension-initiated child runs route back through `internal/daemon.RunManager`, review-provider resolution prefers the run-local runtime manager over a process-global overlay, and existing extension hook/JSON-RPC audit continues to persist through run-owned storage.
- `task_11` is complete: the CLI now has a daemon-client foundation with `internal/api/client`, daemon host bootstrap through `daemon.Run`, client-side workspace and attach-mode resolution for `compozy tasks run`, `runs.default_attach_mode` config precedence across global/workspace/CLI flags, auto-start/reuse of the home-scoped daemon, and root `compozy start` removal in favor of daemon-backed command families.
- `task_12` is complete: daemon-backed observation now restores dense run snapshots plus cursor resume state from `GET /runs/:run_id/snapshot`, follows `/runs/:run_id/stream` with heartbeat/overflow/reconnect handling, exposes `compozy runs attach` and `compozy runs watch`, and routes daemon-backed `tasks run` `ui|stream|detach` presentation through the same remote observation contract.
- `task_13` is complete: `pkg/compozy/runs` now uses daemon-backed list/snapshot/events/stream queries exclusively, preserves the public run summary/replay/watch surface, returns a stable daemon-unavailable error without filesystem fallback, and ships with `80.2%` package coverage plus passing `make verify`.

## Shared Decisions
- Later daemon tasks should preserve the `internal/config/home.go` + `internal/daemon/{boot,info,lock}.go` seam instead of reintroducing workspace-scoped runtime path ownership.
- Keep `run.json`, `events.jsonl`, and turn/job artifacts as compatibility outputs during the daemon migration even though `run.db` is now the primary operational store.
- Later daemon clients should depend on the shared transport contract in `internal/api/core` instead of transport-specific route tables; request IDs, `TransportError`, snapshot/events pagination, and SSE cursor semantics are now canonical there.
- Local daemon HTTP must stay bound to `127.0.0.1`, with the chosen ephemeral port persisted through the daemon host state; UDS remains the primary private transport and must keep `0600` socket permissions.
- Later daemon/client tasks should reuse `internal/daemon.RunManager` as the single run-lifecycle owner instead of rebuilding planning/execution or introducing a second active-run registry.
- Later daemon observation work should bridge daemon snapshots/SSE back into existing consumers (`uiMsg` for the cockpit, event streams for watch mode) instead of introducing a second UI/watch state model.
- Daemon lifecycle retention and shutdown bounds now live under the home-scoped `[runs]` config (`keep_terminal_days`, `keep_max`, `shutdown_drain_timeout`) with defaults of `14`, `200`, and `30s`.
- Sync cleanup in `task_07` is intentionally conservative: workflow `_meta.md` is removed, obviously generated/noncanonical `_tasks.md` is removed once, but canonical authored master task lists remain in the workspace and are snapshotted into `artifact_snapshots`.
- Pre-run workflow sync for daemon-managed task/review runs now happens after `run_id` reservation / `global.db` row creation but before watcher startup; this preserves duplicate `run_id` conflict semantics while still satisfying the techspec requirement that workflow runs sync before execution. Exec runs intentionally skip workflow sync.
- Extension-backed review providers under daemon-owned runs must resolve through the run-local runtime manager first; process-global provider overlays are unsafe once multiple daemon runs execute concurrently.
- Operator-facing workspace commands that accept `id-or-path` should resolve filesystem refs to registered workspace IDs on the client side before calling the current `/api/workspaces/:id` routes; raw path refs are fragile over route parameters even though the registry contract supports them.

## Shared Learnings
- The bootstrap safety net now includes integration coverage for killed-daemon stale-artifact recovery and same-home singleton reuse across unrelated workspaces.
- Public `pkg/compozy/runs` no longer has a workspace-filesystem compatibility seam; daemon transport is now the only read path, while `events.jsonl` remains a compatibility output for direct inspection by tests and tooling.
- Parallel tests that allocate persisted runs need either isolated `$HOME` or explicit test-specific `RunID` values to avoid cross-test collisions in the shared home-scoped run root.
- The daemon SSE contract now uses `RFC3339Nano|sequence` cursors consistently across `Last-Event-ID`, snapshot/event pagination, heartbeats, and overflow frames; later attach/watch clients should reuse those helpers instead of re-parsing ad hoc.
- Persisted `job_state.summary_json` alone is too thin for remote cockpit restore after later lifecycle updates; dense snapshot reconstruction should replay durable run history plus token-usage/session projections when a client needs faithful attach state.
- Terminal fallback events in daemon-owned runs must be appended through the durable journal path (`SubmitWithSeq`) before terminal state is read back from `run.db`; otherwise completion/cancel mirroring can race the async event writer.
- The daemon runtime contract now includes `RuntimeConfig.DaemonOwned`; any public/runtime mirrors, especially `sdk/extension.RuntimeConfig`, must stay aligned when that contract changes.
- Startup reconciliation must not create or auto-recover missing/corrupt `run.db` files. Synthetic `run.crashed` events are best-effort only when an existing per-run DB is still structurally openable; otherwise the durable signal lives in `global.db.runs.error_text`.
- Later watcher and archive tasks can treat `global.db.sync_checkpoints` plus the workflow/task/review projection tables as the operational sync truth; `sync` no longer calls `tasks.RefreshTaskMeta` or rewrites workflow metadata files.
- Watcher-triggered sync history now rides the existing `artifact.updated` runtime event + `run.db.artifact_sync_log` projection instead of a daemon-only side channel; later watch/attach surfaces should reuse that stream.
- Daemon-owned extension subprocesses now receive a per-run host capability token in the environment when daemon callbacks are available, and `host.runs.start` rejects daemon-owned calls if that token context is absent.
- Real daemon CLI integration on macOS needs a short home-scoped daemon root; long temp-home paths can exceed Unix socket path limits and fail before readiness is written.

## Open Risks

## Handoffs
- Transport and persistence tasks can rely on `daemon.Start` / `daemon.QueryStatus` with empty `HomePaths` resolving through `$HOME` only.
- Later daemon tasks should reuse `internal/store/globaldb` for workspace/workflow/run identity instead of reintroducing `cwd`-derived workspace resolution into new daemon paths.
- Formal QA planning and live execution now belong to `task_17` and `task_18`, with shared artifacts rooted under `.compozy/tasks/daemon/qa/`.
- Later daemon transport, snapshot, and reconcile work should read canonical event history and projections from `internal/store/rundb`; `events.jsonl` is now a compatibility mirror, not the source of truth.
- Later daemon manager, CLI, TUI, review-fix, exec, and `pkg/compozy/runs` migration tasks can now build against `internal/api/core` service interfaces and the aligned HTTP/UDS route set without introducing a second transport contract.
- Later daemon client, reconciliation, and migration tasks can use `internal/daemon.RunManager` snapshots, event replay+live watch, and idempotent cancel behavior as the canonical lifecycle surface for active runs.
- Later daemon client tasks can reuse `internal/daemon.Service` for status/health/metrics/stop semantics, while operator cleanup flows can call the new `compozy runs purge` surface instead of reimplementing retention selection.
- Later daemon CLI/TUI and reader tasks can build on `internal/api/client/runs.go`, `internal/core/run/ui.AttachRemote`, `internal/cli/run_observe.go`, and `pkg/compozy/runs.WatchRemote` for snapshot bootstrap plus reconnecting stream consumption.
- Downstream tests that only need durable event-order assertions should read the `events.jsonl` compatibility mirror directly or stand up daemon info; they should not expect `pkg/compozy/runs.Open` to read temp workspace layouts without daemon transport state.
- `task_08` and `task_09` should build on `internal/store/globaldb/sync.go` plus `internal/core/sync.go` rather than re-parsing workflow files again or assuming authored `_tasks.md` should be deleted.
- `task_09` still owns the archive rewrite away from review/workflow metadata file eligibility checks; `task_08` intentionally stopped rewriting metadata in active-run flows without taking over archive-state policy.
- Later review/exec migration tasks can build on `internal/daemon/extension_bridge.go`, `internal/core/extension/review_provider_runtime.go`, and the executor’s runtime-manager provider lookup instead of reviving CLI bootstrap overlays for extension-backed review providers.
- Later CLI/TUI migration tasks should extend `internal/cli/daemon_commands.go` and the daemon-backed `tasks run` contract rather than restoring the removed root `start` path or introducing a second direct-execution workflow surface.
