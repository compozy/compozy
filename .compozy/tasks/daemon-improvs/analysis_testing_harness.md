# Testing Architecture Comparative Analysis: looper vs AGH

## 1. Quick Assessment — looper's Current Test Coverage Shape

### What looper has

**Daemon package** (`/Users/pedronauck/Dev/compozy/looper/internal/daemon/`, ~5,200 LOC of test code):

- `boot_test.go` (597 LOC) — unit-style, stubbed `AcquireDaemonLock`, lifecycle bookkeeping.
- `boot_integration_test.go` (308 LOC) — real `Start()` boot by re-invoking the test binary as a child process via `TestDaemonHelperProcess` (env var `COMPOZY_DAEMON_HELPER=1`). Covers singleton, stale-artifact recovery, home-scoped layout, cross-workspace sharing. **No `//go:build integration` tag.**
- `info_lock_test.go` (321) — info file / lock round-trip.
- `purge_test.go` (165), `reconcile_test.go` (257), `watchers_test.go` (362) — narrow unit tests.
- `run_manager_test.go` (2,548) — large single file covering the daemon's core run orchestrator through a single shared harness `newRunManagerTestEnv` + `runManagerTestDeps` injection (prepare/execute callbacks stub the real work).
- `run_manager_bench_test.go` (62) — single benchmark `BenchmarkRunManagerListWorkspaceRuns`.
- `service_test.go` (147), `shutdown_test.go` (197) — in-process HTTP via `httptest` + gin, no real UDS.
- `review_exec_transport_service_test.go` (413) — transport surface tests.

**Executor package** (`/Users/pedronauck/Dev/compozy/looper/internal/core/run/executor/`, ~5,200 LOC of test code):

- `execution_acp_integration_test.go` (1,655) — **this is the closest thing to a real ACP mock harness in looper.** It installs a shell-script shim onto `PATH` that re-execs the test binary under `TestRunACPHelperProcess`, which then implements `acp-go-sdk` using an in-test `runACPHelperAgent`. Scenarios are passed via the `GO_RUN_ACP_HELPER_SCENARIOS` env var (JSON-encoded `runACPHelperScenario`). See `installACPHelperOnPath`, `loadRunACPHelperScenario`.
- `execution_acp_test.go` (1,159) — unit-level ACP pipeline tests.
- `execution_test.go`, `execution_ui_test.go`, `event_stream_test.go`, `result_test.go`, `runtime_guard_test.go`, `test_helpers_test.go` — focused unit coverage.

**Root test package** (`/Users/pedronauck/Dev/compozy/looper/test/`):

- `helpers_test.go` — 17 lines, only `repoRoot()`.
- `public_api_test.go` — exercises `compozy.Prepare` / `compozy.Run` in dry-run mode.
- `release_config_test.go`, `skills_bundle_test.go`.

### Characteristics

- **Single-process orientation.** Every looper test runs inside `go test` — no spawned daemon binary, no UDS/HTTP client reaching into it.
- **Harness code is local to each _test.go file.** There is no `internal/testutil`, no `internal/extensiontest`, no `internal/e2elane` package. Helpers are per-file (`newRunManagerTestEnv`, `installACPHelperOnPath`).
- **No `//go:build integration` or `e2e` build tags** on any Go file (the one hit was in an archived doc, not source).
- **No artifact capture infrastructure.** No ArtifactManifest, no ArtifactKind registry, no per-run artifact collector.
- **No fault-injection fixture format.** Scenarios are ad-hoc Go structs, not versioned JSON fixtures.
- **Single benchmark file** and it's narrow (one function).
- **Restart testing** is absent — crash-recovery is exercised only via `TestStartRecoversAfterKilledDaemonLeavesStaleArtifacts` at the boot layer (SIGKILL, observe stale socket cleanup). No restart operation state machine.
- **No test lane orchestration.** `make test` runs everything in one pass (fmt/lint/test/build). No separation of unit / integration / e2e / nightly.

---

## 2. Gaps

Each gap names specific AGH functions/files, what they test, why (or why not) looper needs the same, a concrete action, and priority.

### Gap 1 — No reusable "start a real daemon" runtime harness

**AGH reference**: `/Users/pedronauck/dev/compozy/agh/internal/testutil/e2e/runtime_harness.go` (1,333 LOC).

