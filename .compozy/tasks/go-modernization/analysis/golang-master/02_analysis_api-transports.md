# Analysis: API Transports and Runtime Boundaries

## Overview

This slice audits the current Go source for API transport, contract, SSE, window-management, admission, identity, and diagnostic-contract behavior against the repository's Go 1.26.4 modernization brief. It is a source audit, not a change proposal detached from runtime semantics: each recommendation is grounded in an existing ownership boundary, test contract, or concrete failure window.

The inventory covered every production, test, and benchmark Go file under the requested roots. The legacy top-level roots `internal/udsapi`, `internal/httpapi`, `internal/contract`, and `internal/bridgeapi` are absent; their canonical transport and contract packages live under `internal/api/`. The resulting scope is 823 files and 199,250 lines: 661 production files, 159 non-benchmark test files, and 3 benchmark files. All 661 production files are at or below the repository's 500-line hard cap; the largest observed production file is `internal/api/contract/settings_config_payloads.go` at 496 lines.

| Package root | Production | Tests | Benchmarks | Total | Coverage disposition |
| --- | ---: | ---: | ---: | ---: | --- |
| `internal/admission` | 1 | 0 | 0 | 1 | Full-file review |
| `internal/agentidentity` | 4 | 1 | 0 | 5 | Full-file review |
| `internal/api/contract` | 106 | 12 | 1 | 119 | Inventory/static scan; conversion and transport-boundary hot paths reviewed |
| `internal/api/core` | 262 | 70 | 1 | 333 | Inventory/static scan; stream, error, WebSocket, filesystem, and service hot paths reviewed |
| `internal/api/ginutil` | 1 | 1 | 0 | 2 | Full-file review |
| `internal/api/httpapi` | 35 | 26 | 0 | 61 | Lifecycle, middleware, routes, integration tests, and cleanup paths reviewed |
| `internal/api/spec` | 114 | 11 | 0 | 125 | Registry, clone, response builder, and defensive-copy tests reviewed |
| `internal/api/testutil` | 27 | 2 | 0 | 29 | Inventory/static scan; transport helpers reviewed |
| `internal/api/udsapi` | 36 | 25 | 0 | 61 | Lifecycle, helpers, integration tests, and cleanup paths reviewed |
| `internal/diagnosticcontract` | 1 | 0 | 0 | 1 | Full-file review |
| `internal/sse` | 2 | 3 | 1 | 6 | Full-file review |
| `internal/windowmanager` | 72 | 8 | 0 | 80 | Inventory/static scan; coalescing, subscription, version, geometry, and validation paths reviewed |
| **Total** | **661** | **159** | **3** | **823** | **Complete requested-slice inventory** |

The strongest correctness findings are: HTTP shutdown can lose the terminal serve error because it snapshots the error before joining the serving goroutine; the `OperationSpec` clone does not deep-copy multi-body response metadata; and SSE cancellation does not deterministically join its reader-closing goroutine or collect its close error. The strongest test-quality finding is widespread discarded cleanup errors. Several modernization candidates from the baseline are already present—most notably `http.CrossOriginProtection`, `b.Loop`, `json:",omitzero"`, range-over-integer, `slices`/`min`/`max`, and `strings.SplitSeq`—so they must not be reported as missing.

Compozy Impact Audit:

- Native tools: no direct descriptor or tool-ID change is proposed. Checked the HTTP/UDS routing and contract/spec surfaces; the findings concern transport lifecycle, cloning, cancellation ownership, timing configuration, and tests rather than `compozy__*` schemas or capability gates.
- Extensibility and hooks: transport shutdown behavior and timing options can affect extension-backed HTTP/UDS availability, but no hook, capability, bundle, resource, bridge SDK, MCP sidecar, or registry contract changes are required by the audit itself. Any implementation of public timing configuration must complete the repository's config lifecycle rather than introduce package-local constants.
- Workspace data isolation: no new datum or ownership scope is proposed. The reviewed identity path validates daemon session identity and workspace access before constructing agent identity (`internal/agentidentity/identity.go:45-76`, `internal/agentidentity/identity.go:115-188`); none of the findings weakens workspace propagation.
- Official Compozy skill: no immediate change for an analysis artifact. If a later implementation exposes transport timing keys or changes public CLI/API behavior, `skills/compozy/` must be audited in that implementation.

