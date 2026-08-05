## Scope

This slice evaluates Go-modernization opportunities and adjacent runtime gaps in:

- internal/deadentity
- internal/events
- internal/hooks
- internal/logger
- internal/loop
- internal/reasoning
- internal/speed
- internal/testutil
- internal/transcript

The review covers the twelve modernization topics in the supplied review plus eight baseline language/library topics, for twenty topics total. It also records correctness or architecture issues discovered while tracing those mechanisms. This is a bounded slice analysis, not a repository-wide synthesis.

The scanned surface contains 426 Go files and 96,147 lines: 331 production files with 50,568 lines and 95 test files with 45,579 lines. No production file in the slice exceeds the 500-line limit. The largest production files are close enough to the limit that they must not grow without extraction.

## Overview

The slice is already using several current Go mechanisms: errors.AsType, testing.B.Loop, json omitzero, os.OpenRoot, strings.SplitSeq, sync.WaitGroup.Go, range-over-integer loops, and slices/maps/min/max helpers all have real uses. The remaining work is therefore targeted modernization, not a bulk syntax conversion.

The strongest low-risk modernization candidates are:

1. Convert the remaining benchmark loop in internal/loop/coordinator_watch_test.go:840 to testing.B.Loop.
2. Replace manual key collection and sorting in internal/hooks/executor_subprocess_capture.go:26 with slices.Sorted(maps.Keys(...)), preserving deterministic environment ordering.
3. Use strings.Lines or an equivalent standard iterator in transcript pruning, while locking down blank-line, trailing-newline, and CRLF behavior currently rooted at internal/transcript/prune.go:140.
4. Convert manual Add/go/Done worker launches to sync.WaitGroup.Go where callback panic containment is already guaranteed, especially internal/hooks/pool.go:97 and internal/testutil/acpmock/cmd/acpmock-driver/sandbox.go:66.
5. Use testing/synctest only for pure-Go timer ownership in the hook pool tests at internal/hooks/pool_test.go:198, internal/hooks/pool_test.go:217, and internal/hooks/pool_test.go:401. Subprocess, HTTP, Unix-socket, and end-to-end timing must remain on real clocks.
6. Move Windows process-handle adoption to the shared process owner, internal/procutil, instead of adding local hooks or testutil wrappers.
7. Use net.Dialer.DialUnix for the Unix-domain-socket transport created at internal/testutil/e2e/runtime_harness.go:252, retaining context cancellation and HTTP transport behavior.
8. Adopt testing.T.ArtifactDir and testing.T.Attr for the end-to-end artifact collector if their retention semantics can preserve the existing pass/fail contract; do not replace durable daemon logs with testing.T.Output.

More important than the syntax modernization are two teardown defects:

- RuntimeHarness.Stop memoizes the first stop result forever. A canceled first caller can be treated as success before the process exits, after which later cleanup cannot retry. The interaction is visible between internal/testutil/e2e/runtime_harness_process.go:28 and internal/testutil/e2e/runtime_harness_process.go:48.
- Hook subprocess cancellation performs an unbounded receive from waitCh after a kill attempt. If group termination fails or leaves the process alive, the cancellation path can hang before the force-kill fallback runs at internal/hooks/executor_subprocess_lifecycle.go:61 and internal/hooks/executor_subprocess_lifecycle.go:94.

The package surface is live rather than orphaned. All nine scoped packages have out-of-directory consumers. There are no proven dead packages in this slice. The material architecture questions are lifecycle ownership, process cleanup, workspace-keyed state growth, and whether asynchronous hooks should inherit request cancellation.

## Mechanisms / Patterns

### Lifecycle and cancellation ownership

The hook pool owns bounded concurrency through a worker queue and a pool context. Workers are created through manual WaitGroup accounting at internal/hooks/pool.go:97 and execute tasks at internal/hooks/pool.go:169. Shutdown waits for those workers at internal/hooks/pool.go:189. WaitGroup.Go is a mechanical improvement here only because task execution already contains panic recovery; the callback-must-not-panic invariant remains mandatory.

Asynchronous hooks currently derive from the caller's cancellation at internal/hooks/dispatch_async.go:78. This is not accidental: internal/hooks/hooks_test.go:1162 and internal/hooks/hooks_test.go:1203 assert that parent cancellation propagates. That behavior conflicts with the runtime rule that work intentionally outliving an HTTP or UDS request should detach from request cancellation. Changing it is a product/runtime decision, not a syntax cleanup.

