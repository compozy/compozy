# Analysis: lifecycle-processes

## Scope

- **Slice question:** Which Go 1.21–1.26 APIs and lifecycle refactors materially improve goroutine ownership, subprocess safety, timer determinism, maintainability, and resource cleanup in the runtime/process slice without changing externally observable behavior?
- **Primary sources:** the supplied Go-modernization review; all production Go files under `internal/acp`, `internal/admission`, `internal/agentidentity`, `internal/clientstate`, `internal/coordinator`, `internal/daemon`, `internal/diagnosticcontract`, `internal/diagnostics`, `internal/doctor`, `internal/e2elane`, `internal/heartbeat`, `internal/procutil`, `internal/providerauth`, `internal/providerenv`, `internal/providers`, `internal/retry`, `internal/sandbox`, `internal/scheduler`, `internal/session`, `internal/sessions`, `internal/subprocess`, `internal/support`, `internal/toolruntime`, `internal/update`, and `internal/version`; relevant owning tests in those trees.
- **Sources read in full:** the supplied review; the project/runtime instructions; and a 25-file deep working set comprising `internal/retry/backoff.go`, `internal/retry/retry.go`, `internal/retry/retry_test.go`, `internal/procutil/procutil.go`, `internal/procutil/process_group_unix.go`, `internal/procutil/process_group_windows.go`, `internal/procutil/detached.go`, `internal/procutil/parent_watch_unix.go`, `internal/procutil/parent_watch_unix_test.go`, `internal/subprocess/process.go`, `internal/subprocess/process_lifecycle.go`, `internal/subprocess/process_shutdown.go`, `internal/subprocess/transport.go`, `internal/support/service.go`, `internal/support/service_builder.go`, `internal/support/service_test.go`, `internal/session/manager.go`, `internal/session/manager_lifecycle.go`, `internal/session/manager_process_watchers.go`, `internal/session/manager_start_run.go`, `internal/session/compaction.go`, `internal/session/compaction_lifecycle.go`, `internal/sandbox/daytona/tar.go`, `internal/sandbox/daytona/tar_test.go`, and `internal/daemon/task_role_runtime.go`. Additional focused files were read around every semantic search hit.
- **Sources sampled:** every scoped production file was surveyed mechanically for goroutine launches, `sync.WaitGroup`, `sync.Once`, process creation/signaling, timers, network dialing, JSON omission tags, string splitting, materialized lists, benchmark loops, and the review APIs. The 286 scoped test files were sampled by invariant and owning suite; integration/E2E tests were inspected where real processes, clocks, or artifacts affect the recommendation.
- **Total candidates:** 1,117 scoped Go files: 831 production files and 286 test files across 30 packages/directories. The production surface is approximately 136,059 lines. `internal/daemon` accounts for 449 production files, `internal/session` for 182, and `internal/acp` for 50, so package-wide conclusions were based on a full mechanical survey plus targeted deep reads rather than a small-file sample.
- **Execution boundary:** this was analysis-only. No production code, tests, generated artifacts, gates, or external systems were mutated or executed.

## Overview

The review is directionally useful, but the correct outcome for this slice is **selective modernization**, not blanket adoption. The highest-value findings are lifecycle ownership defects that the review's API census did not identify:

1. **Support-bundle work has no daemon-owned lifecycle.** `support.Service.Create` detaches from request cancellation and launches a raw goroutine, but `Service` has no admission/drain state, `WaitGroup`, or `Shutdown`. The daemon constructs the service but does not retain it in `shutdownTargets`. An accepted bundle can therefore continue using snapshot callbacks or writing artifacts after daemon resources begin closing. This is the clearest P0 resource-lifecycle defect.
2. **Inbound subprocess JSON-RPC handlers are untracked.** The transport reader launches one raw goroutine per request. Process teardown cancels the lifecycle context and waits for the reader, but not for those handlers before closing `Process.Done`. A cooperative handler will normally exit, yet the ownership contract is not enforced; a slow or context-ignoring handler can outlive the process lifecycle.
3. **Doctor timeouts can abandon non-cooperative probes.** `Runner.runProbe` returns on `probeCtx.Done` while the probe goroutine remains live if the probe ignores context. An in-process goroutine cannot be force-killed, so `WaitGroup.Go` or `synctest` alone cannot repair this. The probe contract or isolation model needs an explicit decision.

The existing lifecycle foundation is otherwise stronger than the review implies:

- The session manager separately owns synchronous start calls, compactions, prompt drains, finalizations, and process watchers, and joins them during shutdown.
- Managed subprocesses expose `Done`/`Wait`, cancel their lifecycle, reap the root process, and centralize process-tree termination in `procutil`.
- Unix process groups and Windows kill-on-close job objects are already modeled explicitly. A single-process `os.Process.WithHandle` conversion would not replace that group ownership.
- Scheduler tests already use an injected `clockwork.Clock`; retry tests inject both sleep and randomness. Broad `testing/synctest` conversion in those packages would duplicate existing deterministic seams.

Recommended priority:

| Priority | Outcome | Scope |
| --- | --- | --- |
| P0 | Give support-bundle operations daemon-owned admission, cancellation, joining, and shutdown ordering | `internal/support`, daemon boot/shutdown composition |
| P1 | Define and enforce inbound subprocess-handler ownership; decide the non-cooperative doctor-probe contract | `internal/subprocess`, `internal/doctor` |
| P1 | Replace Daytona tar extraction's check-then-open path boundary with `os.OpenRoot` operations | `internal/sandbox/daytona` |
| P2 | Apply `WaitGroup.Go`, `OnceFunc`/`OnceValue`, and a narrow `synctest` pilot where semantics are exact | session, daemon task-role runtime, clientstate, Daytona sessions, procutil, clarify tests |
| P3 | Finish mechanical standard-library adoption (`errors.AsType`, `b.Loop`, `math/rand/v2`, internal iterator utilities, selected string sequences) | scoped packages |
| Defer | Flight recorder, test artifact metadata, HTTP cross-origin protection, and any public iterator redesign | needs security/config/test-harness or out-of-slice design |

Maintainability is currently within the 500-line production cap, but several files leave little room for additions: `internal/session/manager_busy_input.go` (490), `internal/daemon/native_loop_tools.go` (485), `internal/daemon/task_event_bridge_notifier.go` (480), `internal/session/manager_clear.go` (478), `internal/daemon/spawn_reaper.go` (475), `internal/daemon/harness_observability.go` (475), `internal/daemon/native_bundle_resource_tools.go` (471), `internal/subprocess/transport.go` (454), `internal/subprocess/handshake.go` (448), `internal/diagnosticcontract/diagnostics.go` (446), `internal/session/manager.go` (443), and `internal/scheduler/scheduler.go` (440). Lifecycle additions should land in named lifecycle files rather than growing these files through the cap.

