# Analysis: sessions-workspaces

- **Ordinal / slug:** `04_analysis_sessions-workspaces`
- **Slice scope:** `internal/acp/**`, `internal/session/**`, `internal/sessions/**`, `internal/transcript/**`, `internal/workspace/**`, `internal/workspaceaccess/**`, `internal/filesnap/**`, and `internal/clientstate/**`.
- **Question:** Reassess the complete sessions/workspaces Go surface from the beginning under the current `golang-master`, `eng-code-guidelines`, architectural-analysis, and Fowler refactoring guidance. Identify modernization, safety, duplication, ownership, testing, and performance opportunities without changing business rules.
- **Baseline:** Go `1.26.4` (`go.mod:3`), with the `modernize` analyzer enabled (`.golangci.yml:26`).
- **Method:** Read-only source review of every Go file in the slice: 364 files / 91,239 lines. No prior analysis artifact was used, no code was changed, and no test or build command was run under the explorer's scoped-write contract.
- **Severity:** `high` can hang shutdown, corrupt durable identity, leak cross-owner mutable state, or violate a critical repository rule; `medium` creates bounded lifecycle, maintenance, flake, or contract risk; `low` is local debt. Confidence describes the strength of source evidence, not implementation difficulty.

## Overview

The slice already applies much of modern Go well: wrapped errors, defensive clones, stable ordering, range-over-integer loops, `slices`/`maps` in several hot paths, `b.Loop` in most benchmarks, context-aware transaction boundaries, idempotent persistence, and explicit runtime ownership. The fresh review does not support a blanket syntax rewrite. Its highest-value work is lifecycle ownership and type safety: make every spawned task joinable by its owner, make shutdown waits honor the caller's context, remove an unchecked durable `int64` to platform `int` conversion, and close mutable capability/config alias surfaces.

The new Go features are useful selectively. `errors.AsType`, `WaitGroup.Go`, `strings.FieldsSeq`, `sync.OnceFunc`, and the remaining `slices`/`maps` conversions are sound mechanical changes when their current semantics are retained. `omitzero`, `iter.Seq`, `os.OpenRoot`, `Process.WithHandle`, `FlightRecorder`, test artifacts, and interning need either a broader owner, a demonstrated profile, or an explicit contract decision. `math/rand/v2`, `CrossOriginProtection`, typed network dialing, and `bytes.Buffer.Peek` do not match this slice.

### Complete package coverage

| Package | Go files | Production | Test / benchmark | Lines | Surface reviewed | Primary conclusion |
| --- | ---: | ---: | ---: | ---: | --- | --- |
| `internal/acp` | 64 | 50 | 14 | 16,468 | ACP JSON-RPC, subprocesses, terminals, permission/tool handlers, failures, runtime capabilities | Join ACP-owned asynchronous work before closing process lifecycle; modernize errors and collections without weakening process-group parity. |
| `internal/session` | 221 | 181 | 40 | 55,953 | Manager/session lifecycle, prompt pumps, managed input, persistence, querying, streams, sandbox, hooks, crash bundles | Highest-risk area: bounded shutdown, task ownership, durable integer width, and decomposition of lifecycle coordinators while preserving Manager authority. |
| `internal/session/inputqueue` | 1 | 1 | 0 | 354 | Queue admission, steer staging, generations, identifiers | Preserve atomic store admission and `int64` generations; stop silently downgrading identifier entropy on `crypto/rand` failure. |
| `internal/sessions/ledger` | 2 | 1 | 1 | 979 | Session ledger materialization, event store cleanup, deterministic ordering | Strong close-error joining and stable ordering; only small `cmp.Or`/helper simplification is justified. |
| `internal/transcript` | 23 | 20 | 3 | 5,637 | Canonical event codec, transcript assembly/pruning, UI projection, redaction | Canonical bytes and whitespace are contracts; remove a production helper used only by tests and modernize sorting carefully. |
| `internal/workspace` | 25 | 17 | 8 | 6,928 | Registry CRUD, resolution/cache, discovery, identity, config snapshots, benchmarks | Move deep config-clone ownership to the config domain and consolidate path canonicalization; keep defensive snapshots and symlink semantics. |
| `internal/workspaceaccess` | 6 | 5 | 1 | 890 | Cross-workspace authorization, consent, audit | Small and cohesive. Preserve fail-closed decisions and the explicit non-fatal audit-emission policy. |
| `internal/filesnap` | 3 | 1 | 2 | 252 | Filesystem snapshots, equality, cloning, benchmarks | Safe target for `maps.EqualFunc`/`maps.Clone`, with the existing non-nil-empty clone contract retained. |
| `internal/clientstate` | 19 | 15 | 4 | 3,778 | Per-workspace sequencing, bbolt state, atomic apply/watch, purge recovery, metrics | Strong isolation model; make open-time recovery cancellable and retain explicit sequencer/subscription ownership. |

### Priority order

1. **Lifecycle correctness:** findings R1-R5. These are ownership fixes, not stylistic modernization.
2. **Durable and alias safety:** findings R6-R8 and R14.
3. **Test correctness and determinism:** findings R9-R11.
4. **Boundary and structural consolidation:** findings R12-R17.
5. **Mechanical Go 1.26 modernization:** apply only the `adopt` rows in the feature matrix, each within the existing canonical suite.

### Business invariants that refactors must preserve

- Session Manager remains the authoritative transition/lifecycle owner; extracted collaborators must not gain back-pointers that can independently mutate Manager state (`internal/session/manager_types.go:109`, `internal/session/manager_shutdown.go:15`).
- A detached prompt may outlive an HTTP/UDS request, but response delivery remains request-bound; lifecycle contexts intentionally detach cancellation while preserving values (`internal/session/manager_prompt_submit.go:128`, `internal/session/manager_hooks.go:201`).
- Session events are published only after durable append and enrichment (`internal/session/manager_prompt_event_storage.go:82`, `internal/session/manager_prompt_event_storage.go:122`).
- Workspace and client-state reads return isolated snapshots, not mutable cache/store aliases (`internal/workspace/resolver_crud.go:194`, `internal/clientstate/service.go:111`).
- Workspace identity and containment honor canonical roots and symlink resolution; lexical prefix checks alone are insufficient (`internal/session/path_containment.go:10`, `internal/workspace/helpers.go:26`).
- ACP stop/kill behavior preserves process-group semantics on Unix and Windows through the process utility owner (`internal/acp/process_tree_unix.go:29`, `internal/acp/process_tree_windows.go:29`).
- Transcript canonical JSON, ordering, redaction, whitespace, and nil/empty distinctions are runtime contracts, not formatting preferences (`internal/transcript/canonical_payload.go:11`, `internal/transcript/agent_event_codec.go:20`).
- Client-state commits are workspace-scoped, serialized per workspace, and published only after the store commit; slow consumers may be evicted without stalling writers (`internal/clientstate/service.go:133`, `internal/clientstate/hub.go:68`).
- Workspace-access authorization remains fail-closed while audit emission remains deliberately best-effort (`internal/workspaceaccess/default_policy.go:43`).