Subprocess lifecycle is split between hooks/test infrastructure and internal/procutil. Hooks registers each command with procutil at internal/hooks/executor_subprocess_lifecycle.go:21. The Windows implementation currently reopens the child by PID in internal/procutil/process_group_windows.go:31. That is the correct ownership boundary for os.Process.WithHandle and ErrNoHandle: the shared process-group layer should own native handle lifetime, PID-reuse protection, and platform parity.

The end-to-end RuntimeHarness has a separate process lifecycle. Its stop-once field is declared at internal/testutil/e2e/runtime_harness.go:103 and consumed at internal/testutil/e2e/runtime_harness_process.go:28. The current Once pattern conflates idempotence with permanent memoization of a potentially incomplete cleanup attempt.

### Registries, snapshots, and determinism

The events registry exposes sorted snapshots rather than mutable iterators. Sorting is explicit in internal/events/registry_queries.go:9, internal/events/registry_queries.go:48, and internal/events/registry_queries.go:73. That deterministic, cloned-snapshot contract is more useful than introducing a new exported iter.Seq API for a small registry.

Hook subprocess environment capture also depends on deterministic ordering. internal/hooks/executor_subprocess_capture.go:26 manually gathers map keys and sorts them. slices.Sorted(maps.Keys(...)) expresses the same invariant with less local machinery.

The event registry consists of package-owned constant event names and immutable registry records rooted at internal/events/registry.go:386. Interning these values with unique.Handle would add identity machinery without demonstrated allocation pressure and would risk leaking representation changes into JSON, persistence, hooks, and workspace identifiers.

### Filesystem and stream handling

The loop source store already uses os.OpenRoot at internal/loop/source_store.go:164 and opens relative content through the rooted handle at internal/loop/source_store.go:173. It also joins close errors and rejects symlink traversal. This is a pattern to retain, not a migration gap.

Transcript pruning currently materializes all lines with strings.Split at internal/transcript/prune.go:140. A standard line iterator can avoid the allocation when only a count and prefix are needed, but its newline semantics must match the existing transcript contract. The nearby firstNonEmpty helper at internal/transcript/transcript_payload.go:24 deliberately trims only to test emptiness and returns the original string; cmp.Or is not semantically equivalent.

The Unix-domain-socket HTTP client installs a generic DialContext callback at internal/testutil/e2e/runtime_harness.go:252. A pre-resolved net.UnixAddr plus Dialer.DialUnix would make the transport intent explicit while preserving context propagation.

### Test time and artifact ownership

The hook pool tests use real 40–50 ms timing windows at internal/hooks/pool_test.go:198, internal/hooks/pool_test.go:217, and internal/hooks/pool_test.go:401. Those tests exercise Go timers, contexts, channels, and goroutines without an external process, making them suitable for testing/synctest.

Other timer uses in the slice drive subprocesses, Unix sockets, HTTP servers, or end-to-end agents. Virtual time cannot make those external systems advance and would make their tests misleading.

The end-to-end artifact collector creates its own temporary directory at internal/testutil/e2e/artifacts.go:247, retains the directory on failure at internal/testutil/e2e/artifacts.go:253, and removes it on success at internal/testutil/e2e/artifacts.go:257. Any move to testing.T.ArtifactDir must preserve that exact diagnostic contract and the bootstrap manifest's teardown evidence.

### Workspace-scoped state

Dead-entity state is keyed by workspace and entity identity. Workspace propagation into emitted events occurs at internal/deadentity/events.go:39, and the isolation invariant is exercised at internal/deadentity/service_test.go:251. The service maintains an in-memory per-key map at internal/deadentity/service.go:40 and inserts new state at internal/deadentity/service.go:230 without an observed eviction path. A naive eviction would be unsafe because concurrent callers can still hold the per-key state object.

## Relevant Sources

### Mechanical inventory

| Directory | Go files | Production | Tests | Total lines | Production lines | Test lines | Production files over 500 lines |
|---|---:|---:|---:|---:|---:|---:|---:|
| internal/deadentity | 4 | 3 | 1 | 953 | 461 | 492 | 0 |
| internal/events | 11 | 10 | 1 | 955 | 664 | 291 | 0 |
| internal/hooks | 109 | 80 | 29 | 21,306 | 11,063 | 10,243 | 0 |
| internal/logger | 4 | 2 | 2 | 499 | 239 | 260 | 0 |
| internal/loop | 201 | 168 | 33 | 47,285 | 25,667 | 21,618 | 0 |
| internal/reasoning | 2 | 1 | 1 | 122 | 61 | 61 | 0 |
| internal/speed | 2 | 1 | 1 | 189 | 111 | 78 | 0 |
| internal/testutil | 70 | 46 | 24 | 19,201 | 9,229 | 9,972 | 0 |
| internal/transcript | 23 | 20 | 3 | 5,637 | 3,073 | 2,564 | 0 |
| Total | 426 | 331 | 95 | 96,147 | 50,568 | 45,579 | 0 |