## Mechanisms / Patterns

### Lifecycle and ownership model

The recurring safe pattern is:

1. The owner closes admission under the same synchronization boundary that orders `Go` before `Wait`.
2. Every accepted background activity is associated with an owner context and an owner join primitive.
3. Shutdown closes admission, cancels the owner context, waits for all accepted work, and only then closes downstream resources.
4. Request cancellation and owner cancellation are distinct when the product contract says accepted work survives the originating request.

The session manager already follows this model in most places. `startWG` is deliberately different: it counts synchronous, request-owned `Start` calls rather than goroutines spawned by the manager. Likewise, `clarifyBridge.waiters` counts callers blocked inside `Ask`; converting either to `WaitGroup.Go` would change ownership and is incorrect. In contrast, compaction, process-watch, and task-role activation sites perform an actual `Add(1)` immediately followed by a goroutine and are exact `WaitGroup.Go` candidates once launch remains ordered against shutdown.

Process ownership has two layers:

- **Live root handle:** `exec.Cmd.Process` or `os.Process` is reaped through `Wait`; `Done` closes only after lifecycle cleanup.
- **Process tree / recovered process:** Unix process-group IDs and Windows job objects terminate descendants; durable recovery uses PID plus observed start-time evidence to reduce PID-reuse risk.

These are not interchangeable. `os.Process.WithHandle` may improve a narrow single-process handle path, but it cannot represent a Unix negative-PGID signal or a Windows job object and does not eliminate the durable-recovery need to validate PID/start-time identity.

### Exhaustive review decision matrix

Decision vocabulary: **IMPLEMENT** means a behavior-preserving scoped change is justified now; **TARGETED** means only the named cases are justified; **DEFER** requires a separate contract/security/harness decision; **REJECT** means the proposed use is mismatched in this slice; **ADOPTED** means no meaningful residual work was found.

