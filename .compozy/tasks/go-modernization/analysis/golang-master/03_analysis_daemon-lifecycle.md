# Analysis: daemon-lifecycle

- **Slice scope:** every Go source, test, and benchmark under `internal/daemon/**`, `internal/subprocess/**`, `internal/procutil/**`, `internal/support/**`, `internal/diagnostics/**`, `internal/testutil/**`, and `internal/coordinator/**`.
- **Slug / ordinal:** `03_analysis_daemon-lifecycle`.
- **Target:** `.compozy/tasks/go-modernization/analysis/golang-master/03_analysis_daemon-lifecycle.md`.
- **Research question:** re-audit daemon lifecycle, process ownership, concurrency, cleanup, interfaces, tests, and applicable modern Go features under the new `golang-master` doctrine, treating all current code as unproven.
- **Freshness:** this analysis was rebuilt from a new inventory and direct reads of current sources. No previous analysis artifact was read. The supplied review attachment was used only as a list of claims to re-prove against the current tree.

## Overview

The slice is not ready for a mechanical “modern Go” pass. It already uses many current APIs well—`errors.Join`, `errors.AsType`, `sync.WaitGroup.Go`, `b.Loop`, `slices`, `max`, range-over-integer, and explicit timer/ticker ownership—but its highest-risk defects are ownership defects that those APIs do not solve by themselves.

The fresh inventory contains **732 Go files / 208,552 lines**: 529 production sources and 203 test sources, with seven benchmark files included in the test count. Every file participated in file-size and pattern sweeps; the high-signal lifecycle and cleanup paths were then read end to end. No generated Go source was found in the slice, so generated code does not explain or exempt any finding. No production file exceeds the project’s 500-line hard cap. Tests are excluded from that cap, but several canonical suites are so large that they now impede ownership and review.

The audit’s principal conclusions are:

1. **Daemon shutdown loses ownership before shutdown completes.** It copies all runtime handles into a local struct, clears the daemon’s fields, and then performs cleanup. A concurrent `Shutdown` can therefore return while the first shutdown is still running; a failed/timed-out shutdown cannot retry because the handles are gone; and `boot` can observe an apparently empty daemon while old resources remain live. This is a critical, high-confidence control-flow defect [E03, E04].
2. **Windows process-tree ownership leaks on natural child exit.** Job handles are kept in a PID-keyed global map and are removed only by a group-wait path that the normal-exit path does not invoke. Beyond handle/map leakage, PID reuse can cause a later registration to close a newly created kill-on-close job and terminate the new process. This is critical and directly visible from the current control flow [E06, E09].
3. **Several runtimes race work admission against `Wait`.** `WaitGroup.Go` is present, but `modelCatalogRuntime`, task wake draining, scheduler wake draining, coordinator prompt draining, and the authored-context wake prompter lack a common “close admission, cancel, wait” barrier. One worker is completely untracked. This violates the `golang-master` goroutine floor [E10].
4. **Support-bundle construction leaks resources on partial failure and collides by timestamp.** Early returns after `tar`/`gzip` construction remove the temporary path without closing the writers/file; concurrent builds in the same second use the same final and temporary paths with `O_TRUNC` [E05].
5. **Context and error discipline remain systemic issues.** The scope contains 148 `if ctx == nil` fallbacks, 608 `context.Background()` calls, four `context.TODO()` calls, production string-matching of errors, and ignored cleanup/write errors in tests and test-support production code. True lifecycle roots need explicit owner contexts; internal APIs must reject or propagate rather than silently manufacture ancestry [E03, E05, E09, E12, E14, E15, E19].

This was a static/source audit by contract: no tests, builds, formatters, generators, package managers, or Git commands were run. Severity describes plausible impact; confidence describes how directly current source proves the mismatch.

## Mechanisms / Patterns

### Lifecycle ownership model

The reusable invariant for an owned runtime is:

`construct privately → publish atomically → admit work → close admission → cancel → join workers → close resources → publish terminal result`

Daemon boot mostly follows the first half. `bootState` stages resources, `bootCleanup` unwinds registered actions in reverse order with `errors.Join`, and `publishBootState` publishes under the daemon mutex only after construction succeeds [E02]. This is a strong pattern and should be retained.

Shutdown does not follow the second half. `detachShutdownTargets` swaps no durable terminal state into the daemon; it merely copies fields and immediately clears them. The teardown’s completion and result are local to the first caller [E03]. A correct shutdown state needs one owner-visible `stopping` state, one immutable target snapshot, one shared `done` channel, and one stored terminal error. Later callers should wait on the same operation with their own context, not start or simulate a second shutdown.

The same invariant applies below the daemon:

| Component | Admission barrier | Exit signal | Owner wait | Assessment |
|---|---|---|---|---|
| `support.Service` | `lifecycleMu` + `closing` | owner context cancellation | persistent `shutdownDone` | Sound model; keep [E05] |
| `loopActionRuntime` | `spawnMu` + `stopping` | root cancellation | `wg.Wait` | Sound admission ordering; waiter implementation can be shared [E11] |
| `taskRoleRuntime` | `lifecycleMu` + `stopping` | drain cancellation | `wg.Wait` | Correct `Add` ordering; migrate to `wg.Go` inside the same lock [E11] |
| `modelCatalogRuntime` | cancellation check only | runtime cancellation | a new waiter goroutine per shutdown | Race: check-to-spawn gap permits `Go` concurrent with `Wait` [E10] |
| task/scheduler/coordinator event drainers | none | owner context cancellation | a new waiter goroutine per shutdown | Race: public work can be spawned after waiting begins [E10] |
| authored-context event drainer | none | context cancellation | none | Raw goroutine has no owner-visible completion [E10] |
| ACP mock async controls | connection completion is an implicit barrier | lifecycle cancellation | `asyncWG.Wait` | Make the barrier explicit and use `wg.Go`; do not ignore async reporting errors [E20] |