## Mechanisms / Patterns

The matrix below evaluates exactly the 20 modernization candidates from the supplied brief. Decisions use only the requested vocabulary. “Severity” reflects the consequence of leaving the current slice unchanged, not the novelty of the standard-library feature. “Fowler technique” names the applicable refactoring move; “No refactoring indicated” means the candidate does not fit this slice.

| # | Go feature / modernization candidate | Decision | Severity | Confidence | Fowler technique | Evidence and disposition |
| ---: | --- | --- | --- | --- | --- | --- |
| 1 | `errors.AsType[T]` | adopt | low | high | Substitute Algorithm | The slice already uses `errors.AsType` in some mappings (`internal/api/core/errors.go:78-82`), but production retains repeated temporary-variable `errors.As` branches in the same mapper (`internal/api/core/errors.go:321-328`, `internal/api/core/errors.go:419-426`) and identity errors (`internal/agentidentity/errors.go:125-130`). Convert only type-extraction cases; retain `errors.Is` for sentinel identity. |
| 2 | `testing/synctest` | adopt | medium | high | Substitute Algorithm | Pure timer/cancellation units can become deterministic around the coalescer timer (`internal/windowmanager/active_coalescer.go:74-100`) and SSE cancellation tests (`internal/sse/decode_context_test.go:32-49`). Real socket integration tests still need readiness hooks because `synctest` does not make external I/O deterministic. |
| 3 | Range-over-function / `iter.Seq` | reject | low | high | No refactoring indicated | The operation registry intentionally materializes, mutates, and sorts the complete set (`internal/api/spec/operations.go:6-34`); SSE and WebSocket paths are concurrent streams rather than pull-only collections. An iterator would obscure ownership and would not remove necessary allocation. |
| 4 | `os.Process.WithHandle` / `os.ErrNoHandle` | not applicable | none | high | No refactoring indicated | No process spawning, process-handle manipulation, or `os.Process` ownership occurs in the 823-file scope. |
| 5 | `sync.OnceValue` / `sync.OnceFunc` | adopt | low | high | Substitute Algorithm | A local idempotent closure built from `sync.Once` can become `sync.OnceFunc` (`internal/api/core/window_manager_service.go:54-57`). Stateful subscription shutdown should retain its explicit `sync.Once`, because it coordinates fields and `context.AfterFunc` state (`internal/windowmanager/subscription.go:53-76`). |
| 6 | `math/rand/v2` | not applicable | none | high | No refactoring indicated | Neither `math/rand` nor random-number ownership appears in the requested slice. |
| 7 | `cmp.Or` | reject | low | high | No refactoring indicated | Existing fallback code performs domain normalization such as `strings.TrimSpace` before deciding whether to fall back (`internal/api/httpapi/middleware.go:265-271`). `cmp.Or` would silently change whitespace and zero-value semantics rather than simplify equivalent logic. |
| 8 | `T.ArtifactDir` / `T.Attr` / `T.Output` | not applicable | none | high | No refactoring indicated | Scoped tests use buffers and bodies for assertions, not persistent diagnostic artifacts. There is no test-owned output directory or structured test metadata contract in this slice that these APIs would improve. |
| 9 | `http.CrossOriginProtection` | already | low | high | No refactoring indicated | Cross-origin protection is constructed with trusted-origin configuration (`internal/api/httpapi/middleware.go:114-127`), wired into routing (`internal/api/httpapi/routes.go:11-17`), and exercised for accepted/rejected origins (`internal/api/httpapi/middleware_refac_test.go:234-264`). The preliminary “missing protection” hypothesis is disproved. |
| 10 | `runtime/trace.NewFlightRecorder` | not applicable | none | high | No refactoring indicated | Runtime diagnostics capture is outside this slice. `internal/diagnosticcontract` defines and defensively exposes diagnostic wire specifications, not trace-recording lifecycle (`internal/diagnosticcontract/diagnostics.go:343-360`). |
| 11 | `net.Dialer.DialUnix` / `DialTCP` | adopt | low | high | Substitute Algorithm | UDS test clients still select Unix transport through stringly `DialContext` calls (`internal/api/udsapi/helpers_test.go:647-656`, `internal/api/udsapi/udsapi_integration_test.go:211-215`). Typed Unix dialing reduces network/address mismatch possibilities while retaining explicit context and deadlines. |
| 12 | `bytes.Buffer.Peek` | not applicable | none | high | No refactoring indicated | SSE framing is decoded with `bufio.Scanner` (`internal/sse/decode.go:48-50`). Buffer use elsewhere is assertion capture or JSON compaction; no protocol look-ahead currently requires `Peek`. |
| 13 | `unique.Handle` | reject | low | high | No refactoring indicated | Window/session/agent identifiers are high-cardinality runtime data, not a demonstrated repeated-value memory hotspot (`internal/windowmanager/types.go:187-195`). Repeated closed vocabularies already use typed constants. Adoption without profiling would add identity semantics and indirection without evidence. |
| 14 | `b.Loop` | already | none | high | No refactoring indicated | All scoped benchmark loops use `b.Loop`; representative SSE cases are at `internal/sse/perf_bench_test.go:41-58`, and the API core benchmarks follow the same form. No `b.N` loop remains in scope. |
| 15 | `json:",omitzero"` | already | none | high | No refactoring indicated | The slice already uses `omitzero` where zero values have omission semantics, including optional window-manager command timestamps (`internal/windowmanager/commands.go:116`) and operation contract fields (`internal/api/contract/loops.go:458-459`). |
| 16 | `os.OpenRoot` | not applicable | none | high | No refactoring indicated | Filesystem browsing deliberately accepts operator-selected absolute paths and enumerates the local filesystem (`internal/api/core/fs_browse.go:16-18`, `internal/api/core/fs_browse.go:22-39`, `internal/api/core/fs_browse.go:65-83`). There is no bounded root capability to model with `os.Root`; changing that is a product/security decision, not a mechanical modernization. |
| 17 | `strings.SplitSeq` / `FieldsSeq` / `Lines` | already | low | high | No refactoring indicated | Production already uses `strings.SplitSeq` for line-oriented diagnostics and extension parsing (`internal/api/core/status_diagnostics.go:13-23`, `internal/api/core/extensions.go:77`). Remaining `Split` uses require indexing, length, or retained slices and are not equivalent streaming candidates. |
| 18 | `sync.WaitGroup.Go` | adopt | low | high | Substitute Algorithm | Active-command coalescing manually performs `Add(1)`, starts a goroutine, and defers `Done` (`internal/windowmanager/active_coalescer.go:78-88`) before joining at shutdown (`internal/windowmanager/active_coalescer.go:228-260`). `WaitGroup.Go` expresses this ownership directly. |
| 19 | Range over integer | already | none | high | No refactoring indicated | Fixed-count production iteration already uses integer range, including placeholder restoration in the window reducer (`internal/windowmanager/reducer_window_open.go:79-90`). Tests and benchmarks use it as well. |
| 20 | `slices` / `maps` / built-in `min` / `max` | already | none | high | No refactoring indicated | The slice already uses standard helpers for bounds, cloning, sorting, and normalization, including geometry (`internal/windowmanager/geometry.go:15-24`), coalescer calculations (`internal/windowmanager/active_coalescer.go:200-203`, `internal/windowmanager/active_coalescer.go:242-245`), and validation (`internal/windowmanager/validate.go:218-224`). |