| Review item | Scoped evidence | Decision | Recommended action / reason |
| --- | --- | --- | --- |
| `errors.AsType[T]` | 15 residual production `errors.As` calls in 13 files, all typed pointer or diagnostic-carrier extraction | **IMPLEMENT** | Convert the residual calls mechanically, retaining explicit nil checks where the current code guards typed nils. Interface extraction in `diagnostics.ItemFromError` is also expressible as `AsType[diagnosticItemCarrier]`. No new tests should freeze syntax; existing behavior suites own the invariant. |
| `testing.B.Loop` | Three residual index-free `b.N` loops: one in `prompt_skills_test.go`, two in `loop_watch_events_observer_bench_test.go` | **IMPLEMENT** | Replace with `for b.Loop()`. Preserve setup, `ResetTimer`, `StopTimer`, and reported metrics exactly. |
| JSON `,omitzero` | No scoped value `time.Time`/struct field currently combines zero-value ambiguity with `omitempty`; relevant timestamps are pointers. The one value `time.Duration` omission already behaves correctly at numeric zero. | **ADOPTED / no candidate** | Do not churn tags. Pointer timestamps intentionally distinguish absent from present; `omitzero` would add no behavior or clarity here. |
| `os.OpenRoot` | No scoped use. Daytona tar extraction validates parent symlinks and then separately calls `os.OpenFile`, leaving a check/open race. ACP file host also resolves/authorizes absolute paths before separate `os.ReadFile`/`os.WriteFile`, but supports multiple roots and terminal CWDs. | **TARGETED IMPLEMENT** | Convert Daytona extraction first to one root handle plus relative operations. Defer ACP host conversion until policy returns `(root, relativePath)` capabilities and root handles have an explicit close lifecycle; `exec.Cmd.Dir` cannot be made handle-relative by `OpenRoot`. |
| `strings.SplitSeq` / `FieldsSeq` / `Lines` | `update` and Daytona tar already use `SplitSeq`. Remaining splits include both iteration-only paths and paths requiring indexing, reverse traversal, truncation, or joining. | **TARGETED IMPLEMENT** | Convert only iteration-only scans such as heartbeat body/frontmatter line scans and daemon process-list parsing. Keep slices for line slicing, reverse error search, path suffix reconstruction, or word truncation. Count indices explicitly where diagnostics require exact line numbers. |
| `sync.WaitGroup.Go` | Manual production `Add(1)` sites are limited to session start tracking, compaction, process watchers, clarify callers, and task-role activation. Only three correspond directly to owner-spawned goroutines. | **TARGETED IMPLEMENT** | Convert compaction, process watchers, and task-role activation. Use `Go` for the new support owner and, after contract resolution, transport handlers. Do not convert `startWG` or clarify waiters because they count externally executing callers. |
| `range` over integer | Already present in scoped code and tests; no meaningful residual C-style loop whose index is only a counter was found outside the three benchmarks above. | **ADOPTED** | Leave behavior-sensitive index loops intact. `b.Loop` owns benchmark iteration. |
| `slices`, `maps`, built-in `min`/`max` | Strong existing use. Some deterministic key snapshots still manually allocate, collect, and sort. | **TARGETED IMPLEMENT** | Low-priority cleanup: `slices.Sorted(maps.Keys(...))` in doctor and heartbeat, and possibly `slices.Collect(maps.Values(...))` for internal snapshots. Preserve stable ordering and release locks before expensive projection. |
| `testing/synctest` | Zero use, but the review's proposed retry/scheduler targets already inject sleep/randomness or a fake clock. Pure wall-clock tests remain in parent-watch and clarification timeout paths. Process/E2E tests depend on OS clocks and subprocesses. | **TARGETED IMPLEMENT** | Pilot `synctest` in `procutil.TestWatchParentExit` and pure clarification timeout/cancel subtests. Do not wrap subprocess, SSH, Daytona, or daemon E2E tests; fake time does not virtualize external processes or network I/O. Do not replace scheduler's established fake-clock seam. |
| Public `iter.Seq` / range-over-function APIs | Scoped `List`, `ListAll`, and `ListPage` methods are stable snapshot or pagination contracts. Some are transport-facing; others deliberately release locks before callers iterate. | **REJECT public conversion; IMPLEMENT internal utilities** | Keep public slices. Use iterator-producing `maps.Keys`/`maps.Values` only as an internal collection mechanism where it removes boilerplate without leaking lock/transaction lifetime. SQLite event streaming candidates are outside this slice and require their own ownership analysis. |
| `os.Process.WithHandle` / `os.ErrNoHandle` | Started children already carry OS-owned process handles through `exec.Cmd`/`os.Process`. Recovered tool processes intentionally store PID plus start time; process-tree signaling uses PGIDs/job objects. | **REJECT blanket migration** | Preserve centralized group operations. Consider only a separate Windows-specific experiment where an actual stable handle can be retained for the same lifetime as the process record. Treat `ErrNoHandle` explicitly if such an API is introduced; do not manufacture handles from bare durable PIDs and assume identity safety. |
| `sync.OnceValue`, `OnceValues`, `OnceFunc` | 21 scoped production `sync.Once` fields/locals. Most close channels or guard visible lifecycle state. Exact value/function cases exist: clientstate `Close`, Daytona close methods, version restore, session unsubscribe/release callbacks. Update install detection depends on the first caller's context. | **TARGETED IMPLEMENT** | Use `OnceFunc` for local one-shot callbacks and `OnceValue[error]` for close functions whose error must be stable across calls. Do not convert stateful channel closure or context-dependent install detection. |
| `math/rand/v2` | One scoped production v1 use: the default `RandFloat64` for retry jitter. Tests already inject deterministic random functions. | **IMPLEMENT** | Switch the production default to `math/rand/v2.Float64`; preserve the `RandFloat64` seam. Do not claim a measured hot-path speedup: the scoped evidence shows no benchmark or contention profile, and the global-lock rationale in the review is unproven here. |
| `cmp.Or` | First-non-empty helpers in this slice trim whitespace before deciding. Raw `cmp.Or(" ", fallback)` would select whitespace and change normalization behavior. Config precedence itself is outside most of this slice. | **REJECT raw substitution** | Consolidate the duplicate trim-aware daemon helpers into one named normalization helper. `cmp.Or` is acceptable only after normalization, where it is usually cosmetic and not worth another import. |
| `T.ArtifactDir`, `T.Attr`, `T.Output` | E2E/process tests create isolated homes and may contain provider output or secrets. The current QA evidence contract writes durable evidence under `docs/qa`; test-runner artifact retention is a different lifecycle. | **DEFER** | Design one harness-level pilot after deciding retention, redaction, CI upload, and gate-parser behavior. Do not scatter calls through unit tests or use `ArtifactDir` as the runtime home. `T.Output` must receive only already-redacted output. |
| `net/http.CrossOriginProtection` | HTTP handler/middleware ownership is under `internal/api/httpapi`, outside the slice. The daemon file inspected here only composes and starts the server. | **DEFER / hand off** | Audit mutation routes, origin policy, loopback/DNS-rebinding assumptions, WebSocket/SSE behavior, and UDS alternatives in the HTTP transport slice. This analysis cannot safely assert that protection is absent or insert middleware at composition level. |
| `runtime/trace.NewFlightRecorder` | Diagnostics currently owns redacted diagnostic values; support owns capped/redacted archives; daemon owns runtime lifecycle. No trace owner, trigger, config, or artifact contract exists. A runtime trace is global and binary. | **DEFER** | Treat as a new capability: daemon-owned recorder, explicit config/defaults, dump trigger, bounded retention, shutdown, operator authorization, manifest versioning, and a security decision for global cross-workspace data. It does not belong as a utility inside `internal/diagnostics`. |
| `net.Dialer.DialUnix` / `DialTCP` | Only one concrete scoped `net.Dialer.DialContext`: Daytona SSH uses a hostname string and injected generic `network,address` dialer. UDS dialers cited by the review are outside this slice. | **REJECT in-scope conversion** | `DialTCP` would require pre-resolving to `*net.TCPAddr`, changing resolver/Happy-Eyeballs behavior and the injectable test seam. Hand UDS call sites to the UDS transport slice. |
| `bytes.Buffer.Peek` | Subprocess JSON-RPC is newline-delimited and bounded by `bufio.Scanner`; scoped `bytes.Buffer` uses collect full command output or stderr, not incremental framing. | **REJECT** | `Buffer.Peek` solves no current framing problem. Replacing the scanner would risk changing the max-frame and newline contract. |
| `unique.Handle` | Session/workspace IDs in this slice are high-cardinality keys with little repetition; event kind constants are already static. The event store cited by the review is outside the slice. | **REJECT for IDs; hand off event-store study** | Interning session/workspace IDs adds handle/indirection cost with little sharing. Evaluate repetitive event types only with heap/profile evidence in the event-store owner, including handle lifetime and wire-conversion cost. |

### Package coverage

Every scoped package/direct directory was mechanically surveyed. “No action” means no behavior-preserving use with material value was found, not that the package was skipped.

