# Go Modernization Analysis: Surfaces, Extensions, and SDK

This slice audits the executable/API surfaces, extension runtime and protocol, marketplaces and registries, MCP integration, built-in tools, support/update services, and the public Go SDK. The corpus contains 2,025 Go files across 50 package directories and 505,517 lines. Every file was mechanically indexed, line-counted, and pattern-scanned; the implementation and test regions supporting each recommendation below were then read deeply. Very large integration suites were read by candidate test, fixture, and helper boundary rather than treated as undifferentiated files.

## Scope

### Assigned surface

- Commands: `cmd/compozy`, `cmd/compozy-catalog`, `cmd/compozy-codegen`, `cmd/compozy-manifest-check`.
- API and transports: `internal/api`, `internal/cli`, `internal/sse`.
- Extension and SDK: `internal/bridges`, `internal/bridgesdk`, `internal/extension`, `internal/extensionprotocol`, `internal/extensiontest`, `sdk/go`.
- Registries and integrations: `internal/marketplace`, `internal/mcp`, `internal/registry`, `internal/skills`.
- Tooling and operations: `internal/codegen`, `internal/support`, `internal/toolmeta`, `internal/tools`, `internal/update`.

### Coverage matrix

`Prod` means non-`*_test.go`; generated and fixture sources are called out separately. `Deep` means all files are small enough or central enough to have been read as a package unit. `Candidate-deep` means the complete package was mechanically surveyed and every implementation/test region cited by this report was read, while unrelated portions of very large suites were not loaded as one monolithic read.

| Package directory | Go | Prod | Test | Depth / note |
|---|---:|---:|---:|---|
| `cmd/compozy` | 1 | 1 | 0 | Deep |
| `cmd/compozy-catalog` | 4 | 3 | 1 | Deep |
| `cmd/compozy-codegen` | 7 | 5 | 2 | Deep |
| `cmd/compozy-manifest-check` | 1 | 1 | 0 | Deep |
| `internal/api/contract` | 119 | 106 | 13 | Candidate-deep; contract families mapped |
| `internal/api/core` | 333 | 262 | 71 | Candidate-deep; shared handlers and services mapped |
| `internal/api/ginutil` | 2 | 1 | 1 | Deep |
| `internal/api/httpapi` | 61 | 35 | 26 | Deep for server, middleware, routing, parity, and security paths |
| `internal/api/spec` | 125 | 114 | 11 | Candidate-deep; operation registry and transport metadata mapped |
| `internal/api/testutil` | 29 | 27 | 2 | Candidate-deep; support fixtures mapped |
| `internal/api/udsapi` | 61 | 36 | 25 | Deep for server, routing, clients, and parity paths |
| `internal/bridges` | 78 | 58 | 20 | Candidate-deep; JSON, target, delivery, and mapping boundaries mapped |
| `internal/bridges/contract` | 27 | 19 | 8 | Deep |
| `internal/bridgesdk` | 65 | 35 | 30 | Deep for retry, scheduling, peer, HTTP, and lifecycle paths |
| `internal/cli` | 377 | 293 | 84 | Candidate-deep; transport and command dependency surfaces mapped |
| `internal/cli/docpost` | 7 | 5 | 2 | Deep |
| `internal/codegen/jsbin` | 2 | 1 | 1 | Deep |
| `internal/codegen/openapits` | 2 | 1 | 1 | Deep |
| `internal/codegen/sdkgo` | 7 | 6 | 1 | Deep |
| `internal/codegen/sdkts` | 8 | 6 | 2 | Deep |
| `internal/codegen/storeschema` | 6 | 4 | 2 | Deep |
| `internal/extension` | 240 | 173 | 67 | Candidate-deep; runtime, manager, routes, conformance, and provider suites mapped |
| `internal/extension/contract` | 25 | 22 | 3 | Deep |
| `internal/extension/scaffold_templates/loop-watch-source-go` | 1 | 1 | 0 | Deep; template source |
| `internal/extension/scaffold_templates/tool-provider-go` | 1 | 1 | 0 | Deep; template source |
| `internal/extension/surfaces` | 2 | 1 | 1 | Deep |
| `internal/extension/testdata/command-fixture-go` | 1 | 1 | 0 | Deep; fixture source |
| `internal/extension/testdata/secret-guard` | 2 | 1 | 1 | Deep; fixture source |
| `internal/extension/testdata/telegram-reference` | 7 | 4 | 3 | Candidate-deep; fixture implementation and diagnostic helpers mapped |
| `internal/extensionprotocol` | 3 | 2 | 1 | Deep |
| `internal/extensiontest` | 20 | 13 | 7 | Deep |
| `internal/marketplace` | 28 | 23 | 5 | Deep for service lifecycle, refresh, cache, and sources |
| `internal/mcp` | 38 | 32 | 6 | Candidate-deep; executor, transport, peer identity, and caches mapped |
| `internal/mcp/auth` | 20 | 16 | 4 | Deep |
| `internal/mcp/securehttp` | 5 | 4 | 1 | Deep |
| `internal/registry` | 23 | 12 | 11 | Deep |
| `internal/registry/clawhub` | 11 | 9 | 2 | Deep |
| `internal/registry/github` | 10 | 8 | 2 | Deep |
| `internal/registry/gitsrc` | 4 | 3 | 1 | Deep |
| `internal/skills` | 49 | 32 | 17 | Candidate-deep; workspace cache, install, and registry paths mapped |
| `internal/skills/marketplace` | 11 | 10 | 1 | Deep |
| `internal/sse` | 6 | 2 | 4 | Deep |
| `internal/support` | 7 | 6 | 1 | Deep |
| `internal/toolmeta` | 12 | 8 | 4 | Deep |
| `internal/tools` | 60 | 45 | 15 | Candidate-deep; registry, artifact lifecycle, approvals, and built-ins mapped |
| `internal/tools/builtin` | 42 | 41 | 1 | Candidate-deep; descriptors and executors mapped |
| `internal/update` | 18 | 11 | 7 | Deep |
| `sdk/go` | 24 | 19 | 5 | Deep |
| `sdk/go/contracts` | 31 | 31 | 0 | Deep; all generated, never hand-edit |
| `sdk/go/extensiontest` | 2 | 1 | 1 | Deep |