Two ownership patterns dominate the substantive findings:

1. A serving goroutine publishes a terminal error and closes a completion channel; shutdown must first establish “stopping,” then request shutdown, then join, then read the published error, and only then clear lifecycle fields. UDS follows this ordering, while HTTP reads the error too early.
2. A cancellation callback closes a blocking reader; the decoding caller must retain a stop handle, distinguish “callback prevented” from “callback already running,” and join the running callback before choosing the returned error. The existing SSE helper starts a goroutine but does not provide that deterministic join.

## Relevant Sources

- `internal/api/httpapi/server_start.go:84-94` — the serving goroutine publishes `s.serveErr` under lock and then closes `serveDone`.
- `internal/api/httpapi/server_shutdown.go:19-60` — shutdown snapshots `serveErr` before `Shutdown` and before waiting for `serveDone`, then returns that stale snapshot.
- `internal/api/udsapi/server.go:40-46` — explicit UDS lifecycle states distinguish stopped, starting, running, and stopping.
- `internal/api/udsapi/server_shutdown.go:19-73` — marks stopping, shuts down, joins, reads `serveErr` after the join, and clears lifecycle state only after successful completion.
- `internal/api/udsapi/server_test.go:363-420` — the UDS contract rejects a new start while shutdown is in progress.
- `internal/api/spec/spec.go:206-213` — `ResponseSpec` contains both a single `Body` and a multi-variant `Bodies []any` field.
- `internal/api/spec/operation_clone.go:37-47` — the response clone copies the struct and deep-clones `Body`, but not elements or containers reachable from `Bodies`.
- `internal/api/spec/operation_builder.go:41-64` — `buildResponse` consumes `Bodies` to construct a response union.
- `internal/api/spec/goals.go:77-88` and `internal/api/spec/windowmanager.go:316-325` — current operations with multi-body response variants.
- `internal/api/spec/operations_refac_test.go:5-98` — canonical registry defensive-copy suite; it mutates tags, transports, parameters, response status, and request maps but does not mutate `Bodies`.
- `internal/sse/decode.go:31-50` — decoder ownership contract and scanner loop.
- `internal/sse/decode.go:108-133` — cancellation goroutine, unjoined `done` signal, and non-blocking close-error read.
- `internal/windowmanager/subscription.go:53-76` — an existing, clearer `context.AfterFunc` cancellation/close pattern.
- `internal/api/httpapi/httpapi_integration_test.go:312-328` and `internal/api/httpapi/httpapi_integration_test.go:809-827` — discarded response-body close errors and sleep-based synchronization.
- `internal/api/udsapi/udsapi_integration_test.go:2725-2848` and `internal/api/udsapi/udsapi_integration_test.go:3572-3587` — UDS polling/sleep synchronization and cleanup paths.
- `internal/api/httpapi/server.go:24-29` and `internal/api/udsapi/server.go:25-30` — duplicated transport timeout defaults.
- `internal/windowmanager/active_coalescer.go:14`, `internal/api/core/window_manager_ws.go:16-21`, and `internal/api/core/session_stream_live.go:11-14` — operational durations embedded as package constants.
- `internal/windowmanager/types.go:9`, `internal/windowmanager/validate.go:57-60`, and `internal/windowmanager/reducer_history_replace.go:3-11` — canonical snapshot version plus duplicated literal diagnostics.
- `internal/admission/gate.go:18-68` — small typed admission boundary using `atomic.Bool` and a `Checker` contract.
- `internal/agentidentity/identity.go:45-76` and `internal/agentidentity/identity.go:115-188` — daemon-session and workspace-access validation before identity construction.
- `internal/diagnosticcontract/diagnostics.go:343-360` — registry output is copied and sorted before exposure.

