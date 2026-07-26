---
name: 01-daemon-boot
description: QA child report — daemon composition root, boot, lock, shutdown, recovery, migrations, signal handling, version compatibility, subprocess lifetime.
type: qa-child
module: daemon-boot
sources:
  - internal/daemon
  - internal/diagnostics
  - internal/heartbeat
  - internal/store
  - internal/version
  - cmd/agh
  - internal/procutil
  - internal/subprocess
---

# Daemon + Boot — Final QA Plan

## 1. Module Surface

This module owns every byte of state the daemon writes to disk, the only file lock used to enforce singleton ownership, the boot pipeline that wires every other package, and the shutdown order that releases everything. Every claim below cites a `file:line`.

### 1.1 Binary entry point

- `cmd/agh/main.go:12-18` — `main` calls `cli.ExecuteContext(ctx, args, stdout, stderr)` and returns its exit code via `os.Exit`. There is no other entry point. Build metadata (`Version`, `Commit`, `BuildDate`) is injected via `-ldflags` into `internal/version/version.go:8-12`.

### 1.2 Daemon CLI verbs (top-level `agh daemon …`)

Defined in `internal/cli/daemon.go:28-148`. All return structured output via `writeCommandOutput` (`-o json|toon|human`).

| Verb                  | File:line                          | Behavior                                                                                                                                                                                                       |
| --------------------- | ---------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `agh daemon start`    | `internal/cli/daemon.go:41-70`     | Spawns detached child by default; `--foreground` runs in current terminal; hidden `--internal-child` flag (`internalChildFlagName`) is set by the parent so the spawned child runs the foreground path.        |
| `agh daemon relaunch` | `internal/cli/daemon.go:72-91`     | Hidden helper that reads `AGH_INTERNAL_RESTART_OPERATION_ID` (`internal/daemon/restart.go:21-22`) and drives the in-place upgrade replacement step.                                                            |
| `agh status`   | `internal/cli/daemon.go:93-115`    | Tries the live UDS client first, falls back to `daemon.json` + `procutil.Alive` probe. Returns `DaemonStatus` (`internal/cli/daemon.go:318-336`).                                                              |
| `agh daemon stop`     | `internal/cli/daemon.go:117-148`   | Reads `daemon.json`, sends `SIGTERM` to the recorded PID via `deps.signalProcess`, polls for stop via `daemonInfo` (`internal/cli/daemon.go:302-316`) and the UDS status endpoint.                             |

### 1.3 HTTP/UDS endpoints owned by the daemon module

| Endpoint                          | File:line                                          | Behavior                                                                                                                          |
| --------------------------------- | -------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| `GET /api/observe/health`         | `internal/api/httpapi/routes.go:118` → `internal/api/core/handlers.go:868-893` | Returns `HealthResponse{Health, Memory, Automation}`. Composes observer health + memory + automation health.                       |
| `GET /api/daemon/status`          | `internal/api/httpapi/routes.go:251` → `internal/api/core/handlers.go:895-925` | Returns `DaemonStatusResponse` including session counts, network status, version.                                                  |
| UDS `GET /observe/health`         | `internal/api/udsapi/routes.go:140`                | Same handler, UDS transport.                                                                                                      |
| UDS `GET /daemon/status`          | `internal/api/udsapi/routes.go:288`                | Same handler, UDS transport.                                                                                                      |
| HTTP `GET /api/sessions/:id/health` | `internal/api/httpapi/routes.go:72`              | Per-session health derived from `session.HealthStore` and HEARTBEAT.md state.                                                     |
| HTTP `GET /api/bridges/health/stream` | `internal/api/httpapi/routes.go:43`            | SSE stream of bridge runtime health.                                                                                              |
| HTTP `GET /api/memory/health`     | `internal/api/httpapi/routes.go:239`               | Memory subsystem health.                                                                                                          |

### 1.4 Environment variables consumed at boot

- `AGH_HOME` — overrides daemon home root (`internal/config/home.go:62-75`).
- `AGH_INTERNAL_RESTART_OPERATION_ID` — relaunch helper picks up the persisted restart operation id (`internal/daemon/restart.go:21-22`).
- `AGH_DEV_VERIFY_BOUNDARIES` — opt-in best-effort import boundary verification at boot (`internal/daemon/boundary.go:51-58`).
- `HOME` — fallback path used by `internal/config/home.go:69-74` and `ResolveUserAgentsSkillsDir` (`internal/config/home.go:170-192`).

### 1.5 File paths owned by AGH_HOME

All defined in `internal/config/home.go:11-114`:

| Path                         | Constant                          | Purpose                                                                                                  |
| ---------------------------- | --------------------------------- | -------------------------------------------------------------------------------------------------------- |
| `<AGH_HOME>/agh.db`          | `DatabaseName` (line 25)          | Global SQLite catalog; opened via `globaldb.OpenGlobalDB`.                                                |
| `<AGH_HOME>/daemon.lock`     | `DaemonLockName` (line 29)        | Singleton advisory file lock + PID file (`internal/daemon/lock.go:50-106`).                              |
| `<AGH_HOME>/daemon.json`     | `DaemonInfoName` (line 31)        | Discovery record `{pid, port, started_at, network}` written atomically (`internal/daemon/info.go:82-126`). |
| `<AGH_HOME>/daemon.sock`     | `DaemonSocketName` (line 27)      | UDS socket path; daemon removes a stale socket file at boot (`internal/daemon/orphan.go:87-97`).         |
| `<AGH_HOME>/logs/agh.log`    | `LogFileName` (line 33)           | Structured slog output (file logger; `internal/daemon/boot.go:251-256`).                                  |
| `<AGH_HOME>/logs/network.audit` | `NetworkAuditFileName` (line 35) | Append-only AGH Network audit log.                                                                       |
| `<AGH_HOME>/sessions/`       | `SessionsDirName` (line 19)       | Per-session subdirs containing `events.db` + `meta.json`.                                                |
| `<AGH_HOME>/memory/`         | `MemoryDirName` (line 17)         | Persistent memory files.                                                                                 |
| `<AGH_HOME>/skills/`         | `SkillsDirName` (line 15)         | User-level skills.                                                                                       |
| `<AGH_HOME>/agents/`         | `AgentsDirName` (line 13)         | User-authored AGENT.md/SOUL.md/HEARTBEAT.md files.                                                       |
| `<AGH_HOME>/restarts/`       | `RestartsDirName` (line 21)       | Persisted `RestartOperation` JSON files (`internal/daemon/restart.go:50-63`).                            |

`EnsureHomeLayout` (`internal/config/home.go:117-136`) creates HomeDir/AgentsDir/SkillsDir/MemoryDir/SessionsDir/RestartsDir/LogsDir with `0o755`. Note: the layout is **not** ensured for `LogFile`, `DatabaseFile`, `DaemonSocket`, `DaemonLock`, `DaemonInfo` — those are created as a side effect of opening them.

### 1.6 Exit codes / error sentinels

- Daemon binary returns whatever `cli.ExecuteContext` returns; a Cobra-side error becomes a non-zero exit. The CLI does not assign module-specific numeric exit codes.
- Sentinel errors for the lock/restart subsystems:
  - `daemon.ErrAlreadyRunning` (`internal/daemon/lock.go:17`) — wraps an `errAlreadyRunning{pid}` value-type so callers may use `errors.Is` AND read the prior PID via `errAlreadyRunning.Error()`.
  - `daemon.ErrRestartOperationNotFound` (`internal/daemon/restart.go:32`).
  - `errReplacementDaemonExitedBeforeReady`, `errInvalidRestartTransition`, `errRestartNotRunning` — internal restart transition errors (`internal/daemon/restart.go:33-36`).
  - `errMissingNetworkBindingSurface` (`internal/daemon/daemon.go:47-49`) — boot fails if the session manager doesn't satisfy `networkBindableSessionManager` while network is enabled.
  - `subprocess.ErrNotInitialized` / `ErrShutdownInProgress` / `ErrTransportDisabled` (`internal/subprocess/process.go:33-39`).
  - `store.ErrClosed` / `store.ErrDrainTimeout` (`internal/store/store.go:28-32`).

### 1.7 Signals handled

`Daemon.Run` (`internal/daemon/daemon.go:1075-1111`):

- `syscall.SIGINT`, `syscall.SIGTERM` are notified via `signalSource()` (`internal/daemon/daemon.go:1278-1288`).
- On signal: logs `"daemon: received shutdown signal"` (line 1103), then calls `Shutdown` with a `defaultShutdownTimeout = 10 * time.Second` budget (line 45 + line 1107).
- Tests inject signals via `WithSignalBridge` (`internal/daemon/daemon.go:482-486`).
- No custom `SIGHUP` handling — config reload is HTTP/UDS-driven via the settings service (`internal/daemon/boot.go:1646-1675`).

### 1.8 Build flags

- Standard `go build` produces a single binary; ldflags inject `version.Version`, `version.Commit`, `version.BuildDate` (`internal/version/version.go:8-12`).
- Boundary-check pipeline (`magefile.go` `Boundaries`) runs `internal/daemon.verifyImportBoundaries` (`internal/daemon/boundary.go:60-118`) which forbids any `internal/*` package outside `internal/daemon/...` from importing `internal/daemon`, `internal/api/httpapi`, `internal/api/udsapi`, or `internal/cli`.
- Unix vs Windows: `procutil` and `subprocess` use build-tag splits (`process_group_unix.go` vs `process_group_windows.go`, `procutil_windows.go`, `detached_unix.go` vs `detached_windows.go`, `process_started_at_unix.go` vs `process_started_at_windows.go`, `signals_unix.go` vs `signals_windows.go`). Cross-build with `GOOS=windows GOARCH=amd64 go build ./...` is mandatory (CLAUDE.md, `internal/CLAUDE.md` lines 36-37).

## 2. Existing Test Coverage Map

Every `*_test.go` under the daemon-boot scope, with a one-line summary, the behavior it actually proves, and whether it touches a real subprocess / DB / mocked dep.

### 2.1 `internal/daemon/`