### Mechanical findings

- The corpus has 149 files over 500 lines. All are tests or fixture/testdata sources; the only non-`*_test.go` examples are `internal/extension/testdata/telegram-reference/main.go` (882 lines) and `internal/extension/testdata/secret-guard/main.go` (704 lines). No runtime production file outside testdata exceeds the 500-line cap.
- The 31 files in `sdk/go/contracts` are generated. Their owner is the SDK contract generator, not the generated files themselves (`sdk/go/contracts/host_api_method_gen.go:1-3`, `cmd/compozy-codegen/sdk_go_contracts.go:15-38`).
- The public SDK module declares Go 1.26.4 (`sdk/go/go.mod:3`). The root module/toolchain is outside this slice and must be verified before any repository-wide use of the reviewed APIs.
- No production dead code is proven by this slice-local analysis. `PrimeInstallDetection` has only an in-slice test reference (`internal/update/detect.go:10-12`, `internal/update/detect_test.go:107-136`), but composition roots outside the assigned paths may call it.

## Overview

The highest-value modernization work is not a blanket syntax upgrade. It is a small set of boundary-aware changes that reduce timing flakiness, close an HTTP security gap, remove duplicated transport plumbing, and make integration-test diagnostics durable.

| Candidate | Verdict | Why | Effort |
|---|---|---|---|
| `testing/synctest` | **Apply selectively** | Strong fit for in-process timers/tickers and goroutine quiescence; wrong fit for subprocess, socket, filesystem-polling, or real-server readiness | Medium |
| `iter.Seq` / range-over-function | **Apply internally; reject in public contracts** | `maps.Keys` + `slices.Sorted` simplifies deterministic generators without changing returned SDK/API collections | Low |
| `os.Process.WithHandle` | **Defer to `internal/subprocess` owner** | In-scope extensions consume a process abstraction and do not own `os.Process`; MCP peer credentials are not child processes | Medium outside slice |
| `sync.OnceFunc` / `OnceValue` | **Apply narrow `OnceFunc`; defer/reject broad `OnceValue`** | Local idempotent closures improve; stateful lifecycle structs and context-sensitive install detection should retain explicit `sync.Once` | Low |
| `math/rand/v2` | **Apply** | Retry jitter is non-cryptographic and already injectable; deterministic tests retain control | Low |
| `cmp.Or` | **Reject broad replacement** | Existing helpers trim/normalize values; `cmp.Or` only tests Go zero values and would accept whitespace or unnormalized scopes | None |
| `testing.T.ArtifactDir`, `T.Attr`, `T.Output` | **Apply** | Replaces leaked temp diagnostics and enriches provider/conformance failures without changing product behavior | Medium |
| `net/http.CrossOriginProtection` | **Apply with explicit public-route placement** | CORS currently allows requests with no `Origin`; unsafe browser requests need server-side cross-origin rejection while webhooks/OAuth remain compatible | Medium |
| `runtime/trace.NewFlightRecorder` | **Defer pending a privacy/config design** | Natural support-bundle sink exists, but traces are process-global, potentially cross-workspace, and require bounded/redacted consent semantics | High |
| `net.Dialer.DialUnix` / `DialTCP` | **Apply `DialUnix`; reject `DialTCP` here** | Two CLI UDS dial closures can share a typed helper; SSRF-safe TCP dialers intentionally resolve and dial selected numeric IPs through injectable policy | Low |
| `bytes.Buffer.Peek` | **Reject** | No non-consuming prefix-read pattern exists; current buffers need full compacted output | None |
| `unique` | **Reject/defer** | Candidate keys are unbounded workspace, path, tool, server, or auth-derived strings; interning would retain tenant-specific data globally | None |

Recommended build order:

1. Verify the root Go toolchain/API availability. Land behavior-neutral primitives first: `math/rand/v2`, the shared `DialUnix` helper, and the three safe `OnceFunc` conversions.
2. Modernize test diagnostics with `T.ArtifactDir`, `T.Attr`, and `T.Output`; extract one provider-build helper into `internal/extensiontest`.
3. Convert the two strongest timer suites to `testing/synctest`; expand only after the scheduler/SQLite spike proves deterministic quiescence.
4. Add `CrossOriginProtection` to the ordinary HTTP API branch with explicit webhook and OAuth exclusions, preserving current JSON error conventions.
5. Consolidate HTTP/UDS route registration and then narrow CLI consumer interfaces. These are the largest refactors and should not be interleaved with security behavior.
6. Consolidate JSON normalization. Decide separately whether mirrored bridge-contract converters should remain explicit or become generated.
7. Treat flight recording as a new operational feature with its own threat model and config lifecycle, not as incidental instrumentation.

## Mechanisms / Patterns

### 1. Deterministic concurrency with `testing/synctest`

Apply `synctest` where time is the only nondeterministic dependency and all goroutines are in-process:

- `internal/bridgesdk/batching_test.go:9-51` uses a real 20 ms batching timer plus a 250 ms sentinel. The production batcher schedules through `time.AfterFunc` (`internal/bridgesdk/batching.go:256-294`). A `synctest.Run` test can advance fake time and use `synctest.Wait` to prove coalescing, ordering, and shutdown without wall-clock padding.
- `internal/tools/artifact_sweeper_test.go:9-34` drives a 1 ms ticker, waits up to one second, and then sleeps 3 ms to assert shutdown. The owner is `ToolArtifactSweeper.run`, which is a pure ticker loop (`internal/tools/artifact_sweeper.go:83-94`). This is the cleanest initial conversion.
- `internal/marketplace/service_test.go:373-405` sleeps inside a stale refresh to force twelve callers to overlap, and `internal/marketplace/service_test.go:523-539` relies on a 30 ms context deadline. The service has injected `WithNow`, but not an injected refresh scheduler. This suite is a second-wave candidate after verifying that the SQLite operations do not prevent the synctest bubble from becoming durably blocked.