**What it does**: `e2etest.StartRuntimeHarness(t, RuntimeHarnessOptions{...})` builds the `agh` binary once per process (under `buildBinaryMu`, cached as `builtBinaryPath`, honoring `AGH_TEST_DAEMON_BIN`), writes an isolated home directory + TOML config, seeds workspace and mock agents, launches the daemon as a real subprocess, polls readiness over UDS/HTTP, resolves a workspace, and registers `t.Cleanup` to SIGTERM the process. It then exposes typed helpers for every public surface: `CreateSession`, `PromptSession`, `PromptSessionHTTP`, `SessionTranscript`, `CreateNetworkChannel`, `NetworkSend`, `SeedAutomationFixtures`, `InstallExtension`, `ApproveSessionPermission`, plus SSE stream parsing (`readSSERecordsWithCallback`) and a `CLI *CLIClient` that shells out to the real binary.

**Supporting helpers**:
- `ArtifactCollector` in `artifacts.go` — stable per-run manifest (`transcript.json`, `events.json`, 20+ `ArtifactKind` constants) written under a deterministic slug.
- `ConfigSeedOptions` + `configSeedFile` TOML writer in `config_seed.go`.
- `WorkspaceSeedOptions`, `MockAgentSpec` seeding in `mock_agents.go`.
- `transport_parity.go` ensures HTTP and UDS surfaces return identical results.

**Why it matters for looper**: looper's closest equivalent is the `boot_integration_test.go` helper pattern (`startDaemonHelperProcess` that re-execs the test binary under a named test), but it does not give callers a rich API surface. Every meaningful daemon-level integration test in looper today re-mocks the run manager via `newRunManagerTestEnv` — none of them actually send requests to a running daemon, so we have no assurance that wire contracts (UDS, HTTP, SSE) match the in-process implementation. As looper's daemon grows (task-run / review-run / exec-run modes, transport services, watchers, journals), the in-process `runManagerTestEnv` approach will keep diverging from what a real `compozy` binary exposes.

**Action**: Create `/Users/pedronauck/Dev/compozy/looper/internal/testutil/e2e/` with:
1. `runtime_harness.go` — `StartRuntimeHarness(t, Options)` that builds `./cmd/compozy`, seeds `$HOME`, launches the daemon, and polls `/api/daemon/status` readiness.
2. Typed wrappers around the run/task/review surfaces already implemented in `internal/api/core`.
3. An `ArtifactCollector` keyed to `runID` that snapshots `events.json`, `result.json`, and run.db rows.
4. A `CLIClient` that shells out to `bin/compozy` for parity tests.

**Priority**: **P0** — this is the foundation every other gap depends on.

---

### Gap 2 — No separation between unit, integration, and e2e lanes

**AGH reference**: `/Users/pedronauck/dev/compozy/agh/internal/e2elane/lanes.go` + `lanes_test.go`.

**What it does**: Defines four lanes (`LaneRuntime`, `LaneWeb`, `LaneCombined`, `LaneNightly`) each with a `Plan{GoSuites, ScriptSuites, RequiresDaemonServedBrowser, IncludesCredentialedNightly}`. Patterns `RuntimeE2EPattern = "^TestDaemonE2E"` and `NightlyRuntimeE2EPattern = "^TestDaemonNightlyE2E"` give `go test -run` isolation. `PlanForLane` returns defensively-cloned suites. `TestLanePatternsKeepNightlyDaemonScenariosOutOfDefaultRuntimeLane` is the negative test that protects the PR-required lane from accidentally pulling in nightly/credentialed suites.

The `command_wiring_test.go` companion verifies `make test-e2e-*` targets delegate to mage targets and that `package.json` scripts match.

The AGH daemon tests use `//go:build integration && !windows` build tags on every `_integration_test.go` file, so default `go test ./...` stays fast and the runtime harness never boots during the unit lane.

**Why it matters for looper**: Right now `make verify` runs one blob. When the harness from Gap 1 lands, every integration test will spin up a real binary, which is 20s+ per test. Without a lane split they will bloat `make test` and slow `make verify`. The naming convention (`TestDaemonE2E*`) also makes it trivial to parallelize CI matrices later.

**Action**:
1. Add `//go:build integration` tags to `boot_integration_test.go` and any new harness-based tests.
2. Adopt a test name prefix convention (e.g. `TestDaemonE2E*`) and document it.
3. In `Makefile` add `test`, `test-integration`, `test-e2e` targets. Keep `make verify` = unit-only lane; run integration on a separate target.
4. Optional: a minimal `internal/testlane/` package like AGH's `e2elane` if looper grows multiple suites.