- `agent_probes_test.go` — agent probe target collection (`TestCollectAgentProbeTargetsSkipsUnresolvedProviders`). Pure unit, mocked providers.
- `bridges_test.go` — bridge runtime composition tests; in-memory mocks.
- `boot.go` test sites in `daemon_test.go`:
  - `TestBootWithNetworkDisabledKeepsDaemonOperational` — boots minimal daemon, asserts no network manager. Real `globaldb` open.
  - `TestBootWithRegistryMissingResourceDBLeavesResourceServiceUnavailable` — proves resource service is `nil` when registry has no DB handle.
  - `TestBootRunsResourceReconcileBeforeObserverReconcile` — ordering invariant.
  - `TestShutdownClosesResourceReconcileDriver` — driver `Close(ctx)` must run.
  - `TestBootEnabledNetworkLateBindsSessionCallbacksAndPersistsSafeDiagnostics` — proves network manager is wired AFTER session manager construction; persists redacted diagnostics.
  - `TestBootEnabledNetworkRejectsSessionManagersMissingBindingSurface` — verifies the `errMissingNetworkBindingSurface` path.
  - `TestBootRemovesStaleSocketAndCleansOrphans` — boot path: stale socket + orphan PIDs from previous run get cleaned.
  - `TestCleanupOrphansAllowsGracefulExitBeforeSIGKILL` — `orphanCleanupGraceWait` honored before SIGKILL.
  - `TestBootRejectsConcurrentCallWhileFirstBootIsInProgress` — `beginBoot` rejects re-entry (`daemon.go:206-224`).
  - `TestShutdownTearsDownInRequiredOrder` — proves the `shutdownRuntimeWorkers → shutdownServersAndHooks → shutdownPersistentResources` cascade (`daemon.go:1185-1260`).
  - `TestShutdownDrainsHooksBeforeClosingDatabase` — hooks closed before global DB.
  - `TestBootFailureCleansUpStartedResourcesInReverseOrder` — `bootCleanup.run` LIFO order (`boot.go:127-150`).
  - `TestBootFailureWhenWritingDaemonInfoCleansUpAllServers` — http/UDS server shutdown fires when `WriteInfo` fails.
  - `TestVerifyImportBoundariesReportsViolations` — boundary checker catches forbidden imports.
  - `TestStopSessionsIgnoresNotFoundAndHandlesNilManager` — shutdown path (`daemon.go:1290-1318`).
  - `TestStopSessionsUsesShutdownCauseWhenSupported` — uses `StopWithCause(ctx, id, session.CauseShutdown, "daemon shutdown")`.
  - `TestStopSessionsWaitsForInFlightFinalizations` — proves `WaitForFinalizations` is called.
  - `TestRunShutsDownOnInjectedSignal` — full Run/Shutdown via `WithSignalBridge`.
  - `TestRunShutsDownWhenObserverRetentionStartFails` — error path: retention start failure triggers Shutdown.
  - `TestSignalSourceDefaultsToOSSignalRegistration` — verifies `signal.Notify(SIGINT, SIGTERM)`.
  - `TestLoadConfigFromHomeAppliesOverlayAndNormalizesSocket` / `TestLoadConfigFromHomeValidationError` — config loader contract.
- `restart_test.go` — entire restart-store + relaunch helper coverage. Tests the persisted `RestartOperation` validation (terminal vs in-flight invariants), wait-for-release windows, replacement-launch failure paths, env-var passing.
- `lock.go` test sites in `daemon_test.go`:
  - `TestAcquireLockSucceedsWithoutExistingLock`.
  - `TestAcquireLockFailsWhenAnotherDaemonHoldsTheLock`.
  - `TestAcquireLockReclaimsStalePID` — proves the `stalePID` is set when prior PID is dead.
- `info.go`: `TestInfoWriteReadAndRemoveRoundTrip` — atomic write + sync, validate, remove.
- `spawn_reaper_test.go` — `TestSpawnReaperSweepClassifiesReasonsReleasesLeasesAndStopsChildren` — verifies TTL/parent-stopped/orphaned classification and lease release coupling.
- Integration files (`daemon_integration_test.go`, `daemon_acpmock_faults_integration_test.go`, `daemon_mock_agents_integration_test.go`, `daemon_automation_task_integration_test.go`, `daemon_bridge_extension_integration_test.go`, `daemon_network_collaboration_integration_test.go`, `daemon_sandbox_integration_test.go`, `daemon_memory_e2e_integration_test.go`, `daemon_nightly_combined_integration_test.go`, `restart_integration_test.go`, `coordinator_runtime_integration_test.go`, `harness_context_integration_test.go`, `agent_skill_resources_integration_test.go`, `automation_resources_test.go`, `bridge_extension_e2e_assertions_test.go`, `network_e2e_assertions_test.go`, `automation_task_e2e_assertions_test.go`, `prompt_input_composite_integration_test.go`, `tool_mcp_resources_integration_test.go`, `native_extension_tools_integration_test.go`, `native_automation_tools_integration_test.go`, `hook_binding_resources_integration_test.go`, `sandbox_reconcile_integration_test.go`) — these spin up the daemon harness with the mock ACP agent. **Suspicious**: nearly all of these go through `acpmock` rather than a real Claude Code subprocess. The "integration" tag exercises real DB + real subprocess wiring of the `acpmock` binary (a Go test helper), not a real LLM. They prove the **wiring** but not the **runtime contract** with a real ACP agent. Real-LLM coverage for the daemon path is essentially absent.

### 2.2 `internal/diagnostics/`

- `redact_test.go`:
  - `TestRedactHandlesQuotedJSONSecretsAndBounds` — covers `Bearer …`, quoted JSON `"api_key":"…"`, KV assignment, byte-cap with `RedactAndBound`.
  - `TestRedactHandlesRuntimeRegisteredSecrets` — covers `RegisterDynamicSecret` lifecycle: register → redact → release. Pure unit. **Mocked**: none.

### 2.3 `internal/heartbeat/`

- `heartbeat_test.go` — strict YAML frontmatter parsing, body authority claims rejection, time-window allowlists, digest stability, prompt projection, runtime boundary diagnostics. Pure unit.
- `authoring_status_test.go`, `persistence_test.go`, `wake_test.go` — wake-policy persistence and dispatcher lifecycle. Real SQLite via `globaldb`. These exercise the durable-wake dispatcher but NOT the daemon's boot-time wiring (boot wiring is implicit through `bootRuntime` → `authoredContextRuntimeDeps`).

### 2.4 `internal/store/`

- `migrate_test.go`, `migrate_status_test.go`, and `migrate_streams_test.go` — Goose apply/status semantics, fresh/reopen/ahead/legacy/integrity refusal, gap-free embedded streams, shared global+memory ownership, and migrations-to-declarative-schema equivalence. Real SQLite.
- `store_extra_test.go`:
  - `TestStoreSQLiteRecoveryAndFailures` — the corrupt-DB recovery path (`OpenSQLiteDatabase`, `recoverSQLiteDatabase`, companion `-wal`/`-shm` rename, `ShouldRecoverSQLite` markers). Real SQLite, real corruption injected via byte writes.
- `meta_test.go`, `session_lineage_test.go`, `session_liveness_test.go`, `stop_reason_test.go`, `store_helpers_test.go` — SQL helpers, session lineage, session-meta atomic write.
- `globaldb/global_db_test.go` and per-table tests — global DB CRUD against real SQLite.
- `sessiondb/session_db_test.go` — per-session events DB with the dedicated writer goroutine.

### 2.5 `internal/version/`

- `version_test.go` — `Current()` returns dev defaults; `OverrideVersionForTesting` swaps + restore; `Info.String` format.
- `version_bench_test.go` — micro-benchmark.

### 2.6 `cmd/agh/`

- `main_test.go` — exercises `main`'s `run` helper through CLI boundary; no real LLM.

### 2.7 `internal/procutil/`

- `procutil_test.go` — `Alive`, `Signal`, `StartedAt` with current process. Real syscalls but not a child process.
- `process_group_unix_test.go` — `joinProcessGroupKillResult` semantics for the EPERM-after-exit race.
- `process_started_at_unix_test.go` — `StartedAt` env round-trip.
- **Coverage gap noted**: no test exercises `KillCommandProcessGroupAndWait` against a real child group on macOS; the Linux `/proc/<pid>/stat` path is not directly tested (it depends on Linux runner + skip on macOS).

### 2.8 `internal/subprocess/`

- `process_test.go` — extensive: launch, JSON-RPC framing, cooperative shutdown, kill-after-timeout, health-monitor failure thresholds, message size limits, RPC error formatting, race-free `stopHealthMonitor`. Uses `TestSubprocessHelperProcess` as a self-spawned child (a real subprocess of the test binary).
- `process_integration_test.go` — `TestProcessIntegrationLifecycle`, `…CrashRecovery`, `…ConcurrentRequests`. Real subprocess of the test binary.
- `handshake_test.go` — initialize/handshake validation; pure unit.

### 2.9 Suspicious "integration test in name only"

- `internal/daemon/daemon_*_integration_test.go` — all of these actually integrate AGAINST `acpmock`, not against a real ACP-speaking agent. They are valuable wiring tests but never prove the AGH↔Claude Code wire contract. Final QA must add at least one real-Claude-Code path per critical scenario below.
- `internal/daemon/coordinator_runtime_integration_test.go` — same caveat.
- `internal/store/sessiondb/session_db_integration_test.go` — actually does drive real sqlite + real writer goroutine; no caveat.

## 3. Coverage Gaps

For each gap, the claim AGH makes (with citation) and the missing test that would make the claim load-bearing.