Do not apply `synctest` to integration boundaries merely because they contain `time.Sleep`:

- Extension/provider readiness polls real child processes, HTTP endpoints, and files (`internal/extension/reference_integration_test.go:464-482`, `internal/extension/reference_integration_test.go:798-813`, `internal/extension/gchat_provider_integration_test.go:296-323`). Replace polling with explicit readiness events where the protocol owns such an event; otherwise retain bounded real-time integration probes.
- HTTP and UDS integration suites start real listeners and storage (`internal/api/httpapi/httpapi_integration_test.go:315-328`, `internal/api/udsapi/udsapi_integration_test.go:2729-2761`). Virtual time can outrun external I/O and make the test less truthful.
- The progress dispatcher already exposes a test scheduler (`internal/bridgesdk/progress_dispatcher.go:53-80`, `internal/bridgesdk/progress_test.go:647-663`). Prefer its injected scheduling seam over wrapping the entire suite in synctest.

Invariant and owner: timer-driven services must execute at the configured cadence, coalesce the same work exactly once, and stop without post-shutdown callbacks. The canonical owners are the existing batcher, sweeper, and marketplace service suites; do not create duplicate standalone regressions.

### 2. Iterators stay behind deterministic materialization boundaries

Use iterator-producing standard helpers only inside generators:

- Replace manual map-key accumulation and `sort.Strings` in `internal/codegen/sdkgo/type_model.go:73-88` and `internal/codegen/sdkts/generate.go:134-145` with `slices.Sorted(maps.Keys(...))`. The returned slice remains the deterministic materialization boundary; no iterator enters a template or public contract.
- `internal/api/spec/tool_extension_values.go:52-59` may use the same pattern, but the output still must be a sorted `[]string` because it feeds schema generation.

Do not expose `iter.Seq` from `sdk/go`, `internal/api/spec`, or extension contracts. Operation registries encode stable order and defensive-copy behavior (`internal/api/spec/agent_catalog.go:5-13`, `internal/extension/contract/host_api_method_registry.go:476-478`, `internal/extension/contract/host_api_test.go:89-105`). Changing these to lazy iteration would alter API shape, aliasing expectations, and generator determinism for no demonstrated performance gain.

Invariant and owner: generated bytes remain stable across repeated runs. The canonical proof is `internal/codegen/sdkgo/generate_test.go:12-42` plus the repository codegen drift gate.

### 3. Typed UDS dialing, but no TCP policy erosion

Create one context-aware CLI UDS dial helper using `net.Dialer.DialUnix(ctx, "unix", nil, &net.UnixAddr{Name: path, Net: "unix"})`. Reuse it from:

- the HTTP transport in `internal/cli/client_settings_vault.go:27-47`; and
- the websocket transport in `internal/cli/client_window_manager_stream.go:34-56`.

The helper should remain internal to `internal/cli`; it is plumbing, not a public SDK abstraction. Extend the existing CLI client/window-manager tests and UDS integration helpers rather than adding a new package-level test suite.

Do not replace the generic dial path with `DialTCP` in SSRF-safe clients. `internal/bridgesdk/webhook_http.go:20-74` and `internal/mcp/securehttp/transport.go:18-33` resolve hostnames, validate each address, and dial a selected numeric IP through injectable resolver/dialer seams. That policy is more important than typed convenience, and narrowing the interface would make DNS-rebinding tests and transport injection harder.

Invariant and owner: CLI HTTP and websocket calls continue to reach only the configured daemon socket; secure HTTP clients continue to validate the resolved address actually dialed.

### 4. `OnceFunc` only for local idempotent actions

Three closures are direct `sync.OnceFunc` candidates:

- the clarify callback cleanup returned by `internal/extension/host_api_clarify.go:88-95`;
- the window-manager lease end function returned by `internal/api/core/window_manager_service.go:48-57`; and
- the progress dispatch completion action that balances `wg.Add(1)` exactly once across callback and cancellation (`internal/bridgesdk/progress_dispatcher.go:188-211`).

Keep explicit `sync.Once` in lifecycle structs. For example, SDK transport failure transitions couple a mutex, a pending-call map, an error, and a done channel (`sdk/go/transport.go:79`, `sdk/go/transport.go:424-440`). Marketplace, registry, provider, and peer close paths similarly benefit from a visible zero-value synchronization field. Replacing those fields with function values would introduce constructor-only initialization and nil-call risk.

Do not convert update detection to `OnceValue`. `Manager.Installation` resolves with the first caller's context (`internal/update/types.go:174-183`, `internal/update/detect.go:10-23`). A zero-argument cached function cannot preserve caller cancellation semantics without capturing one arbitrary context. Keep the explicit once/result pair unless the API is redesigned around a daemon-owned lifecycle context.

Invariant and owner: cleanup/finish happens exactly once, wait groups remain balanced, and the first install-detection result is cached with its existing context semantics. Existing clarification, window-manager, progress, and update detection suites own these proofs.

### 5. `math/rand/v2` for retry jitter

Switch the non-cryptographic default jitter in `internal/bridgesdk/retry.go:39-56` and `internal/bridgesdk/retry.go:83-110` to `math/rand/v2.Float64`. Preserve `RetryConfig.RandFloat`; deterministic tests already inject constant draws (`internal/bridgesdk/errors_test.go:520-592`). The seeded installer test helper at `internal/registry/installer_test.go:737-747` can use a fixed `rand/v2.PCG` seed or a deterministic byte pattern.

Do not touch cryptographic randomness. PKCE, extension auth tokens, delivery IDs, and approval tokens correctly use `crypto/rand` (`internal/mcp/auth/pkce.go:32-48`, `internal/extension/manager_runtime_launch.go:400-410`, `internal/bridges/delivery_state.go:76-86`, `internal/tools/approval_token.go:94-107`).