## Transferable Patterns

The UDS lifecycle is the strongest local reference implementation. Its state enum makes transitional ownership visible, it rejects restart during shutdown, and it reads the terminal serve error only after the completion channel establishes the happens-before edge (`internal/api/udsapi/server.go:40-46`, `internal/api/udsapi/server_shutdown.go:19-73`). HTTP should converge on the same semantics even if package-boundary constraints favor two small implementations over a shared helper.

Window-manager subscriptions demonstrate a useful cancellation primitive: `context.AfterFunc` ties closure to cancellation while `sync.Once` keeps close idempotent (`internal/windowmanager/subscription.go:53-76`). SSE can reuse the shape while adding an explicit callback-completion handshake because it must decide whether a `Body.Close` error contributes to `Decode`'s returned error.

Contract conversions commonly protect mutable wire data instead of leaking package-owned maps or slices. For example, window-manager conversion paths clone nested layout data before returning it (`internal/api/contract/windowmanager_conversions.go:20-25`, `internal/api/contract/windowmanager_conversions.go:181-203`). `OperationSpec` cloning should hold the same graph-wide invariant for `Bodies`, not merely clone selected fields.

The HTTP middleware already uses the modern cross-origin primitive and applies it at the route boundary (`internal/api/httpapi/middleware.go:114-127`, `internal/api/httpapi/routes.go:11-17`). This is a useful reminder that a modernization inventory must search for construction, wiring, and tests before calling a feature absent.