The repeated “allocate `done`, start a goroutine that calls `wg.Wait`, select on `done`/`ctx.Done`” code is duplicated across runtimes. On timeout, each waiter remains alive until all workers finish; if a worker never exits, the waiter is permanently orphaned. A persistent completion channel closed once by the lifecycle owner removes the duplication and makes repeated shutdown calls joinable.

### Process and process-tree ownership

There are three distinct identities and they must not be conflated:

- the direct `*os.Process` child;
- the Unix process group / Windows Job Object containing descendants;
- the persisted `toolruntime.ProcessRecord`.

Current code frequently reduces the process-tree identity to an integer PID/PGID. Unix uses negative-PGID signals and `/proc` enumeration; Windows stores Job handles in a global `sync.Map` keyed by PID [E06, E07]. The process wait path only forces/waits for the group when shutdown was explicitly requested, so a natural root exit can leave descendants and, on Windows, the Job handle live [E09].

`os.Process.WithHandle` / `os.ErrNoHandle` should be adopted for identity-safe operations on the direct child, but they do not by themselves solve negative-PGID signaling or Job ownership. The durable abstraction should be a process-tree registration handle returned at launch and owned by `subprocess.Process`; it should expose signal/wait/close behavior per platform and be closed on every terminal path. The persisted registry completion is bookkeeping and must not delay closing `Process.Done` indefinitely [E09].

`DetachedProcess` has a narrower footgun: it starts its reaper only when a caller invokes `Wait` or `Done`. Current restart code does call `Wait`, but the exported constructor does not enforce that contract. Starting the single reaper eagerly at construction makes “spawned child is eventually reaped” an invariant of the type rather than a caller convention [E08].

### Cleanup and error contracts

The slice has a strong base: 186 `errors.Join` call sites, reverse-order boot cleanup, checked file closes in `storeseed`, and staged process-start cleanup [E02, E21]. The remaining defects are concentrated at multi-step ownership boundaries:

- Support bundle construction acquires file → gzip writer → tar writer, but early cancellation/manifest failure only attempts path removal. Teardown must always attempt `tar.Close`, `gzip.Close`, `file.Close`, then temporary-file removal, joining every error [E05].
- A process-registry registration failure logs cleanup failure but returns only the registration error, violating “return or log, not both” and losing the cleanup failure to the caller [E09].
- Error text is used as control flow for “file already closed”, scanner token overflow, and permission denial. These need typed/sentinel boundaries; tests may still assert human-readable context in addition to `errors.Is`/`errors.AsType` [E09, E19].
- E2E helpers discard `Body.Close`, file removal, encoder, write, process wait, and manager shutdown errors. This weakens test evidence and violates the repository-wide no-discard rule even though the code is test-oriented [E14, E15].
- Support retention silently keeps an operation when artifact removal fails, without returning or recording why. Keeping it is conservative, but the failure must be observable [E05].

### Context topology

The scope mixes three legitimate context roles:

1. **Process/lifecycle roots** created by constructors such as `support.NewService` and `subprocess.Launch`.
2. **Request contexts** that must be passed unchanged or deliberately bounded.
3. **Cleanup contexts** deliberately detached from caller cancellation but bounded by a cleanup timeout.

The problem is not every `Background`; it is that internal functions silently convert `nil` into `Background`/`TODO`, making missing ancestry indistinguishable from deliberate ownership. `Drain` correctly rejects nil while `Shutdown`, support building, subprocess checkpointing, detached spawning, and boot cleanup normalize it, yielding inconsistent contracts [E03, E05, E08, E09]. Constructors/entry points may create roots and should document them; every other API should reject nil and accept a caller-supplied context. Test helpers should derive timeouts from `testing.T.Context()` rather than `context.Background()` [E13].

### Testing, observability, and performance

No production path in this slice uses `time.Sleep`; all 14 sleeps are tests or helper-process behavior. `testing/synctest` is appropriate only for the in-process notifier polling cases. It cannot virtualize an external subprocess, filesystem watcher, UDS/HTTP server, or OS process table, so those tests need explicit event/protocol barriers or bounded polling rather than blanket conversion [E16].

The E2E artifact collector duplicates the testing runtime’s artifact lifecycle with `os.MkdirTemp`, conditional retention, and ignored removal. `T.ArtifactDir` should own storage and `T.Attr` should record artifact/transport metadata. `T.Output` is useful for helper diagnostics, but it cannot replace path-backed logs that must be handed to an external daemon process [E14]. Both HTTP clients also need explicit timeouts, and the UDS transport is a direct candidate for typed `DialUnix` [E14].

Performance changes remain profile-driven. `b.Loop` has only three legacy loops left, while no evidence supports converting snapshot-returning APIs to `iter.Seq`, interning high-cardinality IDs with `unique`, or inserting `cmp.Or`. `FlightRecorder` is operationally relevant to a long-running daemon, but enabling it without a bounded buffer, trigger/dump contract, redaction decision, and CLI/HTTP/native-tool exposure would create an unmanaged subsystem.

### Modern Go feature disposition

The status vocabulary is exact: `adopt`, `already`, `reject`, `defer`, or `not applicable`.