## Mechanisms / Patterns

### Current lifecycle model

The session layer has the right conceptual owner but three different task-accounting mechanisms: a context-aware `promptDrains` registry, `compactionWG`, and `processWatchWG`, plus `startWG` for synchronous start operations (`internal/session/manager_types.go:115`). The first is the most transferable: `trackPromptDrain` registers a per-task completion channel and `WaitForPromptDrains` waits with a caller context (`internal/session/manager.go:173`, `internal/session/manager.go:203`). The WaitGroup-based paths either wait without context or manufacture an extra waiter goroutine. The model should converge on explicit run handles/done channels for cancellable joins; `startWG` should remain manual because it counts cross-call operations rather than goroutines.

ACP has process lifetime (`processCtx`, `done`) but no owner-level child-task group (`internal/acp/agent_process.go:18`). Process exit cancels the process context, closes terminals, and closes `done`, while steer dispatch and prompt notifier/reporter goroutines can still be running (`internal/acp/client_process.go:22`, `internal/acp/handlers_steer.go:44`, `internal/acp/client_prompt.go:84`). The missing mechanism is not another global goroutine helper; it is an `AgentProcess`-owned task group closed to admission during exit and joined before `done` closes.

Client state is a positive counterexample. Each workspace gets a sequencer with explicit `stop` and `done`; `shutdown` closes once and joins (`internal/clientstate/sequencer.go:55`). Watch subscriptions also have an explicit `done` and are removed by either caller context or subscription closure (`internal/clientstate/service.go:185`, `internal/clientstate/subscription.go:46`).

### Snapshot and storage model

Workspace resolution, session info, transcripts, ledger materialization, and client state deliberately materialize snapshots. That choice protects lock duration, stable ordering, close/error handling, and alias isolation. It is why public `iter.Seq` conversion is a mismatch: a lazy iterator could retain a lock/resource, move an error outside the call boundary, or expose live mutable state. Internal sequence helpers are acceptable only when they are consumed entirely inside the same call.

Durable event and state paths are strong. `recordEventWithAuthoredText` persists before publishing, `appendDurableSessionEvent` retries only the explicit closed-recorder race, the ledger joins deferred close errors, and client-state `Apply` publishes only the committed batch (`internal/session/manager_prompt_event_storage.go:86`, `internal/session/manager_durable_event.go:16`, `internal/sessions/ledger/materializer.go:145`, `internal/clientstate/service.go:134`). Refactors should make those ordering rules more visible, not abstract them behind a generic repository that obscures the commit boundary.

### Go 1.26 / review feature matrix (20 items)