The smaller corrected roots are structurally healthy and should remain boring boundaries. Admission uses a typed checker and atomic state (`internal/admission/gate.go:18-68`); identity validates session and workspace authority before exposing an agent identity (`internal/agentidentity/identity.go:45-76`, `internal/agentidentity/identity.go:115-188`); diagnostics returns a defensive, stable-order registry view (`internal/diagnosticcontract/diagnostics.go:343-360`). No additional modernization mechanism is justified in those packages beyond the targeted `errors.AsType` cleanup in identity error mapping.

Finally, the slice already respects the production file-size boundary: 661 production files were measured and none exceeds 500 lines. Future lifecycle or configuration work should preserve that shape by extracting a focused lifecycle coordinator or timing-options type rather than growing the current server implementations into mixed-responsibility files.

## Risks / Mismatches

**F-01 — HTTP shutdown can lose a serving error and reopen start during drain.** Severity: high. Confidence: high. Fowler technique: Extract Class.

`Shutdown` copies `s.serveErr` while holding the mutex, clears the lifecycle fields, releases the lock, requests server shutdown, waits for `serveDone`, and finally appends the earlier copy (`internal/api/httpapi/server_shutdown.go:19-60`). The serving goroutine publishes `s.serveErr` immediately before closing `serveDone` (`internal/api/httpapi/server_start.go:84-94`). If shutdown takes its snapshot first, the join confirms that publication occurred but the returned error remains the stale pre-join value. Because lifecycle fields are cleared before the drain/join finishes, another `Start` can also enter while shutdown is active. The goroutine may then repopulate `serveErr` after the earlier clear, leaving stale state for the new lifecycle. Align HTTP with the UDS state machine: mark stopping, prevent restart, request shutdown, join, read the terminal error after the join, and clear state only at the terminal transition (`internal/api/udsapi/server_shutdown.go:19-73`). A small private lifecycle coordinator is justified if it can be shared without crossing package boundaries awkwardly; otherwise preserve identical state semantics in both transports.

**F-02 — `OperationSpec` cloning is not deep for multi-body responses.** Severity: medium. Confidence: high. Fowler technique: Extract Function.

`ResponseSpec` supports `Bodies []any` (`internal/api/spec/spec.go:206-213`), and the response builder interprets those values as a union (`internal/api/spec/operation_builder.go:41-64`). `cloneResponseSpecs` copies each `ResponseSpec` and deep-clones only `Body`; it leaves the `Bodies` slice and any nested maps/slices shared (`internal/api/spec/operation_clone.go:37-47`). The existing defensive-copy suite does not exercise this field (`internal/api/spec/operations_refac_test.go:5-98`). Current `Bodies` users are built by operation groups after the registry clone (`internal/api/spec/goals.go:77-88`, `internal/api/spec/windowmanager.go:316-325`), so the defect is latent rather than a demonstrated cross-call mutation today. It nevertheless makes the clone helper's graph-isolation contract false and becomes observable if those groups move into cloned registry data or another caller reuses the helper. Extract a recursive response-body cloning helper and cover the existing invariant in the canonical suite: mutating any multi-body response returned by `Operations()` must not change a later result. Owning layer: spec registry cloning. Canonical suite: `internal/api/spec/operations_refac_test.go`.

**F-03 — SSE cancellation does not deterministically join reader closure.** Severity: medium. Confidence: high. Fowler technique: Substitute Algorithm.

`Decode` documents that it closes the body on cancellation but leaves body ownership with the caller on normal, handler, and `ErrStop` exits (`internal/sse/decode.go:31-46`). `closeReaderOnCancel` starts an untracked goroutine that calls `reader.Close()` and sends its error (`internal/sse/decode.go:108-121`). The caller closes a “done” channel on return but never joins a cancellation goroutine already in progress; `decodeContextError` performs a non-blocking receive and can miss a close error when `Close` has unblocked `Scan` but has not yet returned (`internal/sse/decode.go:124-133`). Replace the raw watcher with `context.AfterFunc` plus a stop/join handshake: if the callback has begun, wait for callback completion before resolving the cancellation and close errors; if it has not begun, stop it. Keep the current caller-owned body behavior for non-cancellation exits explicit in the API contract.