| Package / directory | Coverage result |
| --- | --- |
| `internal/acp` | Convert three typed error extractions. Keep scanner/terminal lifecycle semantics. Daytona-style `OpenRoot` reasoning exposes a similar file-host TOCTOU, but multi-root authorization and terminal CWD require a separate capability-based design. No `Buffer.Peek` fit. |
| `internal/admission` | Atomic daemon drain gate is already minimal and race-safe; no review API adds value. It is the pattern the support service should emulate. |
| `internal/agentidentity` | One safe `errors.AsType[*Error]` conversion. Identity/workspace strings are high-cardinality and are poor `unique.Handle` candidates. |
| `internal/clientstate` | Per-workspace sequencers have explicit stop/done joining. `Engine.Close` is an exact `OnceValue[error]` candidate because concurrent close callers should observe one completion and one cached error. Keep `List` as a sorted cloned snapshot. |
| `internal/coordinator` | No lifecycle gap found. Trim-aware normalization makes `cmp.Or` unsuitable. Prompt construction and metadata parsing do not benefit from iterators materially. |
| `internal/daemon` | Convert task-role activation to `WaitGroup.Go`, three remaining benchmark loops to `b.Loop`, typed error extractions, and duplicate trim-aware helpers. Add support service to boot/shutdown ownership. Defer flight recorder to a designed daemon capability. |
| `internal/diagnosticcontract` | Pure diagnostic contract, already 446 lines; do not place runtime tracing or generic lifecycle code here. No scoped modernization target. |
| `internal/diagnostics` | Convert diagnostic-carrier extraction to `AsType`. Keep redaction boundary pure. Flight-recorder ownership belongs to daemon/support, not this value package. |
| `internal/doctor` | Deterministic registry can use `maps.Keys` + `slices.Sorted`. Timeout goroutine can leak for a non-cooperative probe; requires contract/isolation design, not `synctest`. |
| `internal/e2elane` | Static lane planning only. `T.Attr`/artifact integration belongs in the shared Go test/QA harness and gate tooling, not lane constants. |
| `internal/heartbeat` | Iteration-only body/frontmatter scans and manual sorted keys are safe low-priority sequence/iterator utility targets. Preserve exact diagnostic line/column calculations. |
| `internal/procutil` | Parent watcher is the cleanest `synctest` pilot. Retain PID/start-time validation and process-group APIs; reject blanket `WithHandle`. Detached children own one `Wait` and `Done`. |
| `internal/providerauth` | Pure native-CLI resolution; no lifecycle or standard-library gap found. |
| `internal/providerenv` | Pure environment construction/symlink handling; no scoped candidate. |
| `internal/providers` | Provider commands are context-bounded and capture complete redacted output; `bytes.Buffer.Peek` is irrelevant. No scoped rand or dial migration. |
| `internal/retry` | Switch the one production default to `rand/v2`. Do not add `synctest`: tests already inject `Sleep` and `RandFloat64`, which is more direct and deterministic. |
| `internal/sandbox` | Interfaces/registry have no direct migration. Security and lifecycle work resides in implementations below. |
| `internal/sandbox/daytona` | Use `os.OpenRoot` for tar extraction; use `OnceValue[error]` where close errors are currently lost after the first call; keep generic hostname SSH dial. Real network/process tests must remain real-time. |
| `internal/sandbox/daytona/cmd/compozy-daytona-sidecar` | Stateful `sync.Once` stops a server/process lifecycle and should remain explicit. No safe typed-dial or synctest conversion. |
| `internal/sandbox/local` | Thin provider implementation; no action. |
| `internal/sandbox/providertest` | Provider conformance helper; no production lifecycle owner to migrate. |
| `internal/scheduler` | Already uses `WaitGroup.Go` and injected `clockwork.Clock`; preserve that seam. Public snapshot inputs/outputs remain slices. No synctest conversion needed. |
| `internal/session` | Convert compaction and process watcher launches to `WaitGroup.Go`; keep synchronous `startWG`. Use `OnceFunc` for returned release/unsubscribe callbacks. Preserve joined process shutdown and stable list snapshots. |
| `internal/session/inputqueue` | Bounded queue implementation has no review-API fit. |
| `internal/sessions/ledger` | Single materializer source; no relevant lifecycle or API candidate. |
| `internal/subprocess` | Keep scanner framing and process-group lifecycle. Add explicit inbound-handler ownership after deciding non-cooperative semantics. `WithHandle` and `Buffer.Peek` do not replace current contracts. |
| `internal/support` | P0: detached bundle goroutine has no admission, owner context, join, or shutdown. Add a service lifecycle and expose it to daemon shutdown. |
| `internal/toolruntime` | Durable records validate PID/start-time and signal process groups. Keep slice-returning store contracts. A process handle cannot be serialized across daemon restart. |
| `internal/update` | `SplitSeq` is already used for checksum catalogs. Do not convert install detection to `OnceValue`: it depends on the first caller context. |
| `internal/version` | Returned restore closure is an exact `sync.OnceFunc` candidate; existing serialization and version restoration semantics stay unchanged. |

## Relevant Sources

The following source groups determine the recommendations:

- `internal/support/service.go` and `internal/support/service_builder.go` define detached support operations and request-deadline preservation. `internal/daemon/boot_extension_publishers.go`, `internal/daemon/daemon_shutdown_targets.go`, `internal/daemon/daemon_shutdown_runtime.go`, and `internal/daemon/daemon_shutdown_resources.go` show that the service is composed but not detached/joined during shutdown.
- `internal/session/manager.go`, `internal/session/manager_lifecycle.go`, `internal/session/manager_process_watchers.go`, `internal/session/compaction.go`, `internal/session/compaction_lifecycle.go`, and `internal/session/manager_start_run.go` distinguish manager-owned goroutines from synchronous caller work. This distinction controls where `WaitGroup.Go` is correct.
- `internal/daemon/task_role_runtime.go` and `internal/daemon/task_role_prompt.go` show the shutdown ordering lock around manual `Add`/`Wait` and provide an existing lifecycle suite.
- `internal/subprocess/process.go`, `internal/subprocess/process_lifecycle.go`, `internal/subprocess/process_shutdown.go`, and `internal/subprocess/transport.go` define root-process reaping, lifecycle cancellation, transport reader ownership, and the untracked handler launch.
- `internal/procutil/process_group_unix.go`, `internal/procutil/process_group_windows.go`, `internal/procutil/process_started_at.go`, `internal/procutil/detached.go`, and `internal/toolruntime/registry.go` define process-tree and recovered-process identity. They are the reason `os.Process.WithHandle` is not a drop-in modernization.
- `internal/doctor/doctor.go` demonstrates the fundamental timeout/non-cooperative-goroutine mismatch.
- `internal/procutil/parent_watch_unix.go`, `internal/procutil/parent_watch_unix_test.go`, `internal/daemon/clarify_bridge.go`, and `internal/daemon/clarify_bridge_test.go` are pure timer/channel candidates for a narrow `synctest` pilot.
- `internal/scheduler/scheduler.go`, `internal/scheduler/types.go`, and the scheduler lifecycle tests demonstrate an existing fake-clock seam and existing `WaitGroup.Go` use.
- `internal/retry/backoff.go` and `internal/retry/retry_test.go` demonstrate an existing deterministic `Sleep`/randomness seam and the only production v1 random default in this slice.
- `internal/sandbox/daytona/tar.go` and `internal/sandbox/daytona/tar_test.go` define the archive path security boundary and its canonical tests. `internal/acp/tool_host.go` and `internal/acp/permission.go` expose the broader multi-root follow-up.
- `internal/clientstate/service.go`, `internal/clientstate/concurrency_lifecycle_test.go`, `internal/sandbox/daytona/sidecar_session.go`, `internal/sandbox/daytona/sidecar_transport_cleanup_test.go`, `internal/sandbox/daytona/ssh_session.go`, `internal/sandbox/daytona/ssh_test.go`, and `internal/version/version.go` contain the exact `OnceValue`/`OnceFunc` candidates.
- `internal/daemon/hook_agent_events.go`, `internal/daemon/memory_session_ledger_parse.go`, `internal/daemon/loop_watch_events_observer.go`, `internal/daemon/loop_managed_prompt.go`, and `internal/daemon/role_resolver.go` repeat the same trim-aware first-non-empty algorithm and should share one daemon-local normalization helper instead of importing `cmp`.