| # | Feature | Status | Assessment and evidence |
| ---: | --- | --- | --- |
| 1 | `errors.AsType` | `adopt` | The slice already uses it in ACP/session (`internal/acp/failure.go:51`, `internal/session/resume_repair.go:362`) but retains verbose `var target; errors.As` sites (`internal/acp/client.go:191`, `internal/acp/handlers_helpers.go:67`, `internal/session/manager_managed_input_submit.go:211`). Convert mechanically while preserving explicit typed-nil guards. **Severity:** low; **confidence:** high; **Fowler:** Substitute Algorithm. |
| 2 | `testing.B.Loop` | `adopt` | Almost all benchmarks already use `b.Loop`; two loops remain in `BenchmarkResolverResolve` (`internal/workspace/perf_bench_test.go:85`, `internal/workspace/perf_bench_test.go:100`). Replace those loops and re-evaluate whether manual timer/sink code is still necessary. **Severity:** low; **confidence:** high; **Fowler:** Substitute Algorithm. |
| 3 | JSON `omitzero` | `not applicable` | The 212 scoped `omitempty` tags are predominantly pointers, slices/maps, strings, numbers, and named string enums. The visible value fields (`PromptStopReason`, `State`, `Speed`) have the same zero representation, while canonical/stored JSON is a byte-level contract (`internal/transcript/canonical_payload.go:30`, `internal/session/crash_bundle.go:33`, `internal/acp/prompt_runtime.go:11`). No current struct-zero omission defect justifies churn. **Severity:** contract risk if applied blindly; **confidence:** high; **Fowler:** Introduce Assertion before any future tag change. |
| 4 | `os.OpenRoot` | `defer` | The slice canonicalizes identities and containment but most actual relative file-loading ownership crosses into other packages. A partial conversion would create two security models. First consolidate canonical path policy (`internal/session/path_containment.go:10`, `internal/acp/client_process.go:201`, `internal/workspace/helpers.go:26`), then adopt `OpenRoot` atomically at each user-controlled open boundary. **Severity:** medium; **confidence:** high; **Fowler:** Move Function, then Substitute Algorithm. |
| 5 | `strings` sequence iterators | `adopt` | `SplitSeq` is already used for goal parsing (`internal/session/goal_command.go:232`). `normalizeAutomaticSessionTitle` only needs the first eight words plus truncation detection but materializes every word (`internal/session/session_title.go:88`); `FieldsSeq` can bound work without changing output. Keep materialized `Split` where indexing/line counts are the contract (`internal/acp/handlers_helpers.go:105`, `internal/transcript/prune.go:140`). **Severity:** low; **confidence:** high; **Fowler:** Replace Loop with Pipeline. |
| 6 | `sync.WaitGroup.Go` | `adopt` | Tests already use it. Production compaction and process watchers manually pair `Add`/`Done` around spawned goroutines (`internal/session/compaction.go:114`, `internal/session/manager_process_watchers.go:25`, `internal/session/manager_lifecycle.go:76`). Convert those after moving cleanup into the closure. Do not convert `startWG`: it counts synchronous operations begun in one call and finished in another (`internal/session/manager_start_run.go:23`). **Severity:** medium; **confidence:** high; **Fowler:** Substitute Algorithm. |
| 7 | Range over integers | `already` | Production retry loops use `for attempt := range 2` (`internal/session/query.go:141`, `internal/session/manager_durable_event.go:21`). No remaining production index loop is a clear integer-range modernization; the only `b.N` loops are benchmark item 2. **Severity:** none; **confidence:** high; **Fowler:** none. |
| 8 | `slices`, `maps`, built-in `min`/`max` | `adopt` | Existing use is good (`internal/session/repair_actions.go:88`, `internal/workspace/clone.go:112`, `internal/sessions/ledger/materializer.go:289`). Replace remaining `sort.Strings`/`sort.Slice(Stable)` with `slices.Sort`/`SortFunc`/`SortStableFunc` (`internal/acp/negotiation_error.go:107`, `internal/transcript/transcript.go:112`, `internal/clientstate/store.go:379`, `internal/session/catalog_page.go:118`). `filesnap.Equal`/`Clone` can use `maps.EqualFunc`/`maps.Clone`, explicitly preserving non-nil empty output (`internal/filesnap/filesnap.go:30`). **Severity:** low; **confidence:** high; **Fowler:** Substitute Algorithm. |
| 9 | `testing/synctest` | `adopt` | Use narrowly for pure in-process goroutine/time invariants such as the backpressured prompt test (`internal/acp/types_test.go:210`) and selected fake-Manager polling cases (`internal/session/manager_test.go:5754`). Do not use it for real OS process polling (`internal/acp/process_tree_test.go:135`, `internal/session/manager_stop_integration_test.go:755`). **Severity:** medium (flake reduction); **confidence:** high; **Fowler:** Substitute Algorithm. |
| 10 | `iter.Seq` APIs | `reject` | Current list/query calls return stable defensive snapshots after releasing locks and closing recorders (`internal/session/manager.go:244`, `internal/workspace/resolver_crud.go:194`, `internal/clientstate/service.go:111`, `internal/session/query.go:135`). Lazy public iteration would weaken snapshot, cleanup, and error contracts with no demonstrated allocation problem. **Severity:** high if applied publicly; **confidence:** high; **Fowler:** none—preserve the current API. |
| 11 | `os.Process.WithHandle` | `defer` | ACP delegates group signaling to `internal/procutil` through identical platform wrappers (`internal/acp/process_tree_unix.go:29`, `internal/acp/process_tree_windows.go:29`). Handle ownership belongs in that outside-slice process utility and must retain tree/group semantics. **Severity:** medium; **confidence:** high; **Fowler:** Move Function. |
| 12 | `sync.OnceFunc` / `OnceValue(s)` | `adopt` | Local idempotent callbacks are direct `OnceFunc` targets: start-run release, stream unsubscribe, and catalog unsubscribe (`internal/session/manager_start_run.go:99`, `internal/session/session_stream_broadcast.go:83`, `internal/session/session_catalog_stream.go:58`). Stateful field-level `sync.Once` values that also publish errors/state should remain explicit. No clear `OnceValue(s)` cache exists. **Severity:** low; **confidence:** high; **Fowler:** Substitute Algorithm. |
| 13 | `math/rand/v2` | `not applicable` | All scoped randomness is `crypto/rand` for durable/unique identifiers (`internal/workspace/identity.go:44`, `internal/session/manager_helpers.go:230`, `internal/session/inputqueue/queue.go:332`). Replacing it with pseudo-randomness would weaken the contract. Fix entropy error propagation instead (R14). **Severity:** high if substituted; **confidence:** high; **Fowler:** none. |
| 14 | `cmp.Or` | `adopt` | Adopt only where candidates are already normalized, e.g. the ledger's two pre-trimmable session IDs (`internal/sessions/ledger/materializer.go:271`). Do not replace `firstNonEmpty` variants blindly: some return trimmed text while transcript intentionally returns original whitespace-preserving text (`internal/transcript/transcript_payload.go:24`, `internal/acp/tool_gateway.go:206`). Consolidate exact duplicates per package. **Severity:** low; **confidence:** high; **Fowler:** Consolidate Duplicate Conditional Fragments / Extract Function. |
| 15 | `testing.T.ArtifactDir`, `Attr`, `Output` | `defer` | Integration tests create diagnostic/capture files under `t.TempDir` (`internal/acp/client_test.go:48`, `internal/session/manager_stop_integration_test.go:152`). Ephemeral protocol inputs should stay temporary. Use artifact retention/attributes only after the CI evidence consumer and redaction/retention policy are defined. **Severity:** low; **confidence:** medium; **Fowler:** Move Function. |
| 16 | `runtime/trace.FlightRecorder` | `defer` | Session crash bundles are a natural attachment point (`internal/session/crash_bundle.go:49`, `internal/session/crash_bundle.go:103`), but recorder lifetime, memory budget, privacy, and daemon ownership are outside this slice. Design one daemon-level recorder and attach a bounded trace fragment rather than one recorder per session. **Severity:** medium; **confidence:** high; **Fowler:** Extract Class. |
| 17 | `http.CrossOriginProtection` | `not applicable` | No scoped production package imports `net/http`; ACP handlers are JSON-RPC callbacks (`internal/acp/handlers.go:74`). This belongs to the HTTP/API slice. **Severity:** none; **confidence:** high; **Fowler:** none. |
| 18 | Typed `DialUnix` / `DialTCP` | `not applicable` | No scoped code calls `net.Dial`, `Dialer.Dial`, or constructs Unix/TCP addresses. ACP transport is stdio through the sandbox handle and SDK connection (`internal/acp/start_process.go:64`, `internal/acp/start_process.go:74`). **Severity:** none; **confidence:** high; **Fowler:** none. |
| 19 | `bytes.Buffer.Peek` | `not applicable` | Scoped buffers build complete payloads or collect logs; there is no `bufio.Reader`/incremental frame parser to inspect without consuming. ACP framing is owned by the SDK connection (`internal/sessions/ledger/materializer.go:248`, `internal/acp/tool_gateway.go:81`). **Severity:** none; **confidence:** high; **Fowler:** none. |
| 20 | `unique.Make` / handles | `defer` | Repeated session/workspace/event IDs are durable or wire-visible strings. Replacing them with handles would add conversion and API complexity; no heap profile shows retention dominated by duplicate strings (`internal/clientstate/service.go:37`, `internal/session/session_stream_broadcast.go:23`). Consider only for internal, non-serialized keys after profiling. **Severity:** low; **confidence:** medium; **Fowler:** Replace Primitive with Object only if evidence supports it. |

## Relevant Sources

