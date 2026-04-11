---
status: completed
title: Go SDK sdk/extension package
type: backend
complexity: high
dependencies:
  - task_08
---

# Task 14: Go SDK sdk/extension package

## Overview
Ship the public Go SDK that extension authors import to build Compozy extensions in Go. The SDK wraps the JSON-RPC transport, exposes a handler-registration API for hook events and service methods, provides a typed Host API client, and includes a mock transport for in-process unit testing. It is the minimum surface an author needs to write, test, and run a subprocess extension without hand-rolling JSON-RPC.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
- NOTE: No `_prd.md` exists. Requirements derive from `_techspec.md` Core Interfaces for the SDK and `_protocol.md` for the wire contract.
</critical>

<requirements>
- MUST expose a public `sdk/extension` package at `github.com/compozy/compozy/sdk/extension` (matching the actual module path from `go.mod`).
- MUST provide an `Extension` type that authors use to declare name, version, capabilities, and handlers.
- MUST implement the full initialize handshake client side per `_protocol.md` section 4.
- MUST implement handler registration for `execute_hook` and `on_event` methods with strongly typed payload and patch structs mirrored from the Compozy runtime types.
- MUST expose a `HostAPI` client with methods mirroring every Host API method defined in `_protocol.md` section 5.2 (`host.tasks.*`, `host.runs.start`, `host.memory.*`, `host.artifacts.*`, `host.prompts.render`, `host.events.*`).
- MUST implement `health_check` and `shutdown` handlers with default implementations authors can override.
- MUST provide a `MockTransport` for unit tests so extension authors can test their handlers without spawning a real subprocess.
- MUST provide an in-process test harness that simulates the Compozy host: accepts the extension's initialize request, answers with granted capabilities, dispatches mock hook events, and records Host API calls made by the extension.
- MUST NOT import anything from `internal/core/extension` or any other `internal/` package — the SDK is public API.
- MUST share type definitions with the runtime through a public `pkg/` types package where practical, or via duplicate types with compile-time verification tests that they stay aligned.
</requirements>

## Subtasks
- [x] 14.1 Create `sdk/extension/` package with `Extension` struct, builder API, and `Start()` entry point.
- [x] 14.2 Implement the initialize handshake client: send, receive, validate, track accepted capabilities.
- [x] 14.3 Implement handler registration for `execute_hook` with per-event typed registration (`OnPromptPostBuild`, `OnRunPostShutdown`, etc.) plus a generic `Handle(event, handler)` fallback.
- [x] 14.4 Implement handler registration for `on_event` with a filter API.
- [x] 14.5 Implement the `HostAPI` client covering all eleven Host API methods with strongly typed request/response structs.
- [x] 14.6 Implement `MockTransport` and an in-process `TestHarness` for author unit tests.
- [x] 14.7 Write tests for the SDK itself: handshake, handler dispatch, Host API client, mock transport, and test harness.

## Implementation Details
See TechSpec "Implementation Design → Core Interfaces" for the target SDK shape and `_protocol.md` sections 4, 5, 6, 7, 8, 9 for the wire contract the SDK must implement.

Place files under:
- `sdk/extension/extension.go` — `Extension` type and public API
- `sdk/extension/handlers.go` — typed hook handler registration
- `sdk/extension/host_api.go` — `HostAPI` client
- `sdk/extension/transport.go` — JSON-RPC transport over stdin/stdout
- `sdk/extension/testing/mock_transport.go` — in-memory transport for tests
- `sdk/extension/testing/harness.go` — in-process test harness
- `sdk/extension/extension_test.go`
- `sdk/extension/host_api_test.go`

Important boundaries:
- The SDK lives under `sdk/extension/` (not `internal/`). It is a public module-level package. Extension authors import it directly.
- The SDK must not import anything from `internal/core/extension`. If type duplication is necessary, add a compile-time verification test that catches drift (e.g., a test that uses reflection to compare field sets).
- The runtime types referenced by hook payloads (BatchParams, SessionRequest, Job, RunSummary, etc.) must be exposed via a public package such as `pkg/compozy/extensibility/types/` or copied with verification.

Design principle: an author should be able to write a working "hello world" extension in under 30 lines of Go. The SDK must do all the heavy lifting.

### Relevant Files
- `_protocol.md` sections 4, 5, 6, 7, 8, 9 — Wire contract the SDK implements.
- `internal/core/extension/manager.go` — From task 08. The runtime counterpart whose contract the SDK mirrors.
- `internal/core/extension/host_api.go` — From task 05. Router the SDK talks to.
- `pkg/compozy/events/event.go` — Event envelope shared with `on_event` subscribers.
- `go.mod` — Module path is `github.com/compozy/compozy`. SDK import path is `github.com/compozy/compozy/sdk/extension`.

### Dependent Files
- Task 15 — TypeScript SDK mirrors this shape in TypeScript.
- Future extension author repositories will import `github.com/compozy/compozy/sdk/extension`.

### Related ADRs
- [ADR-001: Subprocess-Only Extension Model](adrs/adr-001.md) — SDK targets subprocess authors.
- [ADR-003: JSON-RPC 2.0 over stdio with Shared internal/core/subprocess Package](adrs/adr-003.md) — Transport contract.
- [ADR-006: Host API Surface for Extension Callbacks](adrs/adr-006.md) — Host API surface mirrored here.

## Deliverables
- Public `sdk/extension/` package with Extension type, handler registration, Host API client, and transport.
- `sdk/extension/testing/` subpackage with `MockTransport` and `TestHarness`.
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration test: a minimal mock extension built against the SDK completes the full lifecycle against the in-process harness **(REQUIRED)**

## Tests
- Unit tests:
  - [x] `Extension.Start()` sends an initialize request and processes the response.
  - [x] `Extension.Start()` rejects a response with an unsupported protocol version.
  - [x] `Extension.Start()` rejects a response whose accepted capabilities exceed the granted set.
  - [x] `OnPromptPostBuild` handler receives the `PromptPostBuildPayload` and its patch is sent back in the response.
  - [x] `OnEvent` handler filter receives only the event kinds declared in the filter list.
  - [x] `HostAPI.Tasks.Create` round-trips through the mock transport with the correct JSON-RPC method and params.
  - [x] `HostAPI.Runs.Start` returns the new run id and parent chain from a mock host response.
  - [x] `HostAPI.Memory.Read` returns `exists: false` and `content: ""` when the mock host says the document is absent.
  - [x] `HostAPI.Artifacts.Write` returns `path_out_of_scope` when the mock host rejects the path.
  - [x] `MockTransport` correlates requests and responses by `id` and tolerates out-of-order delivery.
  - [x] `TestHarness` simulates a hook dispatch and the extension's handler runs to completion.
- Integration tests:
  - [x] A minimal extension written against the SDK completes initialize → execute_hook → host.tasks.list → shutdown against the in-process test harness.
  - [x] The same extension compiled to a binary and spawned by the runtime extension manager (from task 08) completes the same lifecycle end-to-end over real stdio.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `make verify` exits zero with zero lint issues
- The SDK has zero `internal/` imports
- A hello-world extension fits in fewer than 50 lines of Go including imports