## Transferable Patterns

### 1. Add a lifecycle boundary to support-bundle operations (P0)

- **Invariant:** once `Create` accepts an operation, the operation reaches a terminal store state, cannot outlive service shutdown, and never accesses snapshot/database/logger dependencies after the daemon closes them. Request cancellation may remain detached while the daemon is active.
- **Owning layer:** `internal/support.Service`; daemon is only the composition/shutdown owner.
- **Canonical suite:** extend `internal/support/service_test.go::TestServiceCreate` for admission, cancellation, terminal status, and concurrent shutdown. Extend `internal/daemon/daemon_test.go::TestShutdownTearsDownInRequiredOrder` for ordering before persistent stores/logger close. Do not create a duplicate standalone test.
- **Implementation shape:** add service-level admission/closing state, an owner context/cancel, and a `sync.WaitGroup`. Register work with `wg.Go` under the same lock that closes admission. Add `Shutdown(context.Context) error`; during daemon drain, reject new creates, cancel accepted operations (or wait according to the resolved contract), join them, then allow persistent-resource shutdown. Ensure canceled `Build` marks the operation failed/terminal rather than leaving `running` forever. Capture the service in `bootState`/`Daemon`/`shutdownTargets` rather than only the dependency interface.
- **Why first:** later support/trace/artifact work otherwise compounds an ownerless background-write path.

### 2. Make inbound subprocess handler ownership explicit (P1, contract decision required)

- **Invariant:** `Process.Done` does not close while an accepted inbound handler still owns process transport state; no new handler starts after lifecycle drain; handlers observe lifecycle cancellation.
- **Owning layer:** `internal/subprocess.transport`, joined by `Process.waitForExit`.
- **Canonical suite:** extend `internal/subprocess/process_shutdown_test.go::TestProcessShutdownCancellationContract` and the inbound-routing case in `internal/subprocess/process_test.go::TestHandleMethodRoutesInboundRequests` with a blocking cooperative handler. The test must prove cancellation and join, not merely count goroutines.
- **Implementation shape:** add a handler admission lock/closing flag and `handlerWG`; launch through `handlerWG.Go`; after process exit, close handler admission, cancel `lifecycleCtx`, join handlers, then finalize transport/process state. Decide whether a non-cooperative handler is a programmer-contract violation that may block `Done`, or whether every handler gets an explicit timeout. Do not add a detached timeout goroutine that recreates the leak.

### 3. Resolve the doctor-probe timeout contract (P1 design gate)

- **Invariant:** a timed-out probe cannot silently accumulate a permanent goroutine across repeated doctor runs.
- **Owning layer:** `internal/doctor.Runner` for built-ins; extension/process boundary if third-party probes are allowed.
- **Canonical suite:** extend `internal/doctor/doctor_test.go::TestRunner`, which already owns timeout classification.
- **Implementation shape:** audit all built-in probes for prompt context return. If probes are trusted in-process code, state and test the cooperative-context contract and expose a straggler diagnostic/metric. If untrusted extension probes are possible, execute them through an isolatable process boundary; Go cannot safely terminate an arbitrary in-process goroutine. `WaitGroup.Go` and `synctest` are not substitutes.

### 4. Move Daytona extraction to root-relative filesystem capabilities (P1)

- **Invariant:** no archive entry can read, create, replace, or traverse outside the extraction root, including while symlinks are changed concurrently; safe nested files and permitted in-root symlinks retain current round-trip behavior.
- **Owning layer:** `internal/sandbox/daytona` archive extraction.
- **Canonical suite:** extend `internal/sandbox/daytona/tar_test.go::TestExtractTarRejectsUnsafeEntries`, `TestExtractTarRejectsExistingSymlinkEscape`, and `TestWriteAndExtractTarRoundTripWithSymlinkAndExclusions` in place.
- **Implementation shape:** create/evaluate the root once, open one `os.Root`, convert validated archive names to relative paths, and use root-relative `MkdirAll`, `OpenFile`, `Lstat`/remove/symlink operations available in the pinned Go API. Close the root and join close errors. Preserve mode, truncation, byte counts, and symlink policy. Delete `ensureSafeParent` once the capability boundary makes it obsolete; do not retain both mechanisms.
- **Follow-up:** ACP file operations should later adopt the same capability model, but only after the permission policy can identify the selected allowed root and relative path. Do not force terminal working-directory resolution through a file-only API.

### 5. Finish exact `WaitGroup.Go` conversions (P2)

- **Invariant:** shutdown cannot begin `Wait` before an accepted owner-spawned goroutine has been registered; every registered task returns on owner cancellation.
- **Owning layers and canonical suites:** session compaction — `internal/session/manager_hooks_test.go` subtest “Should join a canceled compaction before closing the recorder”; session process watcher — `internal/session/manager_test.go` subtest “Should cancel and join live process watchers during shutdown”; daemon task-role activation — `internal/daemon/task_role_runtime_test.go` lifecycle subtests at lines 100 and 147.
- **Implementation shape:** call `compactionWG.Go` directly at the current Add/go site. Refactor process watcher startup so registration and launch happen under `processWatchMu`, rather than returning a context and launching later. Refactor task-role activation so `wg.Go` occurs while `lifecycleMu` still orders it before `stopping=true`/`Wait`.
- **Explicit non-targets:** keep session `startWG` and clarify `waiters` manual because they track synchronous callers, not spawned tasks.

### 6. Use `OnceFunc`/`OnceValue` only where it strengthens the contract (P2)

- **Invariant:** one-shot cleanup runs once, concurrent callers wait for the same completion, and every caller observes the same error where cleanup returns an error.
- **Owning layers / canonical suites:** clientstate close — `internal/clientstate/concurrency_lifecycle_test.go`; Daytona sidecar close — `internal/sandbox/daytona/sidecar_transport_cleanup_test.go::TestSidecarSessionCleanupContract`; SSH close — `internal/sandbox/daytona/ssh_test.go`; version restore — `internal/version/version_test.go`; session unsubscribe/release — existing catalog/stream/start suites.
- **Implementation shape:** model `Engine.Close` as an initialized `OnceValue[error]` closure and remove redundant `closeDone`/`closeErr` state if the resulting lifecycle remains clear. Use cached error closures for `sidecarSession.CloseWrite`, `sidecarSession.closeLocalResources`, and `sshSession.Close`; the current callback-local `err` means only the first caller sees a close error. Use `OnceFunc` for version restore, catalog/event unsubscribe, and launch-commit release callbacks.
- **Explicit non-targets:** channel-close sentinels, health monitor state, query-store start/stop, prompt supervisor stop, process wait, network-ready state, and update install detection. In particular, `update.detectInstall(ctx)` cannot become a no-argument `OnceValue` without discarding first-caller cancellation semantics.