| Area | Primary sources read | What they establish |
| --- | --- | --- |
| ACP process lifecycle | `internal/acp/agent_process.go:18`, `internal/acp/start_process.go:48`, `internal/acp/client_process.go:22`, `internal/acp/client_control.go:166` | Process context, process exit, done closure, stop escalation, and current absence of a child-task join group. |
| ACP asynchronous handlers | `internal/acp/handlers_steer.go:22`, `internal/acp/client_prompt.go:76`, `internal/acp/handlers_session_state.go:11` | Fire-and-forget steer and prompt notifier/reporter tasks; background fallback at inbound notification boundary. |
| ACP process tree / terminals | `internal/acp/process_tree_unix.go:13`, `internal/acp/process_tree_windows.go:13`, `internal/acp/terminal_process.go:127` | Duplicated platform wrappers, delegated group signaling, and terminal-specific completion state. |
| Session Manager ownership | `internal/session/manager_types.go:109`, `internal/session/manager.go:173`, `internal/session/manager_shutdown.go:15` | Manager state clusters, prompt-drain registry, and shutdown order. |
| Session watcher/compaction ownership | `internal/session/manager_lifecycle.go:63`, `internal/session/manager_process_watchers.go:10`, `internal/session/compaction.go:114`, `internal/session/compaction_lifecycle.go:84` | WaitGroup admission, goroutine launch, cancellation, and context-insensitive/adapted waits. |
| Prompt and managed-input pumps | `internal/session/manager_prompt_submit.go:163`, `internal/session/manager_managed_input_submit.go:63`, `internal/session/synthetic_prompt.go:224`, `internal/session/manager_input_dispatch.go:190` | Both tracked and untracked drain/forward goroutines and the durable generation conversion. |
| Session persistence and streams | `internal/session/manager_prompt_event_storage.go:82`, `internal/session/manager_durable_event.go:16`, `internal/session/session_stream_broadcast.go:45`, `internal/session/session_catalog_stream.go:45` | Persist-before-publish invariant, idempotent append, snapshot/wake streams, and idempotent unsubscribe closures. |
| Session diagnostics/query | `internal/session/crash_bundle.go:49`, `internal/session/query.go:135`, `internal/session/health_hooks.go:90`, `internal/session/manager_prompt_runtime_loop.go:129` | Crash bundle attachment, recorder cleanup boundaries, and fallback contexts. |
| Input queue | `internal/session/inputqueue/queue.go:18`, `internal/session/inputqueue/queue.go:96`, `internal/session/inputqueue/queue.go:330` | Store-owned admission/generation semantics and identifier generation. |
| Ledger | `internal/sessions/ledger/materializer.go:145`, `internal/sessions/ledger/materializer.go:248`, `internal/sessions/ledger/materializer.go:287` | Close-error joining, deterministic payload construction, stable event ordering. |
| Transcript | `internal/transcript/transcript.go:91`, `internal/transcript/prune.go:27`, `internal/transcript/agent_event_codec.go:20`, `internal/transcript/transcript_payload.go:24` | Snapshot assembly, pruning, canonical codec, and whitespace-sensitive fallback helpers. |
| Workspace resolution/cache | `internal/workspace/resolver.go:36`, `internal/workspace/resolver_crud.go:194`, `internal/workspace/clone.go:13`, `internal/workspace/discovery.go:87` | Resolver locking, reconciliation, deep snapshots, and path containment. |
| Workspace identity | `internal/workspace/helpers.go:26`, `internal/workspace/identity.go:42` | Canonical roots and cryptographic durable workspace ULIDs. |
| Workspace access | `internal/workspaceaccess/default_policy.go:43`, `internal/workspaceaccess/policy.go:22` | Fail-closed authorization and best-effort audit contract. |
| File snapshots | `internal/filesnap/filesnap.go:17` | Small equality/clone surface and nil/empty behavior. |
| Client state | `internal/clientstate/service.go:46`, `internal/clientstate/sequencer.go:14`, `internal/clientstate/subscription.go:5`, `internal/clientstate/hub.go:68` | Open-time recovery, workspace-scoped serialization, subscription lifetime, commit publication, and slow-consumer handling. |

## Transferable Patterns

### Patterns to preserve and reuse

1. **Context-aware task registration.** `trackPromptDrain` registers before launch, returns an idempotent completion closure, and allows `WaitForPromptDrains` to select on both each task and shutdown context (`internal/session/manager.go:173`). Generalize this shape into small owner-specific run registries for compactions/process watchers rather than a generic global goroutine registry.

2. **Commit, then publish.** Session event publication follows successful persistence, and client state publishes the exact committed entries (`internal/session/manager_prompt_event_storage.go:112`, `internal/clientstate/service.go:154`). Any extracted service must expose one operation with this ordering; splitting persistence and notification into separately callable public methods would make invalid ordering representable.

3. **Snapshot under lock, transform outside.** Manager copies session pointers under its lock, then builds/sorts `Info` snapshots outside it (`internal/session/manager.go:244`). Workspace and client state return cloned aggregates (`internal/workspace/resolver_crud.go:211`, `internal/clientstate/service.go:130`). Reuse this for capability/config state after encapsulating writable fields.

4. **Prepare/commit/rollback ownership.** Workspace CRUD and client-state purge isolate reversible preparation from durable commit (`internal/workspace/resolver_crud.go:13`, `internal/clientstate/service.go:227`). This is the correct pattern for future cross-store operations because cleanup errors can be joined rather than erased.

5. **Consumer-owned narrow interfaces.** `workspaceaccess` declares the exact mode, consent, and audit dependencies it consumes (`internal/workspaceaccess/default_policy_contracts.go:18`). Session uses capability interfaces at optional boundaries. Preserve compile-time assertions where concrete ownership is stable; do not create umbrella interfaces solely to reduce constructor arguments.

6. **Typed failure normalization.** ACP maps wrapped SDK/provider errors to bounded, redacted session failures and already uses `errors.AsType` in the main paths (`internal/acp/failure.go:48`, `internal/acp/provider_failure.go:95`). Extend the same typed identity to semantic errors currently tested only by strings.

7. **Stable deterministic ordering.** Ledger/transcript/session lists copy before stable sorting (`internal/sessions/ledger/materializer.go:287`, `internal/transcript/transcript.go:110`, `internal/session/manager.go:252`). `slices.SortFunc` modernization must retain every tie-breaker and stability choice.

8. **Explicit best-effort boundary.** Workspace access intentionally logs audit transport failure while returning the authorization result (`internal/workspaceaccess/default_policy.go:43`). This is a documented business boundary and should not be misclassified as accidental log-and-return behavior.

### Canonical test ownership for proposed work