| # | Feature | Status | Slice-specific disposition |
|---:|---|---|---|
| 1 | `errors.AsType` | `adopt` | Twenty uses already prove compatibility; migrate the remaining 59 `errors.As` call sites where the target implements `error`. Make `diagnosticItemCarrier` embed `error`, then use `AsType`; do not force it where a non-error interface is intentional [E12, E18]. |
| 2 | `b.Loop` | `adopt` | Eighteen benchmark loops already use it; convert the three remaining `b.N` loops [E17]. |
| 3 | JSON `omitzero` | `defer` | No value-`time.Time` omission bug was found. Existing optional times are pointers and scalar/slice `omitempty` semantics are public wire contracts. Revisit only with the owning contract/codegen audit [E18]. |
| 4 | `os.OpenRoot` | `not applicable` | The support code opens explicit trusted log paths and `WalkDir` only lists a configured home tree without following descendant symlinks. No untrusted relative traversal in this slice justifies a root capability [E05]. |
| 5 | `strings` sequence APIs | `adopt` | Three current `SplitSeq` loops are good. Convert one-pass parsers such as orphan output; retain slices where code needs indexing/reversal, and use `strings.Cut` for “first line” rather than allocating a split [E17]. |
| 6 | `sync.WaitGroup.Go` | `adopt` | Convert the valid manually gated task-role and ACP mock patterns. First add admission barriers to unsafe runtimes: `Go` does not make `Go`-versus-`Wait` ordering safe when the group may be empty [E10, E11, E20]. |
| 7 | range over integer | `already` | Count-only loops already use it (for example manifest stabilization). Remaining indexed loops use the index or are the benchmark cases covered by `b.Loop` [E17]. |
| 8 | `slices` / `maps` / `min` / `max` | `already` | The slice uses these broadly and idiomatically (`slices.Backward`, `Clone`, `Contains`, `max`). No mechanical rewrite is needed [E02, E22]. |
| 9 | `testing/synctest` | `adopt` | Use for pure in-process goroutine/timer tests, especially notifier polling. Reject it for real subprocess, watcher, socket, and filesystem time [E16]. |
| 10 | `iter.Seq` | `reject` | Current list returns are small snapshots/public contracts; channels model actual concurrent streams. Adding iterators would complicate cancellation and error ownership without measured benefit. |
| 11 | `os.Process.WithHandle` / `os.ErrNoHandle` | `adopt` | Use in the new direct-child/process-tree handle boundary to reduce PID-reuse exposure. Retain platform-specific group/Job logic behind that boundary [E06, E07, E09]. |
| 12 | `sync.OnceFunc` / `OnceValue(s)` | `adopt` | Use selectively for bound, no-argument once-actions such as detached reaping, health-monitor stop, and ready-channel close. Do not wrap context-sensitive `Shutdown(ctx)` itself; callers need independent wait contexts and a shared result [E08, E18]. |
| 13 | `math/rand/v2` | `not applicable` | No `math/rand` use exists in the scope. |
| 14 | `cmp.Or` | `reject` | No default chain is clearer with it. In particular, it must not hide nil contexts or collapse semantically distinct zero values. |
| 15 | `T.ArtifactDir` / `T.Attr` / `T.Output` | `adopt` | Move E2E artifact roots to `ArtifactDir`, annotate paths/transports with `Attr`, and use `Output` for in-process helper diagnostics. Keep path-backed process logs where the external daemon requires a filename [E13, E14]. |
| 16 | `http.CrossOriginProtection` | `defer` | Production HTTP routing is owned outside this slice; daemon only owns a `Server` lifecycle interface. Evaluate at the HTTP handler/router boundary, not in daemon composition [E04]. |
| 17 | `runtime/trace.FlightRecorder` | `defer` | Valuable candidate for daemon diagnostics/support bundles, but first define bounded memory, enablement, redaction, dump trigger, persistence, and agent-manageable surfaces. |
| 18 | typed `DialUnix` / `DialTCP` | `adopt` | Replace the E2E UDS client’s string-network `DialContext` with typed Unix dialing. No TCP-specific call site needs conversion here [E14]. |
| 19 | `bytes.Buffer.Peek` | `reject` | No matching buffer parser exists. Subprocess framing uses `bufio.Scanner`; its token-limit error needs a typed framing strategy, not `Peek` [E09]. |
| 20 | `unique` | `reject` | No profile shows repeated comparable values dominating memory. Converting session/task/event IDs to handles would spread through serialization and APIs without evidence. |

## Relevant Sources

### Exhaustive package inventory

Benchmark files are a subset of the test count.

| Package / directory | Production | Tests | Benchmark files | Total files | Lines |
|---|---:|---:|---:|---:|---:|
| `internal/daemon` | 449 | 154 | 2 | 603 | 179,321 |
| `internal/subprocess` | 11 | 11 | 1 | 22 | 4,792 |
| `internal/procutil` | 13 | 7 | 1 | 20 | 1,891 |
| `internal/support` | 7 | 1 | 0 | 8 | 1,378 |
| `internal/diagnostics` | 2 | 4 | 1 | 6 | 1,133 |
| `internal/coordinator` | 1 | 2 | 1 | 3 | 732 |
| `internal/testutil` | 2 | 3 | 1 | 5 | 707 |
| `internal/testutil/acpmock` | 12 | 4 | 0 | 16 | 4,264 |
| `internal/testutil/acpmock/cmd/acpmock-driver` | 8 | 2 | 0 | 10 | 1,937 |
| `internal/testutil/bridgeformat` | 1 | 0 | 0 | 1 | 132 |
| `internal/testutil/e2e` | 18 | 13 | 0 | 31 | 10,905 |
| `internal/testutil/mcpfixture` | 4 | 2 | 0 | 6 | 1,187 |
| `internal/testutil/storeseed` | 1 | 0 | 0 | 1 | 173 |
| **Total** | **529** | **203** | **7** | **732** | **208,552** |