### 7. Pilot `testing/synctest` where the whole behavior is in-process (P2)

- **Invariant:** parent reparent detection fires exactly once after one virtual poll and returns immediately on cancellation; clarification timeout produces the exact fallback/canceled terminal events without wall-clock sleeps.
- **Owning layer / canonical suite:** `internal/procutil/parent_watch_unix_test.go::TestWatchParentExit`; timeout/cancel subtests in `internal/daemon/clarify_bridge_test.go::TestClarifyBridgeLifecycle`.
- **Implementation shape:** put the component and helper goroutines inside one synctest bubble, use virtual time and `synctest.Wait` for quiescence, and retain a real deadlock guard only outside the bubble if required by the test framework. Do not use arbitrary sleeps to “let the goroutine start.”
- **Explicit non-targets:** retry (already injects `Sleep` and randomness), scheduler (already injects `clockwork.Clock`), and tests that wait for OS processes, SSH, HTTP servers, files watched by the kernel, or Daytona services.

### 8. Complete mechanical standard-library migration as one low-risk batch (P3)

- **Invariant:** typed error classification, benchmark workload, retry delay bounds, stable ordering, and diagnostic line numbers remain byte-for-byte/structurally equivalent.
- **Owning layers / canonical suites:** error extraction — existing `acp`, `agentidentity`, `diagnostics`, session, daemon extension/role/window-manager suites; benchmarks — the three owning benchmark files; retry — `internal/retry/retry_test.go`; heartbeat sequences/keys — `internal/heartbeat/heartbeat_test.go::TestHeartbeatRejectsAuthorityClaims` and `TestParseHeartbeatPolicy`; doctor registry — `internal/doctor/doctor_test.go::TestRegistry`; daemon process parsing — `internal/daemon/daemon_test.go::TestListProcessesAndSignalProcess`.
- **Implementation shape:** migrate all 15 scoped `errors.As` calls; convert the three residual benchmarks to `b.Loop`; switch only the retry production default to `math/rand/v2`; use `SplitSeq` only for forward iteration; use `slices.Sorted(maps.Keys(...))` for deterministic key collection. No new syntax-only tests.

### 9. Consolidate trim-aware defaults without `cmp.Or` (P3)

- **Invariant:** whitespace-only values are skipped and the first trimmed non-empty value wins.
- **Owning layer:** daemon-local string normalization helper in a small named file, not a domain-specific near-cap file and not a cross-package generic utility.
- **Canonical suite:** reuse the hook agent-event, memory session-ledger parse, loop watch-events, managed-loop goal, and role resolver suites that already exercise each call site.
- **Implementation shape:** move the implementation now named `firstNonEmpty` to a neutral daemon helper and delete `firstNonEmptyString`, `firstNonEmptyWatchEventsValue`, `firstNonEmptyManagedGoal`, and `firstRoleValue`. Keep `firstSchedulerWakeValue`'s explicit fallback behavior, optionally delegating its normalized lookup to the shared helper.

### Migration ordering

1. Resolve Open Questions 1 and 2 (support shutdown policy and subprocess non-cooperative handler semantics).
2. Implement and verify support ownership, then subprocess handler ownership; both affect shutdown proofs.
3. Implement the Daytona `OpenRoot` boundary and run its existing security/round-trip suite.
4. Apply exact `WaitGroup.Go` and `OnceFunc`/`OnceValue` conversions, with existing lifecycle suites proving behavior.
5. Pilot `synctest` in parent-watch and clarification only; measure test duration/flakiness before expanding.
6. Land one mechanical standard-library batch (`AsType`, `b.Loop`, `rand/v2`, sequence/iterator utilities, daemon helper consolidation).
7. Treat flight recording, test artifacts, HTTP cross-origin protection, ACP multi-root handles, and event-store interning as separate specs or slice handoffs, not as opportunistic edits.

### Compozy Impact Audit

- **Native tools:** the recommended lifecycle/mechanical batch changes no `compozy__*` IDs, toolsets, descriptors, schemas, digests, risk flags, availability diagnostics, or capability gates. Support-bundle create/download behavior must retain its existing API result shapes; a new drain error, if exposed, must be mapped through the existing API/CLI error contract. A future trace artifact would change the support manifest and is therefore not part of this batch.
- **Extensibility and hooks:** no changes to extensions, hooks, skills/capabilities, tools/resources, bundles, registries, bridge SDKs, or MCP sidecars for the mechanical batch. Support shutdown adds only owner lifecycle. Flight recording would require a daemon registry/owner, support-bundle integration, explicit config lifecycle, and extension-safe authorization; that is why it is deferred.
- **Workspace data isolation:** all immediate changes operate on process/global lifecycle and introduce no new persisted workspace/session/agent datum. Daytona extraction remains scoped to its sandbox root. A flight trace is daemon-global and may contain identifiers from multiple workspaces, so it must not be exposed through a workspace-scoped endpoint without an explicit operator/global authorization model and redaction decision.
- **Official Compozy skill:** no update for behavior-preserving standard-library, test, or ownership refactors. Any future public support artifact, config key, native tool, CLI path, or diagnostic behavior change must update `skills/compozy/` in the same change.
- **Web/Docs impact:** no web route/component/hook or site documentation impact for the immediate mechanical/security refactors. If support-bundle creation gains a publicly observable draining status/error, update generated contracts, CLI/API docs, and the relevant UI state. Flight-recorder configuration/artifacts require config reference and support documentation.
- **Config lifecycle:** no immediate `config.toml` change. Flight recording must not be smuggled in with hard-coded defaults; it needs keys for enablement, buffer bound, dump trigger/retention, documented defaults, and boot/reload/shutdown ownership.
- **QA tracker:** `AsType`, `b.Loop`, `rand/v2`, iterator utilities, `WaitGroup.Go`, and `Once*` are non-user-visible refactors. `OpenRoot` preserves valid extraction and strengthens invalid-path rejection; its canonical package tests own the invariant. Support shutdown is covered by integration tests unless it introduces a new user-visible drain response, in which case the corresponding QA scenario must be added/reset and walked.