| Invariant | Owning layer | Canonical suite to extend | Test shape |
| --- | --- | --- | --- |
| Manager shutdown returns on deadline even when process-exit finalization is blocked | Session lifecycle integration | `internal/session/manager_stop_integration_test.go:85` for real process; `internal/session/manager_test.go:2180` for fake blocked finalizer | Reuse existing suites; prove deadline error and eventual cleanup, do not add a file-existence/static test. |
| ACP process `Done` closes only after owned steer/notifier tasks stop | ACP process lifecycle | `internal/acp/handlers_steer_test.go:1` and existing process lifecycle cases in `internal/acp/client_test.go:250` | Synchronize with channels or `synctest` for pure tasks; assert no post-`Done` event/finalizer call. |
| Managed input generation preserves all persisted values | Session managed-input mapping | `internal/session/manager_busy_input_test.go:1` | Table boundary values around 32-bit and native-int limits; production type should remain `int64`, not merely reject large values. |
| Resolver snapshots cannot alias configuration/cache state | Workspace resolver/cache | `internal/workspace/clone_test.go:1` and `internal/workspace/resolver_cache_contract_test.go:1` | Mutate returned nested collections and prove subsequent resolves unchanged; config domain should own field-completeness tests. |
| Client-state open recovery is cancellable and closes store on failure | Client-state engine | `internal/clientstate/store_service_test.go:1` | Inject a blocked recovery/store boundary, cancel context, and assert error plus closed resource. |
| `filesnap.Clone(nil)` remains a non-nil empty map and clone mutations isolate | File snapshot value helper | `internal/filesnap/filesnap_test.go:97` | Extend the existing invariant when moving to `maps.Clone`; no duplicate regression suite. |
| Transcript JSON remains byte-for-byte canonical after mechanical helpers/tags change | Transcript codec | `internal/transcript/transcript_test.go:1020` | Round-trip/canonical payload behavior; do not snapshot generated prose or implementation-only structs. |

## Risks / Mismatches

### Refactoring and correctness findings

