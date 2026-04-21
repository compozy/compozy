# Observability, Harness Context, Logging, Storage, Journal

Comparative analysis between **looper** (`/Users/pedronauck/Dev/compozy/looper`) and **AGH** (`/Users/pedronauck/dev/compozy/agh`).

Scope: structured logging, harness context/observability tagging, event journal durability, transcript/snapshot storage, file snapshotting, workrefs, settings surface, daemon `info.json`, health, and diagnostic dump.

---

## 1. Quick Assessment

### Where looper is already strong
- **Journal write pipeline** (`internal/core/run/journal/journal.go`) is more evolved than AGH's session event writer: it has single-owner write-loop, bounded inbox with `ErrSubmitTimeout` and backpressure counter, per-batch `persist → flush buffered writer → publish to live bus → fsync on terminal or force`, crash recovery that truncates partial trailing lines, and monotonic journal sequence reconciled against `rundb.CurrentMaxSequence()`. This is arguably better than AGH's `sessiondb` writer loop for durability-before-publish.
- **Shared SQLite primitives** (`internal/store/sqlite.go`, `schema.go`, `values.go`) are nearly verbatim copies of AGH's `internal/store/sqlite.go` + helpers. WAL, `synchronous=NORMAL`, corruption recovery, busy timeout — all present.
- **Split DB topology** (`globaldb/global.db` for cross-run index, `rundb/<run>/run.db` for per-run artifacts, `events.jsonl` alongside) is cleaner than AGH's `globaldb` + per-session `events.db`. The migration framework (`migrations.go`, version table, `SchemaTooNewError`) matches AGH 1:1.
- **Atomic daemon info.json write** (temp+rename+fsync+dir-sync) matches AGH.
- **Structured slog is used consistently** with `"component"` + `"run_id"` in journal and `DropsOnSubmit`/`EventsWritten` counters exposed via atomics.
- **Daemon `/daemon/metrics` Prometheus endpoint exists** in `internal/daemon/service.go` (`daemon_active_runs`, `daemon_registered_workspaces`, `daemon_shutdown_conflicts_total`, `runs_reconciled_crashed_total`). AGH does not have an equivalent dedicated metrics endpoint.