## Risks / Mismatches

1. **The supplied counts are project-wide, not slice-local.** In this slice there are 15 residual production `errors.As` calls, three `b.N` loops, one production `math/rand` use, one concrete `net.Dialer.DialContext`, and scanner-based subprocess framing. Applying conclusions based on the review's 259/14/4/60/18 counts without resolving call sites would over-migrate this surface.
2. **`WaitGroup.Go` is not a generic replacement for every `Add(1)`.** Session `startWG` and clarify waiters count work already executing in caller goroutines. Wrapping that work in new goroutines would alter blocking, error, cancellation, and ordering semantics. `Go` is correct only where the owner actually launches the task and can order launch against shutdown.
3. **Joining exposes non-cooperative code.** Adding a wait group to subprocess handlers without defining handler cancellation may turn a quiet leak into a shutdown hang. That is an improvement only if the context-cooperation contract is deliberate and tested. Doctor probes have the same fundamental constraint.
4. **A process handle is not a process tree.** Unix group signaling and Windows job objects are the actual descendant-cleanup primitive. Replacing them with root-process handles could leak wrappers or grandchildren. Durable records also cannot serialize an OS handle across daemon restart.
5. **Fake time has a boundary.** `synctest` virtualizes Go timers in its bubble; it does not advance child-process, remote SSH, kernel watcher, or external server clocks. Using it around real integration tests can deadlock or create false confidence. Existing injected clocks/sleep functions are preferable when already part of the production design.
6. **`OpenRoot` must be a hard cut, not a second check.** Keeping `ensureSafeParent` plus root operations would duplicate policy and invite divergence. Conversely, a careless conversion can break safe relative symlinks, mode propagation, replacement semantics, or error wrapping. Existing round-trip and unsafe-entry tests must remain the owner.
7. **`OnceValue` changes error and panic replay semantics.** It is beneficial where all callers should observe one close error, but wrong for mutable lifecycle state, restartable runs, or work parameterized by the first caller context. The update detector is the clearest false positive.
8. **`cmp.Or` does not normalize.** Replacing trim-aware fallback helpers would make whitespace a selected value and change identifiers, titles, errors, or defaults. Consolidation is safer than generic substitution.
9. **Public iterators would weaken ownership clarity.** A returned sequence can accidentally retain a lock, transaction, database cursor, or mutable backing state for caller-controlled time. Current slice and page results are explicit snapshots and transport-friendly JSON shapes.
10. **Artifact and trace retention are security features, not test cosmetics.** Provider homes, subprocess output, session data, and global runtime traces can contain credentials or cross-workspace identifiers. `T.ArtifactDir` does not automatically satisfy `docs/qa` evidence, redaction, teardown, or retention requirements. Binary runtime traces cannot be passed through the existing JSON/text redaction helpers.
11. **The `rand/v2` performance claim is unproven.** Preserve the deterministic injection seam and treat the import migration as API hygiene. Do not advertise a lock-contention win without a benchmark/profile on this path.
12. **Mechanical cleanup can still violate file architecture.** Several files are within 10–46 lines of the production cap. New lifecycle state and helpers should be split into focused files; existing near-cap sources must not grow.
13. **Cross-origin protection cannot be concluded from daemon composition.** The actual handler/middleware and WebSocket/SSE behavior live outside the scoped package. A misplaced wrapper can reject legitimate SPA traffic or miss upgrade paths.
14. **No gate evidence was produced.** The scoped explorer contract allowed one analysis-file write and read-only inspection only. Every implementation batch still requires the repository's canonical affected gate and final full gate.

## Open Questions

1. **What is the accepted support-bundle shutdown contract?** Recommended default: stop admission, cancel in-flight builds with a service-owned cause, mark them terminal/failed, join them, then close snapshot stores. If product instead promises completion across daemon shutdown, shutdown must wait up to its context and define what happens after the deadline.
2. **Must an inbound subprocess handler always honor its lifecycle context?** Recommended default: yes, document it as a hard handler contract and make `Process.Done` join accepted handlers. If third-party handlers are possible, add an explicit per-handler deadline or isolate them; do not leave the behavior implicit.
3. **Are doctor probes trusted built-ins only, or can extension code register arbitrary in-process probes?** Trusted built-ins can use a tested cooperative-context contract. Untrusted probes need process isolation because Go cannot cancel a non-cooperative goroutine safely.
4. **Should a flight trace ever be included in a support bundle, and who may request it?** The decision must specify opt-in/default, global versus workspace visibility, sensitive-data classification, manifest/schema version, size/retention, trigger, and config/docs/official-skill changes before implementation.
5. **Does the gate/CI stack retain and index Go test artifacts and attributes?** Before adopting `T.ArtifactDir`/`T.Attr`/`T.Output`, choose one harness-level pilot, define redaction and upload retention, and explain how this evidence relates to—not replaces—the committed `docs/qa` lifecycle.
6. **Which HTTP routes and upgrade paths should `CrossOriginProtection` cover?** Hand this to the HTTP transport/security slice with mutation-route, WebSocket, SSE, loopback, proxy, and UDS evidence. The daemon composition layer alone is insufficient.
7. **Should ACP local file operations receive a root-handle redesign after Daytona extraction?** Recommended default: yes for read/write paths, but only through a permission result that identifies an allowed root plus relative path and owns root closure. Keep terminal CWD authorization as a separately documented limitation rather than pretending `OpenRoot` secures `exec.Cmd.Dir`.

## Evidence