Invariant and owner: retry delays remain within the current backoff/jitter bounds and tests retain deterministic injection. This is a behavior-neutral import/API modernization.

### 6. `CrossOriginProtection` belongs after public exceptions and before ordinary API routes

The HTTP daemon defaults to localhost (`internal/api/httpapi/server_setup.go:49-57`) and globally applies CORS (`internal/api/httpapi/server_setup.go:105-110`). The CORS middleware rejects a disallowed non-empty `Origin`, but requests with no `Origin` proceed (`internal/api/httpapi/middleware.go:53-79`). That leaves unsafe cross-site requests dependent on browser header behavior rather than a dedicated server-side cross-origin policy.

Add `http.NewCrossOriginProtection()` as Gin middleware for the ordinary `/api` route branch. The placement must preserve the two routes registered before the loopback guard:

- signed extension webhooks under `/api/webhooks` (`internal/api/httpapi/routes.go:6-17`, `internal/api/httpapi/routes.go:463-466`); and
- the safe GET OAuth callback at `/api/mcp/oauth/callback` (`internal/api/httpapi/routes.go:6-17`).

Preferred shape: register webhooks and the OAuth callback on the public group, then create a protected subgroup for all ordinary API routes. If implementation constraints require global wrapping, configure exact bypass patterns for the signed webhook paths and confirm the OAuth callback remains accepted. Keep CORS: it still owns allowed response sharing, preflight behavior, and the OpenAI-compatible error envelope.

Tests should prove:

1. same-origin and non-browser requests to ordinary API routes still work;
2. unsafe cross-site requests are rejected even when the current CORS logic would not reject them;
3. OPTIONS behavior and error bodies remain compatible;
4. signed webhook POSTs remain governed by signature validation, not rejected by the browser-origin middleware;
5. the OAuth callback GET remains reachable; and
6. loopback/non-loopback guards retain their existing status and JSON/OpenAI error envelopes.

Canonical owners are `internal/api/httpapi/middleware_refac_test.go:53-86`, the loopback/server cases at `internal/api/httpapi/server_test.go:632-818`, and the existing webhook/OAuth integration suites.

### 7. First-class test artifacts and structured diagnostics

Adopt the new testing APIs in existing integration owners:

- Replace the manually retained reference marker directory (`internal/extension/reference_integration_test.go:408-424`) and the Telegram fixture side-effect directory (`internal/extension/testdata/telegram-reference/main_test.go:1099-1115`) with `t.ArtifactDir()` when the data is diagnostic output. This avoids leaking unmanaged temp directories after failures.
- Stream command output to `t.Output()` while retaining a buffer only when an assertion needs the bytes. The conformance builder currently captures `CombinedOutput` and embeds it in one error (`internal/extension/provider_conformance_discovery_integration_test.go:139-169`). `io.MultiWriter(&buf, t.Output())` gives live, test-scoped diagnostics without weakening failure assertions.
- Add non-secret `t.Attr` dimensions such as provider name, platform, protocol surface, and conformance case. Never attach tokens, home paths, workspace roots, authorization headers, or auth-derived fingerprints.
- On reference-daemon failure, persist bounded logs under `t.ArtifactDir()` instead of only calling `t.Logf` (`internal/extension/reference_integration_test.go:375-379`). Do not archive provider binaries by default; they are large, reproducible, and not the useful diagnostic.

The eight provider integration files repeat package-global `sync.Once`, error state, `go build`, and output formatting, for example Discord (`internal/extension/discord_provider_integration_test.go:314-332`), GChat (`internal/extension/gchat_provider_integration_test.go:270-292`), GitHub (`internal/extension/github_provider_integration_test.go:246-270`), and Linear (`internal/extension/linear_provider_integration_test.go:230-256`). Move that responsibility into one `internal/extensiontest` provider-build helper with structured attributes/output and explicit timeout. Keep provider-specific startup/auth assertions in their owning suites.

### 8. Flight recording requires an operational contract

There is no runtime trace ownership in this slice. The natural delivery mechanism is the support bundle service, which already has size caps, named sources, a builder, and asynchronous operation state (`internal/support/service.go:17-31`, `internal/support/service.go:74-104`, `internal/support/service.go:136-149`). HTTP/UDS handlers and CLI already expose an explicit-consent bundle workflow (`internal/api/core/support.go:17-77`, `internal/cli/support.go:52-101`).

Do not add `runtime/trace.NewFlightRecorder` until a design establishes:

- daemon-lifetime ownership and shutdown ordering;
- opt-in/default behavior and `config.toml` lifecycle;
- bounded memory and maximum emitted artifact size;
- the capture window and explicit user-consent step;
- redaction expectations and whether task/tool/network labels may contain user data;
- process-global scope versus workspace isolation; and
- one support-manifest entry with CLI, HTTP, and UDS parity.

A flight recorder is process-global evidence and may span multiple workspaces. Treating it as a workspace artifact without filtering would violate data-isolation expectations. The first implementation must classify it as global support data and make that scope explicit to the operator.

### 9. Features intentionally not adopted