### Where looper lags AGH materially
- **No central logger package.** AGH has `internal/logger/logger.go` with `New(WithLevel, WithFile, WithMirrorToStderr)` producing a JSON slog handler, a close function, and `ParseLevel`. Looper just relies on `slog.Default()` everywhere and `cmd/compozy/main.go` never configures slog at all. The daemon never writes a structured log file to `~/.compozy/logs/daemon.log` even though `compozyconfig.HomePaths.LogFile` is defined.
- **No harness-context resolver.** AGH's `harness_context.go` is a single authoritative resolver that derives (session class, turn origin, sections, augmenters, reentry mode, detached-run mode) and emits a *diagnostic label* plus `map[string]string` observability tags (`harness.surface`, `harness.session_class`, `harness.turn_origin`, `harness.channel_bound`, `harness.synthetic_reason`, ...). Looper has nothing comparable — origin/intent of runs is reconstructed ad hoc inside `runSnapshotBuilder.applyEvent`.
- **No lifecycle-recorder that separates summary emission from storage writes.** AGH's `harness_observability.go` uses a `harnessLifecycleRecorder` with a pending queue keyed by session ID so startup summaries can be written even before the session row exists in the durable index. Looper has no read-side summary trail.
- **No observer pattern.** AGH's `internal/observe/observer.go` (892 LoC) owns aggregation of session registration, token stats, permission logs, hook runs, bridge health, task health, environment lifecycle events, and exposes them behind a `Health(ctx)` view for a shared `Health` struct with uptime, DB size, bridge aggregates, and task aggregates. Looper's `daemon.Service.Health()` is reduced to `Ready bool` + a single `startup_reconcile_warnings` detail — there is no uptime, DB size, or active-session breakdown.
- **No transcript package.** AGH has `internal/transcript/transcript.go` (canonical `agh.session.event.v1` schema, `Assembler` that folds persisted `SessionEvent`s into renderable `Message`s with tool-call lifecycle). Looper has `internal/core/run/transcript/model.go` but it is a UI ViewModel (append-only Bubble Tea state) — it does not provide canonical replay messages, and rundb's `transcript_messages` projection is not round-tripped through a dedicated assembler.
- **No filesnap / workref primitives.** AGH's `filesnap.Snapshot{ModTime,Size}` plus `Equal/Clone` is used to detect staleness of workflow artifacts on disk. AGH's `workref.PathRef/RootRef` normalize how workspace id+path is passed around transport and runtime. Looper reimplements the concept inline in `globaldb/sync.go` and passes `workspaceID` + `workflowRoot` as loose strings throughout `run_manager.go`.
- **Minimal settings surface.** AGH's `internal/settings/` exports a typed service with `GeneralRuntime`, `MemoryRuntime`, `SkillsRuntime`, `AutomationRuntime`, `NetworkRuntime`, `ObservabilityRuntime` providers, plus `ObservabilitySection{Config, Runtime, LogTailSupport}`. Looper's `internal/config/home.go` only resolves paths; there is no read-side "settings" that pulls daemon runtime status behind a typed interface. `compozy daemon settings`/status UX is not present.
- **No SQLite WAL checkpoint on close.** `store.Checkpoint()` exists in looper (`internal/store/sqlite.go:156`) but is never called. AGH calls `store.Checkpoint()` on `SessionDB.Close()` and `GlobalDB.Close()` to truncate WAL so long-running daemons don't accumulate a multi-GB `-wal` file.
- **No dedicated diagnostic dump.** AGH's `Observer.Health` returns DB sizes (global + per-session WAL + SHM included) and bridge/task aggregates; there is no looper equivalent.
- **`daemon.json` Info does not carry network/listener diagnostics.** AGH's `Info.Network *NetworkInfo { Enabled, Status, ListenerHost, ListenerPort }` gives `daemon status`/external tooling a network snapshot. Looper's `Info` only has PID/HTTPPort/State.
- **Harness-observability metrics are absent from the daemon metrics endpoint.** AGH aggregates bridge backlog/drop/failure counts, auth failures, delivery metrics, task ingress audit categories; looper reports 4 integers.
- **Logger is not run-scoped.** AGH wraps `slog.Logger` with `.With("component", "...", "session_id", ...)` at construction sites to scope logs. Looper mostly uses the package-level `slog.Default()` and appends kv pairs ad hoc. The journal does `slog.Warn(..., "component", "journal", "run_id", j.runID, ...)` manually on every line; only 2 call sites in the whole repo use `logger.With`.

---

## 2. Gaps — AGH reference, why, action, priority

Priority legend: **P0** = production risk or blocks shipping daemon as a long-running process; **P1** = silent regressions / cannot diagnose prod issues; **P2** = DX and cleanup; **P3** = nice-to-have polish.

### P0 — Central logger package with file sink and JSON output
- **AGH ref**: `/Users/pedronauck/dev/compozy/agh/internal/logger/logger.go` (`New(WithLevel, WithFile, WithMirrorToStderr) (*slog.Logger, closeFn, error)`, `ParseLevel`); wired in `agh/internal/daemon/boot.go:237`.
- **Why it matters**: Looper ships `compozyconfig.HomePaths.LogFile = ~/.compozy/logs/daemon.log` but never actually opens that file. Daemon logs go to stderr only. After `compozy daemon start` detaches, stderr is gone and the operator cannot diagnose anything. Every `slog.Warn/Error` in `journal.go`, `run_manager.go`, `watchers.go`, `manager.go`, `exec.go` evaporates into the void.
- **Action**:
  1. Add `internal/logger/logger.go` mirroring AGH with `WithLevel/WithFile/WithMirrorToStderr` and `ParseLevel`.
  2. In `internal/daemon/boot.go` (or the daemon start path) construct the logger before any runtime goroutine spawns, pass it down as a `*slog.Logger` dependency, and register its `closeFn` in the shutdown cleanup list.
  3. Add `Log.Level` + `Log.MirrorStderr` fields to whatever TOML schema drives `compozyconfig`.
  4. Replace `slog.Default()` calls in `internal/daemon/*`, `internal/core/run/journal/journal.go`, `internal/core/run/executor/*`, `internal/core/extension/*` with the injected logger scoped via `.With("component", "<name>")`.