The largest production files are internal/loop/control_plan.go at 494 lines, internal/loop/linter_references.go at 493, internal/testutil/e2e/runtime_harness_sessions.go at 467, internal/loop/linter.go at 457, internal/loop/goal/route.go at 453, internal/hooks/hooks.go at 451, internal/testutil/e2e/config_seed.go at 451, and internal/events/registry.go at 449.

### External import surface

The bounded import scan found the following out-of-directory import-reference hits: deadentity 11, events 84, hooks 267, logger 4, loop 366, reasoning 6, speed 46, testutil 367, and transcript 81. These are reference hits rather than a count of distinct packages, but they prove that none of the nine package surfaces is locally orphaned.

Representative runtime consumers are internal/daemon/boot_dead_entity.go:6 for deadentity, internal/observe/event_content.go:7 for events, internal/config/hooks.go:10 for hooks, internal/daemon/boot_config.go:10 for logger, internal/api/core/loop_catalog.go:7 for loop, internal/config/roles.go:8 for reasoning, internal/acp/speed_config.go:10 for speed, and internal/session/transcript.go:11 for transcript.

### Primary implementation sources

- Dead-entity ownership and state: internal/deadentity/service.go:40, internal/deadentity/service.go:230, internal/deadentity/events.go:39.
- Event registry and deterministic query surface: internal/events/registry.go:386 and internal/events/registry_queries.go:9.
- Hook concurrency and context lifetime: internal/hooks/pool.go:97, internal/hooks/dispatch_async.go:78, and internal/hooks/hooks_test.go:1162.
- Hook subprocess lifecycle: internal/hooks/executor_subprocess_lifecycle.go:21, internal/hooks/executor_subprocess_lifecycle.go:61, and internal/hooks/executor_subprocess_lifecycle.go:94.
- Loop filesystem confinement and error matching: internal/loop/source_store.go:164, internal/loop/source_store.go:173, internal/loop/goal/executor.go:215, and internal/loop/linter.go:424.
- End-to-end artifact and process lifecycle: internal/testutil/e2e/artifacts.go:247, internal/testutil/e2e/runtime_harness.go:103, and internal/testutil/e2e/runtime_harness_process.go:28.
- Transcript pruning and fallback semantics: internal/transcript/prune.go:140 and internal/transcript/transcript_payload.go:24.

## Transferable Patterns

### Twenty-topic modernization matrix

Every row names the disposition, expected benefit, protected invariant or risk, implementation owner, and canonical suite. APPLY means a bounded implementation is justified. REJECT means the mechanism is not semantically suitable for this slice. DEFER means the mechanism may be useful but needs evidence or an owning-layer decision first.