**F-04 — Transport tests systematically discard cleanup failures.** Severity: high. Confidence: high. Fowler technique: Extract Function.

The scope contains 225 assignments of `Close` results to `_` across six test files and 8 discarded `Shutdown` results in tests. Representative HTTP cleanup drops response-body failures (`internal/api/httpapi/httpapi_integration_test.go:312-321`, `internal/api/httpapi/httpapi_integration_test.go:818-827`); UDS integration cleanup does the same (`internal/api/udsapi/udsapi_integration_test.go:2750-2763`). This violates the repository's explicit rule that production and tests must handle every error or document a justification, and it can hide truncated response reads, connection reuse failures, or incomplete teardown. Extract canonical test helpers that drain/close response bodies and report cleanup errors through `t.Cleanup`; likewise, server shutdown helpers must report or fail on shutdown errors. The test invariant belongs to each transport integration suite: a test must not report success if transport/resource cleanup failed.

**F-05 — Integration ordering relies on wall-clock sleeps.** Severity: medium. Confidence: high. Fowler technique: Substitute Algorithm.

Ten `time.Sleep` calls remain in the scoped tests, all in integration paths. Some poll with a deadline (`internal/api/httpapi/httpapi_integration_test.go:315-328`; `internal/api/udsapi/udsapi_integration_test.go:2725-2848`), while at least one fixed 100 ms delay acts as an ordering barrier before an HTTP assertion (`internal/api/httpapi/httpapi_integration_test.go:809-827`). Fixed sleeps are scheduler- and machine-dependent; polling still hides which readiness condition owns the synchronization. Add explicit subscription/readiness signals where the test controls both ends, and use a bounded event-driven retry helper where real network I/O is intentionally under test. Use `testing/synctest` only for pure timer/cancellation units such as coalescer and SSE tests, not as a wrapper around sockets.

**F-06 — Operational timing policy is duplicated and compiled into packages.** Severity: medium. Confidence: high. Fowler technique: Introduce Parameter Object.

HTTP and UDS define parallel timeout defaults separately (`internal/api/httpapi/server.go:24-29`, `internal/api/udsapi/server.go:25-30`). Other externally observable timing policy is embedded in active-command coalescing (`internal/windowmanager/active_coalescer.go:14`), WebSocket behavior (`internal/api/core/window_manager_ws.go:16-21`), and SSE keepalive behavior (`internal/api/core/session_stream_live.go:11-14`). These constants impede deterministic tests and make runtime tuning impossible. Introduce cohesive transport/stream timing options with explicit zero/disable semantics, populated through the existing configuration lifecycle where the behavior is operator-facing. Do not replace constants with scattered functional options that bypass config, CLI/HTTP/UDS management, defaults, docs, or the official skill.

**F-07 — Snapshot diagnostics duplicate the current version as a magic literal.** Severity: low. Confidence: high. Fowler technique: Replace Magic Literal with Symbolic Constant.

`SnapshotVersion` is canonical (`internal/windowmanager/types.go:9`), but validation and history replacement hard-code “must be 3” in diagnostics (`internal/windowmanager/validate.go:57-60`, `internal/windowmanager/reducer_history_replace.go:3-11`). A future version bump can make these errors lie even if validation logic uses the new constant. Format both messages from `SnapshotVersion` or centralize version validation in a narrowly named helper.

**F-08 — Active coalescer manually expands `WaitGroup.Go`.** Severity: low. Confidence: high. Fowler technique: Substitute Algorithm.

The coalescer performs `Add(1)`, launches a goroutine, and defers `Done` (`internal/windowmanager/active_coalescer.go:78-88`), then cancels timers and waits during closure (`internal/windowmanager/active_coalescer.go:228-260`). No ownership race was found: `Add` occurs under the same mutex that prevents new work after close, timers are canceled, and `Wait` joins active goroutines. This is a clarity modernization, not a correctness fix. Replace the triplet with `c.wg.Go(func() { c.runTimer(...) })` and remove the corresponding `Done` from the worker so ownership remains singular.

## Open Questions