| Gap                                                                                         | Claim AGH makes                                                                                                                                              | Test that would make the claim load-bearing                                                                                                                                                                                                                                                                                |
| ------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Real ACP subprocess shutdown via process group**                                          | "Process-group supervision parity. Unix uses process groups; Windows uses forced-exit fallback." `internal/CLAUDE.md:36-37`; `procutil/process_group_unix.go:135-218` enumerates `/proc/<pid>/stat` on Linux. | Spawn `agh sessions start --agent claude-code`, give it a long-running tool that itself spawns a grandchild, then `agh daemon stop`. Assert the entire descendant tree (PID, PGID, grandchild) is gone within `defaultShutdownTimeout (10s)` (`daemon.go:45`).                                                            |
| **Cross-build for Windows is mandatory**                                                    | "Always cross-build with `GOOS=windows GOARCH=amd64 go build` before claiming subprocess work complete." `internal/CLAUDE.md:37`                              | CI gate that runs `GOOS=windows GOARCH=amd64 go build ./...` against `internal/daemon/...`, `internal/procutil/...`, `internal/subprocess/...`. No such gate exists in current CI.                                                                                                                                            |
| **`agh.db` never appears 0-byte after a crash**                                             | `OpenSQLiteDatabase` re-opens with `-wal`/`-shm` rename on corruption (`store/sqlite.go:165-186`).                                                            | Crash-injection test: kill -9 the daemon mid-write, restart, assert no 0-byte `agh.db`, no orphan `-wal` longer than `-shm` checkpoint allows.                                                                                                                                                                              |
| **Lock file is removed only on clean exit**                                                 | `Lock.Release()` writes PID=0 then `Unlock`+`Close` (`lock.go:124-149`). On crash the file remains with stale PID; next boot reclaims via `lock.StalePID()`. | Test: crash daemon, restart, assert `lock.StalePID() > 0` and `daemon.json` is treated as stale and replaced.                                                                                                                                                                                                              |
| **Migration stream rejects checksum drift**                                                 | `store.Apply` validates `atlas.sum` before Goose runs; `TestApplyRejectsAtlasSumDrift` covers edited/added/removed SQL.                                      | Preserve the canonical real-DB suite and verify daemon boot surfaces the same deterministic refusal without mutating the database.                                                                                                                                                                                           |
| **Schema does not silently downgrade**                                                      | `store.Apply` probes the recorded Goose version before `Up` and returns `ErrSchemaAhead`.                                                                    | Boot an older post-Goose binary against a database recorded above its embedded head; assert refusal and byte identity, then prove a newer compatible binary is the state-preserving recovery.                                                                                                                                |
| **`PRAGMA journal_mode = WAL` is non-negotiable**                                           | `configureSQLite` returns an error if mode is not WAL (`store/sqlite.go:106-117`).                                                                            | Test exists in `store_extra_test.go` for happy path; missing: simulate a tampered DB where journal_mode resists WAL setup, assert open fails with the explicit `"journal_mode = …, want wal"` error.                                                                                                                          |
| **Detached lifetime: `prompt/cancel` only cancels via explicit endpoint**                   | "Detached execution lifetime. Any work that outlives an HTTP/UDS request — prompts, network channel sends, automation jobs — MUST detach via `context.WithoutCancel(ctx)`." `internal/CLAUDE.md:33-35`. | E2E: start a real prompt, drop the HTTP request mid-stream, assert the prompt continues to completion. Currently the daemon harness never validates this against a real LLM.                                                                                                                                              |
| **Raw `claim_token` does not appear in any output**                                         | `internal/CLAUDE.md:55`; redaction enforced via `internal/diagnostics/redact.go:11-14` patterns; rejection in API (`internal/api/contract/agents.go:479-498`, `internal/api/core/network.go:283-294`). | E2E: trigger a real claim flow against a real agent, capture **all** of: `agh.log`, SSE stream output, `agh status -o json`, `agh sessions logs <id>`, plus the network audit log. Greedy-grep for the literal regex `\bagh_claim_[A-Za-z0-9]+\b`. Zero matches required. No such full-output sweep exists today. |
| **Subprocess `goleak` clean shutdown**                                                      | `internal/CLAUDE.md:32-34` — Manager-WaitGroup discipline. `subprocess.Process.Wait` and `Shutdown` exist (`subprocess/process.go:320-449`).                  | Runtime-wide `goleak.VerifyNone(t, ignoreOptions...)` after a full `Daemon.Shutdown` cycle including ACP subprocess cleanup. No top-level test today exercises a full Daemon Run + Shutdown cycle under goleak.                                                                                                              |
| **HEARTBEAT.md absence does not break boot**                                                | `heartbeat.Resolve` returns an empty `ResolvedPolicy` on `os.ErrNotExist` (`heartbeat.go:237-245`).                                                          | Boot test exists indirectly through resource projector; missing: assert `agh agent context <agent-id>` returns `present:false` on a fresh AGH_HOME, no diagnostic emitted.                                                                                                                                                  |
| **Boundary check passes against a freshly booted runtime**                                  | `daemon.Boundaries(ctx)` (`internal/daemon/boundary.go:19-45`) is opt-in via `AGH_DEV_VERIFY_BOUNDARIES`.                                                     | A QA gate that exports `AGH_DEV_VERIFY_BOUNDARIES=1` and verifies no warning is logged on boot. Currently a unit test (`TestBoundariesUsesConfiguredRoot` etc.) covers the helper, but no end-to-end gate hooks it into the binary.                                                                                          |
| **Daemon refuses to start while another holds the lock — error message names the prior PID** | `errAlreadyRunning.Error()` (`lock.go:24-29`) prints `daemon: already running with pid <N>`.                                                                  | CLI-level test that asserts the **exact** error string format (operators rely on parsing this).                                                                                                                                                                                                                              |
| **Concurrent `agh daemon start` exits without race**                                        | `runDaemonForeground` (`cli/daemon.go:150-170`) re-checks `daemonInfo` after `ensureHome`. Two daemons would both try `acquireLock`; second fails.            | Race test: launch `agh daemon start --foreground` twice in parallel, assert exactly one survives and the loser exits with the `errAlreadyRunning` message AND non-zero exit.                                                                                                                                                  |
| **`daemon.json` is atomically replaced**                                                    | `WriteInfo` uses `os.CreateTemp` + `Rename` + `syncDir` (`info.go:82-126`).                                                                                   | Crash a writer mid-write, assert that either old or new payload exists, never half-written.                                                                                                                                                                                                                                  |
| **Composition root never imports cli/api/daemon out-of-bounds**                             | `boundary.go:60-118` actively rejects this.                                                                                                                  | Already covered by unit. Missing: a single CI gate that runs `mage Boundaries` AND `daemon.Boundaries(ctx)` against the live tree to catch tooling drift.                                                                                                                                                                    |
| **Restart preserves session ledger and active leases**                                      | `RestartOperation.ActiveSessionCount` is recorded (`restart.go:55-86`); the new daemon reconciles via `bootSessionRepair` (`boot.go:545-617`).                | E2E: start a real ACP session under the OLD binary, trigger restart via the relaunch helper, assert the SAME `session_id` is recoverable from the NEW binary's `agh sessions list -o json`.                                                                                                                                  |
| **`stopReason` after crash is reclaimable**                                                 | `bootShouldRepairSession` repairs sessions whose `StopReason` is `StopAgentCrashed` or `StopError` (`boot.go:623-635`).                                       | E2E: kill -9 the daemon mid-prompt, restart, assert `agh sessions list -o json` shows the prior session as `state:stopped, stop_reason: agent_crashed` and `agh sessions resume <id>` succeeds.                                                                                                                              |
| **Process registry boot reconciliation runs once and reports counters**                     | `bootProcessRegistry` (`boot.go:737-764`) calls `state.processRegistry.ReconcileBoot(ctx)`.                                                                  | E2E: leave 3 stale tool-process records in the registry, restart, assert log line `"daemon: reconciled tool process registry"` with `recovered`/`stale` counts, and assert no orphan tool subprocesses remain.                                                                                                                |

## 4. Real-LLM / Real-Agent Scenarios

Each scenario is one fenced markdown block keyed by the `qa-scenario`/`qa-flow` shape adapted from openclaw (see `_references/openclaw-qa-patterns.md` §2). Numbered DB-01..DB-15. Live runs use `agh sessions start --agent claude-code --workspace ./fixtures/<theme>` against a real Claude Code subagent unless explicitly tagged `provider: none`.

```markdown
### DB-01 — Cold start with empty AGH_HOME

```yaml qa-scenario
id: db-cold-start-empty-home
title: Cold start creates lock, opens DBs, applies migrations, registers UDS, opens HTTP
theme: daemon-boot
coverage:
  primary:
    - daemon.boot.cold-start
  secondary:
    - daemon.lock.acquire
    - daemon.info.write
    - store.migrations.fresh-apply
live: false
provider: none
```

```yaml qa-flow
preconditions:
  - AGH_HOME=$LAB_HOME exported (`agh-qa-bootstrap` fresh manifest, no provider).
  - `$LAB_HOME` directory does not exist.
  - Daemon ports reserved by manifest (no overlap with any other lab).
steps:
  - run: rm -rf "$LAB_HOME" && mkdir -p "$LAB_HOME"
  - run: ls -la "$LAB_HOME"   # MUST be empty
  - run: agh daemon start
  - wait: 5s for daemon ready
  - run: agh status -o json | tee status.json
  - run: jq -r '.pid' status.json
  - run: ls -la "$LAB_HOME"
  - run: jq '.daemon.schema_streams' status.json | tee schema-streams.json
  - run: curl --unix-socket "$LAB_HOME/daemon.sock" http://./daemon/status | jq .
  - run: curl -s "http://127.0.0.1:$AGH_HTTP_PORT/api/observe/health" | jq .
expected_behavior:
  - `daemon.lock` exists with the daemon PID written as ASCII (single integer + \n) per `lock.go:175-193`.
  - `daemon.json` exists with `pid > 0`, `port > 0`, `started_at` ISO-8601 UTC, optional `network` block — validated by `info.Validate` (`info.go:31-46`).
  - `daemon.sock` exists and is a UDS socket.
  - `agh.db` exists; `schema_streams` contains distinct `global` and `memory` rows with version `1`, applied count `1`, and non-empty digests.
  - `logs/agh.log` exists and contains `"daemon: boot reconciliation complete"` (`boot.go:1721-1725`).
  - HTTP `/api/observe/health` returns 200 with `{health, memory, automation}` JSON shape (`handlers.go:868-893`).
  - `agh status -o json` returns `status:"running"` with the UDS-side payload preferred over `daemon.json` (`cli/daemon.go:283-300`).
evidence_to_capture:
  - `$LAB_HOME/logs/agh.log` (full).
  - `$LAB_HOME/daemon.json` (post-ready snapshot).
  - `schema-streams.json`.
  - `status.json`.
  - HTTP/UDS curl outputs.