### P0 — Call `store.Checkpoint()` on rundb + globaldb Close
- **AGH ref**: `agh/internal/store/globaldb/global_db.go:542`, `agh/internal/store/sessiondb/session_db.go:426` both call `store.Checkpoint(ctx, db)` during `Close`.
- **Why it matters**: Looper's daemon is intended to be long-lived. WAL-mode SQLite without periodic checkpointing grows the `-wal` companion unboundedly on write-heavy workloads (and looper writes every event plus projection rows). `rundb.Close` in `internal/store/rundb/run_db.go:118` just calls `r.db.Close()`. After hundreds of runs the global + per-run wal sidecars can reach multi-gigabyte sizes before the connection pool cycles.
- **Action**: In `rundb.Close()` and `globaldb.Close()`, call `store.Checkpoint(ctx, db)` (with a 5s drain context) before closing the handle. Also call it from a periodic timer in daemon lifecycle for the globaldb (every 10–30 min), gated by `context.Done`.

### P1 — Harness context resolver for run origin + observability tags
- **AGH ref**: `agh/internal/daemon/harness_context.go` (`HarnessContextResolver`, `ResolvedHarnessContext`, `ResolvedHarnessPolicy`, `buildHarnessObservabilityTags`).
- **Why it matters**: Looper's runs can originate from CLI form, daemon auto-trigger (reviews/sync), ACP reentry, or ad-hoc `exec`. Today `runSnapshotBuilder.applyEvent` reconstructs this implicitly from `JobQueuedPayload.IDE/TaskType`. There's no single authoritative place that decides which origin a run has, which diagnostic label it gets, or what observability tags it carries — so every log line, event summary, and metric must re-derive that context.
- **Action**:
  1. Add `internal/core/run/harness` (or similar) with a `RunContextResolver` producing `ResolvedRunContext{ Mode (task/review/exec), Origin (cli/daemon/reentry), Policy { IncludeProjections, DiagnosticLabel, ObservabilityTags map[string]string } }`.
  2. Feed `RunManagerConfig` with the resolver; every `activeRun` stores the resolved context and its `Tags` are injected into `slog.With(...)` and the event envelope's `Payload` metadata.
  3. Attach a `harness.*` tag prefix so future Prometheus scraping or TUI filtering is consistent.

### P1 — Observer aggregator for run + daemon health
- **AGH ref**: `agh/internal/observe/observer.go`, `agh/internal/observe/health.go`, `agh/internal/observe/bridges.go`, `agh/internal/observe/tasks.go`.
- **Why it matters**: Looper's `Service.Health` (`internal/daemon/service.go:85`) has no uptime, no DB size, no active-run counts by mode, no extension-health roll-up. Extensions, runs, and workspaces each live in their own cache but nothing aggregates them for `compozy daemon health`. Without this, `SIGUSR1`-style diagnostic dumps or a `--verbose` health check are impossible.
- **Action**: Add `internal/observe/observer.go` with:
  - `ActiveRuns`, `ActiveWorkflows`, `RegisteredWorkspaces` (already available via `RunManager.ActiveRunCount` and `globaldb.CountWorkspaces`).
  - `GlobalDBSizeBytes`, `RunDBSizeBytesTotal` using AGH's `databaseSize(path)` helper (includes `-wal`/`-shm`).
  - `UptimeSeconds`, `Version`, `StartedAt`.
  - Extension health roll-up from `extensions.Manager` (already produces `manager_health.go`).
  - Expose through `/daemon/health` JSON with richer details; keep `Ready` bool for the HTTP status.