### Source routing and read depth

The full source set was checked for package declaration, build constraints, generated markers, file length, contexts, goroutine starts, wait groups, timers/sleeps, process/PID/group operations, resource acquisition/cleanup, error wrapping/matching/discard, interface assertions, JSON tags, and all 20 requested modern features.

Deep reads were routed as follows:

- **Daemon composition and ownership:** staged boot/publication, admission, shutdown target capture/reset, runtime/server/persistence teardown, and the model/task/scheduler/coordinator/authored-context worker lifecycles [E02–E04, E10, E11].
- **Subprocess and OS process trees:** launch, transport, cooperative/escalated shutdown, wait/completion, detached launch/reaping, Unix groups, and Windows Jobs [E06–E09].
- **Support and diagnostics:** service admission, async bundle creation, archive and file cleanup, retention, home-tree/log reads, structured diagnostic errors, and redaction boundaries [E05, E12].
- **Test infrastructure:** shared contexts, artifact collection, HTTP/UDS clients, mock agents, helper subprocesses, async ACP controls, store seeds, MCP fixtures, and benchmark loops [E13–E21].
- **Coordinator leaf package:** deterministic decision/policy helpers were read separately from the daemon-owned coordinator runtime. It already returns cloned slices and contains no independent goroutine/process ownership [E22].

The small `bridgeformat`, `mcpfixture`, and `storeseed` leaf packages contain no additional lifecycle-critical mismatch. `storeseed` is a positive cleanup example; `mcpfixture.MustNew` is an intentionally named programmer-error panic, not a production panic path [E21].

Structural checks found no production file above 500 lines, but the test surface has pronounced Large Module hotspots: `native_tools_test.go` (11,341 lines), `daemon_test.go` (10,244), `daemon_integration_test.go` (4,674), `task_runtime_test.go` (4,263), and `daemon_mock_agents_integration_test.go` (2,559) [E18]. These are canonical-suite ownership concerns, not a reason to duplicate invariants into new regression files.

## Transferable Patterns

1. **Publish one runtime state object, not parallel field lists.** Preserve `bootState`’s staged construction, but let `Daemon` hold a single immutable/owned runtime-state pointer. Shutdown atomically changes `running → stopping`, retains the target state until teardown settles, and then records `stopped + result`. This removes the parallel field maintenance across `bootState`, `publishBootState`, `shutdownTargets`, and `resetRuntimeStateLocked`. Fowler: **Extract Class**, **Encapsulate Variable**, **Move Field**, **Split Phase** [E02–E04].
2. **Use a lifecycle gate around every spawn.** The gate owns `closing`, root cancel, wait group, and persistent `done`. `Start`/`Go` acquires the gate, rejects after close, and increments before releasing the lock. `Shutdown` closes admission under the same lock, cancels, and waits on `done`. Fowler: **Extract Class** and **Replace Inline Code with Function Call**. `support.Service` and `loopActionRuntime` are local templates [E05, E11].
3. **Return an owned process-tree registration handle.** Registration should return a value containing the direct child identity plus Unix-group or Windows-Job state. The owner closes it unconditionally after `cmd.Wait`; signal/wait operations no longer rediscover state through a global PID map. Fowler: **Replace Primitive with Object**, **Extract Class**, **Encapsulate Variable** [E06–E09].
4. **Separate OS exit, descendant exit, persistence, and public completion.** The process’s terminal sequence should be explicit phases. OS/process-tree cleanup must determine `waitErr` and close the public completion signal; bounded registry persistence may follow or be joined according to a documented contract. Concurrent `Shutdown` calls join a single stop operation rather than each issuing transport requests/signals. Fowler: **Split Phase** and **Extract Function** [E09].
5. **Use one cleanup matrix for multi-resource construction.** Record each successful acquisition immediately; on every exit, attempt cleanup in reverse order and `errors.Join` failures. Only commit/rename after all writer/file closes succeed. Use operation identity or `CreateTemp` for unique temporary names. Fowler: **Split Phase**, **Extract Function**, **Move Statements into Function** [E05].
6. **Make context roots rare and named.** Constructors or top-level runners may create `ownerCtx`; request methods require non-nil contexts; cleanup uses a named bounded detachment helper. Remove nil-to-`Background`/`TODO` normalization. Tests derive from `t.Context()`. Fowler: **Change Function Declaration** and **Introduce Assertion** [E03, E05, E08, E09, E13].
7. **Make error categories machine-readable at the boundary.** Convert scanner overflow, benign-close, permission, and process-tree terminal states to sentinels or typed errors once, then use `errors.Is`/`errors.AsType`. Preserve human context with `%w`. Fowler: **Replace Primitive with Object** and **Introduce Special Case** [E09, E12, E19].
8. **Let the testing runtime own artifacts and time where possible.** Use `T.ArtifactDir`/`T.Attr`, `T.Output`, `T.Context`, and targeted `synctest`; use protocol/event barriers for external systems. Preserve the existing artifact manifest as the product evidence contract, but make its root a standard testing artifact directory. Fowler: **Substitute Algorithm** and **Extract Function** [E13–E16].
9. **Split canonical test suites by behavior, not by duplicate invariant.** Move coherent fixture/assertion families out of the five giant daemon test files while retaining package-level access and one owning suite per invariant. Fowler: **Move Function**, **Extract Function**, **Extract Class** [E18].
10. **Keep consumer interfaces narrow.** The composition root may aggregate stores, but each runtime/constructor should depend on its 1–3 method consumer-owned interface. Replace `NewItem`’s seven parallel strings with an `ItemSpec` parameter object and use a canonical compile-time assertion. Fowler: **Introduce Parameter Object**, **Change Function Declaration**, **Extract Class** [E04, E12].

