# Daemon Core Lifecycle & Supervision — Comparative Analysis

Comparison scope: boot, service surface, lifecycle, shutdown, locks, info, process supervision.

- Reference repo (AGH): `/Users/pedronauck/dev/compozy/agh/internal/daemon/`
- Target repo (looper): `/Users/pedronauck/Dev/compozy/looper/internal/daemon/`

---

## 1. Quick Assessment — What looper already has right

The looper daemon inherits the core AGH patterns and, because looper's runtime surface is deliberately smaller (no sessions/network/hooks/automation/extensions/bridges/bundles/memory/skills/resources), most of AGH's mass does not apply. The pieces looper did port are sound:

- **Singleton lock**: `internal/daemon/lock.go` mirrors AGH's `lock.go` almost line-for-line. It uses `gofrs/flock.TryLock`, persists PID, recovers a `StalePID()`, and returns `errAlreadyRunning{pid}` when another owner holds the lock (`lock.go:14-101`). Looper tightens permissions from `0o644`/`0o755` to `0o600`/`0o700`, which is strictly better for a per-user daemon.
- **Info persistence**: `info.go` is a trimmed clone of AGH's `info.go` — atomic temp-file rename plus `fsync` of both file and parent directory (`info.go:77-123`). Looper adds a `State ReadyState` field (`starting`/`ready`/`stopped`) so that CLI probers can distinguish "lock held but not ready yet" from "ready" — AGH relies on the presence of `daemon.json` alone and treats any write as ready.
- **Atomic readiness transition**: `host.MarkReady` rewrites `daemon.json` with `State=ready` after `Prepare` succeeds (`boot.go:199-208`), and a late `SetHTTPPort` rewrites once the ephemeral listener binds (`boot.go:211-224`). This is better than AGH's single-shot write in `bootServers` (`agh/boot.go:1296-1307`).
- **Stale ownership cleanup**: `host.cleanupStaleRuntime` (`boot.go:252-280`) probes existing `daemon.json` for a healthy owner before removing it; it refuses to boot if one is still healthy. Same intent as AGH's `resolveStaleDaemonPID` (`agh/boot.go:356-372`) but folded into a single helper.
- **Process liveness probes**: `process_unix.go` and `process_windows.go` are equivalent to AGH's `procutil.Alive`. The Windows path uses `PROCESS_QUERY_LIMITED_INFORMATION` and checks `STILL_ACTIVE`, which matches AGH's behavior.
- **Service surface**: `service.go` implements `Status`/`Health`/`Metrics`/`Stop` over looper's focused concerns (active runs, workspaces, reconcile counters) and tracks `shutdownConflicts` as a Prometheus-style metric (`service.go:36,115-134`). Health degrades on `CrashEventFailures > 0`. AGH has no equivalent single daemon service type — its status is scattered across per-subsystem observers.
- **Separation of Host from Run**: `host.go` cleanly splits "bootstrap the singleton + transports" from the ad-hoc `daemon.Start/QueryStatus` used by the CLI. That split is *cleaner* than AGH's monolithic `Daemon.boot` (1100+ lines) which pins every subsystem into one struct.
- **Shutdown conflict semantics**: `RunManager.Shutdown` returns an `apicore.NewProblem(409, "daemon_active_runs", …)` when active runs are present and `--force=false` (`shutdown.go:59-69`). That's a richer UX signal than AGH emits.
- **Detached-context discipline**: `detachContext` (`run_manager.go:2501`) uses `context.WithoutCancel` so that a caller canceling the `Stop` RPC does not abort the drain/close in progress. AGH does not do this and relies on explicit timeouts.
- **Startup reconciliation**: `ReconcileStartup` (`reconcile.go:93-168`) marks interrupted runs crashed + appends synthetic `run_crashed` events + records `CrashEventFailures` for health degradation. AGH has `observer.Reconcile` for sessions (`agh/boot.go:1384-1392`); looper adapts this to runs, which is exactly right for its domain.

What's structurally stronger in looper than in AGH:

- One tight `Host` type (bootstrap-only) vs. a god-struct `Daemon` with ~30 fields and 11 `applyDefaults` helpers.
- A three-state `ReadyState` (`starting`/`ready`/`stopped`) so probes can tell bootstrap-in-flight apart from ready.
- Tighter file permissions (`0o600` lock, `0o700` dirs).

---

## 2. Gaps

Gaps are things AGH does that looper does not, where porting (or adapting) would concretely improve looper's supervision story.