| ID | Class | Finding and required direction | Evidence | Severity | Confidence | Fowler technique |
| --- | --- | --- | --- | --- | --- | --- |
| R1 | Lifecycle / business invariant | `shutdownProcessWatchers` ignores the caller's shutdown context and performs a naked `Wait`. Cancellation normally releases watchers, but a watcher that wins `proc.Done` can enter `handleProcessExit(context.WithoutCancel(...))`; that persistence/finalization path can keep shutdown blocked past its deadline. Accept `ctx`, close admission, cancel, and join through context-aware run handles. Preserve complete process-exit finalization when the deadline does not expire. | `internal/session/manager_shutdown.go:18`, `internal/session/manager_process_watchers.go:30`, `internal/session/manager_lifecycle.go:76` | high | high | Change Function Declaration; Encapsulate Variable; Extract Class |
| R2 | Concurrency / mechanical architecture | `waitForCompactions` launches an extra goroutine solely to adapt `WaitGroup.Wait` to a context. On timeout that waiter remains until every compaction exits. Replace the WaitGroup-only accounting with per-run done handles/registry (the prompt-drain pattern), then use `WaitGroup.Go` only for launch mechanics if still useful. | `internal/session/compaction_lifecycle.go:84`, `internal/session/compaction.go:114` | medium | high | Substitute Algorithm; Extract Class |
| R3 | ACP lifecycle / business invariant | Steer dispatch, cancellation notification, and activity reporting are not joined by `AgentProcess`; process exit can cancel the context and close `done` while those tasks are still finalizing or emitting. Add an `AgentProcess` child-task group with admission closure and join it before `close(p.done)`. Replace the cancellation-wait goroutine with `context.AfterFunc` only if a callback-started acknowledgement is also joined. | `internal/acp/handlers_steer.go:44`, `internal/acp/client_prompt.go:84`, `internal/acp/client_prompt.go:183`, `internal/acp/client_process.go:59` | high | high | Extract Class; Move Function |
| R4 | Session lifecycle / business invariant | Some prompt drains/forwards bypass the existing Manager tracker (`go drainPromptSource` in prompt failure and managed input; synthetic forwarding has its own goroutine), while the main prompt pump and queue drain are tracked. Route every Manager-owned pump/drain/forward task through one admission/join mechanism so shutdown cannot return with a producer still touching session state. | `internal/session/manager_prompt_submit.go:163`, `internal/session/manager_managed_input_submit.go:135`, `internal/session/synthetic_prompt.go:235`, `internal/session/manager_input_dispatch.go:190` | high | high | Move Function; Consolidate Duplicate Conditional Fragments |
| R5 | Context ownership | ACP process lifetime starts from `context.Background`, losing request values/tracing; client-state open-time purge recovery performs I/O with an uncancellable background context; two session fallbacks use `context.TODO` even though Manager normally owns a lifecycle context. Use `context.WithoutCancel(ctx)` for deliberately detached lifetimes, add `ctx` to `clientstate.Open`, and make a missing Manager lifecycle context an explicit invariant/error rather than TODO. | `internal/acp/start_process.go:48`, `internal/clientstate/service.go:46`, `internal/clientstate/service.go:78`, `internal/session/health_hooks.go:90`, `internal/session/manager_prompt_runtime_loop.go:129` | medium | high | Change Function Declaration; Introduce Assertion |
| R6 | Numeric / durable contract | Persisted `RunGeneration` is `*int64` but `ManagedInputOwner.RunGeneration` is `int`; the unchecked conversion can truncate on 32-bit targets and creates two widths for one durable identity. Make generation `int64` end-to-end across owner, prompt metadata, lifecycle adapters, and tests. Do not add a compatibility clamp. | `internal/session/manager_managed_input_submit.go:35`, `internal/session/manager_managed_input_submit.go:143`, `internal/session/managed_input_lifecycle.go:13` | high | high | Change Function Declaration; Replace Primitive with Object |
| R7 | Alias safety / dependency direction | Workspace owns roughly 250 lines of field-by-field deep cloning for `config.Config` and nested config types. Every config-field addition creates shotgun surgery and a missed field can leak mutable cache state. Move the complete deep-clone operation to the config domain; workspace should clone only `Workspace`/`ResolvedWorkspace` aggregates by calling that owner. | `internal/workspace/clone.go:58`, `internal/workspace/clone.go:109`, `internal/workspace/clone.go:301` | high | high | Move Function; Encapsulate Collection |
| R8 | Concurrency / alias escape | `acp.AgentProcess.Caps` is exported and mutable while internal writes are protected by `capsMu`; external mutation can bypass the mutex and race with `CapsSnapshot`. The session wrapper repeats a public `Caps` field alongside a snapshot callback. Make mutable capability state private and expose cloned snapshots; options may accept initial values without retaining aliases. | `internal/acp/agent_process.go:18`, `internal/acp/agent_process.go:119`, `internal/session/interfaces.go:130`, `internal/session/interfaces.go:254` | high | high | Encapsulate Variable; Encapsulate Collection |
| R9 | Test correctness / repository rule | At least 88 scoped cleanup calls explicitly discard errors from `Stop`, `Close`, `Kill`, or `Chmod`, and helper-process diagnostics also discard `fmt.Fprintf` errors. This violates the project rule for production and tests and can hide leaked processes/locked databases. Centralize cleanup in `t.Cleanup` helpers that fail the test except for explicitly classified already-stopped/not-exist cases. | `internal/acp/client_test.go:52`, `internal/acp/client_test.go:98`, `internal/session/manager_test.go:140`, `internal/session/manager_test.go:2252`, `internal/workspace/identity_test.go:109` | high | high | Extract Function; Move Statements into Function |
| R10 | Test structure | Test files are navigation bottlenecks: `manager_test.go` is 6,744 lines, `client_test.go` 3,205, `resolver_test.go` 2,749, `manager_hooks_test.go` 2,233, and `transcript_test.go` 2,139. Split existing canonical suites by behavioral responsibility while sharing fixtures; do not create duplicate standalone regressions. Several subtest names also omit the required `"Should …"` form. | `internal/session/manager_test.go:194`, `internal/acp/client_test.go:260`, `internal/workspace/resolver_test.go:120`, `internal/transcript/transcript_test.go:1422`, `internal/acp/client_test.go:611` | medium | high | Extract Class / Extract Module; Move Function |
| R11 | Test determinism | A shared 10-second/10-millisecond polling helper drives many fake Manager concurrency tests; a pure ACP backpressure test also polls with sleep. These are appropriate `synctest` or explicit-channel targets. Real subprocess PID/exit polling is outside the synctest bubble and should retain deadline-based eventual checks. | `internal/session/manager_test.go:5754`, `internal/acp/types_test.go:210`, `internal/acp/process_tree_test.go:135`, `internal/session/manager_stop_integration_test.go:755` | medium | high | Substitute Algorithm |
| R12 | Duplication / security boundary | Directory canonicalization and containment are reimplemented in session, ACP, and workspace with slightly different error vocabularies and symlink steps. Extract one mechanical canonical-directory primitive into the filesystem utility owner, keep domain wrappers for error text/types, and later introduce `os.OpenRoot` only at complete open boundaries. | `internal/session/path_containment.go:10`, `internal/acp/client_process.go:201`, `internal/workspace/helpers.go:26`, `internal/workspace/discovery.go:87` | medium | high | Extract Function; Move Function |
| R13 | Duplication / subprocess architecture | ACP agent and terminal wait paths independently implement wait, group-exit handling, process-record completion, state publication, and `done` closure. Unix and Windows ACP process-tree files are byte-equivalent apart from build tags even though real platform behavior is already delegated. Extract a process-lifecycle supervisor in the process utility owner with agent/terminal-specific completion callbacks; remove redundant ACP platform wrappers. | `internal/acp/client_process.go:22`, `internal/acp/terminal_process.go:127`, `internal/acp/process_tree_unix.go:13`, `internal/acp/process_tree_windows.go:13` | medium | high | Form Template Method; Move Function; Remove Dead Code |
| R14 | Identifier safety | Session and inputqueue ID generators silently fall back from `crypto/rand` to timestamps. Under entropy failure concurrent calls can collide, and callers cannot observe the degradation. Change ID generation to return `(string, error)` and propagate failure through creation/admission. Keep `crypto/rand`; `rand/v2` is not a substitute. | `internal/session/manager_helpers.go:228`, `internal/session/inputqueue/queue.go:330` | medium | high | Change Function Declaration |
| R15 | Dead code / test coupling | Production `transcript.canonicalPayload` is referenced only by a test helper; the runtime codec constructs `canonicalEventPayload` through the actual marshal path. Remove the production-only-for-test helper and rewrite the test against the public/runtime codec so it protects behavior rather than an implementation seam. | `internal/transcript/transcript_payload.go:77`, `internal/transcript/transcript_test.go:2124`, `internal/transcript/agent_event_codec.go:20` | low | high | Remove Dead Code |
| R16 | Error contracts | Several tests determine semantic failure only with `strings.Contains(err.Error(), ...)`, including workspace access, client-state store format, ACP validation, and provider lifecycle. For machine-significant distinctions, introduce or reuse sentinel/typed error identity and assert `errors.Is`/`errors.AsType`; retain string assertions only when exact public wording is itself the contract. | `internal/workspaceaccess/default_policy_test.go:563`, `internal/clientstate/store_service_test.go:642`, `internal/acp/types_test.go:488`, `internal/session/provider_lifecycle_test.go:207` | medium | medium | Replace Primitive with Object; Introduce Assertion |
| R17 | Large class / bounded extraction | `Manager` aggregates lifecycle coordination for starts, compactions, process watchers, prompt drains, managed input, health, and broadcasters. It is an architectural owner, so a wholesale split would damage transition authority. Extract only cohesive state holders (`startRunRegistry`, `processWatcherGroup`, `compactionCoordinator`) that expose commands/results and never retain a Manager back-pointer. The largest production files are under the 500-line cap but `resolver_crud.go` (495), `manager_busy_input.go` (490), and `manager_clear.go` (478) must not grow. | `internal/session/manager_types.go:109`, `internal/session/manager_start_run.go:23`, `internal/session/manager_process_watchers.go:10`, `internal/workspace/resolver_crud.go:1` | medium | medium | Extract Class; Encapsulate Variable |
| R18 | Duplicated fallback semantics | ACP contains two identical trim-and-first-nonblank helpers, and session contains another pair; transcript has a deliberately different helper that returns original text. Consolidate exact duplicates within each package and use `cmp.Or` only on already-trimmed values. Do not centralize across packages or erase the transcript whitespace invariant. | `internal/acp/failure.go:205`, `internal/acp/tool_gateway.go:206`, `internal/session/failure.go:107`, `internal/session/prompt_activity_conversion.go:80`, `internal/transcript/transcript_payload.go:24` | low | high | Extract Function; Consolidate Duplicate Conditional Fragments |
| R19 | Identifier safety / cross-slice correction | Fresh implementation tracing found the same silent entropy downgrade in the shared `store.NewID` API. Its string-only contract has live callers across persistence, API, CLI, network, bridges, task, session, and daemon paths, so it requires a separate repository-wide hard cut rather than being hidden inside the session change. | `internal/store/id.go:12` plus the current whole-repository `store.NewID` call graph | high | high | Change Function Declaration; Change API |
| R20 | Identifier safety / corrected call graph | The workspace `generateID` helper is not dead: it is bound as the resolver default through a function value and invoked during registration and name-collision retry. It has the same timestamp fallback and must become an error-returning resolver dependency; deleting it would break workspace creation. | `internal/workspace/options.go:95`, `internal/workspace/options.go:110`, `internal/workspace/resolver_crud.go:255`, `internal/workspace/resolver_crud.go:278` | medium | high | Change Function Declaration |
| R21 | Identifier safety / panic contract | Durable workspace identity creation uses `ulid.MustNew` with `crypto/rand.Reader`. Entropy failure is an ordinary I/O failure, not an impossible invariant, but the string-only `NewWorkspaceID` and injected `ensureIdentity` generator force it into a panic; GlobalDB workspace insertion also calls the same API. | `internal/workspace/identity.go:43`, `internal/workspace/identity.go:49`, `internal/workspace/identity.go:56`, `internal/store/globaldb/global_db_workspace.go:233` | high | high | Change Function Declaration |

