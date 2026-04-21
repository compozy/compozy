# Daemon Improvements — Consolidated Analysis

**Scope.** Comparative audit of looper's daemon (post-migration at `ab0d26c`) against the AGH reference daemon at `~/dev/compozy/agh`. Six parallel subagents produced domain-specific reports in this directory:

- [`analysis_core_lifecycle.md`](analysis_core_lifecycle.md) — boot, service, shutdown, locks, info, process supervision
- [`analysis_resources_reconcile.md`](analysis_resources_reconcile.md) — extensions, reconcile, bridges, watchers, hooks
- [`analysis_transport_api.md`](analysis_transport_api.md) — UDS/HTTP contracts, ACP bridging, SSE, client timeouts
- [`analysis_task_runtime.md`](analysis_task_runtime.md) — run manager, subprocess, journaling, resume, detached work
- [`analysis_observability_storage.md`](analysis_observability_storage.md) — logger, metrics, health, journal, checkpoint, transcript
- [`analysis_testing_harness.md`](analysis_testing_harness.md) — integration lanes, ACP faults, test utilities, benchmarks

This document consolidates the findings across all six reports and ranks them by blast radius.

---

## Executive summary

Looper's daemon is **structurally sound** — per-run journaling with backpressure, atomic info persistence, clean `Host`/`Run` separation, a real shutdown state machine, `ReconcileStartup` that emits `run.crashed` on interrupted runs, and tighter file permissions than AGH. In several places (journal pipeline, split DB topology, `/daemon/metrics` Prometheus endpoint, extension daemon-ownership enforcement) looper is **better** than AGH.

The gaps are concentrated in three themes:

1. **Resilience holes** inherited from AGH patterns that were not carried over — signal handling, shutdown deadline, orphan reaping, SQLite checkpoint, process-group kill, session-live awareness during boot reconcile, ACP stall detection.
2. **Observability thinness** — the daemon detaches but has no central logger; metrics counters exist but are not surfaced; `/daemon/health` returns `Ready bool` only; transcript projection has no render path.
3. **Test-harness maturity** — no reusable runtime harness, no ACP fault-injection fixture, no UDS↔HTTP parity tests, no integration lane separation.

AGH's platform-scale features (bridges, network, automation, MCP resource sync, memory subsystem, composed prompt assembler, resources.Kernel, Daytona) **do not apply** to looper's local-first single-binary CLI scope and should not be ported.

---

## Priority matrix

### P0 — critical (resilience + correctness)

| # | Gap | Area | AGH reference | Source report |
|---|-----|------|---------------|---------------|
| 1 | Detached daemon has no `signal.Notify` — no graceful exit on `SIGTERM` | core | `daemon.go:1103-1113` | core_lifecycle |
| 2 | `closeHostRuntime` uses `context.Background()` — a hanging HTTP stream blocks daemon exit forever | core | `daemon.go:40,952-955` (`defaultShutdownTimeout = 10s`) | core_lifecycle |
| 3 | No orphan reaping on boot — prior daemon crash leaves `claude`/`codex`/`droid`/`cursor` subprocesses alive | core | `orphan.go:25-125` (`cleanupOrphans`, `listProcesses` via `ps -axo pid=,ppid=`) | core_lifecycle |
| 4 | Central logger missing — `HomePaths.LogFile = ~/.compozy/logs/daemon.log` is defined but never opened; all `slog.Warn/Error` from detached daemon go nowhere | observability | `internal/logger/logger.go` (`New(WithLevel, WithFile, WithMirrorToStderr)`) | observability |
| 5 | `store.Checkpoint()` never called — function exists in looper verbatim from AGH, but neither `rundb.Close` nor `globaldb.Close` invokes it → unbounded `-wal` growth | observability | AGH calls it in both close paths | observability |
| 6 | Client timeout bug — single `defaultRequestTimeout = 5s` at `internal/api/client/client.go:20` applied to every `doJSON`, including sync/fetch/run-start that outlive 5s | transport | — | transport_api |
| 7 | Process-group kill missing — `internal/core/subprocess/process.go` terminates only the immediate pid; MCP/node children leak | transport | `procutil` + `Setpgid` | transport_api |
| 8 | Boot reconcile marks runs `crashed` even when the ACP grandchild is still alive (no session-live awareness in Phase 1) | task runtime | — | task_runtime |
| 9 | No subprocess stall detection on ACP agents — a hung Claude silently burns the entire `cfg.Timeout` | task runtime | — | task_runtime |

**Quick-win bundle (~200 LOC):** #1 + #2 + #3 + #5 + #6 + #7. Fixes the daemon's resilience profile without triggering a refactor.

### P1 — high value