**Priority**: **P1** — necessary before the harness lands or the unit loop will suffer.

---

### Gap 3 — No ACP fixture format / driver binary with fault injection

**AGH reference**: `/Users/pedronauck/dev/compozy/agh/internal/testutil/acpmock/` (fixture.go 519 LOC, registration.go 197, driver_binary.go 100, diagnostics.go 69, plus `cmd/acpmock-driver` and `testdata/`).

**What it does**:
- Versioned (`FixtureVersion = 2`) JSON fixtures with schema validation (`DisallowUnknownFields`): `Fixture{Agents: []AgentFixture{Turns: []TurnFixture{Match, Steps: []Step}}}`.
- **Turn selectors** (`TurnMatch`): exact matching on `turn_source` (user / network), `user_text`, `occurrence`, and a `TurnMatchNetwork` with `MessageID`, `Channel`, `From`, `To`, `InteractionID`, `ReplyTo`, `TraceID`, `CausationID` — i.e. the fixture can route different turns for the same prompt depending on whether it came from a human or a network envelope.
- **Step kinds** span the full ACP vocabulary: `assistant`, `thought`, `tool_call`, `permission`, `environment_exec`, `bridge_response`, `driver_control`.
- **Fault injection via `DriverControlStep`**: `disconnect`, `write_raw_jsonrpc` (inject invalid frames), `block_until_cancel`, plus `async` and `delay_ms` knobs.
- **Out-of-band driver binary**: `driver_binary.go` caches a built `acpmock-driver` executable (`AGH_TEST_ACPMOCK_DRIVER_BIN` override, 45s build timeout, `driverBinaryMu` singleton). `Register(homePaths, opts)` writes a real `AGENT.md` referencing that binary, so the daemon launches a completely separate process for each mock agent — exactly like production.
- **Diagnostics**: every prompt execution appends a `DiagnosticsRecord` (prompt, meta, selected turn, executed steps with outputs/errors/exit codes) to a JSONL file the test can read back with `ReadDiagnostics`.

Fault tests that depend on this: `daemon_acpmock_faults_integration_test.go` — `TestDaemonE2EACPmockCrashMidStreamProjectsRuntimeFailure`, `TestDaemonE2EACPmockInvalidFrameProjectsRuntimeFailure`, `TestDaemonE2EACPmockPermissionDisconnectProjectsRuntimeFailure`. All three assert the SSE stream carries an `error` event, transcript has the pre-crash fragment, no `done` event surfaces, and run-level artifacts are captured.

**Looper's current approach** (`execution_acp_integration_test.go:1329 installACPHelperOnPath`):
- Ad-hoc shell script `#!/bin/sh exec <test binary> -test.run=TestRunACPHelperProcess -- "$@"` written to a tmp dir prepended to PATH.
- Scenario carried through `GO_RUN_ACP_HELPER_SCENARIOS` env var (JSON-encoded `[][]runACPHelperScenario`).
- Single counter file (`GO_RUN_ACP_HELPER_COUNTER_FILE`) picks which scenario per invocation.
- Scenario is struct-typed in Go (`runACPHelperScenario{SessionID, ExpectedLoadSessionID, ExpectedPromptContains, Updates, StopReason, BlockUntilCancel, NewSessionError, PromptError, PromptErrorAfterUpdates}`) — no JSON fixture file on disk, no cross-test reuse.
- Fault injection is limited to two flags: `BlockUntilCancel` and `PromptErrorAfterUpdates`. **No crash-mid-stream, no invalid-frame injection, no permission-disconnect scenarios.**
- No diagnostics artifact.

**Why it matters for looper**: looper runs a daemon that shells out to `codex-acp`, `claude-code`, `droid`, `cursor`. Every bug where an agent crashes mid-stream, sends a malformed JSON-RPC frame, or disconnects during a permission prompt is currently uncoverable. The existing helper proves the re-exec pattern works; what's missing is the fixture schema and the fault-injection vocabulary.