- **`cmp.Or`:** helpers such as `internal/api/core/agent_channel_query.go:122-128`, `internal/api/core/memory_scope.go:21-37`, `internal/api/httpapi/middleware.go:203-210`, and `internal/bridges/target.go:178-184` trim or domain-normalize values. `cmp.Or(" ", fallback)` returns whitespace and is not equivalent. Retain semantic helpers.
- **`bytes.Buffer.Peek`:** the buffer sites compact or encode complete payloads, for example `internal/bridges/json.go:8-36`, `internal/bridges/contract/json.go:9-34`, `internal/api/contract/bridge_json_payload.go:108-139`, and `cmd/compozy-codegen/main.go:421-432`. The corpus scan found no current non-consuming prefix-inspection pattern.
- **`unique`:** workspace cache keys combine dynamic IDs and roots (`internal/skills/registry_workspace_cache.go:66-116`, `internal/skills/registry_workspace_cache.go:365-378`); MCP cache keys include server and auth-sensitive identity (`internal/mcp/executor_cache.go:15-28`, `internal/mcp/executor_cache.go:63-92`); tool registries accept dynamic descriptors (`internal/tools/registry.go:16-31`). Global interning would retain unbounded tenant-specific values and has no profile-backed benefit.
- **`os.Process.WithHandle` in this slice:** extension manager code depends on a `processHandle`/subprocess contract (`internal/extension/manager.go:93-101`, `internal/extension/manager_runtime_launch.go:228-243`). Linux MCP peer credentials identify an already-connected external process through `SO_PEERCRED` and `/proc` (`internal/mcp/peer_linux.go:14-47`); they do not own an `os.Process`. Audit the actual Windows child-process owner under `internal/subprocess` instead.

## Relevant Sources

### Transport and route architecture

- HTTP and UDS route files independently register almost the same resource graph: `internal/api/httpapi/routes.go:6-45` and `internal/api/udsapi/routes.go:5-42`. The files are 467 and 484 lines respectively.
- Both handler types embed the shared core handler surface (`internal/api/httpapi/handlers.go:95-102`, `internal/api/udsapi/server.go:130-136`), so route consolidation need not move business behavior into a transport package.
- Operation specs already declare transport availability, for example `internal/api/spec/agent_catalog.go:5-13`. Existing route parity coverage is partial and task-focused (`internal/api/httpapi/transport_parity_integration_test.go:490-506`).
- HTTP-specific exceptions and policy are real: public webhooks/OAuth callback, CORS, loopback guards, static fallback, and privileged mutation middleware (`internal/api/httpapi/routes.go:6-17`, `internal/api/httpapi/server_setup.go:105-110`). UDS additionally exposes agent-kernel, task-run, and MCP routes (`internal/api/udsapi/routes.go:5-42`).

### SDK and generated ownership

- Contract source authority is in internal registries, including `HostAPIMethodSpecs` (`internal/extension/contract/host_api_method_registry.go:476-478`).
- Generation is dispatched by `cmd/compozy-codegen/main.go:69-78` and `cmd/compozy-codegen/sdk_go_contracts.go:15-38`.
- The Go SDK renderer is `internal/codegen/sdkgo/generate.go:18-28` with type-file ownership in `internal/codegen/sdkgo/type_files.go:40-49`.
- Generated files explicitly forbid hand edits (`sdk/go/contracts/host_api_method_gen.go:1-3`); generator determinism is tested at `internal/codegen/sdkgo/generate_test.go:12-42`.
- Public SDK transport lifecycle is synchronization-sensitive (`sdk/go/transport.go:79`, `sdk/go/transport.go:424-440`) and should not be used as a cosmetic `OnceFunc` conversion target.

### Extension and bridge boundaries

- The extension manager launches through a subprocess abstraction and registry rather than owning an `os.Process` (`internal/extension/manager_runtime_launch.go:228-243`).
- Provider conformance builds and runs real binaries over the extension protocol (`internal/extension/provider_conformance_discovery_integration_test.go:139-169`). Provider-specific integration suites duplicate that build lifecycle.
- Bridge JSON normalization is duplicated almost byte-for-byte in daemon and contract packages (`internal/bridges/json.go:8-36`, `internal/bridges/contract/json.go:9-34`), while the API contract has a third validator-aware variant (`internal/api/contract/bridge_json_payload.go:108-139`).
- Bridge/domain-to-contract conversion is long and mechanical (`internal/bridges/delivery_contract_mapping.go:8-150`, `internal/bridges/target_contract_mapping.go:8-27`) but it currently provides an explicit ownership boundary and must not be erased through unsafe type aliases.

### Caches, isolation, and operational evidence

- Workspace skill cache keys are explicitly derived from workspace identity/root (`internal/skills/registry_workspace_cache.go:66-116`, `internal/skills/registry_workspace_cache.go:365-378`).
- MCP executor caches include server and authentication identity (`internal/mcp/executor_cache.go:15-28`, `internal/mcp/executor_cache.go:63-92`).
- Workspace network routes enforce authorization middleware in both the normal and coordination groups (`internal/api/httpapi/routes.go:312-345`). Shared route registration must preserve that exact middleware placement.
- Support bundles already impose caps and source ownership (`internal/support/service.go:17-31`, `internal/support/service.go:74-104`) and therefore are the correct future sink for bounded trace artifacts.

## Transferable Patterns

### A. Consolidate HTTP/UDS routing around a shared core registrar

**Finding:** `internal/api/httpapi/routes.go` and `internal/api/udsapi/routes.go` duplicate route method/path/handler wiring, creating drift risk. The visible drift already includes UDS-only agent-kernel/task-run/MCP surfaces and HTTP-only webhook/OAuth/OpenAI/static concerns.

**Proposal:** create a shared route-descriptor or registrar layer owned by `internal/api/core` (or a narrowly named sibling package under `internal/api`) that registers ordinary operations against the embedded `BaseHandlers`. Each transport supplies a small policy adapter for:

- transport availability from `api/spec`;
- privileged mutation wrapping;
- resource/operator authorization;
- workspace network authorization;
- HTTP-only public routes, CORS/origin/loopback policy, and static fallback; and
- UDS-only privileged surfaces.

Do not generate handler implementations. A descriptor should point to existing handler methods and preserve registration order/middleware. Add a full route-set parity test derived from `OperationSpec.Transports`, replacing the present task-only parity sample.

**Dependencies:** land after `CrossOriginProtection` so the desired HTTP policy boundaries are explicit. Do not combine with contract generation changes.

**Invariant / canonical suite:** every documented operation exists on every declared transport with the same method and path; HTTP-only and UDS-only operations remain intentional; workspace middleware and error envelopes are unchanged. Extend the existing HTTP transport parity integration suite and the UDS integration suite rather than creating a static file-content test.