| Topic | Decision | Benefit | Risk or invariant | Owner | Canonical suite |
|---|---|---|---|---|---|
| errors.AsType | APPLY selectively | Removes target boilerplate and makes concrete error extraction type-safe. | Preserve wrapped-chain matching and do not replace interface-target or special errors.As semantics mechanically. | internal/loop owning error paths | Existing loop goal executor and linter package suites; no standalone syntax test. |
| testing.B.Loop | APPLY | Removes manual benchmark loop bookkeeping and matches current benchmark semantics. | Benchmark body and setup boundaries must remain identical. | internal/loop | internal/loop/coordinator_watch_test.go benchmark suite. |
| json omitzero | REJECT blanket conversion; retain current targeted uses | Existing uses correctly express zero-value omission for selected fields. | omitempty and omitzero differ for pointers, collections, interfaces, and wire compatibility; schema changes require contract co-ship. | Owning DTO packages in hooks and loop | Existing JSON round-trip/contract suites in the owning package. |
| os.OpenRoot | APPLY as a retained pattern | Keeps source-store file access beneath an explicit filesystem root. | Preserve symlink refusal, relative-path validation, and joined close errors. | internal/loop source store | Existing source-store suite in internal/loop. |
| strings.SplitSeq, FieldsSeq, Lines | APPLY to transcript line traversal and retain current SplitSeq uses | Avoids a full []string allocation in count/prefix operations. | Preserve empty lines, trailing newline count, CRLF handling, and returned original text. | internal/transcript | Existing transcript pruning suite. |
| sync.WaitGroup.Go | APPLY to bounded worker launches | Makes Add/go/Done ownership atomic and easier to audit. | Functions passed to Go must not panic; retain hook panic recovery and shutdown ordering. | internal/hooks pool and internal/testutil/acpmock driver | internal/hooks/pool_test.go and internal/testutil/acpmock/fixture_test.go. |
| Range over integers | APPLY opportunistically; no migration campaign | Keeps counter loops compact and is already broadly adopted. | Do not rewrite loops whose index mutation or unusual bounds carry semantics. | Local owning packages | Existing owning package suite. |
| slices, maps, min, max | APPLY to the deterministic hook environment path and retain existing uses | Replaces manual key materialization/sort code while keeping intent explicit. | Output ordering must stay stable for process environments and tests. | internal/hooks subprocess capture | Existing hook subprocess capture/executor suite. |
| testing/synctest | APPLY only to pure-Go hook-pool timing tests | Eliminates real-clock sleeps and reduces timing flakiness. | Do not virtualize subprocess, network, UDS, HTTP, or end-to-end clocks. | internal/hooks tests | internal/hooks/pool_test.go. |
| iter.Seq and custom range functions | DEFER exported APIs; use standard sequences locally | Standard sequences can reduce temporary allocations without inventing new public abstractions. | Event registry callers rely on deterministic sorted cloned snapshots; lazy iteration can expose mutation and ordering hazards. | internal/events and local call sites | Existing internal/events registry suite. |
| os.Process.WithHandle and ErrNoHandle | APPLY in internal/procutil, not in hooks/testutil wrappers | Uses the handle returned for the actual child and removes the PID-reopen/PID-reuse window on Windows. | One layer must own handle closure; preserve job-object, process-group, and Unix parity. | internal/procutil | internal/procutil/procutil_test.go plus internal/hooks/executor_subprocess_windows_test.go. |
| sync.OnceValue and sync.OnceFunc | DEFER; REJECT direct conversion of RuntimeHarness.Stop | OnceFunc may shorten one cancellation-only site, but offers little material benefit. | Stop cleanup must be retryable after an incomplete/canceled attempt; OnceValue would preserve the current defect. | internal/hooks pool and internal/testutil/e2e lifecycle | internal/hooks/pool_test.go and internal/testutil/e2e/runtime_harness_lifecycle_test.go. |
| math/rand/v2 | REJECT for this slice | No v1 math/rand use exists in the nine scoped directories, so there is no local migration benefit. | Do not create random behavior or change reproducibility without an owning use case. | Out-of-scope owners such as retry/bridge SDK, if separately reviewed | Owning package deterministic/backoff suite outside this slice. |
| cmp.Or | REJECT | No local fallback chain has exact zero-value semantics that improves with cmp.Or. | firstNonEmpty trims only for the emptiness test and returns original text; duration defaults may treat all non-positive values as absent. | internal/transcript or owning config package | Existing transcript payload suite. |
| testing.T.ArtifactDir, T.Attr, T.Output | APPLY ArtifactDir/Attr after retention validation; REJECT T.Output for durable daemon logs | Standardizes artifact discovery and exposes structured test metadata to CI. | Passing runs must clean up, failing runs must retain evidence, and manifests/log paths must remain durable. | internal/testutil/e2e | internal/testutil/e2e/artifacts_test.go and internal/testutil/e2e/runtime_harness_test.go. |
| net/http.CrossOriginProtection | DEFER to internal/api/httpapi | Could add browser-origin defense in depth around unsafe methods. | Must remain compatible with custom CORS, no-Origin CLI/UDS clients, localhost SPA calls, OpenAI-compatible endpoints, and DNS-rebinding protections. | internal/api/httpapi | internal/api/httpapi/middleware_refac_test.go plus browser/API end-to-end coverage. |
| runtime/trace.NewFlightRecorder | DEFER | A bounded pre-failure trace could materially improve daemon incident diagnosis. | Requires explicit trigger, redaction, byte/time budget, concurrency policy, storage, cleanup, and agent-manageable export. | internal/diagnostics and daemon; testutil only captures evidence | Diagnostics unit/integration suite plus internal/testutil/e2e artifact suite. |
| net.Dialer.DialUnix and DialTCP | APPLY DialUnix to the UDS harness client | Makes Unix-socket intent and address type explicit. | Preserve context cancellation, HTTP transport pooling, socket cleanup, and non-TCP behavior. | internal/testutil/e2e RuntimeHarness | Existing RuntimeHarness transport/lifecycle suite. |
| bytes.Buffer.Peek | REJECT | Scoped buffers are write accumulators or template builders; no framing parser benefits from peeking. | Adding Peek would couple write-only code to buffer internals without removing copies or parsing work. | None in this slice; JSON-RPC framing owner if separately profiled | Owning protocol parser suite outside this slice. |
| unique.Handle | REJECT pending heap evidence | No demonstrated allocation or comparison bottleneck justifies global interning. | Event names, workspace IDs, JSON fields, persisted values, and hooks need value semantics; handles would spread representation and lifetime changes. | internal/events only after profiling, with cross-surface owners | Events registry, hooks dispatch, transcript, and workspace-isolation suites. |