failure_signatures:
  - `daemon.json` missing or `port == 0` after status returns running.
  - Either `global` or `memory` is absent, reports version/applied count `0`, or has an empty digest.
  - `agh status` reports `starting` indefinitely (poll loop in `cli/daemon.go:202-242`).
  - Raw `agh_claim_*` token visible anywhere in `agh.log`.
cleanup:
  - agh daemon stop && rm -rf "$LAB_HOME"
```
```

```markdown
### DB-02 — Warm start reconciles state without double migration

```yaml qa-scenario
id: db-warm-start-idempotent-migrations
title: Warm restart reuses existing DB, never re-applies a recorded migration
theme: daemon-boot
coverage:
  primary:
    - store.migrations.idempotent
  secondary:
    - daemon.boot.warm-start
    - daemon.session.repair-on-restart
live: false
provider: none
```

```yaml qa-flow
preconditions:
  - DB-01 has just completed (lab home populated, daemon stopped cleanly).
  - At least 1 prior session row exists in `sessions` table.
steps:
  - run: cp "$LAB_HOME/agh.db" "$LAB_HOME/agh.db.before"
  - run: agh status -o json | jq -S '.daemon.schema_streams' > before.txt
  - run: agh daemon start
  - wait: 5s
  - run: agh status -o json
  - run: agh status -o json | jq -S '.daemon.schema_streams' > after.txt
  - run: diff before.txt after.txt   # MUST be empty
  - run: grep "boot session repair" "$LAB_HOME/logs/agh.log"
expected_behavior:
  - `before.txt == after.txt`: versions, applied counts, and digests are stable across reopen.
  - `boot session repair` log line appears at most once per repaired session (`boot.go:589-596`).
  - No duplicate session_id rows; lineage backfill runs only once per session (`globaldb.go:711-716`).
evidence_to_capture:
  - `before.txt`, `after.txt`, the diff output.
  - `agh.log` excerpt around boot.
failure_signatures:
  - `applied_at` timestamp on any migration changes between runs.
  - `"daemon: boot session repair complete"` repeats for the SAME session_id more than once.
cleanup:
  - agh daemon stop
```
```

```markdown
### DB-03 — Lock collision: second `agh daemon start` exits with stable code + clear error

```yaml qa-scenario
id: db-lock-collision-second-start
title: Two daemon starts against same AGH_HOME — second exits with errAlreadyRunning
theme: daemon-boot
coverage:
  primary:
    - daemon.lock.collision
  secondary:
    - daemon.cli.start.detached
    - daemon.error-message.named-pid
live: false
provider: none
```

```yaml qa-flow
preconditions:
  - DB-01 daemon is up and ready.
steps:
  - run: agh status -o json   # confirm running with PID=$P1
  - run: agh daemon start 2>&1 | tee second.log; echo "exit=$?" >> second.log
  - run: agh status -o json   # original still alive
  - run: cat "$LAB_HOME/daemon.lock"   # must still contain $P1
expected_behavior:
  - Second start fails. Error text matches `daemon: already running (pid=<P1>)` from `runDaemonDetached` (`cli/daemon.go:184`).
  - Second process exits non-zero (Cobra-driven RunE error).
  - `daemon.lock` content is unchanged; first daemon still owns the advisory `flock` (`lock.go:80-106`).
evidence_to_capture:
  - `second.log` (full output + exit code).
  - `daemon.lock` byte-for-byte before/after.
failure_signatures:
  - Either daemon stops accepting requests on UDS.
  - `daemon.lock` rewritten with the second-process PID.
  - Error text omits the prior PID (operator UX bug).
cleanup:
  - agh daemon stop
```
```

```markdown
### DB-04 — Crash recovery: kill -9 mid-prompt, restart, reclaim run

```yaml qa-scenario
id: db-crash-recovery-task-run
title: kill -9 mid-prompt; restart proves task_run is reclaimable, claim_token raw never logged, transcript replay works
theme: daemon-boot
coverage:
  primary:
    - daemon.boot.crash-recovery
    - task.lease.recovery
  secondary:
    - security.claim-token.redaction
    - session.transcript.replay
live: true
provider: claude-code
```

```yaml qa-flow
preconditions:
  - Lab home populated, real Claude Code agent reachable, `PROVIDER_HOME=$LAB_HOME/.provider`.
  - Workspace fixture: `./fixtures/db04-crash-recovery/` with a small AGENT.md.
steps:
  - run: agh daemon start
  - wait: 5s
  - run: agh sessions start --agent claude-code --workspace ./fixtures/db04-crash-recovery -o json | tee sess.json
  - set: SID=$(jq -r '.id' sess.json)
  - run: agh sessions prompt $SID --message "List the files in the workspace and write the listing to NOTES.md" --detach -o json
  - wait: 1s   # let the prompt actually start (assert `agh sessions list -o json | jq '.[].state'` shows running)
  - run: PID=$(jq -r '.pid' "$LAB_HOME/daemon.json")
  - run: kill -9 $PID
  - wait: 2s
  - run: ! agh status -o json   # expect failure (running:false or socket gone)
  - run: agh daemon start
  - wait: 5s
  - run: agh sessions list -o json | jq ".[] | select(.id==\"$SID\")" | tee post.json
  - run: agh sessions resume $SID -o json | tee resume.json
  - run: agh sessions transcript $SID -o json | jq 'length'
  - run: grep -E '\bagh_claim_[A-Za-z0-9]+\b' "$LAB_HOME/logs/agh.log"; echo $?
  - run: agh task list --session-id $SID -o json | tee tasks.json
  - run: jq '.[] | {id, state, claim_token_hash, claim_token}' tasks.json
expected_behavior:
  - After kill -9, `daemon.lock` contains stale PID; new daemon detects it via `lock.StalePID()` (`lock.go:88-92`).
  - Boot session repair runs and logs `"daemon: boot session repair complete"` for $SID with `state:stopped, stop_reason:agent_crashed` (`boot.go:566-596`).
  - `agh sessions resume $SID` re-establishes the session.
  - `transcript $SID` returns the pre-crash messages plus the post-resume continuation; replay assembled by `internal/transcript/...`.
  - `tasks.json`: every row exposes `claim_token_hash` (sha256 hex) and NEVER `claim_token` (per `task/types.go:276,380`, `lease.go:546-548`, `core/tasks.go:1741-1745`).
  - `grep -E '\bagh_claim_[A-Za-z0-9]+\b' agh.log` returns exit code 1 (no match).
evidence_to_capture:
  - sess.json, post.json, resume.json, tasks.json.
  - Full `$LAB_HOME/logs/agh.log` between the two daemon runs.
  - Pre-/post-crash `daemon.json`.
  - SSE stream snapshot from `/api/observe/events?after_seq=<crash-marker>`.
failure_signatures:
  - `agh_claim_…` literal in log.
  - tasks.json contains a `claim_token` field (raw token).
  - Resume returns `ErrSessionNotFound`.
  - Transcript missing the pre-crash prompt, OR replay produces duplicate user messages.
cleanup:
  - agh sessions stop $SID && agh daemon stop
```
```

```markdown
### DB-05 — Graceful shutdown via SIGINT/SIGTERM stops all ACP subprocesses

```yaml qa-scenario
id: db-graceful-shutdown-process-group
title: SIGTERM to daemon stops every ACP subprocess via process group; cross-build for windows must succeed
theme: daemon-boot
coverage:
  primary:
    - daemon.shutdown.graceful
    - subprocess.process-group.signal
  secondary:
    - daemon.cli.stop
live: true
provider: claude-code
```

```yaml qa-flow
preconditions:
  - Daemon up; one real Claude Code session active.
steps:
  - run: agh sessions start --agent claude-code --workspace ./fixtures/db05 -o json | tee sess.json
  - set: SID=$(jq -r '.id' sess.json)
  - run: PARENT_PID=$(jq -r '.pid' "$LAB_HOME/daemon.json")
  - run: pgrep -P $PARENT_PID | tee child_pids.txt   # ACP subprocess pid(s)
  - run: ps -o pid,ppid,pgid,command -ax | grep -E "(claude|agh)" | tee ps.before
  - run: agh daemon stop -o json | tee stop.json
  - wait: 12s   # allow defaultShutdownTimeout=10s + slack
  - run: ps -o pid,ppid,pgid,command -ax | grep -E "(claude|agh)" || true | tee ps.after
  - run: kill -0 $PARENT_PID 2>&1 || echo "parent gone"
  - run: for p in $(cat child_pids.txt); do kill -0 $p 2>&1 || echo "child $p gone"; done
  - run: GOOS=windows GOARCH=amd64 go build ./... | tee winbuild.log
expected_behavior:
  - Every PID in `child_pids.txt` is gone (`kill -0` reports ESRCH).
  - `agh daemon stop -o json` reports `status:"stopped"` (`cli/daemon.go:244-281`).
  - `daemon.lock` exists with `0\n` (cleared by `lock.Release()` writing PID=0; `lock.go:124-149`).
  - `daemon.json` is gone (removed in `shutdownPersistentResources`; `daemon.go:1245-1260`, `RemoveInfo`).
  - `daemon.sock` is gone.
  - Windows cross-build succeeds (winbuild.log empty / exit 0).
evidence_to_capture:
  - ps.before, ps.after, child_pids.txt.
  - stop.json, daemon.json final state.
  - winbuild.log.
failure_signatures:
  - Any PID in child_pids.txt still alive after 12s (process-group signal regressed).
  - `daemon.lock` contents != `0\n` after clean stop.
  - `daemon.sock` left behind (next start would fail until `removeStaleSocket` runs; `orphan.go:87-97`).
  - Windows build error involving `procutil` or `subprocess`.
cleanup:
  - rm -f winbuild.log; clean lab home
```
```

```markdown
### DB-06 — Signal during boot aborts cleanly without orphaning resources

```yaml qa-scenario
id: db-boot-signal-abort
title: SIGTERM arriving before composition root finishes — daemon aborts, releases lock, deletes daemon.json
theme: daemon-boot
coverage:
  primary:
    - daemon.boot.signal-during-boot
  secondary:
    - daemon.lock.release-on-failure
    - daemon.info.cleanup-on-failure