### P1 — Prometheus metrics: journal drop counter, extension failures, reconcile counts
- **AGH ref**: `agh/internal/observe/bridges.go` (`DeliveryBacklog`, `DeliveryDroppedTotal`, `AuthFailuresTotal` aggregated), `agh/internal/observe/tasks.go` (origin/status totals).
- **Why it matters**: `journal.Journal.DropsOnSubmit()` already exists (`internal/core/run/journal/journal.go:337`), but `internal/daemon/service.go:Metrics` doesn't surface it. Same for extension spawn/health failures and watcher debounce drops. When a run silently drops 30% of events because an extension backpressures the fsync path, there is no way to see it.
- **Action**: Extend `Service.Metrics` to include per-run or aggregate counters:
  - `journal_events_written_total`, `journal_drops_on_submit_total`, `journal_buffer_depth`.
  - `extension_spawn_failures_total`, `extension_health_checks_failed_total`.
  - `run_reconcile_runs_total`, `run_reconcile_crash_event_failures_total` (already tracked, just not exposed).
  - `daemon_uptime_seconds`.

### P1 — Transcript canonical schema + Assembler
- **AGH ref**: `agh/internal/transcript/transcript.go` (`CanonicalSchema = "agh.session.event.v1"`, `Assembler.Assemble([]store.SessionEvent) []Message`).
- **Why it matters**: `rundb` has a `transcript_messages` table and `TranscriptMessageRow`, but no code path turns persisted rows into a renderable `Message` list (role, tool_call/tool_result pairing, thinking blocks). The TUI's `transcript.ViewModel` grows from in-memory `acp.SessionUpdate` deltas only; a cold `compozy runs show <id>` cannot reconstruct the conversation from disk. Headless CI replay and extension authors that want post-hoc analysis are both blocked.
- **Action**:
  1. Add `internal/core/run/transcript/canonical.go` with a `CanonicalEventPayload`, `CanonicalSchema = "compozy.run.event.v1"`, and `Assembler.Assemble(rows []rundb.TranscriptMessageRow) []transcript.Message`.
  2. Wire the projection inserts in `rundb.StoreEventBatch` to emit the canonical envelope (so replay is forward-compatible).
  3. Expose replay via `pkg/compozy/runs` for external callers.

### P2 — Lifecycle-recorder for harness/run summary writes with pending queue
- **AGH ref**: `agh/internal/daemon/harness_observability.go` — `harnessLifecycleRecorder` with `pending map[string][]store.EventSummary`, flushed on `OnSessionCreated`.
- **Why it matters**: Looper emits events from the moment a run is queued but the global DB `runs` row may not exist yet (`RunManager.queueRun` writes it synchronously, but observers could still fire before watchers are attached). A generic pending-queue keyed by run_id lets summary writes happen safely regardless of write ordering.
- **Action**: Add `internal/daemon/run_observability.go` that wraps `globaldb` summary writes through a pending map keyed by `runID`, flushed when the run's row is registered. Use it for the "degraded" warnings and reconcile summaries that currently live only in `ReconcileResult`.