### Correctness and architecture findings

| Finding | Decision | Benefit | Risk or invariant | Owner | Canonical suite |
|---|---|---|---|---|---|
| RuntimeHarness stop result is permanently memoized | APPLY root fix | Guarantees mandatory teardown can finish or be retried after a canceled first caller. | Idempotence must mean eventual process termination, not caching a premature nil; preserve joined cleanup errors. | internal/testutil/e2e | internal/testutil/e2e/runtime_harness_lifecycle_test.go. |
| Hook subprocess cancellation can wait forever after failed kill | APPLY root fix | Bounds cancellation and ensures force termination runs before an indefinite join. | Preserve process-group termination, platform parity, output capture, and original/cleanup error joining. | internal/hooks with internal/procutil | Existing Unix/Windows hook subprocess lifecycle suites. |
| Unix and Windows hook subprocess wrappers are identical | APPLY consolidation | Removes exact duplication and leaves OS behavior in procutil, its actual owner. | Cross-build behavior must remain identical; no build-tag-specific hidden dependency may be lost. | internal/hooks | Existing Unix/Windows subprocess suites and cross-build gate. |
| Async hooks inherit request cancellation | DEFER owner decision | Detaching could allow intentionally asynchronous work to complete after HTTP/UDS request return. | Existing tests require propagation; any change must preserve context values, pool cancellation, hook timeout, ordering, and shutdown. | internal/hooks plus API/UDS callers | internal/hooks/hooks_test.go and request-lifecycle integration suites. |
| Dead-entity state map has no observed eviction | DEFER until lifecycle/cardinality is specified | A bounded lifecycle can prevent memory growth across long-lived workspace/entity churn. | Naive deletion can split live per-key state and break threshold, clear-retry, or workspace isolation semantics. | internal/deadentity and daemon lifecycle | internal/deadentity/service_test.go plus daemon lifecycle integration. |
| Fourteen error-return values are explicitly discarded | APPLY | Restores cleanup and I/O failure visibility required by repository policy. | Deferred failures should be joined or reported without masking primary failures; tests must not silently swallow fixture failures. | internal/testutil/acpmock and internal/testutil/e2e | Existing owning artifact, mock-agent, automation, and harness lifecycle suites. |
| Eight production files are within 51 lines of the 500-line cap | APPLY no-growth discipline; DEFER speculative extraction | Prevents the next feature from creating a blocking god-file violation. | Extract by responsibility when touched; do not split merely to satisfy a metric without a coherent boundary. | internal/loop, internal/hooks, internal/events, internal/testutil/e2e | Owning package gates and architectural boundary checks. |

### Compozy Impact Audit

- Native tools: No direct change is recommended to compozy__ tool IDs, toolsets, descriptors, schemas, digests, risk flags, availability diagnostics, capability gates, or CLI/API fallbacks for the mechanical modernizations. RuntimeHarness and artifact changes are test infrastructure. If async-hook cancellation or HTTP origin behavior changes, their native-tool/API fallbacks and capability diagnostics must be audited before implementation.
- Extensibility and hooks: Hook pool scheduling, subprocess termination, and asynchronous context lifetime directly affect extensions and hooks. Implementations must preserve fail-open versus deny semantics, narrowing, stable ordering, configured timeouts, process registration, and shutdown. No config.toml key or default is proposed. Flight recording would need an explicit diagnostics capability/resource and lifecycle rather than an internal-only switch.
- Workspace data isolation: Dead-entity state is workspace-scoped and emitted events carry workspace_id. Any state eviction, caching, or interning must prove that global, workspace, session, and agent identities cannot merge or leak through list/read/cache/SSE/event paths. Mechanical language changes do not alter propagation. A dead-entity lifecycle change must extend the existing workspace-isolation suite.
- Official Compozy skill: Pure Go and test modernization has no impact on skills/compozy because no public command, tool ID, hook event, capability, bundle, resource, memory, network, or task behavior changes. A deliberate change to asynchronous-hook cancellation, diagnostics export, or origin-policy behavior requires a skills/compozy review and update.