live: false
provider: none
```

```yaml qa-flow
preconditions:
  - Test harness exposes `--inject-boot-delay=<duration>` (a debug-only flag; if unavailable, simulate by injecting a slow `WithRegistryOpener` via composition-root tests in `daemon_test.go`).
  - Use the in-process daemon test harness with `WithSignalBridge` to fire SIGTERM during `bootRuntime`.
steps:
  - run: go test -run TestDaemon_SignalDuringBoot_AbortsCleanly -tags daemon_signal_qa ./internal/daemon/...
  - assert: lock file does not exist or contains `0\n`.
  - assert: daemon.json does not exist.
  - assert: bootCleanup ran in reverse order (verifiable via test recorder hook in `bootCleanup.run`, `boot.go:138-150`).
  - assert: no goroutine leak (goleak.VerifyNone with documented ignores).
expected_behavior:
  - `Daemon.Run` returns the boot error wrapped with the cleanup join (`boot.go:138-150`).
  - `bootRuntime → bootLockAndSocket` already added the lock to cleanup (`boot.go:388-409`); cleanup releases it.
  - `bootRuntime → bootRegistryState` registry close runs.
  - `bootServers` server cleanups did NOT register because boot failed before that step.
evidence_to_capture:
  - test stdout, lock file content snapshot pre/post run.
  - goleak report.
failure_signatures:
  - lock left with non-zero PID despite no live daemon.
  - daemon.json present.
  - any goroutine leak (most likely the skills watcher or the spawn reaper if started before failure point).
cleanup:
  - rm -rf $LAB_HOME (test cleans on exit)
```
```

```markdown
### DB-07 — Migration rollback safety: bad migration aborts boot, no partial state

```yaml qa-scenario
id: db-migration-rollback-bad
title: Bad numbered migration rolls back transaction, does NOT record applied row, daemon refuses to boot
theme: daemon-boot
coverage:
  primary:
    - store.migrations.rollback
  secondary:
    - store.migrations.checksum
live: false
provider: none
```

```yaml qa-flow
preconditions:
  - Build a test-only binary with an embedded, gap-free Goose N+1 SQL migration whose Up statement fails after an earlier DDL statement; refresh its test-only `atlas.sum` so the failure reaches Goose rather than checksum validation.
steps:
  - run: agh-qa-bad-migration daemon start --foreground 2>&1 | tee out.log; echo $? >> out.log
  - run: sqlite3 "$LAB_HOME/agh.db" 'SELECT version_id, is_applied FROM goose_db_version_global ORDER BY id;' | tee versions.txt
  - assert: out.log contains "store: apply migration stream \"global\"" and the underlying SQL error.
  - assert: versions.txt does NOT contain applied version N+1.
  - assert: `agh.db.<timestamp>.corrupt` files do NOT appear (this is a runtime error, not corruption).
  - run: agh daemon start   # original binary now boots normally
  - wait: 5s
  - run: agh status -o json | jq -r '.status'
