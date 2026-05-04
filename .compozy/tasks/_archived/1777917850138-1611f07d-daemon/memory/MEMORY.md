# Workflow Memory

Keep only durable, cross-task context here. Do not duplicate facts that are obvious from the repository, PRD documents, or git history.

## Current State
- `task_01` through `task_06` are complete: the home-scoped daemon, shared REST transports, global/per-run SQLite state, singleton lifecycle, startup reconciliation, retention, and graceful shutdown are in place.
- `task_07` through `task_09` are complete: sync now reconciles authored workflow Markdown into `global.db`, active runs keep scoped watcher-driven sync alive, archive eligibility comes from synced DB state, and `_meta.md` is no longer the operational source of truth.
- `task_10` is complete: daemon-owned runs inject per-run extension host capability tokens, child runs route through `internal/daemon.RunManager`, and review-provider resolution prefers the run-local runtime manager.
- `task_11` is complete: CLI daemon bootstrap, workspace resolution/registration, and attach-mode resolution now happen client-side before daemon-backed task execution.
- `task_12` and `task_13` are complete: TUI attach/watch and `pkg/compozy/runs` use daemon-owned snapshots and streams instead of workspace-local run state.
- `task_14`, `task_15`, and `task_17` (formerly `task_16` before the perf-task insertion) are complete: daemon/workspaces/sync/archive/reviews/exec command families are daemon-backed, review Markdown remains authoritative in the workspace, exec I/O contracts stay compatible, and the migration closeout docs/regression cleanup landed.
- `task_16` is complete: the daemon performance optimization pass landed the storage/runtime/streaming/TUI/CLI fixes from `.compozy/tasks/daemon/perf_analysis/` while preserving journal, SSE, `run.db`/`global.db`, and CLI-help contracts.

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
- User-facing docs and fixtures should treat `compozy tasks run`, `compozy reviews fix`, `compozy daemon`, `compozy workspaces`, and `compozy runs attach|watch` as the canonical daemon surface; legacy top-level review commands are compatibility aliases only, and `compozy start` must not return as documented behavior.

## Shared Learnings
- The bootstrap safety net now includes integration coverage for killed-daemon stale-artifact recovery and same-home singleton reuse across unrelated workspaces.
- Public `pkg/compozy/runs` no longer has a workspace-filesystem compatibility seam; daemon transport is now the only read path, while `events.jsonl` remains a compatibility output for direct inspection by tests and tooling.
- Parallel tests that allocate persisted runs need either isolated `$HOME` or explicit test-specific `RunID` values to avoid cross-test collisions in the shared home-scoped run root.
- The daemon SSE contract now uses `RFC3339Nano|sequence` cursors consistently across `Last-Event-ID`, snapshot/event pagination, heartbeats, and overflow frames; later attach/watch clients should reuse those helpers instead of re-parsing ad hoc.
- Persisted `job_state.summary_json` alone is too thin for remote cockpit restore after later lifecycle updates; dense snapshot reconstruction should replay durable run history plus token-usage/session projections when a client needs faithful attach state.
- Terminal fallback events in daemon-owned runs must be appended through the durable journal path (`SubmitWithSeq`) before terminal state is read back from `run.db`; otherwise completion/cancel mirroring can race the async event writer.
- Daemon-backed exec clients must treat the streamed terminal event as an early signal only; the durable terminal boundary is the subsequent terminal `global.db` snapshot after `scope.Close()` drains extension shutdown and audit writes.
- The daemon runtime contract now includes `RuntimeConfig.DaemonOwned`; any public/runtime mirrors, especially `sdk/extension.RuntimeConfig`, must stay aligned when that contract changes.
- Session-update observer hooks now carry the update body as `json.RawMessage` internally so the ACP hot path marshals the public update once while preserving the existing JSON schema delivered to extensions.
- Startup reconciliation must not create or auto-recover missing/corrupt `run.db` files. Synthetic `run.crashed` events are best-effort only when an existing per-run DB is still structurally openable; otherwise the durable signal lives in `global.db.runs.error_text`.
- Later watcher and archive tasks can treat `global.db.sync_checkpoints` plus the workflow/task/review projection tables as the operational sync truth; `sync` no longer calls `tasks.RefreshTaskMeta` or rewrites workflow metadata files.
- Watcher-triggered sync history now rides the existing `artifact.updated` runtime event + `run.db.artifact_sync_log` projection instead of a daemon-only side channel; later watch/attach surfaces should reuse that stream.
- Daemon-owned extension subprocesses now receive a per-run host capability token in the environment when daemon callbacks are available, and `host.runs.start` rejects daemon-owned calls if that token context is absent.
- Real daemon CLI integration on macOS needs a short home-scoped daemon root; long temp-home paths can exceed Unix socket path limits and fail before readiness is written.
- CLI tests that rely on isolated in-process daemon state should pin `HOME`, `XDG_CONFIG_HOME`, `COMPOZY_TEST_CLI_DAEMON_HOME`, and `COMPOZY_TEST_CLI_XDG_CONFIG_HOME` explicitly; relying on ambient package-level env during parallel runs can hide reusable-agent failures.
- Extension and reader docs must describe daemon-owned runtime storage accurately: run inspection is daemon-backed, and extension audit records are durable in `~/.compozy/runs/<run-id>/run.db` (`hook_runs`) rather than workspace-local JSONL paths.
- Extension observer ordering now relies on an explicit executor barrier: `run.post_start` hooks must drain before execution can enter `job.pre_execute`, otherwise hook order becomes load-dependent and extension lifecycle tests can fail under suite load.