### Web/Docs impact

- Mechanical Go/test modernization, error handling, process-handle ownership, and duplicate removal have no user-visible web or packages/site effect.
- CrossOriginProtection can change browser/API interoperability. Adoption requires web end-to-end verification of the local SPA proxy, unsafe-method requests, no-Origin clients, and OpenAI-compatible routes, plus security/API documentation if policy changes.
- Changing asynchronous-hook lifetime is public runtime behavior. It requires hook documentation, official-skill review, and a QA scenario reset or new content-addressed scenario followed by a real walk.
- ArtifactDir and T.Attr affect contributor/QA documentation and CI evidence discovery, not product-facing web behavior.
- Pure refactors require no QA scenario tracker change. Any observable cancellation, origin, diagnostics, or hook behavior change must be flagged and verified before completion.

## Risks / Mismatches

### RuntimeHarness teardown can report success before termination

RuntimeHarness.Stop wraps the entire operation in sync.Once at internal/testutil/e2e/runtime_harness_process.go:28. The underlying stop path signals the process, then treats every wait result other than context deadline as success at internal/testutil/e2e/runtime_harness_process.go:48. context.Canceled is therefore capable of returning nil while the child is still alive. Because stopOnce has already fired, a later teardown cannot retry.

This violates the mandatory process-teardown invariant. The fix should model lifecycle state explicitly: cache confirmed terminal completion, not merely the first call. A canceled caller must either complete a bounded force/join path or return an error while leaving a later caller able to finish cleanup. The current normal-idempotence assertion at internal/testutil/e2e/runtime_harness_lifecycle_test.go:194 is insufficient; the canonical regression should cancel the first stop context, verify the child is not falsely considered stopped, invoke Stop again, and prove termination and clean teardown.

### Hook subprocess cancellation can block before its fallback

Both cancellation branches take a kill result and then receive unconditionally from waitCh at internal/hooks/executor_subprocess_lifecycle.go:61 and internal/hooks/executor_subprocess_lifecycle.go:94. If the first group kill fails or does not terminate every descendant, the receive has no bound. The later force-termination helper is unreachable until that receive completes.

The safe pattern is: request graceful group termination; wait for a bounded grace interval or process exit; force group/process termination when still live; then join the waiter; return joined execution and cleanup errors. The owner must keep the Unix and Windows process-group contracts aligned.

### Platform wrappers duplicate shared behavior

internal/hooks/executor_subprocess_unix.go:14 and internal/hooks/executor_subprocess_windows.go:14 implement the same functions through line 35. The build tags do not currently encode differing behavior; internal/procutil already does. Keeping two identical wrappers invites asymmetric fixes and consumes review surface. Consolidation is justified, but it must be validated through both platform builds and the existing platform-specific tests.

### Request cancellation versus asynchronous work

The production path at internal/hooks/dispatch_async.go:78 and its explicit test at internal/hooks/hooks_test.go:1203 agree with one another but conflict with the wider detached-work rule. There is no safe mechanical answer. If the pool is intended to own async work, derive from context.WithoutCancel while preserving values, then layer the pool cancellation and hook timeout. If request cancellation is part of the hook contract, document that boundary and retain the test. Either choice affects extension authors and agent-visible behavior.

### Workspace-keyed state can grow indefinitely

internal/deadentity/service.go:40 owns a long-lived map, and internal/deadentity/service.go:230 inserts state for each new key. The daemon constructs a global service at internal/daemon/boot_dead_entity.go:18. Existing behavior correctly isolates workspaces and supports fail-open persistence and retryable clear behavior, but no bounded lifecycle was observed. Eviction needs an explicit maximum cardinality, idle policy, or daemon unregister event. Deleting a map entry while goroutines still hold the old state object would create two independent counters for one key.

### Ignored errors violate the repository invariant

The bounded scan found fourteen confirmed discarded error returns after excluding blank identifiers that receive non-error values:

- internal/testutil/acpmock/cmd/acpmock-driver/sandbox.go:74 discards fmt.Fprintf failure.
- internal/testutil/acpmock/driver_test.go:972 and internal/testutil/acpmock/driver_test.go:980 discard process wait failures.
- internal/testutil/e2e/artifacts.go:257 discards os.RemoveAll failure.
- internal/testutil/e2e/automation_tasks.go:376 discards response-body close failure.
- internal/testutil/e2e/mock_agents.go:106, internal/testutil/e2e/mock_agents.go:145, internal/testutil/e2e/mock_agents.go:209, and internal/testutil/e2e/mock_agents.go:242 discard response-body close failures.
- internal/testutil/e2e/runtime_harness_integration_test.go:373 discards blocker close failure.
- internal/testutil/e2e/runtime_harness_lifecycle_test.go:115, internal/testutil/e2e/runtime_harness_lifecycle_test.go:117, internal/testutil/e2e/runtime_harness_lifecycle_test.go:125, and internal/testutil/e2e/runtime_harness_lifecycle_test.go:392 discard writer, closer, reader, or encoder failures.

Production-like helper code should return or join these failures. Test cleanup should report them through the test owner without masking an already-useful primary assertion. Blindly replacing each discard with an assertion is not sufficient; the owning layer must decide whether the failure is primary, cleanup, or best-effort diagnostic output.

### Near-cap files constrain implementation placement

No production file violates the 500-line cap, but internal/loop/control_plan.go:1, internal/loop/linter_references.go:1, internal/testutil/e2e/runtime_harness_sessions.go:1, internal/loop/linter.go:1, internal/loop/goal/route.go:1, internal/hooks/hooks.go:1, internal/testutil/e2e/config_seed.go:1, and internal/events/registry.go:1 are near it. Modernization must not be used as a reason to append unrelated helpers. New lifecycle state, diagnostics, or adapters belong in responsibility-named files.

### Review claims that do not match the bounded evidence

- “No CORS” is stale for the current tree. Custom origin handling exists at internal/api/httpapi/server_setup.go:105 and internal/api/httpapi/middleware.go:53. CrossOriginProtection is therefore a compatibility and defense-in-depth evaluation, not a missing-middleware drop-in.
- math/rand/v2 has no migration target inside the nine scoped directories. Candidate v1 uses mentioned by the review belong to other owners.
- bytes.Buffer.Peek has no matching read/framing mechanism in this slice.
- unique.Handle has no profile-backed hotspot in the slice.
- sync.OnceValue would worsen, not fix, RuntimeHarness teardown semantics.

## Open Questions

1. Are asynchronous hooks contractually request-cancelled, as the current implementation and tests require, or should the hook pool own them beyond HTTP/UDS request lifetime?
2. Does testing.T.ArtifactDir on the supported Go toolchain preserve the current “delete on pass, retain on failure” evidence contract, and which stable T.Attr keys should CI index?
3. What maximum cardinality and lifetime are expected for dead-entity workspace/entity keys, and is there a daemon unregister or workspace-close event that can safely drive eviction?
4. Should net/http.CrossOriginProtection supplement or replace the custom CORS layer, and which no-Origin CLI, localhost SPA, OpenAI-compatible, proxy, and DNS-rebinding cases must remain allowed?
5. What trigger, redaction policy, size/time budget, retention policy, and agent-manageable HTTP/UDS/CLI export should own runtime flight-recorder dumps?
6. Is RuntimeHarness.Stop explicitly required to be retryable after a canceled first caller? The mandatory teardown rule and current false-success path strongly indicate yes, but the lifecycle API should state this invariant.

## Evidence

### Modernization presence and absence