#### Core lifecycle
- Phased quiesce-before-close ordering — `RunManager.Shutdown` before DB close (avoid `sql: database is closed` races).
- Lightweight CLI-driven restart (skip AGH's full restart ledger — overkill).

#### Task runtime
- `SubmitDetachedRun` analog of AGH `harness_detached_work.go` — fire-and-observe for long tasks.
- Typed hook pipeline with patch/deny semantics (AGH `hooks/dispatch.go`).
- Extend ACP resume (today only in exec mode — `run_manager.go:1026-1103`) to task/review runs. **Highest-leverage single improvement.**
- Richer boot recovery actions: `requeue`/`mark_running`/`fail` like AGH `planTaskRunRecovery` instead of a binary "crashed or not".
- Promote `ErrSubmitTimeout` on terminal events from warn-only to hard error.

#### Transport / API
- Create `internal/api/contract` package — DTOs are currently inline in `internal/api/core/interfaces.go`; handlers return anonymous structs (e.g. `runs.go:165`). Unblocks SSE decoder, testutil, spec gatekeeper.
- Reusable SSE decoder with max-line caps — AGH `internal/sse/decode.go`. Today parsing is inline in `internal/api/client/runs.go`.
- `MaskInternalErrors` knob for HTTP — 5xx currently leaks raw Go errors. Fine loopback-only, blocks any remote HTTP.
- Request logging middleware — today only `RequestIDMiddleware` + `ErrorMiddleware` are wired.
- UDS↔HTTP transport parity tests — HTTP has `httpapi/transport_integration_test.go` (1289 lines), UDS has no equivalent.
- `api/testutil` with reusable stubs — each handler test rebuilds fakes (`handlers_service_errors_test.go` is 652 lines of re-work).

#### Resources / reconcile
- Daemon-scoped extension registry with UDS `list/enable/disable/status` — today extension changes only take effect on the next run spawn.
- `ReviewProviderBridge` cache invalidation on manifest change.
- Per-run structured logging + event + extended metrics in reconciliation: `runs_reconciled_total`, `last_reconcile_timestamp_seconds`.

#### Observability
- `HarnessContextResolver` with `ResolvedHarnessPolicy`, `DiagnosticLabel`, and `ObservabilityTags` map — looper reconstructs run origin ad-hoc in `runSnapshotBuilder`.
- `Observer` aggregator for `/daemon/health` — today returns `Ready bool`; AGH exposes uptime, DB size, active sessions, bridge + task aggregates.
- Surface existing counters in `/daemon/metrics` — `Journal.DropsOnSubmit`, extension failures, reconcile counts all exist but are not in `Service.Metrics`.
- Canonical transcript Assembler — `rundb.transcript_messages` has a projection but no code path turning rows back into renderable messages. Headless replay is currently broken.

#### Testing harness
- `StartRuntimeHarness` analog — build binary, launch real daemon subprocess per test with isolated `$HOME`, expose typed HTTP/UDS/CLI clients. Looper's closest analog is one helper in `boot_integration_test.go`.
- ACP fault-injection fixture — AGH has a versioned JSON schema (`fixture.go`), an out-of-process driver binary, and a `DriverControlStep` vocabulary (disconnect, write_raw_jsonrpc, block_until_cancel). Looper's `installACPHelperOnPath` in `execution_acp_integration_test.go` re-execs the test binary via a shell script — crash-mid-stream and invalid-frame scenarios are uncoverable today.
- `//go:build integration` lane separation — looper has zero build tags; everything runs in one `make verify`.

### P2 — polish

- Periodic reconcile scan (today startup-only; SIGKILL'd child leaks a `running` row until next reboot).
- `filesnap` + `workref.PathRef`/`RootRef` for artifact staleness + reproducibility (today loose `(id, path)` tuples threaded everywhere).
- Logger scoping — only 2 uses of `logger.With` across the whole repo.
- `daemon.json` listener diagnostics.
- Completion notifier for non-CLI surfaces.
- `api/spec` gatekeeper to enforce contract snapshots on new routes.
- `b.ReportAllocs()` + `b.Loop()` on the single existing benchmark.
- Typed `actor`/`origin` on run writes.
- Artifact manifest (transcript/events/result capture for CI).
- Consolidate `waitForCondition`/`waitForRun`/`waitForString` helpers into `internal/testutil`.
- Daemon event summaries for preflight, sync, watcher.

---

## Explicitly NOT to port

Common ground across all six analyses — AGH's platform-scale surfaces that conflict with looper's local-first single-binary CLI scope:

- **Platform bridges & resources.** Bridge instances, secret bindings, hook binding resource store, agent/skill/bundle/automation/tool-MCP resource sync, composed prompt assembler, `resources.Kernel` + compensation-rollback machinery.
- **Remote execution.** Environment reconciliation (remote sandboxes), Daytona integration, network collaboration, multi-actor authority, dependency DAG, approval policy, session hook taxonomy.
- **AGH daemon scaffolding.** `harness_context.go`, `harness_detached_work.go`, `bootHooks`/`bootAutomation`/`bootBundles`/`bootExtensions`/`bootNetwork`/`bootResourceReconcile`/`bootSettings`, `ObservabilityTags`/`TelemetrySink` fanout, `Boundaries` import-graph checker, `Server` interface abstraction.
- **Test lanes.** Network collaboration, automation/webhook triggers, memory e2e, environment sandboxing, Daytona, bridge extensions, web browser lanes (~3,700 LOC of AGH integration tests with no looper mapping).
- **OTEL.** Out of scope for a local-first daemon.

Restart testing is gated on a product decision — porting makes sense only if zero-downtime restart lands on the roadmap.

---

## Recommended sequencing

1. **First PR — resilience bundle** (P0 #1, #2, #3, #5, #6, #7). ~200 LOC, no architectural moves, fixes correctness.
2. **Second PR — task-runtime hardening** (P0 #8, #9 + P1 ACP resume extension). The ACP stall + session-live pair eliminates the most common silent-failure class in looper's run pipeline.
3. **Third PR — observability lift** (P0 #4 central logger + P1 `Observer` aggregator + metrics surfacing + transcript Assembler). Makes the daemon debuggable in production.
4. **Fourth PR — contract + testutil refactor** (P1 `internal/api/contract` + `api/testutil` + SSE decoder). Unblocks the UDS↔HTTP parity tests and shrinks handler test files.
5. **Fifth PR — test harness** (P1 `StartRuntimeHarness` + ACP fixture). Targeted at the next time a crash-mid-stream regression ships.

Each PR is independently shippable and each unblocks the next without requiring cross-cutting refactors.