**Effort:** high. The change is mechanically broad but conceptually contained.

### B. Replace repeated server dependency bags with one surface-dependency assembly

**Finding:** the HTTP server (`internal/api/httpapi/server.go:35-113`), UDS server (`internal/api/udsapi/server.go:52-128`), and HTTP handler configuration (`internal/api/httpapi/handlers.go:24-102`) repeat large dependency lists. This makes constructor changes fan out across both transports.

**Proposal:** introduce one `core.SurfaceDependencies` (name illustrative) containing only shared services required by `BaseHandlers`. Construct `BaseHandlers` once through a validated core constructor. Keep listeners, hosts, CORS/security policy, peer credentials, streaming connection state, and HTTP/UDS-specific handlers in transport packages.

**Dependencies:** follow route consolidation, because the shared registrar clarifies which dependencies truly belong to common handlers. Avoid a god config: split optional domain clusters only when the owning handler family can validate them independently.

**Invariant / canonical suite:** server construction still fails on missing required dependencies at the owning layer, and each route observes the same service instance. Existing HTTP/UDS constructor and integration suites own this proof.

**Effort:** high.

### C. Narrow the CLI's consumer interfaces

**Finding:** `DaemonClient` begins at `internal/cli/client.go:26` and spans most of a 470-line file. Every command depends on the aggregate transport interface even when it invokes only one domain.

**Proposal:** keep `unixSocketClient` as the concrete aggregate, but define command-owned narrow interfaces such as `workspaceClient`, `extensionClient`, `supportClient`, and `windowManagerClient` next to their command dependency types. Compose aggregate interfaces only at the CLI wiring root. This lowers fixture cost and exposes which commands require streams versus request/response calls.

**Dependencies:** the shared `DialUnix` helper can land independently first. Interface narrowing should follow route consolidation to avoid simultaneous transport churn.

**Invariant / canonical suite:** command behavior and JSON output are unchanged; compile-time interface assertions ensure the concrete client implements each facet. Existing command suites own their respective interfaces.

**Effort:** medium to high.

### D. Consolidate JSON normalization at the lowest shared boundary

**Finding:** `internal/bridges/json.go` and `internal/bridges/contract/json.go` duplicate raw JSON compaction and object validation. The API contract variant adds a validator and distinct error context.

**Proposal:** extract one low-level helper whose contract is only: clone/compact valid JSON, preserve empty/null semantics, and optionally require an object. Keep package-specific wrappers for exact error prefixes and validator behavior. The owner must be below both daemon and contract packages without importing API layers; if no existing dependency direction supports that, place it in a narrowly named internal JSON package rather than forcing a cycle.

**Invariant / canonical suite:** raw input is never aliased, invalid JSON errors retain their public prefix, empty/null normalization remains unchanged, and non-object values are rejected where required. Consolidate assertions into the existing bridge, bridge-contract, and API-contract JSON suites; each layer should prove only its distinct error/validation contract.

**Effort:** medium.

### E. Decide whether bridge-contract mapping is explicit or generated

**Finding:** delivery and target mappers manually copy mirrored DTOs. This is repetitive, but the copies preserve slice/raw-message ownership and isolate daemon models from wire contracts.

**Proposal:** do not type-alias the layers. Choose one of two deliberate endpoints:

1. retain explicit converters, split by aggregate, and add round-trip/property tests for cloning and enum fidelity; or
2. generate pure converters from the canonical bridge contract schema, keeping handwritten validation and semantic defaults outside generated code.

Generation is justified only if more mirrored families are planned; otherwise explicit, well-tested converters are clearer than another generator.

**Dependencies:** decide after JSON normalization, which removes unrelated noise from the mapper surface.

**Invariant / canonical suite:** wire bytes and enum values are stable; mutable slices/raw messages do not alias across layers; workspace/session/agent identifiers are copied without substitution. Existing bridge contract-mapping tests are the canonical owner.

**Effort:** medium for explicit cleanup, high for generation.

### F. Preserve generated SDK ownership

Any change to extension host methods or public SDK contract shapes must follow this path:

1. edit the canonical internal contract/registry;
2. update the SDK generator/model when required;
3. regenerate all Go SDK contracts through `cmd/compozy-codegen`;
4. update generator tests and public SDK behavior tests; and
5. run the codegen drift gate.

Never patch `sdk/go/contracts/*_gen.go`. Iterator modernization belongs in the generator implementation, not generated artifacts. If public contract behavior changes, co-ship CLI/HTTP/UDS/native-tool descriptors and the official Compozy skill where applicable.

### G. Compozy Impact Audit

- **Native tools:** no `compozy__*` IDs, toolsets, descriptors, schemas, digests, risk flags, or capability gates need to change for the recommended refactors. Checked `internal/toolmeta`, `internal/tools`, and `internal/tools/builtin`. A future generated SDK contract change must separately audit native-tool schemas and fallbacks.
- **Extensibility and hooks:** the recommendations preserve extension protocol, hooks, capabilities, bundles, registries, bridge SDKs, MCP sidecars, and config lifecycle. `CrossOriginProtection` must explicitly exempt signed extension webhooks. Flight recording is deferred because it would require a new support source, config lifecycle, consent contract, and documentation. Generated SDK changes must start at `internal/extension/contract`, not the generated package.
- **Workspace data isolation:** the route consolidation must preserve `AuthorizeNetworkWorkspaceAccess` and all workspace ID propagation. `CrossOriginProtection` reduces browser-origin attack exposure without changing datum scope. `unique` is rejected because interning dynamic workspace/auth-derived keys would create process-global retention. A flight trace is process-global and must never be represented as workspace-scoped without proven filtering.
- **Official Compozy skill:** behavior-neutral refactors and test modernization have no skill impact after checking CLI/HTTP/UDS semantics. Any documented origin-policy behavior, new trace config/support artifact, public SDK method, tool ID, hook event, or capability change requires a corresponding `skills/compozy/` update.