## Risks / Mismatches

| ID | Severity | Confidence | Risk / mismatch | Fowler technique and required correction |
|---|---|---|---|---|
| R1 | Critical | High | Daemon runtime handles are cleared before teardown completes. Concurrent shutdown can return early, retry cannot recover timed-out cleanup, and boot can race live old resources. | **Extract Class + Encapsulate Variable + Split Phase:** durable `runtimeState` with `running/stopping/stopped`, shared `done`, stored result, targets retained until terminal [E02–E04]. |
| R2 | Critical | High | Windows Jobs are stored globally by PID and closed only through group wait. Natural child exit leaks the handle/map entry; PID reuse can close the newly assigned kill-on-close Job and kill the new child. | **Replace Primitive with Object + Extract Class:** registration handle owned and unconditionally closed by `Process`; platform tests for natural exit, descendants, timeout, and PID reuse [E06, E09]. |
| R3 | High | High | `modelCatalogRuntime`, task wake, scheduler wake, coordinator drainers, and authored-context drains lack a complete admission/cancel/wait protocol. `WaitGroup.Go` can race `Wait`, and one raw goroutine is never joined. | **Extract Class + Replace Inline Code with Function Call:** common lifecycle gate; persistent completion channel; no per-call waiter goroutines [E10, E11]. |
| R4 | High | High | Support bundle early failures leave tar/gzip/file owners open; same-second concurrent requests share `.tmp` and final names under `O_TRUNC`. | **Split Phase + Extract Function:** reverse cleanup matrix with `errors.Join`; unique operation-based temp/final paths; atomic rename after close [E05]. |
| R5 | High | High | Subprocess terminal ownership is split: natural exit skips group cleanup, registry cleanup failure is logged but not returned, `ProcessRecord.Complete` can delay `Done`, and concurrent `Shutdown` callers can duplicate RPC/signal escalation. | **Extract Class + Split Phase:** one stop operation/result; unconditional process-tree release; bounded persistence; return joined start-cleanup errors [E09]. |
| R6 | High | High | 148 nil-context fallbacks plus four `TODO` roots hide caller defects and sever cancellation inconsistently. | **Change Function Declaration + Introduce Assertion:** reject nil internally; create named owner/cleanup roots only at documented entry points [E03, E05, E08, E09, E13]. |
| R7 | High | High | Production error-string control flow and ignored cleanup/write errors in test infrastructure make behavior locale/message-dependent and test evidence incomplete. | **Replace Primitive with Object + Extract Function:** typed/sentinel errors; checked close/drain/write helpers; return or record every failure [E09, E14, E15, E19, E20]. |
| R8 | Medium | High | `DetachedProcess` reaping is lazy; the type depends on every caller remembering `Wait`/`Done`. Current restart usage complies, but the exported API does not guarantee it. | **Move Function + OnceFunc:** start the one reaper at construction and expose only observation to callers [E08]. |
| R9 | Medium | High | In-process sleep polling, custom temp artifact retention, background-root test contexts, unbounded HTTP clients, and string-network UDS dialing predate current testing/net APIs. | **Substitute Algorithm + Extract Function:** targeted `synctest`, `T.Context`, `T.ArtifactDir`/`Attr`/`Output`, client timeouts, typed `DialUnix` [E13–E16]. |
| R10 | Medium | High | `Daemon` is a Large Class and its state is duplicated across staging/publish/detach/reset; the package has 449 production files and giant test modules. The 500-line production cap passes, but change locality does not. | **Extract Class + Move Field + Move Function:** owner-specific runtime objects and behavior-oriented canonical test files; do not split arbitrarily by filename count [E02–E04, E18]. |
| R11 | Medium | High | The 13-contract `Registry` interface expands coupling, while diagnostic construction has a Long Parameter List and a noncanonical compile assertion. | **Introduce Parameter Object + Change Function Declaration + Extract Class:** consumer-owned store interfaces, `ItemSpec`, `var _ error = (*StructuredError)(nil)`, error-embedding carrier [E04, E12]. |
| R12 | Low | High | Three legacy benchmark loops, several mechanical `errors.As` sites, selectable `sync.Once` closures, and an unreachable `return nil` after `os.Exit` remain. | **Remove Dead Code + Replace Inline Code with Function Call:** apply only after the ownership changes above [E17, E18, E20]. |

Additional mismatches that should travel with the owning remediation rather than become standalone patches:

- `operationStore.cleanup` conservatively retains entries after delete failure but emits no error/diagnostic [E05].
- The transport detects scanner token overflow by matching `"token too long"`; the framing layer should expose a typed oversized-message error [E09].
- `settings_mcp_runtime` falls back to a lower-cased permission string after `errors.Is(os.ErrPermission)`; normalize provider/platform errors at the source instead [E19].
- The ACP mock’s manual `Add/go/Done` is safe only because connection completion is treated as the admission barrier; encode that barrier and replace its discarded `Fprintf` result [E20].
- Support-bundle shutdown order, extensions/hooks shutdown order, process terminal events, and persisted status must remain observable when ownership is refactored [E03, E05, E09].

### Compozy impact audit for the proposed remediation