- errors.AsType is already used in the slice, including internal/loop/linter.go:424 and internal/testutil/e2e/runtime_harness.go:311. Sixteen legacy errors.As call sites remain across ten files; internal/loop/goal/executor.go:215 is a representative production candidate. Exact target semantics must be checked per call.
- testing.B.Loop appears eleven times across four files. One manual benchmark loop remains at internal/loop/coordinator_watch_test.go:840.
- json omitzero appears eight times across five files, including internal/hooks/payloads_task_loop.go:45, internal/hooks/payloads_task_loop.go:61, internal/loop/dsl/gate_start.go:10, internal/loop/dsl/node_params.go:9, internal/loop/runtime.go:56, and internal/loop/resource_spec.go:59. The same bounded scan found 949 omitempty tags across 48 files, which is evidence against a blanket replacement.
- os.OpenRoot appears at internal/loop/source_store.go:164, with rooted access at internal/loop/source_store.go:173.
- strings.SplitSeq appears at internal/loop/action_channel_result_content_rule.go:59 and internal/loop/action_channel_result_content_rule.go:99. No FieldsSeq or Lines use was found. Transcript pruning still materializes lines at internal/transcript/prune.go:140.
- WaitGroup.Go appears in tests at internal/hooks/hooks_test.go:251, internal/hooks/hooks_test.go:262, and internal/testutil/acpmock/fixture_test.go:1145. Manual production launches remain at internal/hooks/pool.go:97 and internal/testutil/acpmock/cmd/acpmock-driver/sandbox.go:66.
- Range-over-integer loops are already used in production and tests, including internal/hooks/pool.go:100 and internal/deadentity/service_test.go:227.
- The scan found 58 slices calls, 21 maps calls, and 12 built-in min/max calls. The remaining manual map-key sort is internal/hooks/executor_subprocess_capture.go:26.
- No testing/synctest import was found. The bounded test scan found one literal time.Sleep and 46 timer/After-like uses across 20 files, including 33 time.After calls across 12 files.
- No custom iter.Seq or yield-function declaration was found in the nine directories. Existing standard-library sequence use is limited to SplitSeq.
- No os.Process.WithHandle or ErrNoHandle use was found in the scoped directories. The Windows process owner currently reopens by PID at internal/procutil/process_group_windows.go:31 and internal/procutil/process_group_windows.go:45.
- Three files use raw sync.Once; no sync.OnceValue or sync.OnceFunc use was found. RuntimeHarness declares stopOnce at internal/testutil/e2e/runtime_harness.go:103.
- No math/rand or math/rand/v2 import was found in the nine scoped directories.
- No cmp.Or use was found. The semantically non-equivalent helper is internal/transcript/transcript_payload.go:24.
- No testing.T.ArtifactDir, T.Attr, or T.Output use was found. The current artifact lifecycle is internal/testutil/e2e/artifacts.go:247 through internal/testutil/e2e/artifacts.go:257.
- No net/http.CrossOriginProtection use was found. Existing custom CORS setup is internal/api/httpapi/server_setup.go:105 and internal/api/httpapi/middleware.go:53.
- No runtime/trace.NewFlightRecorder use was found.
- No typed DialUnix or DialTCP call was found. The UDS callback uses DialContext at internal/testutil/e2e/runtime_harness.go:252.
- No bytes.Buffer.Peek call was found. A representative write-only accumulator is internal/hooks/executor_subprocess_capture.go:55.
- No unique package use was found. Event constants and registry data are rooted at internal/events/registry.go:386, with deterministic snapshots in internal/events/registry_queries.go:9.

### Behavioral invariants

- Dead-entity workspace isolation is asserted at internal/deadentity/service_test.go:251, fail-open persistence behavior at internal/deadentity/service_test.go:284, and clear retry behavior at internal/deadentity/service_test.go:316. The key includes workspace identity at internal/deadentity/service_test.go:486.
- Emitted dead-entity events propagate workspace identity at internal/deadentity/events.go:39.
- Event list/query output is sorted at internal/events/registry_queries.go:11, internal/events/registry_queries.go:59, and internal/events/registry_queries.go:78.
- Hook pool worker ownership is established at internal/hooks/pool.go:97, task execution at internal/hooks/pool.go:169, and worker join at internal/hooks/pool.go:189.
- Async parent-cancellation behavior is asserted at internal/hooks/hooks_test.go:1203.
- Normal RuntimeHarness Stop idempotence is currently covered at internal/testutil/e2e/runtime_harness_lifecycle_test.go:194, but canceled-first-call recovery is not.
- The supplied modernization review was read from /home/pedronauck/.codex/attachments/36398ca5-d255-4ce7-92d3-0d3429f3838f/pasted-text-1.txt:1 and checked against the bounded source tree rather than accepted as current-state fact.

### Method and limitations

The inventory and occurrence counts came from read-only file, line-count, import, and symbol scans constrained to the named slice, followed by source-level reading of the relevant implementation and canonical tests. Direct owners outside the slice were consulted only where required to establish a boundary or invalidate a repository-level claim, notably internal/procutil, internal/api/httpapi, internal/daemon, and representative consumers.

No code was changed and no test, benchmark, race detector, cross-build, or profiler was run for this analysis. Performance claims are therefore allocation/lifecycle hypotheses until an implementation task records before/after benchmark or profile evidence.