## Risks / Mismatches

1. **Toolchain mismatch.** The SDK submodule declares Go 1.26.4, but root `go.mod`, CI images, release builders, and extension scaffold modules are outside or span beyond this slice. Compiling against APIs unavailable to any lane would break the monorepo even if `sdk/go` builds.
2. **Origin protection can break machine-to-machine ingress.** Applying `CrossOriginProtection` globally without exact route placement can reject signed extension webhooks or alter their error/status contract. CORS and cross-origin rejection solve different problems and both remain necessary.
3. **HTTP error-envelope drift.** Standard `CrossOriginProtection` responses may not match `contract.ErrorPayload` or the OpenAI-compatible error shape. The Gin adapter must translate rejection through existing responders where public contracts require it.
4. **Route consolidation can erase intentional asymmetry.** HTTP public callbacks/static/OpenAI behavior and UDS agent-kernel/task-run/MCP behavior are not duplication bugs. The shared layer must be driven by explicit transport metadata, not an assumption that both route sets are identical.
5. **Workspace middleware ordering.** Moving route registration can accidentally place workspace network routes outside `AuthorizeNetworkWorkspaceAccess` or privilege guards. Test behavior, not source-text presence.
6. **`synctest` at an external-I/O boundary.** Virtual time can advance before SQLite, a child process, a socket, or the filesystem reaches the expected state. Limit initial adoption to the batcher and artifact sweeper; spike marketplace refresh separately.
7. **Artifact retention and secrets.** `T.Output`, `T.Attr`, and `ArtifactDir` make evidence easier to retain. Logs and attributes must be bounded and scrubbed; never emit tokens, auth headers, bound-secret values, provider homes, workspace roots, or raw payloads by default.
8. **`OnceFunc` zero-value regression.** Function-valued lifecycle fields require initialization. Restrict conversion to returned/local closures; keep struct `sync.Once` fields unless constructor-only semantics are already mandatory and tested.
9. **`OnceValue` context capture.** Caching a closure around one caller context can make all future callers inherit a canceled/deadline-bound context. The update manager's current explicit result cache avoids hiding this choice.
10. **Iterator ordering and aliasing.** Map iteration is nondeterministic. `maps.Keys` is safe only when immediately sorted before generation. Lazy public iterators would change defensive-copy and stable-order contracts.
11. **Interning and cross-tenant retention.** `unique` handles are process-global lifetime optimizations. Applying them to workspace, path, server, or auth-derived values trades a speculative allocation win for unbounded retention and harder isolation reasoning.
12. **Flight traces are globally scoped and sensitive.** A trace can reveal scheduling, goroutine labels, network timing, and task/tool activity from multiple workspaces. It needs threat modeling, explicit consent, bounded capture, and support-bundle redaction before implementation.
13. **Generated ownership drift.** Direct edits under `sdk/go/contracts` will be overwritten and can desynchronize the generator, registry, SDK, docs, and extension fixtures.
14. **Mapper over-consolidation.** Type aliases or reflection-based automatic mapping can remove intentional copying, make wire and domain evolution inseparable, and introduce slice/raw-message aliasing.
15. **False dead-code conclusions.** Slice-local reference counts cannot prove reachability because executable composition, web/docs consumers, generated code, and packages outside the scope may call exported functions.

## Open Questions

1. Does the root module, every extension scaffold/module, CI, and release toolchain all target a Go version that includes `testing/synctest`, `T.ArtifactDir`/`T.Attr`/`T.Output`, `CrossOriginProtection`, `DialUnix`/`DialTCP`, `Buffer.Peek`, `WithHandle`, and `NewFlightRecorder`? `sdk/go/go.mod` alone is insufficient evidence.
2. At the actual Windows child-process owner under `internal/subprocess`, are raw process handles currently reopened from PIDs or otherwise exposed to PID-reuse/handle-lifetime races that `os.Process.WithHandle` would solve?
3. Should the canonical HTTP/UDS route source be `api/spec` metadata plus a shared registrar, or a separate hand-authored registrar checked against `api/spec`? The former reduces drift but increases generator/descriptor coupling.
4. What precise response body/status must an unsafe cross-origin HTTP request receive, especially for OpenAI-compatible routes? The standard protection response may need adaptation to existing public error contracts.
5. Can the marketplace refresh suite reach a stable `synctest` bubble while its SQLite operations are active, or should only its scheduler/deadline component be extracted behind an injected clock/timer?
6. Is a runtime trace allowed to include activity from all workspaces in one explicitly global support bundle, or must capture be filtered/isolated per workspace? This decision blocks flight-recorder implementation.
7. Is `PrimeInstallDetection` invoked by composition outside the assigned slice? A repository-wide reachability check is required before retaining or deleting it.

## Evidence

### Corpus and generation

- `sdk/go/go.mod:3` — SDK Go version.
- `sdk/go/contracts/host_api_method_gen.go:1-3` — generated-file ownership marker.
- `cmd/compozy-codegen/main.go:69-78` — SDK contract generation command dispatch.
- `cmd/compozy-codegen/sdk_go_contracts.go:15-38` — generated output orchestration.
- `internal/codegen/sdkgo/generate.go:18-28` — Go SDK generator entry.
- `internal/codegen/sdkgo/type_files.go:40-49` — type-file generation ownership.
- `internal/codegen/sdkgo/generate_test.go:12-42` — deterministic generation proof.
- `internal/extension/contract/host_api_method_registry.go:476-478` — canonical host API method registry.
- `internal/extension/contract/host_api_test.go:89-105` — defensive-copy contract.

### HTTP, UDS, CLI, and isolation