### P2 — File snapshot (`filesnap`) for workflow artifacts
- **AGH ref**: `agh/internal/filesnap/filesnap.go`.
- **Why it matters**: `globaldb/sync.go` (looper) manually compares checksums to detect artifact changes. AGH's `filesnap.Snapshot{ModTime, Size}` + `Equal` gives an O(1) staleness check before you compute a checksum. For reproducibility of a run against the current workflow state, a snapshot captured at run-start lets the run store say "this ran against exactly these artifacts, same ModTime+Size on disk at end → reproducible; different → invalidated".
- **Action**:
  1. Add `internal/filesnap/filesnap.go` (essentially copy AGH's 60-LOC file).
  2. At run start, snapshot `workflowRoot` artifact paths; persist to `run.db` as a `run_artifact_snapshot` row.
  3. In `compozy runs show --verify`, re-snapshot and report drift.

### P2 — Workref value objects
- **AGH ref**: `agh/internal/workref/ref.go` (`PathRef{WorkspaceID, WorkspacePath}`, `RootRef{WorkspaceID, Workspace}`).
- **Why it matters**: Looper passes `(workspaceID string, workflowRoot string)` or `(workspaceID string, projectDir string)` tuples across `run_manager.go`, `watchers.go`, `runtime_config`, and the extension host API. Every refactor risks transposition errors (id vs path). A dedicated type eliminates the class of bug and gives JSON/YAML round-trip behavior for transport payloads.
- **Action**: Add `internal/workref/ref.go` 1:1 from AGH; replace the loose tuples in `RunManagerConfig`, `activeRun`, and `workspace_transport_service.go`.

### P2 — Structured logger scoping (`.With("component", "run_id", ...)`)
- **AGH ref**: AGH call sites consistently use `logger.With("component", "observer", "session_id", ...)` before passing down; the journal style in looper (`slog.Warn(..., "component", "journal", "run_id", j.runID, ...)`) is good but repeated on every line.
- **Why it matters**: Eliminating kv-duplication cuts log-line allocation and makes `grep '"component":"journal"'` work over the JSON file log.
- **Action**: Once the logger package lands, constructors take `*slog.Logger` and immediately scope: `logger = logger.With("component", "journal", "run_id", runID)`. Replace manual kv at every call.

### P2 — Settings service surface
- **AGH ref**: `agh/internal/settings/{service,sections,models}.go`.
- **Why it matters**: Looper has no read-side "what is daemon currently configured with?" beyond `DaemonStatus`. A typed settings service would give `compozy daemon settings`/`compozy config show` a well-defined output and make future extension configuration parity easier.
- **Action**: Defer until v2; stub out `internal/daemon/settings.go` with `GeneralRuntimeStatus` + `ObservabilityRuntimeStatus` interfaces if the surface is needed now.

### P3 — `daemon.json` carries network listener diagnostics
- **AGH ref**: `agh/internal/daemon/info.go` `Info.Network *NetworkInfo { Enabled, Status, ListenerHost, ListenerPort }`.
- **Why it matters**: Minor. Right now `compozy daemon status` cannot distinguish "daemon listening on 127.0.0.1:2323" vs "socket-only". Useful once the HTTP/TCP transport starts being advertised to extensions.
- **Action**: Extend `Info` with an optional `ListenerHost string`. Populate it from `SetHTTPPort`.

### P3 — Diagnostic dump (`SIGUSR1` or `compozy daemon dump`)
- **Why it matters**: A single command that writes `health + metrics + goroutine dump + recent log tail` into a zip under `~/.compozy/diagnostics/<timestamp>.zip` makes bug reports actionable. AGH does not have this either, but looper could leap-frog.
- **Action**: Build once the observer + logger file sink land.

---

## 3. Explicitly Skipped

These items are outside the analysis scope or intentionally deferred:

- **AGH `sessiondb` per-session event storage model** — looper's `events.jsonl + run.db` model is intentionally different (appendable canonical log + sqlite projection). No recommendation to migrate to `sessiondb`'s schema; just adopt the Checkpoint/close discipline.
- **AGH `globaldb_automation.go`, `global_db_bridges.go`, `global_db_network_*`, `hook_bindings.go`** — these cover AGH-specific domains (bridges, network, automation) that looper does not have. Not applicable.
- **AGH `observe/hooks.go`, `observe/reconcile.go`** — hooks/reconcile flow in looper is simpler; would re-audit separately alongside the daemon-runtime analysis.
- **AGH `harness_detached_work.go`, `harness_reentry_bridge.go`** — these drive synthetic reentry for AGH session wake. Looper doesn't have synthetic reentry today; harness-context resolver covers the foundational piece. Skip reentry bridge until `cy-daemon synthetic` is on the roadmap.
- **AGH `internal/settings/collections.go`, `classify.go`** — 30k+ LoC of domain-specific settings collection logic. Defer the full settings surface; ship the service skeleton only if needed.
- **AGH `tasks.go` task observability** — AGH has a first-class "task" abstraction distinct from looper's "run". Task dashboard/triage integration should be evaluated separately against looper's plan/review runtime.
- **OTEL / distributed tracing** — looper does not have distributed deployment; single-binary local-first design means stdlib `slog` with JSON file is sufficient for now.
- **Replacing `modernc.org/sqlite` with CGO sqlite** — both projects use the same pure-Go driver; no change recommended.

---

## Key file references

### looper
- `/Users/pedronauck/Dev/compozy/looper/internal/core/run/journal/journal.go` — single-owner write loop, fsync-on-terminal, durable before publish.
- `/Users/pedronauck/Dev/compozy/looper/internal/daemon/run_snapshot.go` — `runSnapshotBuilder.applyEvent` reconstructs run state from events.
- `/Users/pedronauck/Dev/compozy/looper/internal/daemon/info.go` — `Info{PID, Version, SocketPath, HTTPPort, StartedAt, State}` with atomic write.
- `/Users/pedronauck/Dev/compozy/looper/internal/daemon/service.go` — `Service.Health` / `Service.Metrics` (minimal today).
- `/Users/pedronauck/Dev/compozy/looper/internal/store/sqlite.go` — shared WAL/recovery helpers (includes `Checkpoint` but unused).
- `/Users/pedronauck/Dev/compozy/looper/internal/store/rundb/run_db.go` — per-run SQLite store (close path does not checkpoint).
- `/Users/pedronauck/Dev/compozy/looper/internal/store/rundb/migrations.go` — schema v1+v2 (with dead-index cleanup).
- `/Users/pedronauck/Dev/compozy/looper/internal/store/globaldb/migrations.go` — schema v1..v3.
- `/Users/pedronauck/Dev/compozy/looper/internal/config/home.go` — `HomePaths.LogFile` defined but never opened.
- `/Users/pedronauck/Dev/compozy/looper/internal/api/core/handlers.go` — `DaemonHealth`, `DaemonMetrics` handlers (thin wrappers over `daemon.Service`).
- `/Users/pedronauck/Dev/compozy/looper/cmd/compozy/main.go` — no logger setup, no slog default override.

### AGH reference
- `/Users/pedronauck/dev/compozy/agh/internal/logger/logger.go` — complete logger package (`New`, `WithLevel`, `WithFile`, `WithMirrorToStderr`, `ParseLevel`).
- `/Users/pedronauck/dev/compozy/agh/internal/daemon/boot.go:237` — logger wiring into daemon lifecycle with close registration.
- `/Users/pedronauck/dev/compozy/agh/internal/daemon/harness_context.go` — authoritative harness-context resolver.
- `/Users/pedronauck/dev/compozy/agh/internal/daemon/harness_observability.go` — `harnessLifecycleRecorder` with pending queue.
- `/Users/pedronauck/dev/compozy/agh/internal/daemon/harness_reentry_bridge.go` — synthetic-reentry correlation.
- `/Users/pedronauck/dev/compozy/agh/internal/daemon/info.go` — `Info.Network *NetworkInfo` diagnostics.
- `/Users/pedronauck/dev/compozy/agh/internal/observe/observer.go` — 892-LoC central observer (session registration, token aggregation, permission log, bridge state).
- `/Users/pedronauck/dev/compozy/agh/internal/observe/health.go` — `Health` struct with uptime, DB sizes, bridge + task aggregates.
- `/Users/pedronauck/dev/compozy/agh/internal/observe/bridges.go` — delivery-backlog / drop / failure aggregation.
- `/Users/pedronauck/dev/compozy/agh/internal/observe/tasks.go` — task summary/metrics read model.
- `/Users/pedronauck/dev/compozy/agh/internal/store/sqlite.go:155` — `Checkpoint(ctx, db) PRAGMA wal_checkpoint(TRUNCATE)` — same function exists in looper but is never called.
- `/Users/pedronauck/dev/compozy/agh/internal/store/globaldb/global_db.go:542` — `store.Checkpoint(ctx, g.db)` on close.
- `/Users/pedronauck/dev/compozy/agh/internal/store/sessiondb/session_db.go:426` — `store.Checkpoint(drainCtx, s.db)` on close.
- `/Users/pedronauck/dev/compozy/agh/internal/transcript/transcript.go` — canonical schema `agh.session.event.v1` and `Assembler`.
- `/Users/pedronauck/dev/compozy/agh/internal/filesnap/filesnap.go` — `Snapshot{ModTime, Size}` + `Equal`/`Clone`.
- `/Users/pedronauck/dev/compozy/agh/internal/workref/ref.go` — `PathRef`, `RootRef` value objects.
- `/Users/pedronauck/dev/compozy/agh/internal/settings/service.go` + `models.go` — typed settings service with per-domain runtime providers.