- Review claims and proposed APIs: `/home/pedronauck/.codex/attachments/36398ca5-d255-4ce7-92d3-0d3429f3838f/pasted-text-1.txt:1` and `:30-66`.
- Surface inventory: `rg --files` over the 25 scoped roots returned 831 production `*.go` files and 286 `*_test.go` files. Production files over 500 lines: zero. Near-cap examples are the concrete files and counts listed in Overview.
- Ownerless support work: `internal/support/service.go:111` defines `Service` without lifecycle fields; `internal/support/service.go:139`, `:146`, and `:147` accept work, detach its context, and launch a raw goroutine. `internal/daemon/boot_extension_publishers.go:108` and `:121` construct the service. `internal/daemon/daemon_shutdown_targets.go:14` defines shutdown targets without the support service, while `internal/daemon/daemon_shutdown_runtime.go:5` and `internal/daemon/daemon_shutdown_resources.go:7` perform runtime/resource teardown.
- Existing support detachment contract: `internal/support/service_test.go:137` (`TestServiceCreate`) proves request cancellation does not cancel an accepted bundle, but has no service-shutdown assertion.
- Session ownership distinctions: synchronous starts call `startWG.Add(1)`/`Done` at `internal/session/manager_start_run.go:51` and `:68`; compaction calls `Add` then `go` at `internal/session/compaction.go:114-115`; process-watch registration is at `internal/session/manager_process_watchers.go:26` and launch/Done at `internal/session/manager_lifecycle.go:76-82`.
- Canonical session lifecycle tests: compaction join is owned by `internal/session/manager_hooks_test.go:1758`; process-watcher cancellation/join is owned by `internal/session/manager_test.go:4498`.
- Task-role activation ordering: manual admission/Add/go/Done is at `internal/daemon/task_role_runtime.go:121-152`; shutdown closes admission and waits at `internal/daemon/task_role_prompt.go:200-222`; lifecycle tests are at `internal/daemon/task_role_runtime_test.go:100` and `:147`.
- Existing `WaitGroup.Go` adoption: `internal/scheduler/scheduler.go:137`, `internal/daemon/model_catalog.go:138`, `internal/daemon/spawn_reaper.go:106`, `internal/daemon/network_wake_runner_dispatch.go:158`, `internal/daemon/task_role_prompt.go:161`, and `internal/daemon/coordinator_runtime_reconcile.go:187`.
- Untracked subprocess handlers: `internal/subprocess/transport.go:63` defines context-aware handlers; `internal/subprocess/transport.go:365-380` launches them without a join primitive. Process teardown cancels lifecycle and shuts down the reader at `internal/subprocess/process_lifecycle.go:32`, `:39`, and `:56`, then closes `Done` at `:81`.
- Doctor timeout leak condition: `internal/doctor/doctor.go:105-124` launches a probe goroutine and returns when `probeCtx` expires without joining the probe.
- Process identity/tree safety: root process wait is `internal/subprocess/process_lifecycle.go:32`; session process watchers select process completion/cancellation at `internal/session/manager_lifecycle.go:79-82`; durable verifier defaults to `procutil.MatchesStartTime` at `internal/toolruntime/registry.go:131`; matching is defined at `internal/procutil/process_started_at.go:10`; Unix group signaling is `internal/procutil/process_group_unix.go:53`; Windows job registration/signaling is `internal/procutil/process_group_windows.go:17`, `:31`, and `:83`.
- Timer pilot evidence: parent polling uses `time.NewTicker` at `internal/procutil/parent_watch_unix.go:33`; its tests use two-second wall-clock guards at `internal/procutil/parent_watch_unix_test.go:33` and `:54`. Clarification timeout uses `time.NewTimer` at `internal/daemon/clarify_bridge.go:150`; its owning tests contain millisecond/second polling guards at `internal/daemon/clarify_bridge_test.go:388`, `:437`, and `:449`.
- Existing deterministic seams: scheduler exposes `WithClock` specifically for deterministic tests at `internal/scheduler/types.go:275-279`; retry exposes `RandFloat64` at `internal/retry/backoff.go:22` and injects it throughout `internal/retry/retry_test.go:28`, `:252`, `:261`, `:271`, and `:281`; retry sleep is injected at `internal/retry/retry_test.go:30`, `:116`, and `:159`.
- `rand/v2` candidate: the only scoped production v1 default is `internal/retry/backoff.go:62`.
- `b.Loop` candidates: `internal/daemon/prompt_skills_test.go:305` and `internal/daemon/loop_watch_events_observer_bench_test.go:99` and `:127`.
- Residual typed extraction: `internal/acp/client.go:192`, `internal/acp/failure.go:76`, `internal/acp/handlers_helpers.go:69`, `internal/agentidentity/errors.go:126`, `internal/diagnostics/item.go:353`, plus the ten daemon/session calls enumerated by the scoped `errors.As(` survey.
- `Once*` candidates and false positives: `internal/clientstate/service.go:33` and `:289`; `internal/sandbox/daytona/sidecar_session.go:29-31`, `:76`, and `:113`; `internal/sandbox/daytona/ssh_session.go:23` and `:71`; `internal/version/version.go:43`; context-dependent install detection at `internal/update/detect.go:15-23` with stored state at `internal/update/types.go:182-183`.
- Archive TOCTOU: `internal/sandbox/daytona/tar.go:222` validates a parent, `:225` separately opens the path, and `:297` defines the check. Existing unsafe/symlink/round-trip owners are `internal/sandbox/daytona/tar_test.go:12`, `:50`, and `:64`.
- ACP multi-root follow-up: file reads/writes resolve and then separately access paths in `internal/acp/tool_host.go:121-156`; path authorization/evaluation is in `internal/acp/permission.go:169-271`.
- Typed dial mismatch: the only scoped concrete `net.Dialer.DialContext` is `internal/sandbox/daytona/ssh_transport.go:31`; it receives a generic network/address seam and a hostname built later in the same file.
- Framing mismatch for `Buffer.Peek`: subprocess framing is bounded newline scanning at `internal/subprocess/transport.go:259-260`, not a `bytes.Buffer` parser.
- Iterator/snapshot contracts: clientstate sorted clone list begins at `internal/clientstate/service.go:111`; session stable active snapshot at `internal/session/manager.go:244`; paginated stable union at `internal/session/catalog_page.go:70`; toolruntime snapshot/filter at `internal/toolruntime/memory_store.go:63`.
- Internal iterator-utility candidates: manual sorted keys in `internal/doctor/doctor.go:247-260` and `internal/heartbeat/source_path.go:218-224`; safe existing `SplitSeq` at `internal/update/manager_archive.go:26` and `internal/sandbox/daytona/tar.go:316`.
- `omitzero` no-op evidence: pointer timestamps are intentional at `internal/acp/types.go:227-234`, `internal/update/types.go:108`, and `internal/sandbox/daytona/state.go:33`; the scoped value duration is numeric `omitempty` at `internal/daemon/hook_binding_resources.go:59`.
- Trim-aware default semantics and duplication: `internal/daemon/hook_agent_events.go:331-338`, `internal/daemon/memory_session_ledger_parse.go:166-173`, `internal/daemon/loop_watch_events_observer.go:418-425`, `internal/daemon/loop_managed_prompt.go:274-281`, and `internal/daemon/role_resolver.go:355-362` all trim before selecting.