1. Should HTTP and UDS share a private lifecycle coordinator, or should HTTP only adopt the UDS state semantics? The important contract is the transition ordering and restart exclusion; reuse is secondary to keeping package boundaries clear.
2. Is a cancellation-time `Body.Close` failure part of the public `sse.Decode` error contract, and if both cancellation and close fail should they be joined? The current code attempts to surface it, but its non-blocking receive makes the contract nondeterministic.
3. Which timing values are operator policy versus internal transport plumbing? Operator-visible values should become documented configuration with CLI/HTTP/UDS lifecycle coverage; genuinely internal values can remain constructor options used by tests.
4. For response cleanup, are any close failures intentionally ignorable after a complete body read? If so, the justification must be explicit at the helper boundary; the repository rule still forbids silent `_ = body.Close()` calls.
5. For the `Bodies` regression, should the canonical test mutate a nested map/slice as well as the outer slice? The graph-isolation invariant implies both; the existing `operations_refac_test.go` is the owning suite and should not be duplicated at another layer.

## Evidence

- Repository/toolchain evidence: `go.mod` declares Go 1.26.4, so every evaluated standard-library feature is available without a version bridge.
- Inventory evidence: recursive `rg --files` classification covered 823 Go files under the requested roots—661 production, 159 non-benchmark tests, and 3 benchmarks—and static-pattern scans were applied to the complete inventory. Every file in `internal/admission`, `internal/agentidentity`, `internal/diagnosticcontract`, and `internal/sse` was read in full; high-risk lifecycle, cloning, streaming, configuration, and integration-test files in the larger API/window-manager roots were reviewed line by line.
- Size evidence: all 661 production files are at or below 500 lines; the maximum observed production length is 496 lines in `internal/api/contract/settings_config_payloads.go`.
- Error-modernization evidence: 7 scoped production uses of `errors.AsType` coexist with 20 traditional production `errors.As` sites; only type-extraction sites are candidates, not sentinel checks.
- Benchmark evidence: all 8 scoped benchmark loops use `b.Loop`, with no `b.N` loop remaining; representative usage is `internal/sse/perf_bench_test.go:41-58`.
- JSON evidence: 8 scoped `omitzero` tag occurrences already model intentional zero omission; representative declarations are `internal/windowmanager/commands.go:116` and `internal/api/contract/loops.go:458-459`.
- Cross-origin evidence: construction, route wiring, and behavior tests are present at `internal/api/httpapi/middleware.go:114-127`, `internal/api/httpapi/routes.go:11-17`, and `internal/api/httpapi/middleware_refac_test.go:234-264`; the preliminary absence hypothesis is explicitly disproved.
- Cleanup evidence: the complete test scan found 225 discarded `Close` results across six test files and 8 discarded `Shutdown` results. These counts identify a suite-level cleanup policy problem rather than an isolated call site.
- Synchronization evidence: 10 scoped `time.Sleep` calls occur in integration tests; representative fixed-delay and polling paths are `internal/api/httpapi/httpapi_integration_test.go:315-328`, `internal/api/httpapi/httpapi_integration_test.go:809-827`, and `internal/api/udsapi/udsapi_integration_test.go:2725-2848`.
- Lifecycle proof: HTTP reads `serveErr` before the completion-channel join (`internal/api/httpapi/server_shutdown.go:19-60`), while the serving goroutine writes immediately before completion (`internal/api/httpapi/server_start.go:84-94`). UDS demonstrates the corrected state and publication order (`internal/api/udsapi/server_shutdown.go:19-73`).
- Clone proof: `Bodies` is mutable response-spec state (`internal/api/spec/spec.go:206-213`), is consumed by schema construction (`internal/api/spec/operation_builder.go:41-64`), and is omitted from the clone's deep-copy work (`internal/api/spec/operation_clone.go:37-47`).
- Cancellation proof: SSE creates a raw cancellation goroutine and later performs a non-blocking error receive without joining it (`internal/sse/decode.go:108-133`); window-manager subscriptions provide a local `context.AfterFunc` reference pattern (`internal/windowmanager/subscription.go:53-76`).
- Negative evidence: no scoped `os.Process`, process-spawn, or `math/rand` ownership exists; no bounded filesystem root exists; no protocol uses `bytes.Buffer.Peek`; and no profiling result supports interning high-cardinality runtime identifiers.
- Validation boundary: this task performed source inspection only, as required. No production/test file was mutated, and no test, build, formatter, linter, gate, Git operation, or prior analysis artifact was run or read.