**Action**:
1. Port the fixture format (or a narrowed subset — drop network matchers for now since looper doesn't have a network channel feature) to `/Users/pedronauck/Dev/compozy/looper/internal/testutil/acpmock/`.
2. Move `runACPHelperAgent` out of `execution_acp_integration_test.go` into `cmd/acpmock-driver/` so the driver is a real binary.
3. Add `DriverControlStep` equivalents: `disconnect`, `write_raw_jsonrpc`, `block_until_cancel`.
4. Write `ReadDiagnostics` and have the driver emit one JSONL record per prompt for post-hoc assertions.
5. Back the existing `installACPHelperOnPath` tests with fixtures so they share vocabulary with future daemon-level tests.

**Priority**: **P0 for fault scenarios, P1 for migrating existing tests**. The crash/disconnect/invalid-frame gaps are real production risks.

---

### Gap 4 — No restart operation state machine tests

**AGH reference**:
- `/Users/pedronauck/dev/compozy/agh/internal/daemon/restart.go` (implementation).
- `/Users/pedronauck/dev/compozy/agh/internal/daemon/restart_test.go` (1,133 LOC).
- `/Users/pedronauck/dev/compozy/agh/internal/daemon/restart_integration_test.go` (228) — `TestRequestRestartPersistsPreRestartContextBeforeShutdownSignal`, `TestRelaunchHelperFailurePersistsAfterOldDaemonExit`, `TestBootMarksRestartOperationReadyAfterFreshDaemonInfo`.

**What it tests**: a persisted restart-operation store with status transitions (`pending → stopping → waiting_release → starting → ready` / `failed`), fault scenarios where the replacement boot dies before the new daemon is ready, helper processes that supervise the old daemon's release before launching the replacement. Uses `sequentialTime` clock injection, an `oldAlive atomic.Bool` to simulate process lifetime, and `newRelaunchHelper` with configurable poll interval / release timeout / ready timeout.

**Why it matters (or not) for looper**: looper's daemon doesn't yet have a `restart` operation. `compozy daemon restart` today just does stop-then-start with no durable state in between, which is fine for a small binary. If looper intends to support in-place upgrades, extension hot-reload, or config reload without losing in-flight runs, this gap becomes critical. Otherwise it's a deferred concern.

**Action**:
- **If a restart command is on the roadmap**: adopt AGH's store pattern (`newRestartStore`, `RestartOperation`, `RestartStatus*`, `Transition`) and port the three integration tests verbatim against looper's home paths.
- **If not**: skip. Revisit when `compozy daemon reload` or zero-downtime upgrade lands.

**Priority**: **P2** (deferred unless zero-downtime restart is on the near-term roadmap).

---

### Gap 5 — Benchmark coverage is minimal

**AGH reference**:
- `/Users/pedronauck/dev/compozy/agh/internal/daemon/perf_bench_test.go` (319 LOC): `BenchmarkResourceCatalogSnapshotAgentRecords`, `BenchmarkResourceAgentCatalogResolveAgentWorkspaceHit`, `BenchmarkAgentSkillSourceSyncerSyncNoop`, `BenchmarkToolMCPSourceSyncerSyncNoop`. Each uses `b.Loop()` (Go 1.24+ benchmark loop), `b.ReportAllocs()`, and helper factories that wire real `resources.Kernel` against a per-benchmark SQLite DB (`OpenGlobalDB(..., b.TempDir())`).
- `/Users/pedronauck/dev/compozy/agh/internal/extensiontest/perf_bench_test.go` (122): `BenchmarkBuildConformanceMatrix`, `BenchmarkScriptedPromptDriverPrompt`, `BenchmarkReadJSONLinesFileStateRecord`.

**Looper's current coverage**: `run_manager_bench_test.go` has a single benchmark (`BenchmarkRunManagerListWorkspaceRuns`) that uses the classic `for i := 0; i < b.N; i++` loop and doesn't set `b.ReportAllocs()`.

**Why it matters**: the daemon is hot-path for every CLI command. Catalog lookups, run list pagination, and event-journal reads are called per keystroke once the TUI polls the daemon. Without baseline benchmarks, regressions only surface as "the UI feels slow."

**Action**:
1. Add `b.ReportAllocs()` and migrate to `b.Loop()` in the existing benchmark.
2. Add benchmarks for: event journal page reads, run snapshot assembly, workflow catalog lookups (if looper has an equivalent catalog), workspace resolution, JSONL deltas replay.
3. Consider `testing.B.ReportMetric` for domain metrics (events/s, bytes/op).

**Priority**: **P2** — not blocking, but cheap to add alongside Gap 1's harness.

---

### Gap 6 — No artifact capture / manifest pattern

**AGH reference**: `/Users/pedronauck/dev/compozy/agh/internal/testutil/e2e/artifacts.go` (461 LOC). Defines 20 `ArtifactKind` constants (`ArtifactKindTranscript`, `ArtifactKindEvents`, `ArtifactKindNetworkMessages`, `ArtifactKindBridgeHealth`, `ArtifactKindProviderCalls`, `ArtifactKindToolHostDiagnostics`, `ArtifactKindCombinedFlow`, `ArtifactKindSessionEnvironment`, `ArtifactKindBrowserTrace`, etc.) each with a stable relative path. `CaptureJSON`, `ArtifactPath`, and an `ArtifactManifest{Version, Artifacts}` give every test a deterministic set of output files for post-run debugging.

**Why it matters for looper**: when an e2e test fails in CI, you need the transcript, event journal, result.json, and any diagnostics persisted as CI artifacts. Without a manifest pattern, each test cobbles together its own `t.TempDir()` dance. The manifest also becomes a stable API for release-qualification dashboards.

**Action**: alongside Gap 1, add a narrow `ArtifactCollector` with only the kinds looper needs today: `transcript` (session-transcript file), `events` (event-journal JSONL), `result` (result.json), `run_db` (sqlite snapshot path), `logs` (out/err logs). Wire `t.Cleanup` to print manifest paths on failure.

**Priority**: **P1** — small cost, big debugging payoff once e2e tests exist.

---

### Gap 7 — No extension-test harness

**AGH reference**: `/Users/pedronauck/dev/compozy/agh/internal/extensiontest/` (bridge_adapter_harness.go is 1,756 LOC alone; bridge_conformance_matrix.go 456; bridge_adapter_harness_integration_test.go 372). Provides a standardized marker-file contract (`EnvHandshakePath`, `EnvOwnershipPath`, `EnvStatePath`, `EnvDeliveryPath`, etc.) so a reference extension binary can emit JSONL records the test harness reads back to assert on extension lifecycle events. Plus a conformance matrix `BuildConformanceMatrix` that scores providers against `CoverageTargetMultiInstance`, `CoverageTargetRestartRecovery`, `CoverageTargetAuthDegradation`.

**Why it matters for looper**: looper _does_ have extensions (`internal/core/extension`, `sdk/extension`, `sdk/extension-sdk-ts`), and it has `extension_bridge.go` in daemon. But it has no extension-conformance harness — extension-hook tests are scattered through `run_manager_test.go` (`TestExtensionBridgeStartRunCreatesDetachedExecRun`, `TestExtensionBridgeStartRunCreatesDetachedTaskRun`, `TestExtensionBridgeStartRunCreatesDetachedReviewRun`) and use in-process mocks.

**Action**: if third-party extensions become a supported surface, adopt the marker-file adapter pattern. For now, until an external extension ecosystem exists, this is deferred.

**Priority**: **P3** (deferred until looper ships a public extension SDK that external authors use).

---

### Gap 8 — Race-detection strategy is implicit

**Both repos**: run `go test -race` via `make test`. Neither repo documents race-sensitive hotspots.

**AGH**: many integration tests are structured specifically to hit race conditions — e.g. `TestRunManagerAllowsConcurrentDistinctRunIDsAndStreamsLiveEvents` (looper has a similar test name) depends on real goroutine scheduling. The mock-agent isolation test `TestDaemonE2EMockAgentsRemainIsolated` runs two mock-agent subprocesses in parallel with `t.Parallel()` to flush out cross-session contamination.

**Looper**: `run_manager_test.go` uses `t.Parallel()` sparingly (the env helper does not call it). A fast scan shows most daemon tests rely on sequential execution.

**Action**: nothing architectural. When Gap 1 lands, ensure the harness calls `t.Parallel()` where isolation is real (each harness has its own `t.TempDir()`, own `$HOME`, own socket). Port AGH's pattern of `t.Parallel()` at the top of integration tests.

**Priority**: **P2** — improvement, not a gap.

---

### Gap 9 — No CLI-parity tests

**AGH reference**: The runtime harness exposes `h.CLI *CLIClient` (`runtime_harness.go:95`). `CLIClient.RunJSONInDir(ctx, dir, &out, args...)` shells out to the built `agh` binary. Tests like `TestDaemonE2EMemoryCatalogCLIHTTPParityAndLegacyPathIsolation` invoke the memory write/search both via CLI and HTTP and diff the results.

**Why it matters for looper**: looper's daemon exposes the same operations via UDS, HTTP, and CLI. Today nothing asserts those three paths return identical JSON. When a refactor touches the task_runtime_form, one surface can drift without the others complaining.

**Action**: once Gap 1 lands, add `transport_parity_test.go` that exercises `compozy run list`, `compozy run get`, `compozy daemon status` via CLI, UDS, HTTP and diffs the responses. AGH's `transport_parity.go` is a good model (only 169 LOC).

**Priority**: **P1** — cheap to add once the harness exists; catches real regressions.

---

### Gap 10 — No test-utility package for shared primitives

**AGH reference**: `/Users/pedronauck/dev/compozy/agh/internal/testutil/testutil.go` — exports `Context(t)` (cancel-on-cleanup) and `FreeTCPPort(t)` (pseudo-random free port picker that avoids reuse under `-race` + parallel packages).

**Looper**: no `internal/testutil`. `Context(t)` equivalents are inline per-test. No free-port helper (looper doesn't yet bind TCP ports in integration tests, so this hasn't bitten yet).

**Action**: create `/Users/pedronauck/Dev/compozy/looper/internal/testutil/testutil.go` with at minimum `Context(t)` and `WaitFor(t, timeout, desc, pred func() bool)` (looper has `waitForCondition`, `waitForRun`, `waitForString` duplicated across files — centralize them).

**Priority**: **P2**.

---

## 3. Explicitly Skipped — AGH Tests for Features looper Doesn't Have

These AGH test suites cover features that are out of scope for looper. They are listed so future readers know the omission is intentional, not an oversight.

| AGH test file | Feature | Why skipped for looper |
|---|---|---|
| `daemon_network_collaboration_integration_test.go` (951 LOC) | Multi-agent network channels, direct-reply, peer IDs, envelope routing (`CreateNetworkChannel`, `NetworkSend`, `NetworkInbox`) | looper has no network/collaboration subsystem. `TestDaemonE2ENetworkDirectReplyLifecycleWithMockAgents` etc. exercise `--session --channel --kind say/direct` CLI flows that don't exist in compozy. |
| `daemon_bridge_extension_integration_test.go` (560) | External bridge extensions (Telegram, Slack-style), `InstallExtension`, `ComputeDirectoryChecksum`, bridge route persistence | looper's "extensions" are in-process hooks into the run pipeline (run.pre_start, job.pre_execute). It has no external message-bridge concept. |
| `daemon_automation_task_integration_test.go` (458) | Webhook triggers, cron-like jobs (`SeedAutomationFixtures`, `DeliverGlobalWebhook`, `AutomationScopeWorkspace`, `ScheduleModeEvery`) | looper has no automation/trigger surface; runs are initiated by CLI invocation only. |
| `daemon_memory_e2e_integration_test.go` (467) | Persistent per-user / per-project memory with CLI / HTTP parity (`memory.MemoryTypeUser`, `memory.ScopeWorkspace`, search API) | looper has no memory subsystem. |
| `daemon_environment_sandbox_integration_test.go` (489) | Environment profiles, sandboxed tool-host execution with allow/block outcomes, Daytona provider nightly | looper executes agents on the host machine directly. No environment sandboxing. |
| `daemon_nightly_combined_integration_test.go` (785) | Nightly credentialed runs combining automation + bridge + network + environment | Combines the above out-of-scope features. |
| `daemon_mock_agents_integration_test.go` → `TestDaemonE2EToolPermissionFixtureEventsSurface` parts that cover network permission approval | ACP permission approve-always flow via SSE | looper's permission flow today is CLI-interactive; the SSE permission contract would only matter once daemon-mediated approvals land. Partial value — worth revisiting. |
| `e2elane` `LaneWeb`, `LaneNightly`, `DaytonaNightlyE2EPattern` | Playwright browser tests + Daytona provider | looper has no web UI (it's a TUI). Lane infra itself is still useful (Gap 2); only the Web/Daytona-specific lanes are skipped. |
| `extensiontest/bridge_conformance_matrix.go` + `bridge_adapter_harness.go` | Third-party bridge conformance scoring | looper has no bridge extension surface (see Gap 7). |

---

## Summary Action Table

| Priority | Action | Effort | Dependencies |
|---|---|---|---|
| P0 | Build `internal/testutil/e2e` runtime harness (Gap 1) | Large (~1,500 LOC incl. helpers) | none |
| P0 | ACP fixture schema + out-of-process driver binary with fault injection (Gap 3) | Medium (~800 LOC) | none, but benefits from Gap 1 |
| P1 | Integration/e2e lane split + `//go:build integration` tags (Gap 2) | Small | Gap 1 |
| P1 | `ArtifactCollector` + `ArtifactKind` manifest (Gap 6) | Small | Gap 1 |
| P1 | CLI/UDS/HTTP transport parity tests (Gap 9) | Small | Gap 1 |
| P1 | Migrate existing `installACPHelperOnPath` callers to the new fixture driver (Gap 3 follow-up) | Medium | Gap 3 |
| P2 | `internal/testutil` shared primitives (`Context`, `WaitFor`, `FreeTCPPort`) (Gap 10) | Trivial | none |
| P2 | Expand daemon benchmarks with `b.Loop()` + allocs (Gap 5) | Small | none |
| P2 | Systematic `t.Parallel()` in integration tests (Gap 8) | Trivial | Gap 1 |
| P2 | Restart operation state machine + tests (Gap 4) | Medium | gated on product decision |
| P3 | Extension conformance harness (Gap 7) | Large | gated on public extension SDK |

---

## Key File References

### looper (current state)
- `/Users/pedronauck/Dev/compozy/looper/internal/daemon/boot_integration_test.go` — closest thing to an integration harness.
- `/Users/pedronauck/Dev/compozy/looper/internal/daemon/run_manager_test.go:1796` — `newRunManagerTestEnv` shared harness.
- `/Users/pedronauck/Dev/compozy/looper/internal/daemon/run_manager_bench_test.go` — only benchmark file.
- `/Users/pedronauck/Dev/compozy/looper/internal/core/run/executor/execution_acp_integration_test.go:1169` — `TestRunACPHelperProcess` ACP mock helper.
- `/Users/pedronauck/Dev/compozy/looper/internal/core/run/executor/execution_acp_integration_test.go:1329` — `installACPHelperOnPath`.
- `/Users/pedronauck/Dev/compozy/looper/test/helpers_test.go` — 17-line utility.

### AGH (reference)
- `/Users/pedronauck/dev/compozy/agh/internal/testutil/e2e/runtime_harness.go` — the runtime harness (1,333 LOC).
- `/Users/pedronauck/dev/compozy/agh/internal/testutil/e2e/artifacts.go` — artifact manifest.
- `/Users/pedronauck/dev/compozy/agh/internal/testutil/e2e/config_seed.go` — TOML config seeding.
- `/Users/pedronauck/dev/compozy/agh/internal/testutil/e2e/mock_agents.go` — mock-agent registration.
- `/Users/pedronauck/dev/compozy/agh/internal/testutil/e2e/transport_parity.go` — HTTP/UDS parity.
- `/Users/pedronauck/dev/compozy/agh/internal/testutil/acpmock/fixture.go` — ACP fixture schema.
- `/Users/pedronauck/dev/compozy/agh/internal/testutil/acpmock/driver_binary.go` — driver binary builder.
- `/Users/pedronauck/dev/compozy/agh/internal/testutil/acpmock/registration.go` — `Register(homePaths, opts)`.
- `/Users/pedronauck/dev/compozy/agh/internal/testutil/acpmock/diagnostics.go` — `ReadDiagnostics`.
- `/Users/pedronauck/dev/compozy/agh/internal/e2elane/lanes.go` — lane definitions.
- `/Users/pedronauck/dev/compozy/agh/internal/testutil/testutil.go` — `Context(t)`, `FreeTCPPort(t)`.
- `/Users/pedronauck/dev/compozy/agh/internal/daemon/daemon_acpmock_faults_integration_test.go` — canonical fault tests.
- `/Users/pedronauck/dev/compozy/agh/internal/daemon/daemon_acpmock_helpers_integration_test.go` — `createFixtureBackedSession`, `mockFixturePath`.
- `/Users/pedronauck/dev/compozy/agh/internal/daemon/perf_bench_test.go` — benchmark patterns.
- `/Users/pedronauck/dev/compozy/agh/internal/daemon/restart_integration_test.go` — restart state machine integration.