## Open Risks

## Handoffs
- Transport and persistence tasks can rely on `daemon.Start` / `daemon.QueryStatus` with empty `HomePaths` resolving through `$HOME` only.
- Later daemon tasks should reuse `internal/store/globaldb` for workspace/workflow/run identity instead of reintroducing `cwd`-derived workspace resolution into new daemon paths.
- Formal QA planning and live execution now belong to `task_18` and `task_19` (formerly `task_17` and `task_18`), with shared artifacts rooted under `.compozy/tasks/daemon/qa/`.
- Later daemon transport, snapshot, and reconcile work should read canonical event history and projections from `internal/store/rundb`; `events.jsonl` is now a compatibility mirror, not the source of truth.
- Later daemon manager, CLI, TUI, review-fix, exec, and `pkg/compozy/runs` migration tasks can now build against `internal/api/core` service interfaces and the aligned HTTP/UDS route set without introducing a second transport contract.
- Later daemon client, reconciliation, and migration tasks can use `internal/daemon.RunManager` snapshots, event replay+live watch, and idempotent cancel behavior as the canonical lifecycle surface for active runs.
- Later daemon client tasks can reuse `internal/daemon.Service` for status/health/metrics/stop semantics, while operator cleanup flows can call the new `compozy runs purge` surface instead of reimplementing retention selection.
- Later daemon CLI/TUI and reader tasks can build on `internal/api/client/runs.go`, `internal/core/run/ui.AttachRemote`, `internal/cli/run_observe.go`, and `pkg/compozy/runs.WatchRemote` for snapshot bootstrap plus reconnecting stream consumption.
- Downstream tests that only need durable event-order assertions should read the `events.jsonl` compatibility mirror directly or stand up daemon info; they should not expect `pkg/compozy/runs.Open` to read temp workspace layouts without daemon transport state.
- Later review/exec migration tasks can build on `internal/daemon/extension_bridge.go`, `internal/core/extension/review_provider_runtime.go`, `internal/api/client`, and the daemon-backed command surface from task 14 instead of reviving CLI bootstrap overlays or local execution paths.
- Later CLI/TUI migration tasks should extend `internal/cli/daemon_commands.go` and the daemon-backed `tasks run` contract rather than restoring the removed root `start` path or introducing a second direct-execution workflow surface.