expected_behavior:
  - Goose rolls back the transactional Up migration; no partial schema object remains.
  - No partial schema state (the failing migration's CREATE statements get rolled back).
  - Original binary recovers cleanly because nothing was recorded.
evidence_to_capture:
  - out.log; versions.txt; full `agh.db` schema dump pre and post.
failure_signatures:
  - An applied N+1 row appears in `goose_db_version_global`.
  - Daemon boots successfully against bad migration (rollback skipped).
cleanup:
  - rm $LAB_HOME/agh.db && agh daemon stop
```
```

```markdown
### DB-08 — `-wal` / `-shm` companion recovery on corruption

```yaml qa-scenario
id: db-sqlite-corruption-recovery
title: Corrupt agh.db is moved to `.corrupt.<ts>` along with -wal/-shm; daemon reopens cleanly
theme: daemon-boot
coverage:
  primary:
    - store.sqlite.recovery
  secondary:
    - daemon.boot.degraded
live: false
provider: none
```

```yaml qa-flow
preconditions:
  - Daemon stopped; `$LAB_HOME/agh.db` exists.
steps:
  - run: printf 'NOT A VALID SQLITE' > "$LAB_HOME/agh.db"   # corrupt
  - run: dd if=/dev/zero of="$LAB_HOME/agh.db-wal" bs=1k count=8 2>/dev/null   # create matching wal
  - run: dd if=/dev/zero of="$LAB_HOME/agh.db-shm" bs=1k count=8 2>/dev/null
  - run: agh daemon start 2>&1 | tee boot.log
  - wait: 5s
  - run: ls -la "$LAB_HOME"/agh.db* | tee files.txt
  - run: agh status -o json | jq '.daemon.schema_streams' | tee schema-streams.json
expected_behavior:
  - `OpenSQLiteDatabase` first attempt fails with one of the markers in `ShouldRecoverSQLite` (`sqlite.go:188-208`).
  - `recoverSQLiteDatabase` (`sqlite.go:165-186`) renames `agh.db` → `agh.db.corrupt.<RFC3339nano>`, plus `-wal` / `-shm`.
  - Reopen succeeds; the new fresh DB reports global and memory Goose streams at their embedded heads.
  - boot.log records "store: recover sqlite database" only if the second open also fails (otherwise quiet recovery).
evidence_to_capture:
  - files.txt (proves three corrupt-marked files exist).
  - boot.log.
  - `schema-streams.json` (both daemon-global streams have versions, applied counts, and digests).
failure_signatures:
  - boot.log contains a fatal "open sqlite database" error and daemon never starts.
  - Only `agh.db` gets renamed; `-wal` and `-shm` are left next to a fresh DB.
cleanup:
  - rm -f $LAB_HOME/agh.db.corrupt.* && agh daemon stop
```
```

```markdown
### DB-09 — Schema reopen after restart: identical schema after clean restart

```yaml qa-scenario
id: db-schema-reopen-identical
title: Schema is byte-for-byte identical after clean stop+start
theme: daemon-boot
coverage:
  primary:
    - store.schema.reopen-stable
live: false
provider: none
```

```yaml qa-flow
steps:
  - run: agh daemon start && sleep 3
  - run: sqlite3 "$LAB_HOME/agh.db" '.schema' | sha256sum > schema.before
  - run: agh daemon stop && sleep 2
  - run: agh daemon start && sleep 3
  - run: sqlite3 "$LAB_HOME/agh.db" '.schema' | sha256sum > schema.after
  - assert: cmp schema.before schema.after
  - run: agh status -o json | jq -S '.daemon.schema_streams' > migrations.before2
  - run: agh daemon stop && agh daemon start && sleep 3
  - run: agh status -o json | jq -S '.daemon.schema_streams' > migrations.after2
  - assert: diff migrations.before2 migrations.after2
expected_behavior:
  - schema sha matches before/after.
  - stream versions, applied counts, and digests match.
evidence_to_capture:
  - schema.before, schema.after, sha values, diff outputs.
failure_signatures:
  - Schema differs because code bypassed the declarative Goose stream.
  - A stream's applied count or digest changes without an appended migration.
cleanup:
  - agh daemon stop
```
```

```markdown
### DB-10 — Multi-binary upgrade: old stops, new boots against same DB

```yaml qa-scenario
id: db-binary-upgrade
title: Upgrade path — older binary stops, newer binary boots against same DB without data loss
theme: daemon-boot
coverage:
  primary:
    - daemon.upgrade
    - version.compatibility
live: true
provider: claude-code
```

```yaml qa-flow
preconditions:
  - Two binaries: `$AGH_OLD` (previous build) and `$AGH_NEW` (current build).
  - Lab home contains an active session and at least one task_run row.
steps:
  - run: $AGH_OLD daemon start && sleep 3
  - run: $AGH_OLD sessions start --agent claude-code --workspace ./fixtures/db10 -o json | tee sess.json
  - set: SID=$(jq -r '.id' sess.json)
  - run: $AGH_OLD sessions prompt $SID --message "ack and stop" -o json
  - run: $AGH_OLD status -o json | jq -r '.version' | tee old_version.txt
  - run: $AGH_OLD daemon stop && sleep 2
  - run: $AGH_NEW daemon start && sleep 5
  - run: $AGH_NEW status -o json | jq -r '.version' | tee new_version.txt
  - run: $AGH_NEW sessions list -o json | jq ".[] | select(.id==\"$SID\")" | tee post_upgrade.json
  - run: $AGH_NEW status -o json | jq '.daemon.schema_streams' | tee migrations.txt
  - run: $AGH_NEW sessions resume $SID -o json
  - run: $AGH_NEW sessions transcript $SID -o json | jq 'length'
expected_behavior:
  - Status `version` field reflects each binary's `version.Current().Version` (`cli/daemon.go:333`).
  - All migrations from old binary remain; any new migrations apply once.
  - $SID resumable; transcript contains pre-upgrade events.
evidence_to_capture:
  - old_version.txt, new_version.txt, post_upgrade.json, migrations.txt.
failure_signatures:
  - $SID missing from $AGH_NEW session list.
  - migration row gets re-applied (different `applied_at`).
cleanup:
  - $AGH_NEW sessions stop $SID && $AGH_NEW daemon stop
```
```

```markdown
### DB-11 — Heartbeat / liveness probe under load and during stuck subprocess

```yaml qa-scenario
id: db-heartbeat-under-load
title: /api/observe/health stays responsive under prompt load and reports degraded state when an ACP subprocess hangs
theme: daemon-boot
coverage:
  primary:
    - daemon.diagnostics.health-under-load
  secondary:
    - subprocess.health-monitor
live: true
provider: claude-code
```

```yaml qa-flow
steps:
  - run: agh daemon start && sleep 3
  - run: agh sessions start --agent claude-code --workspace ./fixtures/db11 -o json | tee sess.json
  - set: SID=$(jq -r '.id' sess.json)
  - run: for i in {1..5}; do agh sessions prompt $SID --message "Compute $i+$i" --detach -o json &; done; wait
  - run: while true; do curl -s http://127.0.0.1:$AGH_HTTP_PORT/api/observe/health | jq -r '.health.status, .memory.status, .automation.status'; sleep 1; done | head -20 | tee health.txt
  - run: # induce stall: send SIGSTOP to the ACP subprocess pid
  - run: ACP_PID=$(jq -r '.subprocess_pid' < <(agh sessions get $SID -o json)); kill -STOP $ACP_PID
  - wait: 30s
  - run: curl -s http://127.0.0.1:$AGH_HTTP_PORT/api/observe/health | jq . | tee stalled-health.json
  - run: agh sessions get $SID -o json | jq '.stall_state, .stall_reason' | tee stall.txt
  - run: kill -CONT $ACP_PID
expected_behavior:
  - Health endpoint returns 200 within 1s during all probes.
  - Subprocess `HealthState.Healthy` flips to false after `health_check` consecutive failures cross threshold (`subprocess/health.go:149-161`).
  - `agh sessions get` reports stall_state and stall_reason populated.
  - No raw `claim_token` in any output.
evidence_to_capture:
  - health.txt, stalled-health.json, stall.txt.
failure_signatures:
  - Health endpoint hangs (>5s) even once.
  - Stalled subprocess never marked unhealthy.
cleanup:
  - agh sessions stop $SID && agh daemon stop
```
```

```markdown
### DB-12 — Diagnostics probe in normal / unhealthy / degraded states

```yaml qa-scenario
id: db-diagnostics-states
title: agh status -o json + GET /api/diagnostics/* expose accurate state across normal, degraded, and unhealthy
theme: daemon-boot
coverage:
  primary:
    - daemon.diagnostics.state-accuracy
  secondary:
    - daemon.cli.status
live: false
provider: none
```

```yaml qa-flow
steps:
  - run: agh status -o json | jq . | tee normal.json
  - assert: normal.json.status == "running"
  - run: # Force memory directory unwritable to provoke degradation
  - run: chmod 000 "$LAB_HOME/memory" && curl -s http://127.0.0.1:$AGH_HTTP_PORT/api/memory/health | jq . | tee degraded-memory.json
  - run: chmod 755 "$LAB_HOME/memory"
  - run: # provoke automation degradation by editing a webhook to use a missing secret
  - run: curl -s http://127.0.0.1:$AGH_HTTP_PORT/api/observe/health | jq . | tee composite-health.json
  - run: agh daemon stop && sleep 2
  - run: agh status -o json | jq . | tee stopped.json
  - assert: stopped.json.status == "stopped"
  - assert: stopped.json.network == null   # `daemonStatusWithState` zeros network when stopped (cli/daemon.go:318-336)
expected_behavior:
  - normal.json shows `status:running` with `pid > 0`.
  - degraded-memory.json contract response indicates the memory subsystem failure cleanly (no panic, no claim_token leak).
  - stopped.json was synthesized from `daemon.json` (or absence) without contacting the daemon.
evidence_to_capture:
  - normal.json, degraded-memory.json, composite-health.json, stopped.json.
failure_signatures:
  - daemon status hangs after stop.
  - composite-health.json contains a 5xx error JSON instead of structured `health/memory/automation` payload.
cleanup:
  - chmod 755 "$LAB_HOME/memory" && agh daemon start
```
```

```markdown
### DB-13 — Subprocess managed-stop discipline: no orphan goroutines under goleak

```yaml qa-scenario
id: db-goleak-clean-shutdown
title: Daemon Run + Shutdown leaks zero goroutines (goleak.VerifyNone)
theme: daemon-boot
coverage:
  primary:
    - daemon.shutdown.goroutine-discipline
  secondary:
    - subprocess.health-monitor.stop
    - skills.watcher.stop
live: false
provider: none
```

```yaml qa-flow
preconditions:
  - In-process Go test under `internal/daemon/` with `goleak`.
steps:
  - run: go test -run TestDaemon_Goleak_FullCycle -race -count=1 ./internal/daemon/... | tee goleak.log
expected_behavior:
  - Test:
    1. `daemon.New(WithSignalBridge(ch))` boot.
    2. Start a real `subprocess.Process` against `TestSubprocessHelperProcess`.
    3. Call `Daemon.Shutdown(ctx)`.
    4. `defer goleak.VerifyNone(t, goleak.IgnoreCurrent(), goleak.IgnoreTopFunction("modernc.org/sqlite/lib._cgo_…"))`.
  - Manager-WaitGroup join completes (`internal/CLAUDE.md:32-34`).
  - `stopHealthMonitor` is race-free (already covered by `TestStopHealthMonitorIsRaceFree`, `subprocess/process_test.go`).
  - Skills watcher cancel + done channel drains (`boot.go:1135-1153`, `boot.go:1197-1198`).
evidence_to_capture:
  - goleak.log on failure (full goroutine dump).
failure_signatures:
  - Any goroutine leak whose top frame is in `internal/skills`, `internal/scheduler`, `internal/automation`, `internal/network`, `internal/subprocess`, or `internal/observe`.
cleanup:
  - none (in-process)
```
```

```markdown
### DB-14 — Detached lifetime: HTTP request cancel does NOT cancel a real prompt

```yaml qa-scenario
id: db-detached-prompt-lifetime
title: Closing the HTTP/UDS request mid-prompt does not cancel the prompt; explicit prompt/cancel does
theme: daemon-boot
coverage:
  primary:
    - daemon.detached-lifetime
  secondary:
    - sessions.prompt.cancel-endpoint
live: true
provider: claude-code
```

```yaml qa-flow
steps:
  - run: agh daemon start && sleep 3
  - run: agh sessions start --agent claude-code --workspace ./fixtures/db14 -o json | tee sess.json
  - set: SID=$(jq -r '.id' sess.json)
  - run: # Start prompt over HTTP with --detach=false to get the streaming response, then kill the curl client
  - run: timeout 1 curl -N -X POST -d '{"message":"List files and summarize"}' http://127.0.0.1:$AGH_HTTP_PORT/api/sessions/$SID/prompt 2>/dev/null || true
  - wait: 1s
  - run: agh sessions get $SID -o json | jq -r '.state' | tee state1.txt
  - assert: state1.txt == "running"   # daemon kept the prompt going (context.WithoutCancel; internal/CLAUDE.md:33-35)
  - run: agh sessions transcript $SID -o json | jq 'length' | tee transcript_count.txt
  - run: # Now exercise the explicit cancel
  - run: curl -X POST http://127.0.0.1:$AGH_HTTP_PORT/api/sessions/$SID/prompt/cancel -o cancel.json
  - wait: 2s
  - run: agh sessions get $SID -o json | jq -r '.state' | tee state2.txt
expected_behavior:
  - state1 = running (request abort did not cancel the prompt).
  - cancel.json = success and state2 ≠ running.
  - Transcript contains the partial response generated before the request was aborted.
  - No raw claim_token in agh.log.
evidence_to_capture:
  - state1.txt, state2.txt, cancel.json, transcript_count.txt.
failure_signatures:
  - state1 == "stopped" or "idle" (detached lifetime regressed).
  - prompt/cancel ignored.
cleanup:
  - agh sessions stop $SID && agh daemon stop
```
```

```markdown
### DB-15 — HTTP and UDS refuse connections cleanly during shutdown

```yaml qa-scenario
id: db-listeners-shutdown-clean
title: HTTP and UDS listen until Shutdown is called, then refuse new connections cleanly
theme: daemon-boot
coverage:
  primary:
    - daemon.shutdown.listener-discipline
live: false
provider: none
```

```yaml qa-flow
steps:
  - run: agh daemon start && sleep 3
  - run: # Open a long-poll SSE connection
  - run: (curl -N "http://127.0.0.1:$AGH_HTTP_PORT/api/observe/events" > sse.log) &
  - set: SSE_PID=$!
  - run: agh daemon stop -o json &
  - wait: 3s
  - run: # While shutting down, attempt a fresh request
  - run: curl --max-time 2 "http://127.0.0.1:$AGH_HTTP_PORT/api/observe/health" 2>&1 | tee shutting.log
  - run: curl --max-time 2 --unix-socket "$LAB_HOME/daemon.sock" http://./observe/health 2>&1 | tee shutting-uds.log
  - wait: until daemon stopped (poll daemon.json absence)
  - run: kill -0 $SSE_PID 2>&1 || echo "sse client closed"
expected_behavior:
  - Existing SSE connection terminates cleanly (server-side close, not crash).
  - New requests during shutdown either fail with a connection refused / EOF, or return 5xx with a structured body — never panic.
  - After Shutdown completes, `daemon.sock` file is gone.
evidence_to_capture:
  - sse.log (must end with a clean stream terminator), shutting.log, shutting-uds.log.
failure_signatures:
  - sse.log shows a panic stack trace.
  - daemon.sock present after Shutdown.
  - HTTP returns 200 after Shutdown completes.
cleanup:
  - rm -f $LAB_HOME/daemon.sock
```
```

## 5. Edge Cases (gates, not full scenarios)

These must be exercised by short asserts inside the QA gate suite:

- **Full disk during migration**: `applyMigration` calls `tx.ExecContext` which can fail with `SQLITE_FULL`. The deferred `rollbackMigrationTx` (`schema.go:387-399`) must still rollback; assert that the daemon never records the migration row.
- **fsync failure** during `WriteInfo` (`info.go:114-117`) — temp file path may leak; verify `os.Remove(tempPath)` (`info.go:106-108`) cleans up.
- **EBUSY on socket creation** — `removeStaleSocket` is unconditional `os.Remove` ignoring `ErrNotExist` (`orphan.go:87-97`); test with a socket file held by an unrelated process.
- **macOS `/private/var/folders` symlink canonicalization** — Skill sidecar containment uses `EvalSymlinks` (`internal/CLAUDE.md:57`); identical pattern must apply when the QA harness puts AGH_HOME under macOS temp paths.
- **Time-jump during heartbeat** — system clock moves backward (e.g. NTP); verify `defaultRestartPollInterval` logic in `restart.go` does not deadlock waiting for a deadline that has retroactively passed.
- **Stale lock from previous PID that has been recycled** — `procutil.Alive(stalePID)` checks `syscall.Kill(pid, 0)` (`procutil.go:13-20`); a recycled PID would erroneously look alive. Document this as a known limitation; the lock content alone is insufficient — `daemon.json.started_at` recovery (`boot.go:419-426`) cross-checks.
- **DB file 0 bytes** (e.g. previous fsync interrupted) — must be detected by `ShouldRecoverSQLite` (`sqlite.go:188-208`); current marker list does not include `"file is empty"`. Add a regression test to either expand the marker list or to fail-fast with a clear error.
- **Lock file with non-numeric content** — `readLockPID` (`lock.go:151-173`) tolerates and returns 0; verify boot proceeds.
- **`daemon.json` from a future schema version** — currently no schema-version field on daemon.json; `Info.Validate` only checks PID/port/started_at. Document as a deferred surface; add a top-level "schema_version" field to enable future hard cuts.
- **`restarts/` directory full of stale operations** — `RestartOperation.terminal()` allows pruning; ensure `daemon.startedAt > restartOperation.UpdatedAt + maxRetention` triggers GC. No GC exists today — flag.
- **`NetworkAuditFile` rotation under load** — append-only log; verify no rotation happens mid-run (or, if rotation lands, that boot tolerates a `.1`/`.2` suffix).
- **Two daemons on different AGH_HOME, same socket path config** — `cfg.Daemon.Socket` from config takes precedence over `homePaths.DaemonSocket`. Test that `removeStaleSocket(state.cfg.Daemon.Socket)` (`boot.go:405-408`) actually targets the configured socket, not the default.
- **`exec.LookPath` resolution for the relaunch binary** — `procutil.resolveLaunchBinary` (`detached.go:76-82`) requires the new binary on PATH; restart helper test must cover the missing-binary case.

## 6. Integration Surfaces

Matrix of dependencies and obligations.

| Other module                    | Daemon obligation to module                                                                 | Module obligation to daemon                                                                                          | Citation                                       |
| ------------------------------- | -------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------- |
| `internal/session`              | Provide `SessionManager` via `newSessionManager` factory; wire HookSet, PromptAssembler.    | Implement `core.SessionManager`; honor `StopWithCause`, `WaitForFinalizations`, `ListAll`.                            | `daemon.go:557-589`, `daemon.go:1290-1318`     |
| `internal/acp`                  | Wrap with `session.NewACPDriverAdapter`; pass `ProcessRegistry` for tool process bookkeeping. | Manage subprocess lifecycle; refuse new prompts after shutdown.                                                       | `daemon.go:583-588`                            |
| `internal/store/globaldb`       | Open via `globaldb.OpenGlobalDB(ctx, path)`; close on shutdown.                              | Implement `Registry` interface (observe + workspace + audit + channel + message + vault store).                       | `daemon.go:537-541`, `daemon.go:1245-1253`     |
| `internal/store/sessiondb`      | Open per-session DB via session manager.                                                     | Drain the per-session writer goroutine on `Close`.                                                                    | `sessiondb/session_db.go:170-178`              |
| `internal/store`                | Apply every embedded Goose stream with `store.Apply`; never reconcile columns through `EnsureSchema`. | Validate `atlas.sum`, reject legacy/ahead state, and preserve transactional failure semantics.                        | `migrate.go`, `migrate_integrity.go`           |
| `internal/heartbeat`            | Wire authoring/status/wake services into RuntimeDeps.                                        | Provide bounded prompt projection; surface diagnostics list.                                                          | `boot.go:830-872`                              |
| `internal/version`              | Expose `Current().Version` to status payloads.                                               | Read-only; mutex-guarded `OverrideVersionForTesting` (test-only).                                                     | `cli/daemon.go:333`, `version.go:34-52`        |
| `internal/procutil`             | Use for `Alive`, `Signal`, `SpawnDetachedLoggedProcess`, `KillCommandProcessGroupAndWait`.    | Build-tag split for unix/windows; `KillProcessGroupIDAndWait` returns explicit unsupported error on windows.          | `procutil_windows.go:12-37`                    |
| `internal/subprocess`           | Manage handshake + health monitor for ACP/extension subprocesses.                            | Cooperative shutdown via `Shutdown(ctx)`; `Wait()` joins; `stopHealthMonitor` is race-free.                           | `subprocess/process.go:320-449`                |
| `internal/diagnostics`          | Use `Redact` and `RedactAndBound` before persisting crash evidence.                          | Globally-registered dynamic secrets; idempotent register/release.                                                    | `diagnostics/redact.go:35-94`                  |
| `internal/api/core`             | Expose handler-side health/status via `BaseHandlers`.                                        | No transport-specific parsing; share via `core.Observer`.                                                            | `daemon.go:944-1023`                            |
| `internal/api/httpapi`          | Constructed by `httpFactory`; receives full `RuntimeDeps`.                                   | Start/Stop lifecycle; clean refuse during shutdown.                                                                   | `daemon.go:944-981`, `boot.go:1600-1644`       |
| `internal/api/udsapi`           | Constructed by `udsFactory`; receives full `RuntimeDeps`.                                    | Same lifecycle.                                                                                                      | `daemon.go:982-1022`                            |
| `internal/network`              | Optional; created when `cfg.Network.Enabled`. Bound to session manager via `networkBindableSessionManager` interface. | Implement `Shutdown(ctx)`; provide `Status()` for daemon.json.                                                       | `boot.go:1067-1105`, `daemon.go:1677-1706`     |
| `internal/observe`              | Daemon-owned; implements `core.Observer`, `session.Notifier`, and `Reconcile`.                | Honor `StartRetention`/`ShutdownRetention` if implemented.                                                            | `daemon.go:88-92, 591-616, 1063-1072`          |
| `internal/automation`           | Optional; gated on `cfg.Automation.Enabled`.                                                 | Implement `Start`/`Shutdown`/`SessionObserver`/`HookTelemetrySink`/`MemoryObserver`.                                  | `boot.go:1257-1299`                            |
| `internal/extension`            | Wire host API and capability checker.                                                        | `Start`/`Stop`/`Reload`/`Get`/`HookDeclarations`.                                                                    | `daemon.go:618-642, 222-228`                   |
| `internal/skills`               | Periodic watcher started in `bootHooks` (`boot.go:1135-1153`).                              | Provide `Registry`, `MCPResolver`, `CatalogProvider`. Respect `cfg.Skills.PollInterval`.                              | `boot.go:291-300, 1135-1153`                   |
| `internal/memory/consolidation` | Optional Dream runtime; started in `Run` (`daemon.go:1083-1085`).                            | Provide `Start(ctx)` / `Shutdown()` / `Enabled()`.                                                                   | `daemon.go:1083-1085`, `boot.go:469-477`       |
| `internal/cli`                  | Daemon CLI verbs live here.                                                                  | Use `commandDeps.signalProcess`, `processAlive`, `executable`, `spawnDetached`, `runRelaunchHelper` injection points. | `cli/daemon.go:478-506`                       |

## 7. DX Cliffs

Concrete, with symptom + minimal repro + fix surface.

1. **`daemon: already running` doesn't always name the lock file path.**
   - Symptom: error message says `daemon: already running with pid 12345` but operators don't know which AGH_HOME owns the collision (matters when running multiple labs).
   - Repro: set `AGH_HOME=$LAB_A`, start; in another shell `AGH_HOME=$LAB_A` start.
   - Fix surface: `internal/daemon/lock.go:24-28`. Include `lock.path` in the formatted error.

2. **`agh status` after `kill -9` reports `starting` for ~poll-interval seconds.**
   - Symptom: `cli/daemon.go:283-300` falls back to `daemonInfo`; if PID still has children (or is recycled), the poll loop returns stale `starting` state.
   - Repro: kill -9, immediately run `agh status -o json`.
   - Fix surface: cross-check `daemon.json.started_at` against `procutil.MatchesStartTime` (`procutil/process_started_at_unix.go`).

3. **Subprocess-stopped-with-non-zero-exit log line prints raw stderr.**
   - Symptom: `subprocess/process.go:489-503` joins `proc.Wait()` errors; if a provider CLI prints an API key in its stderr, that key lands in `agh.log`.
   - Repro: configure an extension whose binary errors with `unauthorized: api_key=<KEY>` on launch.
   - Fix surface: route the joined error through `diagnostics.Redact` before structured logging (consistent with the redact promise in `internal/CLAUDE.md:55`).

4. **Exit codes are inconsistent.**
   - Symptom: `agh daemon stop` returns 1 if "not running"; `agh daemon start` returns 1 if "already running"; both via Cobra's RunE error path. Operators cannot distinguish via exit code.
   - Fix surface: define explicit numeric codes in `internal/cli/...` for "not running" (e.g. 4) vs "already running" (e.g. 5) vs "io" (3) and document.

5. **`daemon.lock` zeroed on clean shutdown but the file persists.**
   - Symptom: `ls -la $LAB_HOME` after a clean stop still shows `daemon.lock`. Newcomers think the lock is leaked.
   - Fix surface: either `os.Remove` after `Unlock` in `Lock.Release` (`lock.go:124-149`), or document the convention with a comment in the file.

6. **Boundary check is opt-in via env var.**
   - Symptom: a developer adds an illegal import; `make verify` passes locally because they didn't export `AGH_DEV_VERIFY_BOUNDARIES=1`. They only learn from CI.
   - Fix surface: turn the runtime check on by default for development builds (use `version.Version == "dev"`).

7. **Restart-pending sentinel handling.**
   - Symptom: if a relaunch helper aborts after writing `RestartStatusStarting` but before the new daemon writes a fresh `daemon.json`, `agh status` will keep returning the OLD `daemon.json` — operators think the upgrade succeeded.
   - Fix surface: `cli/daemon.go:283-300` should consult `restartStore.LatestOperation` before trusting `daemon.json`; mark "restart in progress" prominently.

8. **`ShouldRecoverSQLite` marker list is hardcoded English strings.**
   - Symptom: future SQLite versions or modernc-sqlite revs may change error wording; recovery silently regresses to non-recovery.
   - Fix surface: also recover on the structured `sqlite.Error` code where `ExtendedCode` is `SQLITE_CORRUPT (11)` or `SQLITE_NOTADB (26)` (`sqlite.go:188-208`).

9. **Heartbeat preferences silently truncate at `MaxBodyBytes`.**
   - Symptom: HEARTBEAT.md exceeds bytes → diagnostic stored, prompt projection truncated. Operators see truncated output but no top-level "your heartbeat is too long" log.
   - Fix surface: `heartbeat.go:466-481` — emit a `slog.Warn` at the daemon level when boot-time resolution surfaces an `oversized_*` diagnostic.

10. **`agh status -o json` `network` field missing when `daemon.json` was written before the network bound itself.**
    - Symptom: legitimate race during boot; status JSON has `network: null` for ~1s. Web UI renders "network: unknown".
    - Fix surface: stage `WriteInfo` to update `daemon.json` after `bootNetwork` completes (currently `bootServers → daemonNetworkInfo → WriteInfo`, `boot.go:1622-1638`, runs only once).

## 8. Failure Modes QA Must Catch

If any of these slip, we ship a broken daemon.

1. **Raw `claim_token` ever reaching disk or wire.** `grep -E '\bagh_claim_[A-Za-z0-9]+\b' agh.log` returns no matches across DB-04 / DB-11 / DB-14 / DB-10 evidence. Same grep across all `*.json` evidence files. (Cited claim: `internal/CLAUDE.md:55`.)
2. **Orphan ACP subprocess after `agh daemon stop`.** Verified by DB-05 step `kill -0 $CHILD_PID`. (Cited claim: `internal/CLAUDE.md:36-37`.)
3. **Migration applied twice.** Each `schema_streams.applied_count` remains equal to the embedded gap-free SQL count after any boot/restart cycle.
4. **`daemon.lock` remaining with non-zero PID after clean stop.** Always `0\n` after `Lock.Release` (`lock.go:124-149`).
5. **`daemon.json` left behind after clean stop.** `RemoveInfo` runs in `shutdownPersistentResources` (`daemon.go:1245-1248`).
6. **`agh.db` 0-byte after crash.** Detected by `ShouldRecoverSQLite` recovery path; if the file is exactly 0 bytes, marker list must include the appropriate signature.
7. **`agh daemon start` after `kill -9` reports stale PID.** Stale PID handling (`lock.go:88-92`, `boot.go:411-427`) cleanly transitions to a fresh boot.
8. **Schema differs between two clean restarts.** DB-09 hash-equality.
9. **HTTP/UDS panic during shutdown.** DB-15 sse.log must not contain a goroutine stack trace.
10. **Goroutine leak after Shutdown.** DB-13 goleak gate.
11. **Detached prompt cancelled by transport client disconnect.** DB-14 state1 == "running".
12. **Boundary violation in production code.** `daemon.Boundaries(ctx)` returns nil during boot when `AGH_DEV_VERIFY_BOUNDARIES=1`.
13. **Cross-build for Windows fails on `procutil`/`subprocess` work.** `GOOS=windows GOARCH=amd64 go build ./...` exit 0.
14. **Recovery path leaves orphaned `-wal` / `-shm` next to a fresh DB.** DB-08 files.txt validation.

## 9. Fixtures / Bootstrap Requirements

The QA harness for daemon-boot must:

- Use `agh-qa-bootstrap` to obtain `bootstrap-manifest.json` + `bootstrap.env` with:
  - **Unique `AGH_HOME`** under `~/.agh-qa/<run-id>/lab-home` (per worktree-isolation directive in CLAUDE.md and the standing directive on parallel QA runs).
  - **Unique daemon ports** for HTTP and any other listener; default port `:2123` is forbidden.
  - **Unique `tmux-bridge` socket path** if the bridge is exercised.
  - `PROVIDER_HOME` and `PROVIDER_CODEX_HOME` derived from the manifest — never `~/.codex` directly.
  - `AGH_WEB_API_PROXY_TARGET` exported when the daemon is not on `:2123`.
- Provide a **real Claude Code agent** path: `agh sessions start --agent claude-code --workspace ./fixtures/<theme>` (no fictional CLIs).
- Provide a **deterministic mock ACP** path for non-LLM scenarios: the existing `internal/acp/acpmock` test binary, used only for scenarios explicitly tagged `provider: none`.
- Provide a **goleak harness** under `internal/daemon/qa/` (or co-resident in `daemon_test.go` under build tag `daemon_signal_qa`/`daemon_goleak_qa`) so signal-during-boot, goleak-clean-shutdown, and boot-failure-cleanup gates run without the full E2E setup.
- Provide a **schema-validator script** (`scripts/qa/schema-validator.sh`) that diffs the live `agh.db .schema` against a checked-in golden file; fails on drift.
- Provide a **claim-token redaction grep gate** (`scripts/qa/claim-token-grep.sh`) run after each scenario over every captured artifact.
- Provide **artifact layout** under `.artifacts/qa/<run-id>/daemon-boot/db-<NN>/` per scenario:
  - `qa-report.md` — Worked / Failed / Blocked / Follow-up summary.
  - `qa-summary.json` — machine-readable.
  - `qa-output.log` — combined stdout/stderr.
  - `qa-observed-events.json` — SSE events (redacted by default).
- Provide a **bakeoff order** for the cross-LLM lane (per `_references/openclaw-qa-patterns.md` §6): GPT → Claude → Gemini, but daemon-boot scenarios are provider-symmetric (the boot flow does not depend on a specific LLM family). Mark scenarios `live: true` so the bakeoff lane picks them up only when running the live frontier.
- Provide a **windows cross-build gate** target in `Makefile`: `make crossbuild-windows` running `GOOS=windows GOARCH=amd64 go build ./...` and failing the QA round if any package under `internal/daemon`, `internal/procutil`, `internal/subprocess` fails to compile.
- Provide a **`bootstrap-manifest.json` reuse rule**: a fresh manifest per run by default; reuse only when the QA loop is explicitly continued.

## 10. Citations

- `cmd/agh/main.go:1-18` — single binary entrypoint; all real behavior delegated to `internal/cli`.
- `internal/cli/daemon.go:28-507` — CLI verbs, status/start/stop/relaunch wiring, detached-spawn helper.
- `internal/daemon/daemon.go:1-1318` — composition root + Options + Run/Shutdown sequencing; canonical place where every other subsystem is wired.
- `internal/daemon/boot.go:152-1779` — boot pipeline (18-step list at line 178), `bootCleanup` LIFO unwinding, lock acquisition, registry open, session-repair-on-boot.
- `internal/daemon/lock.go:1-194` — singleton file lock + PID file with stale-PID recovery via `procutil.Alive`.
- `internal/daemon/info.go:1-155` — atomic `daemon.json` write/read/remove with `Validate`.
- `internal/daemon/orphan.go:1-126` — orphan PID cleanup on stale-PID detection; stale socket removal.
- `internal/daemon/restart.go:1-200+` — durable `RestartOperation` validation; relaunch helper protocol.
- `internal/daemon/spawn_reaper.go:1-100` — TTL/parent-stopped/orphaned classification with lease release coupling.
- `internal/daemon/boundary.go:1-119` — best-effort import boundary verification opt-in via env.
- `internal/daemon/agent_probes.go` — agent probe target collection (used for diagnostics).
- `internal/diagnostics/redact.go:1-110` — `Redact`, `RedactAndBound`, `RegisterDynamicSecret` lifecycle.
- `internal/heartbeat/heartbeat.go:1-500+` — HEARTBEAT.md resolution with strict YAML, body-authority rejection, prompt projection.
- `internal/store/store.go:1-110` — file/db constants, registry interfaces (SessionRegistry composes everything).
- `internal/store/sqlite.go:1-231` — `OpenSQLiteDatabase` with WAL configuration, busy_timeout, corrupt-DB recovery, `-wal`/`-shm` companion rename.
- `internal/store/migrate.go` + `migrate_integrity.go` — Goose stream application, ahead/legacy refusal, and `atlas.sum` validation.
- `internal/store/meta.go:1-77` — atomic session-meta write via `fileutil.AtomicWriteFile` + `syncDirectory`.
- `internal/store/globaldb/schema/` — declarative global schema, embedded gap-free Goose SQL, and `atlas.sum`.
- `internal/store/sessiondb/session_db.go:1-180` — per-session DB with dedicated writer goroutine; sequence numbering; canonical event schema string.
- `internal/version/version.go:1-58` — build metadata, `OverrideVersionForTesting` mutex.
- `internal/procutil/procutil.go:1-32` — `Alive`/`Signal` thin wrappers around `syscall.Kill`.
- `internal/procutil/process_group_unix.go:1-255` — `ConfigureCommandProcessGroup`, `SignalProcessGroupID` with Linux `/proc/<pid>/stat` enumeration, `KillProcessGroupIDAndWait` with EPERM-after-exit reconciliation.
- `internal/procutil/process_group_windows.go:1-37` — explicit unsupported-error stubs (parity gap).
- `internal/procutil/detached.go:1-139` — `SpawnDetachedLoggedProcess` with log-tail attachment on failure.
- `internal/subprocess/process.go:1-500+` — managed subprocess: launch, JSON-RPC transport, cooperative shutdown, kill-after-timeout, lifecycle context with `context.WithoutCancel`.
- `internal/subprocess/health.go:1-188` — health monitor with `ConsecutiveFailures` threshold, race-free stop.
- `internal/subprocess/handshake.go:1-100` — initialize protocol; bridge runtime version negotiation.
- `internal/api/core/handlers.go:868-925` — `Health` and `DaemonStatus` handler implementations shared by HTTP and UDS.
- `internal/api/httpapi/routes.go:43,72,118,239,251` — health/status/diagnostics endpoint registration.
- `internal/api/udsapi/routes.go:38,71,140,274,288` — UDS parity registration.
- `internal/CLAUDE.md:32-37,55-57,124-129` — load-bearing invariants: detached lifetime, process-group parity, claim_token redaction, symlink hardening, lifecycle hook semantics.
- `CLAUDE.md` (root): greenfield zero-legacy rule; `make verify` gate; commit style; worktree isolation rule; provider-home isolation; subagent read-only rule.
- `_references/openclaw-qa-patterns.md` — scenario shape, real-LLM vs mock policy, four-artifact contract, bakeoff loop.
- `_references/hermes-qa-patterns.md` — sister reference for hermes-style QA composition.