### Performance observations

- The slice has real benchmarks for ACP handlers/process output, session list/sandbox dispatch, workspace resolve/list/clone, transcript codecs, and file snapshots (`internal/acp/acp_bench_test.go:16`, `internal/session/perf_bench_test.go:20`, `internal/workspace/perf_bench_test.go:78`, `internal/transcript/transcript_bench_test.go:12`, `internal/filesnap/filesnap_bench_test.go:11`). Preserve them and convert the last `b.N` loops before comparing changes.
- Candidate hot paths are `Manager.ListAll` (clone + O(n log n) sort), resolver cache cloning (deep config clone), transcript canonical sort/marshal, and client-state hub publication under a hub lock. None justifies pooling, interning, iterator APIs, or lock redesign without benchmark/profile evidence.
- `sync.Pool`, `unsafe`, and cross-request object reuse are not indicated. The state carried by these packages is alias-sensitive and correctness-dominant.

### Compozy Impact Audit (analysis only)

- **Native tools:** No implementation change. Checked ACP tool gateway/handlers and session event persistence; any future lifecycle refactor must retain tool call/result ordering, descriptor/capability snapshots, permission boundaries, and durable-before-publish behavior.
- **Extensibility and hooks:** No implementation change. Checked session hooks, ACP tool host/gateway, sandbox tool host, advertised commands, and workspace configuration snapshots. Extracted coordinators must not bypass hook dispatch or create a second registry/config lifecycle.
- **Workspace data isolation:** Central to R7/R8. Workspace resolver returns deep clones; client state sequences and publishes by `WorkspaceID`; workspace access compares actor/target workspace IDs. Future refactors must prove workspace ID propagation, cache snapshot isolation, subscription scoping, and no mutable capability/config aliases across workspaces.
- **Official Compozy skill:** No impact from this analysis-only artifact. If later changes alter public CLI/HTTP/UDS paths, tool IDs, capabilities, hooks, config, or lifecycle semantics, `skills/compozy/` must be audited in that implementation workstream.

## Open Questions

1. **Process utility ownership:** Should the follow-up scope include `internal/procutil`/`internal/subprocess` so ACP agent and terminal supervision plus `Process.WithHandle` can be designed atomically without weakening process-group behavior?
2. **Config clone owner:** Does the config package already have an authoritative deep-clone contract outside this slice, or should the refactor introduce one and migrate every consumer in the same hard cut?
3. **Lifecycle deadline semantics:** When Manager shutdown reaches its deadline during process-exit finalization, should it return immediately while an explicitly daemon-owned finalizer continues, or must finalization be aborted and recovered on next boot? The current code implicitly chooses “wait forever” on the narrow race path.
4. **Managed-input contract width:** Which outside-slice API/store/codegen surfaces also expose `RunGeneration` as `int`? The change should be an atomic `int64` hard cut across all of them.
5. **Diagnostic retention:** Is there an existing CI consumer and redaction policy for `T.ArtifactDir`/`T.Attr`/`T.Output`? Without one, moving transient provider captures to durable artifacts risks retaining secrets.
6. **Flight recorder ownership:** Which daemon diagnostics component should own one bounded `runtime/trace.FlightRecorder`, and what memory/privacy budget permits attaching trace fragments to session crash bundles?
7. **Filesystem capability boundary:** Which outside-slice package performs the final user-controlled reads/writes after workspace/ACP path resolution? `os.OpenRoot` should be introduced there together with the canonicalization refactor, not in only one caller.
8. **Error identity scope:** Which error messages are public CLI/API contracts versus internal diagnostics? That classification is needed before replacing string-only assertions with typed identities while retaining user-facing copy tests where justified.

## Evidence

All citations below are existing readable repository paths; entries are deduplicated by path.