### 2.1 HIGH — No signal handling in the daemon process

**AGH**: `Daemon.Run` (`agh/daemon.go:930-956`) installs `signal.Notify(ch, SIGINT, SIGTERM)` via `signalSource()` (`agh/daemon.go:1103-1113`), selects on context + signal, logs the received signal (`"daemon: received shutdown signal"`), then creates a 10-second timeout context (`defaultShutdownTimeout` at `agh/daemon.go:40`) and runs graceful shutdown. There is also `WithSignalBridge` (`agh/daemon.go:424-428`) so tests can inject fake signals deterministically.

**looper**: `daemon.Run` (`host.go:36-70`) only waits on `runCtx.Done()`. It never calls `signal.Notify`. Signal handling lives in the *CLI layer* (`signalCommandContext` in `internal/cli/command_context.go:17`). When the daemon process is launched via `--foreground` (from `internal/cli/daemon.go:148-154`), signals do reach it because the CLI sets up `signal.NotifyContext` before passing `ctx` into `runCLIDaemonForeground`. But when the daemon is launched *detached* via `launchCLIDaemonProcessWithExecutable` (`daemon_commands.go:226-242`), the parent returns after `command.Process.Release()` and the detached child never installs its own signal handler. A `SIGTERM` delivered to that PID will terminate the process with no call to `closeHostRuntime` — leaving `daemon.json`, the socket, and the flock artifacts on disk (they'll get cleaned up on next boot via `cleanupStaleRuntime`, but the current daemon state is silent about the crash).

**Why it matters for looper**: The RunManager drains runs and closes DBs in `closeHostRuntime` only when `runCtx.Done()` fires. If an operator types `kill` or `pkill compozy daemon` on the detached daemon PID, the graceful drain is skipped, SQLite writes in flight can be lost, and in-flight run rows remain in non-terminal states until the next `ReconcileStartup`.

**Suggested action** — *Port, adapted*. In `host.go:Run`, after `runCtx, stop := context.WithCancel(ctx)` wire a small signal source:

```go
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
defer signal.Stop(sigCh)

select {
case <-runCtx.Done():
case sig := <-sigCh:
    slog.Info("daemon: received shutdown signal", "signal", sig.String())
    stop()
}
```

Wrap the final `closeHostRuntime` call in a bounded shutdown context (see §2.2). Consider a `WithSignalBridge` option for tests (AGH precedent).

**Priority: High** — current detached daemon has no graceful exit on external signals.

---

### 2.2 HIGH — Shutdown has no bounded timeout / no shutdown context

**AGH**: `Daemon.Run` builds an explicit `shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)` (10s) before calling `d.Shutdown(shutdownCtx)` (`agh/daemon.go:952-955`). Every subsystem shutdown receives that deadline and can honor it.

**looper**: `closeHostRuntime` (`host.go:228-243`) calls `udsServer.Shutdown(context.Background())`, `httpServer.Shutdown(context.Background())`, `db.Close()`, `host.Close(context.Background())` with *no deadline*. If the HTTP listener has lingering long-lived streams (e.g. run event stream SSE), `httpServer.Shutdown` can hang indefinitely. `RunManager.Shutdown` has its own `shutdownDrainTimeout` (`shutdown.go:78`), but nothing above it enforces a whole-daemon ceiling.

**Suggested action** — *Port*. Introduce `defaultShutdownTimeout = 10 * time.Second` (or source it from `RunLifecycleSettings.ShutdownDrainTimeout` with a floor) and use `context.WithTimeout(context.Background(), …)` in `Run` before calling `closeHostRuntime`. Thread that ctx through the shutdown calls. This is a small change but eliminates the "daemon doesn't exit" failure mode.

**Priority: High**.

---

### 2.3 HIGH — No orphan/child process cleanup on boot

**AGH**: On boot, when `resolveStaleDaemonPID` (`agh/boot.go:356-372`) reports a stale PID, AGH calls `d.cleanupOrphans(ctx, stalePID)` (`agh/orphan.go:25-55`). This lists every process in the system via `ps -axo pid=,ppid=` (`listProcesses` at `agh/orphan.go:99-125`), filters those whose `PPID == stalePID`, sends `SIGTERM`, polls for exit with `orphanGraceWait = 2s` / `orphanPollWait = 100ms`, and escalates to `SIGKILL` if still alive. `cleanupOrphans` tolerates errors and aggregates them.

**looper**: No analogous logic. `host.cleanupStaleRuntime` (`boot.go:252-280`) only removes the stale socket and `daemon.json`. If a previous daemon crashed while spawning agent CLI subprocesses (Claude, Codex, Droid, Cursor) those child processes continue running, holding their own stdin/stdout pipes and likely their SQLite run-DB connections. The next daemon boot will see no owning process but will also not reap them.

**Why it matters for looper**: looper's `RunManager` spawns agent CLI processes (per `internal/cli/agent/*` and `internal/core/run/exec`). A crashed daemon leaves orphaned `claude`/`codex`/`droid`/`cursor` processes writing into the old run's artifact dir. On next boot, `ReconcileStartup` marks those runs crashed on disk, but the real agent subprocesses are still burning tokens and potentially still writing to `run.db`.

**Suggested action** — *Port*. Add `orphan.go` to looper with the same `cleanupOrphans` flow. Hook it into `cleanupStaleRuntime` right after detecting `StalePID() > 0`:

```go
if stalePID := h.lock.StalePID(); stalePID > 0 {
    if err := cleanupOrphans(ctx, stalePID, h.listProcesses, h.signalProcess, h.processAlive); err != nil {
        // log.Warn rather than fail — orphan cleanup is best-effort
    }
}
```

The implementation can lift `agh/orphan.go:99-125` directly; `ps -axo pid=,ppid=` is portable across macOS + Linux. Windows can short-circuit to `return nil, nil` or use `CreateToolhelp32Snapshot` if cross-platform parity is required.

**Priority: High** — crashed looper daemons currently leak agent subprocesses.

---

### 2.4 MEDIUM — No readiness signaling channel for callers inside the daemon process

**AGH**: `Daemon.readyCh chan struct{}` + `readyClosed bool` (`agh/daemon.go:303-304`) is closed once at the end of `publishBootState` (`agh/boot.go:1436-1439`). Tests block on `<-d.readyCh` to synchronize; harness tests rely on this. While it is mostly test-scaffolding in AGH, it generalizes to "internal components that need to wait for boot-complete without polling `daemon.json`".

**looper**: No in-process readiness channel. Internal consumers either rely on filesystem polling of `daemon.json` (as the CLI does in `daemon_commands.go:136-151`) or must be called after `Prepare` returns. That's fine for the current architecture (there are no in-process post-ready consumers), so this is lower priority.

**Suggested action** — *Skip for now, revisit if boot ordering grows*. Document the contract: "any consumer that needs to know when the daemon is ready goes through `Health`/`Status` over the transport, not an in-process channel." Add a test-only `Ready() <-chan struct{}` on `Host` only if an integration test needs it.

**Priority: Low**.

---

### 2.5 MEDIUM — No restart ("reload") story

**AGH**: Full restart pipeline (`agh/restart.go`):
- Persistent restart operation ledger in `~/.agh/restarts/*.json` with a state machine `pending → stopping → waiting_release → starting → ready|failed` (`agh/restart.go:40-47`, `restartTransitionAllowed` at `401-410`).
- `Daemon.RequestRestart` creates the operation, spawns a detached `agh daemon relaunch` helper via `procutil.SpawnDetachedLoggedProcess`, then sends `SIGTERM` to itself (`agh/restart.go:414-443`).
- `RunRelaunchHelper` (`agh/restart.go:576-671`) waits for the old daemon to release lock+socket+info, then launches the replacement with `AGH_INTERNAL_RESTART_OPERATION_ID` env var. The replacement, after its own boot, calls `markRestartReadyIfRequested` (`agh/restart.go:874-895`) to advance the ledger to `ready`.
- Validates `hasFreshDaemonInfo` (`agh/restart.go:168-173`) so we know the new daemon actually replaced the old PID and StartedAt.

**looper**: No restart command or ledger. Operators must `compozy daemon stop --force` then `compozy daemon start` manually, racing the filesystem cleanup and without a durable audit trail.

**Why it matters for looper**: Re-tuning config (`RunLifecycleSettings`), picking up a new binary version after upgrade, or rebinding the HTTP port all require a full cycle. Today, the CLI does not tell the operator whether the new daemon successfully took over. If the replacement boot fails (e.g. `ReconcileStartup` error), the old CLI users get a silent "no daemon" until the next probe.

**Suggested action** — *Adapt, simplified*. The full AGH ledger is overkill for looper's scope. A lighter port:

1. Add `compozy daemon restart` in `internal/cli/daemon.go` that: (a) reads current `daemon.json`, (b) calls `client.StopDaemon(ctx, force)`, (c) polls for `ReadyStateStopped` or missing `daemon.json`, (d) calls the existing `launchCLIDaemonProcess`, (e) waits for `Health().Ready`.
2. Only add the durable ledger if operators want "was the restart successful, and when" auditing. Looper's existing `shutdownConflicts` metric + a new `restarts_total` counter may be enough.
3. Skip the detached helper pattern — looper's CLI is the orchestrator, so steps (a)-(e) can run in-CLI with a bounded timeout. The AGH helper exists because AGH restart can be kicked off from *inside* the daemon via a transport RPC; looper's CLI is already the caller.

**Priority: Medium** — operator ergonomics and safe config reloads.

---

### 2.6 MEDIUM — Shutdown ordering is flat; no explicit phase split

**AGH**: `shutdownDetached` splits teardown into three explicit phases (`agh/daemon.go:1022-1085`):
1. `shutdownRuntimeWorkers`: dream, skills watcher, resource reconcile, extensions, automation, sessions, tasks. (These produce work; stop producers first.)
2. `shutdownServersAndHooks`: HTTP server, UDS server, bridges, network, hooks. (Stop I/O listeners + event fanouts.)
3. `shutdownPersistentResources`: remove `daemon.json`, close registry/DB, release lock, close logger. (Durable state last.)

Each phase accumulates errors via `appendWrappedError` (`agh/daemon.go:1087-1092`) and `errors.Join`s at the end. Crucially, phase 1 cancels all work *before* phase 2 stops the listener, so in-flight RPCs aren't cut off mid-response.

**looper**: `closeHostRuntime` (`host.go:228-243`) stops HTTP → UDS → DB → Host in a single flat sequence. It does not first quiesce the `RunManager` (active runs may still be calling into the DB when `db.Close()` fires, resulting in `sql: database is closed` errors in run logs). The `Service.Stop` path does trigger `runManager.Shutdown` via the transport, but *that* only runs when the CLI calls `StopDaemon`; a signal-driven shutdown (once §2.1 lands) would bypass it.

**Suggested action** — *Adapt*. In `closeHostRuntime`, split into phases:

```go
// Phase 1: quiesce work producers
if runtime.runManager != nil { _ = runtime.runManager.Shutdown(ctx, true /*force*/ ) }
// Phase 2: stop listeners
httpServer.Shutdown(ctx); udsServer.Shutdown(ctx)
// Phase 3: release persistent state
host.Close(ctx); db.Close()
```

The `RunManager` is already plumbed through `prepareHostRuntime` but is *not* retained in `hostRuntime`. Add it:

```go
type hostRuntime struct {
    db         *globaldb.GlobalDB
    runManager *RunManager
    udsServer  *udsapi.Server
    httpServer *httpapi.Server
}
```

**Priority: Medium** — prevents `sql: database is closed` races on daemon exit.

---

### 2.7 LOW — Run() returns `nil` on `StartOutcomeAlreadyRunning` without context-wait

**AGH**: `Daemon.Run` always reaches the select-on-signal block (`agh/daemon.go:943-950`). If another daemon is already running AGH never gets past `AcquireLock` so this path is N/A.

**looper**: `daemon.Run` (`host.go:62-66`) returns `nil` immediately when `result.Outcome == StartOutcomeAlreadyRunning`:

```go
if result.Outcome == StartOutcomeAlreadyRunning {
    return nil
}
```

This is *correct* for `compozy daemon start` (detach and exit). But if the caller invoked `daemon.Run` expecting to block (e.g. `compozy daemon start --foreground` on a machine where another daemon is already running), they get a silent `nil` return and the CLI exits with code 0, suggesting success. The CLI never re-probes the existing daemon's health.

**Suggested action** — *Adapt*. Either:
- (a) Return a sentinel error like `daemon.ErrAlreadyRunning` so the CLI can decide whether to report "daemon already running; exiting" vs. attaching to it, or
- (b) Block on `runCtx.Done()` regardless (the CLI's existing context wiring will cancel properly).

Option (a) is cleaner and matches the AGH contract (`agh/lock.go:17` exports `ErrAlreadyRunning`).

**Priority: Low** — current behavior is correct for the detached flow; only confusing for foreground.

---

### 2.8 LOW — No boundary (import-graph) verification

**AGH**: `Daemon.Boundaries` (`agh/boundary.go:19-45`) uses `go/parser` + `filepath.WalkDir` to verify that nothing under `internal/` imports `internal/daemon`, `internal/api/httpapi`, `internal/api/udsapi`, or `internal/cli` (`agh/boundary.go:62-67`). Gated by `AGH_DEV_VERIFY_BOUNDARIES=1` (`agh/boundary.go:47-58`). It runs at the tail of boot (`agh/boot.go:1394-1398`) and only logs warnings.

**Why AGH has it**: AGH is a large platform with many internal consumers; package boundaries drift.

**looper**: No boundary checker. Looper's `CLAUDE.md` uses `golangci-lint` to enforce structure (and the project is smaller), so this is largely redundant.

**Suggested action** — *Skip*. Looper already has zero-tolerance lint; add a `revive`/`depguard` rule in `.golangci.yml` if you want import-graph enforcement rather than a runtime boundary check.

**Priority: Low**.

---

### 2.9 LOW — `Service.Stop` drops signal errors; no shutdown telemetry

**AGH**: Shutdown errors are wrapped with `appendWrappedError` and returned from `Daemon.Shutdown` (`agh/daemon.go:1022-1028`). Any caller of `Run()` sees the joined error.

**looper**: `Service.Stop` (`service.go:138-149`) bumps `shutdownConflicts` on RunManager conflict (good), but when `requestStop` is called, any error from stopping servers/DB downstream is swallowed by `host.go:Run` which returns `closeHostRuntime(…)` — the return value actually does propagate, but no metric or slog record captures it. The `Service.Metrics` body exposes `daemon_shutdown_conflicts_total` but not `daemon_shutdown_errors_total`.

**Suggested action** — *Low-cost add*. Add a `shutdownErrors atomic.Int64` to `Service` and emit it as `daemon_shutdown_errors_total`. Log at error level in `closeHostRuntime` when any sub-shutdown returns non-nil (`slog.Error("daemon: shutdown subsystem failed", "subsystem", "http", "error", err)`).

**Priority: Low**.

---

### 2.10 LOW — No lifecycle logging of boot phases

**AGH**: `bootFinalize` emits `slog.Info("daemon: boot reconciliation complete", "indexed_sessions", ..., "orphaned_sessions", ...)` (`agh/boot.go:1388-1392`). Each shutdown phase has wrapped errors surfaced via `appendWrappedError`.

**looper**: `loadHostPersistence` + `prepareHostRuntime` are silent. `ReconcileStartup` returns a `ReconcileResult` but no slog happens. If `CrashEventFailures > 0` the health endpoint reports degraded — but there's no log line for the operator to inspect.

**Suggested action** — *Cheap fix*. After `ReconcileStartup` and after `MarkReady`, emit a structured log line:

```go
slog.Info("daemon: boot reconciliation complete",
    "reconciled_runs", reconcileResult.ReconciledRuns,
    "crash_events_appended", reconcileResult.CrashEventAppended,
    "crash_event_failures", reconcileResult.CrashEventFailures,
    "last_run_id", reconcileResult.LastReconciledRunID,
)
slog.Info("daemon: ready",
    "pid", host.info.PID,
    "http_port", host.info.HTTPPort,
    "socket_path", host.info.SocketPath,
    "version", host.info.Version,
)
```

**Priority: Low** — quality-of-life for operators reading `~/.compozy/daemon.log`.

---

## 3. Do NOT Recommend — AGH patterns intentionally skipped

- **`harness_context.go` (`agh/harness_context.go:175-528`)** — AGH's harness-context resolver normalizes session types, channels, synthetic turn metadata, detached run metadata, and emits observability tags. Looper has *no* sessions, no synthetic turns, no prompt providers. Every concept in that file is session-machinery. **Skip.**
- **`harness_detached_work.go` (`agh/harness_detached_work.go:1-633`)** — AGH's detached-harness work bridge exists because AGH sessions can enqueue side-effect tasks (via `taskpkg.Manager`) that wake another session later. Looper runs one task per `Run`, there is no session-to-session wake flow. The looper equivalent (if ever needed) would be completely different because looper's unit of work is a `Run` not a `Session`. **Skip.**
- **All `bootHooks`/`bootAutomation`/`bootBundles`/`bootExtensions`/`bootNetwork`/`bootResourceReconcile`/`bootSettings` phases (`agh/boot.go:794-1342`)** — every one of these subsystems is AGH-specific (extensions host-API, bridge runtime, automation store, bundle registry, network peer manager, skills watcher, consolidation service). Looper's boot legitimately does not need them. The looper boot-step analogue is `loadHostPersistence` (`host.go:112-145`) which does the minimum: open DB, load settings, reconcile startup. This is right-sized. **Skip.**
- **`ResolvedHarnessPolicy.ObservabilityTags` / `TelemetrySink`s** — AGH fans out hook telemetry across many sinks because many subsystems consume it. Looper's telemetry is the `Service.Metrics` endpoint. **Skip.**
- **`defaultDetachedStart` + `SpawnDetachedLoggedProcess` from `agh/restart.go:211-218`** — this is the AGH in-daemon spawning helper used for restart helper + consolidation spawner. Looper already has `launchCLIDaemonProcessWithExecutable` (`internal/cli/daemon_commands.go:202-242`) in the CLI layer; lifting this into the daemon would duplicate it. **Skip** unless the restart story (§2.5) evolves to require in-daemon spawning.
- **`notifier_test.go` fanout pattern** — tied entirely to AGH session lifecycle hooks. **Skip.**
- **Server interface abstraction (`agh/daemon.go:97-101`, `ServerFactory` at `agh/daemon.go:130`)** — AGH abstracts HTTP/UDS behind `Server` to ease testing. Looper uses the concrete `udsapi.Server` / `httpapi.Server` directly in `startHostTransports` (`host.go:187-226`). The abstraction is mostly scaffolding for AGH's much larger test surface; for looper's 4 boot steps, `startHostTransports` is clearer without it. **Skip**, unless test seams for transport errors become painful.

---

## 4. Summary Recommendation Table

| Gap | Action | Priority | Est. scope |
| --- | --- | --- | --- |
| 2.1 Signal handling inside daemon process | Port (adapt) | High | ~30 LOC in `host.go` |
| 2.2 Bounded shutdown timeout | Port | High | ~10 LOC in `host.go` |
| 2.3 Orphan/child process cleanup | Port | High | ~130 LOC new `orphan.go` |
| 2.4 In-process readiness channel | Skip (revisit) | Low | — |
| 2.5 Restart command + ledger | Adapt, lightweight | Medium | ~150 LOC CLI + optional ledger |
| 2.6 Phased shutdown ordering | Adapt | Medium | ~20 LOC in `host.go` |
| 2.7 `StartOutcomeAlreadyRunning` sentinel | Adapt | Low | ~5 LOC |
| 2.8 Import-boundary verification | Skip (use lint) | Low | — |
| 2.9 Shutdown error metric + log | Cheap add | Low | ~15 LOC |
| 2.10 Boot-phase slog lines | Cheap add | Low | ~10 LOC |

Together, 2.1 + 2.2 + 2.3 + 2.6 form the minimal "supervise the daemon properly" bundle: the daemon exits gracefully on signals, with a deadline, after quiescing work and reaping orphaned agent CLI children. That's the critical delta.

## Key file references

**looper (target):**
- `/Users/pedronauck/Dev/compozy/looper/internal/daemon/host.go` — bootstrap entry + `closeHostRuntime` (all HIGH gaps land here)
- `/Users/pedronauck/Dev/compozy/looper/internal/daemon/boot.go` — `Start`, `cleanupStaleRuntime`, `MarkReady`
- `/Users/pedronauck/Dev/compozy/looper/internal/daemon/shutdown.go` — RunManager drain
- `/Users/pedronauck/Dev/compozy/looper/internal/daemon/service.go` — transport-facing metrics (`shutdownConflicts`)
- `/Users/pedronauck/Dev/compozy/looper/internal/daemon/lock.go` — already good
- `/Users/pedronauck/Dev/compozy/looper/internal/daemon/process_unix.go`, `process_windows.go` — already good
- `/Users/pedronauck/Dev/compozy/looper/internal/cli/daemon_commands.go` — `launchCLIDaemonProcessWithExecutable` (relevant for restart)

**AGH (reference):**
- `/Users/pedronauck/dev/compozy/agh/internal/daemon/daemon.go:930-1143` — signal handling, shutdown phases, stopSessions pattern
- `/Users/pedronauck/dev/compozy/agh/internal/daemon/orphan.go:25-125` — orphan cleanup + `ps` lister (direct port candidate)
- `/Users/pedronauck/dev/compozy/agh/internal/daemon/restart.go:414-903` — restart operation ledger + relaunch helper
- `/Users/pedronauck/dev/compozy/agh/internal/daemon/boot.go:1269-1313` — boot servers + info write order
- `/Users/pedronauck/dev/compozy/agh/internal/daemon/boundary.go` — boundary checker (not recommended)
