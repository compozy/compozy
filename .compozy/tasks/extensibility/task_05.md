---
status: completed
title: Hook dispatcher and Host API router
type: backend
complexity: medium
dependencies:
  - task_02
  - task_04
---

# Task 05: Hook dispatcher and Host API router

## Overview
Implement the two RPC glue layers that sit between the Compozy runtime and extension subprocesses. The hook dispatcher routes host→extension calls through a priority-ordered chain of subscribed extensions for each mutable hook event and dispatches observe-only events concurrently. The Host API router routes extension→host calls to the right service handler, applying capability enforcement and audit logging on entry. This task implements both layers as thin, protocol-agnostic code that will be wired into real extensions in task 08.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
- NOTE: No `_prd.md` exists. Requirements derive from `_techspec.md`, `_protocol.md`, and ADR-004.
</critical>

<requirements>
- MUST implement `HookDispatcher` with `DispatchMutable(ctx, hook, input)` and `DispatchObserver(ctx, hook, payload)` methods.
- MUST build a priority-sorted chain per hook event at construction time using the `HookDeclaration.Priority` field, breaking ties alphabetically by extension name per ADR-004.
- MUST feed the output of each mutable hook call as the input to the next extension in the chain (chain-of-responsibility).
- MUST log each dispatch through the `AuditLogger` from task 04, including latency and result.
- MUST call `CapabilityChecker` from task 04 before invoking any extension.
- MUST honor `required` versus optional hook semantics: a required hook failure aborts the chain and returns an error; an optional hook failure logs a warning and continues with the pre-failure value.
- MUST apply each hook's `Timeout` as a per-extension context deadline.
- MUST implement `HostAPIRouter` with a single `Handle(ctx, extension, method, params)` entry point that routes to registered service handlers, checks capabilities, and records audit entries.
- MUST expose a `RegisterService(namespace, handler)` method so task 06 can plug in typed service handlers.
- MUST NOT implement any actual Host API method bodies — those belong to task 06.
- MUST NOT spawn any subprocess or open any real transport — those belong to tasks 07 and 08.
</requirements>

## Subtasks
- [x] 05.1 Implement `HookDispatcher` construction from a registry of extensions plus their declared hooks, producing a per-event priority-sorted chain.
- [x] 05.2 Implement `DispatchMutable` with chain-of-responsibility semantics, per-extension timeouts, capability checks, and audit entries.
- [x] 05.3 Implement `DispatchObserver` with fan-out concurrent dispatch and best-effort delivery (no chain, no mutation).
- [x] 05.4 Implement `HostAPIRouter` with `RegisterService` and `Handle` methods; `Handle` routes on the `host.<namespace>.<verb>` prefix.
- [x] 05.5 Implement standard error responses (`-32601 method_not_found`, `-32001 capability_denied`, `-32003 not_initialized`, `-32004 shutdown_in_progress`) per `_protocol.md` section 10.
- [x] 05.6 Write tests covering chain ordering, tiebreak, required/optional failure modes, observer fan-out, and router error codes.

## Implementation Details
See TechSpec "Implementation Design → Core Interfaces" for the dispatcher type signatures, "System Architecture → Data Flow → Hook dispatch (mutable)" for the runtime flow, `_protocol.md` section 6 for the hook dispatch wire contract, `_protocol.md` section 10 for the error model, and ADR-004 for the priority ordering rationale.

Place files under:
- `internal/core/extension/dispatcher.go` — `HookDispatcher` type and methods
- `internal/core/extension/chain.go` — priority chain construction helper
- `internal/core/extension/host_api.go` — `HostAPIRouter` type and registration
- `internal/core/extension/host_api_errors.go` — standard error response builders
- `internal/core/extension/dispatcher_test.go`
- `internal/core/extension/host_api_test.go`

The dispatcher and router must both depend on an `ExtensionCaller` interface that represents "the ability to send a JSON-RPC request to an extension subprocess" without binding to a real transport. Task 08 provides the real implementation; tests in this task use an in-memory fake.

### Relevant Files
- `internal/core/extension/capability.go` — From task 04. Called on entry by both dispatcher and router.
- `internal/core/extension/audit.go` — From task 04. Records every dispatch/call.
- `internal/core/extension/manifest.go` — From task 02. Provides `HookDeclaration` and capability taxonomy.
- `_protocol.md` section 6 — Hook dispatch wire contract.
- `_protocol.md` section 10 — Error model.
- `adrs/adr-004.md` — Priority pipeline rationale.

### Dependent Files
- Task 06 (Host API services) registers service handlers with the router.
- Task 08 (extension manager lifecycle) injects the real `ExtensionCaller` transport and constructs the dispatcher per run.
- Tasks 10 and 11 (hook insertion) call `DispatchMutable`/`DispatchObserver` from pipeline phases.

### Related ADRs
- [ADR-004: Priority-Ordered Mutation Pipeline for Hooks](adrs/adr-004.md) — Ordering and chain semantics.
- [ADR-005: Capability-Based Security Without Trust Tiers](adrs/adr-005.md) — Capability checks on every dispatch entry.
- [ADR-006: Host API Surface for Extension Callbacks](adrs/adr-006.md) — Router shape.

## Deliverables
- `HookDispatcher` with deterministic priority chain execution, capability enforcement, and audit entry emission.
- `HostAPIRouter` with pluggable service handlers and standard JSON-RPC error responses.
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration tests exercising a multi-extension chain with mixed required/optional hooks **(REQUIRED)**

## Tests
- Unit tests:
  - [x] Chain construction orders extensions ascending by priority with alphabetical tiebreak on equal priority.
  - [x] `DispatchMutable` passes the mutated payload from one extension to the next in chain order.
  - [x] `DispatchMutable` returns the final mutated payload after the last extension in the chain.
  - [x] A required extension returning an error aborts the chain and propagates the error with the extension name attached.
  - [x] An optional extension returning an error is logged and skipped; the chain continues with the value it had before the failing extension.
  - [x] A hook timeout produces a deadline-exceeded error wrapped with the extension name.
  - [x] `DispatchObserver` fans out concurrently to all subscribers and does not block when one subscriber is slow.
  - [x] `HostAPIRouter.Handle` returns `-32601 method_not_found` for an unregistered namespace.
  - [x] `HostAPIRouter.Handle` returns `-32001 capability_denied` when the capability check fails.
  - [x] `HostAPIRouter.Handle` returns `-32003 not_initialized` when called before the manager marks the extension Ready.
  - [x] Every `DispatchMutable`, `DispatchObserver`, and `HostAPIRouter.Handle` call produces an audit record.
- Integration tests:
  - [x] Three-extension chain with priorities 100, 500, 900 on `prompt.post_build` produces the expected final prompt when each extension appends a suffix.
  - [x] The middle extension failing as optional leaves the first and third extensions' contributions in the final result.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `make verify` exits zero with zero lint issues
- Dispatcher and router are consumable by task 08 without code changes
- Chain ordering is deterministic across multiple runs for the same extension set