- `internal/api/httpapi/server_setup.go:49-57` — localhost/port defaults.
- `internal/api/httpapi/server_setup.go:105-110` — global middleware order.
- `internal/api/httpapi/middleware.go:53-79` — current CORS/origin behavior.
- `internal/api/httpapi/middleware.go:227-290` — loopback classification and guards.
- `internal/api/httpapi/routes.go:6-45` — HTTP public exceptions and ordinary route registration.
- `internal/api/httpapi/routes.go:312-345` — workspace network authorization middleware.
- `internal/api/httpapi/routes.go:463-466` — signed webhook paths.
- `internal/api/httpapi/server_start.go:42-56` — TCP listener/server ownership.
- `internal/api/udsapi/routes.go:5-42` — UDS route registration and transport-only surfaces.
- `internal/api/httpapi/handlers.go:95-102` — HTTP embedding of shared core handlers.
- `internal/api/udsapi/server.go:130-136` — UDS embedding of shared core handlers.
- `internal/api/spec/agent_catalog.go:5-13` — operation transport metadata example.
- `internal/api/httpapi/transport_parity_integration_test.go:490-506` — existing partial parity assertion.
- `internal/cli/client.go:25-470` — aggregate daemon client interface.
- `internal/cli/client_settings_vault.go:27-47` — HTTP-over-UDS dial closure.
- `internal/cli/client_window_manager_stream.go:34-56` — websocket-over-UDS duplicate dial closure.

### Concurrency, timers, and randomness

- `internal/bridgesdk/batching.go:256-294` — timer-driven batch scheduling.
- `internal/bridgesdk/batching_test.go:9-51` — wall-clock batch test candidate.
- `internal/tools/artifact_sweeper.go:83-94` — ticker loop.
- `internal/tools/artifact_sweeper_test.go:9-34` — ticker/sleep test candidate.
- `internal/marketplace/service_test.go:373-405` — sleep-coordinated concurrent refresh.
- `internal/marketplace/service_test.go:523-539` — real-time deadline test.
- `internal/bridgesdk/progress_dispatcher.go:188-211` — exactly-once completion action.
- `internal/bridgesdk/progress_test.go:647-663` — injected progress scheduler.
- `internal/extension/host_api_clarify.go:88-95` — local once-only cleanup closure.
- `internal/api/core/window_manager_service.go:48-57` — local once-only lease end closure.
- `internal/update/types.go:174-183` — explicit install cache fields.
- `internal/update/detect.go:10-23` — first-caller context semantics.
- `internal/update/detect_test.go:107-136` — cached install detection test.
- `sdk/go/transport.go:79` and `sdk/go/transport.go:424-440` — stateful SDK transport failure transition.
- `internal/bridgesdk/retry.go:39-56` and `internal/bridgesdk/retry.go:83-110` — injectable retry jitter defaults.
- `internal/bridgesdk/errors_test.go:520-592` — deterministic jitter injection.
- `internal/registry/installer_test.go:737-747` — fixed-seed non-cryptographic test helper.
- `internal/mcp/auth/pkce.go:32-48` — cryptographic randomness boundary.

### Test diagnostics and extension processes

- `internal/extension/reference_integration_test.go:375-424` — failure logs and retained marker directory.
- `internal/extension/testdata/telegram-reference/main_test.go:1099-1115` — retained fixture side-effect directory.
- `internal/extension/provider_conformance_discovery_integration_test.go:139-169` — real provider build/run lifecycle.
- `internal/extension/discord_provider_integration_test.go:314-332` — duplicated provider build helper.
- `internal/extension/gchat_provider_integration_test.go:270-292` — duplicated provider build helper.
- `internal/extension/github_provider_integration_test.go:246-270` — duplicated provider build helper.
- `internal/extension/linear_provider_integration_test.go:230-256` — duplicated provider build helper.
- `internal/extension/reference_integration_test.go:464-482` and `internal/extension/reference_integration_test.go:798-813` — external-process/filesystem polling, unsuitable for synctest.
- `internal/extension/manager.go:93-101` — in-scope process abstraction.
- `internal/extension/manager_runtime_launch.go:228-243` — subprocess launch ownership boundary.
- `internal/mcp/peer_linux.go:14-47` — external peer identity via `SO_PEERCRED` and `/proc`.

### Refactor and non-adoption evidence

- `internal/codegen/sdkgo/type_model.go:73-88` — manual sorted map-key materialization.
- `internal/codegen/sdkts/generate.go:134-145` — duplicate sorted map-key materialization.
- `internal/api/spec/tool_extension_values.go:52-59` — sorted schema-value materialization.
- `internal/api/core/agent_channel_query.go:122-128` — trim-aware fallback helper.
- `internal/api/core/memory_scope.go:21-37` — domain-normalizing fallback helpers.
- `internal/api/httpapi/middleware.go:203-210` — trim-aware fallback helper.
- `internal/bridges/target.go:178-184` — trim-aware fallback helper.
- `internal/bridges/json.go:8-36` and `internal/bridges/contract/json.go:9-34` — duplicated JSON normalization.
- `internal/api/contract/bridge_json_payload.go:108-139` — validator-aware JSON normalization variant.
- `internal/bridges/delivery_contract_mapping.go:8-150` — mirrored delivery DTO conversion.
- `internal/bridges/target_contract_mapping.go:8-27` — mirrored target DTO conversion.
- `internal/bridgesdk/webhook_http.go:20-74` — resolver/selected-IP dial policy.
- `internal/mcp/securehttp/transport.go:18-33` — secure HTTP dialer contract.
- `internal/skills/registry_workspace_cache.go:66-116` and `internal/skills/registry_workspace_cache.go:365-378` — dynamic workspace cache keys.
- `internal/mcp/executor_cache.go:15-28` and `internal/mcp/executor_cache.go:63-92` — dynamic server/auth cache keys.
- `internal/tools/registry.go:16-31` — dynamic tool descriptor registry.
- `internal/support/service.go:17-31`, `internal/support/service.go:74-104`, and `internal/support/service.go:136-149` — bounded support source/builder/operation model.
- `internal/api/core/support.go:17-77` — transport-shared support operation.
- `internal/cli/support.go:52-101` — explicit-consent CLI support bundle workflow.