- **Native tools:** no tool ID/schema change is inherently required for lifecycle encapsulation. Verify availability diagnostics and support/restart/process status outputs because terminal timing and joined errors will change. Any support-bundle filename/path change must be checked against native/API download consumers.
- **Extensibility and hooks:** preserve the current extension, automation, bridge, network, hook, resource reconciler, and session shutdown ordering. The new lifecycle gate must also cover extension-driven callbacks that can spawn owned work during drain.
- **Workspace data isolation:** daemon/process/support ownership is global or process-scoped in this slice; no new workspace-scoped datum is proposed. Verify that support-bundle and process-record diagnostics retain existing authorization/redaction and do not introduce cross-workspace caches or events.
- **Official Compozy skill:** internal-only ownership changes require no skill update. Update the bundled skill only if public CLI/API/native-tool behavior, support artifact naming, error codes, process semantics, or operational trace controls change.

## Open Questions

1. **Is a `*Daemon` intended to support boot-after-shutdown?** `resetRuntimeStateLocked` suggests reuse, but readiness is one-shot. The new state machine must either make restart-on-instance explicit and tested or reject it deterministically. **UNCONFIRMED.**
2. **What is the terminal contract when the direct child exits naturally while descendants remain?** The current shutdown path treats explicit stop differently from natural exit. Decide whether descendants must always be terminated, merely awaited, or transferred to another owner. **UNCONFIRMED.**
3. **May `toolruntime.ProcessRecord.Complete` block or perform remote/contended I/O?** If yes, it must not gate `Process.Done`; define its timeout/retry/diagnostic contract. **UNCONFIRMED.**
4. **How should `os.Process.WithHandle` compose with Unix negative-PGID operations and Windows Job Objects on supported Go/OS versions?** A focused Linux/Windows platform spike and Windows CI test are required before the process-tree abstraction is finalized. **UNCONFIRMED.**
5. **Is the timestamp-only support-bundle filename externally promised?** Greenfield policy permits a hard cut, but API/site/CLI consumers should be checked before adding operation identity or `CreateTemp` uniqueness. **UNCONFIRMED.**
6. **Which package owns production cross-origin policy?** The scoped daemon code owns only server lifecycle. `http.CrossOriginProtection` must be assessed in the actual HTTP router/middleware slice. **UNCONFIRMED.**
7. **Should `FlightRecorder` be an always-running bounded diagnostic, an on-demand support-bundle source, or disabled unless configured?** The answer determines config keys, memory budget, redaction, native/CLI/API controls, and support artifact behavior. **UNCONFIRMED.**
8. **Which giant test file is the canonical owner for each lifecycle invariant before splitting?** The consolidation must map invariant → owning layer → canonical suite so moving helpers does not duplicate coverage. **UNCONFIRMED.**

## Evidence

Evidence is deduplicated by ID; earlier sections refer to these IDs rather than repeating source citations.

