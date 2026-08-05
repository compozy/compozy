# Analysis: extensions-bridges

- Ordinal / slug: 07_analysis_extensions-bridges
- Slice question: Re-audit extensibility, the bridge SDK, hooks, bundles, and marketplace under the current Go 1.26 golang-master doctrine, emphasizing lifecycle correctness, safe modernization, duplication, Fowler smells, and preservation of extension contracts.
- Primary scope: every Go source, test, benchmark, scaffold, and testdata program under internal/extension/**, internal/extensionprotocol/**, internal/extensiontest/**, internal/bridges/**, internal/bridgesdk/**, internal/bundles/**, internal/hooks/**, and internal/marketplace/**.
- Coverage: 643 Go files: extension 279, extensionprotocol 3, extensiontest 20, bridges 105, bridgesdk 65, bundles 35, hooks 108, and marketplace 28.
- Review mode: all 643 files were inventoried and pattern-scanned; lifecycle, contract, filesystem, concurrency, error, benchmark, and fixture hotspots were read in full. Previous analysis artifacts were deliberately not consulted.
- Validation boundary: this was a read-only source audit. No tests, generators, formatters, package managers, or Git commands were run.

## 1. Overview

This slice has a sound high-level decomposition. The extension manager owns discovery, capability grants, resource registration, subprocess initialization, and recovery. extensionprotocol owns leaf wire identifiers, extension/contract maps those identifiers to typed payloads, extensiontest owns provider-conformance support, bridges owns daemon-side delivery, bridgesdk owns provider-side transport and lifecycle, bundles owns activation projection, hooks owns typed dispatch, and marketplace owns bounded catalog refresh and persistence.

The fresh review changes the priority of the modernization work. The first work should not be syntactic replacement. The highest-value changes are lifecycle repairs: make extension activation transactional, give bridgesdk.Peer an explicit terminal state and cancellable read ownership, preserve deadlines for marketplace notification and compensating work, reject work after Broker.Close, and make provider lifecycle shutdown and join semantics explicit. These are correctness changes that should land before broad use of WaitGroup.Go or other conveniences.

Modern Go adoption is already strong. The slice uses errors.AsType, b.Loop, omitzero, strings.SplitSeq, range-over-integer, slices, maps, min/max, and WaitGroup.Go in appropriate places. The remaining safe mechanical wins are narrow: convert one benchmark loop, replace math/rand with math/rand/v2 for retry jitter, migrate conventional typed error extraction, and use the typed network dial API in the webhook dialer. os.OpenRoot, synctest, the new testing artifact APIs, and CrossOriginProtection are valuable only at the specific boundaries described below.

No production source file exceeds the 500-line cap, but several are at immediate risk: internal/bridgesdk/runtime.go has 481 lines, internal/extension/contract/host_api_method_registry.go has 479, internal/bundles/resource_projection.go has 478, and internal/marketplace/service.go has 459. The 882-line telegram reference and 704-line secret-guard programs are executable test fixtures, not shipped production packages; they are still contract-bearing artifacts and require different treatment. The positive telegram reference should converge on shared SDK helpers, while the adversarial secret guard must remain an independent raw-protocol fixture.

## 2. Mechanisms / Patterns

The dominant mechanisms are staged extension startup, generation-based subprocess supervision, per-route delivery workers, provider-owned SDK lifecycles, typed hook pipelines, compensating bundle projection, refresh-flight collapse, and strict JSON/wire registries. The design generally favors consumer-owned interfaces and explicit ownership. The mismatches arise when a stage acquires durable state without registering its inverse, or when detached cleanup also loses its deadline.

The required Go 1.26 feature assessment follows. Status is intentionally conservative at public wire and SDK boundaries.

| # | Feature | Status | Evidence and decision |
|---:|---|---|---|
| 1 | errors.AsType | adopt | It is already used correctly in internal/bridgesdk/peer.go:247, internal/bridgesdk/errors.go:158, and internal/marketplace/service.go:441. Convert the remaining pointer-target forms in internal/extension/validate_bundle.go:90-129 and internal/bridgesdk/errors.go:236-262 where the target is a concrete error type. Preserve explicit non-nil checks when callers depend on a populated pointer. |
| 2 | b.Loop | adopt | Almost every benchmark already uses it. The residual timed loop is internal/extensiontest/perf_bench_test.go:52. Convert that loop to b.Loop; using b.N only to pre-size the recording slice at line 35 remains valid. |
| 3 | omitzero | already | Correct zero-value uses exist for time and value structs at internal/bridges/contract/targets.go:61, internal/hooks/payloads_task_loop.go:45, and internal/extension/contract/host_api.go:175. Do not mass-convert omitempty fields: pointer, slice, and explicit absence semantics are wire contracts. |
| 4 | os.OpenRoot | adopt | The managed installer canonicalizes paths and then performs path-based Stat, ReadDir, Lstat, EvalSymlinks, and Open operations at internal/extension/install_managed_tree.go:24-63 and internal/extension/install_managed_package.go:91-166. Use rooted, relative operations for source and staging trees to close symlink-swap windows while retaining the current containment and cycle rules. |
| 5 | strings Seq | already | Production uses strings.SplitSeq without materialization in internal/extension/semantic_version.go:113 and internal/extension/host_api_bridges_prompt.go:101. Keep materialized helpers only where callers require a reusable slice or trimming/filtering. |
| 6 | WaitGroup.Go | adopt | It is already used by internal/bridgesdk/provider_lifecycle.go:289 and tests. Convert simple Add/go/Done ownership in broker and manager workers after their closed-state invariants are fixed. Do not mechanically convert internal/bridgesdk/batching.go:293 because timer Stop conditionally balances Done, and do not convert Peer handlers until a panic boundary exists because WaitGroup.Go functions must not panic. |
| 7 | range-int | already | Representative uses are internal/extensiontest/perf_bench_test.go:67,96,99. The remaining index loops either belong to the old benchmark loop or require an index for reflection/cache addressing; no broad rewrite is warranted. |
| 8 | slices/maps/min/max | already | Representative uses include slices.SortFunc in internal/bundles/service.go:180, maps.Copy in internal/bridges/delivery_broker_metrics.go:154, and min in internal/bridges/catalog_records.go:126. The slice already follows this doctrine consistently. |
| 9 | synctest | adopt | The timer-driven unit test in internal/bridgesdk/batching_test.go:12-45 is a direct candidate. The sleep at internal/marketplace/service_test.go:381 should instead become a start/release channel barrier because it coordinates work rather than virtual time. External-process provider polling must remain real, bounded polling. |
| 10 | iter.Seq | defer | Public registries intentionally return materialized, deterministic slices at internal/extensionprotocol/host_api.go:97-199, and contract results include slice-shaped wire values at internal/extension/contract/host_api_method_registry.go:42-44. No measured internal streaming need justifies an iterator API or a source compatibility change. |
| 11 | Process.WithHandle | not applicable | Scoped subprocess code delegates process-group configuration and termination to the procutil/subprocess owner, as shown by internal/extension/build_runner.go:47-71 and internal/hooks/executor_subprocess_lifecycle.go:69-83. The handle migration belongs in that owning package, not in this slice. |
| 12 | OnceFunc/OnceValue(s) | adopt | Stateful close/init gates such as internal/bridgesdk/provider_lifecycle.go:54-58 should remain explicit sync.Once fields. The repeated provider build caches, for example internal/extension/discord_provider_integration_test.go:35,314-333 and internal/extension/gchat_provider_integration_test.go:48,270-293, should move to one shared builder and may use OnceValues once inputs are immutable. |
| 13 | rand/v2 | adopt | Retry jitter is the only non-cryptographic v1 use: internal/bridgesdk/retry.go:6,55,106. Change the default injected function to math/rand/v2.Float64; retain the injection seam so tests remain deterministic. crypto/rand uses are unrelated and must remain. |
| 14 | cmp.Or | reject | The two firstNonEmpty helpers trim before choosing at internal/extension/marketplace_helpers.go:41-46 and internal/bridges/target.go:178-184; cmp.Or would change that behavior. Numeric defaults use domain rules such as less-than-or-equal-to-zero at internal/bridgesdk/retry.go:100-121, so zero-value selection is not equivalent. |
| 15 | T.ArtifactDir/Attr/Output | adopt | internal/extensiontest/bridge_adapter_harness.go:83-100 manually creates and preserves a temporary evidence directory. Route evidence through the caller's artifact directory, tag provider/platform/contract dimensions with Attr, and attach subprocess diagnostics with Output. Keep provider binaries at fixed manifest paths only where the runtime contract requires them. |
| 16 | CrossOriginProtection | adopt | internal/bridgesdk/provider_http.go:69-73 installs the provider handler directly. Add an explicit HTTP security policy and wrap browser-reachable provider endpoints with CrossOriginProtection, with same-origin/cross-origin tests. Server-to-server webhook traffic without browser headers must remain accepted. |
| 17 | FlightRecorder | not applicable | The hookTrace value at internal/hooks/pipeline.go:139 is domain execution evidence, not runtime/trace diagnostics. Process-wide flight recording belongs to the diagnostics owner outside this slice and should not be embedded in provider or hook packages. |
| 18 | typed Dial | adopt | internal/bridgesdk/webhook_http.go:20-21 exposes a generic DialContext seam and line 74 re-serializes a validated IP and port. Move the injected seam to typed TCP dialing and construct a typed destination after validation; preserve resolver injection, public-IP checks, redirect refusal, and timeout behavior. |
| 19 | Buffer.Peek | not applicable | Peer framing is newline-delimited JSON through bufio.Scanner at internal/bridgesdk/peer.go:67-75,175. Other buffers are encoders or output capture, not parsers needing look-ahead. Buffer.Peek would add no useful invariant here. |
| 20 | unique | reject | Host methods and hook events are public string identities, for example internal/extensionprotocol/host_api.go:3-28 and internal/hooks/events.go:8-59. Interning would complicate representation and contract code without a memory profile showing repeated-string retention as a hotspot. |

The feature matrix implies a sequencing rule: repair ownership and terminal-state invariants first; then apply local mechanical modernization; finally consider API-affecting or profiling-dependent features. That ordering prevents modern syntax from making an unsafe lifecycle look complete.

## 3. Relevant Sources

Every Go source matched by the eight authorized roots was included in the inventory and in scans for package declarations, errors.As/AsType, contexts, goroutine starts, WaitGroup ownership, discarded errors, panic/exit paths, numeric conversions, wire tags, modern library features, benchmark loops, sleeps, and file size. The table records source-complete coverage; “deep reads” identifies the highest-risk files read end to end, not an exclusion of the remaining sources.

| Root | Go files | Logical packages and artifact classes | Coverage result |
|---|---:|---|---|
| internal/extension/** | 279 | extensionpkg; contract; surfaces; Go scaffold templates; command/digest/secret-guard/telegram-reference testdata programs; unit, integration, and benchmark sources | Complete inventory and pattern scan. Deep reads covered manager startup/lifecycle/supervision, runtime launch, failure cleanup, build/install, marketplace lifecycle, host contract registries, scaffolds, and both large testdata programs. |
| internal/extensionprotocol/** | 3 | leaf extension protocol identifiers and registry helpers | All files read or declaration-scanned; host_api.go was read in full because it is a wire-order source. |
| internal/extensiontest/** | 20 | provider conformance harness, scripted drivers, marker/state readers, benchmarks | Complete inventory and pattern scan; bridge_adapter_harness.go and perf_bench_test.go were read in full. |
| internal/bridges/** | 105 | bridges daemon package plus bridges/contract | Complete inventory and pattern scan; delivery broker, route worker, registry, metrics, target normalization, and delivery contract types were deep-read. |
| internal/bridgesdk/** | 65 | provider SDK transport, runtime, lifecycle, batching, HTTP/webhook, retry, markers, and tests | Complete inventory and pattern scan; Peer, runtime, provider lifecycle/HTTP, webhook dialer, batching, retry, markers, and error taxonomy were deep-read. |
| internal/bundles/** | 35 | bundles service plus bundles/model | Complete inventory and pattern scan; activation service, resource projection, and resource-store compensation paths were deep-read. |
| internal/hooks/** | 108 | typed hook catalog, sync pipeline, async pool, subprocess executor, telemetry, payloads, and tests | Complete inventory and pattern scan; pool, pipeline, subprocess lifecycle, event catalog, and event identifiers were deep-read. |
| internal/marketplace/** | 28 | catalog source, strict decode/validation, refresh service, lifecycle, persistence, notification, and tests | All files inventoried; service.go, service_lifecycle.go, source.go, store.go, and the concurrency test were deep-read. |

The principal 25-file working set was:

1. internal/extension/manager.go; internal/extension/manager_lifecycle.go; internal/extension/manager_supervision.go; internal/extension/manager_runtime_launch.go; internal/extension/manager_failure_state.go.
2. internal/extension/build_runner.go; internal/extension/install_managed_tree.go; internal/extension/install_managed_package.go; internal/extension/contract/host_api_method_registry.go; internal/extension/testdata/telegram-reference/main.go.
3. internal/extensionprotocol/host_api.go; internal/extensiontest/bridge_adapter_harness.go; internal/bridges/delivery_broker.go; internal/bridges/delivery_route_worker.go; internal/bridges/contract/delivery_types.go.
4. internal/bridgesdk/peer.go; internal/bridgesdk/runtime.go; internal/bridgesdk/provider_lifecycle.go; internal/bridgesdk/provider_http.go; internal/bridgesdk/webhook_http.go.
5. internal/bundles/service.go; internal/bundles/resource_projection.go; internal/hooks/pipeline.go; internal/hooks/executor_subprocess_lifecycle.go; internal/marketplace/service.go.

Additional full reads were used to validate individual conclusions, including internal/marketplace/source.go, internal/marketplace/service_lifecycle.go, internal/bridgesdk/batching.go, internal/bridgesdk/batching_test.go, internal/bridgesdk/provider_markers.go, internal/extension/install_managed_path.go, internal/extension/testdata/secret-guard/main.go, and the provider integration build helpers.

Artifact ownership matters:

- The Go scaffold templates are product outputs. A modernization is valid only if a newly generated extension still compiles against the supported SDK and preserves the generated command/wire contract.
- The telegram-reference program is a positive reference provider. Duplicating lifecycle and marker machinery there weakens conformance because the test can pass without exercising the shared SDK implementation.
- The secret-guard program is adversarial boundary evidence. Its raw scanner/RPC implementation must not be replaced with bridgesdk.Peer if doing so would stop testing malformed or hostile protocol behavior.
- Integration tests and testdata are exempt from the production 500-line cap, but not from error handling, deterministic cleanup, race freedom, or contract clarity.

## 4. Transferable Patterns

Several patterns are already strong and should be preserved while fixing the mismatches:

- Consumer-owned narrow interfaces. The webhook resolver/dialer seams in internal/bridgesdk/webhook_http.go:14-24 and marketplace Source/Notifier boundaries keep network and notification tests local. Extend these interfaces only for a concrete lifecycle or typed-dial need; do not create provider-wide abstractions.
- Typed hook dispatch rather than a generic event bus. HookEvent, typed payloads, generic pipeline[P, R], and explicit descriptor tables preserve compile-time payload ownership. The consistency duplication needs a better source of truth, but the typed architecture itself is correct.
- Normalize, validate, then copy at wire boundaries. DeliveryMode.Normalize deliberately accepts established aliases at internal/bridges/contract/delivery_types.go:18-24, while constructors and clone helpers prevent caller-owned slices/maps from leaking. Preserve those accepted literals unless a coordinated hard-cut contract change updates runtime, SDK, scaffolds, tests, and documentation together.
- Deterministic materialized registries. Host methods and hook events have stable order for SDK/codegen output. Keep deterministic slices at contract edges even if internal iteration helpers become lazy.
- Bounded untrusted input. Peer limits frames, marketplace limits response bytes, and the webhook client validates every resolved IP before dialing. The OpenRoot and typed-dial changes should strengthen these boundaries without weakening size, SSRF, redirect, or symlink controls.
- Explicit lifecycle owners. Manager, Broker, ProviderLifecycle, asyncPool, and CatalogService each have an identifiable owner. The repair is to complete their state machines—acquire, stop, join, compensate—not to replace them with global goroutines or an event bus.
- Error aggregation on cleanup. Source and marker code already uses named returns and errors.Join in several places. Generalize that shape so the primary failure remains visible while shutdown, rollback, close, and persistence failures are retained.

The transferable implementation shape is:

1. Split Phase: make preparation pure where possible, then make acquisition/commit explicit.
2. Encapsulate Variable: keep running/closing/closed and transport error state under the same lock as work admission.
3. Extract Function: centralize bounded detached cleanup contexts and drain/close handling.
4. Introduce Parameter Object: group lifecycle timeouts, hook-run failure data, and provider build inputs rather than growing positional APIs.
5. Remove Dead Code and Combine Functions into Transform only after contract/codegen ownership is proven.

## 5. Risks / Mismatches

**F-01 — Extension startup acquires grants and source-session state without a complete rollback.** Severity: high. Confidence: high. Fowler: Split Phase plus Extract Function.

startOne executes validate, register, and initialize as independent early-return phases at internal/extension/manager_supervision.go:18-35. Validation mutates CapabilityChecker through RegisterForSession at lines 96-110. Runtime launch activates a resource source session before method registration and initialization at internal/extension/manager_runtime_launch.go:43-76,105-130,362-375. Its failure helper only runs redaction cleanup and process shutdown at lines 89-101. Capability unregister and source reset exist only in the later unregister path at internal/extension/manager_failure_state.go:175-203.

An error while loading resources, registering a Host API method, initializing the subprocess, or validating supported hook events can therefore leave authority and/or an activated source session associated with a generation that never became active. Refactor startup into a prepared activation and an acquisition transaction whose cleanup stack is registered immediately after each side effect and executed in reverse order under a bounded cleanup context. Host API initialization may require temporary authority, so “register only at the end” is insufficient; explicit rollback is required. During recovery, source-session rollback must be generation-aware so a failed new generation cannot erase a still-authoritative previous projection. The owning manager suite should prove the invariant: every failed phase leaves no process, grant, activated source session, handlers, or registered resources.

**F-02 — bridgesdk.Peer lacks a coherent terminal state, cancellable read ownership, nil-I/O validation, and a panic boundary.** Severity: high. Confidence: high. Fowler: Encapsulate Variable plus Introduce Special Case and Change Function Declaration.

NewPeer accepts unchecked Reader/Writer values at internal/bridgesdk/peer.go:67-77. Call inserts into pending without checking whether serving has ended at lines 102-139. Serve checks ctx only after Scanner.Scan returns at lines 167-181, so cancellation cannot unblock an open reader. Handler goroutines start directly at lines 201-208; a provider handler panic terminates the process. failTransport records an error and closes the pending map at lines 342-350, but a racing or subsequent Call can insert a new channel after that close and wait until its own context expires.

Add one terminal state guarded with pending admission, reject calls immediately after EOF/failure/close, and close pending calls exactly once with the terminal cause. Define who owns and can close the input so Serve cancellation unblocks Scan. Contain handler panics at the RPC boundary and return an internal RPC error while recording the panic; only after that is WaitGroup.Go safe. Validate nil I/O through a coordinated SDK constructor change rather than allowing a later panic. Because this is a source-level SDK contract, update all Go scaffolds, reference providers, and conformance tests in the same hard cut. Tests should prove cancellation joins the read loop, post-terminal calls fail immediately, concurrent failure cannot orphan pending calls, and a handler panic does not terminate the provider.

**F-03 — Marketplace refresh notification becomes unbounded and can strand Close waiters.** Severity: high. Confidence: high. Fowler: Extract Function plus Introduce Parameter Object.

Failure persistence correctly reattaches refreshTimeout after WithoutCancel at internal/marketplace/service.go:352-353. Notification instead calls NotifyCatalogRefresh with bare context.WithoutCancel at line 374, which removes cancellation and the original deadline. Close starts a goroutine waiting on flightWG at internal/marketplace/service_lifecycle.go:13-25; it may return on its caller deadline, but the refresh and close-wait goroutines remain if a notifier never returns.

Create one bounded detached-operation helper and apply it to notification, failure persistence, and any must-attempt lifecycle work. The timeout should be explicit service configuration or a documented reuse of refreshTimeout. The existing service suite should use a notifier that waits on ctx.Done and prove Close converges without a lingering refresh flight.

**F-04 — Error classes depend on message text in marketplace and development activation.** Severity: high. Confidence: high. Fowler: Replace Primitive with Object plus Replace Conditional with Polymorphism.

Marketplace classification falls back to strings.Contains over “decode JSON”, “validate”, “is required”, and “unknown field” at internal/marketplace/service.go:447-453. The strings originate across strict decode and validation layers in internal/marketplace/source.go:130-217. A wording change can silently alter durable failure class.

The extension development startup has a more concrete misclassification: internal/extension/manager_dev_lifecycle.go:203-218 treats any error containing “development origin” as missing-origin. Errors for an origin escaping the workspace and an origin that is not a directory contain that phrase at internal/extension/dev_generation.go:40-61, so policy/shape failures can be reported as missing state.

Introduce typed decode and validation wrappers for marketplace and use errors.AsType/Is exclusively. Introduce distinct sentinels or typed errors for missing, outside-workspace, wrong-type, and invalid generation failures. Tests should assert classification from error identity while deliberately varying message text.

**F-05 — HTTPSource closes but does not safely drain early response paths.** Severity: medium. Confidence: high. Fowler: Extract Function plus Move Function.

HTTPSource defers Close at internal/marketplace/source.go:108-114, then returns immediately for non-2xx status or oversized Content-Length at lines 115-120 before consuming the body. This reduces connection reuse and leaves cleanup behavior inconsistent across outcomes. Add a shared bounded drain-and-close helper that respects the request deadline, retains the primary status/size error, and joins a close error. Do not unboundedly download a hostile body merely to reuse a connection; drain only within the configured response budget and close otherwise. The source suite should use a recording body to prove reads and Close occur on success, status failure, oversize, and decode failure.

**F-06 — Hook subprocess registry completion uses unbounded Background contexts.** Severity: high. Confidence: high. Fowler: Extract Function plus Introduce Parameter Object.

Normal completion, interruption checkpoint, and terminal completion call registry operations with context.Background at internal/hooks/executor_subprocess_lifecycle.go:38-54. Process termination itself is bounded, but a blocked registry write can prevent a hook from returning after the process has exited or been killed.

Use a bounded context derived from context.WithoutCancel(ctx) for must-attempt checkpoint/completion work, retain shutdown and persistence failures with errors.Join, and put the timeout in hook executor configuration. The existing subprocess lifecycle suite should inject a registry that waits for cancellation and prove every success, cancellation, forced-kill, and persistence-failure path joins.

**F-07 — Bundle activation compensation discards cancellation and deadlines.** Severity: high. Confidence: high. Fowler: Extract Function plus Introduce Parameter Object.

rollbackActivationAndReconcileLocked creates context.WithoutCancel(ctx) and uses it for rollback and reconciliation at internal/bundles/service.go:211-225. WithoutCancel has no deadline, so a storage or projection failure can turn compensation into unbounded work. internal/bundles/resource_store.go:364-368 demonstrates a partial better pattern by reattaching an existing deadline, although it still needs a default when no caller deadline exists.

Define a bounded compensation context whose deadline is the earlier of an explicit compensation timeout and any still-useful caller deadline. Apply the same policy to rollback, reconcile, and failure recording, and join every failure. The canonical bundle service tests should cover canceled callers, blocked rollback, blocked reconciliation, and rollback failure while preserving the original activation error.

**F-08 — ProviderLifecycle ignores caller contexts and its Wait can outlive callers or return before admission closes.** Severity: high. Confidence: high. Fowler: Encapsulate Variable plus Change Function Declaration.

Initialize discards its context and creates a hard-coded 15-second Background timeout at internal/bridgesdk/provider_lifecycle.go:17,99-115. Shutdown also discards its context and uses Background unless the wire request includes DeadlineMS at lines 231-252. Wait starts a new goroutine per call at lines 294-309. Because Go admission is separate, calling Wait before Stop can race with later task admission or return while future work is still legal.

Move initialization and shutdown defaults into ProviderLifecycleConfig. Initialize should honor the supplied context under the configured ceiling. Shutdown should use the minimum of caller, request, and configured deadlines, with a bounded default. Closing admission and stopping must precede a definitive join; expose one lifecycle-owned done channel rather than spawning a waiter for each call. Any public Go callback also needs a documented no-panic rule or an SDK panic boundary. Tests should cover Initialize cancellation, zero DeadlineMS, repeated Wait, Wait-before-Stop, and a task that ignores cancellation.

**F-09 — Broker.Close does not close admission and can race WaitGroup.Add with Wait.** Severity: high. Confidence: high. Fowler: Encapsulate Variable plus Introduce Special Case.

Close cancels and waits at internal/bridges/delivery_broker.go:80-95 but records no closed state. Register, delivery, resume, and replay paths can still call ensureRouteLocked at lines 186,266,405. ensureRouteLocked can add a worker and start a goroutine at internal/bridges/delivery_route_worker.go:9-33. Concurrent post-close work can therefore mutate state after flush, create a worker under an already-canceled lifecycle, or race positive Add against Wait.

Store running/closing/closed under the same mutex used for route admission, return an explicit ErrBrokerClosed from all work-admitting APIs, close admission before cancel/wait, and make Close idempotent. Once the invariant is established, internal worker starts are good WaitGroup.Go candidates. The broker suite should race Close with register/deliver/resume and assert no post-close worker, ledger mutation, blocked waiter, or race-detector finding.

**F-10 — Provider integration suites duplicate build/poll/server machinery and discard real errors.** Severity: high. Confidence: high. Fowler: Extract Function plus Introduce Parameter Object.

Eight provider suites repeat sync.Once build globals and nearly identical go build functions: internal/extension/discord_provider_integration_test.go:35,314-333; internal/extension/gchat_provider_integration_test.go:48,270-293; internal/extension/github_provider_integration_test.go:39,246-273; internal/extension/linear_provider_integration_test.go:37,230-257; internal/extension/slack_provider_integration_test.go:37,276-299; internal/extension/teams_provider_integration_test.go:42,359-382; internal/extension/telegram_provider_integration_test.go:35,315-338; and internal/extension/whatsapp_provider_integration_test.go:37,290-312. Discord uniquely uses unbounded exec.Command at line 318.

The same suites discard response-body Close, JSON decode/encode, io.WriteString, and hash Write errors—for example internal/extension/linear_provider_integration_test.go:363-587 and internal/extension/teams_provider_integration_test.go:519-704—contrary to the repository's no-discarded-errors invariant. Extract one extensiontest provider builder with timeout, immutable inputs, captured output, and optional OnceValues caching. Extract strict HTTP test response/body helpers. Use channel barriers for in-process concurrency, synctest only for virtualizable timers, and bounded polling for external providers. Keep these invariants in the existing provider integration suites rather than duplicating standalone regressions.

**F-11 — The positive telegram reference bypasses shared provider lifecycle and marker code.** Severity: medium. Confidence: high. Fowler: Replace Inline Code with Function Call plus Extract Function.

internal/extension/testdata/telegram-reference/main.go declares its own environment keys and marker records at lines 27-35 and 140-170, and its own initialize/background/shutdown lifecycle around lines 180-310. It uses bridgesdk.NewRuntime at line 187 but does not use ProviderLifecycle or AdapterMarkers, whose shared contract lives in internal/bridgesdk/provider_lifecycle.go and internal/bridgesdk/provider_markers.go:29-196. A defect in the shared lifecycle or marker implementation can therefore coexist with a passing “reference” adapter.

Refactor the positive reference to use shared SDK lifecycle, markers, HTTP ownership, retry, and target helpers, retaining only Telegram-specific behavior. In contrast, keep internal/extension/testdata/secret-guard/main.go independent because it is an adversarial raw-protocol fixture; handle or explicitly justify its discarded Fprintf/Close errors at lines 400-409 and 573-647 without routing hostile frames through the implementation under test.

**F-12 — Contract registries require shotgun edits and contain panic/dead-code candidates.** Severity: medium. Confidence: medium. Fowler: Combine Functions into Transform plus Remove Dead Code.

A Host API addition currently touches constants and ordered lists in internal/extensionprotocol/host_api.go:3-199, aliases in internal/extension/contract/host_api.go:29-120, and typed method specs in internal/extension/contract/host_api_method_registry.go:14-475. This is intentional leaf-package isolation, but it remains shotgun surgery. Generate all derived constants/lists/aliases/spec partitions from one declarative contract source while preserving package direction, method spelling, and wire order.

Hooks similarly maintain allHookEvents and hookEventSpecs separately, then enforce consistency with a package-init panic at internal/hooks/events_catalog.go:5-92. BuildHookContracts already returns an error, while the panic wrapper HookContracts at internal/extension/contract/sdk_contract_builder.go:10-38 is referenced only by tests inside the authorized slice. Derive ordered lookup structures from one source so inconsistency is unrepresentable. Before deleting HookContracts, perform a repository-wide use search outside this slice; if it truly has no production caller, remove it and keep the error-returning builder.

**F-13 — Near-cap production files and a hook data clump will make the fixes regress architecture.** Severity: medium. Confidence: high. Fowler: Extract Class plus Introduce Parameter Object.

internal/bundles/resource_projection.go:19-478 contains plan types, Build, owner-map/count derivation, Apply, desired-state resolution, capacity estimation, and clone helpers. Split planning/ownership from application before adding compensation logic. internal/marketplace/service.go is 459 lines; move typed error classification and bounded notification into named files as part of F-03/F-04. internal/bridgesdk/runtime.go is 481 lines; split protocol command groups before extending lifecycle APIs. The 479-line host method registry should be split by generated contract family rather than hand-partitioned.

finishHookFailure takes context, payload, hook, depth, two time values, raw patch, trace, outcome, run error, and return error at internal/hooks/pipeline.go:229-241. Introduce a hookRunFailure parameter object owned by the pipeline package so telemetry/trace changes do not expand every call site. Do not move unrelated responsibilities into a generic helper package.

**F-14 — Managed install containment is vulnerable to path re-resolution races.** Severity: medium. Confidence: medium-high. Fowler: Encapsulate Variable plus Move Function.

The installer carefully validates package names, resolves symlinks, rejects escapes, and tracks cycles in internal/extension/install_managed_tree.go:14-78, internal/extension/install_managed_path.go:11-58, and internal/extension/install_managed_package.go:12-185. It then reopens resolved path strings with os.Open/Stat/ReadDir. An attacker able to mutate the source tree between validation and open can swap a symlink or directory entry.

Make an os.Root the authority for all relative source operations and, where useful, a separate Root for the private staging target. Preserve the existing behavior of dereferencing allowed in-root symlinks and rejecting escape/cycles; OpenRoot is not a replacement for semantic package-name validation. Add a race-oriented filesystem test that swaps a symlink between discovery and open and proves the copy cannot escape the root.

**F-15 — Several persistence/API conversions rely on unproven int width.** Severity: low-medium. Confidence: medium.
Fowler: Extract Function plus Change Function Declaration.

Marketplace reads int64 database values into int fields at internal/marketplace/store.go:178-180. Host API task payloads convert child/dependency counts and attempts to int at internal/extension/host_api_task_payloads.go:35-36,95. Manifest validation converts json error offsets to int before clamping at internal/extension/validate_bundle.go:101-115,145-156. These are safe on expected 64-bit systems and for normally validated data, but the invariant is implicit and corrupted storage or a supported 32-bit build can overflow.

Either align durable/API count fields on int64 or centralize checked conversion and propagate an error. Clamp JSON offsets in int64 space before conversion. Confirm supported architectures before deciding whether the database cases are actionable. No broad alias leak was found: constructors and outward-facing helpers generally clone maps/slices, and that behavior should remain covered.

No other production discarded-error site was found by the scoped scan. The only non-test, non-fixture blank-identifier initializer is the hook consistency panic described in F-12. No broad dead subsystem was demonstrated; HookContracts is the sole localized candidate, subject to a repository-wide use check by the parent.

## 6. Open Questions

1. Does the resource source-session store support generation-scoped deactivate/rollback, or only broad ResetSource? A transactional manager fix must not let a failed recovery generation delete valid state from another generation.
2. Does bridgesdk.Peer own stdin in every supported provider runtime, and can it close that reader on cancellation? If not, which source-compatible ownership hook should the SDK expose while preserving the JSON-RPC wire contract?
3. Can ProviderHTTPServer bind to public/browser-reachable addresses in supported deployments, and are there legitimate browser-originated webhook calls? This determines whether CrossOriginProtection is default-on or an explicit policy mode.
4. Must provider test binaries and marker evidence stay at fixed repository-relative paths because manifests reference them, or may extensiontest copy manifests/binaries into T.ArtifactDir and inject the resolved paths?
5. Which artifact is authoritative for Host API generation: extensionprotocol constants, the typed method-spec table, or an external schema? The registry refactor should generate from the existing owner rather than create a second source.
6. Are 32-bit targets supported for the daemon, CLI, generated SDK validation, or tests? If not, record that platform invariant; if yes, F-15 requires checked conversions before modernization closes.
7. Does any production package outside the authorized slice call contract.HookContracts? A global use search is required before removing the panic wrapper.

## 7. Evidence

The following evidence paths are real, readable sources and are listed once here even when cited multiple times above.

- internal/extension/manager.go
- internal/extension/manager_lifecycle.go
- internal/extension/manager_supervision.go
- internal/extension/manager_runtime_launch.go
- internal/extension/manager_failure_state.go
- internal/extension/manager_process_lifecycle.go
- internal/extension/capability.go
- internal/extension/build_runner.go
- internal/extension/install_managed_tree.go
- internal/extension/install_managed_package.go
- internal/extension/install_managed_path.go
- internal/extension/semantic_version.go
- internal/extension/host_api_bridges_prompt.go
- internal/extension/validate_bundle.go
- internal/extension/marketplace_helpers.go
- internal/extension/manager_dev_lifecycle.go
- internal/extension/dev_generation.go
- internal/extension/scaffold.go
- internal/extension/contract/host_api.go
- internal/extension/contract/host_api_method_registry.go
- internal/extension/contract/sdk_contract_builder.go
- internal/extension/discord_provider_integration_test.go
- internal/extension/gchat_provider_integration_test.go
- internal/extension/github_provider_integration_test.go
- internal/extension/linear_provider_integration_test.go
- internal/extension/slack_provider_integration_test.go
- internal/extension/teams_provider_integration_test.go
- internal/extension/telegram_provider_integration_test.go
- internal/extension/whatsapp_provider_integration_test.go
- internal/extension/testdata/telegram-reference/main.go
- internal/extension/testdata/secret-guard/main.go
- internal/extensionprotocol/host_api.go
- internal/extensiontest/bridge_adapter_harness.go
- internal/extensiontest/perf_bench_test.go
- internal/bridges/delivery_broker.go
- internal/bridges/delivery_route_worker.go
- internal/bridges/delivery_broker_metrics.go
- internal/bridges/catalog_records.go
- internal/bridges/target.go
- internal/bridges/contract/delivery_types.go
- internal/bridges/contract/targets.go
- internal/bridgesdk/peer.go
- internal/bridgesdk/runtime.go
- internal/bridgesdk/provider_lifecycle.go
- internal/bridgesdk/provider_http.go
- internal/bridgesdk/webhook_http.go
- internal/bridgesdk/retry.go
- internal/bridgesdk/batching.go
- internal/bridgesdk/batching_test.go
- internal/bridgesdk/provider_markers.go
- internal/bridgesdk/errors.go
- internal/bundles/service.go
- internal/bundles/resource_projection.go
- internal/bundles/resource_store.go
- internal/hooks/events.go
- internal/hooks/events_catalog.go
- internal/hooks/pipeline.go
- internal/hooks/pool.go
- internal/hooks/executor_subprocess_lifecycle.go
- internal/hooks/payloads_task_loop.go
- internal/marketplace/service.go
- internal/marketplace/service_lifecycle.go
- internal/marketplace/source.go
- internal/marketplace/store.go
- internal/marketplace/service_test.go