- `go.mod:3` — Go language baseline.
- `.golangci.yml:26` — `modernize` analyzer enabled.
- `internal/acp/acp_bench_test.go:16` — ACP benchmark suite using `b.Loop`.
- `internal/acp/agent_process.go:18` — ACP process state, capability lock, and done channel.
- `internal/acp/client.go:187` — remaining verbose `errors.As` request-error classification.
- `internal/acp/client_control.go:166` — stop escalation and process wait contexts.
- `internal/acp/client_process.go:22` — agent process completion sequence; workspace path normalization at line 201.
- `internal/acp/client_prompt.go:76` — cancellation notifier; activity reporter at line 183.
- `internal/acp/client_test.go:48` — helper capture files; discarded cleanup errors at lines 52 and 98; large canonical suite.
- `internal/acp/failure.go:48` — `errors.AsType` usage and failure normalization; duplicate fallback helper at line 205.
- `internal/acp/handlers.go:30` — ACP JSON wire payload tags and handler contract.
- `internal/acp/handlers_helpers.go:67` — remaining verbose typed-error extraction; materialized line slicing at line 105.
- `internal/acp/handlers_session_state.go:11` — inbound handler background context fallback.
- `internal/acp/handlers_steer.go:22` — staged steer consumption and fire-and-forget dispatch at line 44.
- `internal/acp/handlers_steer_test.go:1` — canonical steer behavior suite.
- `internal/acp/process_tree_test.go:135` — real OS polling that is not a synctest candidate.
- `internal/acp/process_tree_unix.go:13` — Unix ACP process-tree wrapper.
- `internal/acp/process_tree_windows.go:13` — equivalent Windows ACP process-tree wrapper.
- `internal/acp/provider_failure.go:95` — existing `errors.AsType` provider classification.
- `internal/acp/start_process.go:48` — process lifetime context and stdio connection creation.
- `internal/acp/terminal_process.go:127` — terminal wait/completion lifecycle.
- `internal/acp/tool_gateway.go:81` — complete buffer construction; duplicate fallback helper at line 206.
- `internal/acp/types_test.go:210` — pure in-process backpressure polling candidate.
- `internal/clientstate/concurrency_lifecycle_test.go:26` — existing `WaitGroup.Go` test usage.
- `internal/clientstate/hub.go:68` — workspace subscription publication and slow-consumer handling.
- `internal/clientstate/sequencer.go:14` — explicit per-workspace task ownership.
- `internal/clientstate/service.go:46` — engine open/recovery; clone/list/apply/watch contracts.
- `internal/clientstate/store.go:378` — remaining `sort.Slice` candidate.
- `internal/clientstate/store_service_test.go:634` — store-format tests and string-based semantic error checks.
- `internal/clientstate/subscription.go:5` — subscription close/done ownership.
- `internal/filesnap/filesnap.go:17` — snapshot read, equality, and clone semantics.
- `internal/filesnap/filesnap_bench_test.go:11` — file snapshot benchmarks using `b.Loop`.
- `internal/filesnap/filesnap_test.go:97` — clone independence and non-nil empty-map contract.
- `internal/session/compaction.go:108` — manual WaitGroup task launch.
- `internal/session/compaction_lifecycle.go:84` — context adapter goroutine around `WaitGroup.Wait`.
- `internal/session/crash_bundle.go:49` — crash-bundle attachment and write path.
- `internal/session/failure.go:107` — session first-nonblank helper.
- `internal/session/goal_command.go:228` — existing `strings.SplitSeq` use.
- `internal/session/health_hooks.go:90` — lifecycle context TODO fallback.
- `internal/session/inputqueue/queue.go:18` — queue contract and timestamp entropy fallback at line 330.
- `internal/session/interfaces.go:130` — session process capability exposure and snapshot adapter.
- `internal/session/managed_input_lifecycle.go:13` — managed-input generation typed as `int`.
- `internal/session/manager.go:173` — context-aware prompt drain registry and session list snapshot.
- `internal/session/manager_busy_input_test.go:1` — canonical managed/busy input suite.
- `internal/session/manager_durable_event.go:16` — idempotent durable append and closed-recorder retry.
- `internal/session/manager_hooks.go:201` — Manager lifecycle-context fallback.
- `internal/session/manager_input_dispatch.go:190` — tracked queued-input drain.
- `internal/session/manager_lifecycle.go:63` — process watcher and detached exit finalization.
- `internal/session/manager_managed_input_submit.go:35` — persisted `int64` generation and unchecked conversion at line 157.
- `internal/session/manager_process_watchers.go:10` — watcher admission and context-insensitive shutdown wait.
- `internal/session/manager_prompt_event_storage.go:82` — persist/enrich before publish.
- `internal/session/manager_prompt_runtime_loop.go:129` — detached stop context TODO fallback.
- `internal/session/manager_prompt_submit.go:163` — untracked failure drain and tracked prompt pump.
- `internal/session/manager_shutdown.go:15` — Manager shutdown ordering.
- `internal/session/manager_start_run.go:23` — cross-call start operation tracking and once-only release.
- `internal/session/manager_stop_integration_test.go:85` — real process shutdown canonical suite and PID polling.
- `internal/session/manager_test.go:194` — large canonical Manager suite, cleanup discards, and polling helper at line 5754.
- `internal/session/manager_types.go:109` — Manager state/lifecycle responsibility clusters.
- `internal/session/path_containment.go:10` — session path canonicalization and containment.
- `internal/session/perf_bench_test.go:20` — session benchmarks using `b.Loop`.
- `internal/session/prompt_activity_conversion.go:80` — duplicate first-nonblank helper.
- `internal/session/query.go:135` — eager query with retry/cleanup boundary.
- `internal/session/resume_repair.go:362` — existing `errors.AsType` use.
- `internal/session/session_catalog_stream.go:45` — catalog subscription and idempotent cancellation.
- `internal/session/session_stream_broadcast.go:45` — session snapshot/wake stream and context watcher.
- `internal/session/session_title.go:88` — bounded-title candidate for `FieldsSeq`.
- `internal/session/synthetic_prompt.go:224` — synthetic prompt forwarding goroutine.
- `internal/sessions/ledger/materializer.go:145` — event-store cleanup, payload buffer, stable sort, and first-nonblank helper.
- `internal/sessions/ledger/materializer_test.go:1` — canonical ledger materialization suite.
- `internal/transcript/agent_event_codec.go:20` — actual canonical event marshal path.
- `internal/transcript/canonical_payload.go:11` — persisted canonical JSON schema/tags.
- `internal/transcript/prune.go:27` — transcript pruning and line materialization.
- `internal/transcript/projection.go:20` — projection construction and internal context creation.
- `internal/transcript/transcript.go:91` — eager transcript assembly and stable ordering.
- `internal/transcript/transcript_bench_test.go:12` — transcript codec/assembly benchmarks using `b.Loop`.
- `internal/transcript/transcript_payload.go:24` — whitespace-preserving fallback helper and test-only production helper at line 77.
- `internal/transcript/transcript_test.go:1020` — canonical transcript behavior suite and helper-only call at line 2124.
- `internal/workspace/clone.go:13` — workspace snapshot cloning and config-owned field enumeration.
- `internal/workspace/clone_test.go:1` — deep clone canonical suite.
- `internal/workspace/discovery.go:87` — lexical containment after canonical discovery.
- `internal/workspace/helpers.go:26` — canonical root and unreferenced ID helper at line 134.
- `internal/workspace/identity.go:42` — durable ULID generation using `crypto/rand`.
- `internal/workspace/perf_bench_test.go:78` — two remaining `b.N` loops and resolver benchmarks.
- `internal/workspace/resolver.go:36` — resolver/cache ownership.
- `internal/workspace/resolver_cache_contract_test.go:1` — cache isolation canonical suite.
- `internal/workspace/resolver_crud.go:13` — transactional CRUD and eager cloned list at line 194.
- `internal/workspace/resolver_test.go:120` — large resolver behavior suite and discarded chmod error at line 1166.
- `internal/workspaceaccess/default_policy.go:43` — authorization plus best-effort audit boundary.
- `internal/workspaceaccess/default_policy_contracts.go:18` — narrow consumer-owned dependency interfaces.
- `internal/workspaceaccess/default_policy_test.go:563` — string-based semantic error assertion example.
- `internal/workspaceaccess/policy.go:22` — actor/workspace/seam contract.