| ID | Source citations | Observation |
|---|---|---|
| E01 | `/home/pedronauck/Projects/compozy/.agents/skills/golang-master/SKILL.md:1`; `/home/pedronauck/Projects/compozy/.agents/skills/golang-master/references/errors.md:1`; `concurrency.md:1`; `context.md:1`; `safety.md:1`; `interfaces-generics.md:1`; `style-naming.md:1`; `testing.md:1`; `performance.md:1`; `modernize.md:1`; `layout.md:1`; `/home/pedronauck/Projects/compozy/.agents/skills/eng-code-guidelines/SKILL.md:1`; `/home/pedronauck/Projects/compozy/.agents/skills/eng-cleanup-failure-paths/SKILL.md:1`; `/home/pedronauck/Projects/compozy/.agents/skills/architectural-analysis/SKILL.md:1`; `/home/pedronauck/Projects/compozy/.agents/skills/refactoring-analysis/SKILL.md:1`; `internal/CLAUDE.md:1` | Normative inputs read in full, including every referenced error, concurrency, context, safety, interface, style, testing, performance, modernization, layout, cleanup, architectural-smell, and Fowler-refactoring reference. Relative reference names in this row are under the preceding `golang-master/references/` directory. |
| E02 | `internal/daemon/boot.go:54`; `internal/daemon/boot.go:147`; `internal/daemon/boot.go:157`; `internal/daemon/boot.go:168`; `internal/daemon/boot_publish.go:3`; `internal/daemon/boot_cleanup.go:9`; `internal/daemon/boot_cleanup.go:27`; `internal/daemon/boot_cleanup.go:31`; `internal/daemon/boot_cleanup.go:36` | Boot stages resources, registers reverse cleanup, joins cleanup errors, and publishes only after successful construction. Publication manually copies a long field list. |
| E03 | `internal/daemon/daemon_lifecycle.go:104`; `internal/daemon/daemon_lifecycle.go:108`; `internal/daemon/daemon_lifecycle.go:109`; `internal/daemon/daemon_lifecycle.go:134`; `internal/daemon/daemon_shutdown_targets.go:53`; `internal/daemon/daemon_shutdown_targets.go:98`; `internal/daemon/daemon_shutdown_targets.go:102`; `internal/daemon/daemon_shutdown_runtime.go:5`; `internal/daemon/daemon_shutdown_resources.go:8` | Shutdown drains, detaches, clears runtime fields, then tears down local targets. Cleanup ordering is explicit and error-accumulating, but completion/result ownership is not retained by `Daemon`. |
| E04 | `internal/daemon/daemon.go:73`; `internal/daemon/daemon.go:224`; `internal/daemon/drain.go:21`; `internal/daemon/drain.go:25`; `internal/daemon/drain.go:31` | `Registry` embeds 13 store contracts; `Daemon` mixes static factories/configuration and runtime resources. Drain has the desired explicit nil-context/admission contract. |
| E05 | `internal/support/service.go:115`; `internal/support/service.go:149`; `internal/support/service.go:153`; `internal/support/service.go:162`; `internal/support/service.go:193`; `internal/support/service_lifecycle.go:9`; `internal/support/service_lifecycle.go:17`; `internal/support/service_lifecycle.go:24`; `internal/support/service_builder.go:24`; `internal/support/service_builder.go:36`; `internal/support/service_builder.go:50`; `internal/support/service_builder.go:55`; `internal/support/service_builder.go:58`; `internal/support/service_builder.go:62`; `internal/support/service_builder.go:86`; `internal/support/service_builder.go:89`; `internal/support/operation_store.go:74`; `internal/support/operation_store.go:85`; `internal/support/home_tree.go:13`; `internal/support/home_tree.go:56`; `internal/support/archive_writer.go:93` | Service admission is gated correctly. Builder early exits do not close acquired writers/file; names are second-resolution and shared by concurrent workers; retention suppresses delete failures. Home-tree access does not establish an `OpenRoot` use case; range-int is already used. |
| E06 | `internal/procutil/process_group_windows.go:17`; `internal/procutil/process_group_windows.go:31`; `internal/procutil/process_group_windows.go:63`; `internal/procutil/process_group_windows.go:99`; `internal/procutil/process_group_windows.go:176`; `internal/procutil/process_group_windows.go:198`; `internal/procutil/process_group_windows.go:204` | Windows Job handles are PID-keyed globals; duplicate registration closes the new kill-on-close Job; removal happens only after a successful Job wait. |
| E07 | `internal/procutil/process_group_unix.go:27`; `internal/procutil/process_group_unix.go:53`; `internal/procutil/process_group_unix.go:67`; `internal/procutil/process_group_unix.go:88`; `internal/procutil/process_group_unix.go:100`; `internal/procutil/process_group_unix.go:124`; `internal/procutil/process_group_unix.go:140`; `internal/procutil/process_group_unix.go:250` | Unix group identity is an integer PGID; Linux enumerates `/proc`, then raw signals members/group. Group polling is timer-owned rather than sleep-based. |
| E08 | `internal/procutil/detached.go:54`; `internal/procutil/detached.go:64`; `internal/procutil/detached.go:72`; `internal/procutil/detached.go:81`; `internal/procutil/detached.go:174`; `internal/procutil/detached.go:241` | Reaping begins lazily from `Wait`/`Done`; log read failure is discarded in favor of the process error; the reverse-line parser still materializes a split for valid indexing/reversal reasons. |
| E09 | `internal/subprocess/process.go:142`; `internal/subprocess/process.go:146`; `internal/subprocess/process.go:184`; `internal/subprocess/process.go:187`; `internal/subprocess/process.go:200`; `internal/subprocess/process.go:241`; `internal/subprocess/process.go:276`; `internal/subprocess/process_lifecycle.go:32`; `internal/subprocess/process_lifecycle.go:42`; `internal/subprocess/process_lifecycle.go:74`; `internal/subprocess/process_lifecycle.go:81`; `internal/subprocess/process_lifecycle.go:169`; `internal/subprocess/process_shutdown.go:109`; `internal/subprocess/process_shutdown.go:126`; `internal/subprocess/process_shutdown.go:132`; `internal/subprocess/process_shutdown.go:161`; `internal/subprocess/process_shutdown.go:178`; `internal/subprocess/transport.go:287`; `internal/subprocess/transport.go:370`; `internal/subprocess/health.go:69` | Natural wait skips group force/wait; record completion precedes `done`; registry-start cleanup error is logged but not returned; concurrent shutdown is not coalesced; two production paths match error text; `AsType` and `sync.Once` are already present. |
| E10 | `internal/daemon/model_catalog.go:110`; `internal/daemon/model_catalog.go:120`; `internal/daemon/model_catalog.go:138`; `internal/daemon/model_catalog.go:172`; `internal/daemon/model_catalog.go:177`; `internal/daemon/task_wake_bridge.go:54`; `internal/daemon/task_wake_bridge.go:118`; `internal/daemon/task_wake_bridge.go:142`; `internal/daemon/task_wake_bridge.go:152`; `internal/daemon/scheduler_waker.go:301`; `internal/daemon/scheduler_waker.go:305`; `internal/daemon/scheduler_waker.go:326`; `internal/daemon/scheduler_waker.go:336`; `internal/daemon/coordinator_runtime_reconcile.go:183`; `internal/daemon/coordinator_runtime_reconcile.go:187`; `internal/daemon/coordinator_runtime_wake.go:58`; `internal/daemon/coordinator_runtime_wake.go:68`; `internal/daemon/authored_context_runtime.go:230`; `internal/daemon/authored_context_runtime.go:238` | Work can be spawned without a lock-shared stopping gate; shutdown creates transient waiters; authored-context drain is raw and untracked. |
| E11 | `internal/daemon/task_role_runtime.go:130`; `internal/daemon/task_role_runtime.go:137`; `internal/daemon/task_role_runtime.go:143`; `internal/daemon/task_role_runtime.go:152`; `internal/daemon/loop_action_runtime.go:167`; `internal/daemon/loop_action_runtime.go:169`; `internal/daemon/loop_action_runtime.go:172`; `internal/daemon/loop_action_runtime.go:265`; `internal/daemon/loop_action_runtime.go:269`; `internal/daemon/loop_action_runtime.go:277` | These runtimes order admission closure and wait-group increment under a shared lock; they are the local positive templates. |
| E12 | `internal/diagnostics/item.go:49`; `internal/diagnostics/item.go:273`; `internal/diagnostics/item.go:343`; `internal/diagnostics/item.go:348`; `internal/diagnostics/item.go:353` | `NewItem` accepts seven parallel strings; compile assertion is an array trick; carrier does not embed `error`, forcing legacy `errors.As`. |
| E13 | `internal/testutil/testutil.go:24`; `internal/testutil/testutil.go:28` | Shared test contexts derive from `context.Background`, not `testing.T.Context`. |
| E14 | `internal/testutil/e2e/artifacts.go:242`; `internal/testutil/e2e/artifacts.go:247`; `internal/testutil/e2e/artifacts.go:252`; `internal/testutil/e2e/artifacts.go:257`; `internal/testutil/e2e/runtime_harness.go:241`; `internal/testutil/e2e/runtime_harness.go:249`; `internal/testutil/e2e/runtime_harness.go:253`; `internal/testutil/e2e/mock_agents.go:106`; `internal/testutil/e2e/mock_agents.go:145`; `internal/testutil/e2e/mock_agents.go:209`; `internal/testutil/e2e/mock_agents.go:242`; `internal/testutil/e2e/automation_tasks.go:376` | Custom artifact root/removal, clients without timeout, string-network UDS dialing, and ignored HTTP body closes. |
| E15 | `internal/subprocess/process_test.go:900`; `internal/subprocess/process_test.go:997`; `internal/subprocess/process_test.go:1124`; `internal/testutil/e2e/runtime_harness_lifecycle_test.go:115`; `internal/testutil/e2e/runtime_harness_lifecycle_test.go:117`; `internal/testutil/e2e/runtime_harness_lifecycle_test.go:466`; `internal/daemon/restart_test.go:504`; `internal/daemon/daemon_test.go:3976` | Tests discard shutdown, send, write, close, encode, lock release, and chdir restoration errors. |
| E16 | `internal/daemon/task_event_bridge_notifier_test.go:404`; `internal/daemon/task_event_bridge_notifier_test.go:430`; `internal/daemon/task_event_bridge_notifier_test.go:449`; `internal/subprocess/process_test.go:1043`; `internal/procutil/detached_test.go:19`; `internal/daemon/loop_resources_integration_test.go:263`; `internal/testutil/e2e/runtime_harness_lifecycle_test.go:253` | In-process notifier sleeps are `synctest` candidates; subprocess, detached process, filesystem watcher, and runtime harness sleeps cross real-system boundaries. |
| E17 | `internal/daemon/prompt_skills_test.go:305`; `internal/daemon/loop_watch_events_observer_bench_test.go:99`; `internal/daemon/loop_watch_events_observer_bench_test.go:127`; `internal/daemon/auto_title_generator.go:228`; `internal/daemon/orphan.go:106`; `internal/daemon/native_tool_memory_format.go:83` | Three benchmark loops retain `b.N`; sequence APIs are already used; one-pass parsing and first-line extraction still allocate full splits. |
| E18 | `internal/daemon/native_tools_test.go:1`; `internal/daemon/daemon_test.go:1`; `internal/daemon/daemon_integration_test.go:1`; `internal/daemon/task_runtime_test.go:1`; `internal/daemon/daemon_mock_agents_integration_test.go:1`; `internal/daemon/memory_provider_service.go:125`; `internal/daemon/restart.go:58`; `internal/daemon/hook_binding_resources.go:59`; `internal/daemon/network_wake_runner.go:88` | Measured test line counts are 11,341 / 10,244 / 4,674 / 4,263 / 2,559. Legacy `errors.As`, pointer-time/scalar JSON omission, and selectable `sync.Once` sites remain. |
| E19 | `internal/daemon/settings_mcp_runtime.go:235`; `internal/daemon/native_extension_errors.go:27`; `internal/daemon/role_fallback.go:172` | Permission logic matches text after checking a sentinel; additional concrete typed errors still use legacy `errors.As`. |
| E20 | `internal/testutil/acpmock/cmd/acpmock-driver/main.go:96`; `internal/testutil/acpmock/cmd/acpmock-driver/main.go:98`; `internal/testutil/acpmock/cmd/acpmock-driver/helpers.go:212`; `internal/testutil/acpmock/cmd/acpmock-driver/sandbox.go:66`; `internal/testutil/acpmock/cmd/acpmock-driver/sandbox.go:74`; `internal/testutil/acpmock/cmd/acpmock-driver/sandbox.go:105`; `internal/testutil/acpmock/cmd/acpmock-driver/sandbox.go:108` | ACP mock waits after connection completion, but spawn gating is implicit; async stderr errors are discarded; `return nil` after `os.Exit` is unreachable. |
| E21 | `internal/testutil/storeseed/seed.go:41`; `internal/testutil/storeseed/seed.go:52`; `internal/testutil/storeseed/seed.go:114`; `internal/testutil/storeseed/seed.go:121`; `internal/testutil/mcpfixture/fixture.go:58`; `internal/testutil/bridgeformat/corpus.go:1` | Store seed cleanup joins failures and is a positive helper pattern; MCP fixture panic is explicitly `MustNew`; bridgeformat is static corpus data with no lifecycle owner. |
| E22 | `internal/coordinator/coordinator.go:66`; `internal/coordinator/coordinator.go:74`; `internal/coordinator/coordinator.go:103`; `internal/coordinator/coordinator.go:160`; `internal/coordinator/coordinator.go:215` | Coordinator leaf package is deterministic, clones exported allowlists, and owns no goroutines or processes; runtime lifecycle resides in daemon. |
| E23 | `/home/pedronauck/.codex/attachments/36398ca5-d255-4ce7-92d3-0d3429f3838f/pasted-text-1.txt:1` | Supplied review attachment was read in full but treated only as a hypothesis source; every retained claim above has independent current-source evidence. |
